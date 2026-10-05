package render

import (
	"encoding/json"
	"testing"

	managerconfig "github.com/openwrt-singbox/singbox-manager/internal/config"
)

func TestVMessSecurityTLSIsNotRenderedAsCipher(t *testing.T) {
	// A VMess node whose share link put "tls" in the security field must not emit
	// security:"tls" (an invalid cipher sing-box rejects); TLS goes in the TLS block.
	ob, err := renderNodeOutbound(managerconfig.Node{
		ID: "v", Type: "vmess", Server: "example.com", Port: 443, UUID: "u",
		Security: "tls", TLS: true,
	})
	if err != nil {
		t.Fatalf("render vmess: %v", err)
	}
	if _, ok := ob["security"]; ok {
		t.Fatalf("expected no vmess cipher for security=tls, got %v", ob["security"])
	}
	if _, ok := ob["tls"]; !ok {
		t.Fatal("expected TLS block to be rendered for a TLS vmess node")
	}

	// A genuine cipher is preserved.
	ob, err = renderNodeOutbound(managerconfig.Node{
		ID: "v2", Type: "vmess", Server: "example.com", Port: 443, UUID: "u",
		Security: "aes-128-gcm",
	})
	if err != nil {
		t.Fatalf("render vmess: %v", err)
	}
	if ob["security"] != "aes-128-gcm" {
		t.Fatalf("expected cipher preserved, got %v", ob["security"])
	}
}

func TestRenderVLESSRealityEmitsRealityAndUTLS(t *testing.T) {
	ob, err := renderNodeOutbound(managerconfig.Node{
		ID: "r", Type: "vless", Server: "example.com", Port: 9443, UUID: "u",
		Security: "reality", SNI: "www.speedtest.net",
		RealityPublicKey: "PUBKEY", RealityShortID: "sid123", Fingerprint: "chrome",
		Insecure: true,
	})
	if err != nil {
		t.Fatalf("render reality: %v", err)
	}
	tls, ok := ob["tls"].(map[string]any)
	if !ok {
		t.Fatal("expected tls block")
	}
	reality, ok := tls["reality"].(map[string]any)
	if !ok {
		t.Fatal("expected reality block")
	}
	if reality["public_key"] != "PUBKEY" || reality["short_id"] != "sid123" {
		t.Fatalf("reality params not rendered: %v", reality)
	}
	utls, ok := tls["utls"].(map[string]any)
	if !ok || utls["fingerprint"] != "chrome" {
		t.Fatalf("expected utls fingerprint chrome, got %v", tls["utls"])
	}
	// Reality is incompatible with insecure; it must be dropped.
	if _, ok := tls["insecure"]; ok {
		t.Fatal("insecure must not be set alongside reality")
	}
}

func TestRenderRealityDefaultsFingerprintToChrome(t *testing.T) {
	ob, _ := renderNodeOutbound(managerconfig.Node{
		ID: "r", Type: "vless", Server: "e.com", Port: 443, UUID: "u",
		Security: "reality", RealityPublicKey: "PK",
	})
	tls := ob["tls"].(map[string]any)
	utls, ok := tls["utls"].(map[string]any)
	if !ok || utls["fingerprint"] != "chrome" {
		t.Fatalf("expected default chrome fingerprint, got %v", tls["utls"])
	}
}

func TestBuildProbeConfigRendersNodesAndSkipsUnsupported(t *testing.T) {
	nodes := []managerconfig.Node{
		{ID: "ok", Enabled: true, Type: "vless", Server: "example.com", Port: 443, UUID: "u", TLS: true, Transport: "ws", Path: "/p", Tag: "ok"},
		{ID: "bad", Enabled: true, Type: "vless", Server: "example.com", Port: 443, UUID: "u", Transport: "xhttp"},
		{ID: "off", Enabled: false, Type: "vless", Server: "example.com", Port: 443, UUID: "u"},
	}

	data, tags, skipped, err := BuildProbeConfig(nodes, ProbeAPI{Listen: "127.0.0.1:19090", Secret: "s3cr3t"})
	if err != nil {
		t.Fatalf("build probe config: %v", err)
	}
	if tags["ok"] != "ok" {
		t.Fatalf("expected renderable node to map to its tag, got %q", tags["ok"])
	}
	if _, ok := tags["bad"]; ok {
		t.Fatal("unsupported-transport node should not be testable")
	}
	if _, ok := skipped["bad"]; !ok {
		t.Fatal("unsupported-transport node should be reported in skipped")
	}
	if _, ok := tags["off"]; ok {
		t.Fatal("disabled node should be excluded")
	}

	var document struct {
		Outbounds    []map[string]any `json:"outbounds"`
		Experimental struct {
			ClashAPI struct {
				ExternalController string `json:"external_controller"`
				Secret             string `json:"secret"`
			} `json:"clash_api"`
		} `json:"experimental"`
	}
	if err := json.Unmarshal(data, &document); err != nil {
		t.Fatalf("probe config is not valid json: %v", err)
	}
	if document.Experimental.ClashAPI.ExternalController != "127.0.0.1:19090" {
		t.Fatalf("clash api controller not set: %q", document.Experimental.ClashAPI.ExternalController)
	}
	if document.Experimental.ClashAPI.Secret != "s3cr3t" {
		t.Fatalf("clash api secret not set: %q", document.Experimental.ClashAPI.Secret)
	}
	// direct + block + the single renderable node.
	if len(document.Outbounds) != 3 {
		t.Fatalf("expected 3 outbounds (direct, block, ok), got %d", len(document.Outbounds))
	}
}
