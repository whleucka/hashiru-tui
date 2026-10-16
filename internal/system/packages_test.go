package system

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// cmdRun answers by the whole command line; anything else isn't installed.
type cmdRun map[string]struct {
	out  string
	code int
}

func (f cmdRun) Run(_ context.Context, name string, args ...string) ([]byte, error) {
	r, ok := f[strings.Join(append([]string{name}, args...), " ")]
	switch {
	case !ok:
		return nil, &exec.Error{Name: name, Err: exec.ErrNotFound}
	case r.code != 0:
		return []byte(r.out), exitErr(r.code)
	}
	return []byte(r.out), nil
}

func TestParseManifest(t *testing.T) {
	got := ParseManifest("aur.txt", []byte("# AUR packages\n\nyay-bin\n  # indented comment\n  claude-code  \n"))
	want := []ManifestEntry{{"yay-bin", "aur.txt", true}, {"claude-code", "aur.txt", true}}
	if !slices.Equal(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestReadPackagesOnThisMachine(t *testing.T) {
	run := cmdRun{
		"pacman -Qq":   {out: string(fixture(t, "pacman-Qq.txt"))},
		"pacman -Qqe":  {out: string(fixture(t, "pacman-Qqe.txt"))},
		"pacman -Qqm":  {out: string(fixture(t, "pacman-Qqm.txt"))},
		"pacman -Qdtq": {out: string(fixture(t, "pacman-Qdtq.txt"))},
	}
	p := ReadPackages(context.Background(), run.Run, "testdata/repo", t.TempDir())
	if p.PacmanErr != nil || p.ManifestErr != nil {
		t.Fatalf("errors: %v, %v", p.PacmanErr, p.ManifestErr)
	}
	if len(p.Missing) != 0 {
		t.Errorf("missing on a machine Hashiru installed: %+v", p.Missing)
	}
	if len(p.Managed) < 150 {
		t.Errorf("only %d managed", len(p.Managed))
	}
	if !slices.ContainsFunc(p.Managed, func(e ManifestEntry) bool { return e.Name == "yay-bin" && e.AUR }) {
		t.Error("yay-bin not managed from aur.txt")
	}
	if slices.Contains(p.Yours, "chromium") {
		t.Error("chromium is in apps.txt, not yours")
	}
	// Everything explicit is either Hashiru's or yours.
	explicit := strings.Fields(string(fixture(t, "pacman-Qqe.txt")))
	for _, name := range explicit {
		if !slices.Contains(p.Yours, name) && !slices.ContainsFunc(p.Managed, func(e ManifestEntry) bool { return e.Name == name }) {
			t.Errorf("%s is explicit but neither managed nor yours", name)
		}
	}
	if !slices.Equal(p.Orphans, []string{"wrk-debug"}) || len(p.Foreign) != 7 {
		t.Errorf("orphans %v, foreign %v", p.Orphans, p.Foreign)
	}
}

func TestClassify(t *testing.T) {
	manifests := []ManifestEntry{
		{Name: "git", File: "base.txt"},
		{Name: "libnotify", File: "wayland.txt"}, // installed as a dependency
		{Name: "gone", File: "apps.txt"},
		{Name: "fonts-group", File: "fonts.txt"},
		{Name: "half-group", File: "fonts.txt"},
	}
	q := PacmanQuery{
		Installed: []string{"git", "libnotify", "ttf-a", "ttf-b", "htop", "ttf-c"},
		Explicit:  []string{"git", "ttf-a", "ttf-b", "htop"},
		Groups: map[string][]string{
			"fonts-group": {"ttf-a", "ttf-b"},
			"half-group":  {"ttf-c", "ttf-missing"},
		},
	}
	p := Classify(manifests, q)
	names := func(es []ManifestEntry) (out []string) {
		for _, e := range es {
			out = append(out, e.Name)
		}
		return out
	}
	if got := names(p.Managed); !slices.Equal(got, []string{"git", "libnotify", "fonts-group"}) {
		t.Errorf("managed %v", got)
	}
	if got := names(p.Missing); !slices.Equal(got, []string{"gone", "half-group"}) {
		t.Errorf("missing %v", got)
	}
	if !slices.Equal(p.Yours, []string{"htop"}) {
		t.Errorf("yours %v (a group's members are Hashiru's)", p.Yours)
	}
}

func TestReadPackagesExpandsGroupsOnlyForMissingNames(t *testing.T) {
	run := cmdRun{
		"pacman -Qq":              {out: "git\nttf-a\n"},
		"pacman -Qqe":             {out: "git\nttf-a\n"},
		"pacman -Qqm":             {code: 1}, // nothing foreign
		"pacman -Qdtq":            {code: 1}, // no orphans
		"pacman -Sgq fonts-group": {out: "ttf-a\n"},
	}
	repo := t.TempDir()
	os.MkdirAll(filepath.Join(repo, "pacman"), 0o755)
	os.WriteFile(filepath.Join(repo, "pacman", "base.txt"), []byte("git\nfonts-group\n"), 0o644)
	p := ReadPackages(context.Background(), run.Run, repo, t.TempDir())
	if p.PacmanErr != nil || len(p.Managed) != 2 || len(p.Yours) != 0 || p.Orphans != nil || p.Foreign != nil {
		t.Errorf("%+v", p)
	}
}

func TestReadPackagesWithoutPacman(t *testing.T) {
	p := ReadPackages(context.Background(), cmdRun{}.Run, "testdata/repo", t.TempDir())
	if !errors.Is(p.PacmanErr, exec.ErrNotFound) || p.Managed != nil {
		t.Errorf("%+v", p)
	}
}

func TestReadPackagesWithoutARepo(t *testing.T) {
	run := cmdRun{"pacman -Qq": {out: "htop\n"}, "pacman -Qqe": {out: "htop\n"}, "pacman -Qqm": {code: 1}, "pacman -Qdtq": {code: 1}}
	p := ReadPackages(context.Background(), run.Run, "", t.TempDir())
	if p.ManifestErr == nil || !slices.Equal(p.Yours, []string{"htop"}) {
		t.Errorf("no repo: everything explicit is yours, and say why: %+v", p)
	}
}

func TestCacheSize(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "a.pkg.tar.zst"), make([]byte, 1000), 0o644)
	os.WriteFile(filepath.Join(dir, "a.pkg.tar.zst.sig"), make([]byte, 24), 0o644)
	os.Mkdir(filepath.Join(dir, "download-xyz"), 0o755)
	if n, err := dirSize(dir); err != nil || n != 1024 {
		t.Errorf("size %d, %v", n, err)
	}
	if _, err := dirSize(filepath.Join(dir, "nope")); err == nil {
		t.Error("missing dir: no error")
	}
}
