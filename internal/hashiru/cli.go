package hashiru

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

// CLI runs the hashiru command. Bin is the executable, "hashiru" on PATH by
// default; tests point it at a fake.
type CLI struct {
	Bin string
}

func (c CLI) bin() string {
	if c.Bin == "" {
		return "hashiru"
	}
	return c.Bin
}

// Status runs `hashiru status --porcelain`. It fetches the upstream, so it can
// take up to ~10s on a bad network: run it off the UI thread.
func (c CLI) Status(ctx context.Context) (Status, error) {
	out, err := c.run(ctx, "status", "--porcelain")
	if err != nil {
		return Status{}, err
	}
	return ParseStatus(bytes.NewReader(out))
}

// Doctor runs `hashiru doctor --json`. Doctor exits 1 when a check fails, which
// is a result, not an error, so that exit code is accepted.
func (c CLI) Doctor(ctx context.Context) (Report, error) {
	out, err := c.run(ctx, "doctor", "--json")
	var ee *exec.ExitError
	if err != nil && !(errors.As(err, &ee) && ee.ExitCode() == 1) {
		return Report{}, err
	}
	return ParseDoctor(bytes.NewReader(out))
}

// Argv is the command line for a hashiru subcommand, for running it somewhere
// other than here: a herdr pane, or the terminal via tea.ExecProcess.
func (c CLI) Argv(args ...string) []string {
	return append([]string{c.bin()}, args...)
}

func (c CLI) run(ctx context.Context, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, c.bin(), args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		if msg := strings.TrimSpace(stderr.String()); msg != "" {
			return out, fmt.Errorf("hashiru %s: %w: %s", strings.Join(args, " "), err, msg)
		}
		return out, fmt.Errorf("hashiru %s: %w", strings.Join(args, " "), err)
	}
	return out, nil
}
