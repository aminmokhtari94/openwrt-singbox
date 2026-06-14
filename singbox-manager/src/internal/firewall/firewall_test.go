package firewall

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	managerconfig "github.com/openwrt-singbox/singbox-manager/internal/config"
)

func TestRenderTProxyNftablesInclude(t *testing.T) {
	cfg := managerconfig.DefaultConfig()
	cfg.Manager.TProxyPort = 7893
	cfg.Manager.DNSPort = 1053
	cfg.TProxy.Enabled = true
	cfg.TProxy.LANIfnames = []string{"br-lan"}
	cfg.TProxy.IncludeSubnet = []string{"192.168.1.0/24"}
	cfg.TProxy.ExcludeSubnet = []string{"10.0.0.0/8", "fd00::/8"}
	cfg.TProxy.IncludeMAC = []string{"00:11:22:33:44:55"}
	cfg.TProxy.DNSHijack = true

	data, err := Render(cfg)
	if err != nil {
		t.Fatalf("render firewall: %v", err)
	}
	got := string(data)
	for _, want := range []string{
		"chain singbox_manager_tproxy_prerouting",
		`iifname != { "br-lan" } return`,
		"ether saddr != { 00:11:22:33:44:55 } return",
		"meta nfproto ipv4 ip daddr != { 192.168.1.0/24 } return",
		"meta nfproto ipv6 return",
		"udp dport 53 redirect to :1053",
		"meta nfproto ipv4 ip daddr { 10.0.0.0/8 } return",
		"meta nfproto ipv6 ip6 daddr { fd00::/8 } return",
		"meta l4proto { tcp, udp } meta mark set 0x1 tproxy to :7893 accept",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("rendered firewall missing %q:\n%s", want, got)
		}
	}
}

func TestApplyAndCleanupTProxyInclude(t *testing.T) {
	cfg := managerconfig.DefaultConfig()
	cfg.TProxy.Enabled = true
	cfg.TProxy.LANIfnames = []string{"br-lan"}

	path := filepath.Join(t.TempDir(), "90-singbox-manager.nft")
	if err := Apply(cfg, path); err != nil {
		t.Fatalf("apply firewall: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected include file: %v", err)
	}
	cfg.TProxy.Enabled = false
	if err := Apply(cfg, path); err != nil {
		t.Fatalf("cleanup disabled firewall: %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("include file still exists after cleanup: %v", err)
	}
}

func TestRenderTProxyKillSwitchForwardChain(t *testing.T) {
	cfg := managerconfig.DefaultConfig()
	cfg.TProxy.Enabled = true
	cfg.TProxy.KillSwitch = true
	cfg.TProxy.LANIfnames = []string{"eth2"}
	cfg.TProxy.ExcludeSubnet = []string{"192.168.0.0/16"}

	data, err := Render(cfg)
	if err != nil {
		t.Fatalf("render firewall: %v", err)
	}
	got := string(data)
	for _, want := range []string{
		"chain singbox_manager_tproxy_prerouting",
		"chain singbox_manager_kill_switch_forward",
		`iifname != { "eth2" } return`,
		"meta nfproto ipv4 ip daddr { 192.168.0.0/16 } return",
		"counter drop",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("rendered firewall missing %q:\n%s", want, got)
		}
	}
}

func TestRenderKillSwitchOnly(t *testing.T) {
	cfg := managerconfig.DefaultConfig()
	cfg.TProxy.Enabled = true
	cfg.TProxy.KillSwitch = true
	cfg.TProxy.LANIfnames = []string{"eth2"}

	data, err := RenderKillSwitch(cfg)
	if err != nil {
		t.Fatalf("render kill switch: %v", err)
	}
	got := string(data)
	if strings.Contains(got, "singbox_manager_tproxy_prerouting") {
		t.Fatalf("kill switch only render included tproxy chain:\n%s", got)
	}
	if !strings.Contains(got, "chain singbox_manager_kill_switch_forward") || !strings.Contains(got, "counter drop") {
		t.Fatalf("kill switch only render missing drop chain:\n%s", got)
	}
}
