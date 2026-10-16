package tui

import (
	tea "charm.land/bubbletea/v2"
)

// hwTab is one of the Hardware area's tabs: a small area of its own, which
// the area routes keys and messages to.
type hwTab interface {
	name() string  // for deep links: "firmware"
	label() string // in the tab bar, with a count if it has one
	warn() bool    // draw the tab in the warning colour
	// update sees every message but key presses, whichever tab is showing.
	update(tea.Msg) tea.Cmd
	handleKey(tea.KeyPressMsg) (tea.Cmd, bool)
	view(width, height int) string
	keys() []Key
	actions() []Action
}

// hardware is the machine's hardware, a tab each: monitors, power,
// firmware.
type hardware struct {
	e    *env
	tabs []hwTab
	tab  int
}

func newHardware(e *env) *hardware {
	return &hardware{e: e, tabs: []hwTab{newFirmware(e)}}
}

func (h *hardware) Name() string { return "Hardware" }

func (h *hardware) Tabs() []string {
	names := make([]string, len(h.tabs))
	for i, t := range h.tabs {
		names[i] = t.name()
	}
	return names
}

func (h *hardware) Tab() int { return h.tab }

func (h *hardware) SetTab(i int) { h.tab = i }

func (h *hardware) Update(msg tea.Msg) tea.Cmd {
	var cmds []tea.Cmd
	for _, t := range h.tabs {
		cmds = append(cmds, t.update(msg))
	}
	return tea.Batch(cmds...)
}

func (h *hardware) HandleKey(msg tea.KeyPressMsg) (tea.Cmd, bool) {
	if !h.Typing() {
		if i, ok := nextTab(msg.String(), h.tab, len(h.tabs)); ok {
			h.tab = i
			return nil, true
		}
	}
	return h.tabs[h.tab].handleKey(msg)
}

// Typing is whether the current tab has a text input focused, so `?` and
// `[` `]` go to it.
func (h *hardware) Typing() bool {
	t, ok := h.tabs[h.tab].(interface{ Typing() bool })
	return ok && t.Typing()
}

func (h *hardware) Keys() []Key {
	keys := h.tabs[h.tab].keys()
	if h.Typing() {
		return keys
	}
	return append([]Key{tabKey}, keys...)
}

// Actions are every tab's, so the palette reaches them all; running one
// shows its tab.
func (h *hardware) Actions() []Action {
	var acts []Action
	for i, t := range h.tabs {
		for _, a := range t.actions() {
			run := a.Run
			a.Run = func() tea.Cmd {
				h.tab = i
				return run()
			}
			acts = append(acts, a)
		}
	}
	return acts
}

func (h *hardware) View(width, height int) string {
	labels := make([]string, len(h.tabs))
	for i, t := range h.tabs {
		labels[i] = t.label()
	}
	bar := tabBar(labels, h.tab, func(i int) bool { return h.tabs[i].warn() })
	return bar + "\n" + h.tabs[h.tab].view(width, height-1)
}
