package adapter

import (
	"errors"
	"net/netip"
	"slices"
	"strings"
	"testing"
)

func valid() Config {
	return Config{Name: "MiniVPN", Prefix: netip.MustParsePrefix("10.9.0.2/24"), MTU: 1400}
}

func TestAConfigBecomesTwoNetshCommands(t *testing.T) {
	want := [][]string{
		{"interface", "ipv4", "set", "address", "name=MiniVPN", "source=static", "address=10.9.0.2", "mask=255.255.255.0"},
		{"interface", "ipv4", "set", "subinterface", "MiniVPN", "mtu=1400", "store=active"},
	}

	if got := valid().netsh(); !slices.EqualFunc(got, want, slices.Equal) {
		t.Errorf("got %q", got)
	}
}

func TestTheSameConfigBecomesTwoIPCommandsOnLinux(t *testing.T) {
	want := [][]string{
		{"addr", "add", "10.9.0.2/24", "dev", "MiniVPN"},
		{"link", "set", "dev", "MiniVPN", "mtu", "1400", "up"},
	}

	if got := valid().ip(); !slices.EqualFunc(got, want, slices.Equal) {
		t.Errorf("got %q", got)
	}
}

func TestTheMaskFollowsThePrefix(t *testing.T) {
	for prefix, mask := range map[string]string{"10.9.0.2/30": "255.255.255.252", "10.9.0.2/16": "255.255.0.0", "10.9.0.2/32": "255.255.255.255"} {
		c := valid()
		c.Prefix = netip.MustParsePrefix(prefix)

		if got := c.netsh()[0][7]; got != "mask="+mask {
			t.Errorf("%s: got %s, want mask=%s", prefix, got, mask)
		}
	}
}

func TestAValidConfigPasses(t *testing.T) {
	edges := []Config{valid()}
	for _, mtu := range []int{576, 65535} {
		c := valid()
		c.MTU = mtu
		edges = append(edges, c)
	}

	long := valid()
	long.Name = strings.Repeat("n", 127)
	edges = append(edges, long)

	for _, c := range edges {
		if err := c.check(); err != nil {
			t.Errorf("%q, MTU %d: %v", c.Name, c.MTU, err)
		}
	}
}

func TestCheckRefusesWhatWintunOrIPv4Cannot(t *testing.T) {
	cases := map[string]struct {
		edit func(*Config)
		want error
	}{
		"empty name":      {func(c *Config) { c.Name = "" }, ErrName},
		"128-letter name": {func(c *Config) { c.Name = strings.Repeat("n", 128) }, ErrName},
		"IPv6 address":    {func(c *Config) { c.Prefix = netip.MustParsePrefix("fd00::2/64") }, ErrPrefix},
		"no address":      {func(c *Config) { c.Prefix = netip.Prefix{} }, ErrPrefix},
		"MTU 575":         {func(c *Config) { c.MTU = 575 }, ErrMTU},
		"MTU 65536":       {func(c *Config) { c.MTU = 65536 }, ErrMTU},
	}

	for name, tc := range cases {
		c := valid()
		tc.edit(&c)

		if err := c.check(); !errors.Is(err, tc.want) {
			t.Errorf("%s: got %v, want %v", name, err, tc.want)
		}
	}
}

func TestTheNameLimitCountsUTF16Units(t *testing.T) {
	c := valid()
	c.Name = strings.Repeat("🛡", 64)

	if err := c.check(); !errors.Is(err, ErrName) {
		t.Errorf("64 shields take 128 UTF-16 units: got %v, want ErrName", err)
	}
}
