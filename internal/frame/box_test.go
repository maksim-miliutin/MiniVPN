package frame

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"testing"

	"golang.org/x/crypto/chacha20poly1305"
)

func randomPacket(t *testing.T, size int) []byte {
	t.Helper()

	p := make([]byte, size)
	rand.Read(p)

	return p
}

func TestAPacketComesBackTheSame(t *testing.T) {
	k := NewKey()

	for _, size := range []int{0, 1, 20, 1400, 65535} {
		p := randomPacket(t, size)

		got, err := ServerBox(k).Open(nil, ClientBox(k).Seal(nil, p))
		if err != nil || !bytes.Equal(got, p) {
			t.Errorf("client to server, %d bytes: err %v", size, err)
		}

		got, err = ClientBox(k).Open(nil, ServerBox(k).Seal(nil, p))
		if err != nil || !bytes.Equal(got, p) {
			t.Errorf("server to client, %d bytes: err %v", size, err)
		}
	}
}

func TestAnyDamagedByteKeepsTheFrameShut(t *testing.T) {
	k := NewKey()
	frame := ClientBox(k).Seal(nil, randomPacket(t, 100))

	for i := range frame {
		damaged := bytes.Clone(frame)
		damaged[i] ^= 0x01

		if _, err := ServerBox(k).Open(nil, damaged); !errors.Is(err, ErrNotAuthentic) {
			t.Fatalf("byte %d flipped: got %v", i, err)
		}
	}
}

func TestAnotherKeyDoesNotOpen(t *testing.T) {
	frame := ClientBox(NewKey()).Seal(nil, randomPacket(t, 100))

	if _, err := ServerBox(NewKey()).Open(nil, frame); !errors.Is(err, ErrNotAuthentic) {
		t.Errorf("got %v", err)
	}
}

func TestACutFrameDoesNotOpen(t *testing.T) {
	k := NewKey()
	frame := ClientBox(k).Seal(nil, randomPacket(t, 100))

	for n := range len(frame) {
		want := ErrNotAuthentic
		if n < Overhead {
			want = ErrShort
		}

		if _, err := ServerBox(k).Open(nil, frame[:n]); !errors.Is(err, want) {
			t.Fatalf("cut to %d bytes: got %v, want %v", n, err, want)
		}
	}
}

func TestTwoSealsOfOnePacketDiffer(t *testing.T) {
	box := ClientBox(NewKey())
	p := randomPacket(t, 100)

	if bytes.Equal(box.Seal(nil, p), box.Seal(nil, p)) {
		t.Error("two frames of one packet are equal: the nonce repeats")
	}
}

func TestAFrameDoesNotOpenForItsSender(t *testing.T) {
	k := NewKey()

	for name, box := range map[string]*Box{"client": ClientBox(k), "server": ServerBox(k)} {
		if _, err := box.Open(nil, box.Seal(nil, randomPacket(t, 100))); !errors.Is(err, ErrNotAuthentic) {
			t.Errorf("the %s opened its own frame: got %v", name, err)
		}
	}
}

func TestTheFrameIsANonceThenTheSealedPacket(t *testing.T) {
	k := NewKey()
	p := randomPacket(t, 100)
	frame := ClientBox(k).Seal(nil, p)

	raw, err := base64.StdEncoding.DecodeString(k.Text())
	if err != nil {
		t.Fatal(err)
	}

	aead, err := chacha20poly1305.NewX(raw)
	if err != nil {
		t.Fatal(err)
	}

	n := chacha20poly1305.NonceSizeX
	got, err := aead.Open(nil, frame[:n], frame[n:], []byte("c"))
	if err != nil || !bytes.Equal(got, p) {
		t.Errorf("the library alone does not open a client frame: %v", err)
	}
}

func TestSealAppendsAndLeavesThePacketAlone(t *testing.T) {
	k := NewKey()
	p := randomPacket(t, 100)
	keep := bytes.Clone(p)

	out := ClientBox(k).Seal([]byte("head"), p)
	if !bytes.Equal(p, keep) {
		t.Error("Seal changed the packet")
	}

	if string(out[:4]) != "head" || len(out) != 4+len(p)+Overhead {
		t.Fatalf("Seal did not append one frame after dst: %d bytes", len(out))
	}

	got, err := ServerBox(k).Open([]byte("head"), out[4:])
	if err != nil || string(got[:4]) != "head" || !bytes.Equal(got[4:], p) {
		t.Errorf("Open did not append the packet after dst: %v", err)
	}
}
