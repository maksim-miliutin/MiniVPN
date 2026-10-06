package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"

	"github.com/maksim-miliutin/MiniVPN/internal/link"
)

func main() {
	c, err := parse(os.Args[1:])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}

	if err := do(c); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func do(c command) error {
	if c.name == "genkey" {
		if err := genkey(c.key, createKeyFile); err != nil {
			return err
		}

		fmt.Printf("minivpn: wrote %s; carry it to the other end yourself\n", c.key)

		return nil
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	return run(ctx, c, deps{listen: listenUDP, open: openDevice, read: os.ReadFile, out: os.Stdout})
}

func listenUDP(address string) (conn, error) {
	u, err := link.Listen(address)
	if err != nil {
		return nil, err
	}

	return u, nil
}
