package firewall

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	managerconfig "github.com/openwrt-singbox/singbox-manager/internal/config"
)

func baseConfig() managerconfig.Config {
	cfg := managerconfig.DefaultConfig()
	cfg.Manager.TProxyPort = 7893
	cfg.Manager.DNSPort = 1053
	cfg.Transparent.LANIfnames = []string{"br-lan"}
	return cfg
}

func mustRender(t *testing.T, cfg managerconfig.Config) string {
	t.Helper()
	data, err := Render(cfg)
	if err != nil {
		t.Fatalf("render firewall: %v", err)
	}
	return string(data)
}

func requireAll(t *testing.T, got string, wants ...string) {
	t.Helper()
	for _, want := range wants {
		if !strings.Contains(got, want) {
			t.Fatalf("rendered firewall missing %q:\n%s", want, got)
		}
	}
}

func requireNone(t *testing.T, got string, unwanted ...string) {
	t.Helper()
	for _, bad := range unwanted {
		if strings.Contains(got, bad) {
			t.Fatalf("rendered firewall unexpectedly contains %q:\n%s", bad, got)
		}
	}
}

// Default tproxy: a single catch-all mangle chain, and (crucially) no nat
// `redirect` statement anywhere — DNS is left to sing-box's hijack-dns route
// rule.
func TestRenderDefaultTProxy(t *testing.T) {
	cfg := baseConfig()
	cfg.Transparent.DefaultMode = "tproxy"
	cfg.Transparent.DNSHijack = true

	got := mustRender(t, cfg)
	requireAll(t, got,
		"chain singbox_manager_tproxy {",
		"type filter hook prerouting priority mangle; policy accept;",
		`iifname != { "br-lan" } return`,
		"\tgoto singbox_manager_tproxy_do\n",
		"meta l4proto { tcp, udp } meta mark set 0x1 tproxy to :7893 accept",
	)
	requireNone(t, got,
		"chain singbox_manager_redirect {",
		"redirect", // the whole redirect path is gone
		"dnat",
	)
}

// DNS capture: with hijack enabled, in-scope port-53 traffic must be tproxied
// *before* the local/reserved returns, so DNS aimed at the router itself (the
// default LAN resolver) still reaches sing-box instead of being returned.
func TestRenderDNSCaptureBeforeLocalReturn(t *testing.T) {
	cfg := baseConfig()
	cfg.Transparent.DefaultMode = "tproxy"
	cfg.Transparent.DNSHijack = true

	got := mustRender(t, cfg)
	requireAll(t, got,
		"meta l4proto { tcp, udp } th dport 53 goto singbox_manager_tproxy_do",
	)
	dnsIdx := strings.Index(got, "th dport 53 goto singbox_manager_tproxy_do")
	localIdx := strings.Index(got, "fib daddr type local return")
	reservedIdx := strings.Index(got, "ip daddr "+reservedV4+" return")
	if dnsIdx < 0 || localIdx < 0 || reservedIdx < 0 {
		t.Fatalf("missing expected DNS-capture/return rules:\n%s", got)
	}
	if dnsIdx > localIdx || dnsIdx > reservedIdx {
		t.Fatalf("DNS capture must precede the local/reserved returns:\n%s", got)
	}
}

// Without hijack enabled there is no early port-53 tproxy: DNS aimed at the
// router is returned like any other local traffic.
func TestRenderNoDNSCaptureWhenHijackOff(t *testing.T) {
	cfg := baseConfig()
	cfg.Transparent.DefaultMode = "tproxy"
	cfg.Transparent.DNSHijack = false

	requireNone(t, mustRender(t, cfg), "th dport 53")
}

// Whitelist mode with hijack scopes DNS capture to the tproxy set only.
func TestRenderDNSCaptureWhitelistScoped(t *testing.T) {
	cfg := baseConfig()
	cfg.Transparent.DefaultMode = "off"
	cfg.Transparent.DNSHijack = true
	cfg.Transparent.Devices = []managerconfig.Device{
		{ID: "phone", Enabled: true, IPv4: "192.168.1.50", Mode: "tproxy"},
	}

	got := mustRender(t, cfg)
	requireAll(t, got,
		"ip saddr @singbox_manager_tproxy4 meta l4proto { tcp, udp } th dport 53 goto singbox_manager_tproxy_do",
	)
	// Still no catch-all goto in whitelist mode.
	requireNone(t, got, "\tgoto singbox_manager_tproxy_do\n")
}

// Per-device UDP bypass: default tproxy with one device whose UDP egresses
// directly. The device's address lands in the udpbypass set, and the tproxy
// chain returns its UDP before the catch-all goto (so TCP is still tproxied).
func TestRenderUDPBypass(t *testing.T) {
	cfg := baseConfig()
	cfg.Transparent.DefaultMode = "tproxy"
	cfg.Transparent.Devices = []managerconfig.Device{
		{ID: "console", Enabled: true, IPv4: "192.168.1.50", Mode: "tproxy", BypassUDP: true},
	}

	got := mustRender(t, cfg)
	requireAll(t, got,
		"meta l4proto udp ip saddr @singbox_manager_udpbypass4 return",
		"meta l4proto udp ether saddr @singbox_manager_udpbypass_mac return",
		"\tgoto singbox_manager_tproxy_do\n",
		// the device address lands in the udpbypass set
		"set singbox_manager_udpbypass4 {",
		"elements = { 192.168.1.50 }",
	)

	// The UDP return must come before the catch-all goto so only UDP bypasses;
	// TCP still falls through to the tproxy path.
	udpIdx := strings.Index(got, "meta l4proto udp ip saddr @singbox_manager_udpbypass4 return")
	gotoIdx := strings.Index(got, "\tgoto singbox_manager_tproxy_do\n")
	if udpIdx < 0 || gotoIdx < 0 || udpIdx > gotoIdx {
		t.Fatalf("UDP-bypass return must precede the tproxy goto:\n%s", got)
	}
}

// DNS capture must run *before* the UDP-bypass returns. A UDP-bypass device
// (proxied TCP, direct UDP) still needs its DNS hijacked — DNS is UDP/53, so if
// the udpbypass return fired first the query would leak direct and sing-box
// could not route it. Regression test for that ordering.
func TestRenderDNSCaptureBeforeUDPBypass(t *testing.T) {
	cfg := baseConfig()
	cfg.Transparent.DefaultMode = "tproxy"
	cfg.Transparent.DNSHijack = true
	cfg.Transparent.Devices = []managerconfig.Device{
		{ID: "console", Enabled: true, IPv4: "192.168.1.50", Mode: "tproxy", BypassUDP: true},
	}

	got := mustRender(t, cfg)
	dnsIdx := strings.Index(got, "th dport 53 goto singbox_manager_tproxy_do")
	udpIdx := strings.Index(got, "meta l4proto udp ip saddr @singbox_manager_udpbypass4 return")
	if dnsIdx < 0 || udpIdx < 0 {
		t.Fatalf("missing expected DNS-capture/udpbypass rules:\n%s", got)
	}
	if dnsIdx > udpIdx {
		t.Fatalf("DNS capture must precede the UDP-bypass returns:\n%s", got)
	}
}

// A bypass_udp flag on a non-tproxy device is ignored: a bypassed device is
// already fully direct, so it never enters the udpbypass set.
func TestRenderUDPBypassIgnoredWhenNotTProxy(t *testing.T) {
	cfg := baseConfig()
	cfg.Transparent.DefaultMode = "tproxy"
	cfg.Transparent.Devices = []managerconfig.Device{
		{ID: "tv", Enabled: true, IPv4: "192.168.1.51", Mode: "bypass", BypassUDP: true},
	}

	b := bucketDevices(cfg)
	if len(b.udpBypass4) != 0 {
		t.Fatalf("udpbypass bucket = %v, want empty for a bypass-mode device", b.udpBypass4)
	}
}

// DefaultMode off is a whitelist: the tproxy chain acts only on its explicit
// set, with no catch-all goto.
func TestRenderWhitelist(t *testing.T) {
	cfg := baseConfig()
	cfg.Transparent.DefaultMode = "off"
	cfg.Transparent.Devices = []managerconfig.Device{
		{ID: "phone", Enabled: true, IPv4: "192.168.1.50", Mode: "tproxy"},
		{ID: "tv", Enabled: true, IPv4: "192.168.1.51", Mode: "bypass"},
	}

	got := mustRender(t, cfg)
	requireAll(t, got,
		"ip saddr @singbox_manager_tproxy4 goto singbox_manager_tproxy_do",
		"elements = { 192.168.1.50 }",
	)
	// No catch-all goto in the tproxy chain when proxying is a whitelist.
	requireNone(t, got, "\tgoto singbox_manager_tproxy_do\n")
}

// A device resolves to a tproxy or bypass bucket; a tproxy device that opts out
// of proxied UDP appears in the udpbypass bucket too (so its TCP is still
// proxied while its UDP egresses directly).
func TestBuckets(t *testing.T) {
	cfg := baseConfig()
	cfg.Transparent.DefaultMode = "off"
	cfg.Transparent.Devices = []managerconfig.Device{
		{ID: "a", Enabled: true, MAC: "aa:bb:cc:dd:ee:01", Mode: "tproxy"},
		{ID: "b", Enabled: true, MAC: "aa:bb:cc:dd:ee:02", Mode: "bypass"},
		{ID: "c", Enabled: true, MAC: "aa:bb:cc:dd:ee:03", Mode: "tproxy", BypassUDP: true},
		{ID: "d", Enabled: false, MAC: "aa:bb:cc:dd:ee:04", Mode: "tproxy"},
	}

	b := bucketDevices(cfg)
	assertExactly(t, "tproxy", b.tproxyMAC, "aa:bb:cc:dd:ee:01", "aa:bb:cc:dd:ee:03")
	assertExactly(t, "bypass", b.bypassMAC, "aa:bb:cc:dd:ee:02")
	assertExactly(t, "udpbypass", b.udpBypassMAC, "aa:bb:cc:dd:ee:03")
}

func assertExactly(t *testing.T, label string, got []string, want ...string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("%s bucket = %v, want %v", label, got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("%s bucket = %v, want %v", label, got, want)
		}
	}
}

func TestApplyAndCleanup(t *testing.T) {
	cfg := baseConfig()
	cfg.Transparent.DefaultMode = "tproxy"

	path := filepath.Join(t.TempDir(), "90-singbox-manager.nft")
	if err := Apply(cfg, path); err != nil {
		t.Fatalf("apply firewall: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected include file: %v", err)
	}

	// Disabling all proxying removes the fragment.
	cfg.Transparent.DefaultMode = "off"
	if err := Apply(cfg, path); err != nil {
		t.Fatalf("cleanup disabled firewall: %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("include file still exists after cleanup: %v", err)
	}
}

func TestRenderKillSwitch(t *testing.T) {
	cfg := baseConfig()
	cfg.Transparent.DefaultMode = "tproxy"
	cfg.Transparent.KillSwitch = true

	got := mustRender(t, cfg)
	requireAll(t, got,
		"chain singbox_manager_kill_switch_forward {",
		"type filter hook forward priority filter; policy accept;",
		"counter drop",
	)

	// RenderKillSwitch alone emits the drop chain but not the proxy chains.
	data, err := RenderKillSwitch(cfg)
	if err != nil {
		t.Fatalf("render kill switch: %v", err)
	}
	ks := string(data)
	requireAll(t, ks, "chain singbox_manager_kill_switch_forward {", "counter drop")
	requireNone(t, ks, "chain singbox_manager_tproxy {", "chain singbox_manager_redirect {")
}

// The kill switch must let UDP-bypass devices' UDP through, otherwise the
// "block forwarded traffic" drop would defeat the per-device UDP bypass.
func TestRenderKillSwitchAllowsUDPBypass(t *testing.T) {
	cfg := baseConfig()
	cfg.Transparent.DefaultMode = "tproxy"
	cfg.Transparent.KillSwitch = true
	cfg.Transparent.Devices = []managerconfig.Device{
		{ID: "console", Enabled: true, IPv4: "192.168.1.50", Mode: "tproxy", BypassUDP: true},
	}

	data, err := RenderKillSwitch(cfg)
	if err != nil {
		t.Fatalf("render kill switch: %v", err)
	}
	ks := string(data)
	udpIdx := strings.Index(ks, "meta l4proto udp ip saddr @singbox_manager_udpbypass4 return")
	dropIdx := strings.Index(ks, "counter drop")
	if udpIdx < 0 || dropIdx < 0 || udpIdx > dropIdx {
		t.Fatalf("kill switch must return UDP-bypass traffic before dropping:\n%s", ks)
	}
}

func TestRenderOffReturnsNothing(t *testing.T) {
	cfg := baseConfig()
	cfg.Transparent.DefaultMode = "off"
	data, err := Render(cfg)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if data != nil {
		t.Fatalf("expected nil render when nothing is proxied, got:\n%s", data)
	}
}
