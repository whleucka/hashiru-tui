package system

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadBatteries(t *testing.T) {
	bats := ReadBatteries("testdata/sysroot")
	if len(bats) != 1 {
		t.Fatalf("batteries = %+v", bats)
	}
	b := bats[0]
	if b.Name != "BAT0" || b.Capacity != 32 || b.Status != "Discharging" || b.Cycles != 57 {
		t.Errorf("battery = %+v", b)
	}
	// 50.83 of 58 Wh.
	if b.Health < 0.876 || b.Health > 0.877 {
		t.Errorf("health = %v, want 0.876", b.Health)
	}
	if !b.Thresholds || b.Start != 0 || b.Stop != 100 {
		t.Errorf("thresholds = %v %d/%d", b.Thresholds, b.Start, b.Stop)
	}
}

func TestReadBatteriesSparse(t *testing.T) {
	root := t.TempDir()
	d := filepath.Join(root, "sys/class/power_supply/BAT1")
	write := func(name, v string) {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(d, name), []byte(v+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	// charge_* rather than energy_*, no cycle count, no thresholds.
	write("capacity", "80")
	write("status", "Charging")
	write("charge_full", "4000000")
	write("charge_full_design", "5000000")
	bats := ReadBatteries(root)
	if len(bats) != 1 {
		t.Fatalf("batteries = %+v", bats)
	}
	b := bats[0]
	if b.Health != 0.8 || b.Cycles != -1 || b.Thresholds {
		t.Errorf("battery = %+v", b)
	}
	if got := ReadBatteries(t.TempDir()); len(got) != 0 {
		t.Errorf("no power_supply: %+v", got)
	}
}

func TestOnAC(t *testing.T) {
	if on, known := OnAC("testdata/sysroot"); !known || on {
		t.Errorf("fixture's AC is offline: on %v known %v", on, known)
	}
	if _, known := OnAC(t.TempDir()); known {
		t.Error("no mains supply: unknown")
	}
}

func TestPlatformProfile(t *testing.T) {
	cur, choices := PlatformProfile("testdata/sysroot")
	if cur != "balanced" || strings.Join(choices, " ") != "low-power balanced performance" {
		t.Errorf("profile = %q %v", cur, choices)
	}
	if cur, choices := PlatformProfile(t.TempDir()); cur != "" || choices != nil {
		t.Errorf("none: %q %v", cur, choices)
	}
}

func TestParseTLPStatus(t *testing.T) {
	s := ParseTLPStatus(fixture(t, "tlp-stat-s.txt"))
	if s.Version != "1.10.2" || s.Profile != "balanced" || s.Source != "battery" || !s.Enabled {
		t.Errorf("tlp = %+v", s)
	}
	if s := ParseTLPStatus([]byte("garbage")); s.Profile != "" || s.Enabled {
		t.Errorf("garbage = %+v", s)
	}
}

func TestParseIdle(t *testing.T) {
	bat := ParseIdle(fixture(t, "hypridle-battery.conf"))
	ac := ParseIdle(fixture(t, "hypridle-ac.conf"))
	if bat.Lock != 300 || bat.Off != 330 || ac.Lock != 900 || ac.Off != 930 {
		t.Errorf("battery %+v, ac %+v", bat, ac)
	}
	if got := ParseIdle([]byte("general {\n}\n")); got.Lock != 0 || got.Off != 0 {
		t.Errorf("no listeners = %+v", got)
	}
}

func TestRewriteIdle(t *testing.T) {
	in := fixture(t, "hypridle-battery.conf")
	out, err := RewriteIdle(in, Idle{Lock: 600, Off: 630})
	if err != nil {
		t.Fatal(err)
	}
	if got := ParseIdle(out); got.Lock != 600 || got.Off != 630 {
		t.Errorf("rewritten = %+v", got)
	}
	// Only the two timeout lines change.
	a, b := strings.Split(string(in), "\n"), strings.Split(string(out), "\n")
	if len(a) != len(b) {
		t.Fatalf("line count %d → %d", len(a), len(b))
	}
	var changed []string
	for i := range a {
		if a[i] != b[i] {
			changed = append(changed, strings.TrimSpace(b[i]))
		}
	}
	if strings.Join(changed, "|") != "timeout = 600|timeout = 630" {
		t.Errorf("changed lines = %q", changed)
	}
	if _, err := RewriteIdle([]byte("general {\n}\n"), Idle{Lock: 60, Off: 90}); err == nil {
		t.Error("no listeners to rewrite: want an error")
	}
}

func TestHashiruCapabilities(t *testing.T) {
	if !HasChargeCommand(fixture(t, "hashiru-help.txt")) || HasChargeCommand([]byte("Commands:\n  fingerprint [on|off]\n")) {
		t.Error("HasChargeCommand")
	}
	if !IdleReloadable(fixture(t, "hypridle-battery.sh")) || IdleReloadable([]byte("#!/bin/bash\nupower --monitor\n")) {
		t.Error("IdleReloadable")
	}
}
