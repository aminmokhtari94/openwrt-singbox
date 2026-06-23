package runtime

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	managerconfig "github.com/openwrt-singbox/singbox-manager/internal/config"
	"github.com/openwrt-singbox/singbox-manager/internal/firewall"
)

func TestApplyFirewallFlushesManagerSetsBeforeReload(t *testing.T) {
	cfg := managerconfig.DefaultConfig()
	cfg.Transparent.DefaultMode = "tproxy"
	cfg.Transparent.LANIfnames = []string{"eth2"}

	var commands []string
	oldRouteCommand := routeCommand
	routeCommand = func(args ...string) error {
		command := strings.Join(args, " ")
		commands = append(commands, command)
		// A set that does not exist yet must not abort the apply.
		if strings.HasPrefix(command, "nft flush set") {
			return errors.New("exit status 1: Error: No such file or directory")
		}
		if strings.Contains(command, " rule del ") {
			return errors.New("exit status 2: RTNETLINK answers: No such process")
		}
		return nil
	}
	t.Cleanup(func() { routeCommand = oldRouteCommand })

	reloadCalled := false
	oldFirewallReload := FirewallReload
	FirewallReload = func() error { reloadCalled = true; return nil }
	t.Cleanup(func() { FirewallReload = oldFirewallReload })

	path := filepath.Join(t.TempDir(), "90-singbox-manager.nft")
	result := Result{}
	if err := applyFirewall(cfg, Paths{NftablesInclude: path}, &result); err != nil {
		t.Fatalf("apply firewall: %v", err)
	}
	if !reloadCalled {
		t.Fatal("firewall reload was not called")
	}
	for _, name := range firewall.ManagedSets {
		want := "nft flush set " + firewall.FW4Table + " " + name
		found := false
		for _, command := range commands {
			if command == want {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("missing flush for set %q; commands = %#v", name, commands)
		}
	}
}

// A missing nft_tproxy module must surface an actionable error naming the
// kmod-nft-tproxy package, not the opaque modprobe/fw4 failure.
func TestApplyFirewallReportsMissingTProxyModule(t *testing.T) {
	cfg := managerconfig.DefaultConfig()
	cfg.Transparent.DefaultMode = "tproxy"
	cfg.Transparent.LANIfnames = []string{"eth2"}

	oldRouteCommand := routeCommand
	routeCommand = func(args ...string) error {
		if strings.Join(args, " ") == "modprobe nft_tproxy" {
			return errors.New("exit status 1: modprobe: module nft_tproxy not found")
		}
		return nil
	}
	t.Cleanup(func() { routeCommand = oldRouteCommand })

	path := filepath.Join(t.TempDir(), "90-singbox-manager.nft")
	result := Result{}
	err := applyFirewall(cfg, Paths{NftablesInclude: path}, &result)
	if err == nil {
		t.Fatal("expected error when nft_tproxy module is missing")
	}
	if !strings.Contains(err.Error(), "kmod-nft-tproxy") {
		t.Fatalf("error should name the kmod-nft-tproxy package, got: %v", err)
	}
}

func TestStatusAliveTreatsZombieAsStopped(t *testing.T) {
	alive, ok := statusAlive([]byte("Name:\tsing-box\nState:\tZ (zombie)\n"))
	if !ok {
		t.Fatal("expected state line")
	}
	if alive {
		t.Fatal("expected zombie process to be stopped")
	}

	alive, ok = statusAlive([]byte("Name:\tsing-box\nState:\tS (sleeping)\n"))
	if !ok {
		t.Fatal("expected state line")
	}
	if !alive {
		t.Fatal("expected sleeping process to be alive")
	}
}

func TestRemovePIDFileIfMatches(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sing-box.pid")
	if err := os.WriteFile(path, []byte("123\n"), 0644); err != nil {
		t.Fatalf("write pid file: %v", err)
	}

	removePIDFileIfMatches(path, 456)
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("pid file removed for non-matching pid: %v", err)
	}

	removePIDFileIfMatches(path, 123)
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("pid file exists after matching remove: %v", err)
	}
}

func TestApplyTProxyRoutesAddsPolicyRoutes(t *testing.T) {
	var commands []string
	oldRouteCommand := routeCommand
	routeCommand = func(args ...string) error {
		command := strings.Join(args, " ")
		commands = append(commands, command)
		if strings.Contains(command, " rule del ") {
			return errors.New("exit status 2: RTNETLINK answers: No such process")
		}
		return nil
	}
	t.Cleanup(func() { routeCommand = oldRouteCommand })

	if err := applyTProxyRoutes(); err != nil {
		t.Fatalf("apply tproxy routes: %v", err)
	}
	want := []string{
		"ip -4 route add local 0.0.0.0/0 dev lo table 100",
		"ip -6 route add local ::/0 dev lo table 100",
		"ip -4 rule del fwmark 0x1 lookup 100",
		"ip -4 rule add fwmark 0x1 lookup 100",
		"ip -6 rule del fwmark 0x1 lookup 100",
		"ip -6 rule add fwmark 0x1 lookup 100",
	}
	if strings.Join(commands, "\n") != strings.Join(want, "\n") {
		t.Fatalf("commands = %#v, want %#v", commands, want)
	}
}

func TestApplyTProxyRoutesIgnoresExistingRoutes(t *testing.T) {
	oldRouteCommand := routeCommand
	routeCommand = func(args ...string) error {
		command := strings.Join(args, " ")
		if strings.Contains(command, " route add ") {
			return errors.New("exit status 2: RTNETLINK answers: File exists")
		}
		if strings.Contains(command, " rule del ") {
			return errors.New("exit status 2: RTNETLINK answers: No such process")
		}
		return nil
	}
	t.Cleanup(func() { routeCommand = oldRouteCommand })

	if err := applyTProxyRoutes(); err != nil {
		t.Fatalf("apply tproxy routes should ignore existing route errors: %v", err)
	}
}

func TestCleanupTProxyRoutesDeletesPolicyRoutes(t *testing.T) {
	var commands []string
	deleted := map[string]bool{}
	oldRouteCommand := routeCommand
	routeCommand = func(args ...string) error {
		command := strings.Join(args, " ")
		commands = append(commands, command)
		if strings.Contains(command, " rule del ") {
			if deleted[command] {
				return errors.New("exit status 2: RTNETLINK answers: No such process")
			}
			deleted[command] = true
		}
		return nil
	}
	t.Cleanup(func() { routeCommand = oldRouteCommand })

	if err := cleanupTProxyRoutes(); err != nil {
		t.Fatalf("cleanup tproxy routes: %v", err)
	}
	want := []string{
		"ip -4 rule del fwmark 0x1 lookup 100",
		"ip -4 rule del fwmark 0x1 lookup 100",
		"ip -6 rule del fwmark 0x1 lookup 100",
		"ip -6 rule del fwmark 0x1 lookup 100",
		"ip -4 route del local 0.0.0.0/0 dev lo table 100",
		"ip -6 route del local ::/0 dev lo table 100",
	}
	if strings.Join(commands, "\n") != strings.Join(want, "\n") {
		t.Fatalf("commands = %#v, want %#v", commands, want)
	}
}

func TestApplyTProxyRoutesRemovesDuplicatePolicyRules(t *testing.T) {
	var commands []string
	deletesLeft := map[string]int{
		"ip -4 rule del fwmark 0x1 lookup 100": 2,
		"ip -6 rule del fwmark 0x1 lookup 100": 1,
	}
	oldRouteCommand := routeCommand
	routeCommand = func(args ...string) error {
		command := strings.Join(args, " ")
		commands = append(commands, command)
		if strings.Contains(command, " rule del ") {
			if deletesLeft[command] == 0 {
				return errors.New("exit status 2: RTNETLINK answers: No such process")
			}
			deletesLeft[command]--
		}
		return nil
	}
	t.Cleanup(func() { routeCommand = oldRouteCommand })

	if err := applyTProxyRoutes(); err != nil {
		t.Fatalf("apply tproxy routes: %v", err)
	}
	want := []string{
		"ip -4 route add local 0.0.0.0/0 dev lo table 100",
		"ip -6 route add local ::/0 dev lo table 100",
		"ip -4 rule del fwmark 0x1 lookup 100",
		"ip -4 rule del fwmark 0x1 lookup 100",
		"ip -4 rule del fwmark 0x1 lookup 100",
		"ip -4 rule add fwmark 0x1 lookup 100",
		"ip -6 rule del fwmark 0x1 lookup 100",
		"ip -6 rule del fwmark 0x1 lookup 100",
		"ip -6 rule add fwmark 0x1 lookup 100",
	}
	if strings.Join(commands, "\n") != strings.Join(want, "\n") {
		t.Fatalf("commands = %#v, want %#v", commands, want)
	}
}

func TestCleanupTProxyRoutesIgnoresMissingRoutes(t *testing.T) {
	oldRouteCommand := routeCommand
	routeCommand = func(args ...string) error {
		return errors.New("exit status 2: RTNETLINK answers: No such process")
	}
	t.Cleanup(func() { routeCommand = oldRouteCommand })

	if err := cleanupTProxyRoutes(); err != nil {
		t.Fatalf("cleanup tproxy routes should ignore missing route errors: %v", err)
	}
}

func TestCleanupFirewallLeavesKillSwitchOnly(t *testing.T) {
	cfg := managerconfig.DefaultConfig()
	cfg.Transparent.DefaultMode = "tproxy"
	cfg.Transparent.KillSwitch = true
	cfg.Transparent.LANIfnames = []string{"eth2"}

	oldRouteCommand := routeCommand
	routeCommand = func(args ...string) error {
		return errors.New("exit status 2: RTNETLINK answers: No such process")
	}
	t.Cleanup(func() { routeCommand = oldRouteCommand })

	oldFirewallReload := FirewallReload
	FirewallReload = func() error { return nil }
	t.Cleanup(func() { FirewallReload = oldFirewallReload })

	path := filepath.Join(t.TempDir(), "90-singbox-manager.nft")
	result := Result{}
	if err := cleanupFirewall(cfg, Paths{NftablesInclude: path}, &result); err != nil {
		t.Fatalf("cleanup firewall: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read kill switch include: %v", err)
	}
	got := string(data)
	if strings.Contains(got, "chain singbox_manager_tproxy {") {
		t.Fatalf("cleanup left tproxy chain in kill switch mode:\n%s", got)
	}
	if !strings.Contains(got, "singbox_manager_kill_switch_forward") || !strings.Contains(got, "counter drop") {
		t.Fatalf("cleanup did not leave kill switch drop chain:\n%s", got)
	}
}

func TestTeardownRemovesKillSwitchFirewall(t *testing.T) {
	cfg := managerconfig.DefaultConfig()
	cfg.Transparent.DefaultMode = "tproxy"
	cfg.Transparent.KillSwitch = true
	cfg.Transparent.LANIfnames = []string{"eth2"}

	oldRouteCommand := routeCommand
	routeCommand = func(args ...string) error {
		return errors.New("exit status 2: RTNETLINK answers: No such process")
	}
	t.Cleanup(func() { routeCommand = oldRouteCommand })

	oldFirewallReload := FirewallReload
	FirewallReload = func() error { return nil }
	t.Cleanup(func() { FirewallReload = oldFirewallReload })

	dir := t.TempDir()
	path := filepath.Join(dir, "90-singbox-manager.nft")
	if err := os.WriteFile(path, []byte("chain singbox_manager_kill_switch_forward {}\n"), 0644); err != nil {
		t.Fatalf("write firewall include: %v", err)
	}

	_, err := Control(cfg, ActionTeardown, Paths{
		PIDFile:         filepath.Join(dir, "sing-box.pid"),
		NftablesInclude: path,
	}, nil)
	if err != nil {
		t.Fatalf("teardown: %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("kill switch include still exists after teardown: %v", err)
	}
}
