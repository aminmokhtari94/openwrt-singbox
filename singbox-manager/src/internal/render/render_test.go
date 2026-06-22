package render

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	managerconfig "github.com/openwrt-singbox/singbox-manager/internal/config"
)

// targetConfig builds a manager config that mirrors the reference sing-box
// configuration: per-source DNS rules, a per-source route rule combined with a
// rule set, remote rule sets, and a VLESS httpupgrade node behind a selector.
func targetConfig() managerconfig.Config {
	cfg := managerconfig.DefaultConfig()
	cfg.Manager.Enabled = true
	cfg.Manager.LogLevel = "info"
	cfg.Manager.RuntimeMode = "rule"
	cfg.Manager.ActiveGroup = "home"
	cfg.Manager.MixedPort = 2080
	cfg.Manager.TProxyPort = 7893
	cfg.Manager.DNSPort = 1053
	cfg.Transparent.DefaultMode = "tproxy"
	cfg.Transparent.DNSHijack = true
	cfg.Transparent.LANIfnames = []string{"br-lan"}

	cfg.Groups["home"] = managerconfig.Group{
		ID:           "home",
		Enabled:      true,
		Name:         "Home",
		Strategy:     "manual",
		SelectedNode: "vless1",
		RouteFinal:   "proxy",
		DNSFinal:     "cloudflare_doh",
	}

	cfg.Nodes["vless1"] = managerconfig.Node{
		ID:        "vless1",
		Enabled:   true,
		Type:      "vless",
		Server:    "69.84.182.16",
		Port:      443,
		UUID:      "abd4fd93-f23d-4c2e-ac8a-bce395947e6d",
		TLS:       true,
		SNI:       "v2.mokhtari94.ir",
		ALPN:      "http/1.1",
		Insecure:  true,
		Transport: "httpupgrade",
		Host:      "v2.mokhtari94.ir",
		Path:      "/87IIO0MKecX3r8BQxdso",
		Tag:       "vless_0b96da71_f27d0bbdc3",
	}

	cfg.DNSServers["cloudflare_doh"] = managerconfig.DNSServer{
		ID: "cloudflare_doh", Enabled: true, Name: "Cloudflare DoH",
		Type: "doh", Address: "https://1.1.1.1/dns-query", Detour: "proxy",
	}
	cfg.DNSServers["cloudflare_doh_direct"] = managerconfig.DNSServer{
		ID: "cloudflare_doh_direct", Enabled: true, Name: "Cloudflare DoH (direct)",
		Type: "doh", Address: "https://1.1.1.1/dns-query", Detour: "direct",
	}
	cfg.DNSServers["local_udp"] = managerconfig.DNSServer{
		ID: "local_udp", Enabled: true, Name: "Local UDP",
		Type: "udp", Address: "223.5.5.5",
	}

	cfg.DNSRules["d10_dev124"] = managerconfig.DNSRule{
		ID: "d10_dev124", Enabled: true, Name: "Device 124 DNS",
		Group: "home", Sources: []string{"192.168.200.124/32"}, Server: "cloudflare_doh",
	}
	cfg.DNSRules["d20_dev125"] = managerconfig.DNSRule{
		ID: "d20_dev125", Enabled: true, Name: "Device 125 DNS",
		Group: "home", Sources: []string{"192.168.200.125/32"}, Server: "cloudflare_doh_direct",
	}

	cfg.RuleSets["geoip-ir"] = managerconfig.RuleSet{
		ID: "geoip-ir", Enabled: true, Name: "GeoIP Iran", Type: "remote", Format: "binary",
		URL:            "https://raw.githubusercontent.com/Chocolate4U/Iran-sing-box-rules/rule-set/geoip-ir.srs",
		UpdateInterval: "168h",
	}
	cfg.RuleSets["geosite-ir"] = managerconfig.RuleSet{
		ID: "geosite-ir", Enabled: true, Name: "Geosite Iran", Type: "remote", Format: "binary",
		URL:            "https://raw.githubusercontent.com/Chocolate4U/Iran-sing-box-rules/rule-set/geosite-ir.srs",
		UpdateInterval: "168h",
	}

	cfg.RouteRules["r10_dev124_direct"] = managerconfig.RouteRule{
		ID: "r10_dev124_direct", Enabled: true, Name: "Device 124 direct",
		Group: "home", Sources: []string{"192.168.200.124/32"}, Outbound: "direct",
	}
	cfg.RouteRules["r20_dev125_ir_direct"] = managerconfig.RouteRule{
		ID: "r20_dev125_ir_direct", Enabled: true, Name: "Device 125 Iran direct",
		Group: "home", Sources: []string{"192.168.200.125/32"}, RuleSets: []string{"geoip-ir"}, Outbound: "direct",
	}
	cfg.RouteRules["r30_dev125_proxy"] = managerconfig.RouteRule{
		ID: "r30_dev125_proxy", Enabled: true, Name: "Device 125 proxy",
		Group: "home", Sources: []string{"192.168.200.125/32"}, Outbound: "proxy",
	}
	cfg.RouteRules["r40_geoip_direct"] = managerconfig.RouteRule{
		ID: "r40_geoip_direct", Enabled: true, Name: "Iran IP direct",
		Group: "home", RuleSets: []string{"geoip-ir"}, Outbound: "direct",
	}
	cfg.RouteRules["r50_geosite_direct"] = managerconfig.RouteRule{
		ID: "r50_geosite_direct", Enabled: true, Name: "Iran site direct",
		Group: "home", RuleSets: []string{"geosite-ir"}, Outbound: "direct",
	}
	return cfg
}

func TestRenderMatchesGoldenConfig(t *testing.T) {
	cfg := targetConfig()

	got, err := Render(cfg)
	if err != nil {
		t.Fatalf("render: %v", err)
	}

	wantPath := filepath.Join("testdata", "config.golden.json")
	if os.Getenv("UPDATE_GOLDEN") != "" {
		if err := os.WriteFile(wantPath, got, 0644); err != nil {
			t.Fatalf("update golden: %v", err)
		}
	}
	want, err := os.ReadFile(wantPath)
	if err != nil {
		t.Fatalf("read golden: %v", err)
	}
	if string(got) != string(want) {
		t.Fatalf("rendered config mismatch\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

func TestRenderPerSourceDNSRules(t *testing.T) {
	cfg := targetConfig()
	data, err := Render(cfg)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	var document struct {
		DNS dnsConfig `json:"dns"`
	}
	if err := json.Unmarshal(data, &document); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(document.DNS.Servers) != 3 {
		t.Fatalf("dns servers = %d, want 3\n%s", len(document.DNS.Servers), data)
	}
	if document.DNS.Final != "cloudflare_doh" {
		t.Fatalf("dns final = %q, want cloudflare_doh", document.DNS.Final)
	}
	if len(document.DNS.Rules) != 2 {
		t.Fatalf("dns rules = %d, want 2\n%s", len(document.DNS.Rules), data)
	}
	first := document.DNS.Rules[0]
	if first["server"] != "cloudflare_doh" {
		t.Fatalf("dns rule[0] server = %#v, want cloudflare_doh", first["server"])
	}
	if cidrs, ok := first["source_ip_cidr"].([]any); !ok || len(cidrs) != 1 || cidrs[0] != "192.168.200.124/32" {
		t.Fatalf("dns rule[0] source = %#v, want 192.168.200.124/32", first["source_ip_cidr"])
	}
}

func TestRenderDNSServerEmitsExplicitDirectDetour(t *testing.T) {
	cfg := targetConfig()
	data, err := Render(cfg)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	var document struct {
		DNS dnsConfig `json:"dns"`
	}
	if err := json.Unmarshal(data, &document); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	byTag := map[string]map[string]any{}
	for _, server := range document.DNS.Servers {
		byTag[server["tag"].(string)] = server
	}
	if byTag["cloudflare_doh_direct"]["detour"] != "direct" {
		t.Fatalf("cloudflare_doh_direct detour = %#v, want direct", byTag["cloudflare_doh_direct"]["detour"])
	}
	if byTag["cloudflare_doh"]["detour"] != "proxy" {
		t.Fatalf("cloudflare_doh detour = %#v, want proxy", byTag["cloudflare_doh"]["detour"])
	}
	if byTag["cloudflare_doh"]["path"] != "/dns-query" {
		t.Fatalf("cloudflare_doh path = %#v, want /dns-query", byTag["cloudflare_doh"]["path"])
	}
}

func TestRenderCombinedRuleSetAndSourceRoute(t *testing.T) {
	cfg := targetConfig()
	data, err := Render(cfg)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	var document struct {
		Route routeConfig `json:"route"`
	}
	if err := json.Unmarshal(data, &document); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	// Find the combined rule: source 192.168.200.125/32 AND rule_set geoip-ir.
	var combined map[string]any
	for _, rule := range document.Route.Rules {
		_, hasSource := rule["source_ip_cidr"]
		_, hasRuleSet := rule["rule_set"]
		if hasSource && hasRuleSet {
			combined = rule
			break
		}
	}
	if combined == nil {
		t.Fatalf("no combined source+rule_set route rule found\n%s", data)
	}
	if combined["outbound"] != "direct" {
		t.Fatalf("combined rule outbound = %#v, want direct", combined["outbound"])
	}
	if cidrs := combined["source_ip_cidr"].([]any); cidrs[0] != "192.168.200.125/32" {
		t.Fatalf("combined rule source = %#v", combined["source_ip_cidr"])
	}
	if sets := combined["rule_set"].([]any); sets[0] != "geoip-ir" {
		t.Fatalf("combined rule rule_set = %#v", combined["rule_set"])
	}
	if len(document.Route.RuleSet) != 2 {
		t.Fatalf("rule_set definitions = %d, want 2\n%s", len(document.Route.RuleSet), data)
	}
	if document.Route.Final != "proxy" {
		t.Fatalf("route final = %q, want proxy", document.Route.Final)
	}
}

func TestRenderVLESSHTTPUpgradeNode(t *testing.T) {
	cfg := targetConfig()
	data, err := Render(cfg)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	outbound := renderedOutbound(t, data, "vless_0b96da71_f27d0bbdc3")
	if outbound["type"] != "vless" || outbound["uuid"] != "abd4fd93-f23d-4c2e-ac8a-bce395947e6d" {
		t.Fatalf("vless outbound = %#v", outbound)
	}
	transport, ok := outbound["transport"].(map[string]any)
	if !ok || transport["type"] != "httpupgrade" || transport["host"] != "v2.mokhtari94.ir" || transport["path"] != "/87IIO0MKecX3r8BQxdso" {
		t.Fatalf("transport = %#v", outbound["transport"])
	}
	tls, ok := outbound["tls"].(map[string]any)
	if !ok || tls["enabled"] != true || tls["insecure"] != true || tls["server_name"] != "v2.mokhtari94.ir" {
		t.Fatalf("tls = %#v", outbound["tls"])
	}
	alpn, ok := tls["alpn"].([]any)
	if !ok || len(alpn) != 1 || alpn[0] != "http/1.1" {
		t.Fatalf("alpn = %#v", tls["alpn"])
	}
	proxy := renderedOutbound(t, data, "proxy")
	if proxy["type"] != "selector" || proxy["default"] != "vless_0b96da71_f27d0bbdc3" {
		t.Fatalf("proxy selector = %#v", proxy)
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
				RouteFinal:    "proxy",
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
		RouteFinal:    "proxy",
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

func TestRenderManualUnsupportedTransportFallsBackToDirect(t *testing.T) {
	cfg := managerconfig.DefaultConfig()
	cfg.Manager.ActiveGroup = "home"
	cfg.Groups["home"] = managerconfig.Group{
		ID:           "home",
		Enabled:      true,
		Name:         "Home",
		Strategy:     "manual",
		RouteFinal:   "proxy",
		SelectedNode: "node-a",
	}
	cfg.Nodes["node-a"] = managerconfig.Node{ID: "node-a", Enabled: true, Type: "vless", Server: "edge.example.com", Port: 443, UUID: "00000000-0000-0000-0000-000000000001", Transport: "xhttp"}

	// A selected node with a transport sing-box cannot render must not fail the
	// whole render: that would block every reload (including firewall changes).
	// It falls back to a direct selector instead.
	data, err := Render(cfg)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if strings.Contains(string(data), "xhttp") {
		t.Fatalf("rendered config still references the unsupported node:\n%s", data)
	}

	var document struct {
		Outbounds []map[string]any `json:"outbounds"`
	}
	if err := json.Unmarshal(data, &document); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	var proxy map[string]any
	for _, outbound := range document.Outbounds {
		if outbound["tag"] == "proxy" {
			proxy = outbound
			break
		}
	}
	if proxy == nil {
		t.Fatalf("no proxy outbound rendered:\n%s", data)
	}
	if proxy["type"] != "selector" || proxy["default"] != "direct" {
		t.Fatalf("proxy outbound = %v, want direct selector fallback", proxy)
	}
}

func TestRenderRemoteRuleSetOmitsCachePath(t *testing.T) {
	cfg := managerconfig.DefaultConfig()
	cfg.Manager.ActiveGroup = "home"
	cfg.Manager.RuntimeMode = "rule"
	cfg.Groups["home"] = managerconfig.Group{
		ID: "home", Enabled: true, Name: "Home", Strategy: "manual", RouteFinal: "proxy",
	}
	cfg.RuleSets["geoip-ir"] = managerconfig.RuleSet{
		ID: "geoip-ir", Enabled: true, Name: "GeoIP Iran", Type: "remote", Format: "binary",
		URL: "https://example.com/geoip-ir.srs", UpdateInterval: "24h",
	}
	cfg.RouteRules["r1"] = managerconfig.RouteRule{
		ID: "r1", Enabled: true, Name: "Iran direct", Group: "home",
		RuleSets: []string{"geoip-ir"}, Outbound: "direct",
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
	if entry["download_detour"] != "proxy" {
		t.Fatalf("remote rule set download detour = %#v, want proxy", entry["download_detour"])
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
		ID: "home", Enabled: true, Name: "Home", Strategy: "manual", RouteFinal: "proxy",
	}
	cfg.RuleSets["geoip-ir"] = managerconfig.RuleSet{
		ID: "geoip-ir", Enabled: true, Name: "GeoIP Iran", Type: "remote", Format: "binary",
		URL: "https://example.com/geoip-ir.srs", Path: path,
	}
	cfg.RouteRules["r1"] = managerconfig.RouteRule{
		ID: "r1", Enabled: true, Name: "Iran direct", Group: "home",
		RuleSets: []string{"geoip-ir"}, Outbound: "direct",
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

func TestRenderDomainResolverForDirectOutbound(t *testing.T) {
	cfg := managerconfig.DefaultConfig()
	cfg.Manager.ActiveGroup = "home"
	cfg.Groups["home"] = managerconfig.Group{
		ID: "home", Enabled: true, Name: "Home", Strategy: "manual", RouteFinal: "proxy", DNSFinal: "remote_doh",
	}
	cfg.DNSServers["local_udp"] = managerconfig.DNSServer{
		ID: "local_udp", Enabled: true, Type: "udp", Address: "223.5.5.5", Detour: "direct",
	}
	cfg.DNSServers["remote_doh"] = managerconfig.DNSServer{
		ID: "remote_doh", Enabled: true, Type: "doh", Address: "https://1.1.1.1/dns-query", Detour: "proxy",
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
	if document.Route.DefaultDomainResolver != "remote_doh" {
		t.Fatalf("default domain resolver = %q, want remote_doh", document.Route.DefaultDomainResolver)
	}
	direct := renderedOutbound(t, data, "direct")
	if direct["domain_resolver"] != "local_udp" {
		t.Fatalf("direct domain resolver = %#v, want local_udp", direct["domain_resolver"])
	}
}

// DNS hijack renders the hijack-dns route rule but no dedicated DNS inbound:
// the firewall tproxies port 53 into the tproxy inbound, and hijack-dns answers
// it there, so a separate dns-in inbound is redundant.
func TestRenderDNSHijackRouteNoDedicatedInbound(t *testing.T) {
	cfg := managerconfig.DefaultConfig()
	cfg.Manager.ActiveGroup = "home"
	cfg.Transparent.DefaultMode = "tproxy"
	cfg.Transparent.LANIfnames = []string{"br-lan"}
	cfg.Transparent.DNSHijack = true
	cfg.Groups["home"] = managerconfig.Group{
		ID: "home", Enabled: true, Name: "Home", Strategy: "manual", RouteFinal: "proxy",
	}
	cfg.DNSServers["local_udp"] = managerconfig.DNSServer{
		ID: "local_udp", Enabled: true, Type: "udp", Address: "223.5.5.5",
	}

	data, err := Render(cfg)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if strings.Contains(string(data), `"tag": "dns-in"`) {
		t.Fatalf("rendered config still emits the removed dns-in inbound:\n%s", data)
	}
	if !strings.Contains(string(data), `"action": "hijack-dns"`) {
		t.Fatalf("rendered config missing DNS hijack route:\n%s", data)
	}
}

func TestRenderTUNInbound(t *testing.T) {
	cfg := managerconfig.DefaultConfig()
	cfg.Manager.ActiveGroup = "home"
	cfg.Groups["home"] = managerconfig.Group{
		ID: "home", Enabled: true, Name: "Home", Strategy: "manual", RouteFinal: "proxy",
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
}

func TestRenderSupportsAllNodeTypes(t *testing.T) {
	for _, tc := range []struct {
		name string
		node managerconfig.Node
	}{
		{
			name: "hysteria2",
			node: managerconfig.Node{
				ID: "hy2", Enabled: true, Type: "hysteria2", Server: "hy.example.com",
				Port: 443, Password: "secret", TLS: true, SNI: "hy.example.com",
			},
		},
		{
			name: "tuic",
			node: managerconfig.Node{
				ID: "tuic", Enabled: true, Type: "tuic", Server: "tuic.example.com",
				Port: 443, UUID: "00000000-0000-0000-0000-000000000001", Password: "secret",
				TLS: true, Congestion: "bbr", UDPRelayMode: "native",
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := managerconfig.DefaultConfig()
			cfg.Manager.ActiveGroup = "home"
			cfg.Groups["home"] = managerconfig.Group{
				ID: "home", Enabled: true, Name: "Home", Strategy: "manual", RouteFinal: "proxy",
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

func TestRenderPackageDefaultConfig(t *testing.T) {
	cfg, err := managerconfig.Load(filepath.Clean("../../../files/etc/config/singbox-manager"))
	if err != nil {
		t.Fatalf("load package default config: %v", err)
	}
	data, err := Render(*cfg)
	if err != nil {
		t.Fatalf("render package default config: %v", err)
	}
	var document struct {
		Outbounds []map[string]any `json:"outbounds"`
		Route     routeConfig      `json:"route"`
	}
	if err := json.Unmarshal(data, &document); err != nil {
		t.Fatalf("rendered default config is not valid JSON: %v\n%s", err, data)
	}
	renderedOutbound(t, data, "proxy")
	if len(document.Route.RuleSet) == 0 {
		t.Fatalf("expected the default config to render rule sets:\n%s", data)
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

func TestRenderHTTPTransport(t *testing.T) {
	node := managerconfig.Node{
		ID: "n", Type: "vless", Server: "1.2.3.4", Port: 443, UUID: "u",
		TLS: true, Transport: "http", Host: "h.example", Path: "/p",
	}
	out, err := renderNodeOutbound(node)
	if err != nil {
		t.Fatalf("render http transport: %v", err)
	}
	transport, ok := out["transport"].(map[string]any)
	if !ok || transport["type"] != "http" || transport["path"] != "/p" {
		t.Fatalf("transport = %#v", out["transport"])
	}
}

func TestRenderXHTTPTransportUnsupported(t *testing.T) {
	node := managerconfig.Node{
		ID: "vless_5c3b4e91_e3e8460f7f", Type: "vless", Server: "1.2.3.4",
		Port: 443, UUID: "u", TLS: true, Transport: "xhttp",
	}
	_, err := renderNodeOutbound(node)
	if err == nil {
		t.Fatal("expected error for unsupported xhttp transport")
	}
	if !strings.Contains(err.Error(), "xhttp") || !strings.Contains(err.Error(), "not supported") {
		t.Fatalf("error = %q, want a clear xhttp-not-supported message", err)
	}
}
