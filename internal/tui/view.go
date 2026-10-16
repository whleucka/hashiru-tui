package tui

import (
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

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
	styleTab      = lipgloss.NewStyle().Padding(0, 1).Foreground(colorMuted)
	styleTabOn    = lipgloss.NewStyle().Padding(0, 1).Bold(true).Reverse(true)
	styleMuted    = lipgloss.NewStyle().Foreground(colorMuted)
	styleKey      = lipgloss.NewStyle().Foreground(colorMuted).Width(11)
	styleOK       = lipgloss.NewStyle().Foreground(colorOK)
	styleWarn     = lipgloss.NewStyle().Foreground(colorWarn)
	styleFail     = lipgloss.NewStyle().Foreground(colorFail)
	styleSelected = lipgloss.NewStyle().Bold(true)
)

func (m *Model) View() tea.View {
	var b strings.Builder
	b.WriteString(m.header())
	b.WriteString("\n\n")
	switch m.tab {
	case tabDashboard:
		b.WriteString(m.dashboard())
	case tabDoctor:
		b.WriteString(m.doctor())
	}

	// Pin the footer to the bottom.
	body := b.String()
	if pad := m.height - lipgloss.Height(body) - 2; pad > 0 {
		body += strings.Repeat("\n", pad)
	}
	body += "\n" + m.footer()

	v := tea.NewView(body)
	v.AltScreen = true
	return v
}

func (m *Model) header() string {
	parts := []string{styleTitle.Render("走 hashiru")}
	for t := tab(0); t < tabCount; t++ {
		label := fmt.Sprintf("%d %s", t+1, t)
		if t == m.tab {
			parts = append(parts, styleTabOn.Render(label))
		} else {
			parts = append(parts, styleTab.Render(label))
		}
	}
	return strings.Join(parts, " ")
}

func (m *Model) footer() string {
	help := "tab switch · r refresh · u update · e config · L follow log · q quit"
	if m.tab == tabDoctor {
		help = "j/k move · enter expand · " + help
	}
	if m.note == "" {
		return styleMuted.Render(help)
	}
	note := styleOK.Render(m.note)
	if m.noteErr {
		note = styleFail.Render(m.note)
	}
	return note + "\n" + styleMuted.Render(help)
}

// --- Dashboard -----------------------------------------------------------------

func (m *Model) dashboard() string {
	var b strings.Builder
	row := func(k, v string) { b.WriteString(styleKey.Render(k) + v + "\n") }

	switch {
	case m.status == nil && m.statusBusy:
		row("status", styleMuted.Render("loading… (fetches upstream)"))
	case m.status == nil && m.statusErr != nil:
		row("status", styleFail.Render(m.statusErr.Error()))
	case m.status != nil:
		s := m.status
		row("version", orUnknown(s.Version))
		row("commit", short(s.Commit))
		if s.Head != "" && s.Head != s.Commit {
			row("checkout", short(s.Head)+styleWarn.Render("  (not what was last installed)"))
		}
		row("installed", when(s.Installed))
		if !s.Updated.IsZero() {
			row("updated", when(s.Updated))
		}
		row("repo", s.Repo)
		row("tree", treeLine(s))
		row("upstream", upstreamLine(s))
	}

	b.WriteString("\n")
	switch {
	case m.report == nil && m.reportBusy:
		row("doctor", styleMuted.Render("running…"))
	case m.report == nil && m.reportErr != nil:
		row("doctor", styleFail.Render(m.reportErr.Error()))
	case m.report != nil:
		row("doctor", summaryLine(m.report))
	}
	return b.String()
}

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
		return styleWarn.Render(fmt.Sprintf("%d commit(s) behind, press u to update", s.Behind))
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

// --- Doctor --------------------------------------------------------------------

func (m *Model) doctor() string {
	switch {
	case m.report == nil && m.reportBusy:
		return styleMuted.Render("running doctor…")
	case m.report == nil && m.reportErr != nil:
		return styleFail.Render(m.reportErr.Error())
	case m.report == nil:
		return ""
	}

	var b strings.Builder
	b.WriteString(summaryLine(m.report) + "\n\n")
	for i, sec := range m.sections() {
		cursor := "  "
		name := sec.Name
		if i == m.cursor {
			cursor = styleTitle.Render("▸ ")
			name = styleSelected.Render(name)
		}
		fmt.Fprintf(&b, "%s%s %s %s\n", cursor, mark(sec.Worst()), name,
			styleMuted.Render(fmt.Sprintf("(%d)", len(sec.Checks))))

		// Problems are always shown; the rest only when expanded.
		open := m.expanded[sec.Name]
		for _, c := range sec.Checks {
			if open || c.Status == hashiru.Warn || c.Status == hashiru.Fail {
				fmt.Fprintf(&b, "      %s %s\n", mark(c.Status), c.Message)
			}
		}
	}
	return b.String()
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

// --- Helpers -------------------------------------------------------------------

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
