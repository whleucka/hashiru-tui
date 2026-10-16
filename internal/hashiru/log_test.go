package hashiru

import (
	"strings"
	"testing"
)

const sampleLog = `[2026-10-01 09:00:00] [INFO] Clone finished
[2026-10-14 15:15:18] [INFO] ===== HASHIRU RUN START =====
[2026-10-14 15:15:21] [INFO] [1/2] Running: 10-base.sh
:: Synchronizing package databases...
[2026-10-14 15:15:26] [OK] [1/2] Completed: 10-base.sh (5s)
[2026-10-14 15:15:26] [INFO] [2/2] Running: 45-config.sh
[2026-10-14 15:15:27] [WARN] kitty override missing
[2026-10-14 15:15:28] [OK] [2/2] Completed: 45-config.sh (1m 2s)
[2026-10-14 15:15:28] [OK] Hashiru installation complete in 1m 10s — 1 warning(s)
[2026-10-14 15:15:29] [OK] Update complete (v1.13.0-25-g38e7ede)
[2026-10-15 18:06:14] [ERROR] /opt/hashiru has uncommitted changes — commit or stash them first.
[2026-10-15 18:08:35] [ERROR] /opt/hashiru has uncommitted changes — commit or stash them first.
[2026-10-15 18:59:58] [INFO] Pulling latest Hashiru into /opt/hashiru
[2026-10-15 19:00:00] [INFO] ===== HASHIRU RUN START =====
[2026-10-15 19:00:01] [INFO] [1/1] Running: 30-desktop.sh
[2026-10-15 19:00:09] [ERROR] Script failed at line 208 with exit code 1
`

func TestParseLog(t *testing.T) {
	runs, err := ParseLog(strings.NewReader(sampleLog))
	if err != nil {
		t.Fatal(err)
	}
	// Each refused update is its own loose entry: they were separate commands.
	want := []Outcome{Loose, Succeeded, Loose, Loose, Failed}
	if len(runs) != len(want) {
		t.Fatalf("got %d runs, want %d", len(runs), len(want))
	}
	for i, o := range want {
		if runs[i].Outcome != o {
			t.Errorf("run %d: %v, want %v", i, runs[i].Outcome, o)
		}
	}

	ok := runs[1]
	if len(ok.Lines) != 8 {
		t.Errorf("succeeded run has %d lines, want 8 (incl. raw output and Update complete)", len(ok.Lines))
	}
	if ok.Warns != 1 || ok.Errors != 0 {
		t.Errorf("warns %d errors %d", ok.Warns, ok.Errors)
	}
	if len(ok.Stages) != 2 || !ok.Stages[1].Done || ok.Stages[1].Duration != "1m 2s" {
		t.Errorf("stages = %+v", ok.Stages)
	}

	if runs[2].Errors != 1 {
		t.Errorf("loose run errors = %d, want 1", runs[2].Errors)
	}

	failed := runs[4]
	if len(failed.Stages) != 1 || failed.Stages[0].Done {
		t.Errorf("failed run stages = %+v", failed.Stages)
	}
	if failed.Lines[0].Text != "Pulling latest Hashiru into /opt/hashiru" {
		t.Errorf("update's pull should open the run, got %q", failed.Lines[0].Text)
	}
}

func TestParseLineRaw(t *testing.T) {
	ln := ParseLine(" core downloading...")
	if ln.Level != "" || !ln.Time.IsZero() || ln.Text != " core downloading..." {
		t.Errorf("raw line = %+v", ln)
	}
	ln = ParseLine("[2026-10-14 15:15:27] [WARN] x [y] z")
	if ln.Level != "WARN" || ln.Text != "x [y] z" || ln.Time.Hour() != 15 {
		t.Errorf("line = %+v", ln)
	}
}

func TestParseLogRealFile(t *testing.T) {
	// Smoke test against this machine's log, when there is one.
	runs, err := ReadLog(LogPath())
	if err != nil {
		t.Skip(err)
	}
	if len(runs) == 0 {
		t.Error("a log file with no runs")
	}
}
