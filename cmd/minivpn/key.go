package main

import (
	"fmt"
	"io"
	"os"

	"github.com/maksim-miliutin/MiniVPN/internal/frame"
)

func genkey(path string, create func(string) (io.WriteCloser, error)) error {
	f, err := create(path)
	if err != nil {
		return fmt.Errorf("minivpn: %s: %w", path, err)
	}

	if _, err := io.WriteString(f, frame.NewKey().Text()+"\n"); err != nil {
		f.Close()

		return fmt.Errorf("minivpn: writing %s: %w", path, err)
	}

	if err := f.Close(); err != nil {
		return fmt.Errorf("minivpn: closing %s: %w", path, err)
	}

	return nil
}

func createKeyFile(path string) (io.WriteCloser, error) {
	return os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
}

func readKey(path string, read func(string) ([]byte, error)) (frame.Key, error) {
	text, err := read(path)
	if err != nil {
		return frame.Key{}, fmt.Errorf("minivpn: reading the key: %w", err)
	}

	k, err := frame.ParseKey(string(text))
	if err != nil {
		return frame.Key{}, fmt.Errorf("minivpn: %s: %w", path, err)
	}

	return k, nil
}
