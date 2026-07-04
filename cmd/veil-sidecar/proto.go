package main

import "github.com/veil-proto/veil-windows/control"

const (
	cmdParseConfig     = "parseConfig"
	cmdSerializeConfig = "serializeConfig"
)

type request struct {
	Cmd          string        `json:"cmd"`
	Config       string        `json:"config,omitempty"`
	Name         string        `json:"name,omitempty"`
	Since        uint64        `json:"since,omitempty"`
	ParsedConfig *ParsedConfig `json:"parsedConfig,omitempty"`
}

type response struct {
	OK           bool              `json:"ok"`
	Error        string            `json:"error,omitempty"`
	Status       *control.Status   `json:"status,omitempty"`
	Logs         []control.LogLine `json:"logs,omitempty"`
	ParsedConfig *ParsedConfig     `json:"parsedConfig,omitempty"`
	Config       string            `json:"config,omitempty"`
}

type ParsedConfig struct {
	Interface ParsedInterface `json:"interface"`
	Peers     []ParsedPeer    `json:"peers"`
}

type ParsedInterface struct {
	PrivateKey             string `json:"privateKey"`
	Address                string `json:"address,omitempty"`
	BindAddress            string `json:"bindAddress,omitempty"`
	ListenPort             int    `json:"listenPort,omitempty"`
	NID                    string `json:"nid"`
	NetSecret              string `json:"netSecret,omitempty"`
	NetSecretInsecure      bool   `json:"netSecretInsecure,omitempty"`
	AllowInsecureNetSecret bool   `json:"allowInsecureNetSecret,omitempty"`
	Padding                string `json:"padding,omitempty"`
	DNS                    string `json:"dns,omitempty"`
	FwMark                 int    `json:"fwMark,omitempty"`
}

type ParsedPeer struct {
	PublicKey           string   `json:"publicKey"`
	AllowedIPs          []string `json:"allowedIPs,omitempty"`
	Endpoint            string   `json:"endpoint,omitempty"`
	PersistentKeepalive int      `json:"persistentKeepalive,omitempty"`
	PresharedKey        string   `json:"presharedKey,omitempty"`
}
