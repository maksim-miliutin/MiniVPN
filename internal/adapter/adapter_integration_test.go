//go:build integration && (windows || linux)

package adapter

import (
	"bytes"
	"encoding/binary"
	"errors"
	"net"
	"net/netip"
	"os"
	"testing"
	"time"

	"github.com/maksim-miliutin/MiniVPN/internal/packet"
)

var (
	testLocal = netip.MustParseAddr("10.99.0.2")
	testPeer  = netip.AddrPortFrom(netip.MustParseAddr("10.99.0.1"), 9)
)

func checksum(b []byte) uint16 {
	var s uint32
	for i := 0; i < len(b); i += 2 {
		w := uint32(b[i]) << 8
		if i+1 < len(b) {
			w |= uint32(b[i+1])
		}
		s += w
	}

	for s>>16 != 0 {
		s = s&0xffff + s>>16
	}

	return ^uint16(s)
}

func udp(src, dst netip.AddrPort, payload []byte) []byte {
	p := make([]byte, 28+len(payload))
	p[0], p[8], p[9] = 0x45, 64, 17
	binary.BigEndian.PutUint16(p[2:4], uint16(len(p)))
	copy(p[12:16], src.Addr().AsSlice())
	copy(p[16:20], dst.Addr().AsSlice())
	binary.BigEndian.PutUint16(p[10:12], checksum(p[:20]))

	binary.BigEndian.PutUint16(p[20:22], src.Port())
	binary.BigEndian.PutUint16(p[22:24], dst.Port())
	binary.BigEndian.PutUint16(p[24:26], uint16(8+len(payload)))
	copy(p[28:], payload)

	pseudo := append(append([]byte{}, p[12:20]...), 0, 17, p[24], p[25])
	sum := checksum(append(pseudo, p[20:]...))
	if sum == 0 {
		sum = 0xffff
	}
	binary.BigEndian.PutUint16(p[26:28], sum)

	return p
}

func dial(t *testing.T) *net.UDPConn {
	t.Helper()

	local := net.UDPAddrFromAddrPort(netip.AddrPortFrom(testLocal, 0))
	for range 50 {
		conn, err := net.DialUDP("udp4", local, net.UDPAddrFromAddrPort(testPeer))
		if err == nil {
			return conn
		}

		time.Sleep(100 * time.Millisecond)
	}

	t.Fatal("Windows did not take 10.99.0.2 as a source address within 5 seconds")
	return nil
}

func TestAPacketGoesOutAndItsReplyComesBack(t *testing.T) {
	ready(t)

	a, err := Open(Config{Name: "MiniVPNTest", Prefix: netip.PrefixFrom(testLocal, 24), MTU: 1400})
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()

	conn := dial(t)
	defer conn.Close()

	found := make(chan []byte, 1)
	go func() {
		buf := make([]byte, 65535)
		for {
			n, err := a.Read(buf)
			if err != nil {
				return
			}

			h, err := packet.ParseIPv4(buf[:n])
			if err == nil && h.Dst == testPeer.Addr() && buf[9] == 17 && bytes.HasSuffix(buf[:n], []byte("ping")) {
				found <- bytes.Clone(buf[:n])
				return
			}
		}
	}()

	var out []byte
	deadline := time.After(10 * time.Second)
	for out == nil {
		conn.Write([]byte("ping"))

		select {
		case out = <-found:
		case <-time.After(200 * time.Millisecond):
		case <-deadline:
			t.Fatal("no ping came out of the adapter in 10 seconds")
		}
	}

	if h, _ := packet.ParseIPv4(out); h.Src != testLocal {
		t.Errorf("the ping left from %v, want %v", h.Src, testLocal)
	}

	local := conn.LocalAddr().(*net.UDPAddr).AddrPort()
	if err := a.Write(udp(testPeer, local, []byte("pong"))); err != nil {
		t.Fatal(err)
	}

	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	got := make([]byte, 16)
	n, err := conn.Read(got)
	if err != nil || string(got[:n]) != "pong" {
		t.Fatalf("the reply written into the adapter did not reach the socket: %q, %v", got[:n], err)
	}
}

func TestCloseWakesABlockedRead(t *testing.T) {
	ready(t)

	a, err := Open(Config{Name: "MiniVPNTest", Prefix: netip.PrefixFrom(testLocal, 24), MTU: 1400})
	if err != nil {
		t.Fatal(err)
	}

	stopped := make(chan error, 1)
	go func() {
		buf := make([]byte, 65535)
		for {
			if _, err := a.Read(buf); err != nil {
				stopped <- err
				return
			}
		}
	}()

	time.Sleep(200 * time.Millisecond)
	a.Close()

	select {
	case err := <-stopped:
		if !errors.Is(err, os.ErrClosed) {
			t.Errorf("Read ended with %v, want os.ErrClosed", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Close left Read blocked")
	}
}
