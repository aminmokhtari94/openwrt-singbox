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
	option routing_profile 'routes'
	option dns_profile 'dns'
	list subscription 'sub'

config subscription 'sub'
	option enabled '0'
	option format 'auto'

config routing_profile 'routes'
	list ruleset 'local'
	option final 'proxy'

config source_rule 'phone'
	option enabled '1'
	option name 'Phone'
	option profile 'routes'
	list source_ip '192.168.1.20'
	option outbound 'direct'

config ruleset 'local'
	option type 'local'
	option format 'srs'
	option path '/tmp/local.srs'
	option last_update '2026-06-02T12:00:00Z'

config dns_profile 'dns'
	list server 'udp'

config dns_server 'udp'
	option type 'udp'
	option address '223.5.5.5'

config pac 'pac'
	option enabled '1'
	option source 'custom'
	option selected_custom 'office'
	option local_bypass '1'
	list whitelist '.lan'
	list blacklist '.blocked.example'
	list custom_rule 'direct intranet.example'

config pac_custom 'office'
	option enabled '1'
	option name 'Office'
	option content 'function FindProxyForURL(url, host) { return "DIRECT"; }'

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
	if cfg.DNSServers["udp"].Address != "223.5.5.5" {
		t.Fatalf("dns address = %q, want 223.5.5.5", cfg.DNSServers["udp"].Address)
	}
	if cfg.RuleSets["local"].LastUpdate != "2026-06-02T12:00:00Z" {
		t.Fatalf("ruleset last_update = %q", cfg.RuleSets["local"].LastUpdate)
	}
	if cfg.SourceRules["phone"].Sources[0] != "192.168.1.20" || cfg.SourceRules["phone"].Outbound != "direct" {
		t.Fatalf("source rule = %#v", cfg.SourceRules["phone"])
	}
	if !cfg.PAC.Enabled || cfg.PAC.Source != "custom" || cfg.PAC.SelectedCustom != "office" || cfg.PAC.Whitelist[0] != ".lan" || cfg.PAC.Blacklist[0] != ".blocked.example" {
		t.Fatalf("PAC = %#v", cfg.PAC)
	}
	if cfg.CustomPACs["office"].Content == "" {
		t.Fatal("expected custom PAC content")
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
	if cfg.ActiveDNSProfile() == nil {
		t.Fatal("expected active DNS profile")
	}
	if _, ok := cfg.Routing["china_direct"]; !ok {
		t.Fatal("expected built-in China Direct routing profile")
	}
	if _, ok := cfg.Routing["russia_direct"]; !ok {
		t.Fatal("expected built-in Russia Direct routing profile")
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

config pac 'pac'
	option enabled '0'

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
	cfg.Groups["home"] = Group{ID: "home", Enabled: true, Name: "Home", Strategy: "manual"}
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

func TestValidateAcceptsDNSOnlySourceRule(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Groups["home"] = Group{
		ID:             "home",
		Enabled:        true,
		Name:           "Home",
		RoutingProfile: "routes",
		Strategy:       "manual",
	}
	cfg.Routing["routes"] = RoutingProfile{
		ID:      "routes",
		Enabled: true,
		Name:    "Routes",
		Mode:    "rule",
		Final:   "proxy",
	}
	cfg.SourceRules["phone_dns"] = SourceRule{
		ID:       "phone_dns",
		Enabled:  true,
		Name:     "Phone DNS",
		Profile:  "routes",
		Sources:  []string{"192.168.1.20"},
		Outbound: "dns",
	}

	if err := Validate(cfg); err != nil {
		t.Fatalf("validate DNS-only source rule: %v", err)
	}
}

func TestValidateRejectsInvalidTProxyFilters(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Groups["home"] = Group{ID: "home", Enabled: true, Name: "Home", Strategy: "manual"}
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
	cfg.Groups["home"] = Group{ID: "home", Enabled: true, Name: "Home", Strategy: "manual"}
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

func TestSubscriptionRefreshErrorIsSeparateFromHealth(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "singbox-manager")
	data := `
config manager 'main'
	option active_group 'home'

config group 'home'
	option name 'Home'

config subscription 'sub'
	option enabled '1'
	option url 'https://example.com/sub'
	option health 'ok'
	option last_error 'old'
`
	if err := os.WriteFile(path, []byte(data), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	if err := MarkSubscriptionError(path, "sub", "fetch failed\nwith details"); err != nil {
		t.Fatalf("mark subscription error: %v", err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("load failed config: %v", err)
	}
	if cfg.Subscriptions["sub"].Health != "ok" {
		t.Fatalf("subscription health = %q, want ok", cfg.Subscriptions["sub"].Health)
	}
	if cfg.Subscriptions["sub"].LastError != "fetch failed" {
		t.Fatalf("subscription last_error = %q, want fetch failed", cfg.Subscriptions["sub"].LastError)
	}

	if err := ReplaceSubscriptionNodes(path, "sub", []Node{{ID: "node", Enabled: true, Type: "direct", Subscription: "sub"}}); err != nil {
		t.Fatalf("replace subscription nodes: %v", err)
	}
	cfg, err = Load(path)
	if err != nil {
		t.Fatalf("load refreshed config: %v", err)
	}
	if cfg.Subscriptions["sub"].LastError != "" {
		t.Fatalf("subscription last_error = %q, want empty", cfg.Subscriptions["sub"].LastError)
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

func TestDNSCRUDCleansReferences(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "singbox-manager")
	data := `
config manager 'main'
	option active_group 'home'

config group 'home'
	option name 'Home'
	option dns_profile 'dns'

config dns_profile 'dns'
	option name 'DNS'
	list server 'udp'

config dns_server 'udp'
	option type 'udp'
	option address '1.1.1.1'
`
	if err := os.WriteFile(path, []byte(data), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	if err := UpsertDNSServer(path, DNSServer{ID: "doh", Enabled: true, Name: "DoH", Type: "doh", Address: "https://dns.example/dns-query", Detour: "proxy"}); err != nil {
		t.Fatalf("upsert dns server: %v", err)
	}
	if err := UpsertDNSProfile(path, DNSProfile{ID: "dns", Enabled: true, Name: "DNS", Mode: "split", Servers: []string{"udp", "doh"}, Hijack: true}); err != nil {
		t.Fatalf("upsert dns profile: %v", err)
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
	if len(cfg.DNSProfiles["dns"].Servers) != 1 || cfg.DNSProfiles["dns"].Servers[0] != "doh" {
		t.Fatalf("dns profile servers = %#v, want doh", cfg.DNSProfiles["dns"].Servers)
	}

	if err := DeleteDNSProfile(path, "dns"); err != nil {
		t.Fatalf("delete dns profile: %v", err)
	}
	cfg, err = Load(path)
	if err != nil {
		t.Fatalf("load after profile delete: %v", err)
	}
	if _, ok := cfg.DNSProfiles["dns"]; ok {
		t.Fatal("dns profile still exists")
	}
	if cfg.Groups["home"].DNSProfile != "" {
		t.Fatalf("group dns profile = %q, want empty", cfg.Groups["home"].DNSProfile)
	}
}

func TestRuleSetCRUDCleansRoutingReferences(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "singbox-manager")
	data := `
config manager 'main'
	option active_group 'home'

config group 'home'
	option name 'Home'
	option routing_profile 'routes'

config routing_profile 'routes'
	list ruleset 'geoip-ir'
	option final 'proxy'

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
	if len(cfg.Routing["routes"].RuleSets) != 0 {
		t.Fatalf("routing rulesets = %#v, want empty", cfg.Routing["routes"].RuleSets)
	}
}

func TestRoutingProfileCRUDCleansGroupReferences(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "singbox-manager")
	data := `
config manager 'main'
	option active_group 'home'

config group 'home'
	option name 'Home'
	option routing_profile 'routes'

config ruleset 'geoip_ir'
	option id 'geoip-ir'
	option enabled '1'
	option type 'remote'
	option format 'srs'
	option url 'https://example.com/geoip-ir.srs'

config routing_profile 'routes'
	option enabled '1'
	option name 'Routes'
	option mode 'rule'
	list ruleset 'geoip-ir'
	option final 'proxy'
`
	if err := os.WriteFile(path, []byte(data), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	if err := UpsertRoutingProfile(path, RoutingProfile{ID: "routes", Enabled: true, Name: "Updated Routes", Mode: "rule", RuleSets: []string{"geoip-ir"}, Final: "direct"}); err != nil {
		t.Fatalf("upsert routing profile: %v", err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("load updated config: %v", err)
	}
	if cfg.Routing["routes"].Name != "Updated Routes" {
		t.Fatalf("routing profile name = %q", cfg.Routing["routes"].Name)
	}
	if cfg.Routing["routes"].Final != "direct" {
		t.Fatalf("routing profile final = %q", cfg.Routing["routes"].Final)
	}

	if err := DeleteRoutingProfile(path, "routes"); err != nil {
		t.Fatalf("delete routing profile: %v", err)
	}
	cfg, err = Load(path)
	if err != nil {
		t.Fatalf("load after routing profile delete: %v", err)
	}
	if _, ok := cfg.Routing["routes"]; ok {
		t.Fatal("routing profile still exists")
	}
	if cfg.Groups["home"].RoutingProfile != "" {
		t.Fatalf("group routing_profile = %q, want empty", cfg.Groups["home"].RoutingProfile)
	}
}
