package config

import "testing"

func TestEffectiveResolvesDefault(t *testing.T) {
	cases := []struct {
		name        string
		defaultMode string
		deviceMode  string
		want        string
	}{
		{"inherit tproxy", "tproxy", "default", "tproxy"},
		{"inherit tproxy empty", "tproxy", "", "tproxy"},
		{"inherit off", "off", "default", "off"},
		{"override to tproxy", "off", "tproxy", "tproxy"},
		{"override to bypass", "tproxy", "bypass", "bypass"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tp := Transparent{DefaultMode: tc.defaultMode}
			got := tp.Effective(Device{Mode: tc.deviceMode})
			if got != tc.want {
				t.Fatalf("Effective(%q over %q) = %q, want %q", tc.deviceMode, tc.defaultMode, got, tc.want)
			}
		})
	}
}

func TestUsesTProxyAcrossDefaultsAndOverrides(t *testing.T) {
	cases := []struct {
		name        string
		transparent Transparent
		wantTProxy  bool
		wantActive  bool
	}{
		{
			name:        "off with no devices",
			transparent: Transparent{DefaultMode: "off"},
		},
		{
			name:        "off but a tproxy device",
			transparent: Transparent{DefaultMode: "off", Devices: []Device{{Enabled: true, Mode: "tproxy", MAC: "aa:bb:cc:dd:ee:01"}}},
			wantTProxy:  true,
			wantActive:  true,
		},
		{
			name:        "default tproxy with a bypass device",
			transparent: Transparent{DefaultMode: "tproxy", Devices: []Device{{Enabled: true, Mode: "bypass", MAC: "aa:bb:cc:dd:ee:01"}}},
			wantTProxy:  true,
			wantActive:  true,
		},
		{
			name:        "disabled device does not count",
			transparent: Transparent{DefaultMode: "off", Devices: []Device{{Enabled: false, Mode: "tproxy", MAC: "aa:bb:cc:dd:ee:01"}}},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := Config{Transparent: tc.transparent}
			if got := cfg.UsesTProxy(); got != tc.wantTProxy {
				t.Errorf("UsesTProxy = %v, want %v", got, tc.wantTProxy)
			}
			if got := tc.transparent.Active(); got != tc.wantActive {
				t.Errorf("Active = %v, want %v", got, tc.wantActive)
			}
		})
	}
}

func TestUDPBypassDevices(t *testing.T) {
	tp := Transparent{
		DefaultMode: "tproxy",
		Devices: []Device{
			{ID: "console", Enabled: true, Mode: "tproxy", BypassUDP: true, MAC: "aa:bb:cc:dd:ee:01"},
			{ID: "inherits", Enabled: true, Mode: "default", BypassUDP: true, MAC: "aa:bb:cc:dd:ee:02"},
			{ID: "bypassed", Enabled: true, Mode: "bypass", BypassUDP: true, MAC: "aa:bb:cc:dd:ee:03"},
			{ID: "no-flag", Enabled: true, Mode: "tproxy", MAC: "aa:bb:cc:dd:ee:04"},
			{ID: "disabled", Enabled: false, Mode: "tproxy", BypassUDP: true, MAC: "aa:bb:cc:dd:ee:05"},
		},
	}
	got := tp.UDPBypassDevices()
	if len(got) != 2 {
		t.Fatalf("UDPBypassDevices returned %d devices, want 2: %#v", len(got), got)
	}
	if got[0].ID != "console" || got[1].ID != "inherits" {
		t.Fatalf("UDPBypassDevices = %q, %q, want console, inherits", got[0].ID, got[1].ID)
	}
}
