package tui

import tea "charm.land/bubbletea/v2"

// Area is one entry in the sidebar: Overview, Updates, Logs, and so on.
// Adding one is a type implementing this and a line in newAreas.
type Area interface {
	Name() string
	// Update sees every message except key presses, so an area can pick up
	// the results of its own commands and the shared state's reloads.
	Update(tea.Msg) tea.Cmd
	// HandleKey gets first refusal on keys while the area is selected. It
	// reports whether it used the key; unused keys fall back to the shell
	// (esc to the sidebar, q to quit, ...).
	HandleKey(tea.KeyPressMsg) (tea.Cmd, bool)
	View(width, height int) string
	Help() string
	// Actions are what the area can do, for the command palette.
	Actions() []Action
}

// Action is something an area can do, named for the palette.
type Action struct {
	Title string // "Update the system"
	Key   string // the area's key for it, if it has one
	Run   func() tea.Cmd
}

// env is what an area gets from the shell.
type env struct {
	src   Source
	state *State
	// run launches argv; see Model.run.
	run func(argv []string, label string, focus bool) tea.Cmd
	// refresh reloads the shared state.
	refresh func() tea.Cmd
}

// areaSizeMsg tells areas how much room the shell gives them.
type areaSizeMsg struct{ width, height int }

func newAreas(e *env) []Area {
	return []Area{
		newOverview(e),
		newUpdates(e),
		newLogs(e),
	}
}
