package link

import (
	"fmt"
	"net"
	"net/netip"
)

type UDP struct {
	conn *net.UDPConn
}

func Listen(address string) (*UDP, error) {
	local, err := net.ResolveUDPAddr("udp", address)
	if err != nil {
		return nil, fmt.Errorf("link: %q: %w", address, err)
	}

	conn, err := net.ListenUDP("udp", local)
	if err != nil {
		return nil, fmt.Errorf("link: listening on %s: %w", address, err)
	}

	return &UDP{conn: conn}, nil
}

func (u *UDP) ReadFrom(buf []byte) (int, netip.AddrPort, error) {
	return u.conn.ReadFromUDPAddrPort(buf)
}

func (u *UDP) WriteTo(datagram []byte, to netip.AddrPort) error {
	_, err := u.conn.WriteToUDPAddrPort(datagram, to)

	return err
}

func (u *UDP) LocalAddr() netip.AddrPort {
	return u.conn.LocalAddr().(*net.UDPAddr).AddrPort()
}

func (u *UDP) Close() error {
	return u.conn.Close()
}
