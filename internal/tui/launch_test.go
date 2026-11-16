package tui

import (
	"os/exec"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

// recordLaunch is recordExec for tests that want the label too: it hands rec
// each command and its label, and the command exits at once.
func recordLaunch(rec func(argv []string, label string)) func(*exec.Cmd, tea.ExecCallback) tea.Cmd {
	return func(c *exec.Cmd, done tea.ExecCallback) tea.Cmd {
		msg := done(nil)
		label := ""
		if d, ok := msg.(execDoneMsg); ok {
			label = d.label
		}
		rec(c.Args, label)
		return func() tea.Msg { return msg }
	}
}

func TestLaunchTakesTerminal(t *testing.T) {
	var ran []string
	calls := 0
	m := New(Options{
		Source: fakeSource{calls: &calls},
		Exec: func(c *exec.Cmd, done tea.ExecCallback) tea.Cmd {
			ran = append(ran, strings.Join(c.Args, " "))
			return func() tea.Msg { return done(nil) }
		},
	})

	drain(m, m.launch(Launch{Argv: []string{"hashiru", "update"}, Label: "hashiru update", Kind: Interactive, Refresh: true}))
	if len(ran) != 1 || ran[0] != "hashiru update" {
		t.Fatalf("ran %v", ran)
	}
	if calls != 1 {
		t.Errorf("Refresh: state reloaded %d times after exit, want 1", calls)
	}
	if !strings.Contains(m.note, "hashiru update finished") {
		t.Errorf("note = %q", m.note)
	}

	drain(m, m.launch(Launch{Argv: []string{"btop"}, Label: "btop", Kind: Watch}))
	if len(ran) != 2 || calls != 1 {
		t.Errorf("Watch without Refresh: ran %v, reloads %d", ran, calls)
	}
}

func TestLaunchGUIDetaches(t *testing.T) {
	var started []string
	m := New(Options{
		Source: fakeSource{},
		Detach: func(argv []string) error {
			started = append(started, strings.Join(argv, " "))
			return nil
		},
	})
	drain(m, m.launch(Launch{Argv: []string{"pavucontrol"}, Label: "pavucontrol", Kind: GUI}))
	if len(started) != 1 || started[0] != "pavucontrol" {
		t.Errorf("started %v", started)
	}
	if m.noteErr {
		t.Errorf("note = %q", m.note)
	}

	m.opts.Detach = func([]string) error { return exec.ErrNotFound }
	drain(m, m.launch(Launch{Argv: []string{"blueman-manager"}, Label: "blueman", Kind: GUI}))
	if !m.noteErr {
		t.Error("a GUI app that can't start should show an error")
	}
}
