# MiniVPN

A small encrypted UDP tunnel for Windows, written in Go to show how a VPN works
inside: a virtual network adapter hands over IP packets, each packet is sealed
into an encrypted frame, and the frames travel between two machines over UDP.

This is a learning project, not a replacement for a real VPN; for anything that
matters, use WireGuard. One server talks to one client, and only the tunnel's own
subnet goes through it.

It does not run yet.

## What it does not have

Both ends share one pre-shared key, so there is no protection against replayed
frames, no forward secrecy and no key rotation; a handshake would bring them.

## Dependencies

- golang.org/x/crypto: XChaCha20-Poly1305, the cipher that seals every frame.
- golang.zx2c4.com/wintun: Go bindings for Wintun, the virtual network adapter.
- golang.org/x/sys: the Windows calls around the adapter, and processor detection
  for x/crypto.

The adapter also needs wintun.dll from https://www.wintun.net (wintun/bin/amd64 in
the zip) next to the executable, and administrator rights; neither is part of this
repository.

## Building

Windows on amd64 and the Go version named in go.mod.

    go build ./cmd/minivpn
