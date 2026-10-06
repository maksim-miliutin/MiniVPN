# MiniVPN

A small encrypted UDP tunnel for Windows, written in Go to show how a VPN works
inside: a virtual network adapter hands over IP packets, each packet is sealed
into an encrypted frame, and the frames travel between two machines over UDP.

This is a learning project, not a replacement for a real VPN; for anything that
matters, use WireGuard. One server talks to one client, and only the tunnel's own
subnet goes through it.

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

## Running

Both ends run from an administrator window, with wintun.dll next to minivpn.exe.

    minivpn genkey                       # writes minivpn.key; copy it to the other end
    minivpn server                       # UDP port 51821, tunnel address 10.9.0.1/24
    minivpn client -server HOST:51821    # tunnel address 10.9.0.2/24

Flags -key, -addr and -peer change the defaults. Ctrl+C stops either end and prints
how many frames did not open and how many packets were refused.

Windows Firewall blocks the server's UDP port and inbound traffic inside the tunnel,
ping included. To allow both for a test:

    netsh advfirewall firewall add rule name=MiniVPN dir=in action=allow protocol=UDP localport=51821
    netsh advfirewall firewall add rule name="MiniVPN ping" dir=in action=allow protocol=icmpv4:8,any remoteip=10.9.0.0/24
