package render

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	managerconfig "github.com/openwrt-singbox/singbox-manager/internal/config"
)

func TestRenderMatchesGoldenConfig(t *testing.T) {
	cfg := managerconfig.DefaultConfig()
	cfg.Manager.Enabled = true
	cfg.Manager.LogLevel = "debug"
	cfg.Manager.RuntimeMode = "rule"
	cfg.Manager.ActiveGroup = "home"
	cfg.Groups["home"] = managerconfig.Group{
		ID:             "home",
		Enabled:        true,
		Name:           "Home",
		RoutingProfile: "iran_direct",
		DNSProfile:     "split",
		Strategy:       "manual",
		SelectedNode:   "ss1",
	}
	cfg.Nodes["ss1"] = managerconfig.Node{
		ID:       "ss1",
		Enabled:  true,
		Type:     "shadowsocks",
		Server:   "edge.example.com",
		Port:     8388,
		Method:   "2022-blake3-aes-128-gcm",
		Password: "secret",
		Tag:      "node-ss",
	}
	cfg.Routing["iran_direct"] = managerconfig.RoutingProfile{
		ID:       "iran_direct",
		Enabled:  true,
		Name:     "Iran Direct",
		Mode:     "rule",
		RuleSets: []string{"geoip-ir"},
		Final:    "proxy",
	}
	cfg.RuleSets["geoip-ir"] = managerconfig.RuleSet{
		ID:      "geoip-ir",
		Enabled: true,
		Name:    "GeoIP Iran",
		Type:    "local",
		Format:  "srs",
		Path:    "/tmp/geoip-ir.srs",
	}
	cfg.DNSProfiles["split"] = managerconfig.DNSProfile{
		ID:      "split",
		Enabled: true,
		Name:    "Split",
		Mode:    "split",
		Servers: []string{"local_udp"},
	}
	cfg.DNSServers["local_udp"] = managerconfig.DNSServer{
		ID:      "local_udp",
		Enabled: true,
		Name:    "Local UDP",
		Type:    "udp",
		Address: "223.5.5.5",
		Detour:  "direct",
	}

	got, err := Render(cfg)
	if err != nil {
		t.Fatalf("render: %v", err)
	}

	wantPath := filepath.Join("testdata", "config.golden.json")
	want, err := os.ReadFile(wantPath)
	if err != nil {
		t.Fatalf("read golden: %v", err)
	}
	if string(got) != string(want) {
		t.Fatalf("rendered config mismatch\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

func TestRenderGroupStrategies(t *testing.T) {
	for _, tc := range []struct {
		strategy     string
		outboundType string
	}{
		{strategy: "selector", outboundType: "selector"},
		{strategy: "urltest", outboundType: "urltest"},
		{strategy: "load-balance", outboundType: "urltest"},
	} {
		t.Run(tc.strategy, func(t *testing.T) {
			cfg := managerconfig.DefaultConfig()
			cfg.Manager.ActiveGroup = "home"
			cfg.Groups["home"] = managerconfig.Group{
				ID:            "home",
				Enabled:       true,
				Name:          "Home",
				Strategy:      tc.strategy,
				SelectedNode:  "node-b",
				Subscriptions: []string{"sub-a"},
			}
			cfg.Nodes["node-a"] = managerconfig.Node{ID: "node-a", Enabled: true, Type: "direct", Tag: "node-a", Subscription: "sub-a"}
			cfg.Nodes["node-b"] = managerconfig.Node{ID: "node-b", Enabled: true, Type: "direct", Tag: "node-b"}

			data, err := Render(cfg)
			if err != nil {
				t.Fatalf("render: %v", err)
			}
			proxy := renderedOutbound(t, data, "proxy")
			if proxy["type"] != tc.outboundType {
				t.Fatalf("proxy type = %v, want %s\n%s", proxy["type"], tc.outboundType, data)
			}
			outbounds, ok := proxy["outbounds"].([]any)
			if !ok || len(outbounds) < 2 {
				t.Fatalf("proxy outbounds = %#v, want strategy pool", proxy["outbounds"])
			}
			if tc.strategy == "selector" && proxy["default"] != "node-b" {
				t.Fatalf("selector default = %v, want node-b", proxy["default"])
			}
		})
	}
}

func TestRenderStrategySkipsUnsupportedTransportNodes(t *testing.T) {
	cfg := managerconfig.DefaultConfig()
	cfg.Manager.ActiveGroup = "home"
	cfg.Groups["home"] = managerconfig.Group{
		ID:            "home",
		Enabled:       true,
		Name:          "Home",
		Strategy:      "urltest",
		Subscriptions: []string{"sub-a"},
	}
	cfg.Nodes["node-a"] = managerconfig.Node{ID: "node-a", Enabled: true, Type: "vless", Server: "edge.example.com", Port: 443, UUID: "00000000-0000-0000-0000-000000000001", Transport: "xhttp", Subscription: "sub-a"}
	cfg.Nodes["node-b"] = managerconfig.Node{ID: "node-b", Enabled: true, Type: "direct", Tag: "node-b", Subscription: "sub-a"}

	data, err := Render(cfg)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	proxy := renderedOutbound(t, data, "proxy")
	outbounds, ok := proxy["outbounds"].([]any)
	if !ok || len(outbounds) != 1 || outbounds[0] != "node-b" {
		t.Fatalf("proxy outbounds = %#v, want only supported node-b\n%s", proxy["outbounds"], data)
	}
}

func TestRenderManualUnsupportedTransportErrors(t *testing.T) {
	cfg := managerconfig.DefaultConfig()
	cfg.Manager.ActiveGroup = "home"
	cfg.Groups["home"] = managerconfig.Group{
		ID:           "home",
		Enabled:      true,
		Name:         "Home",
		Strategy:     "manual",
		SelectedNode: "node-a",
	}
	cfg.Nodes["node-a"] = managerconfig.Node{ID: "node-a", Enabled: true, Type: "vless", Server: "edge.example.com", Port: 443, UUID: "00000000-0000-0000-0000-000000000001", Transport: "xhttp"}

	_, err := Render(cfg)
	if err == nil || !strings.Contains(err.Error(), "unsupported transport") {
		t.Fatalf("Render error = %v, want unsupported transport", err)
	}
}

func TestRenderHTTPUpgradeTransportIncludesPathAndHost(t *testing.T) {
	cfg := managerconfig.DefaultConfig()
	cfg.Manager.ActiveGroup = "home"
	cfg.Groups["home"] = managerconfig.Group{
		ID:           "home",
		Enabled:      true,
		Name:         "Home",
		Strategy:     "manual",
		SelectedNode: "vmess-httpupgrade",
	}
	cfg.Nodes["vmess-httpupgrade"] = managerconfig.Node{
		ID:        "vmess-httpupgrade",
		Enabled:   true,
		Type:      "vmess",
		Server:    "edge.example.com",
		Port:      80,
		UUID:      "00000000-0000-0000-0000-000000000001",
		Transport: "httpupgrade",
		Host:      "front.example.com",
		Path:      "/upgrade",
		SNI:       "front.example.com",
	}

	data, err := Render(cfg)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	outbound := renderedOutbound(t, data, "vmess-httpupgrade")
	transport, ok := outbound["transport"].(map[string]any)
	if !ok {
		t.Fatalf("transport = %#v, want object\n%s", outbound["transport"], data)
	}
	if transport["type"] != "httpupgrade" || transport["host"] != "front.example.com" || transport["path"] != "/upgrade" {
		t.Fatalf("transport = %#v, want httpupgrade host/path", transport)
	}
	if _, ok := outbound["tls"]; ok {
		t.Fatalf("outbound enabled TLS from SNI without explicit TLS/security: %#v", outbound["tls"])
	}
}

func TestRenderRemoteRuleSetOmitsCachePath(t *testing.T) {
	cfg := managerconfig.DefaultConfig()
	cfg.Manager.ActiveGroup = "home"
	cfg.Manager.RuntimeMode = "rule"
	cfg.Groups["home"] = managerconfig.Group{
		ID:             "home",
		Enabled:        true,
		Name:           "Home",
		Strategy:       "manual",
		RoutingProfile: "routes",
	}
	cfg.Routing["routes"] = managerconfig.RoutingProfile{
		ID:       "routes",
		Enabled:  true,
		Name:     "Routes",
		Mode:     "rule",
		RuleSets: []string{"geoip-ir"},
		Final:    "proxy",
	}
	cfg.RuleSets["geoip-ir"] = managerconfig.RuleSet{
		ID:             "geoip-ir",
		Enabled:        true,
		Name:           "GeoIP Iran",
		Type:           "remote",
		Format:         "srs",
		URL:            "https://example.com/geoip-ir.srs",
		Path:           "/etc/singbox-manager/rulesets/geoip-ir.srs",
		UpdateInterval: "24h",
	}

	data, err := Render(cfg)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	var document struct {
		Route routeConfig `json:"route"`
	}
	if err := json.Unmarshal(data, &document); err != nil {
		t.Fatalf("unmarshal rendered config: %v", err)
	}
	if len(document.Route.RuleSet) != 1 {
		t.Fatalf("rule sets = %d, want 1\n%s", len(document.Route.RuleSet), data)
	}
	entry := document.Route.RuleSet[0]
	if entry["type"] != "remote" || entry["url"] == "" {
		t.Fatalf("remote rule set = %#v", entry)
	}
	if _, ok := entry["path"]; ok {
		t.Fatalf("remote rule set leaked cache path: %#v", entry)
	}
	if entry["download_detour"] != "direct" {
		t.Fatalf("remote rule set download detour = %#v, want direct", entry["download_detour"])
	}
}

func TestRenderRemoteRuleSetUsesExistingCachePath(t *testing.T) {
	path := filepath.Join(t.TempDir(), "geoip-ir.srs")
	if err := os.WriteFile(path, []byte("cache"), 0644); err != nil {
		t.Fatalf("write cache: %v", err)
	}
	cfg := managerconfig.DefaultConfig()
	cfg.Manager.ActiveGroup = "home"
	cfg.Manager.RuntimeMode = "rule"
	cfg.Groups["home"] = managerconfig.Group{
		ID:             "home",
		Enabled:        true,
		Name:           "Home",
		Strategy:       "manual",
		RoutingProfile: "routes",
	}
	cfg.Routing["routes"] = managerconfig.RoutingProfile{
		ID:       "routes",
		Enabled:  true,
		Name:     "Routes",
		Mode:     "rule",
		RuleSets: []string{"geoip-ir"},
		Final:    "proxy",
	}
	cfg.RuleSets["geoip-ir"] = managerconfig.RuleSet{
		ID:      "geoip-ir",
		Enabled: true,
		Name:    "GeoIP Iran",
		Type:    "remote",
		Format:  "srs",
		URL:     "https://example.com/geoip-ir.srs",
		Path:    path,
	}

	data, err := Render(cfg)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	var document struct {
		Route routeConfig `json:"route"`
	}
	if err := json.Unmarshal(data, &document); err != nil {
		t.Fatalf("unmarshal rendered config: %v", err)
	}
	entry := document.Route.RuleSet[0]
	if entry["type"] != "local" || entry["path"] != path {
		t.Fatalf("cached rule set = %#v, want local path %s", entry, path)
	}
	if _, ok := entry["url"]; ok {
		t.Fatalf("cached rule set should not include remote url: %#v", entry)
	}
}

func TestRenderDNSProfilesAndSecureServers(t *testing.T) {
	cfg := managerconfig.DefaultConfig()
	cfg.Manager.ActiveGroup = "home"
	cfg.Groups["home"] = managerconfig.Group{
		ID:         "home",
		Enabled:    true,
		Name:       "Home",
		Strategy:   "manual",
		DNSProfile: "proxy_dns",
	}
	cfg.DNSProfiles["proxy_dns"] = managerconfig.DNSProfile{
		ID:      "proxy_dns",
		Enabled: true,
		Name:    "Proxy DNS",
		Mode:    "proxy",
		Servers: []string{"cloudflare", "quad9"},
	}
	cfg.DNSServers["cloudflare"] = managerconfig.DNSServer{
		ID:      "cloudflare",
		Enabled: true,
		Type:    "doh",
		Address: "https://1.1.1.1/dns-query",
	}
	cfg.DNSServers["quad9"] = managerconfig.DNSServer{
		ID:      "quad9",
		Enabled: true,
		Type:    "dot",
		Address: "tls://dns.quad9.net",
		Detour:  "direct",
	}

	data, err := Render(cfg)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	var document struct {
		DNS dnsConfig `json:"dns"`
	}
	if err := json.Unmarshal(data, &document); err != nil {
		t.Fatalf("unmarshal rendered config: %v", err)
	}
	if len(document.DNS.Servers) != 2 {
		t.Fatalf("dns servers = %d, want 2\n%s", len(document.DNS.Servers), data)
	}
	if document.DNS.Servers[0]["type"] != "https" || document.DNS.Servers[0]["detour"] != "proxy" {
		t.Fatalf("cloudflare server = %#v, want https detour proxy", document.DNS.Servers[0])
	}
	if document.DNS.Servers[1]["type"] != "tls" {
		t.Fatalf("quad9 server = %#v, want tls", document.DNS.Servers[1])
	}
	if _, ok := document.DNS.Servers[1]["detour"]; ok {
		t.Fatalf("quad9 server = %#v, direct detour should be omitted", document.DNS.Servers[1])
	}
}

func TestRenderSplitDNSUsesDomainResolvers(t *testing.T) {
	cfg := managerconfig.DefaultConfig()
	cfg.Manager.ActiveGroup = "home"
	cfg.Groups["home"] = managerconfig.Group{
		ID:         "home",
		Enabled:    true,
		Name:       "Home",
		Strategy:   "manual",
		DNSProfile: "split",
	}
	cfg.DNSProfiles["split"] = managerconfig.DNSProfile{
		ID:      "split",
		Enabled: true,
		Name:    "Split",
		Mode:    "split",
		Servers: []string{"local_udp", "remote_doh"},
	}
	cfg.DNSServers["local_udp"] = managerconfig.DNSServer{
		ID:      "local_udp",
		Enabled: true,
		Type:    "udp",
		Address: "223.5.5.5",
		Detour:  "direct",
	}
	cfg.DNSServers["remote_doh"] = managerconfig.DNSServer{
		ID:      "remote_doh",
		Enabled: true,
		Type:    "doh",
		Address: "https://1.1.1.1/dns-query",
		Detour:  "proxy",
	}

	data, err := Render(cfg)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	var document struct {
		DNS       dnsConfig        `json:"dns"`
		Outbounds []map[string]any `json:"outbounds"`
		Route     routeConfig      `json:"route"`
	}
	if err := json.Unmarshal(data, &document); err != nil {
		t.Fatalf("unmarshal rendered config: %v", err)
	}
	if len(document.DNS.Rules) != 0 {
		t.Fatalf("dns rules = %#v, want no deprecated outbound rules", document.DNS.Rules)
	}
	if document.Route.DefaultDomainResolver != "remote_doh" {
		t.Fatalf("default domain resolver = %q, want remote_doh", document.Route.DefaultDomainResolver)
	}
	direct := renderedOutbound(t, data, "direct")
	if direct["domain_resolver"] != "local_udp" {
		t.Fatalf("direct domain resolver = %#v, want local_udp", direct["domain_resolver"])
	}
}

func TestRenderDNSHijackInboundAndRoute(t *testing.T) {
	cfg := managerconfig.DefaultConfig()
	cfg.Manager.ActiveGroup = "home"
	cfg.Manager.DNSPort = 1053
	cfg.Groups["home"] = managerconfig.Group{
		ID:         "home",
		Enabled:    true,
		Name:       "Home",
		Strategy:   "manual",
		DNSProfile: "split",
	}
	cfg.DNSProfiles["split"] = managerconfig.DNSProfile{
		ID:      "split",
		Enabled: true,
		Name:    "Split",
		Mode:    "split",
		Hijack:  true,
		Servers: []string{"local_udp"},
	}
	cfg.DNSServers["local_udp"] = managerconfig.DNSServer{
		ID:      "local_udp",
		Enabled: true,
		Type:    "udp",
		Address: "223.5.5.5",
	}

	data, err := Render(cfg)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if !strings.Contains(string(data), `"tag": "dns-in"`) {
		t.Fatalf("rendered config missing DNS inbound:\n%s", data)
	}
	if !strings.Contains(string(data), `"action": "hijack-dns"`) {
		t.Fatalf("rendered config missing DNS hijack route:\n%s", data)
	}
}

func TestRenderSourceIPRouteRules(t *testing.T) {
	cfg := managerconfig.DefaultConfig()
	cfg.Manager.ActiveGroup = "home"
	cfg.Manager.RuntimeMode = "rule"
	cfg.Groups["home"] = managerconfig.Group{
		ID:             "home",
		Enabled:        true,
		Name:           "Home",
		Strategy:       "manual",
		RoutingProfile: "routes",
	}
	cfg.Routing["routes"] = managerconfig.RoutingProfile{
		ID:      "routes",
		Enabled: true,
		Name:    "Routes",
		Mode:    "rule",
		Final:   "proxy",
	}
	cfg.SourceRules["phone"] = managerconfig.SourceRule{
		ID:       "phone",
		Enabled:  true,
		Name:     "Phone",
		Profile:  "routes",
		Sources:  []string{"192.168.1.20", "fd00::2"},
		Outbound: "direct",
	}

	data, err := Render(cfg)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if !strings.Contains(string(data), `"source_ip_cidr": [`) {
		t.Fatalf("rendered config missing source_ip_cidr:\n%s", data)
	}
	if !strings.Contains(string(data), `"192.168.1.20/32"`) || !strings.Contains(string(data), `"fd00::2/128"`) {
		t.Fatalf("rendered config missing normalized source CIDRs:\n%s", data)
	}
}

func TestRenderTUNInbound(t *testing.T) {
	cfg := managerconfig.DefaultConfig()
	cfg.Manager.ActiveGroup = "home"
	cfg.Groups["home"] = managerconfig.Group{
		ID:       "home",
		Enabled:  true,
		Name:     "Home",
		Strategy: "manual",
	}
	cfg.TUN.Enabled = true
	cfg.TUN.AutoRoute = true
	cfg.TUN.AutoRedirect = false
	cfg.TUN.Inet4Address = "172.19.0.1/30"
	cfg.TUN.Inet6Address = "fdfe:dcba:9876::1/126"

	data, err := Render(cfg)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	for _, want := range []string{
		`"type": "tun"`,
		`"interface_name": "singbox0"`,
		`"address": [`,
		`"172.19.0.1/30"`,
		`"fdfe:dcba:9876::1/126"`,
		`"auto_route": true`,
		`"auto_redirect": false`,
	} {
		if !strings.Contains(string(data), want) {
			t.Fatalf("rendered config missing %q:\n%s", want, data)
		}
	}
	for _, deprecated := range []string{`"inet4_address"`, `"inet6_address"`} {
		if strings.Contains(string(data), deprecated) {
			t.Fatalf("rendered config contains deprecated %s:\n%s", deprecated, data)
		}
	}
}

func TestRenderSupportsMilestoneThreeNodeTypes(t *testing.T) {
	for _, tc := range []struct {
		name string
		node managerconfig.Node
	}{
		{
			name: "hysteria2",
			node: managerconfig.Node{
				ID:       "hy2",
				Enabled:  true,
				Type:     "hysteria2",
				Server:   "hy.example.com",
				Port:     443,
				Password: "secret",
				TLS:      true,
				SNI:      "hy.example.com",
			},
		},
		{
			name: "tuic",
			node: managerconfig.Node{
				ID:           "tuic",
				Enabled:      true,
				Type:         "tuic",
				Server:       "tuic.example.com",
				Port:         443,
				UUID:         "00000000-0000-0000-0000-000000000001",
				Password:     "secret",
				TLS:          true,
				Congestion:   "bbr",
				UDPRelayMode: "native",
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := managerconfig.DefaultConfig()
			cfg.Manager.ActiveGroup = "home"
			cfg.Groups["home"] = managerconfig.Group{
				ID:           "home",
				Enabled:      true,
				Name:         "Home",
				Strategy:     "manual",
				SelectedNode: tc.node.ID,
			}
			cfg.Nodes[tc.node.ID] = tc.node

			data, err := Render(cfg)
			if err != nil {
				t.Fatalf("render: %v", err)
			}
			if !strings.Contains(string(data), `"type": "`+tc.node.Type+`"`) {
				t.Fatalf("rendered config missing %s outbound:\n%s", tc.node.Type, data)
			}
		})
	}
}

func renderedOutbound(t *testing.T, data []byte, tag string) map[string]any {
	t.Helper()
	var document struct {
		Outbounds []map[string]any `json:"outbounds"`
	}
	if err := json.Unmarshal(data, &document); err != nil {
		t.Fatalf("unmarshal rendered config: %v", err)
	}
	for _, outbound := range document.Outbounds {
		if outbound["tag"] == tag {
			return outbound
		}
	}
	t.Fatalf("outbound %q not found in %s", tag, data)
	return nil
}
