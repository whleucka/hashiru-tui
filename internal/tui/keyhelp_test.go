package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

func TestKeyHelpOpensOverTheArea(t *testing.T) {
	m := New(Options{Source: fakeSource{}})
	load(t, m)
	goTo(t, m, "Logs")
	press(m, "?")
	if !m.keyHelp {
		t.Fatal("? didn't open the popup")
	}
	out := ansi.Strip(m.View().Content)
	for _, want := range []string{"Logs", "Sources", "hashiru log", "Global", "ctrl+p"} {
		if !strings.Contains(out, want) {
			t.Errorf("popup missing %q:\n%s", want, out)
		}
	}
	// The area is still drawn around the popup.
	if !strings.Contains(out, "走 hashiru") {
		t.Errorf("popup replaced the screen instead of covering part of it:\n%s", out)
	}
}

func TestKeyHelpFromSidebarShowsTheSelectedArea(t *testing.T) {
	m := New(Options{Source: fakeSource{}})
	load(t, m)
	press(m, areaKey(t, m, "Snapshots"), "?")
	out := ansi.Strip(m.keyHelpView())
	if !strings.Contains(out, "new snapshot") || !strings.Contains(out, "jump to area") {
		t.Errorf("sidebar popup:\n%s", out)
	}
}

func TestKeyHelpTakesEveryKey(t *testing.T) {
	for _, closer := range []string{"?", "esc", "q"} {
		m := New(Options{Source: fakeSource{}})
		load(t, m)
		goTo(t, m, "Updates")
		press(m, "?")
		_, cmd := m.Update(key("j")) // would move Updates' cursor
		drain(m, cmd)
		_, cmd = m.Update(key(closer))
		if cmd != nil {
			if _, quit := cmd().(tea.QuitMsg); quit {
				t.Errorf("%s quit instead of closing the popup", closer)
			}
		}
		if m.keyHelp {
			t.Errorf("%s didn't close the popup", closer)
		}
		if !m.inArea {
			t.Errorf("%s left the area", closer)
		}
	}
}

func TestQuestionMarkTypesIntoAFocusedInput(t *testing.T) {
	m := bindsModel(t)
	press(m, "/", "?")
	if m.keyHelp {
		t.Error("? opened the popup while filtering")
	}
	if got := m.areas[m.sel].(*keybinds).input.Value(); got != "?" {
		t.Errorf("filter = %q, want ?", got)
	}
	if strings.Contains(m.footer(), "? keys") {
		t.Errorf("footer offers ? while it types: %q", m.footer())
	}
}

func TestFooterOffersKeyHelp(t *testing.T) {
	m := New(Options{Source: fakeSource{}})
	load(t, m)
	m.width = 30 // too narrow for the keys; "? keys" still shows
	if f := ansi.Strip(m.footer()); !strings.HasSuffix(f, "? keys") {
		t.Errorf("footer: %q", f)
	}
}

func TestWallpaperPreviewHidesUnderAnOverlay(t *testing.T) {
	m := New(Options{Source: fakeSource{}})
	load(t, m)
	goTo(t, m, "Wallpaper")
	press(m, "?")
	w := m.area().(*wallpaper)
	if !w.covered {
		t.Error("wallpaper not told about the popup")
	}
	press(m, "esc")
	if w.covered {
		t.Error("wallpaper still covered after closing")
	}
}
