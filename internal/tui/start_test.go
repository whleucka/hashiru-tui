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

func TestStartOnATab(t *testing.T) {
	for _, tc := range []struct{ link, area, tab string }{
		{"packages.orphans", "Packages", "Orphans"},
		{"Logs.JOURNAL", "Logs", "Journal errors"},
		{"services.failed", "Services", "Failed"},
	} {
		m := New(Options{Source: fakeSource{}})
		if err := m.Start(tc.link, ""); err != nil {
			t.Fatalf("%s: %v", tc.link, err)
		}
		load(t, m)
		if m.area().Name() != tc.area || !m.inArea {
			t.Errorf("%s: opened on %s (in area %v)", tc.link, m.area().Name(), m.inArea)
		}
		// The current tab is the one drawn lit; find it by its place in Tabs.
		tabs := m.area().(Tabbed)
		if got := tabs.Tabs()[tabs.Tab()]; !strings.HasPrefix(strings.ToLower(tc.tab), got) {
			t.Errorf("%s: on tab %q, want %s", tc.link, got, tc.tab)
		}
	}
}

func TestStartRejectsUnknownTabs(t *testing.T) {
	m := New(Options{Source: fakeSource{}})
	if err := m.Start("packages.nope", ""); err == nil || !strings.Contains(err.Error(), "orphans") {
		t.Errorf("unknown tab: %v", err)
	}
	if err := m.Start("snapshots.new", ""); err == nil || !strings.Contains(err.Error(), "no tabs") {
		t.Errorf("tab on an area without tabs: %v", err)
	}
}

// Tabs names every tab [ and ] reach, in their order, so a deep link can
// reach each one.
func TestTabsMatchTheTabBar(t *testing.T) {
	m := New(Options{Source: fakeSource{}})
	load(t, m)
	for _, a := range m.areas {
		tabs, ok := a.(Tabbed)
		if !ok {
			continue
		}
		n := len(tabs.Tabs())
		for i := 1; i <= n; i++ {
			a.HandleKey(tea.KeyPressMsg{Code: ']', Text: "]"})
			if want := i % n; tabs.Tab() != want {
				t.Fatalf("%s: after %d ] on tab %d, want %d (Tabs has %d names)", a.Name(), i, tabs.Tab(), want, n)
			}
		}
	}
}
