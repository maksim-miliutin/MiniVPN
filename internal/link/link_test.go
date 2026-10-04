package link

import (
	"bytes"
	"errors"
	"io"
	"net/netip"
	"sync"
	"testing"

	"github.com/maksim-miliutin/MiniVPN/internal/frame"
)

const largestIPv6Payload = 65535 - 8 // the IPv6 length field leaves its own header out

var (
	clientAddr = netip.MustParseAddrPort("192.0.2.10:40000")
	serverAddr = netip.MustParseAddrPort("198.51.100.1:51820")
	strayAddr  = netip.MustParseAddrPort("203.0.113.66:666")
)

type datagram struct {
	from netip.AddrPort
	data []byte
}

// A wire delivers datagrams by address and, like Linux, cuts one to the reader's
// buffer; an empty inbox reads as io.EOF, so no test can block.
type wire struct {
	mu     sync.Mutex
	queued map[netip.AddrPort][]datagram
}

type end struct {
	wire *wire
	addr netip.AddrPort
}

func newWire() *wire {
	return &wire{queued: map[netip.AddrPort][]datagram{}}
}

func (w *wire) at(addr netip.AddrPort) end {
	return end{wire: w, addr: addr}
}

func (w *wire) put(to, from netip.AddrPort, data []byte) {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.queued[to] = append(w.queued[to], datagram{from: from, data: bytes.Clone(data)})
}

func (w *wire) waiting(at netip.AddrPort) int {
	w.mu.Lock()
	defer w.mu.Unlock()

	return len(w.queued[at])
}

func (w *wire) total() int {
	w.mu.Lock()
	defer w.mu.Unlock()

	n := 0
	for _, q := range w.queued {
		n += len(q)
	}

	return n
}

func (e end) ReadFrom(buf []byte) (int, netip.AddrPort, error) {
	e.wire.mu.Lock()
	defer e.wire.mu.Unlock()

	q := e.wire.queued[e.addr]
	if len(q) == 0 {
		return 0, netip.AddrPort{}, io.EOF
	}

	e.wire.queued[e.addr] = q[1:]

	return copy(buf, q[0].data), q[0].from, nil
}

func (e end) WriteTo(data []byte, to netip.AddrPort) error {
	e.wire.put(to, e.addr, data)

	return nil
}

func send(t *testing.T, l *Link, packet string) {
	t.Helper()

	if err := l.Send([]byte(packet)); err != nil {
		t.Fatal(err)
	}
}

func receive(t *testing.T, l *Link) string {
	t.Helper()

	packet, err := l.Receive(nil)
	if err != nil {
		t.Fatal(err)
	}

	return string(packet)
}

func pair(w *wire) (frame.Key, *Link, *Link) {
	k := frame.NewKey()
	client := New(w.at(clientAddr), frame.ClientBox(k), serverAddr)
	server := New(w.at(serverAddr), frame.ServerBox(k), netip.AddrPort{})

	return k, client, server
}

func TestAPacketCrossesTheLinkBothWays(t *testing.T) {
	_, client, server := pair(newWire())

	send(t, client, "ping")
	if got := receive(t, server); got != "ping" {
		t.Fatalf("the server got %q", got)
	}

	send(t, server, "pong")
	if got := receive(t, client); got != "pong" {
		t.Fatalf("the client got %q", got)
	}
}

func TestAServerCannotSendBeforeTheClientSpeaks(t *testing.T) {
	w := newWire()
	_, _, server := pair(w)

	if err := server.Send([]byte("pong")); !errors.Is(err, ErrNoPeer) {
		t.Fatalf("got %v, want ErrNoPeer", err)
	}

	if n := w.total(); n != 0 {
		t.Errorf("%d datagrams went out with no peer to send to", n)
	}
}

func TestAFrameThatDoesNotOpenIsDroppedSilently(t *testing.T) {
	w := newWire()
	k, client, server := pair(w)
	send(t, client, "hello")
	receive(t, server)

	w.put(serverAddr, strayAddr, []byte("garbage"))
	w.put(serverAddr, strayAddr, frame.ClientBox(frame.NewKey()).Seal(nil, []byte("another key")))
	w.put(serverAddr, strayAddr, frame.ServerBox(k).Seal(nil, []byte("bounced back")))

	if _, err := server.Receive(nil); !errors.Is(err, io.EOF) {
		t.Fatalf("got %v, want the wire to run dry with nothing let through", err)
	}

	if n := server.Dropped(); n != 3 {
		t.Errorf("dropped %d frames, want 3", n)
	}

	send(t, server, "still yours")
	if w.waiting(clientAddr) != 1 || w.waiting(strayAddr) != 0 {
		t.Error("the frames that did not open moved the peer or drew a reply")
	}
}

func TestThePeerFollowsTheLastFrameThatOpened(t *testing.T) {
	w := newWire()
	k, client, server := pair(w)
	moved := netip.MustParseAddrPort("192.0.2.77:41000")

	send(t, client, "from home")
	receive(t, server)
	send(t, New(w.at(moved), frame.ClientBox(k), serverAddr), "from the train")
	receive(t, server)
	send(t, server, "where are you")

	if w.waiting(moved) != 1 || w.waiting(clientAddr) != 0 {
		t.Error("the reply did not follow the client to its new address")
	}
}

func TestAMappedIPv4PeerIsAnsweredAsIPv4(t *testing.T) {
	w := newWire()
	k, _, server := pair(w)
	mapped := netip.MustParseAddrPort("[::ffff:192.0.2.10]:40000")

	w.put(serverAddr, mapped, frame.ClientBox(k).Seal(nil, []byte("hi")))
	receive(t, server)
	send(t, server, "hello")

	if w.waiting(clientAddr) != 1 {
		t.Error("the reply went to the mapped form instead of 192.0.2.10:40000")
	}
}

func TestTheLargestDatagramFits(t *testing.T) {
	_, client, server := pair(newWire())
	big := bytes.Repeat([]byte{0xA5}, largestIPv6Payload-frame.Overhead)

	if err := client.Send(big); err != nil {
		t.Fatal(err)
	}

	got, err := server.Receive(nil)
	if err != nil || !bytes.Equal(got, big) {
		t.Errorf("got %d bytes of %d: %v", len(got), len(big), err)
	}
}

// Only go test -race can fail this: it watches the peer that both goroutines share.
func TestSendAndReceiveCanRunAtOnce(t *testing.T) {
	w := newWire()
	k, _, server := pair(w)
	for i := range 100 {
		from := netip.AddrPortFrom(clientAddr.Addr(), uint16(40000+i))
		w.put(serverAddr, from, frame.ClientBox(k).Seal(nil, []byte("hi")))
	}

	var wg sync.WaitGroup
	wg.Go(func() {
		for range 100 {
			server.Receive(nil)
		}
	})
	wg.Go(func() {
		for range 100 {
			server.Send([]byte("hello"))
		}
	})
	wg.Wait()
}
