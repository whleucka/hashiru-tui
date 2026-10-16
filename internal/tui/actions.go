package tui

import (
	"context"
	"os/exec"

	tea "charm.land/bubbletea/v2"
)

// run runs argv in a herdr pane when there is one, otherwise hands it this
// terminal until it exits. label names the pane and the footer note.
//
// The pane goes to the right when the terminal is wider than it is tall (cells
// are about twice as tall as wide), below otherwise, the same rule mantis-tui
// uses.
func (m *Model) run(argv []string, label string, focus bool) tea.Cmd {
	if m.opts.RunInPane == nil {
		c := exec.Command(argv[0], argv[1:]...)
		return tea.ExecProcess(c, func(err error) tea.Msg {
			return execDoneMsg{label: label, err: err}
		})
	}

	run := m.opts.RunInPane
	right := m.width >= 2*m.height
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), paneTimeout)
		defer cancel()
		if err := run(ctx, argv, label, right, focus); err != nil {
			return noteMsg{text: label, err: err}
		}
		// A pane runs on its own, so there is no "finished" to wait for: press
		// r once it is done.
		return noteMsg{text: label + " opened in a new pane (r to refresh after)"}
	}
}
