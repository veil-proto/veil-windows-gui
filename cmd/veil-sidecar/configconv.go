// Package main: config <-> ParsedConfig conversion. Deliberately has
// no build tag (unlike handler_windows.go) — it only touches
// github.com/veil-proto/veil/config, so this logic can be unit tested without
// a Windows target.
package main

import (
	"encoding/hex"
	"fmt"

	"github.com/veil-proto/veil/config"
)

// toParsedConfig converts a loaded Config into the JSON-friendly shape the
// control protocol sends to the frontend. Byte fields become hex strings,
// matching how the .conf format already represents them on disk.
func toParsedConfig(cfg *config.Config) ParsedConfig {
	pc := ParsedConfig{
		Interface: ParsedInterface{
			PrivateKey:             hex.EncodeToString(cfg.Interface.PrivateKey),
			Address:                cfg.Interface.Address,
			BindAddress:            cfg.Interface.BindAddress,
			ListenPort:             cfg.Interface.ListenPort,
			NID:                    hex.EncodeToString(cfg.Interface.NID),
			NetSecret:              hex.EncodeToString(cfg.Interface.NetSecret),
			NetSecretInsecure:      cfg.Interface.NetSecretInsecure,
			AllowInsecureNetSecret: cfg.Interface.AllowInsecureNetSecret,
			Padding:                cfg.Interface.Padding,
			DNS:                    cfg.Interface.DNS,
			FwMark:                 cfg.Interface.FwMark,
		},
		Peers: make([]ParsedPeer, 0, len(cfg.Peers)),
	}
	for _, p := range cfg.Peers {
		pc.Peers = append(pc.Peers, ParsedPeer{
			PublicKey:           hex.EncodeToString(p.PublicKey),
			AllowedIPs:          p.AllowedIPs,
			Endpoint:            p.Endpoint,
			PersistentKeepalive: p.PersistentKeepalive,
			PresharedKey:        hex.EncodeToString(p.PresharedKey),
		})
	}
	return pc
}

// fromParsedConfig is toParsedConfig's inverse: decodes the frontend's edited
// structured config back into a config.Config, ready for Validate()/Serialize().
func fromParsedConfig(pc ParsedConfig) (*config.Config, error) {
	privKey, err := hex.DecodeString(pc.Interface.PrivateKey)
	if err != nil {
		return nil, fmt.Errorf("invalid PrivateKey: %w", err)
	}
	nid, err := hex.DecodeString(pc.Interface.NID)
	if err != nil {
		return nil, fmt.Errorf("invalid NID: %w", err)
	}
	var netSecret []byte
	if !pc.Interface.NetSecretInsecure {
		netSecret, err = hex.DecodeString(pc.Interface.NetSecret)
		if err != nil {
			return nil, fmt.Errorf("invalid NetSecret: %w", err)
		}
	}

	cfg := &config.Config{
		Interface: config.InterfaceConfig{
			PrivateKey:             privKey,
			Address:                pc.Interface.Address,
			BindAddress:            pc.Interface.BindAddress,
			ListenPort:             pc.Interface.ListenPort,
			NID:                    nid,
			NetSecret:              netSecret,
			NetSecretInsecure:      pc.Interface.NetSecretInsecure,
			AllowInsecureNetSecret: pc.Interface.AllowInsecureNetSecret,
			Padding:                pc.Interface.Padding,
			DNS:                    pc.Interface.DNS,
			FwMark:                 pc.Interface.FwMark,
		},
	}
	for i, p := range pc.Peers {
		pubKey, err := hex.DecodeString(p.PublicKey)
		if err != nil {
			return nil, fmt.Errorf("peer[%d]: invalid PublicKey: %w", i, err)
		}
		var psk []byte
		if p.PresharedKey != "" {
			psk, err = hex.DecodeString(p.PresharedKey)
			if err != nil {
				return nil, fmt.Errorf("peer[%d]: invalid PresharedKey: %w", i, err)
			}
		}
		cfg.Peers = append(cfg.Peers, config.PeerConfig{
			PublicKey:           pubKey,
			AllowedIPs:          p.AllowedIPs,
			Endpoint:            p.Endpoint,
			PersistentKeepalive: p.PersistentKeepalive,
			PresharedKey:        psk,
		})
	}
	return cfg, nil
}
