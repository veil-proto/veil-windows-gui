package main

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/veil-proto/veil/config"
)

func minimalParsedConfig() ParsedConfig {
	return ParsedConfig{
		Interface: ParsedInterface{
			PrivateKey: validKeyHex(),
			NID:        validKeyHex(),
			NetSecret:  validKeyHex(),
		},
	}
}

func validKeyHex() string { return strings.Repeat("ab", 32) }

func testConfigText() string {
	return "[Interface]\n" +
		"PrivateKey = " + validKeyHex() + "\n" +
		"NID = " + validKeyHex() + "\n" +
		"NetSecret = " + validKeyHex() + "\n" +
		"Address = 10.0.0.1/24\n" +
		"[Peer]\n" +
		"PublicKey = " + validKeyHex() + "\n" +
		"AllowedIPs = 10.0.0.0/24, 192.168.1.0/24\n" +
		"Endpoint = example.com:51820\n" +
		"PersistentKeepalive = 25\n"
}

// TestToParsedConfigThenFromParsedConfigRoundTrips proves ParseConfig's
// output can be fed straight back through SerializeConfig's input side
// (fromParsedConfig) and reproduce the same config.Config — this is the
// exact round trip the split-tunnel editor drives (load, edit fields, save).
func TestToParsedConfigThenFromParsedConfigRoundTrips(t *testing.T) {
	cfg, err := config.LoadConfigString(testConfigText())
	if err != nil {
		t.Fatalf("LoadConfigString: %v", err)
	}

	pc := toParsedConfig(cfg)
	if pc.Interface.PrivateKey != validKeyHex() {
		t.Errorf("PrivateKey = %q", pc.Interface.PrivateKey)
	}
	if len(pc.Peers) != 1 || pc.Peers[0].PublicKey != validKeyHex() {
		t.Fatalf("Peers = %+v", pc.Peers)
	}
	if len(pc.Peers[0].AllowedIPs) != 2 {
		t.Fatalf("AllowedIPs = %v", pc.Peers[0].AllowedIPs)
	}

	back, err := fromParsedConfig(pc)
	if err != nil {
		t.Fatalf("fromParsedConfig: %v", err)
	}
	if string(back.Interface.PrivateKey) != string(cfg.Interface.PrivateKey) {
		t.Error("PrivateKey did not round-trip")
	}
	if string(back.Interface.NID) != string(cfg.Interface.NID) {
		t.Error("NID did not round-trip")
	}
	if len(back.Peers) != 1 || string(back.Peers[0].PublicKey) != string(cfg.Peers[0].PublicKey) {
		t.Error("Peer PublicKey did not round-trip")
	}
	if back.Peers[0].PersistentKeepalive != 25 {
		t.Errorf("PersistentKeepalive = %d, want 25", back.Peers[0].PersistentKeepalive)
	}

	// The round-tripped config must still validate and serialize.
	if err := back.Validate(); err != nil {
		t.Fatalf("round-tripped config failed Validate: %v", err)
	}
	if serialized := back.Serialize(); !strings.Contains(serialized, "PersistentKeepalive = 25") {
		t.Errorf("serialized config missing PersistentKeepalive: %s", serialized)
	}
}

func TestToParsedConfigEmitsEmptyPeersArray(t *testing.T) {
	cfg, err := config.LoadConfigString("[Interface]\n" +
		"PrivateKey = " + validKeyHex() + "\n" +
		"NID = " + validKeyHex() + "\n" +
		"NetSecret = " + validKeyHex() + "\n")
	if err != nil {
		t.Fatalf("LoadConfigString: %v", err)
	}

	b, err := json.Marshal(toParsedConfig(cfg))
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if !strings.Contains(string(b), `"peers":[]`) {
		t.Fatalf("ParsedConfig JSON should contain an empty peers array, got %s", b)
	}
}

// TestFromParsedConfigRejectsBadHex verifies a malformed hex field (e.g. a
// typo from manual editing) surfaces as an error instead of a truncated key,
// matching config.Validate's own "never silently truncate" invariant.
func TestFromParsedConfigRejectsBadHex(t *testing.T) {
	pc := minimalParsedConfig()
	pc.Interface.PrivateKey = "not-hex"
	if _, err := fromParsedConfig(pc); err == nil {
		t.Fatal("expected an error for malformed PrivateKey hex")
	}
}

// TestNetSecretInsecureSkipsNetSecretDecode verifies fromParsedConfig doesn't
// try to hex-decode an empty NetSecret when NetSecretInsecure is set.
func TestNetSecretInsecureSkipsNetSecretDecode(t *testing.T) {
	pc := minimalParsedConfig()
	pc.Interface.NetSecret = ""
	pc.Interface.NetSecretInsecure = true
	pc.Interface.AllowInsecureNetSecret = true
	cfg, err := fromParsedConfig(pc)
	if err != nil {
		t.Fatalf("fromParsedConfig: %v", err)
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
}
