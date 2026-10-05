package adapter

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"sync/atomic"

	"golang.org/x/sys/windows"
	"golang.zx2c4.com/wintun"
)

const ringCapacity = 8 << 20

// One GUID for every run, or Windows files a new network, firewall profile and all,
// each time the adapter appears.
var guid = &windows.GUID{Data1: 0x725c8750, Data2: 0x656c, Data3: 0x41ed, Data4: [8]byte{0x86, 0xac, 0x94, 0xcc, 0x54, 0xa6, 0xc8, 0x8e}}

type Adapter struct {
	adapter  *wintun.Adapter
	session  wintun.Session
	readWait windows.Handle
	closed   atomic.Bool
	running  sync.WaitGroup
	once     sync.Once
}

func Open(c Config) (*Adapter, error) {
	if err := c.check(); err != nil {
		return nil, err
	}

	ad, err := wintun.CreateAdapter(c.Name, "MiniVPN", guid)
	if err != nil {
		return nil, fmt.Errorf("adapter: creating %q: %w", c.Name, err)
	}

	for _, args := range c.netsh() {
		out, err := exec.Command("netsh", args...).CombinedOutput()
		if err != nil {
			ad.Close()

			return nil, fmt.Errorf("adapter: netsh %s: %w: %s", strings.Join(args, " "), err, bytes.TrimSpace(out))
		}
	}

	session, err := ad.StartSession(ringCapacity)
	if err != nil {
		ad.Close()

		return nil, fmt.Errorf("adapter: starting a session: %w", err)
	}

	return &Adapter{adapter: ad, session: session, readWait: session.ReadWaitEvent()}, nil
}

func (a *Adapter) Read(buf []byte) (int, error) {
	a.running.Add(1)
	defer a.running.Done()

	for !a.closed.Load() {
		packet, err := a.session.ReceivePacket()
		switch {
		case err == nil:
			// The packet lives in the driver's ring only until released: copy it first.
			n := copy(buf, packet)
			a.session.ReleaseReceivePacket(packet)

			return n, nil
		case errors.Is(err, windows.ERROR_NO_MORE_ITEMS):
			windows.WaitForSingleObject(a.readWait, windows.INFINITE)
		case errors.Is(err, windows.ERROR_HANDLE_EOF):
			return 0, os.ErrClosed
		default:
			return 0, fmt.Errorf("adapter: receiving: %w", err)
		}
	}

	return 0, os.ErrClosed
}

func (a *Adapter) Write(packet []byte) error {
	a.running.Add(1)
	defer a.running.Done()

	if a.closed.Load() {
		return os.ErrClosed
	}

	buf, err := a.session.AllocateSendPacket(len(packet))
	switch {
	case err == nil:
		copy(buf, packet)
		a.session.SendPacket(buf)

		return nil
	case errors.Is(err, windows.ERROR_BUFFER_OVERFLOW):
		// A full ring drops the packet, as a busy router would; the loop goes on.
		return nil
	case errors.Is(err, windows.ERROR_HANDLE_EOF):
		return os.ErrClosed
	default:
		return fmt.Errorf("adapter: sending: %w", err)
	}
}

func (a *Adapter) Close() error {
	var err error

	a.once.Do(func() {
		// Ending the session frees the ring a reader may wait on: wake it, wait it out.
		a.closed.Store(true)
		windows.SetEvent(a.readWait)
		a.running.Wait()
		a.session.End()
		err = a.adapter.Close()
	})

	return err
}
