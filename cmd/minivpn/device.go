//go:build windows || linux

package main

import "github.com/maksim-miliutin/MiniVPN/internal/adapter"

func openDevice(c adapter.Config) (device, error) {
	a, err := adapter.Open(c)
	if err != nil {
		return nil, err
	}

	return a, nil
}
