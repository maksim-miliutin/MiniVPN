package adapter

import (
	"errors"
	"testing"
)

func TestOpenChecksTheConfigBeforeTouchingTheDriver(t *testing.T) {
	if _, err := Open(Config{}); !errors.Is(err, ErrName) {
		t.Fatalf("got %v, want ErrName before any driver call", err)
	}
}
