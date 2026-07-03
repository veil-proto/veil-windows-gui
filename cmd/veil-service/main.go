//go:build windows

// Command veil-service is the VEIL Windows tunnel service. It runs as a Windows
// service (started by the Service Control Manager), holds the tunnel data plane,
// and exposes the control channel (\\.\pipe\veil-service) that veil-tray and any
// CLI drive. Running as a service is what gives the client auto-start at boot,
// SCM auto-restart on crash, and a tunnel that survives user logoff.
//
//	veil-service install     # register + start the service (run elevated)
//	veil-service uninstall   # stop + remove the service (run elevated)
//	veil-service run         # run in the foreground for debugging
//
// With no argument it expects to be launched by the SCM.
package main

import (
	"fmt"
	"log"
	"os"

	"golang.org/x/sys/windows/svc"
)

const (
	serviceName    = "VEILTunnel"
	serviceDisplay = "VEIL Tunnel Service"
	serviceDesc    = "VEIL VPN tunnel data plane and local control channel."
)

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "install":
			if err := installService(); err != nil {
				log.Fatalf("install: %v", err)
			}
			fmt.Printf("%s installed and started\n", serviceName)
			return
		case "uninstall":
			if err := removeService(); err != nil {
				log.Fatalf("uninstall: %v", err)
			}
			fmt.Printf("%s removed\n", serviceName)
			return
		case "run":
			runConsole()
			return
		default:
			log.Fatalf("usage: veil-service [install|uninstall|run]")
		}
	}

	isSvc, err := svc.IsWindowsService()
	if err != nil {
		log.Fatalf("could not determine service context: %v", err)
	}
	if !isSvc {
		log.Fatalf("veil-service must be started by the Service Control Manager; use: veil-service [install|uninstall|run]")
	}
	runService()
}
