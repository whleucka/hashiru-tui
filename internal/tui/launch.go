package tui

import (
	"os/exec"
	"syscall"

	tea "charm.land/bubbletea/v2"
)

// Kind is how a launched tool wants to run.
type Kind int

const (
	// Interactive tools want the keyboard: nmtui, $EDITOR, hashiru update.
	Interactive Kind = iota
	// Watch tools are for looking at: journalctl -f, btop.
	Watch
	// GUI apps get their own window: blueman-manager, pavucontrol.
	GUI
)

// Launch is one tool to start.
type Launch struct {
	Argv  []string
	Label string // names the footer note
	Kind  Kind
	// Refresh reloads state when the tool exits. A GUI window runs on its own,
	// so it never refreshes.
	Refresh bool
}

// launch starts l the best way available:
//
//   - GUI: detached, whatever else is true, and the TUI carries on.
//   - Otherwise: the tool takes over this terminal until it exits.
func (m *Model) launch(l Launch) tea.Cmd {
	if l.Label == "" {
		l.Label = l.Argv[0]
	}

	if l.Kind == GUI {
		detach := m.opts.Detach
		return func() tea.Msg {
			if err := detach(l.Argv); err != nil {
				return noteMsg{text: "couldn't open " + l.Label, err: err}
			}
			return noteMsg{text: "opened " + l.Label}
		}
	}

	c := exec.Command(l.Argv[0], l.Argv[1:]...)
	return m.opts.Exec(c, func(err error) tea.Msg {
		return execDoneMsg{label: l.Label, err: err, refresh: l.Refresh}
	})
}

// detach starts argv in its own session, so closing the TUI (or its floating
// window) doesn't take the app with it.
func detach(argv []string) error {
	c := exec.Command(argv[0], argv[1:]...)
	c.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := c.Start(); err != nil {
		return err
	}
	go c.Wait() // reap it when it exits
	return nil
}
