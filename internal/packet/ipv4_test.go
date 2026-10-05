package packet

import (
	"encoding/binary"
	"encoding/hex"
	"errors"
	"net/netip"
	"testing"
)

// The header from Wikipedia's IPv4 checksum example: 115 bytes in all,
// from 192.168.0.1 to 192.168.0.199.
const documented = "45000073000040004011b861c0a80001c0a800c7"

func documentedPacket(t *testing.T) []byte {
	t.Helper()

	header, err := hex.DecodeString(documented)
	if err != nil {
		t.Fatal(err)
	}

	return append(header, make([]byte, 115-len(header))...)
}

func TestADocumentedHeaderParses(t *testing.T) {
	h, err := ParseIPv4(documentedPacket(t))
	if err != nil {
		t.Fatal(err)
	}

	if h.Src != netip.MustParseAddr("192.168.0.1") || h.Dst != netip.MustParseAddr("192.168.0.199") {
		t.Errorf("got %v to %v", h.Src, h.Dst)
	}
}

func TestOptionsDoNotMoveTheAddresses(t *testing.T) {
	p := documentedPacket(t)
	p[0] = 0x46

	h, err := ParseIPv4(p)
	if err != nil || h.Src != netip.MustParseAddr("192.168.0.1") {
		t.Errorf("a 24-byte header: got %v, %v", h.Src, err)
	}
}

func TestIPv6IsNotLetIn(t *testing.T) {
	p := make([]byte, 40)
	p[0] = 0x60

	if _, err := ParseIPv4(p); !errors.Is(err, ErrNotIPv4) {
		t.Errorf("got %v, want ErrNotIPv4", err)
	}
}

func TestAShortPacketIsRefused(t *testing.T) {
	p := documentedPacket(t)

	for n := range 20 {
		if _, err := ParseIPv4(p[:n]); !errors.Is(err, ErrShort) {
			t.Fatalf("%d bytes: got %v, want ErrShort", n, err)
		}
	}
}

func TestALengthThatLiesIsRefused(t *testing.T) {
	p := documentedPacket(t)

	cases := map[string][]byte{
		"one byte missing": p[:len(p)-1],
		"one byte extra":   append(append([]byte{}, p...), 0),
	}

	for name, c := range cases {
		if _, err := ParseIPv4(c); !errors.Is(err, ErrLength) {
			t.Errorf("%s: got %v, want ErrLength", name, err)
		}
	}
}

func TestAHeaderLengthOutOfRangeIsRefused(t *testing.T) {
	tooShort := documentedPacket(t)
	tooShort[0] = 0x44

	tooLong := documentedPacket(t)[:40]
	tooLong[0] = 0x4f
	binary.BigEndian.PutUint16(tooLong[2:4], 40)

	for name, p := range map[string][]byte{"16-byte header": tooShort, "60 bytes of header in 40": tooLong} {
		if _, err := ParseIPv4(p); !errors.Is(err, ErrHeader) {
			t.Errorf("%s: got %v, want ErrHeader", name, err)
		}
	}
}
