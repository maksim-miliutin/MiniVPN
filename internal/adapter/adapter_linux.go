package adapter

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"golang.org/x/sys/unix"
)

const ifnameMax = unix.IFNAMSIZ - 1 // bytes, the closing zero left out

type Adapter struct {
	file *os.File
}

func Open(c Config) (*Adapter, error) {
	if err := c.check(); err != nil {
		return nil, err
	}

	if len(c.Name) > ifnameMax {
		return nil, ErrName
	}

	fd, err := unix.Open("/dev/net/tun", unix.O_RDWR|unix.O_CLOEXEC, 0)
	if err != nil {
		return nil, fmt.Errorf("adapter: opening /dev/net/tun: %w", err)
	}

	ifr, err := unix.NewIfreq(c.Name)
	if err != nil {
		unix.Close(fd)

		return nil, fmt.Errorf("adapter: %q: %w", c.Name, err)
	}

	// Open, ioctl and nonblock all come before os.NewFile hands the descriptor to
	// the poller; only then does Close wake a blocked Read.
	ifr.SetUint16(unix.IFF_TUN | unix.IFF_NO_PI)
	if err := unix.IoctlIfreq(fd, unix.TUNSETIFF, ifr); err != nil {
		unix.Close(fd)

		return nil, fmt.Errorf("adapter: creating %q: %w", c.Name, err)
	}

	if err := unix.SetNonblock(fd, true); err != nil {
		unix.Close(fd)

		return nil, fmt.Errorf("adapter: %q: %w", c.Name, err)
	}

	file := os.NewFile(uintptr(fd), "/dev/net/tun")
	for _, args := range c.ip() {
		out, err := exec.Command("ip", args...).CombinedOutput()
		if err != nil {
			file.Close()

			return nil, fmt.Errorf("adapter: ip %s: %w: %s", strings.Join(args, " "), err, bytes.TrimSpace(out))
		}
	}

	return &Adapter{file: file}, nil
}

func (a *Adapter) Read(buf []byte) (int, error) {
	n, err := a.file.Read(buf)
	if err != nil {
		return 0, fmt.Errorf("adapter: receiving: %w", err)
	}

	return n, nil
}

func (a *Adapter) Write(packet []byte) error {
	if _, err := a.file.Write(packet); err != nil {
		return fmt.Errorf("adapter: sending: %w", err)
	}

	return nil
}

func (a *Adapter) Close() error {
	return a.file.Close()
}
