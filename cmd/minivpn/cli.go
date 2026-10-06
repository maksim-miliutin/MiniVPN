package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"net/netip"
)

var (
	ErrUsage       = errors.New("minivpn: usage: minivpn genkey [file] | server [-listen :51821] | client -server host:port; both ends take -key, -addr, -peer")
	ErrNoServer    = errors.New("minivpn: the client needs -server host:port")
	ErrNotIPv4     = errors.New("minivpn: the tunnel speaks IPv4 only")
	ErrPeerOutside = errors.New("minivpn: the peer is outside the tunnel subnet")
	ErrPeerIsSelf  = errors.New("minivpn: the peer has this end's own address")
)

type command struct {
	name   string
	key    string
	listen string
	server string
	addr   netip.Prefix
	peer   netip.Addr
}

func parse(args []string) (command, error) {
	if len(args) == 0 {
		return command{}, ErrUsage
	}

	switch args[0] {
	case "genkey":
		return parseGenkey(args[1:])
	case "server", "client":
		return parseEnd(args[0], args[1:])
	}

	return command{}, ErrUsage
}

func parseGenkey(args []string) (command, error) {
	if len(args) > 1 {
		return command{}, ErrUsage
	}

	c := command{name: "genkey", key: "minivpn.key"}
	if len(args) == 1 {
		c.key = args[0]
	}

	return c, nil
}

func parseEnd(name string, args []string) (command, error) {
	c := command{name: name, listen: ":0"}
	ours, theirs := "10.9.0.1/24", "10.9.0.2"

	flags := flag.NewFlagSet(name, flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	if name == "server" {
		flags.StringVar(&c.listen, "listen", ":51821", "")
	} else {
		ours, theirs = "10.9.0.2/24", "10.9.0.1"
		flags.StringVar(&c.server, "server", "", "")
	}
	flags.StringVar(&c.key, "key", "minivpn.key", "")
	flags.TextVar(&c.addr, "addr", netip.MustParsePrefix(ours), "")
	flags.TextVar(&c.peer, "peer", netip.MustParseAddr(theirs), "")

	if err := flags.Parse(args); err != nil {
		return command{}, fmt.Errorf("%w: %w", ErrUsage, err)
	}

	if flags.NArg() > 0 {
		return command{}, ErrUsage
	}

	return c, c.check()
}

func (c command) check() error {
	if c.name == "client" && c.server == "" {
		return ErrNoServer
	}

	if !c.addr.Addr().Is4() || !c.peer.Is4() {
		return ErrNotIPv4
	}

	if !c.addr.Contains(c.peer) {
		return ErrPeerOutside
	}

	if c.peer == c.addr.Addr() {
		return ErrPeerIsSelf
	}

	return nil
}
