package main

import (
	"net"
	"os"
	"path/filepath"
	"testing"

	managerconfig "github.com/openwrt-singbox/singbox-manager/internal/config"
)

func TestLoadConfigReadsManagerAndActiveGroup(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "singbox-manager")
	config := `
config manager 'main'
	option enabled '1'
	option active_group 'office'
	option runtime_mode 'global'
	option socket_path '/tmp/test.sock'
	option sing_box_bin '/usr/bin/sing-box'

config group 'home'
	option selected_node 'home-node'

config group 'office'
	option selected_node 'office-node'

config node 'home-node'
	option type 'direct'

config node 'office-node'
	option type 'direct'
`

	if err := os.WriteFile(configPath, []byte(config), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := loadConfig(configPath)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	if !cfg.Enabled {
		t.Fatal("expected manager enabled")
	}
	if cfg.ActiveGroup != "office" {
		t.Fatalf("active group = %q, want office", cfg.ActiveGroup)
	}
	if cfg.RuntimeMode != "global" {
		t.Fatalf("runtime mode = %q, want global", cfg.RuntimeMode)
	}
	if cfg.SocketPath != "/tmp/test.sock" {
		t.Fatalf("socket path = %q, want /tmp/test.sock", cfg.SocketPath)
	}
	if cfg.SelectedNode != "office-node" {
		t.Fatalf("selected node = %q, want office-node", cfg.SelectedNode)
	}
}

func TestStatusUnavailableKeepsConfigContext(t *testing.T) {
	cfg := ManagerConfig{
		Enabled:      true,
		ActiveGroup:  "travel",
		RuntimeMode:  "rule",
		SelectedNode: "manual-1",
	}

	status := statusUnavailable(cfg, os.ErrNotExist)
	if status.Daemon {
		t.Fatal("expected daemon offline")
	}
	if !status.ManagerEnabled {
		t.Fatal("expected manager enabled")
	}
	if status.ActiveGroup != "travel" {
		t.Fatalf("active group = %q, want travel", status.ActiveGroup)
	}
	if status.SelectedOutbound != "manual-1" {
		t.Fatalf("selected outbound = %q, want manual-1", status.SelectedOutbound)
	}
}

func TestCmdlineHasArgMatchesExactRuntimeConfig(t *testing.T) {
	cmdline := []byte("/usr/bin/sing-box\x00run\x00-c\x00/var/run/sing-box/config.json\x00")
	if !cmdlineHasArg(cmdline, "/var/run/sing-box/config.json") {
		t.Fatal("expected runtime config argument to match")
	}
	if cmdlineHasArg(cmdline, "/tmp/other-config.json") {
		t.Fatal("unexpected match for other config")
	}
}

func TestImportSubscriptionImportsDirectConfigLink(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "singbox-manager")
	config := `
config manager 'main'
	option active_group 'home'

config group 'home'
	option name 'Home'
	option strategy 'urltest'
`
	if err := os.WriteFile(configPath, []byte(config), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	result := importSubscription(configPath, mustMarshal(map[string]any{
		"request": map[string]any{
			"id":    "paste",
			"name":  "Paste",
			"input": "vless://00000000-0000-0000-0000-000000000001@example.com:443?security=tls&sni=edge.example.com&type=ws#VLESS",
		},
	}))
	if result["ok"] != true {
		t.Fatalf("import result = %#v", result)
	}
	if result["imported"] != 1 {
		t.Fatalf("imported = %#v, want 1", result["imported"])
	}

	cfg, err := managerconfig.Load(configPath)
	if err != nil {
		t.Fatalf("load updated config: %v", err)
	}
	if cfg.Subscriptions["paste"].Enabled {
		t.Fatal("direct config import should create a non-refreshable source")
	}
	if len(cfg.Groups["home"].Subscriptions) != 1 || cfg.Groups["home"].Subscriptions[0] != "paste" {
		t.Fatalf("group subscriptions = %#v, want paste", cfg.Groups["home"].Subscriptions)
	}
	found := false
	for _, node := range cfg.Nodes {
		if node.Subscription == "paste" && node.Type == "vless" && node.Server == "example.com" {
			found = true
		}
	}
	if !found {
		t.Fatalf("imported nodes = %#v", cfg.Nodes)
	}
}

func TestCollectDevicesDropsInfraAndKeepsHosts(t *testing.T) {
	leases := []Device{
		{IP: "192.168.1.10", MAC: "aa:bb:cc:dd:ee:10", Name: "laptop", Source: "dhcp"},
		{IP: "192.168.1.1", MAC: "aa:bb:cc:dd:ee:01", Name: "router", Source: "dhcp"},
	}
	arp := []Device{
		{IP: "192.168.1.20", MAC: "aa:bb:cc:dd:ee:20", Source: "arp"},
		{IP: "192.168.1.10", MAC: "aa:bb:cc:dd:ee:10", Source: "arp"},  // dup of lease
		{IP: "192.168.1.255", MAC: "ff:ff:ff:ff:ff:ff", Source: "arp"}, // broadcast
		{IP: "224.0.0.251", MAC: "01:00:5e:00:00:fb", Source: "arp"},   // multicast
		{IP: "169.254.1.5", MAC: "aa:bb:cc:dd:ee:fe", Source: "arp"},   // link-local
	}
	skip := map[string]bool{
		"192.168.1.1":   true, // gateway / router self
		"192.168.1.255": true, // directed broadcast
	}

	got := collectDevices(leases, arp, skip)
	if len(got) != 2 {
		t.Fatalf("collectDevices returned %d devices, want 2: %#v", len(got), got)
	}
	if got[0].IP != "192.168.1.10" || got[1].IP != "192.168.1.20" {
		t.Fatalf("devices = %#v, want sorted 192.168.1.10, 192.168.1.20", got)
	}
}

func TestBroadcastIP(t *testing.T) {
	cases := []struct {
		cidr string
		want string
	}{
		{"192.168.1.1/24", "192.168.1.255"},
		{"10.0.0.1/8", "10.255.255.255"},
		{"172.16.5.4/30", "172.16.5.7"},
		{"fe80::1/64", ""}, // IPv6 has no broadcast
	}
	for _, tc := range cases {
		ip, ipNet, err := net.ParseCIDR(tc.cidr)
		if err != nil {
			t.Fatalf("parse %q: %v", tc.cidr, err)
		}
		ipNet.IP = ip // use the host address, not the network address
		if got := broadcastIP(ipNet); got != tc.want {
			t.Errorf("broadcastIP(%q) = %q, want %q", tc.cidr, got, tc.want)
		}
	}
}

func TestDefaultGatewaysParsesLittleEndianHex(t *testing.T) {
	dir := t.TempDir()
	routePath := filepath.Join(dir, "route")
	// Default route via 192.168.2.1; the LAN route has no gateway (00000000).
	route := "Iface\tDestination\tGateway\tFlags\tRefCnt\tUse\tMetric\tMask\tMTU\tWindow\tIRTT\n" +
		"eth0\t00000000\t0102A8C0\t0003\t0\t0\t0\t00000000\t0\t0\t0\n" +
		"br-lan\t0002A8C0\t00000000\t0001\t0\t0\t0\t00FFFFFF\t0\t0\t0\n"
	if err := os.WriteFile(routePath, []byte(route), 0644); err != nil {
		t.Fatalf("write route: %v", err)
	}

	gws := defaultGateways(routePath)
	if len(gws) != 1 || gws[0] != "192.168.2.1" {
		t.Fatalf("defaultGateways = %#v, want [192.168.2.1]", gws)
	}
}

func TestIsPickableHostIP(t *testing.T) {
	pickable := []string{"192.168.1.10", "10.0.0.5", "2001:db8::5"}
	for _, ip := range pickable {
		if !isPickableHostIP(ip) {
			t.Errorf("isPickableHostIP(%q) = false, want true", ip)
		}
	}
	rejected := []string{"", "not-an-ip", "0.0.0.0", "127.0.0.1", "255.255.255.255", "224.0.0.1", "169.254.0.1", "fe80::1", "::1"}
	for _, ip := range rejected {
		if isPickableHostIP(ip) {
			t.Errorf("isPickableHostIP(%q) = true, want false", ip)
		}
	}
}

func TestSelectNodeRejectsNodeOutsideActiveGroup(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "singbox-manager")
	config := `
config manager 'main'
	option active_group 'home'

config group 'home'
	option name 'Home'
	list subscription 'home-sub'

config subscription 'home-sub'
	option format 'auto'

config subscription 'office-sub'
	option format 'auto'

config node 'office-node'
	option type 'direct'
	option subscription 'office-sub'
`
	if err := os.WriteFile(configPath, []byte(config), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	result := selectNode(configPath, mustMarshal(map[string]any{"id": "office-node"}))
	if result["ok"] != false {
		t.Fatalf("select result = %#v, want failure", result)
	}
	cfg, err := managerconfig.Load(configPath)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.Groups["home"].SelectedNode != "" {
		t.Fatalf("selected node = %q, want empty", cfg.Groups["home"].SelectedNode)
	}
}
