package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
)

// Start sets where the TUI opens: focused on an area (by name, any case),
// or one of its tabs ("logs.journal"), and/or running an action by ID once
// the window has a size, for deep links like `hashiru-tui --run
// snapshots.new`. Call it before the program runs; an unknown name is an
// error saying what there is.
func (m *Model) Start(area, run string) error {
	if area != "" {
		name, tab, hasTab := strings.Cut(area, ".")
		i := m.areaIndex(name)
		if i < 0 {
			return fmt.Errorf("no area %q; there are: %s", name, strings.Join(m.AreaNames(), ", "))
		}
		if hasTab {
			if err := m.openTab(m.areas[i], tab); err != nil {
				return err
			}
		}
		m.sel, m.inArea = i, true
	}
	if run != "" {
		act, ok := m.action(run)
		if !ok {
			return fmt.Errorf("no action %q; --list-actions lists them", run)
		}
		// Open on the action's area unless one was asked for.
		prefix, _, _ := strings.Cut(act.ID, ".")
		if i := m.areaIndex(prefix); i >= 0 && area == "" {
			m.sel, m.inArea = i, true
		}
		m.startRun = &act
	}
	return nil
}

// openTab switches a to the tab named tab (any case).
func (m *Model) openTab(a Area, tab string) error {
	t, ok := a.(Tabbed)
	if !ok {
		return fmt.Errorf("%s has no tabs", a.Name())
	}
	for i, name := range t.Tabs() {
		if strings.EqualFold(name, tab) {
			t.SetTab(i)
			return nil
		}
	}
	return fmt.Errorf("no tab %q in %s; there are: %s", tab, a.Name(), strings.Join(t.Tabs(), ", "))
}

// jump focuses the named area and delivers msg to it.
func (m *Model) jump(area string, msg tea.Msg) tea.Cmd {
	i := m.areaIndex(area)
	if i < 0 {
		return nil
	}
	m.sel, m.inArea = i, true
	return m.areas[i].Update(msg)
}

// AreaNames are the sidebar's areas, in order.
func (m *Model) AreaNames() []string {
	names := make([]string, len(m.areas))
	for i, a := range m.areas {
		names[i] = a.Name()
	}
	return names
}

// Actions is every action there is, for --list-actions.
func (m *Model) Actions() []Action { return m.allActions() }

func (m *Model) areaIndex(name string) int {
	for i, a := range m.areas {
		if strings.EqualFold(a.Name(), name) {
			return i
		}
	}
	return -1
}

func (m *Model) action(id string) (Action, bool) {
	for _, a := range m.allActions() {
		if a.ID == id {
			return a, true
		}
	}
	return Action{}, false
}
