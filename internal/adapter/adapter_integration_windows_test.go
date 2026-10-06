//go:build integration

package adapter

import (
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/sys/windows"
)

func ready(t *testing.T) {
	t.Helper()

	if !windows.GetCurrentProcessToken().IsElevated() {
		t.Fatal("run from an administrator window: only administrators may create adapters")
	}

	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(filepath.Join(filepath.Dir(exe), "wintun.dll")); err != nil {
		t.Fatal("put wintun.dll next to the test binary: wintun looks there and in System32 only")
	}
}
