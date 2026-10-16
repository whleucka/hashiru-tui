package tui

import (
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
)

func TestOverviewStatsReloadOnlyWhileShown(t *testing.T) {
	ticks, reads := 0, 0
	m := New(Options{
		Source: fakeSource{},
		Tick: func(d time.Duration, _ func(time.Time) tea.Msg) tea.Cmd {
			if d != sysInterval {
				t.Errorf("tick every %v", d)
			}
			ticks++
			return nil // tests fire the tick themselves
		},
		Statfs: func(string) (uint64, uint64, error) { reads++; return 1000 << 30, 250 << 30, nil },
	})
	load(t, m)
	if ticks != 1 {
		t.Fatalf("Init armed %d tickers, want 1", ticks)
	}

	tick := func() {
		_, cmd := m.Update(sysTickMsg{})
		drain(m, cmd)
	}

	before := reads
	tick() // Overview is selected at start
	if reads != before+1 || ticks != 2 {
		t.Errorf("on Overview: %d reads, %d ticks", reads-before, ticks)
	}

	goTo(t, m, "Updates")
	before = reads
	tick()
	if reads != before || ticks != 3 {
		t.Errorf("on Updates: %d reads (want 0), %d ticks (want re-armed)", reads-before, ticks)
	}

	press(m, "r") // refresh reloads, but doesn't start a second ticker
	if ticks != 3 {
		t.Errorf("refresh armed a ticker: %d", ticks)
	}
}
