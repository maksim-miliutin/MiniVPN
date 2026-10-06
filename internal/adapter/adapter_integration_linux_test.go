//go:build integration

package adapter

import (
	"os"
	"testing"
)

func ready(t *testing.T) {
	t.Helper()

	if os.Geteuid() != 0 {
		t.Fatal("run as root, for example with sudo: only root may create TUN interfaces")
	}
}
