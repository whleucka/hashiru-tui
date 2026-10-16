package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/whleucka/hashiru-tui/internal/hashiru"
)

const notesLs = "docs/releases/v1.12.0.md\ndocs/releases/v1.13.0.md\ndocs/releases/v1.14.0.md\n"

func notesModel(t *testing.T, st hashiru.Status, p probe) *Model {
	t.Helper()
	m := New(Options{Source: fakeSource{status: st}, Probe: p.run})
	load(t, m)
	goTo(t, m, "Updates")
	return m
}

func TestNotesWhenBehindShowWhatAnUpdateBrings(t *testing.T) {
	st := hashiru.Status{Version: "v1.13.0-4-gabc", Repo: "/opt/hashiru", Behind: 9, BehindOK: true}
	m := notesModel(t, st, probe{
		"git -C /opt/hashiru ls-tree --name-only @{u} docs/releases/": notesLs,
		"git -C /opt/hashiru show @{u}:docs/releases/v1.14.0.md":      "# v1.14.0\n\nPackages release.\nNew area.\n\n## Packages\n\n- Groups.\n",
		"git -C /opt/hashiru ls-tree --name-only HEAD docs/releases/": "unused",
		"git -C /opt/hashiru show HEAD:docs/releases/v1.13.0.md":      "# v1.13.0\n\nShould not show.\n",
	})
	out := ansi.Strip(m.View().Content)
	if !strings.Contains(out, "what an update brings") || !strings.Contains(out, "v1.14.0  Packages release. New area.") {
		t.Errorf("summary:\n%s", out)
	}
	if strings.Contains(out, "Should not show") {
		t.Errorf("showed this version's notes while behind:\n%s", out)
	}

	press(m, "c")
	out = ansi.Strip(m.View().Content)
	if !strings.Contains(out, "Packages") || !strings.Contains(out, "- Groups.") || strings.Contains(out, "Health") {
		t.Errorf("full notes:\n%s", out)
	}
	press(m, "esc")
	if m.area().(*updates).notes.reading || !m.inArea {
		t.Error("esc should close the notes and stay in Updates")
	}
}

func TestNotesUpToDateShowThisVersion(t *testing.T) {
	st := hashiru.Status{Version: "v1.13.0", Repo: "/opt/hashiru", BehindOK: true}
	m := notesModel(t, st, probe{
		"git -C /opt/hashiru ls-tree --name-only HEAD docs/releases/": notesLs,
		"git -C /opt/hashiru show HEAD:docs/releases/v1.13.0.md":      "# v1.13.0\n\nFingerprint release.\n",
	})
	out := ansi.Strip(m.View().Content)
	if !strings.Contains(out, "this version") || !strings.Contains(out, "v1.13.0  Fingerprint release.") {
		t.Errorf("summary:\n%s", out)
	}
}

func TestNotesWithoutGitSayWhy(t *testing.T) {
	st := hashiru.Status{Version: "v1.13.0", Repo: "/opt/hashiru", BehindOK: true}
	m := notesModel(t, st, probe{}) // git "not installed"
	out := ansi.Strip(m.View().Content)
	if !strings.Contains(out, "git") || strings.Contains(out, "reading…") {
		t.Errorf("no git:\n%s", out)
	}
	press(m, "c") // nothing to read; stays on the summary
	if m.area().(*updates).notes.reading {
		t.Error("c opened empty notes")
	}
}

func TestNotesWithoutARepoDontHang(t *testing.T) {
	m := notesModel(t, hashiru.Status{}, probe{})
	if out := ansi.Strip(m.View().Content); strings.Contains(out, "reading…") {
		t.Errorf("no repo but still reading:\n%s", out)
	}
}

func TestRunNotesBeforeTheyLoadOpensThemOnArrival(t *testing.T) {
	st := hashiru.Status{Version: "v1.13.0", Repo: "/opt/hashiru", BehindOK: true}
	p := probe{
		"git -C /opt/hashiru ls-tree --name-only HEAD docs/releases/": notesLs,
		"git -C /opt/hashiru show HEAD:docs/releases/v1.13.0.md":      "# v1.13.0\n\nFingerprint release.\n",
	}
	m := New(Options{Source: fakeSource{status: st}, Probe: p.run})
	if err := m.Start("", "updates.notes"); err != nil {
		t.Fatal(err)
	}
	load(t, m) // sizes the window (runs the action), then loads
	if !m.area().(*updates).notes.reading {
		t.Error("notes asked for at start didn't open once loaded")
	}
}
