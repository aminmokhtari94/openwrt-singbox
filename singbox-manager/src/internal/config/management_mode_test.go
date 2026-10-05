package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestManagerActiveHonoursPause(t *testing.T) {
	cases := []struct {
		enabled bool
		paused  bool
		active  bool
	}{
		{enabled: true, paused: false, active: true},
		{enabled: true, paused: true, active: false},
		{enabled: false, paused: false, active: false},
		{enabled: false, paused: true, active: false},
	}
	for _, tc := range cases {
		m := Manager{Enabled: tc.enabled, Paused: tc.paused}
		if got := m.Active(); got != tc.active {
			t.Fatalf("Active(enabled=%v paused=%v) = %v, want %v", tc.enabled, tc.paused, got, tc.active)
		}
	}
}

func TestLoadParsesPausedOption(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "singbox-manager")
	data := "config manager 'main'\n\toption enabled '1'\n\toption paused '1'\n\toption active_group 'home'\n\nconfig group 'home'\n\toption name 'Home'\n"
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
	if !cfg.Manager.Paused {
		t.Fatal("expected manager paused")
	}
	if cfg.Manager.Active() {
		t.Fatal("a paused manager must not be Active()")
	}
}

func TestSetManagerPausedRoundTrips(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "singbox-manager")
	data := "config manager 'main'\n\toption enabled '1'\n\toption active_group 'home'\n\nconfig group 'home'\n\toption name 'Home'\n"
	if err := os.WriteFile(path, []byte(data), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	if err := SetManagerPaused(path, true); err != nil {
		t.Fatalf("set paused: %v", err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if !cfg.Manager.Paused {
		t.Fatal("expected paused=true after SetManagerPaused(true)")
	}

	if err := SetManagerPaused(path, false); err != nil {
		t.Fatalf("clear paused: %v", err)
	}
	cfg, err = Load(path)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if cfg.Manager.Paused {
		t.Fatal("expected paused=false after SetManagerPaused(false)")
	}
}
