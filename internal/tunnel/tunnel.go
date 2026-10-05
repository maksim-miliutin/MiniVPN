package tunnel

import (
	"errors"
	"fmt"
	"net"
	"net/netip"
	"sync/atomic"

	"github.com/maksim-miliutin/MiniVPN/internal/packet"
)

const maxPacket = 65535

type Device interface {
	Read(buf []byte) (int, error)
	Write(packet []byte) error
}

type Peer interface {
	Send(packet []byte) error
	Receive(dst []byte) ([]byte, error)
}

type Tunnel struct {
	dev     Device
	peer    Peer
	peerIP  netip.Addr
	refused atomic.Uint64
}

func New(dev Device, peer Peer, peerIP netip.Addr) *Tunnel {
	return &Tunnel{dev: dev, peer: peer, peerIP: peerIP}
}

func (t *Tunnel) Outbound() error {
	buf := make([]byte, maxPacket)

	for {
		n, err := t.dev.Read(buf)
		if err != nil {
			return fmt.Errorf("tunnel: reading the device: %w", err)
		}

		if _, err := packet.ParseIPv4(buf[:n]); err != nil {
			continue
		}

		// Like a router, drop a packet that cannot go; only a closed socket ends the loop.
		if err := t.peer.Send(buf[:n]); errors.Is(err, net.ErrClosed) {
			return fmt.Errorf("tunnel: sending: %w", err)
		}
	}
}

func (t *Tunnel) Inbound() error {
	buf := make([]byte, 0, maxPacket)

	for {
		p, err := t.peer.Receive(buf[:0])
		if err != nil {
			return fmt.Errorf("tunnel: receiving: %w", err)
		}

		h, err := packet.ParseIPv4(p)
		if err != nil || h.Src != t.peerIP {
			t.refused.Add(1)
			continue
		}

		if err := t.dev.Write(p); err != nil {
			return fmt.Errorf("tunnel: writing the device: %w", err)
		}
	}
}

func (t *Tunnel) Refused() uint64 {
	return t.refused.Load()
}
