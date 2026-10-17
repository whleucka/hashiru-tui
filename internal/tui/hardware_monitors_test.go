package tui

import (
	"context"
	"os"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/whleucka/hashiru-tui/internal/system"
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

// applyRig is the Monitors tab with a temp home, a recorded hyprctl reload
// and no real timer: tests send the ticks.
type applyRig struct {
	m       *Model
	mon     *monitors
	file    string
	reloads *int
}

func newApplyRig(t *testing.T, existing string) applyRig {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", "")
	home := t.TempDir()
	file := home + "/.config/hashiru/hypr/monitors.lua"
	if existing != "" {
		if err := os.MkdirAll(home+"/.config/hashiru/hypr", 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(file, []byte(existing), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	p := monProbe(t)
	reloads := new(int)
	m := New(Options{
		Source: fakeSource{},
		Home:   home,
		Probe: func(ctx context.Context, name string, args ...string) ([]byte, error) {
			if name == "hyprctl" && len(args) == 1 && args[0] == "reload" {
				*reloads++
				return []byte("ok"), nil
			}
			return p.run(ctx, name, args...)
		},
	})
	load(t, m)
	goTo(t, m, "Hardware")
	return applyRig{m, m.area().(*hardware).tabs[0].(*monitors), file, reloads}
}

func (r applyRig) read(t *testing.T) (string, bool) {
	t.Helper()
	b, err := os.ReadFile(r.file)
	if os.IsNotExist(err) {
		return "", false
	}
	if err != nil {
		t.Fatal(err)
	}
	return string(b), true
}

func (r applyRig) ticks(n int) {
	for range n {
		_, cmd := r.m.Update(monTickMsg{seq: r.mon.seq})
		drain(r.m, cmd)
	}
}

// stageScale stages DP-2 at 1.5.
func (r applyRig) stageScale() { press(r.m, "j", "s", "j", "j", "enter") }

func TestMonitorsApplyKeep(t *testing.T) {
	r := newApplyRig(t, "")
	press(r.m, "a")
	if _, ok := r.read(t); ok || *r.reloads != 0 {
		t.Fatal("a with nothing staged wrote or reloaded")
	}
	r.stageScale()
	press(r.m, "a")
	got, ok := r.read(t)
	if !ok || !strings.HasPrefix(got, "-- Written by hashiru-tui") || !strings.Contains(got, `scale = 1.5`) {
		t.Fatalf("written:\n%s", got)
	}
	if *r.reloads != 1 {
		t.Errorf("%d reloads, want 1", *r.reloads)
	}
	// The fixture doesn't change, so Hyprland "used" 1.25: say so.
	if !strings.Contains(r.m.note, "DP-2 at 1.25, not 1.5") {
		t.Errorf("note = %q", r.m.note)
	}
	if out := view(r.m); !strings.Contains(out, "Keep this layout?") || !strings.Contains(out, "15s") {
		t.Errorf("no countdown:\n%s", out)
	}
	r.ticks(3)
	if out := view(r.m); !strings.Contains(out, "12s") {
		t.Errorf("countdown didn't count:\n%s", out)
	}
	press(r.m, "y")
	if got2, _ := r.read(t); got2 != got || *r.reloads != 1 {
		t.Errorf("y should keep the file and not reload again (%d reloads)", *r.reloads)
	}
	if r.mon.pending != nil || r.mon.staged != nil {
		t.Error("still pending or staged after keeping")
	}
	// A tick left over from the countdown does nothing.
	r.ticks(20)
	if _, ok := r.read(t); !ok {
		t.Error("a stale tick reverted a kept layout")
	}
}

func TestMonitorsApplyTimesOut(t *testing.T) {
	r := newApplyRig(t, "")
	r.stageScale()
	press(r.m, "a")
	r.ticks(14)
	if _, ok := r.read(t); !ok {
		t.Fatal("reverted early")
	}
	r.ticks(1)
	if _, ok := r.read(t); ok {
		t.Error("timeout should remove a file that wasn't there before")
	}
	if *r.reloads != 2 {
		t.Errorf("%d reloads, want 2 (apply, revert)", *r.reloads)
	}
	// The changes are still staged, to adjust and try again.
	if r.mon.staged == nil || r.mon.pending != nil {
		t.Error("revert should leave the changes staged")
	}
}

func TestMonitorsApplyAnyOtherKeyReverts(t *testing.T) {
	ours := system.MonitorsHeader + "\nhl.monitor({ output = \"eDP-1\", mode = \"preferred\", position = \"0x0\", scale = 1 })\n"
	r := newApplyRig(t, ours)
	r.stageScale()
	press(r.m, "a")
	if out := view(r.m); strings.Contains(out, "by hand") {
		t.Fatalf("our own file shouldn't ask:\n%s", out)
	}
	press(r.m, "q")
	if got, _ := r.read(t); got != ours {
		t.Errorf("not restored:\n%s", got)
	}
	if r.m.quitting {
		t.Error("q during the countdown reverts; it doesn't ask to quit")
	}
}

func TestMonitorsApplyHandWritten(t *testing.T) {
	hand := "hl.monitor({ output = \"eDP-1\", mode = \"preferred\", position = \"0x0\", scale = 1 })\n"
	r := newApplyRig(t, hand)
	r.stageScale()
	press(r.m, "a")
	if out := view(r.m); !strings.Contains(out, "by hand") || !strings.Contains(out, "monitors.lua.bak") {
		t.Fatalf("should ask first:\n%s", out)
	}
	press(r.m, "n")
	if got, _ := r.read(t); got != hand || *r.reloads != 0 {
		t.Fatal("n should change nothing")
	}
	press(r.m, "a", "y")
	bak, err := os.ReadFile(r.file + ".bak")
	if err != nil || string(bak) != hand {
		t.Errorf("bak = %q, %v", bak, err)
	}
	if got, _ := r.read(t); !system.HasHeader([]byte(got)) {
		t.Errorf("not written:\n%s", got)
	}
	r.ticks(15)
	if got, _ := r.read(t); got != hand {
		t.Errorf("revert should put the hand-written file back:\n%s", got)
	}
}

func TestMonitorsQuitMidCountdownReverts(t *testing.T) {
	r := newApplyRig(t, "")
	r.stageScale()
	press(r.m, "a")
	r.m.Update(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})
	if _, ok := r.read(t); ok || *r.reloads != 2 {
		t.Errorf("ctrl+c mid-countdown should put the old layout back (%d reloads)", *r.reloads)
	}
}
