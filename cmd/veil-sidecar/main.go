//go:build windows

// Command veil-sidecar is the VEIL tunnel backend for the Tauri desktop
// client. It holds the tunnel data plane (via wintunnel.Tunnel) and speaks
// the control protocol (github.com/veil-proto/veil-windows/control) over its
// own stdin/stdout.
//
// There is deliberately no Windows Service and no named pipe here: the Tauri
// Rust shell spawns this binary directly as a sidecar process and pipes
// control-protocol JSON lines to its stdin/stdout. The tunnel lives only as
// long as this process does — closing the app tears the tunnel down, same
// as most consumer VPN clients. Bringing up a TUN adapter and changing
// routes needs Administrator, so the parent app (and therefore this process)
// must be launched elevated; there is no separate elevation step here.
package main

import (
	"log"
	"os"
)

func main() {
	h := newHandler()
	autoReconnect(h)

	log.Printf("veil-sidecar: serving control protocol on stdio")
	if err := serveControlIO(h, os.Stdin, os.Stdout); err != nil {
		log.Printf("control protocol failed: %v", err)
	}

	// serveControlIO returns once stdin closes (the parent process exited or closed
	// the pipe) — tear the tunnel down instead of leaving it running orphaned.
	if err := h.Disconnect(); err != nil {
		log.Printf("shutdown disconnect: %v", err)
	}
}
