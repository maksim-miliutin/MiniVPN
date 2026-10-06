package adapter

import (
	"errors"
	"strings"
	"testing"
)

func TestOpenChecksTheConfigBeforeTouchingTheDevice(t *testing.T) {
	long := valid()
	long.Name = strings.Repeat("n", 16)

	for name, c := range map[string]Config{"empty": {}, "16 bytes, over IFNAMSIZ": long} {
		if _, err := Open(c); !errors.Is(err, ErrName) {
			t.Errorf("%s: got %v, want ErrName before /dev/net/tun is opened", name, err)
		}
	}
}
