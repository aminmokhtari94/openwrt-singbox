package pac

import (
	"strings"
	"testing"

	managerconfig "github.com/openwrt-singbox/singbox-manager/internal/config"
)

func TestRenderIncludesLocalBypassAndRules(t *testing.T) {
	cfg := managerconfig.DefaultConfig()
	cfg.Manager.MixedListen = "192.168.1.1"
	cfg.Manager.MixedPort = 2080
	cfg.PAC.Enabled = true
	cfg.PAC.LocalBypass = true
	cfg.PAC.CustomRules = []string{"proxy .blocked.example", "direct intranet.example"}
	cfg.PAC.Whitelist = []string{".direct.example"}
	cfg.PAC.Blacklist = []string{"*.proxy.example"}

	data, err := Render(cfg)
	if err != nil {
		t.Fatalf("render pac: %v", err)
	}
	text := string(data)
	for _, want := range []string{
		"PROXY 192.168.1.1:2080; DIRECT",
		"SINGBOX_MANAGER_LOCAL_BYPASS = true",
		`"pattern":".blocked.example","action":"proxy"`,
		`"pattern":"intranet.example","action":"direct"`,
		"FindProxyForURL",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("PAC missing %q\n%s", want, text)
		}
	}
}

func TestRenderNormalizesUnspecifiedProxyListenToFallback(t *testing.T) {
	cfg := managerconfig.DefaultConfig()
	cfg.Manager.MixedListen = "0.0.0.0"
	cfg.Manager.MixedPort = 2080

	data, err := Render(cfg)
	if err != nil {
		t.Fatalf("render pac: %v", err)
	}
	if !strings.Contains(string(data), "PROXY 127.0.0.1:2080; DIRECT") {
		t.Fatalf("PAC proxy directive did not normalize unspecified listen\n%s", data)
	}
}

func TestRenderUsesRequestHostForUnspecifiedProxyListen(t *testing.T) {
	cfg := managerconfig.DefaultConfig()
	cfg.Manager.MixedListen = "0.0.0.0"
	cfg.Manager.MixedPort = 2080

	data, err := RenderForProxyHost(cfg, "192.168.200.1:1088")
	if err != nil {
		t.Fatalf("render pac: %v", err)
	}
	if !strings.Contains(string(data), "PROXY 192.168.200.1:2080; DIRECT") {
		t.Fatalf("PAC proxy directive did not use request host\n%s", data)
	}
}

func TestRenderUsesIPv6RequestHostForUnspecifiedProxyListen(t *testing.T) {
	cfg := managerconfig.DefaultConfig()
	cfg.Manager.MixedListen = "::"
	cfg.Manager.MixedPort = 2080

	data, err := RenderForProxyHost(cfg, "[fd00::1]:1088")
	if err != nil {
		t.Fatalf("render pac: %v", err)
	}
	if !strings.Contains(string(data), "PROXY [fd00::1]:2080; DIRECT") {
		t.Fatalf("PAC proxy directive did not use IPv6 request host\n%s", data)
	}
}

func TestRenderActiveReturnsSelectedCustomPAC(t *testing.T) {
	cfg := managerconfig.DefaultConfig()
	cfg.PAC.Source = "custom"
	cfg.PAC.SelectedCustom = "office"
	cfg.CustomPACs["office"] = managerconfig.CustomPAC{
		ID:      "office",
		Enabled: true,
		Name:    "Office",
		Content: `function FindProxyForURL(url, host) { return "DIRECT"; }`,
	}

	data, err := RenderActive(cfg)
	if err != nil {
		t.Fatalf("render active pac: %v", err)
	}
	if string(data) != cfg.CustomPACs["office"].Content {
		t.Fatalf("active PAC = %q, want custom content", data)
	}
}
