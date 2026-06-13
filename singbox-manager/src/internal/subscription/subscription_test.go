package subscription

import (
	"encoding/base64"
	"strings"
	"testing"
	"time"

	managerconfig "github.com/openwrt-singbox/singbox-manager/internal/config"
)

func TestParseDetectsBase64Subscription(t *testing.T) {
	payload := strings.Join([]string{
		"vless://00000000-0000-0000-0000-000000000001@example.com:443?security=tls&sni=edge.example.com&type=ws&host=cdn.example.com&path=%2Fws#VLESS",
		"trojan://secret@example.org:443?security=tls&sni=trojan.example.org#Trojan",
		"ss://YWVzLTEyOC1nY206cGFzc0Bzcy5leGFtcGxlLmNvbTo4Mzg4#SS",
		"hysteria2://hy-pass@hy.example.net:443?sni=hy.example.net&insecure=1#HY2",
		"tuic://00000000-0000-0000-0000-000000000002:tuic-pass@tuic.example.net:443?congestion_control=bbr&udp_relay_mode=native&sni=tuic.example.net#TUIC",
	}, "\n")
	encoded := base64.StdEncoding.EncodeToString([]byte(payload))

	nodes, err := Parse([]byte(encoded), "auto")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(nodes) != 5 {
		t.Fatalf("node count = %d, want 5", len(nodes))
	}
	if nodes[0].Type != "vless" || nodes[0].Transport != "ws" || nodes[0].SNI != "edge.example.com" || !nodes[0].TLS {
		t.Fatalf("vless node = %#v", nodes[0])
	}
	if nodes[2].Type != "shadowsocks" || nodes[2].Method != "aes-128-gcm" || nodes[2].Password != "pass" {
		t.Fatalf("shadowsocks node = %#v", nodes[2])
	}
	if !nodes[3].Insecure {
		t.Fatalf("hysteria2 insecure = false, want true")
	}
	if nodes[4].Type != "tuic" || nodes[4].Congestion != "bbr" || nodes[4].UDPRelayMode != "native" {
		t.Fatalf("tuic node = %#v", nodes[4])
	}
}

func TestAssignSourceCreatesDeterministicIDs(t *testing.T) {
	nodes, err := Parse([]byte("trojan://secret@example.org:443#Trojan"), "plain")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	first := AssignSource(nodes, "Example Sub")
	second := AssignSource(nodes, "Example Sub")
	if first[0].ID == "" {
		t.Fatal("expected node id")
	}
	if first[0].ID != second[0].ID {
		t.Fatalf("id = %q, want %q", first[0].ID, second[0].ID)
	}
	if first[0].Subscription != "Example Sub" {
		t.Fatalf("subscription = %q, want Example Sub", first[0].Subscription)
	}
}

func TestParseVMessDoesNotUseTransportHostAsSNI(t *testing.T) {
	payload := `{"v":"2","ps":"VMess","add":"edge.example.com","port":"80","id":"00000000-0000-0000-0000-000000000001","aid":"0","scy":"auto","net":"httpupgrade","type":"none","host":"front.example.com","path":"/upgrade","tls":""}`
	uri := "vmess://" + base64.StdEncoding.EncodeToString([]byte(payload))

	node, err := ParseURI(uri)
	if err != nil {
		t.Fatalf("parse vmess: %v", err)
	}
	if node.Transport != "httpupgrade" || node.Host != "front.example.com" || node.Path != "/upgrade" {
		t.Fatalf("vmess transport fields = %#v", node)
	}
	if node.SNI != "" {
		t.Fatalf("SNI = %q, want empty when only transport host is set", node.SNI)
	}
	if node.TLS {
		t.Fatal("TLS = true, want false for vmess tls empty")
	}
}

func TestDueUsesSubscriptionUpdateInterval(t *testing.T) {
	now := time.Date(2026, 6, 3, 12, 0, 0, 0, time.UTC)
	source := managerconfig.Subscription{
		ID:             "sub",
		Enabled:        true,
		URL:            "https://example.com/sub",
		UpdateInterval: "2h",
		LastUpdate:     now.Add(-3 * time.Hour).Format(time.RFC3339),
	}
	if !Due(source, now) {
		t.Fatal("expected subscription to be due")
	}

	source.LastUpdate = now.Add(-30 * time.Minute).Format(time.RFC3339)
	if Due(source, now) {
		t.Fatal("expected subscription not to be due")
	}

	source.Enabled = false
	if Due(source, now) {
		t.Fatal("disabled subscription should not be due")
	}
}
