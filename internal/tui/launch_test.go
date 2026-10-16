package tui

import (
	"context"
	"errors"
	"os/exec"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

type paneCall struct {
	argv         string
	right, focus bool
}

// herdrModel is a model inside herdr, recording the panes it opens.
func herdrModel(t *testing.T, w, h int) (*Model, *[]paneCall) {
	t.Helper()
	var calls []paneCall
	m := New(Options{
		Source: fakeSource{},
		RunInPane: func(_ context.Context, argv []string, _ string, right, focus bool) error {
			calls = append(calls, paneCall{strings.Join(argv, " "), right, focus})
			return nil
		},
		Detach: func([]string) error { t.Error("no GUI launch expected"); return nil },
	})
	m.Update(tea.WindowSizeMsg{Width: w, Height: h})
	return m, &calls
}

func TestLaunchInHerdr(t *testing.T) {
	m, calls := herdrModel(t, 120, 40) // wide: panes go right
	drain(m, m.launch(Launch{Argv: []string{"nmtui"}, Label: "nmtui", Kind: Interactive}))
	drain(m, m.launch(Launch{Argv: []string{"journalctl", "-f"}, Label: "journal", Kind: Watch}))

	want := []paneCall{
		{"nmtui", true, true},
		{"journalctl -f", true, false}, // watched, not typed into
	}
	if len(*calls) != 2 || (*calls)[0] != want[0] || (*calls)[1] != want[1] {
		t.Errorf("panes = %+v, want %+v", *calls, want)
	}
	if !strings.Contains(m.note, "new pane") {
		t.Errorf("note = %q", m.note)
	}
}

func TestLaunchPaneGoesDownWhenTall(t *testing.T) {
	m, calls := herdrModel(t, 80, 60)
	drain(m, m.launch(Launch{Argv: []string{"btop"}, Kind: Watch}))
	if (*calls)[0].right {
		t.Error("80x60 is tall: the pane should go below")
	}
}

func TestLaunchHerdrError(t *testing.T) {
	m := New(Options{
		Source: fakeSource{},
		RunInPane: func(context.Context, []string, string, bool, bool) error {
			return errors.New("no such pane")
		},
	})
	drain(m, m.launch(Launch{Argv: []string{"nmtui"}, Label: "nmtui", Kind: Interactive}))
	if !m.noteErr || !strings.Contains(m.note, "no such pane") {
		t.Errorf("note = %q (err %v)", m.note, m.noteErr)
	}
}

func TestLaunchWithoutHerdrTakesTerminal(t *testing.T) {
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
		RunInPane: func(context.Context, []string, string, bool, bool) error {
			t.Error("GUI apps don't go in panes")
			return nil
		},
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
