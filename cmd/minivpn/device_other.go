//go:build !windows && !linux

package main

import (
	"errors"

	"github.com/maksim-miliutin/MiniVPN/internal/adapter"
)

var ErrUnsupported = errors.New("minivpn: the adapter runs on Windows and Linux only")

func openDevice(adapter.Config) (device, error) {
	return nil, ErrUnsupported
}
