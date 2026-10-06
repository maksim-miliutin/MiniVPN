package main

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"net/netip"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/maksim-miliutin/MiniVPN/internal/adapter"
	"github.com/maksim-miliutin/MiniVPN/internal/frame"
)

var (
	serverAt = netip.MustParseAddrPort("198.51.100.1:51821")
	clientAt = netip.MustParseAddrPort("192.0.2.10:40000")
)

type datagram struct {
	from netip.AddrPort
	data []byte
}

// The air carries datagrams between the sockets on it and, like UDP, loses those
// sent where no socket listens.
type air struct {
	mu    sync.Mutex
	socks map[netip.AddrPort]*sock
}

type sock struct {
	air   *air
	at    netip.AddrPort
	inbox chan datagram
	done  chan struct{}
	once  sync.Once
}

func newAir() *air {
	return &air{socks: map[netip.AddrPort]*sock{}}
}

func (a *air) open(at netip.AddrPort) *sock {
	s := &sock{air: a, at: at, inbox: make(chan datagram, 64), done: make(chan struct{})}

	a.mu.Lock()
	defer a.mu.Unlock()
	a.socks[at] = s

	return s
}

func (s *sock) ReadFrom(buf []byte) (int, netip.AddrPort, error) {
	select {
	case d := <-s.inbox:
		return copy(buf, d.data), d.from, nil
	case <-s.done:
		return 0, netip.AddrPort{}, net.ErrClosed
	}
}

func (s *sock) WriteTo(data []byte, to netip.AddrPort) error {
	if s.closed() {
		return net.ErrClosed
	}

	s.air.mu.Lock()
	dst := s.air.socks[to]
	s.air.mu.Unlock()

	if dst != nil {
		select {
		case dst.inbox <- datagram{from: s.at, data: bytes.Clone(data)}:
		default:
		}
	}

	return nil
}

func (s *sock) Close() error {
	s.once.Do(func() { close(s.done) })

	return nil
}

func (s *sock) closed() bool {
	select {
	case <-s.done:
		return true
	default:
		return false
	}
}

// A host stands for the operating system behind the adapter: what it sends goes
// into the tunnel, what the tunnel delivers it gets.
type host struct {
	sends chan []byte
	gets  chan []byte
	done  chan struct{}
	once  sync.Once
}

func newHost() *host {
	return &host{sends: make(chan []byte, 16), gets: make(chan []byte, 16), done: make(chan struct{})}
}

func (h *host) Read(buf []byte) (int, error) {
	select {
	case p := <-h.sends:
		return copy(buf, p), nil
	case <-h.done:
		return 0, os.ErrClosed
	}
}

func (h *host) Write(packet []byte) error {
	select {
	case h.gets <- bytes.Clone(packet):
	default:
	}

	return nil
}

func (h *host) Close() error {
	h.once.Do(func() { close(h.done) })

	return nil
}

func depsFor(s *sock, h *host, key string, out io.Writer) deps {
	return deps{
		listen: func(string) (conn, error) { return s, nil },
		open:   func(adapter.Config) (device, error) { return h, nil },
		read:   func(string) ([]byte, error) { return []byte(key), nil },
		out:    out,
	}
}

func start(ctx context.Context, c command, d deps) <-chan error {
	ended := make(chan error, 1)
	go func() { ended <- run(ctx, c, d) }()

	return ended
}

func ipv4(src, dst string, size int) []byte {
	p := make([]byte, size)
	p[0] = 0x45
	binary.BigEndian.PutUint16(p[2:4], uint16(size))
	copy(p[12:16], netip.MustParseAddr(src).AsSlice())
	copy(p[16:20], netip.MustParseAddr(dst).AsSlice())

	return p
}

func await(t *testing.T, packets <-chan []byte, what string) []byte {
	t.Helper()

	select {
	case p := <-packets:
		return p
	case <-time.After(2 * time.Second):
		t.Fatalf("%s did not arrive in 2 seconds", what)
		return nil
	}
}

func stopped(t *testing.T, name string, ended <-chan error) {
	t.Helper()

	select {
	case err := <-ended:
		if err != nil {
			t.Errorf("the %s stopped with %v", name, err)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("the %s did not stop in 2 seconds", name)
	}
}

func pair(t *testing.T) (server, client command) {
	t.Helper()

	server, err := parse([]string{"server"})
	if err != nil {
		t.Fatal(err)
	}

	client, err = parse([]string{"client", "-server", serverAt.String()})
	if err != nil {
		t.Fatal(err)
	}

	return server, client
}

func TestAPacketCrossesFromClientToServerAndBack(t *testing.T) {
	key := frame.NewKey().Text()
	sky := newAir()
	serverHost, clientHost := newHost(), newHost()
	var serverOut, clientOut bytes.Buffer
	server, client := pair(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	serverEnded := start(ctx, server, depsFor(sky.open(serverAt), serverHost, key, &serverOut))
	clientEnded := start(ctx, client, depsFor(sky.open(clientAt), clientHost, key, &clientOut))

	ping := ipv4("10.9.0.2", "10.9.0.1", 84)
	for range 2 {
		clientHost.sends <- ping
		if got := await(t, serverHost.gets, "the ping"); !bytes.Equal(got, ping) {
			t.Fatal("the server got another packet than the client sent")
		}
	}

	pong := ipv4("10.9.0.1", "10.9.0.2", 84)
	serverHost.sends <- pong
	if got := await(t, clientHost.gets, "the pong"); !bytes.Equal(got, pong) {
		t.Fatal("the client got another packet than the server sent")
	}

	cancel()
	stopped(t, "server", serverEnded)
	stopped(t, "client", clientEnded)

	want := map[string]string{
		serverOut.String(): "packets out 1, in 2; frames that did not open 0, packets refused 0",
		clientOut.String(): "packets out 2, in 1; frames that did not open 0, packets refused 0",
	}
	for out, line := range want {
		if !strings.Contains(out, line) {
			t.Errorf("summary %q, want %q", out, line)
		}
	}
}

func TestAClientWithAnotherKeyGetsNowhere(t *testing.T) {
	sky := newAir()
	serverHost, clientHost := newHost(), newHost()
	var serverOut bytes.Buffer
	server, client := pair(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	serverEnded := start(ctx, server, depsFor(sky.open(serverAt), serverHost, frame.NewKey().Text(), &serverOut))
	clientEnded := start(ctx, client, depsFor(sky.open(clientAt), clientHost, frame.NewKey().Text(), io.Discard))

	clientHost.sends <- ipv4("10.9.0.2", "10.9.0.1", 84)
	select {
	case <-serverHost.gets:
		t.Fatal("a packet sealed with another key reached the server")
	case <-time.After(300 * time.Millisecond):
	}

	cancel()
	stopped(t, "server", serverEnded)
	stopped(t, "client", clientEnded)

	if !strings.Contains(serverOut.String(), "in 0; frames that did not open 2,") {
		t.Errorf("want the first keepalive and the packet dropped; the server's summary: %q", serverOut.String())
	}
}

func TestTheAdapterGetsTheTunnelAddressAndMTU(t *testing.T) {
	var asked adapter.Config
	s := newAir().open(serverAt)
	server, _ := pair(t)

	d := depsFor(s, nil, frame.NewKey().Text(), io.Discard)
	d.open = func(c adapter.Config) (device, error) {
		asked = c

		return nil, errors.New("no wintun.dll")
	}

	if err := run(context.Background(), server, d); err == nil {
		t.Fatal("run went on without an adapter")
	}

	want := adapter.Config{Name: "MiniVPN", Prefix: netip.MustParsePrefix("10.9.0.1/24"), MTU: 1400}
	if asked != want {
		t.Errorf("asked for %+v, want %+v", asked, want)
	}

	if !s.closed() {
		t.Error("the socket stayed open after the adapter failed")
	}
}

func TestTheMTULeavesRoomForTheFrame(t *testing.T) {
	const outerIPv6, udp, path = 40, 8, 1500

	if mtu+outerIPv6+udp+frame.Overhead > path {
		t.Errorf("%d + %d + %d + %d is over %d", mtu, outerIPv6, udp, frame.Overhead, path)
	}
}

func TestTheFirstTickComesAtOnceAndStoppingEndsThem(t *testing.T) {
	stop := make(chan struct{})
	ticks := every(time.Hour, stop)

	select {
	case <-ticks:
	case <-time.After(time.Second):
		t.Fatal("the first tick waited")
	}

	close(stop)
	select {
	case _, open := <-ticks:
		if open {
			t.Fatal("a tick came after stopping")
		}
	case <-time.After(time.Second):
		t.Fatal("the ticks went on after stopping")
	}
}

func TestALoopThatFailsStopsTheRunWithItsError(t *testing.T) {
	gone := newHost()
	gone.Close()
	server, _ := pair(t)

	err := run(context.Background(), server, depsFor(newAir().open(serverAt), gone, frame.NewKey().Text(), io.Discard))
	if !errors.Is(err, os.ErrClosed) {
		t.Errorf("got %v, want the adapter's own error", err)
	}
}
