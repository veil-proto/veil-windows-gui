//go:build windows

package main

import (
	"log"
	"os"
	"os/signal"

	"golang.org/x/sys/windows/svc"

	"github.com/veil-proto/veil-windows/control"
)

// veilService is the SCM handler. It stands up the control channel, restores any
// previously-active tunnel, and tears everything down on stop.
type veilService struct {
	handler control.Handler
}

func (s *veilService) Execute(_ []string, r <-chan svc.ChangeRequest, changes chan<- svc.Status) (bool, uint32) {
	changes <- svc.Status{State: svc.StartPending}

	l, err := control.Listen()
	if err != nil {
		log.Printf("control listen: %v", err)
		return true, 1
	}
	go (&control.Server{Handler: s.handler}).Serve(l)

	autoReconnect(s.handler)

	changes <- svc.Status{State: svc.Running, Accepts: svc.AcceptStop | svc.AcceptShutdown}
	for c := range r {
		switch c.Cmd {
		case svc.Interrogate:
			changes <- c.CurrentStatus
		case svc.Stop, svc.Shutdown:
			changes <- svc.Status{State: svc.StopPending}
			s.handler.Disconnect()
			l.Close()
			return false, 0
		}
	}
	return false, 0
}

func runService() {
	if err := svc.Run(serviceName, &veilService{handler: newHandler()}); err != nil {
		log.Fatalf("service failed: %v", err)
	}
}

// runConsole runs the same control server in the foreground, for debugging
// without installing the service. Requires the same privileges (adapter
// creation, routing) as the service, so run it from an elevated shell.
func runConsole() {
	h := newHandler()
	l, err := control.Listen()
	if err != nil {
		log.Fatalf("control listen: %v", err)
	}
	defer l.Close()
	go (&control.Server{Handler: h}).Serve(l)
	autoReconnect(h)

	log.Printf("veil-service running in console mode on %s; Ctrl+C to stop", control.PipeName)
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt)
	<-sigCh
	h.Disconnect()
}
