package main

import (
	"errors"
	"net/netip"
	"testing"
)

func TestTheDefaultsMakeAWorkingPair(t *testing.T) {
	server, err := parse([]string{"server"})
	if err != nil {
		t.Fatal(err)
	}

	client, err := parse([]string{"client", "-server", "198.51.100.1:51821"})
	if err != nil {
		t.Fatal(err)
	}

	if server.listen != ":51821" || server.key != "minivpn.key" || client.key != "minivpn.key" {
		t.Errorf("server listens on %q with %q, client reads %q", server.listen, server.key, client.key)
	}

	if client.listen != ":0" || client.server != "198.51.100.1:51821" {
		t.Errorf("client listens on %q and sends to %q", client.listen, client.server)
	}

	if server.addr.Addr() != client.peer || client.addr.Addr() != server.peer {
		t.Errorf("the ends do not point at each other: server %v peer %v, client %v peer %v",
			server.addr, server.peer, client.addr, client.peer)
	}
}

func TestFlagsOverrideTheDefaults(t *testing.T) {
	c, err := parse([]string{"client", "-server", "vpn.example:4000", "-key", "other.key", "-addr", "10.7.0.5/24", "-peer", "10.7.0.9"})
	if err != nil {
		t.Fatal(err)
	}

	want := command{name: "client", key: "other.key", listen: ":0", server: "vpn.example:4000",
		addr: netip.MustParsePrefix("10.7.0.5/24"), peer: netip.MustParseAddr("10.7.0.9")}
	if c != want {
		t.Errorf("got %+v", c)
	}
}

func TestAClientNeedsAServer(t *testing.T) {
	if _, err := parse([]string{"client"}); !errors.Is(err, ErrNoServer) {
		t.Errorf("got %v, want ErrNoServer", err)
	}
}

func TestThePeerIsAnotherAddressInTheSubnet(t *testing.T) {
	cases := map[string]struct {
		args []string
		want error
	}{
		"outside": {[]string{"server", "-peer", "10.10.0.2"}, ErrPeerOutside},
		"itself":  {[]string{"server", "-peer", "10.9.0.1"}, ErrPeerIsSelf},
	}

	for name, c := range cases {
		if _, err := parse(c.args); !errors.Is(err, c.want) {
			t.Errorf("%s: got %v, want %v", name, err, c.want)
		}
	}
}

func TestWhatIsNotACommandIsUsage(t *testing.T) {
	for _, args := range [][]string{
		{},
		{"start"},
		{"genkey", "a.key", "b.key"},
		{"server", "-listen"},
		{"server", "-server", "198.51.100.1:51821"},
		{"client", "-listen", ":4000", "-server", "198.51.100.1:51821"},
		{"client", "-server", "198.51.100.1:51821", "extra"},
	} {
		if _, err := parse(args); !errors.Is(err, ErrUsage) {
			t.Errorf("%q: got %v, want ErrUsage", args, err)
		}
	}
}

func TestTheTunnelIsIPv4Only(t *testing.T) {
	for _, args := range [][]string{
		{"server", "-addr", "fd00::1/64", "-peer", "fd00::2"},
		{"server", "-peer", "::ffff:10.9.0.2"},
	} {
		if _, err := parse(args); !errors.Is(err, ErrNotIPv4) {
			t.Errorf("%q: got %v, want ErrNotIPv4", args, err)
		}
	}
}

func TestGenkeyTakesAnOptionalPath(t *testing.T) {
	for args, want := range map[string]string{"": "minivpn.key", "other.key": "other.key"} {
		line := []string{"genkey"}
		if args != "" {
			line = append(line, args)
		}

		c, err := parse(line)
		if err != nil || c.name != "genkey" || c.key != want {
			t.Errorf("%q: got %+v, %v", line, c, err)
		}
	}
}
