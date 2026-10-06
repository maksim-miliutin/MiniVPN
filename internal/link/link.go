package link

import (
	"errors"
	"fmt"
	"net/netip"
	"sync"
	"sync/atomic"

	"github.com/maksim-miliutin/MiniVPN/internal/frame"
)

// Shorter, and Linux cuts a longer datagram silently while Windows returns an error.
const maxDatagram = 65535

var ErrNoPeer = errors.New("link: no peer address yet")

type Conn interface {
	ReadFrom(buf []byte) (int, netip.AddrPort, error)
	WriteTo(datagram []byte, to netip.AddrPort) error
}

type Link struct {
	conn    Conn
	box     *frame.Box
	dropped atomic.Uint64

	// Receive owns in alone; Send may come from several goroutines and takes out in turn.
	in      []byte
	sending sync.Mutex
	out     []byte

	mu   sync.Mutex
	peer netip.AddrPort
}

func New(conn Conn, box *frame.Box, peer netip.AddrPort) *Link {
	return &Link{
		conn: conn,
		box:  box,
		in:   make([]byte, maxDatagram),
		out:  make([]byte, 0, maxDatagram),
		peer: peer,
	}
}

func (l *Link) Send(packet []byte) error {
	peer := l.currentPeer()
	if !peer.IsValid() {
		return ErrNoPeer
	}

	l.sending.Lock()
	defer l.sending.Unlock()

	l.out = l.box.Seal(l.out[:0], packet)
	if err := l.conn.WriteTo(l.out, peer); err != nil {
		return fmt.Errorf("link: sending to %s: %w", peer, err)
	}

	return nil
}

func (l *Link) Receive(dst []byte) ([]byte, error) {
	for {
		n, from, err := l.conn.ReadFrom(l.in)
		if err != nil {
			return nil, fmt.Errorf("link: receiving: %w", err)
		}

		packet, err := l.box.Open(dst, l.in[:n])
		if err != nil {
			l.dropped.Add(1)
			continue
		}

		l.follow(from)

		return packet, nil
	}
}

func (l *Link) Dropped() uint64 {
	return l.dropped.Load()
}

func (l *Link) currentPeer() netip.AddrPort {
	l.mu.Lock()
	defer l.mu.Unlock()

	return l.peer
}

func (l *Link) follow(from netip.AddrPort) {
	l.mu.Lock()
	defer l.mu.Unlock()

	// A dual-stack socket reports an IPv4 peer as ::ffff:a.b.c.d.
	l.peer = netip.AddrPortFrom(from.Addr().Unmap(), from.Port())
}
