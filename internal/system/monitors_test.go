package system

import (
	"strings"
	"testing"
)

func docked(t *testing.T) Layout {
	t.Helper()
	ms, err := ParseMonitors(fixture(t, "hyprctl-monitors-docked.json"))
	if err != nil {
		t.Fatal(err)
	}
	return LayoutOf(ms)
}

func TestParseMonitors(t *testing.T) {
	ms, err := ParseMonitors(fixture(t, "hyprctl-monitors.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(ms) != 1 {
		t.Fatalf("got %d monitors", len(ms))
	}
	m := ms[0]
	if m.Name != "eDP-1" || m.Description != "Lenovo Group Limited 0x41C6" || m.Width != 1920 || m.Height != 1200 ||
		m.X != 320 || m.Y != 1440 || m.Scale != 1 || m.Disabled || !m.Focused || len(m.Modes) != 11 {
		t.Errorf("monitor = %+v", m)
	}
	if m.Modes[0] != "1920x1200@60.00" {
		t.Errorf("modes lose their Hz: %q", m.Modes[0])
	}
	if _, err := ParseMonitors([]byte("Couldn't connect")); err == nil {
		t.Error("bad JSON: no error")
	}
}

func TestLayoutOf(t *testing.T) {
	l := docked(t)
	if len(l) != 3 {
		t.Fatalf("layout = %+v", l)
	}
	if o := l[0]; o.Mode != "1920x1200@60.00" || o.Output() != "eDP-1" {
		t.Errorf("panel = %+v, rule output %q", o, o.Output())
	}
	if o := l[1]; o.Mode != "2560x1440@143.99" || o.Output() != "desc:LG Electronics LG ULTRAGEAR 605BNHC03417" {
		t.Errorf("LG = %+v, rule output %q", o, o.Output())
	}
	if w, h := l[1].Logical(); w != 2048 || h != 1152 {
		t.Errorf("LG at 1.25 is %dx%d, want 2048x1152", w, h)
	}
	if w, h := l[2].Logical(); w != 1080 || h != 1920 {
		t.Errorf("rotated Dell is %dx%d, want 1080x1920", w, h)
	}
	if !l[2].Disabled {
		t.Error("Dell should be disabled")
	}
}

func TestSetMode(t *testing.T) {
	l := docked(t)
	l[1].SetMode("1920x1080@60.00")
	if w, h := l[1].Logical(); l[1].Mode != "1920x1080@60.00" || w != 1536 || h != 864 {
		t.Errorf("after SetMode: %+v, logical %dx%d", l[1], w, h)
	}
	l[1].SetMode("preferred")
	if l[1].Mode != "preferred" || l[1].W != 1920 {
		t.Errorf("preferred keeps the last known size for placing: %+v", l[1])
	}
}

func TestPlace(t *testing.T) {
	for _, tc := range []struct {
		rel        string
		x0, y0, x1 int // the panel's position, and the LG's x, after placing
	}{
		{"below", 0, 1152, 0},
		{"above", 0, 0, 0}, // the panel moves up to -1200; everything shifts down
		{"right", 2048, 0, 0},
		{"left", 0, 0, 1920},
	} {
		l := docked(t)
		l = Place(l, 0, tc.rel, 1)
		if l[0].X != tc.x0 || l[0].Y != tc.y0 || l[1].X != tc.x1 {
			t.Errorf("%s: panel at %dx%d, LG at x %d; want %dx%d, %d", tc.rel, l[0].X, l[0].Y, l[1].X, tc.x0, tc.y0, tc.x1)
		}
		if tc.rel == "above" && l[1].Y != 1200 {
			t.Errorf("above: LG at y %d, want 1200", l[1].Y)
		}
	}
}

func TestEnabled(t *testing.T) {
	l := docked(t)
	if !l.AnyEnabled() {
		t.Error("two are on")
	}
	l[0].Disabled, l[1].Disabled = true, true
	if l.AnyEnabled() {
		t.Error("none are on")
	}
}

func TestParseRules(t *testing.T) {
	rules := ParseRules(fixture(t, "monitors-handwritten.lua"))
	if len(rules) != 2 { // this machine's has no catch-all
		t.Fatalf("rules = %+v", rules)
	}
	if rules[0].Output != "eDP-1" || rules[1].Output != "desc:LG Electronics LG ULTRAGEAR 605BNHC03417" {
		t.Errorf("outputs = %q %q", rules[0].Output, rules[1].Output)
	}
	if got := ParseRules([]byte(`hl.monitor({ output = "", mode = "preferred", position = "auto", scale = 1 })`)); len(got) != 1 || got[0].Output != "" {
		t.Errorf("catch-all = %+v", got)
	}
	if !strings.HasPrefix(rules[1].Line, "hl.monitor({ output = \"desc:LG") {
		t.Errorf("line = %q", rules[1].Line)
	}
	// Commented-out rules aren't rules.
	if got := ParseRules([]byte(`-- hl.monitor({ output = "DP-1", mode = "preferred" })`)); len(got) != 0 {
		t.Errorf("comment parsed as %+v", got)
	}
}

func TestMatches(t *testing.T) {
	m := Monitor{Name: "DP-2", Description: "LG Electronics LG ULTRAGEAR 605BNHC03417"}
	for out, want := range map[string]bool{
		"DP-2": true, "desc:LG Electronics LG ULTRAGEAR 605BNHC03417": true, "desc:LG Electronics": true,
		"DP-1": false, "desc:BNQ": false, "": false,
	} {
		if Matches(out, m) != want {
			t.Errorf("Matches(%q) = %v", out, !want)
		}
	}
}

const wantDocked = MonitorsHeader + `
hl.monitor({ output = "eDP-1", mode = "1920x1200@60.00", position = "320x1440", scale = 1 })
hl.monitor({ output = "desc:LG Electronics LG ULTRAGEAR 605BNHC03417", mode = "2560x1440@143.99", position = "0x0", scale = 1.25 })
hl.monitor({ output = "desc:Dell Inc. DELL P2419H 7XYZ", disabled = true })

-- Anything not named above: preferred mode, placed left to right.
hl.monitor({ output = "", mode = "preferred", position = "auto", scale = 1 })
`

func TestMonitorsLuaDocked(t *testing.T) {
	l := docked(t)
	ms, _ := ParseMonitors(fixture(t, "hyprctl-monitors-docked.json"))
	got := string(MonitorsLua(l, ParseRules(fixture(t, "monitors-handwritten.lua")), ms))
	if got != wantDocked {
		t.Errorf("got\n%s\nwant\n%s", got, wantDocked)
	}
	if !HasHeader([]byte(got)) || HasHeader(fixture(t, "monitors-handwritten.lua")) {
		t.Error("HasHeader tells ours from a hand-written one")
	}
}

func TestMonitorsLuaKeepsAbsentDisplays(t *testing.T) {
	ms, _ := ParseMonitors(fixture(t, "hyprctl-monitors.json"))
	l := LayoutOf(ms)
	l[0].Scale = 1.5
	got := string(MonitorsLua(l, ParseRules(fixture(t, "monitors-handwritten.lua")), ms))
	want := MonitorsHeader + `
hl.monitor({ output = "eDP-1", mode = "1920x1200@60.00", position = "320x1440", scale = 1.5 })

-- Not connected when this was written; kept as they were.
hl.monitor({ output = "desc:LG Electronics LG ULTRAGEAR 605BNHC03417", mode = "2560x1440@143.99Hz", position = "0x0", scale = 1 })

-- Anything not named above: preferred mode, placed left to right.
hl.monitor({ output = "", mode = "preferred", position = "auto", scale = 1 })
`
	if got != want {
		t.Errorf("got\n%s\nwant\n%s", got, want)
	}
}

func TestMonitorsLuaNeverDisablesThePanel(t *testing.T) {
	ms, _ := ParseMonitors(fixture(t, "hyprctl-monitors-docked.json"))
	l := LayoutOf(ms)
	l[0].Disabled = true // clamshell's doing, while the lid is shut
	got := string(MonitorsLua(l, nil, ms))
	if !strings.Contains(got, `hl.monitor({ output = "eDP-1", mode = "1920x1200@60.00"`) || strings.Contains(got, `"eDP-1", disabled`) {
		t.Errorf("the panel's rule is clamshell.lua's to disable:\n%s", got)
	}
}
