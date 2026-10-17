package tui

import (
	"context"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

func ctrlP() tea.KeyPressMsg { return tea.KeyPressMsg{Code: 'p', Mod: tea.ModCtrl} }

func typeText(m *Model, s string) {
	for _, r := range s {
		_, cmd := m.Update(tea.KeyPressMsg{Code: r, Text: string(r)})
		drain(m, cmd)
	}
}

func TestPaletteRunsAnAction(t *testing.T) {
	var ran []string
	m := New(Options{
		Source: fakeSource{},
		RunInPane: func(_ context.Context, argv []string, _ string, _, _ bool) error {
			ran = append(ran, strings.Join(argv, " "))
			return nil
		},
	})
	load(t, m)

	m.Update(ctrlP())
	if !m.pal.open {
		t.Fatal("ctrl+p should open the palette")
	}
	out := m.View().Content
	for _, want := range []string{"Go to Updates", "Update the system"} {
		if !strings.Contains(out, want) {
			t.Errorf("palette missing %q:\n%s", want, out)
		}
	}

	// Words in any order, any case.
	typeText(m, "SYSTEM upd")
	if got := m.pal.visible(); len(got) != 1 || !strings.Contains(got[0].title, "Update the system") {
		t.Fatalf("filtered to %+v", got)
	}

	press(m, "enter")
	if m.pal.open {
		t.Error("enter should close the palette")
	}
	if len(ran) != 1 || ran[0] != "hashiru update" {
		t.Errorf("ran %v", ran)
	}
}

func TestPaletteGoesToArea(t *testing.T) {
	m := New(Options{Source: fakeSource{}})
	load(t, m)
	press(m, ":") // from the sidebar
	if !m.pal.open {
		t.Fatal(": in the sidebar should open the palette")
	}
	typeText(m, "go logs")
	press(m, "enter")
	if m.area().Name() != "Logs" || !m.inArea {
		t.Errorf("area = %s, inArea %v", m.area().Name(), m.inArea)
	}
}

// An area's action shows its area: run from Snapshots, a package search
// is no use searching in a Packages you can't see.
func TestPaletteActionGoesToItsArea(t *testing.T) {
	m := New(Options{Source: fakeSource{}})
	load(t, m)
	goTo(t, m, "Snapshots")
	m.Update(ctrlP())
	typeText(m, "search the repos")
	press(m, "enter")
	if m.area().Name() != "Packages" || !m.inArea {
		t.Fatalf("area = %s, inArea %v", m.area().Name(), m.inArea)
	}
	if !m.typing() {
		t.Error("the search box should have the keyboard")
	}

	// A shell action stays where it is.
	goTo(t, m, "Snapshots")
	m.Update(ctrlP())
	typeText(m, "show keys")
	press(m, "enter")
	if m.area().Name() != "Snapshots" {
		t.Errorf("shell.keys moved to %s", m.area().Name())
	}
}

func TestPaletteEscCloses(t *testing.T) {
	m := New(Options{Source: fakeSource{}})
	load(t, m)
	m.Update(ctrlP())
	_, cmd := m.Update(key("q"))
	if cmd != nil {
		if _, quit := cmd().(tea.QuitMsg); quit {
			t.Fatal("q in the palette should be text, not quit")
		}
	}
	press(m, "esc")
	if m.pal.open {
		t.Error("esc should close the palette")
	}
}

func TestPaletteMovesSelection(t *testing.T) {
	m := New(Options{Source: fakeSource{}})
	load(t, m)
	m.Update(ctrlP())
	m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	m.Update(tea.KeyPressMsg{Code: tea.KeyUp})
	if m.pal.cursor != 1 {
		t.Errorf("cursor = %d, want 1", m.pal.cursor)
	}
}

func TestPalettePlaceholderShowsInFull(t *testing.T) {
	m := New(Options{Source: fakeSource{}})
	load(t, m)
	m.Update(ctrlP())
	if out := ansi.Strip(m.View().Content); !strings.Contains(out, "type to search") {
		t.Errorf("placeholder cut short:\n%s", out)
	}
}

func TestActionIDsAreUniqueAndWellFormed(t *testing.T) {
	m := New(Options{Source: fakeSource{}})
	load(t, m)
	seen := map[string]bool{}
	for _, a := range m.allActions() {
		area, verb, ok := strings.Cut(a.ID, ".")
		if !ok || area == "" || verb == "" || a.ID != strings.ToLower(a.ID) || strings.ContainsAny(a.ID, " /") {
			t.Errorf("%q: ID %q isn't <area>.<verb>", a.Title, a.ID)
		}
		if seen[a.ID] {
			t.Errorf("duplicate ID %q", a.ID)
		}
		seen[a.ID] = true
	}
	for _, id := range []string{"updates.update", "snapshots.new", "config.edit-hashiru-conf", "config.edit-hypr-local-lua", "config.edit-zshrc-local", "shell.quit"} {
		if !seen[id] {
			t.Errorf("no action %q", id)
		}
	}
}

func TestPaletteListsRecentsFirstAndRemembersThem(t *testing.T) {
	path := t.TempDir() + "/state/recent"
	opts := Options{Source: fakeSource{}, RecentPath: path, RunInPane: func(context.Context, []string, string, bool, bool) error { return nil }}
	m := New(opts)
	load(t, m)
	m.Update(ctrlP())
	typeText(m, "follow kernel")
	press(m, "enter")

	// A new model, as after a restart, reads them back.
	m = New(opts)
	load(t, m)
	m.Update(ctrlP())
	got := m.pal.visible()
	if len(got) == 0 || got[0].id != "logs.follow-kernel" || !got[0].recent {
		t.Fatalf("first item = %+v, want the recent logs.follow-kernel", got[0])
	}
	if out := ansi.Strip(m.View().Content); !strings.Contains(out, "recent · K · logs.follow-kernel") {
		t.Errorf("recent not marked:\n%s", out)
	}
	// Typing searches everything once, without the recents section.
	typeText(m, "kernel")
	if got := m.pal.visible(); len(got) != 1 || got[0].recent {
		t.Errorf("filtered = %+v", got)
	}
}

func TestRecentKeepsTheNewestTen(t *testing.T) {
	path := t.TempDir() + "/recent"
	var ids []string
	for _, id := range []string{"a.1", "a.2", "a.3", "a.4", "a.5", "a.6", "a.7", "a.8", "a.9", "a.10", "a.11", "a.3"} {
		ids = addRecent(path, ids, id)
	}
	if want := "a.3 a.11 a.10 a.9 a.8 a.7 a.6 a.5 a.4 a.2"; strings.Join(ids, " ") != want {
		t.Errorf("ids = %v, want %s", ids, want)
	}
	if got := loadRecent(path); strings.Join(got, " ") != strings.Join(ids, " ") {
		t.Errorf("saved %v, read back %v", ids, got)
	}
	if got := loadRecent(t.TempDir() + "/missing"); got != nil {
		t.Errorf("missing file = %v", got)
	}
}

func TestPaletteHasShellActions(t *testing.T) {
	m := New(Options{Source: fakeSource{}})
	load(t, m)
	goTo(t, m, "Updates")
	m.Update(ctrlP())
	typeText(m, "show keys")
	press(m, "enter")
	if !m.keyHelp {
		t.Error("'Show keys' didn't open the popup")
	}
}
