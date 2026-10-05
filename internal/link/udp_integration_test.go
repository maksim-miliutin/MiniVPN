//go:build integration

package link

import (
	"bytes"
	"net/netip"
	"testing"
	"time"

	"github.com/maksim-miliutin/MiniVPN/internal/frame"
)

const largestIPv4Payload = 65535 - 20 - 8 // the IPv4 length field counts its own header in

func listen(t *testing.T) *UDP {
	t.Helper()

	conn, err := Listen("127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() { conn.Close() })

	return conn
}

func within(t *testing.T, l *Link) []byte {
	t.Helper()

	type result struct {
		packet []byte
		err    error
	}

	done := make(chan result, 1)
	go func() {
		packet, err := l.Receive(nil)
		done <- result{packet, err}
	}()

	select {
	case r := <-done:
		if r.err != nil {
			t.Fatal(r.err)
		}

		return r.packet
	case <-time.After(5 * time.Second):
		t.Fatal("nothing arrived in 5 seconds")
		return nil
	}
}

func loopbackPair(t *testing.T) (client, server *Link) {
	k := frame.NewKey()
	serverConn, clientConn := listen(t), listen(t)
	server = New(serverConn, frame.ServerBox(k), netip.AddrPort{})
	client = New(clientConn, frame.ClientBox(k), serverConn.LocalAddr())

	return client, server
}

func TestTwoLinksTalkOverLoopbackUDP(t *testing.T) {
	client, server := loopbackPair(t)

	send(t, client, "ping")
	if got := within(t, server); string(got) != "ping" {
		t.Fatalf("the server got %q", got)
	}

	send(t, server, "pong")
	if got := within(t, client); string(got) != "pong" {
		t.Fatalf("the client got %q", got)
	}
}

func TestTheLargestIPv4DatagramCrossesLoopback(t *testing.T) {
	client, server := loopbackPair(t)
	big := bytes.Repeat([]byte{0xA5}, largestIPv4Payload-frame.Overhead)

	if err := client.Send(big); err != nil {
		t.Fatal(err)
	}

	if got := within(t, server); !bytes.Equal(got, big) {
		t.Errorf("got %d bytes of %d", len(got), len(big))
	}
}
