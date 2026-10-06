package main

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/netip"
	"time"

	"github.com/maksim-miliutin/MiniVPN/internal/adapter"
	"github.com/maksim-miliutin/MiniVPN/internal/frame"
	"github.com/maksim-miliutin/MiniVPN/internal/link"
	"github.com/maksim-miliutin/MiniVPN/internal/tunnel"
)

// 1500 minus an outer IPv6 header (40), UDP (8), the nonce (24) and the tag (16),
// rounded down.
const mtu = 1400

type device interface {
	tunnel.Device
	Close() error
}

type conn interface {
	link.Conn
	Close() error
}

type deps struct {
	listen func(address string) (conn, error)
	open   func(adapter.Config) (device, error)
	read   func(path string) ([]byte, error)
	out    io.Writer
}

func run(ctx context.Context, c command, d deps) error {
	key, err := readKey(c.key, d.read)
	if err != nil {
		return err
	}

	box, server := frame.ServerBox(key), netip.AddrPort{}
	if c.name == "client" {
		box = frame.ClientBox(key)
		if server, err = resolve(c.server); err != nil {
			return err
		}
	}

	sock, err := d.listen(c.listen)
	if err != nil {
		return err
	}

	dev, err := d.open(adapter.Config{Name: "MiniVPN", Prefix: c.addr, MTU: mtu})
	if err != nil {
		sock.Close()

		return err
	}

	l := link.New(sock, box, server)
	t := tunnel.New(dev, l, c.peer)
	fmt.Fprintf(d.out, "minivpn: %s up as %s, peer %s\n", c.name, c.addr, c.peer)

	stop := make(chan struct{})
	loops := []func() error{t.Outbound, t.Inbound}
	if c.name == "client" {
		loops = append(loops, func() error { return t.KeepAlive(every(tunnel.KeepAliveEvery, stop)) })
	}

	ended := make(chan error, len(loops))
	for _, loop := range loops {
		go func() { ended <- loop() }()
	}

	running := len(loops)
	var failed error
	select {
	case <-ctx.Done():
	case failed = <-ended:
		running--
	}

	close(stop)
	dev.Close()
	sock.Close()
	for range running {
		<-ended
	}

	fmt.Fprintf(d.out, "minivpn: stopped; %d frames did not open, %d packets refused\n", l.Dropped(), t.Refused())

	return failed
}

func resolve(server string) (netip.AddrPort, error) {
	addr, err := net.ResolveUDPAddr("udp", server)
	if err != nil {
		return netip.AddrPort{}, fmt.Errorf("minivpn: %q: %w", server, err)
	}

	at := addr.AddrPort()

	return netip.AddrPortFrom(at.Addr().Unmap(), at.Port()), nil
}

// The first tick goes at once, so the server learns where the client is before any
// traffic needs it.
func every(d time.Duration, stop <-chan struct{}) <-chan time.Time {
	ticks := make(chan time.Time, 1)
	ticks <- time.Now()

	go func() {
		defer close(ticks)

		ticker := time.NewTicker(d)
		defer ticker.Stop()

		for {
			select {
			case now := <-ticker.C:
				select {
				case ticks <- now:
				default:
				}
			case <-stop:
				return
			}
		}
	}()

	return ticks
}
