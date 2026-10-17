package tui

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func readFixture(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile("../system/testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// powerHome is a home with Hashiru's hypridle confs and script stowed, and
// an override for battery when override is set.
func powerHome(t *testing.T, script string, override bool) string {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", "")
	home := t.TempDir()
	put := func(rel, body string) {
		p := filepath.Join(home, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	put(".config/hypr/hypridle-ac.conf", readFixture(t, "hypridle-ac.conf"))
	put(".config/hypr/hypridle-battery.conf", readFixture(t, "hypridle-battery.conf"))
	put(".config/hypr/scripts/hypridle-battery", script)
	if override {
		put(".config/hashiru/hypr/hypridle-battery.conf",
			strings.Replace(readFixture(t, "hypridle-battery.conf"), "timeout = 300", "timeout = 600", 1))
	}
	return home
}

type powerRig struct {
	m        *Model
	home     string
	launched *[]string
	quiet    *[]string
}

func newPowerRig(t *testing.T, help string, home, sysroot string) powerRig {
	t.Helper()
	p := monProbe(t)
	p["tlp-stat -s"] = readFixture(t, "tlp-stat-s.txt")
	if help != "" {
		p["hashiru help"] = help
	}
	var launched, quiet []string
	m := New(Options{
		Source:  fakeSource{},
		Home:    home,
		SysRoot: sysroot,
		Probe: func(ctx context.Context, name string, args ...string) ([]byte, error) {
			line := strings.Join(append([]string{name}, args...), " ")
			if strings.HasSuffix(line, "hypridle-battery reload") {
				quiet = append(quiet, "hypridle-battery reload")
				return nil, nil
			}
			return p.run(ctx, name, args...)
		},
		RunInPane: func(_ context.Context, argv []string, _ string, _, _ bool) error {
			launched = append(launched, strings.Join(argv, " "))
			return nil
		},
	})
	load(t, m)
	goTo(t, m, "Hardware")
	if err := m.openTab(m.area(), "power"); err != nil {
		t.Fatal(err)
	}
	return powerRig{m, home, &launched, &quiet}
}

func TestPowerTab(t *testing.T) {
	home := powerHome(t, readFixture(t, "hypridle-battery.sh"), false)
	r := newPowerRig(t, readFixture(t, "hashiru-help.txt"), home, "")
	out := view(r.m)
	for _, want := range []string{
		"Monitors (3)", "Power", "Firmware (2)",
		"BAT0", "32%", "discharging", "health 88%", "57 cycles",
		"thresholds", "0/100",
		"balanced", "on battery", "TLP 1.10.2",
		"AC", "15 min", "battery", "5 min", "+30s", "Hashiru's",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in\n%s", want, out)
		}
	}
	if strings.Contains(out, "needs Hashiru") {
		t.Errorf("this Hashiru has everything:\n%s", out)
	}
}

func TestPowerTabOverride(t *testing.T) {
	home := powerHome(t, readFixture(t, "hypridle-battery.sh"), true)
	r := newPowerRig(t, readFixture(t, "hashiru-help.txt"), home, "")
	out := view(r.m)
	if !strings.Contains(out, "10 min") || !strings.Contains(out, "yours") {
		t.Errorf("battery override not shown:\n%s", out)
	}
}

func TestPowerTabOldHashiru(t *testing.T) {
	home := powerHome(t, "#!/bin/bash\nupower --monitor\n", false)
	r := newPowerRig(t, "Commands:\n  fingerprint [on|off]\n", home, "")
	out := view(r.m)
	if strings.Count(out, "needs Hashiru v1.14.0") != 2 {
		t.Errorf("thresholds and idle should each say what they need:\n%s", out)
	}
}

func TestPowerTabNoBattery(t *testing.T) {
	home := powerHome(t, readFixture(t, "hypridle-battery.sh"), false)
	r := newPowerRig(t, readFixture(t, "hashiru-help.txt"), home, t.TempDir())
	out := view(r.m)
	if !strings.Contains(out, "no battery") || !strings.Contains(out, "balanced") || strings.Contains(out, "thresholds") {
		t.Errorf("no battery:\n%s", out)
	}
}
