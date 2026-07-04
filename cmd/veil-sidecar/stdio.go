package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"io"

	"github.com/veil-proto/veil-windows/control"
)

type sidecarHandler interface {
	control.Handler
	ParseConfig(configText string) (ParsedConfig, error)
	SerializeConfig(pc ParsedConfig) (string, error)
}

func serveControlIO(h sidecarHandler, in io.Reader, out io.Writer) error {
	r := bufio.NewReader(in)
	enc := json.NewEncoder(out)
	for {
		line, err := r.ReadBytes('\n')
		if len(line) > 0 {
			resp := dispatch(h, line)
			if encErr := enc.Encode(resp); encErr != nil {
				return encErr
			}
		}
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return err
		}
	}
}

func dispatch(h sidecarHandler, line []byte) response {
	var req request
	if err := json.Unmarshal(line, &req); err != nil {
		return response{Error: err.Error()}
	}

	switch req.Cmd {
	case control.CmdStatus:
		st := h.Status()
		return response{OK: true, Status: &st}
	case control.CmdConnect:
		if err := h.Connect(req.Config, req.Name); err != nil {
			return response{Error: err.Error()}
		}
		st := h.Status()
		return response{OK: true, Status: &st}
	case control.CmdDisconnect:
		if err := h.Disconnect(); err != nil {
			return response{Error: err.Error()}
		}
		st := h.Status()
		return response{OK: true, Status: &st}
	case control.CmdLogs:
		logs := h.Logs(req.Since)
		st := h.Status()
		return response{OK: true, Status: &st, Logs: logs}
	case cmdParseConfig:
		pc, err := h.ParseConfig(req.Config)
		if err != nil {
			return response{Error: err.Error()}
		}
		return response{OK: true, ParsedConfig: &pc}
	case cmdSerializeConfig:
		if req.ParsedConfig == nil {
			return response{Error: "serializeConfig: missing parsedConfig"}
		}
		cfg, err := h.SerializeConfig(*req.ParsedConfig)
		if err != nil {
			return response{Error: err.Error()}
		}
		return response{OK: true, Config: cfg}
	default:
		return response{Error: "unknown command: " + req.Cmd}
	}
}
