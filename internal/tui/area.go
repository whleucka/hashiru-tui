package tui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/whleucka/hashiru-tui/internal/system"
)

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
	// Keys are the area's keys right now, for the footer and the keymap
	// popup. They follow the area's mode (filtering, reading a diff, ...).
	Keys() []Key
	// Actions are what the area can do, for the command palette.
	Actions() []Action
}

// Tabbed is an area with tabs ([ and ] go through them). Tabs are short,
// stable names in that order, for deep links like `hashiru-tui logs.journal`;
// the labels drawn in the tab bar can carry counts and change.
type Tabbed interface {
	Tabs() []string
	Tab() int
	SetTab(int)
}

// Action is something an area can do, named for the palette.
type Action struct {
	// ID is "<area>.<verb>": stable, for deep links and the palette's
	// recents. Titles can change; IDs don't.
	ID    string
	Title string // "Update the system"
	Key   string // the area's key for it, if it has one
	Run   func() tea.Cmd
}

// Key is one key (or a few that do the same thing) and what it does.
type Key struct {
	Keys  string // "j/k"
	Desc  string // "move"
	Group string // the popup's heading for it; "" for the area's main keys
}

// env is what an area gets from the shell.
type env struct {
	src   Source
	probe system.Run
	state *State
	// launch starts another tool; see Model.launch.
	launch func(Launch) tea.Cmd
	// refresh reloads the shared state.
	refresh func() tea.Cmd
	// jump focuses the named area and hands it msg, so one area can send
	// you to a thing in another (a failed unit in Logs → Services).
	jump func(area string, msg tea.Msg) tea.Cmd
}

// areaSizeMsg tells areas how much room the shell gives them.
type areaSizeMsg struct{ width, height int }

func newAreas(e *env, opts Options, graphics bool) []Area {
	return []Area{
		newOverview(e),
		newUpdates(e),
		newPackages(e, opts.PacmanCache),
		newKeybinds(e),
		newConnectivity(e),
		newWallpaper(e, opts.Home, graphics),
		newSnapshots(e),
		newConfig(e, opts.Home),
		newLogs(e),
		newServices(e),
		newHardware(e, opts),
	}
}
