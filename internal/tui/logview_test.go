package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/whleucka/hashiru-tui/internal/hashiru"
)

const testLog = `[2026-10-14 11:47:50] [INFO] Pulling latest Hashiru into /opt/hashiru
[2026-10-14 11:47:51] [INFO] ===== HASHIRU RUN START =====
[2026-10-14 11:47:52] [INFO] [1/1] Running: 45-config.sh
[2026-10-14 11:47:53] [WARN] kitty override missing
[2026-10-14 11:48:10] [OK] [1/1] Completed: 45-config.sh (18s)
[2026-10-14 11:48:11] [OK] Hashiru installation complete in 20s
[2026-10-15 09:00:00] [INFO] ===== HASHIRU RUN START =====
[2026-10-15 09:00:01] [INFO] [1/1] Running: 30-desktop.sh
:: Synchronizing package databases...
[2026-10-15 09:00:09] [ERROR] Script failed at line 208 with exit code 1
`

func parseRuns(t *testing.T, s string) []hashiru.Run {
	t.Helper()
	runs, err := hashiru.ParseLog(strings.NewReader(s))
	if err != nil {
		t.Fatal(err)
	}
	return runs
}

func logModel(t *testing.T) *Model {
	t.Helper()
	m := New(Options{Source: fakeSource{runs: parseRuns(t, testLog)}})
	load(t, m)
	m.Update(key("3"))
	return m
}

func TestLogListsRunsNewestFirst(t *testing.T) {
	m := logModel(t)
	out := m.View().Content

	failed := strings.Index(out, "stage 30-desktop.sh")
	update := strings.Index(out, "update, 1 stages")
	if failed < 0 || update < 0 || failed > update {
		t.Fatalf("want the failed run listed above the update:\n%s", out)
	}
	// The newest run is selected, so its lines are showing.
	if !strings.Contains(out, "Script failed at line 208") {
		t.Errorf("selected run's lines missing:\n%s", out)
	}
}

func TestLogSelectAndFilter(t *testing.T) {
	m := logModel(t)
	m.Update(key("j")) // the update
	out := m.View().Content
	if !strings.Contains(out, "Completed: 45-config.sh") {
		t.Fatalf("j should show the older run:\n%s", out)
	}

	m.Update(key("w"))
	out = m.View().Content
	if strings.Contains(out, "Completed: 45-config.sh") {
		t.Error("w should hide INFO and OK lines")
	}
	if !strings.Contains(out, "kitty override missing") {
		t.Error("w should keep the warning")
	}
}

func TestLogEnterScrollsBodyNotList(t *testing.T) {
	m := logModel(t)
	m.Update(key("enter"))
	m.Update(key("j"))
	if m.logv.sel != 0 {
		t.Errorf("j in the body moved the run selection to %d", m.logv.sel)
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	m.Update(key("j"))
	if m.logv.sel != 1 {
		t.Errorf("after esc, j should move to run 1, got %d", m.logv.sel)
	}
}

func TestLogKeepsSelectionAsRunsArrive(t *testing.T) {
	m := logModel(t)
	m.Update(key("j")) // reading the older run
	before := m.logv.run().Start

	more := testLog + "[2026-10-15 10:00:00] [INFO] ===== HASHIRU RUN START =====\n"
	m.Update(logMsg{runs: parseRuns(t, more)})
	if got := m.logv.run().Start; !got.Equal(before) {
		t.Errorf("selection jumped to the run at %v", got)
	}
}

func TestLogFollowPinsNewest(t *testing.T) {
	m := logModel(t)
	m.Update(key("j"))
	_, cmd := m.Update(key("f"))
	if cmd == nil {
		t.Fatal("f should schedule a tick")
	}
	if m.logv.sel != 0 {
		t.Errorf("following should select the newest run, sel = %d", m.logv.sel)
	}

	more := testLog + "[2026-10-15 10:00:00] [INFO] ===== HASHIRU RUN START =====\n"
	m.Update(logMsg{runs: parseRuns(t, more)})
	if m.logv.sel != 0 || !m.logv.run().Start.Equal(parseRuns(t, more)[2].Start) {
		t.Error("following should move to a run that just started")
	}

	m.Update(key("f"))
	if _, cmd := m.Update(logTickMsg{}); cmd != nil {
		t.Error("a tick after stopping should not reschedule")
	}
}
