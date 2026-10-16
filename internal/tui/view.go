package tui

import (
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/whleucka/hashiru-tui/internal/hashiru"
)

// ANSI colours, so the TUI follows the terminal's theme (kitty's, on Hashiru).
var (
	colorAccent = lipgloss.Color("12")
	colorMuted  = lipgloss.Color("8")
	colorOK     = lipgloss.Color("2")
	colorWarn   = lipgloss.Color("3")
	colorFail   = lipgloss.Color("9")

	styleTitle    = lipgloss.NewStyle().Bold(true).Foreground(colorAccent)
	styleMuted    = lipgloss.NewStyle().Foreground(colorMuted)
	styleKey      = lipgloss.NewStyle().Foreground(colorMuted).Width(11)
	styleOK       = lipgloss.NewStyle().Foreground(colorOK)
	styleWarn     = lipgloss.NewStyle().Foreground(colorWarn)
	styleFail     = lipgloss.NewStyle().Foreground(colorFail)
	styleSelected = lipgloss.NewStyle().Bold(true)
	styleItemOn   = lipgloss.NewStyle().Bold(true).Reverse(true)
	styleSidebar  = lipgloss.NewStyle().
			Border(lipgloss.NormalBorder(), false, true, false, false).
			BorderForeground(colorMuted).
			PaddingRight(1)
)

const (
	sidebarWidth = 18 // inside the border
	footerLines  = 2  // note, help
)

// areaSize is the room left for the selected area.
func (m *Model) areaSize() (int, int) {
	w := m.width - sidebarWidth - 3 // border, padding, gap
	h := m.height - footerLines
	return max(w, 10), max(h, 3)
}

func (m *Model) View() tea.View {
	w, h := m.areaSize()

	content := m.area().View(w, h)
	if m.pal.open {
		content = m.pal.view(w, h)
	}
	body := lipgloss.JoinHorizontal(lipgloss.Top,
		styleSidebar.Height(h).Render(m.sidebar()),
		" ",
		clip(content, w, h),
	)
	v := tea.NewView(body + "\n" + m.footer())
	v.AltScreen = true
	return v
}

func (m *Model) sidebar() string {
	lines := []string{styleTitle.Render("走 hashiru"), ""}
	for i, a := range m.areas {
		label := fmt.Sprintf("%d %s", i+1, a.Name())
		label = fmt.Sprintf("%-*s", sidebarWidth, label)
		switch {
		case i == m.sel && !m.inArea:
			label = styleItemOn.Render(label)
		case i == m.sel:
			label = styleSelected.Render(label)
		default:
			label = styleMuted.Render(label)
		}
		lines = append(lines, label)
	}
	return strings.Join(lines, "\n")
}

// The shell's own keys, from the sidebar and from inside an area.
var (
	sidebarKeys = []Key{
		{Keys: "j/k", Desc: "area", Group: "Global"},
		{Keys: "1-9", Desc: "jump to area", Group: "Global"},
		{Keys: "enter", Desc: "open", Group: "Global"},
		{Keys: ":", Desc: "palette", Group: "Global"},
		{Keys: "r", Desc: "refresh", Group: "Global"},
		{Keys: "q", Desc: "quit", Group: "Global"},
	}
	inAreaKeys = []Key{
		{Keys: "esc", Desc: "back", Group: "Global"},
		{Keys: "ctrl+p", Desc: "palette", Group: "Global"},
		{Keys: "r", Desc: "refresh", Group: "Global"},
		{Keys: "q", Desc: "quit", Group: "Global"},
	}
)

// keys is everything that works right now, the area's first.
func (m *Model) keys() []Key {
	switch {
	case m.pal.open:
		return m.pal.keys()
	case m.inArea:
		return append(m.area().Keys(), inAreaKeys...)
	}
	return sidebarKeys
}

func (m *Model) footer() string {
	help := styleMuted.Render(keyLine(m.keys(), m.width))

	note := ""
	switch {
	case m.note != "" && m.noteErr:
		note = styleFail.Render(m.note)
	case m.note != "":
		note = styleOK.Render(m.note)
	}
	return ansi.Truncate(note, m.width, "…") + "\n" + help
}

// keyLine joins keys as "j/k move · enter edit", dropping whole keys from the
// end rather than cutting one in half.
func keyLine(keys []Key, width int) string {
	const sep = " · "
	line := ""
	for i, k := range keys {
		part := k.Keys + " " + k.Desc
		if i > 0 {
			part = sep + part
		}
		if ansi.StringWidth(line+part) > width {
			break
		}
		line += part
	}
	return line
}

// clip cuts s to h lines of at most w cells, so an area can't push the
// footer off screen.
func clip(s string, w, h int) string {
	lines := strings.Split(s, "\n")
	if len(lines) > h {
		lines = lines[:h]
	}
	for i, l := range lines {
		lines[i] = ansi.Truncate(l, w, "…")
	}
	return strings.Join(lines, "\n")
}

// --- Shared pieces -------------------------------------------------------------

func treeLine(s *hashiru.Status) string {
	line := styleOK.Render("clean")
	if s.Dirty {
		line = styleWarn.Render("dirty: 'hashiru update' refuses until it's committed or stashed")
	}
	if s.Untracked > 0 {
		line += styleMuted.Render(fmt.Sprintf("  (%d untracked, ignored by update)", s.Untracked))
	}
	return line
}

func upstreamLine(s *hashiru.Status) string {
	switch {
	case !s.BehindOK:
		return styleMuted.Render("unknown (no upstream, or offline)")
	case s.Behind == 0:
		return styleOK.Render("up to date")
	default:
		return styleWarn.Render(fmt.Sprintf("%d commit(s) behind", s.Behind))
	}
}

func summaryLine(r *hashiru.Report) string {
	parts := []string{styleOK.Render(fmt.Sprintf("%d ok", r.Summary.OK))}
	warn := fmt.Sprintf("%d warnings", r.Summary.Warn)
	if r.Summary.Warn > 0 {
		parts = append(parts, styleWarn.Render(warn))
	} else {
		parts = append(parts, styleMuted.Render(warn))
	}
	fail := fmt.Sprintf("%d failures", r.Summary.Fail)
	if r.Summary.Fail > 0 {
		parts = append(parts, styleFail.Render(fail))
	} else {
		parts = append(parts, styleMuted.Render(fail))
	}
	return strings.Join(parts, styleMuted.Render(" · "))
}

func mark(l hashiru.Level) string {
	switch l {
	case hashiru.OK:
		return styleOK.Render("✔")
	case hashiru.Warn:
		return styleWarn.Render("●")
	case hashiru.Fail:
		return styleFail.Render("✘")
	default:
		return styleMuted.Render("·")
	}
}

func short(commit string) string {
	if len(commit) > 12 {
		return commit[:12]
	}
	return orUnknown(commit)
}

func orUnknown(s string) string {
	if s == "" {
		return styleMuted.Render("unknown")
	}
	return s
}

func when(t time.Time) string {
	if t.IsZero() {
		return styleMuted.Render("unknown")
	}
	return t.Format("2006-01-02 15:04") + styleMuted.Render("  ("+ago(time.Since(t))+")")
}

func ago(d time.Duration) string {
	switch {
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 48*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd ago", int(d.Hours()/24))
	}
}
