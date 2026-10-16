package tui

import (
	"os"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

func bindsModel(t *testing.T) *Model {
	t.Helper()
	out, err := os.ReadFile("../system/testdata/hyprctl-binds.txt")
	if err != nil {
		t.Fatal(err)
	}
	m := New(Options{Source: fakeSource{}, Probe: probe{"hyprctl": string(out)}.run})
	load(t, m)
	goTo(t, m, "Keybinds")
	return m
}

func TestKeybindsList(t *testing.T) {
	m := bindsModel(t)
	out := ansi.Strip(m.View().Content)
	if !strings.Contains(out, "Super+/") || !strings.Contains(out, "Show keybinds") {
		t.Errorf("keybinds missing:\n%s", out)
	}
}

func TestKeybindsFilter(t *testing.T) {
	m := bindsModel(t)
	press(m, "/")
	typeText(m, "btop")
	out := ansi.Strip(m.View().Content)
	if !strings.Contains(out, "System monitor (btop)") || strings.Contains(out, "Show keybinds") {
		t.Errorf("filter 'btop':\n%s", out)
	}

	press(m, "enter") // keep the filter, leave the input
	if m.area().(*keybinds).filtering {
		t.Error("enter should leave the input")
	}
	if out := ansi.Strip(m.View().Content); strings.Contains(out, "Show keybinds") {
		t.Error("enter should keep the filter")
	}

	press(m, "esc") // clears the filter
	if out := ansi.Strip(m.View().Content); !strings.Contains(out, "Show keybinds") {
		t.Error("esc should clear the filter")
	}
	if !m.inArea {
		t.Error("the first esc clears the filter; it shouldn't leave the area")
	}
	press(m, "esc")
	if m.inArea {
		t.Error("the second esc returns to the sidebar")
	}
}

func TestKeybindsWithoutHyprland(t *testing.T) {
	m := New(Options{Source: fakeSource{}})
	load(t, m)
	goTo(t, m, "Keybinds")
	if out := m.View().Content; !strings.Contains(out, "hyprctl") {
		t.Errorf("want a not-installed message:\n%s", out)
	}
}
