# MiniVPN

A small encrypted UDP tunnel for Windows and Linux, written in Go to show how a VPN
works inside: a virtual network adapter hands over IP packets, each packet is sealed
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
- golang.org/x/sys: the Windows and Linux calls around the adapter, and processor
  detection for x/crypto.

On Windows the adapter also needs wintun.dll from https://www.wintun.net
(wintun/bin/amd64 in the zip) next to the executable, and administrator rights;
neither is part of this repository. On Linux it needs root and the ip tool from
iproute2.

## Building

Windows or Linux on amd64, and the Go version named in go.mod.

    go build ./cmd/minivpn

A Linux build from Windows PowerShell, for example for WSL 2. Cgo stays off for it:
MiniVPN has no C in it, and a Windows C compiler cannot build for Linux.

    $env:GOOS = "linux"; $env:CGO_ENABLED = "0"; go build ./cmd/minivpn; Remove-Item Env:GOOS, Env:CGO_ENABLED

## Running

Both ends run with administrator rights (root on Linux), and on Windows with
wintun.dll next to minivpn.exe.

    minivpn genkey                       # writes minivpn.key; copy it to the other end
    minivpn server                       # UDP port 51821, tunnel address 10.9.0.1/24
    minivpn client -server HOST:51821    # tunnel address 10.9.0.2/24

Flags -key, -addr and -peer change the defaults. Ctrl+C stops either end and prints
how many packets went out through the tunnel and came in, how many frames did not
open and how many packets were refused.

Windows Firewall blocks the server's UDP port and inbound traffic inside the tunnel,
ping included. To allow both for a test:

    netsh advfirewall firewall add rule name=MiniVPN dir=in action=allow protocol=UDP localport=51821
    netsh advfirewall firewall add rule name="MiniVPN ping" dir=in action=allow protocol=icmpv4:8,any remoteip=10.9.0.0/24

If Windows once asked about network access for minivpn.exe and the prompt was closed,
it added rules that block the program, and a block beats the rules above:

    Get-NetFirewallApplicationFilter -Program (Resolve-Path .\minivpn.exe).Path | Get-NetFirewallRule | Where-Object Action -eq Block | Remove-NetFirewallRule

One PC is enough when the other end runs in WSL 2, which has a kernel and a network
stack of its own; there the Windows side is the default gateway, as
`ip route show default` prints it. Before trusting a ping from WSL, check that
`ip route get 10.9.0.1` names dev MiniVPN: without the client, WSL still reaches
10.9.0.1 straight through its own network, and Windows answers. Two ends inside one
Windows do not work at all: both tunnel addresses would be local, and traffic
between them would never enter the tunnel.

Checked so far: Windows with Linux in WSL 2 on one PC, and two Linux network
namespaces, both with ping each way and 1400-byte packets passing with fragmentation
forbidden while 1401 bytes are refused. Two separate Windows machines have not been
tried yet.
