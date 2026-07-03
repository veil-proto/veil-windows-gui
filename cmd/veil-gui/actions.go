//go:build windows

package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/veil-proto/veil-windows/control"
)

func client() (*control.Client, error) {
	conn, err := control.Dial(800 * time.Millisecond)
	if err != nil {
		return nil, err
	}
	return control.NewClient(conn), nil
}

func status() (control.Response, error) {
	c, err := client()
	if err != nil {
		return control.Response{}, err
	}
	defer c.Close()
	return c.Status()
}

func connect(configText, name string) (control.Response, error) {
	c, err := client()
	if err != nil {
		return control.Response{}, err
	}
	defer c.Close()
	return c.Connect(configText, name)
}

func disconnect() (control.Response, error) {
	c, err := client()
	if err != nil {
		return control.Response{}, err
	}
	defer c.Close()
	return c.Disconnect()
}

// fetchLogs pulls every log line the service has captured since the given
// cursor. Errors (e.g. service unavailable) are treated as "no new logs" by
// the caller rather than surfaced as a connection-status error, since the
// Logs tab is secondary to the main connection status already shown by
// status().
func fetchLogs(since uint64) ([]control.LogLine, error) {
	c, err := client()
	if err != nil {
		return nil, err
	}
	defer c.Close()
	return c.Logs(since)
}

func human(n uint64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	v := float64(n)
	for _, suffix := range []string{"KiB", "MiB", "GiB", "TiB"} {
		v /= unit
		if v < unit {
			return fmt.Sprintf("%.1f %s", v, suffix)
		}
	}
	return fmt.Sprintf("%.1f PiB", v/unit)
}

func ago(unix int64) string {
	if unix <= 0 {
		return "never"
	}
	d := time.Since(time.Unix(unix, 0))
	if d < 0 {
		d = 0
	}
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%ds ago", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd ago", int(d.Hours()/24))
	}
}

func titleFromLink(s string) string {
	if i := strings.LastIndexByte(s, '#'); i >= 0 && i+1 < len(s) {
		return s[i+1:]
	}
	return ""
}
