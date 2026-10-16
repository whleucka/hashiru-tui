package tui

import (
	"context"
	"errors"
	"os/exec"
	"strings"

	tea "charm.land/bubbletea/v2"
)

// probe is a fake system.Run: canned stdout per command name, and "not
// installed" for anything else.
type probe map[string]string

func (p probe) run(_ context.Context, name string, _ ...string) ([]byte, error) {
	out, ok := p[name]
	if !ok {
		return nil, &exec.Error{Name: name, Err: exec.ErrNotFound}
	}
	return []byte(out), nil
}

// Tests never touch the real machine: unless a test passes its own Probe,
// every command reads as not installed.
func init() { defaultProbe = probe{}.run }

// recordExec is an Options.Exec that records argv and exits at once.
func recordExec(ran *[]string) func(*exec.Cmd, tea.ExecCallback) tea.Cmd {
	return func(c *exec.Cmd, done tea.ExecCallback) tea.Cmd {
		*ran = append(*ran, strings.Join(c.Args, " "))
		return func() tea.Msg { return done(nil) }
	}
}

// Nor the network.
func init() {
	defaultGet = func(context.Context, string) ([]byte, error) {
		return nil, errors.New("offline (test)")
	}
}
