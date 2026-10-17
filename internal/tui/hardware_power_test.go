package tui

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func readFixture(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile("../system/testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// powerHome is a home with Hashiru's hypridle confs stowed, and
// an override for battery when override is set.
func powerHome(t *testing.T, override bool) string {
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

func newPowerRig(t *testing.T, home, sysroot string) powerRig {
	t.Helper()
	p := monProbe(t)
	p["tlp-stat -s"] = readFixture(t, "tlp-stat-s.txt")
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
	home := powerHome(t, false)
	r := newPowerRig(t, home, "")
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
}

func TestPowerTabOverride(t *testing.T) {
	home := powerHome(t, true)
	r := newPowerRig(t, home, "")
	out := view(r.m)
	if !strings.Contains(out, "10 min") || !strings.Contains(out, "yours") {
		t.Errorf("battery override not shown:\n%s", out)
	}
}

func TestPowerTabNoBattery(t *testing.T) {
	home := powerHome(t, false)
	r := newPowerRig(t, home, t.TempDir())
	out := view(r.m)
	if !strings.Contains(out, "no battery") || !strings.Contains(out, "balanced") || strings.Contains(out, "0/100") {
		t.Errorf("no battery:\n%s", out)
	}
}

func backspaces(m *Model, n int) {
	for range n {
		_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyBackspace})
		drain(m, cmd)
	}
}

func TestPowerProfile(t *testing.T) {
	r := newPowerRig(t, powerHome(t, false), "")
	press(r.m, "p")
	if out := view(r.m); !strings.Contains(out, "power-saver") {
		t.Fatalf("profile picker:\n%s", out)
	}
	press(r.m, "k", "enter") // opens on balanced; up is performance
	if strings.Join(*r.launched, "|") != "sudo tlp performance" {
		t.Errorf("launched %v", *r.launched)
	}
}

func TestPowerIdleEditAndReset(t *testing.T) {
	r := newPowerRig(t, powerHome(t, false), "")
	_, acOverride := (&power{home: r.home}).idlePaths("ac")
	_, batOverride := (&power{home: r.home}).idlePaths("battery")

	press(r.m, "i")
	if out := view(r.m); !strings.Contains(out, "on AC") {
		t.Fatalf("no prompt:\n%s", out)
	}
	press(r.m, "enter") // AC stays 15
	backspaces(r.m, 2)
	press(r.m, "1", "0", "enter") // battery 5 → 10
	if _, err := os.Stat(acOverride); err == nil {
		t.Error("AC didn't change; it shouldn't get an override")
	}
	b, err := os.ReadFile(batOverride)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), "timeout = 600") || !strings.Contains(string(b), "timeout = 630") {
		t.Errorf("override:\n%s", b)
	}
	if strings.Join(*r.quiet, "|") != "hypridle-battery reload" {
		t.Errorf("reloads = %v", *r.quiet)
	}
	if out := view(r.m); !strings.Contains(out, "10 min") || !strings.Contains(out, "yours") {
		t.Errorf("not re-read:\n%s", out)
	}

	press(r.m, "I")
	if _, err := os.Stat(batOverride); err == nil {
		t.Error("I should remove the override")
	}
	if len(*r.quiet) != 2 {
		t.Errorf("reloads = %v", *r.quiet)
	}
}

func TestPowerIdleRejectsNonsense(t *testing.T) {
	r := newPowerRig(t, powerHome(t, false), "")
	press(r.m, "i")
	backspaces(r.m, 2)
	press(r.m, "x", "enter")
	if out := view(r.m); !strings.Contains(out, "minutes") || !strings.Contains(out, "on AC") {
		t.Errorf("should stay on the AC prompt and say why:\n%s", out)
	}
	press(r.m, "esc")
	if len(*r.quiet) != 0 {
		t.Errorf("esc wrote something: %v", *r.quiet)
	}
}

func TestPowerCharge(t *testing.T) {
	r := newPowerRig(t, powerHome(t, false), "")
	press(r.m, "c")
	backspaces(r.m, 3)
	press(r.m, "7", "5", "enter")
	backspaces(r.m, 3)
	press(r.m, "8", "0", "enter")
	press(r.m, "C")
	if want := "hashiru charge 75 80|hashiru charge off"; strings.Join(*r.launched, "|") != want {
		t.Errorf("launched %v, want %s", *r.launched, want)
	}

	press(r.m, "c")
	backspaces(r.m, 3)
	press(r.m, "8", "0", "enter")
	backspaces(r.m, 3)
	press(r.m, "7", "5", "enter")
	if len(*r.launched) != 2 || !strings.Contains(view(r.m), "start < stop") {
		t.Errorf("80/75 should be refused: %v\n%s", *r.launched, view(r.m))
	}
}

func TestPowerActions(t *testing.T) {
	m := New(Options{Source: fakeSource{}})
	ids := map[string]bool{}
	for _, a := range m.Actions() {
		ids[a.ID] = true
	}
	for _, id := range []string{"hardware.power-profile", "hardware.idle", "hardware.idle-reset", "hardware.charge", "hardware.charge-off"} {
		if !ids[id] {
			t.Errorf("no action %s", id)
		}
	}
}
