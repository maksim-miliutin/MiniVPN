package frame

import (
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"fmt"
	"slices"

	"golang.org/x/crypto/chacha20poly1305"
)

const (
	nonceSize = chacha20poly1305.NonceSizeX
	Overhead  = nonceSize + chacha20poly1305.Overhead
)

var (
	ErrShort        = errors.New("frame: shorter than a nonce and a tag")
	ErrNotAuthentic = errors.New("frame: not authentic for this key and side")
)

type Box struct {
	aead cipher.AEAD

	// Each side seals with its own byte as additional data and opens only the
	// other's, so a frame bounced back to its sender stays shut.
	mine   []byte
	theirs []byte
}

func ClientBox(k Key) *Box {
	return newBox(k, 'c', 's')
}

func ServerBox(k Key) *Box {
	return newBox(k, 's', 'c')
}

func newBox(k Key, mine, theirs byte) *Box {
	aead, err := chacha20poly1305.NewX(k.bytes[:])
	if err != nil {
		panic(err)
	}

	return &Box{aead: aead, mine: []byte{mine}, theirs: []byte{theirs}}
}

func (b *Box) Seal(dst, packet []byte) []byte {
	dst = slices.Grow(dst, Overhead+len(packet))
	nonce := dst[len(dst) : len(dst)+nonceSize]
	rand.Read(nonce)

	return b.aead.Seal(dst[:len(dst)+nonceSize], nonce, packet, b.mine)
}

func (b *Box) Open(dst, frame []byte) ([]byte, error) {
	if len(frame) < Overhead {
		return nil, ErrShort
	}

	packet, err := b.aead.Open(dst, frame[:nonceSize], frame[nonceSize:], b.theirs)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrNotAuthentic, err)
	}

	return packet, nil
}
