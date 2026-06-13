package main

import (
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
