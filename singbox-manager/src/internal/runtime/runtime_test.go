package runtime

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

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
		commands = append(commands, strings.Join(args, " "))
		return nil
	}
	t.Cleanup(func() { routeCommand = oldRouteCommand })

	if err := applyTProxyRoutes(); err != nil {
		t.Fatalf("apply tproxy routes: %v", err)
	}
	want := []string{
		"ip -4 rule add fwmark 0x1 lookup 100",
		"ip -4 route add local 0.0.0.0/0 dev lo table 100",
		"ip -6 rule add fwmark 0x1 lookup 100",
		"ip -6 route add local ::/0 dev lo table 100",
	}
	if strings.Join(commands, "\n") != strings.Join(want, "\n") {
		t.Fatalf("commands = %#v, want %#v", commands, want)
	}
}

func TestApplyTProxyRoutesIgnoresExistingRoutes(t *testing.T) {
	oldRouteCommand := routeCommand
	routeCommand = func(args ...string) error {
		return errors.New("exit status 2: RTNETLINK answers: File exists")
	}
	t.Cleanup(func() { routeCommand = oldRouteCommand })

	if err := applyTProxyRoutes(); err != nil {
		t.Fatalf("apply tproxy routes should ignore existing route errors: %v", err)
	}
}

func TestCleanupTProxyRoutesDeletesPolicyRoutes(t *testing.T) {
	var commands []string
	oldRouteCommand := routeCommand
	routeCommand = func(args ...string) error {
		commands = append(commands, strings.Join(args, " "))
		return nil
	}
	t.Cleanup(func() { routeCommand = oldRouteCommand })

	if err := cleanupTProxyRoutes(); err != nil {
		t.Fatalf("cleanup tproxy routes: %v", err)
	}
	want := []string{
		"ip -4 rule del fwmark 0x1 lookup 100",
		"ip -4 route del local 0.0.0.0/0 dev lo table 100",
		"ip -6 rule del fwmark 0x1 lookup 100",
		"ip -6 route del local ::/0 dev lo table 100",
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
