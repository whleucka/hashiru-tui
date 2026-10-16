package tui

import (
	"context"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
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
