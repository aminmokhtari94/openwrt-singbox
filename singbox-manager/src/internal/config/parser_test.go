package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadParsesStrictTypedConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "singbox-manager")
	data := `
config manager 'main'
	option enabled '1'
	option active_group 'home'
	option runtime_mode 'rule'

config group 'home'
	option name 'Home'
	option route_final 'proxy'
	option dns_final 'udp'
	list subscription 'sub'

config subscription 'sub'
	option enabled '0'
	option format 'auto'

config route_rule 'phone'
	option enabled '1'
	option name 'Phone'
	option group 'home'
	list source_ip '192.168.1.20'
	option outbound 'direct'

config route_rule 'iran'
	option enabled '1'
	option name 'Iran'
	option group 'home'
	list ruleset 'local'
	option outbound 'direct'

config dns_rule 'phone_dns'
	option enabled '1'
	option name 'Phone DNS'
	option group 'home'
	list source_ip '192.168.1.20'
	option server 'udp'

config ruleset 'local'
	option type 'local'
	option format 'srs'
	option path '/tmp/local.srs'
	option last_update '2026-06-02T12:00:00Z'

config dns_server 'udp'
	option type 'udp'
	option address '223.5.5.5'

config tproxy 'tproxy'
	option enabled '0'
	option kill_switch '1'

config tun 'tun'
	option enabled '0'
`
	if err := os.WriteFile(path, []byte(data), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	if !cfg.Manager.Enabled {
		t.Fatal("expected manager enabled")
	}
	if cfg.Groups["home"].Subscriptions[0] != "sub" {
		t.Fatalf("subscription ref = %q, want sub", cfg.Groups["home"].Subscriptions[0])
	}
	if cfg.Groups["home"].DNSFinal != "udp" {
		t.Fatalf("group dns_final = %q, want udp", cfg.Groups["home"].DNSFinal)
	}
	if cfg.DNSServers["udp"].Address != "223.5.5.5" {
		t.Fatalf("dns address = %q, want 223.5.5.5", cfg.DNSServers["udp"].Address)
	}
	if cfg.RuleSets["local"].LastUpdate != "2026-06-02T12:00:00Z" {
		t.Fatalf("ruleset last_update = %q", cfg.RuleSets["local"].LastUpdate)
	}
	if cfg.RouteRules["phone"].Sources[0] != "192.168.1.20" || cfg.RouteRules["phone"].Outbound != "direct" {
		t.Fatalf("route rule = %#v", cfg.RouteRules["phone"])
	}
	if cfg.RouteRules["iran"].RuleSets[0] != "local" {
		t.Fatalf("route rule ruleset = %#v", cfg.RouteRules["iran"].RuleSets)
	}
	if cfg.DNSRules["phone_dns"].Server != "udp" {
		t.Fatalf("dns rule server = %q, want udp", cfg.DNSRules["phone_dns"].Server)
	}
	if !cfg.TProxy.KillSwitch {
		t.Fatal("expected tproxy kill switch")
	}
}

func TestLoadPackageDefaultConfig(t *testing.T) {
	path := filepath.Clean("../../../files/etc/config/singbox-manager")
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("load package default config: %v", err)
	}
	if cfg.Manager.ActiveGroup != "home" {
		t.Fatalf("active group = %q, want home", cfg.Manager.ActiveGroup)
	}
	if cfg.Manager.MixedListen != "0.0.0.0" {
		t.Fatalf("mixed listen = %q, want 0.0.0.0", cfg.Manager.MixedListen)
	}
	if _, ok := cfg.Groups["home"]; !ok {
		t.Fatal("expected built-in home group")
	}
	if cfg.RuleSets["geoip-ir"].URL == "" {
		t.Fatal("expected built-in Iran ruleset URL")
	}
}

func TestLoadAcceptsLegacySingletonSectionNames(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "singbox-manager")
	data := `
config manager 'main'
	option active_group 'home'

config group 'home'
	option name 'Home'

config tproxy 'tproxy'
	option enabled '0'

config tun 'tun'
	option enabled '0'
`
	if err := os.WriteFile(path, []byte(data), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	if _, err := Load(path); err != nil {
		t.Fatalf("load legacy singleton names: %v", err)
	}
}

func TestSelectNodeUpdatesGroupSelectedNode(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "singbox-manager")
	data := `
config manager 'main'
	option active_group 'home'

config group 'home'
	option name 'Home'
	option strategy 'urltest'
	option selected_node 'old'

config node 'old'
	option type 'direct'

config node 'new'
	option type 'direct'
`
	if err := os.WriteFile(path, []byte(data), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	if err := SelectNode(path, "home", "new"); err != nil {
		t.Fatalf("select node: %v", err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("load updated config: %v", err)
	}
	if cfg.Groups["home"].SelectedNode != "new" {
		t.Fatalf("selected node = %q, want new", cfg.Groups["home"].SelectedNode)
	}
	if cfg.Groups["home"].Strategy != "manual" {
		t.Fatalf("strategy = %q, want manual", cfg.Groups["home"].Strategy)
	}
}

func TestSetManagerEnabledUpdatesMainSection(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "singbox-manager")
	data := `
config manager 'main'
	option enabled '0'
	option active_group 'home'

config group 'home'
	option name 'Home'
`
	if err := os.WriteFile(path, []byte(data), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	if err := SetManagerEnabled(path, true); err != nil {
		t.Fatalf("set manager enabled: %v", err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("load updated config: %v", err)
	}
	if !cfg.Manager.Enabled {
		t.Fatal("expected manager enabled")
	}
}

func TestSetManagerRuntimeModeUpdatesMainSection(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "singbox-manager")
	data := `
config manager 'main'
	option active_group 'home'
	option runtime_mode 'rule'

config group 'home'
	option name 'Home'
`
	if err := os.WriteFile(path, []byte(data), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	if err := SetManagerRuntimeMode(path, "global"); err != nil {
		t.Fatalf("set runtime mode: %v", err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("load updated config: %v", err)
	}
	if cfg.Manager.RuntimeMode != "global" {
		t.Fatalf("runtime mode = %q, want global", cfg.Manager.RuntimeMode)
	}
	if err := SetManagerRuntimeMode(path, "bogus"); err == nil {
		t.Fatal("expected invalid runtime mode error")
	}
}

func TestLoadRejectsMalformedUCI(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "singbox-manager")
	data := `
config manager 'main'
	option active_group 'home
`
	if err := os.WriteFile(path, []byte(data), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	_, err := Load(path)
	if err == nil {
		t.Fatal("expected malformed UCI error")
	}
	if !strings.Contains(err.Error(), "unterminated quoted string") {
		t.Fatalf("error = %q, want unterminated quote", err)
	}
}

func TestValidateRejectsTProxyAndTUNConflict(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Groups["home"] = Group{ID: "home", Enabled: true, Name: "Home", Strategy: "manual", RouteFinal: "proxy"}
	cfg.TProxy.Enabled = true
	cfg.TUN.Enabled = true

	err := Validate(cfg)
	if err == nil {
		t.Fatal("expected validation error")
	}
	if !strings.Contains(err.Error(), "cannot both be enabled") {
		t.Fatalf("error = %q, want mutual exclusion", err)
	}
}

func TestValidateAcceptsRouteAndDNSRules(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Groups["home"] = Group{
		ID: "home", Enabled: true, Name: "Home", Strategy: "manual", RouteFinal: "proxy", DNSFinal: "udp",
	}
	cfg.DNSServers["udp"] = DNSServer{ID: "udp", Enabled: true, Name: "UDP", Type: "udp", Address: "1.1.1.1"}
	cfg.RuleSets["geoip-ir"] = RuleSet{ID: "geoip-ir", Enabled: true, Name: "GeoIP", Type: "remote", Format: "binary", URL: "https://example.com/geoip-ir.srs"}
	cfg.RouteRules["dev"] = RouteRule{ID: "dev", Enabled: true, Name: "Device", Group: "home", Sources: []string{"192.168.1.20"}, RuleSets: []string{"geoip-ir"}, Outbound: "direct"}
	cfg.DNSRules["dev_dns"] = DNSRule{ID: "dev_dns", Enabled: true, Name: "Device DNS", Group: "home", Sources: []string{"192.168.1.20"}, Server: "udp"}

	if err := Validate(cfg); err != nil {
		t.Fatalf("validate route and dns rules: %v", err)
	}
}

func TestValidateRejectsRuleWithMissingReferences(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Groups["home"] = Group{ID: "home", Enabled: true, Name: "Home", Strategy: "manual", RouteFinal: "proxy"}
	cfg.RouteRules["bad"] = RouteRule{ID: "bad", Enabled: true, Name: "Bad", Group: "missing", Sources: []string{"192.168.1.20"}, Outbound: "direct"}
	cfg.DNSRules["bad_dns"] = DNSRule{ID: "bad_dns", Enabled: true, Name: "Bad DNS", Group: "home", Sources: []string{"192.168.1.20"}, Server: "missing"}

	err := Validate(cfg)
	if err == nil {
		t.Fatal("expected validation error")
	}
	for _, want := range []string{
		"route_rule.bad.group references missing group",
		"dns_rule.bad_dns.server references missing dns_server",
	} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error = %q, want %q", err, want)
		}
	}
}

func TestLoadUnvalidatedReturnsInvalidRows(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "singbox-manager")
	data := `
config manager 'main'
	option active_group 'home'

config group 'home'
	option name 'Home'

config route_rule 'device_124'
	option enabled '1'
	option name 'Device'
	option group 'home'
	list source_ip '192.168.200.124'
	option outbound 'direct'

config dns_rule 'device_124_dns'
	option enabled '1'
	option name 'Device DNS'
	option group 'home'
	list source_ip '192.168.200.124'
	option server 'missing'
`
	if err := os.WriteFile(path, []byte(data), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	if _, err := Load(path); err == nil {
		t.Fatal("expected validated load to fail")
	}
	cfg, err := LoadUnvalidated(path)
	if err != nil {
		t.Fatalf("load unvalidated config: %v", err)
	}
	if _, ok := cfg.RouteRules["device_124"]; !ok {
		t.Fatal("expected route rule to remain visible")
	}
	if _, ok := cfg.DNSRules["device_124_dns"]; !ok {
		t.Fatal("expected invalid dns rule to remain visible")
	}
}

func TestValidateRejectsInvalidTProxyFilters(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Groups["home"] = Group{ID: "home", Enabled: true, Name: "Home", Strategy: "manual", RouteFinal: "proxy"}
	cfg.TProxy.Enabled = true
	cfg.TProxy.IncludeSubnet = []string{"not-a-cidr"}
	cfg.TProxy.ExcludeSubnet = []string{"192.168.1.1"}
	cfg.TProxy.IncludeMAC = []string{"not-a-mac"}

	err := Validate(cfg)
	if err == nil {
		t.Fatal("expected validation error")
	}
	for _, want := range []string{
		"tproxy.main.lan_ifname is required",
		"tproxy.main.include_subnet is invalid",
		"tproxy.main.exclude_subnet is invalid",
		"tproxy.main.include_mac is invalid",
	} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error = %q, want %q", err, want)
		}
	}
}

func TestValidateRejectsInvalidTUNAddresses(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Groups["home"] = Group{ID: "home", Enabled: true, Name: "Home", Strategy: "manual", RouteFinal: "proxy"}
	cfg.TUN.Inet4Address = "fdfe:dcba:9876::1/126"
	cfg.TUN.Inet6Address = "172.19.0.1/30"

	err := Validate(cfg)
	if err == nil {
		t.Fatal("expected validation error")
	}
	for _, want := range []string{
		"tun.main.inet4_address is invalid",
		"tun.main.inet6_address is invalid",
	} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error = %q, want %q", err, want)
		}
	}
}

func TestReplaceSubscriptionNodesKeepsManualNodes(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "singbox-manager")
	data := `
config manager 'main'
	option active_group 'home'

config group 'home'
	option selected_node 'manual'
	list subscription 'sub'

config subscription 'sub'
	option enabled '1'
	option url 'https://example.com/sub'
	option format 'plain'

config node 'old-sub-node'
	option type 'trojan'
	option server 'old.example.com'
	option port '443'
	option password 'old'
	option subscription 'sub'

config node 'manual'
	option type 'direct'
`
	if err := os.WriteFile(path, []byte(data), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	err := ReplaceSubscriptionNodes(path, "sub", []Node{{
		ID:           "new-sub-node",
		Enabled:      true,
		Name:         "New",
		Type:         "trojan",
		Server:       "new.example.com",
		Port:         443,
		Password:     "secret",
		Subscription: "sub",
	}})
	if err != nil {
		t.Fatalf("replace subscription nodes: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("load updated config: %v", err)
	}
	if _, ok := cfg.Nodes["old-sub-node"]; ok {
		t.Fatal("old subscription node still exists")
	}
	if _, ok := cfg.Nodes["manual"]; !ok {
		t.Fatal("manual node was removed")
	}
	if cfg.Nodes["new-sub-node"].Subscription != "sub" {
		t.Fatalf("new node subscription = %q, want sub", cfg.Nodes["new-sub-node"].Subscription)
	}
	if cfg.Subscriptions["sub"].Health != "ok" {
		t.Fatalf("subscription health = %q, want ok", cfg.Subscriptions["sub"].Health)
	}
}

func TestUpsertSubscriptionAttachesActiveGroup(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "singbox-manager")
	data := `
config manager 'main'
	option active_group 'home'

config group 'home'
	option name 'Home'
	option strategy 'urltest'
`
	if err := os.WriteFile(path, []byte(data), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	err := UpsertSubscription(path, Subscription{
		ID:      "sub",
		Enabled: true,
		Name:    "Sub",
		URL:     "https://example.com/sub",
		Format:  "auto",
	}, "home")
	if err != nil {
		t.Fatalf("upsert subscription: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("load updated config: %v", err)
	}
	if cfg.Subscriptions["sub"].URL != "https://example.com/sub" {
		t.Fatalf("subscription url = %q", cfg.Subscriptions["sub"].URL)
	}
	if len(cfg.Groups["home"].Subscriptions) != 1 || cfg.Groups["home"].Subscriptions[0] != "sub" {
		t.Fatalf("group subscriptions = %#v, want sub", cfg.Groups["home"].Subscriptions)
	}
}

func TestDeleteSubscriptionRemovesImportedNodesAndGroupReference(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "singbox-manager")
	data := `
config manager 'main'
	option active_group 'home'

config group 'home'
	option name 'Home'
	list subscription 'sub'

config subscription 'sub'
	option enabled '1'
	option url 'https://example.com/sub'

config node 'from-sub'
	option type 'direct'
	option subscription 'sub'

config node 'manual'
	option type 'direct'
`
	if err := os.WriteFile(path, []byte(data), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	if err := DeleteSubscription(path, "sub"); err != nil {
		t.Fatalf("delete subscription: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("load updated config: %v", err)
	}
	if _, ok := cfg.Subscriptions["sub"]; ok {
		t.Fatal("subscription still exists")
	}
	if _, ok := cfg.Nodes["from-sub"]; ok {
		t.Fatal("subscription node still exists")
	}
	if _, ok := cfg.Nodes["manual"]; !ok {
		t.Fatal("manual node was removed")
	}
	if len(cfg.Groups["home"].Subscriptions) != 0 {
		t.Fatalf("group subscriptions = %#v, want empty", cfg.Groups["home"].Subscriptions)
	}
}

func TestDeleteDNSServerCleansRulesAndFinal(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "singbox-manager")
	data := `
config manager 'main'
	option active_group 'home'

config group 'home'
	option name 'Home'
	option dns_final 'udp'

config dns_server 'udp'
	option type 'udp'
	option address '1.1.1.1'

config dns_server 'doh'
	option type 'doh'
	option address 'https://dns.example/dns-query'
	option detour 'proxy'

config dns_rule 'phone'
	option group 'home'
	list source_ip '192.168.1.20'
	option server 'udp'
`
	if err := os.WriteFile(path, []byte(data), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	if err := DeleteDNSServer(path, "udp"); err != nil {
		t.Fatalf("delete dns server: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("load updated config: %v", err)
	}
	if _, ok := cfg.DNSServers["udp"]; ok {
		t.Fatal("dns server still exists")
	}
	if _, ok := cfg.DNSRules["phone"]; ok {
		t.Fatal("dependent dns rule should be removed")
	}
	if cfg.Groups["home"].DNSFinal != "" {
		t.Fatalf("group dns_final = %q, want empty", cfg.Groups["home"].DNSFinal)
	}
}

func TestRuleSetCRUDCleansRuleReferences(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "singbox-manager")
	data := `
config manager 'main'
	option active_group 'home'

config group 'home'
	option name 'Home'

config route_rule 'iran'
	option group 'home'
	list ruleset 'geoip-ir'
	option outbound 'direct'

config ruleset 'geoip_ir'
	option id 'geoip-ir'
	option enabled '1'
	option type 'remote'
	option format 'srs'
	option url 'https://example.com/geoip-ir.srs'
	option last_update '2026-06-02T12:00:00Z'
`
	if err := os.WriteFile(path, []byte(data), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	if err := UpsertRuleSet(path, RuleSet{ID: "geoip-ir", Enabled: true, Name: "GeoIP Iran", Type: "remote", Format: "srs", URL: "https://mirror.example/geoip-ir.srs"}); err != nil {
		t.Fatalf("upsert ruleset: %v", err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("load updated config: %v", err)
	}
	if cfg.RuleSets["geoip-ir"].URL != "https://mirror.example/geoip-ir.srs" {
		t.Fatalf("ruleset url = %q", cfg.RuleSets["geoip-ir"].URL)
	}
	if cfg.RuleSets["geoip-ir"].LastUpdate != "2026-06-02T12:00:00Z" {
		t.Fatalf("ruleset last_update = %q", cfg.RuleSets["geoip-ir"].LastUpdate)
	}

	if err := DeleteRuleSet(path, "geoip-ir"); err != nil {
		t.Fatalf("delete ruleset: %v", err)
	}
	cfg, err = Load(path)
	if err != nil {
		t.Fatalf("load after ruleset delete: %v", err)
	}
	if _, ok := cfg.RuleSets["geoip-ir"]; ok {
		t.Fatal("ruleset still exists")
	}
	// The route rule only matched the deleted ruleset, so it is dropped too.
	if _, ok := cfg.RouteRules["iran"]; ok {
		t.Fatal("matcher-less route rule should be removed")
	}
}

func TestRouteRuleCRUD(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "singbox-manager")
	data := `
config manager 'main'
	option active_group 'home'

config group 'home'
	option name 'Home'
`
	if err := os.WriteFile(path, []byte(data), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	if err := UpsertRouteRule(path, RouteRule{ID: "dev", Enabled: true, Name: "Device", Group: "home", Sources: []string{"192.168.1.20"}, Outbound: "direct"}); err != nil {
		t.Fatalf("upsert route rule: %v", err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("load updated config: %v", err)
	}
	if cfg.RouteRules["dev"].Outbound != "direct" || cfg.RouteRules["dev"].Sources[0] != "192.168.1.20" {
		t.Fatalf("route rule = %#v", cfg.RouteRules["dev"])
	}

	if err := UpsertRouteRule(path, RouteRule{ID: "dev", Enabled: true, Name: "Device", Group: "home", Sources: []string{"192.168.1.30"}, Outbound: "proxy"}); err != nil {
		t.Fatalf("update route rule: %v", err)
	}
	cfg, _ = Load(path)
	if cfg.RouteRules["dev"].Outbound != "proxy" || cfg.RouteRules["dev"].Sources[0] != "192.168.1.30" {
		t.Fatalf("updated route rule = %#v", cfg.RouteRules["dev"])
	}

	if err := DeleteRouteRule(path, "dev"); err != nil {
		t.Fatalf("delete route rule: %v", err)
	}
	cfg, _ = Load(path)
	if _, ok := cfg.RouteRules["dev"]; ok {
		t.Fatal("route rule still exists")
	}
}

func TestSetGroupSettingsUpdatesGroup(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "singbox-manager")
	data := `
config manager 'main'
	option active_group 'home'

config group 'home'
	option name 'Home'
	option strategy 'manual'
	option route_final 'proxy'
	option health 'ok'

config dns_server 'udp'
	option type 'udp'
	option address '1.1.1.1'

config subscription 'sub'
	option enabled '1'
	option url 'https://example.com/sub'
`
	if err := os.WriteFile(path, []byte(data), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	err := SetGroupSettings(path, Group{
		ID:            "home",
		Name:          "Home Net",
		Strategy:      "urltest",
		RouteFinal:    "direct",
		DNSFinal:      "udp",
		Subscriptions: []string{"sub"},
	})
	if err != nil {
		t.Fatalf("set group settings: %v", err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("load updated config: %v", err)
	}
	group := cfg.Groups["home"]
	if group.Name != "Home Net" || group.Strategy != "urltest" || group.RouteFinal != "direct" || group.DNSFinal != "udp" {
		t.Fatalf("group = %#v", group)
	}
	if len(group.Subscriptions) != 1 || group.Subscriptions[0] != "sub" {
		t.Fatalf("group subscriptions = %#v, want sub", group.Subscriptions)
	}
	if group.Health != "ok" {
		t.Fatalf("group health = %q, want preserved ok", group.Health)
	}
}
