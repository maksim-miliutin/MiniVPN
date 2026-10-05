package packet

import (
	"encoding/binary"
	"errors"
	"net/netip"
)

const headerMin = 20

var (
	ErrShort   = errors.New("packet: shorter than an IPv4 header")
	ErrNotIPv4 = errors.New("packet: not IPv4")
	ErrHeader  = errors.New("packet: header length out of range")
	ErrLength  = errors.New("packet: total length differs from the bytes at hand")
)

type IPv4 struct {
	Src netip.Addr
	Dst netip.Addr
}

func ParseIPv4(packet []byte) (IPv4, error) {
	if len(packet) < headerMin {
		return IPv4{}, ErrShort
	}

	if packet[0]>>4 != 4 {
		return IPv4{}, ErrNotIPv4
	}

	header := int(packet[0]&0x0f) * 4
	if header < headerMin || header > len(packet) {
		return IPv4{}, ErrHeader
	}

	if int(binary.BigEndian.Uint16(packet[2:4])) != len(packet) {
		return IPv4{}, ErrLength
	}

	return IPv4{
		Src: netip.AddrFrom4([4]byte(packet[12:16])),
		Dst: netip.AddrFrom4([4]byte(packet[16:20])),
	}, nil
}
