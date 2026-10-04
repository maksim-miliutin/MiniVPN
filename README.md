# MiniVPN

A small encrypted UDP tunnel for Windows, written in Go to show how a VPN works
inside: a virtual network adapter hands over IP packets, each packet is sealed
into an encrypted frame, and the frames travel between two machines over UDP.

This is a learning project, not a replacement for a real VPN; for anything that
matters, use WireGuard. One server talks to one client, and only the tunnel's own
subnet goes through it.

It does not run yet: so far the repository holds only the skeleton.

## Building

Windows on amd64 and the Go version named in go.mod.

    go build ./cmd/minivpn
