package adapter

import (
	"errors"
	"fmt"
	"net"
	"net/netip"
	"strconv"
	"unicode/utf16"
)

const (
	nameMax = 127   // UTF-16 units; wintun's AdapterNameMax of 128 counts the closing zero
	mtuMin  = 576   // the datagram every IPv4 host must accept
	mtuMax  = 65535 // wintun's PacketSizeMax
)

var (
	ErrName   = errors.New("adapter: the name is empty or too long")
	ErrPrefix = errors.New("adapter: the address is not IPv4 with a mask")
	ErrMTU    = errors.New("adapter: the MTU is out of range")
)

type Config struct {
	Name   string
	Prefix netip.Prefix
	MTU    int
}

func (c Config) check() error {
	if n := len(utf16.Encode([]rune(c.Name))); n == 0 || n > nameMax {
		return ErrName
	}

	if !c.Prefix.IsValid() || !c.Prefix.Addr().Is4() {
		return ErrPrefix
	}

	if c.MTU < mtuMin || c.MTU > mtuMax {
		return fmt.Errorf("%w: %d", ErrMTU, c.MTU)
	}

	return nil
}

func (c Config) netsh() [][]string {
	mask := net.IP(net.CIDRMask(c.Prefix.Bits(), 32)).String()

	return [][]string{
		{"interface", "ipv4", "set", "address", "name=" + c.Name, "source=static", "address=" + c.Prefix.Addr().String(), "mask=" + mask},
		{"interface", "ipv4", "set", "subinterface", c.Name, "mtu=" + strconv.Itoa(c.MTU), "store=active"},
	}
}
