package system

import (
	"context"
	"os"
	"os/exec"
	"testing"
)

func fixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestParseUpdates(t *testing.T) {
	pkgs := ParseUpdates(fixture(t, "checkupdates.txt"))
	if len(pkgs) != 16 {
		t.Fatalf("got %d packages, want 16", len(pkgs))
	}
	want := Package{Name: "bubblewrap", Old: "0.13.0-1", New: "0.13.1-1"}
	if pkgs[0] != want {
		t.Errorf("first = %+v, want %+v", pkgs[0], want)
	}
}

func TestParseUpdatesYayAge(t *testing.T) {
	// yay -Qua appends how long ago the new version was published.
	pkgs := ParseUpdates(fixture(t, "yay-qua.txt"))
	if len(pkgs) != 2 {
		t.Fatalf("got %d packages", len(pkgs))
	}
	if pkgs[0].Name != "spotify" || pkgs[0].New != "1:1.3.4.258-2" {
		t.Errorf("first = %+v", pkgs[0])
	}
}

// fakeRun answers commands by name.
type fakeRun map[string]struct {
	out  string
	code int // exit code; -1 means not installed
}

func (f fakeRun) Run(_ context.Context, name string, _ ...string) ([]byte, error) {
	r, ok := f[name]
	switch {
	case !ok || r.code == -1:
		return nil, &exec.Error{Name: name, Err: exec.ErrNotFound}
	case r.code != 0:
		return []byte(r.out), exitErr(r.code)
	}
	return []byte(r.out), nil
}

func TestPendingNothingToDo(t *testing.T) {
	run := fakeRun{
		"checkupdates": {code: 2}, // checkupdates: 2 means no updates
		"yay":          {code: 1}, // yay -Qua: 1 with no output means none
	}
	p := Pending(context.Background(), run.Run)
	if p.PacmanErr != nil || p.AURErr != nil {
		t.Fatalf("errors: %v / %v", p.PacmanErr, p.AURErr)
	}
	if len(p.Pacman) != 0 || len(p.AUR) != 0 || p.AURMissing {
		t.Errorf("pending = %+v", p)
	}
}

func TestPendingErrorsAndMissing(t *testing.T) {
	run := fakeRun{
		"checkupdates": {out: "", code: 1},
		"yay":          {code: -1},
	}
	p := Pending(context.Background(), run.Run)
	if p.PacmanErr == nil {
		t.Error("checkupdates exit 1 is an error")
	}
	if !p.AURMissing || p.AURErr != nil {
		t.Errorf("no yay should be AURMissing, not an error: %+v", p)
	}
}

func TestPendingCounts(t *testing.T) {
	run := fakeRun{
		"checkupdates": {out: "a 1-1 -> 2-1\nb 1-1 -> 1-2\n"},
		"yay":          {out: "c 1 -> 2 [1d]\n"},
	}
	p := Pending(context.Background(), run.Run)
	if len(p.Pacman) != 2 || len(p.AUR) != 1 {
		t.Errorf("pending = %+v", p)
	}
	if p.PacmanErr != nil || p.AURErr != nil {
		t.Errorf("errors: %v / %v", p.PacmanErr, p.AURErr)
	}
}

// codeErr stands in for *exec.ExitError, which tests can't construct.
type codeErr int

func (c codeErr) Error() string { return "exit status " + string(rune('0'+int(c))) }
func (c codeErr) ExitCode() int { return int(c) }

func exitErr(code int) error { return codeErr(code) }
