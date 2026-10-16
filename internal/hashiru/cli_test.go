package hashiru

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// fakeHashiru writes a shell script standing in for the hashiru CLI.
func fakeHashiru(t *testing.T, body string) CLI {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "hashiru")
	if err := os.WriteFile(bin, []byte("#!/bin/sh\n"+body), 0o755); err != nil {
		t.Fatal(err)
	}
	return CLI{Bin: bin}
}

func TestDoctorFailingCheckIsNotAnError(t *testing.T) {
	// doctor exits 1 when a check fails; the report is still the answer.
	c := fakeHashiru(t, `echo '{"checks":[{"section":"Install","status":"fail","message":"x"}],"summary":{"ok":0,"warn":0,"fail":1}}'; exit 1`)
	rep, err := c.Doctor(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if rep.Summary.Fail != 1 {
		t.Errorf("Fail = %d, want 1", rep.Summary.Fail)
	}
}

func TestDoctorOtherExitIsAnError(t *testing.T) {
	c := fakeHashiru(t, `echo "doctor.sh: unknown option: --json" >&2; exit 2`)
	if _, err := c.Doctor(context.Background()); err == nil {
		t.Error("want an error for exit 2 (an old hashiru without --json)")
	}
}

func TestStatus(t *testing.T) {
	c := fakeHashiru(t, `[ "$1 $2" = "status --porcelain" ] || exit 9; printf 'version=v1.0.0\nbehind=3\n'`)
	s, err := c.Status(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if s.Version != "v1.0.0" || s.Behind != 3 {
		t.Errorf("Status = %+v", s)
	}
}
