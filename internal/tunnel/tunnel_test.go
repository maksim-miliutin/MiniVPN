package tunnel

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"net/netip"
	"testing"
	"time"
)

var (
	localIP = netip.MustParseAddr("10.9.0.2")
	peerIP  = netip.MustParseAddr("10.9.0.1")
)

func ipv4(src, dst netip.Addr, size int) []byte {
	p := make([]byte, size)
	p[0] = 0x45
	binary.BigEndian.PutUint16(p[2:4], uint16(size))
	copy(p[12:16], src.AsSlice())
	copy(p[16:20], dst.AsSlice())

	return p
}

// The device and the peer below run dry with io.EOF, so every loop in a test ends.
type device struct {
	reads   [][]byte
	written [][]byte
}

func (d *device) Read(buf []byte) (int, error) {
	if len(d.reads) == 0 {
		return 0, io.EOF
	}

	n := copy(buf, d.reads[0])
	d.reads = d.reads[1:]

	return n, nil
}

func (d *device) Write(packet []byte) error {
	d.written = append(d.written, bytes.Clone(packet))

	return nil
}

type peer struct {
	inbox  [][]byte
	sent   [][]byte
	refuse []error
}

func (p *peer) Send(packet []byte) error {
	if len(p.refuse) > 0 {
		err := p.refuse[0]
		p.refuse = p.refuse[1:]
		if err != nil {
			return err
		}
	}

	p.sent = append(p.sent, bytes.Clone(packet))

	return nil
}

func (p *peer) Receive(dst []byte) ([]byte, error) {
	if len(p.inbox) == 0 {
		return nil, io.EOF
	}

	packet := append(dst, p.inbox[0]...)
	p.inbox = p.inbox[1:]

	return packet, nil
}

func TestOutboundSendsIPv4AndLeavesTheRest(t *testing.T) {
	a, b := ipv4(localIP, peerIP, 60), ipv4(localIP, peerIP, 1400)
	v6 := make([]byte, 40)
	v6[0] = 0x60
	dev := &device{reads: [][]byte{a, v6, {1, 2, 3}, b}}
	to := &peer{}

	if err := New(dev, to, peerIP).Outbound(); !errors.Is(err, io.EOF) {
		t.Fatalf("got %v, want the device to run dry", err)
	}

	if len(to.sent) != 2 || !bytes.Equal(to.sent[0], a) || !bytes.Equal(to.sent[1], b) {
		t.Errorf("sent %d packets, want the two IPv4 ones in order", len(to.sent))
	}
}

func TestOutboundOutlivesAPacketThatCouldNotGo(t *testing.T) {
	a, b := ipv4(localIP, peerIP, 60), ipv4(localIP, peerIP, 80)
	to := &peer{refuse: []error{errors.New("no peer address yet"), nil}}

	New(&device{reads: [][]byte{a, b}}, to, peerIP).Outbound()

	if len(to.sent) != 1 || !bytes.Equal(to.sent[0], b) {
		t.Errorf("sent %d packets, want only the second", len(to.sent))
	}
}

func TestOutboundStopsWhenTheSocketCloses(t *testing.T) {
	dev := &device{reads: [][]byte{ipv4(localIP, peerIP, 60), ipv4(localIP, peerIP, 60)}}
	to := &peer{refuse: []error{net.ErrClosed}}

	if err := New(dev, to, peerIP).Outbound(); !errors.Is(err, net.ErrClosed) {
		t.Fatalf("got %v, want net.ErrClosed", err)
	}

	if len(dev.reads) != 1 {
		t.Errorf("%d packets left unread, want 1: the loop went on after the socket closed", len(dev.reads))
	}
}

func TestInboundWritesWhatThePeerSends(t *testing.T) {
	p := ipv4(peerIP, localIP, 100)
	dev := &device{}
	tun := New(dev, &peer{inbox: [][]byte{p}}, peerIP)

	if err := tun.Inbound(); !errors.Is(err, io.EOF) {
		t.Fatalf("got %v, want the peer to run dry", err)
	}

	if len(dev.written) != 1 || !bytes.Equal(dev.written[0], p) || tun.Refused() != 0 {
		t.Errorf("wrote %d packets, refused %d", len(dev.written), tun.Refused())
	}
}

func TestInboundRefusesWhatThePeerCannotHaveSent(t *testing.T) {
	good := ipv4(peerIP, localIP, 100)
	spoofed := ipv4(netip.MustParseAddr("10.9.0.99"), localIP, 100)
	v6 := make([]byte, 40)
	v6[0] = 0x60
	lying := ipv4(peerIP, localIP, 100)[:99]

	dev := &device{}
	tun := New(dev, &peer{inbox: [][]byte{spoofed, v6, lying, good}}, peerIP)
	tun.Inbound()

	if len(dev.written) != 1 || !bytes.Equal(dev.written[0], good) {
		t.Errorf("wrote %d packets, want only the good one", len(dev.written))
	}

	if n := tun.Refused(); n != 3 {
		t.Errorf("refused %d, want 3", n)
	}
}

func TestEachTickSendsOneEmptyFrame(t *testing.T) {
	to := &peer{}
	ticks := make(chan time.Time, 3)
	for range 3 {
		ticks <- time.Now()
	}
	close(ticks)

	if err := New(&device{}, to, peerIP).KeepAlive(ticks); err != nil {
		t.Fatal(err)
	}

	if len(to.sent) != 3 {
		t.Fatalf("sent %d keepalives, want 3", len(to.sent))
	}

	for _, p := range to.sent {
		if len(p) != 0 {
			t.Errorf("a keepalive carries %d bytes, want none", len(p))
		}
	}
}

func TestKeepAliveStopsWhenTheSocketCloses(t *testing.T) {
	ticks := make(chan time.Time, 2)
	ticks <- time.Now()
	ticks <- time.Now()
	close(ticks)

	err := New(&device{}, &peer{refuse: []error{net.ErrClosed}}, peerIP).KeepAlive(ticks)
	if !errors.Is(err, net.ErrClosed) {
		t.Fatalf("got %v, want net.ErrClosed", err)
	}

	if len(ticks) != 1 {
		t.Error("the loop went on after the socket closed")
	}
}

func TestAKeepAliveIsNotRefused(t *testing.T) {
	dev := &device{}
	tun := New(dev, &peer{inbox: [][]byte{{}, ipv4(peerIP, localIP, 100)}}, peerIP)
	tun.Inbound()

	if tun.Refused() != 0 || len(dev.written) != 1 {
		t.Errorf("refused %d, wrote %d: a keepalive passed for a bad packet", tun.Refused(), len(dev.written))
	}
}
