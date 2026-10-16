package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

func snapModel(t *testing.T, opts Options) *Model {
	t.Helper()
	opts.Source = fakeSource{}
	opts.Probe = probe{
		"snapper --jsonout -c root list --columns number,date,type,description,cleanup": fixtureText(t, "snapper-list.json"),
		"snapper -c root status 584..0": "+..... /etc/new.conf\nc..... /etc/pacman.conf\n-..... /usr/bin/gone\n",
	}.run
	m := New(opts)
	load(t, m)
	goTo(t, m, "Snapshots")
	return m
}

func TestSnapshotsListNewestFirst(t *testing.T) {
	m := snapModel(t, Options{})
	out := ansi.Strip(m.View().Content)
	if !strings.Contains(out, "584") || !strings.Contains(out, "timeline") {
		t.Errorf("list:\n%s", out)
	}
	if i, j := strings.Index(out, "584"), strings.Index(out, "583"); i < 0 || j < 0 || i > j {
		t.Errorf("want 584 above 583:\n%s", out)
	}
}

func TestSnapshotsDiff(t *testing.T) {
	m := snapModel(t, Options{})
	press(m, "enter")
	out := ansi.Strip(m.View().Content)
	for _, want := range []string{"since #584", "1 created", "1 deleted", "1 modified", "/etc/pacman.conf"} {
		if !strings.Contains(out, want) {
			t.Errorf("diff missing %q:\n%s", want, out)
		}
	}
	press(m, "esc")
	if !m.inArea {
		t.Error("esc should close the diff and stay in the area")
	}
	if out := ansi.Strip(m.View().Content); strings.Contains(out, "Changes since") {
		t.Errorf("diff still showing:\n%s", out)
	}
	press(m, "esc")
	if m.inArea {
		t.Error("a second esc returns to the sidebar")
	}
}

func TestSnapshotsCreate(t *testing.T) {
	var ran []string
	m := snapModel(t, Options{Exec: recordExec(&ran)})
	press(m, "n")
	typeText(m, "before nvidia")
	press(m, "enter")
	want := "sudo snapper -c root create -d before nvidia"
	if len(ran) != 1 || ran[0] != want {
		t.Errorf("ran %v, want %q", ran, want)
	}

	press(m, "n")
	typeText(m, "never mind")
	press(m, "esc")
	if len(ran) != 1 {
		t.Errorf("esc should cancel, ran %v", ran)
	}
}

func TestSnapshotsWithoutSnapper(t *testing.T) {
	m := New(Options{Source: fakeSource{}})
	load(t, m)
	goTo(t, m, "Snapshots")
	if out := m.View().Content; !strings.Contains(out, "snapper") {
		t.Errorf("want a not-installed message:\n%s", out)
	}
}
