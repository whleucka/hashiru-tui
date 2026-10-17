// Package system reads the machine: package updates, network, keybinds and
// the like. Every command goes through a Run function the caller supplies,
// and every parser is a pure function, so tests need no real tools.
package system

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

// Run runs a command and returns its stdout. A non-zero exit is an error
// with an ExitCode() method, as *exec.ExitError has.
type Run func(ctx context.Context, name string, args ...string) ([]byte, error)

// Command is the real Run. stderr goes into the error, if there is any.
func Command(ctx context.Context, name string, args ...string) ([]byte, error) {
	c := exec.CommandContext(ctx, name, args...)
	var stderr bytes.Buffer
	c.Stderr = &stderr
	out, err := c.Output()
	if err != nil && stderr.Len() > 0 {
		return out, fmt.Errorf("%s: %w: %s", name, err, strings.TrimSpace(stderr.String()))
	}
	return out, err
}

// exitCode is err's exit code, if it is an exit.
func exitCode(err error) (int, bool) {
	var e interface{ ExitCode() int }
	if errors.As(err, &e) {
		return e.ExitCode(), true
	}
	return 0, false
}

// missing reports whether err means the command isn't installed.
func missing(err error) bool {
	return errors.Is(err, exec.ErrNotFound)
}

// Missing reports whether err means the command isn't installed.
func Missing(err error) bool { return missing(err) }
