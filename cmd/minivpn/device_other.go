//go:build !windows

package main

import (
	"errors"

	"github.com/maksim-miliutin/MiniVPN/internal/adapter"
)

var ErrWindowsOnly = errors.New("minivpn: the adapter runs on Windows only")

func openDevice(adapter.Config) (device, error) {
	return nil, ErrWindowsOnly
}
