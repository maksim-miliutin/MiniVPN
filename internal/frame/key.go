package frame

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
)

const keySize = 32

var (
	ErrKeyText = errors.New("frame: the key is not base64")
	ErrKeySize = errors.New("frame: the key is the wrong length")
)

type Key struct {
	bytes [keySize]byte
}

func NewKey() Key {
	var k Key
	rand.Read(k.bytes[:])

	return k
}

func ParseKey(text string) (Key, error) {
	raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(text))
	if err != nil {
		return Key{}, fmt.Errorf("%w: %w", ErrKeyText, err)
	}

	if len(raw) != keySize {
		return Key{}, ErrKeySize
	}

	var k Key
	copy(k.bytes[:], raw)

	return k, nil
}

func (k Key) Text() string {
	return base64.StdEncoding.EncodeToString(k.bytes[:])
}

func (Key) Format(f fmt.State, _ rune) {
	fmt.Fprint(f, "frame.Key(hidden)")
}
