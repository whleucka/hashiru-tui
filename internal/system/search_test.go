package system

import (
	"context"
	"testing"
)

func TestParseSearch(t *testing.T) {
	repo := ParseSearch(fixture(t, "pacman-Ss.txt"))
	if len(repo) != 2 || repo[0] != (Found{Repo: "extra", Name: "lazygit", Version: "0.66.1-1", Description: "Simple terminal UI for git commands", Installed: true}) {
		t.Errorf("pacman: %+v", repo)
	}
	aur := ParseSearch(fixture(t, "yay-Ssa.txt"))
	if len(aur) != 7 {
		t.Fatalf("yay: %d results", len(aur))
	}
	if a := aur[0]; a.Name != "lazyssh" || !a.AUR() || a.Installed || a.Description == "" {
		t.Errorf("first: %+v", a)
	}
	if y := aur[6]; y.Name != "yay-bin" || !y.Installed {
		t.Errorf("yay-bin should be installed: %+v", y)
	}
	if older := ParseSearch([]byte("core/bash 5.3-1 [installed: 5.2-1]\n    shell\n")); !older[0].Installed {
		t.Error("[installed: older] is installed")
	}
}

func TestSearchPackages(t *testing.T) {
	run := cmdRun{
		"pacman -Ss lazy docker": {out: string(fixture(t, "pacman-Ss.txt"))},
		"yay -Ssa lazy docker":   {code: 1}, // nothing in the AUR
	}
	s := SearchPackages(context.Background(), run.Run, "  lazy docker ")
	if len(s.Repo) != 2 || s.AUR != nil || s.RepoErr != nil || s.AURErr != nil || s.AURMissing {
		t.Errorf("%+v", s)
	}
	none := SearchPackages(context.Background(), cmdRun{}.Run, "x")
	if !none.AURMissing || none.RepoErr == nil {
		t.Errorf("no tools: %+v", none)
	}
	if empty := SearchPackages(context.Background(), run.Run, "  "); empty.Repo != nil || empty.RepoErr != nil {
		t.Errorf("empty query ran: %+v", empty)
	}
}
