package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

func logsModel(t *testing.T, opts Options) *Model {
	t.Helper()
	opts.Source = fakeSource{runs: parseRuns(t, testLog)}
	opts.Probe = probe{
		"journalctl -p err -b -o short-iso --no-pager -n 200": fixtureText(t, "journal-err.txt"),
		"systemctl --failed --plain --no-legend":              fixtureText(t, "failed-system.txt"),
		"systemctl --user --failed --plain --no-legend":       "",
	}.run
	m := New(opts)
	load(t, m)
	goTo(t, m, "Logs")
	return m
}

func TestLogsSources(t *testing.T) {
	m := logsModel(t, Options{})
	out := ansi.Strip(m.View().Content)
	if !strings.Contains(out, "Hashiru") || !strings.Contains(out, "Journal errors (50)") || !strings.Contains(out, "Failed units (1)") {
		t.Errorf("source bar:\n%s", out)
	}

	press(m, "s")
	out = ansi.Strip(m.View().Content)
	if !strings.Contains(out, "hyprlock") || !strings.Contains(out, "conversation failed") {
		t.Errorf("journal errors:\n%s", out)
	}

	press(m, "s")
	out = ansi.Strip(m.View().Content)
	if !strings.Contains(out, "reflector.service") {
		t.Errorf("failed units:\n%s", out)
	}

	press(m, "s")
	if logsOf(m).src != srcHashiru {
		t.Error("s should wrap back to Hashiru")
	}
}

func TestLogsFailedUnitOpensItsJournal(t *testing.T) {
	var ran []string
	m := logsModel(t, Options{Exec: recordExec(&ran)})
	press(m, "s", "s", "l")
	if len(ran) != 1 || ran[0] != "journalctl -b -u reflector.service" {
		t.Errorf("ran %v", ran)
	}
}

func TestLogsFailedUnitEnterGoesToServices(t *testing.T) {
	m := logsModel(t, Options{})
	press(m, "s", "s", "enter")
	if m.area().Name() != "Services" || !m.inArea {
		t.Fatalf("on %s", m.area().Name())
	}
	if sv := m.area().(*services); sv.tab != tabFailed {
		t.Errorf("tab %v, want Failed", sv.tab)
	}
}

func TestLogsFollowActions(t *testing.T) {
	var ran []string
	m := logsModel(t, Options{Exec: recordExec(&ran)})
	press(m, "J", "K")
	if strings.Join(ran, ",") != "journalctl -f,journalctl -k -f" {
		t.Errorf("ran %v", ran)
	}
}

func TestLogsTabsSwitchWithBrackets(t *testing.T) {
	m := New(Options{Source: fakeSource{}})
	load(t, m)
	goTo(t, m, "Logs")
	l := m.area().(*logsArea)
	press(m, "]")
	if l.src != srcJournal {
		t.Errorf("] → %d, want journal", l.src)
	}
	press(m, "[", "[") // back to Hashiru, then wraps to failed units
	if l.src != srcFailed {
		t.Errorf("[ [ → %d, want failed units", l.src)
	}
	press(m, "s") // the old key still goes forward
	if l.src != srcHashiru {
		t.Errorf("s → %d, want Hashiru", l.src)
	}
	// The footer says how to switch; the tab bar is just the tabs.
	if out := ansi.Strip(m.View().Content); strings.Contains(out, "[ ]") || !strings.Contains(out, "[/] switch tab") {
		t.Errorf("want the [/] hint in the footer only:\n%s", out)
	}
}
