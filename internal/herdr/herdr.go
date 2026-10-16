// Package herdr opens panes beside hashiru-tui when it runs inside herdr, the
// same way mantis-tui does: `herdr pane split` prints JSON with the new
// pane at .result.pane.pane_id, and `herdr pane run` types a command into that
// pane's shell and presses Enter.
//
// A pane is a real shell, so sudo prompts, pinentry and $EDITOR all just work
// there, which is why the TUI prefers a pane over suspending itself.
package herdr

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// Available reports whether we are running inside a herdr pane.
func Available() bool {
	if os.Getenv("HERDR_PANE_ID") == "" {
		return false
	}
	_, err := exec.LookPath("herdr")
	return err == nil
}

// Split splits the calling pane, right or down, and returns the new pane's id.
// focus moves the cursor to it (interactive commands); otherwise focus stays
// in the TUI (a log to watch).
func Split(ctx context.Context, right, focus bool) (string, error) {
	dir := "down"
	if right {
		dir = "right"
	}
	args := []string{"pane", "split", "--current", "--direction", dir}
	if !focus {
		args = append(args, "--no-focus")
	}
	out, err := exec.CommandContext(ctx, "herdr", args...).Output()
	if err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			return "", fmt.Errorf("herdr pane split: %s", strings.TrimSpace(string(ee.Stderr)))
		}
		return "", fmt.Errorf("herdr pane split: %w", err)
	}
	return parseSplit(out)
}

func parseSplit(out []byte) (string, error) {
	var r struct {
		Result struct {
			Pane struct {
				PaneID string `json:"pane_id"`
			} `json:"pane"`
		} `json:"result"`
	}
	if err := json.Unmarshal(out, &r); err != nil {
		return "", fmt.Errorf("herdr pane split: unexpected output: %w", err)
	}
	if r.Result.Pane.PaneID == "" {
		return "", errors.New("herdr pane split: no pane id in response")
	}
	return r.Result.Pane.PaneID, nil
}

// ShellQuote quotes s for a POSIX shell, leaving plain words alone.
func ShellQuote(s string) string {
	if s != "" && strings.Trim(s, "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789-_=+./:,@%") == "" {
		return s
	}
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// CommandLine quotes argv for the shell in a herdr pane.
func CommandLine(argv []string) string {
	q := make([]string, len(argv))
	for i, a := range argv {
		q[i] = ShellQuote(a)
	}
	return strings.Join(q, " ")
}

// Run types argv into pane's shell and presses Enter.
func Run(ctx context.Context, pane string, argv []string) error {
	out, err := exec.CommandContext(ctx, "herdr", "pane", "run", pane, CommandLine(argv)).CombinedOutput()
	if err != nil {
		return fmt.Errorf("herdr pane run: %v: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// Rename sets the pane's label in herdr's UI.
func Rename(ctx context.Context, pane, label string) error {
	out, err := exec.CommandContext(ctx, "herdr", "pane", "rename", pane, label).CombinedOutput()
	if err != nil {
		return fmt.Errorf("herdr pane rename: %v: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// Open splits a pane, names it, and runs argv in it. A failed rename doesn't
// stop the command; its error is returned once the command is running.
func Open(ctx context.Context, argv []string, label string, right, focus bool) error {
	pane, err := Split(ctx, right, focus)
	if err != nil {
		return err
	}
	renameErr := Rename(ctx, pane, label)
	if err := Run(ctx, pane, argv); err != nil {
		return err
	}
	return renameErr
}
