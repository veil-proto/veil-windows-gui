//go:build windows

package main

import (
	"fmt"
	"log"
	"os"
	"time"

	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/mgr"
)

// installService registers veil-service with the SCM as an auto-start service
// and configures SCM to restart it on crash, then starts it. Run elevated.
func installService() error {
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("locate executable: %w", err)
	}

	m, err := mgr.Connect()
	if err != nil {
		return fmt.Errorf("connect to SCM (are you elevated?): %w", err)
	}
	defer m.Disconnect()

	if s, err := m.OpenService(serviceName); err == nil {
		s.Close()
		return fmt.Errorf("%s is already installed", serviceName)
	}

	s, err := m.CreateService(serviceName, exe, mgr.Config{
		DisplayName:  serviceDisplay,
		Description:  serviceDesc,
		StartType:    mgr.StartAutomatic,
		ErrorControl: mgr.ErrorNormal,
	})
	if err != nil {
		return fmt.Errorf("create service: %w", err)
	}
	defer s.Close()

	// SCM auto-restart on crash: restart after 5s each time; reset the failure
	// counter after a day of health. This is the Windows-side answer to the
	// server's systemd Restart=on-failure (roadmap Phase K, process crash).
	recovery := []mgr.RecoveryAction{
		{Type: mgr.ServiceRestart, Delay: 5 * time.Second},
		{Type: mgr.ServiceRestart, Delay: 5 * time.Second},
		{Type: mgr.ServiceRestart, Delay: 5 * time.Second},
	}
	if err := s.SetRecoveryActions(recovery, uint32((24 * time.Hour).Seconds())); err != nil {
		log.Printf("warning: set recovery actions: %v", err)
	}

	if err := s.Start(); err != nil {
		return fmt.Errorf("start service: %w", err)
	}
	return nil
}

// removeService stops (best-effort) and deletes the service. Run elevated.
func removeService() error {
	m, err := mgr.Connect()
	if err != nil {
		return fmt.Errorf("connect to SCM (are you elevated?): %w", err)
	}
	defer m.Disconnect()

	s, err := m.OpenService(serviceName)
	if err != nil {
		return fmt.Errorf("%s is not installed", serviceName)
	}
	defer s.Close()

	if _, err := s.Control(svc.Stop); err != nil {
		log.Printf("warning: stop service: %v", err)
	} else {
		// Give the tunnel a moment to tear down cleanly before deletion.
		time.Sleep(time.Second)
	}
	if err := s.Delete(); err != nil {
		return fmt.Errorf("delete service: %w", err)
	}
	return nil
}
