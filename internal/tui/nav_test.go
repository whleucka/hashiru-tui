package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/whleucka/hashiru-tui/internal/system"
)

func TestMoveKey(t *testing.T) {
	for _, tc := range []struct {
		key      string
		i, n, pg int
		want     int
		ok       bool
	}{
		{"j", 0, 10, 20, 1, true},
		{"k", 0, 10, 20, 0, true}, // stops at the top
		{"ctrl+d", 0, 100, 20, 10, true},
		{"ctrl+d", 95, 100, 20, 99, true}, // stops at the bottom
		{"ctrl+u", 15, 100, 20, 5, true},
		{"ctrl+u", 3, 100, 20, 0, true},
		{"ctrl+d", 0, 5, 1, 1, true},  // a tiny screen still moves
		{"ctrl+d", 0, 0, 20, 0, true}, // an empty list stays at 0
		{"x", 4, 10, 20, 4, false},
	} {
		got, ok := moveKey(tc.key, tc.i, tc.n, tc.pg)
		if got != tc.want || ok != tc.ok {
			t.Errorf("%s from %d of %d (page %d): %d %v, want %d %v", tc.key, tc.i, tc.n, tc.pg, got, ok, tc.want, tc.ok)
		}
	}
}

func TestCellFitsTheColumn(t *testing.T) {
	if got := cell("htop", 8); got != "htop    " {
		t.Errorf("short: %q", got)
	}
	if got := cell("desktop-like-mobile-config-firefox", 12); got != "desktop-lik…" || ansi.StringWidth(got) != 12 {
		t.Errorf("long: %q", got)
	}
	if got := cell("浏览器浏览器浏览器", 7); ansi.StringWidth(got) != 7 {
		t.Errorf("wide: %q is %d cells", got, ansi.StringWidth(got))
	}
}

// The screenshot this came from: long AUR names and versions pushed the
// description column out of line.
func TestSearchColumnsStayInLine(t *testing.T) {
	m, _ := actionsModel(t)
	p := m.area().(*packages)
	p.group = groupSearch
	p.search = &system.Search{AUR: []system.Found{
		{Repo: "aur", Name: "desktop-like-mobile-config-firefox", Version: "4.0.0-1", Description: "MARK one"},
		{Repo: "aur", Name: "spidermonkey-git", Version: "130.0a1+20240803.1+gd92a07556499-1", Description: "MARK two"},
		{Repo: "aur", Name: "htop", Version: "1-1", Description: "MARK three"},
	}}
	col := -1
	for _, l := range strings.Split(ansi.Strip(m.View().Content), "\n") {
		if i := strings.Index(l, "MARK"); i >= 0 {
			w := ansi.StringWidth(l[:i])
			if col >= 0 && w != col {
				t.Errorf("description at column %d, want %d:\n%s", w, col, l)
			}
			col = w
		}
	}
	if col < 0 {
		t.Fatal("no rows rendered")
	}
}

func TestHalfPageInAList(t *testing.T) {
	m, _ := actionsModel(t)
	press(m, "[", "[") // Hashiru's 191
	m.Update(ctrlD())
	if p := m.area().(*packages); p.sel < 5 {
		t.Errorf("ctrl+d moved to %d", p.sel)
	}
}

func ctrlD() tea.KeyPressMsg { return tea.KeyPressMsg{Code: 'd', Mod: tea.ModCtrl} }

func TestCellMidKeepsTheEnd(t *testing.T) {
	da := cellMid("firefox-developer-edition-i18n-da", 30)
	de := cellMid("firefox-developer-edition-i18n-de", 30)
	if da == de || !strings.HasSuffix(da, "i18n-da") || ansi.StringWidth(da) != 30 {
		t.Errorf("%q / %q", da, de)
	}
	if got := cellMid("htop", 10); got != "htop      " {
		t.Errorf("short: %q", got)
	}
}
