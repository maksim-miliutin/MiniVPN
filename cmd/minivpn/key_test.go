package main

import (
	"bytes"
	"errors"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/maksim-miliutin/MiniVPN/internal/frame"
)

type keyFile struct {
	bytes.Buffer
	closed bool
}

func (f *keyFile) Close() error {
	f.closed = true

	return nil
}

func into(f *keyFile) func(string) (io.WriteCloser, error) {
	return func(string) (io.WriteCloser, error) { return f, nil }
}

func from(text string) func(string) ([]byte, error) {
	return func(string) ([]byte, error) { return []byte(text), nil }
}

func TestGenkeyWritesAKeyThatReadsBack(t *testing.T) {
	f := &keyFile{}
	if err := genkey("minivpn.key", into(f)); err != nil {
		t.Fatal(err)
	}

	if !f.closed || !strings.HasSuffix(f.String(), "\n") {
		t.Errorf("closed %v, text %q: want a closed file ending in a newline", f.closed, f.String())
	}

	if _, err := readKey("minivpn.key", from(f.String())); err != nil {
		t.Errorf("the written key does not read back: %v", err)
	}
}

func TestTwoGenkeysWriteDifferentKeys(t *testing.T) {
	a, b := &keyFile{}, &keyFile{}
	genkey("a.key", into(a))
	genkey("b.key", into(b))

	if a.String() == b.String() {
		t.Error("two runs wrote the same key")
	}
}

func TestGenkeyNeverOverwrites(t *testing.T) {
	exists := func(string) (io.WriteCloser, error) { return nil, os.ErrExist }

	err := genkey("minivpn.key", exists)
	if !errors.Is(err, os.ErrExist) || !strings.Contains(err.Error(), "minivpn.key") {
		t.Errorf("got %v, want os.ErrExist naming the file", err)
	}
}

func TestReadKeyNamesTheFileButNotTheText(t *testing.T) {
	secret := frame.NewKey().Text()
	broken := secret[:40] + "!!!!"

	_, err := readKey("minivpn.key", from(broken))
	if !errors.Is(err, frame.ErrKeyText) || !strings.Contains(err.Error(), "minivpn.key") {
		t.Fatalf("got %v, want frame.ErrKeyText naming the file", err)
	}

	if strings.Contains(err.Error(), secret[:8]) {
		t.Errorf("the error repeats the key text: %v", err)
	}
}

func TestAMissingKeyFileIsReported(t *testing.T) {
	missing := func(string) ([]byte, error) { return nil, os.ErrNotExist }

	if _, err := readKey("minivpn.key", missing); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("got %v, want os.ErrNotExist", err)
	}
}
