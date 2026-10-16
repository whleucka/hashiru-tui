package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

func TestOverviewSystemSummary(t *testing.T) {
	m := New(Options{Source: fakeSource{}})
	load(t, m)
	out := ansi.Strip(m.View().Content)
	for _, want := range []string{"t14s", "7.2.7-arch1-1", "up 14d", "of 30.5 GiB", "250.0 GiB free of 1000.0 GiB", "32% discharging"} {
		if !strings.Contains(out, want) {
			t.Errorf("Overview missing %q:\n%s", want, out)
		}
	}
}
