package tui

import (
	"os"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

// monProbe answers hyprctl with the docked fixture: the panel, an LG at
// 1.25 and a rotated Dell that's off.
func monProbe(t *testing.T) probe {
	t.Helper()
	out, err := os.ReadFile("../system/testdata/hyprctl-monitors-docked.json")
	if err != nil {
		t.Fatal(err)
	}
	p := fwProbe(t)
	p["hyprctl monitors all -j"] = string(out)
	return p
}

func monitorsModel(t *testing.T) *Model {
	t.Helper()
	m, _ := hardwareModel(t, monProbe(t))
	return m
}

func view(m *Model) string { return ansi.Strip(m.View().Content) }

func TestMonitorsList(t *testing.T) {
	m := monitorsModel(t)
	out := view(m)
	for _, want := range []string{
		"Monitors (3)", "Firmware (2)",
		"eDP-1", "1920x1200@60.00", "320x1440",
		"DP-2", "LG Electronics LG ULTRAGEAR", "2560x1440@143.99", "1.25",
		"HDMI-A-1", "90°", "off",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in\n%s", want, out)
		}
	}
}

func TestMonitorsDetails(t *testing.T) {
	m := monitorsModel(t)
	press(m, "j", "enter")
	out := view(m)
	for _, want := range []string{"DP-2", "LG Electronics LG ULTRAGEAR 605BNHC03417", "2560x1440@59.95", "1920x1080@60.00", "desc:LG Electronics LG ULTRAGEAR 605BNHC03417"} {
		if !strings.Contains(out, want) {
			t.Errorf("details missing %q in\n%s", want, out)
		}
	}
	press(m, "esc")
	if !strings.Contains(view(m), "HDMI-A-1") {
		t.Error("esc didn't go back")
	}
}

func TestMonitorsStageScaleAndRotation(t *testing.T) {
	m := monitorsModel(t)
	press(m, "j", "s")
	if out := view(m); !strings.Contains(out, "Scale for DP-2") || !strings.Contains(out, "1.5") {
		t.Fatalf("scale picker:\n%s", out)
	}
	// 1, 1.25, 1.333, 1.5: the picker opens on the current scale.
	press(m, "j", "j", "enter")
	press(m, "t")
	out := view(m)
	for _, want := range []string{"1.25 → 1.5", "0° → 90°", "Monitors (3) *"} {
		if !strings.Contains(out, want) {
			t.Errorf("staged: missing %q in\n%s", want, out)
		}
	}
	// esc drops what's staged rather than leaving the area.
	press(m, "esc")
	if out := view(m); strings.Contains(out, "→") || !m.inArea {
		t.Errorf("esc should drop the changes and stay:\n%s", out)
	}
}

func TestMonitorsStageModeAndPlace(t *testing.T) {
	m := monitorsModel(t)
	press(m, "m")
	if out := view(m); !strings.Contains(out, "preferred") || !strings.Contains(out, "1280x800@60.00") {
		t.Fatalf("mode picker:\n%s", out)
	}
	press(m, "k", "enter") // the first entry, preferred
	if out := view(m); !strings.Contains(out, "1920x1200@60.00 → preferred") {
		t.Errorf("mode not staged:\n%s", out)
	}

	press(m, "p")
	out := view(m)
	if !strings.Contains(out, "below DP-2") || strings.Contains(out, "HDMI-A-1") {
		t.Fatalf("place picker should list the enabled others:\n%s", out)
	}
	// left, right, above, below DP-2: below is the fourth.
	press(m, "j", "j", "j", "enter")
	if out := view(m); !strings.Contains(out, "320x1440 → 0x1152") {
		t.Errorf("not placed below the LG (1152 tall at 1.25):\n%s", out)
	}
}

func TestMonitorsOnOff(t *testing.T) {
	m := monitorsModel(t)
	press(m, "d")
	if out := view(m); strings.Contains(out, "→") || !strings.Contains(out, "clamshell") {
		t.Errorf("the panel is clamshell.lua's to turn off:\n%s", out)
	}
	press(m, "j", "j", "e")
	if out := view(m); !strings.Contains(out, "off → on") {
		t.Errorf("Dell not staged on:\n%s", out)
	}
}

func TestMonitorsActionsAndDeepLink(t *testing.T) {
	m := New(Options{Source: fakeSource{}, Probe: monProbe(t).run})
	if err := m.Start("hardware.monitors", ""); err != nil {
		t.Fatal(err)
	}
	load(t, m)
	if out := view(m); !strings.Contains(out, "DP-2") {
		t.Errorf("not on Monitors:\n%s", out)
	}
}
