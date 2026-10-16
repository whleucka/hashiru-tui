package hashiru

import (
	"os"
	"strings"
	"testing"
)

func TestParseStatus(t *testing.T) {
	f, err := os.Open("testdata/status.txt")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	s, err := ParseStatus(f)
	if err != nil {
		t.Fatal(err)
	}
	if s.Version != "v1.13.0-25-g38e7ede" {
		t.Errorf("Version = %q", s.Version)
	}
	if s.Repo != "/opt/hashiru" {
		t.Errorf("Repo = %q", s.Repo)
	}
	if !s.Dirty {
		t.Error("Dirty = false, want true")
	}
	if !s.BehindOK || s.Behind != 0 {
		t.Errorf("Behind = %d (ok %v), want 0 (ok true)", s.Behind, s.BehindOK)
	}
	if s.Installed.Year() != 2026 || s.Updated.IsZero() {
		t.Errorf("Installed = %v, Updated = %v", s.Installed, s.Updated)
	}
}

func TestParseStatusUnknowns(t *testing.T) {
	// An offline machine with no stamp: empty values mean unknown, not an error,
	// and keys this version doesn't know about are skipped.
	in := "version=\ninstalled=\nbehind=\ndirty=0\nfuture_key=whatever\n"
	s, err := ParseStatus(strings.NewReader(in))
	if err != nil {
		t.Fatal(err)
	}
	if s.BehindOK {
		t.Error("BehindOK = true for an empty behind")
	}
	if !s.Installed.IsZero() {
		t.Errorf("Installed = %v, want zero", s.Installed)
	}
}

func TestParseStatusBadValue(t *testing.T) {
	if _, err := ParseStatus(strings.NewReader("untracked=lots\n")); err == nil {
		t.Error("want an error for a non-numeric untracked")
	}
}
