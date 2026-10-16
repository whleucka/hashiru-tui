package tui

import (
	"context"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestStartOnAnArea(t *testing.T) {
	m := New(Options{Source: fakeSource{}})
	if err := m.Start("snapshots", ""); err != nil {
		t.Fatal(err)
	}
	load(t, m)
	if m.area().Name() != "Snapshots" || !m.inArea {
		t.Errorf("opened on %s (in area %v)", m.area().Name(), m.inArea)
	}
}

func TestStartRunsAnActionOnceSized(t *testing.T) {
	var ran []string
	m := New(Options{
		Source: fakeSource{},
		RunInPane: func(_ context.Context, argv []string, _ string, _, _ bool) error {
			ran = append(ran, strings.Join(argv, " "))
			return nil
		},
	})
	if err := m.Start("", "updates.update"); err != nil {
		t.Fatal(err)
	}
	drain(m, m.Init())
	if len(ran) != 0 {
		t.Fatalf("ran before the window had a size: %v", ran)
	}
	_, cmd := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	drain(m, cmd)
	if len(ran) != 1 || ran[0] != "hashiru update" {
		t.Errorf("ran %v, want hashiru update once", ran)
	}
	if m.area().Name() != "Updates" || !m.inArea {
		t.Errorf("opened on %s, want the action's area", m.area().Name())
	}
	// A later resize doesn't run it again.
	_, cmd = m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	drain(m, cmd)
	if len(ran) != 1 {
		t.Errorf("resize ran it again: %v", ran)
	}
}

func TestStartRejectsUnknownNames(t *testing.T) {
	m := New(Options{Source: fakeSource{}})
	if err := m.Start("nope", ""); err == nil || !strings.Contains(err.Error(), "Overview") {
		t.Errorf("unknown area: %v", err)
	}
	if err := m.Start("", "nope.x"); err == nil {
		t.Error("unknown action: no error")
	}
}
