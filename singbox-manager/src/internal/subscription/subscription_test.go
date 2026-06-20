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

func TestParseSingBoxJSONConfig(t *testing.T) {
	config := `{
		"log": {"level": "warn"},
		"dns": {"servers": [{"type": "udp", "tag": "dns-remote", "server": "1.1.1.2", "detour": "proxy"}]},
		"inbounds": [{"type": "tun", "tag": "tun-in"}],
		"outbounds": [
			{"type": "selector", "tag": "proxy", "outbounds": ["amin shadowsocks"]},
			{"type": "urltest", "tag": "Best Latency", "outbounds": ["amin shadowsocks"]},
			{"type": "direct", "tag": "direct"},
			{"type": "shadowsocks", "tag": "amin shadowsocks", "server": "n1.uppc.site", "server_port": 1080, "method": "chacha20-ietf-poly1305", "password": "GK_pass"},
			{"type": "vmess", "tag": "amin vmess ws tls", "server": "188.114.98.0", "server_port": 2053, "uuid": "d975f9d5-d0eb-4e71-b016-4b78531bbe57",
				"transport": {"type": "ws", "headers": {"host": ["n1t.uppc.site"]}, "path": "/qlb4hqR5wd5g6WYb"},
				"tls": {"enabled": true, "server_name": "n1t.uppc.site"}},
			{"type": "trojan", "tag": "amin trojan httpupgrade tls", "server": "chatgpt.com", "server_port": 2087, "password": "us9w5pass",
				"transport": {"type": "httpupgrade", "host": "n1t.uppc.site", "path": "/catd-hux"},
				"tls": {"enabled": true, "server_name": "n1t.uppc.site"}},
			{"type": "vless", "tag": "amin IR vless tcp", "server": "nir.uppc.site", "server_port": 56309, "uuid": "6a8ec418-1166-4f8d-b780-ed15f8411c13"}
		],
		"route": {"final": "proxy"},
		"endpoints": []
	}`

	nodes, err := Parse([]byte(config), "auto")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(nodes) != 4 {
		t.Fatalf("node count = %d, want 4 (selector/urltest/direct skipped)", len(nodes))
	}

	ss := nodes[0]
	if ss.Type != "shadowsocks" || ss.Server != "n1.uppc.site" || ss.Port != 1080 || ss.Method != "chacha20-ietf-poly1305" || ss.Password != "GK_pass" {
		t.Fatalf("shadowsocks node = %#v", ss)
	}

	vmess := nodes[1]
	if vmess.Type != "vmess" || vmess.Transport != "ws" || vmess.Path != "/qlb4hqR5wd5g6WYb" || vmess.Host != "n1t.uppc.site" {
		t.Fatalf("vmess transport = %#v", vmess)
	}
	if !vmess.TLS || vmess.SNI != "n1t.uppc.site" {
		t.Fatalf("vmess tls = %#v", vmess)
	}

	trojan := nodes[2]
	if trojan.Type != "trojan" || trojan.Transport != "httpupgrade" || trojan.Host != "n1t.uppc.site" || trojan.Path != "/catd-hux" {
		t.Fatalf("trojan transport = %#v", trojan)
	}

	vless := nodes[3]
	if vless.Type != "vless" || vless.Port != 56309 || vless.UUID != "6a8ec418-1166-4f8d-b780-ed15f8411c13" {
		t.Fatalf("vless node = %#v", vless)
	}
}

func TestParseSingBoxJSONConfigBase64(t *testing.T) {
	config := `{"outbounds":[{"type":"shadowsocks","tag":"ss","server":"ss.example.com","server_port":8388,"method":"aes-128-gcm","password":"pass"}]}`
	encoded := base64.StdEncoding.EncodeToString([]byte(config))

	nodes, err := Parse([]byte(encoded), "auto")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(nodes) != 1 || nodes[0].Type != "shadowsocks" || nodes[0].Server != "ss.example.com" {
		t.Fatalf("nodes = %#v", nodes)
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

func TestParseVMessStandardURI(t *testing.T) {
	uri := "vmess://abd4fd93-f23d-4c2e-ac8a-bce395947e6d@104.21.33.234:80?type=httpupgrade&host=v2.mokhtari94.ir&path=/qlb4hqR5wdecX3r8BQxdso&packetEncoding=xudp#Auto%20http%20httpupgrade%20CDN%20vmess"

	node, err := ParseURI(uri)
	if err != nil {
		t.Fatalf("parse standard vmess: %v", err)
	}
	if node.Type != "vmess" {
		t.Fatalf("type = %q, want vmess", node.Type)
	}
	if node.UUID != "abd4fd93-f23d-4c2e-ac8a-bce395947e6d" {
		t.Fatalf("uuid = %q", node.UUID)
	}
	if node.Server != "104.21.33.234" || node.Port != 80 {
		t.Fatalf("server/port = %s:%d", node.Server, node.Port)
	}
	if node.Transport != "httpupgrade" || node.Host != "v2.mokhtari94.ir" || node.Path != "/qlb4hqR5wdecX3r8BQxdso" {
		t.Fatalf("transport fields = %#v", node)
	}
	if node.TLS {
		t.Fatal("TLS = true, want false on a port-80 vmess link with no security param")
	}
	if node.Security != "auto" {
		t.Fatalf("cipher = %q, want auto (must not be the TLS setting)", node.Security)
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
