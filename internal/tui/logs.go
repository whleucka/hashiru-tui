package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/whleucka/hashiru-tui/internal/system"
)

// logSource is what the Logs area is showing.
type logSource int

const (
	srcHashiru logSource = iota // Hashiru's install log
	srcJournal                  // this boot's errors
	srcFailed                   // failed systemd units
	srcCount
)

// logsArea shows logs from a few sources, s to switch. Following anything
// live is a launch (J, K, L), since journalctl -f already does it well.
type logsArea struct {
	e   *env
	src logSource

	lv logView // Hashiru

	journal    []system.JournalLine // newest first
	journalErr error
	journalTop int // first line shown

	failed    []system.Unit
	failedErr error
	failedSel int
}

type (
	journalMsg struct {
		lines []system.JournalLine
		err   error
	}
	failedMsg struct {
		units []system.Unit
		err   error
	}
)

func newLogs(e *env) *logsArea { return &logsArea{e: e, lv: newLogView()} }

func (l *logsArea) Name() string { return "Logs" }

func (l *logsArea) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case areaSizeMsg:
		l.lv.setSize(msg.width, msg.height-2) // less the source bar
	case refreshMsg:
		return tea.Batch(loadLog(l.e.src), l.loadJournal(), l.loadFailed())
	case logMsg:
		l.lv.setRuns(msg.runs, msg.err)
	case logTickMsg:
		if !l.lv.follow {
			return nil // stopped since the tick was scheduled
		}
		return tea.Batch(loadLog(l.e.src), followTick())
	case journalMsg:
		l.journal, l.journalErr = msg.lines, msg.err
	case failedMsg:
		l.failed, l.failedErr = msg.units, msg.err
		l.failedSel = min(l.failedSel, max(len(l.failed)-1, 0))
	}
	return nil
}

func (l *logsArea) loadJournal() tea.Cmd {
	probe := l.e.probe
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		out, err := probe(ctx, "journalctl", system.JournalErrorsArgs...)
		if err != nil {
			return journalMsg{err: err}
		}
		lines := system.ParseJournal(out)
		for i, j := 0, len(lines)-1; i < j; i, j = i+1, j-1 {
			lines[i], lines[j] = lines[j], lines[i] // newest first
		}
		return journalMsg{lines: lines}
	}
}

func (l *logsArea) loadFailed() tea.Cmd {
	probe := l.e.probe
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		sys, err := probe(ctx, "systemctl", system.FailedArgs...)
		if err != nil {
			return failedMsg{err: err}
		}
		units := system.ParseFailed(sys, false)
		if user, err := probe(ctx, "systemctl", append([]string{"--user"}, system.FailedArgs...)...); err == nil {
			units = append(units, system.ParseFailed(user, true)...)
		}
		return failedMsg{units: units}
	}
}

func (l *logsArea) Actions() []Action {
	return []Action{
		{Title: "Follow the journal (journalctl -f)", Key: "J", Run: l.watch("journal", "journalctl", "-f")},
		{Title: "Follow kernel messages (journalctl -k -f)", Key: "K", Run: l.watch("kernel", "journalctl", "-k", "-f")},
		{Title: "Follow the Hashiru log (hashiru log -f)", Key: "L", Run: func() tea.Cmd {
			return l.e.launch(Launch{Argv: l.e.src.Argv("log", "-f"), Label: "hashiru log", Kind: Watch})
		}},
	}
}

func (l *logsArea) watch(label string, argv ...string) func() tea.Cmd {
	return func() tea.Cmd {
		return l.e.launch(Launch{Argv: argv, Label: label, Kind: Watch})
	}
}

func (l *logsArea) HandleKey(msg tea.KeyPressMsg) (tea.Cmd, bool) {
	k := msg.String()
	if k == "s" {
		l.src = (l.src + 1) % srcCount
		return nil, true
	}
	for _, a := range l.Actions() {
		if k == a.Key {
			return a.Run(), true
		}
	}

	switch l.src {
	case srcHashiru:
		return l.lv.handleKey(msg)
	case srcJournal:
		switch k {
		case "j", "down":
			if l.journalTop < len(l.journal)-1 {
				l.journalTop++
			}
		case "k", "up":
			if l.journalTop > 0 {
				l.journalTop--
			}
		default:
			return nil, false
		}
		return nil, true
	case srcFailed:
		switch k {
		case "j", "down":
			if l.failedSel < len(l.failed)-1 {
				l.failedSel++
			}
		case "k", "up":
			if l.failedSel > 0 {
				l.failedSel--
			}
		case "enter", "l", "right":
			return l.openUnit(), true
		default:
			return nil, false
		}
		return nil, true
	}
	return nil, false
}

// openUnit shows the selected failed unit's journal for this boot, in a
// pager, so it's interactive.
func (l *logsArea) openUnit() tea.Cmd {
	if l.failedSel >= len(l.failed) {
		return nil
	}
	u := l.failed[l.failedSel]
	argv := []string{"journalctl", "-b", "-u", u.Name}
	if u.User {
		argv = []string{"journalctl", "--user", "-b", "-u", u.Name}
	}
	return l.e.launch(Launch{Argv: argv, Label: u.Name, Kind: Interactive})
}

func (l *logsArea) Help() string {
	common := "s source · J journal · K kernel · L hashiru log"
	switch l.src {
	case srcHashiru:
		return l.lv.help() + " · " + common
	case srcFailed:
		return "j/k move · enter its journal · " + common
	}
	return "j/k scroll · " + common
}

func (l *logsArea) View(width, height int) string {
	var b strings.Builder
	b.WriteString(l.sourceBar() + "\n\n")
	switch l.src {
	case srcHashiru:
		b.WriteString(l.lv.view())
	case srcJournal:
		b.WriteString(l.journalView(height - 2))
	case srcFailed:
		b.WriteString(l.failedView())
	}
	return b.String()
}

func (l *logsArea) sourceBar() string {
	names := []string{
		"Hashiru",
		fmt.Sprintf("Journal errors (%d)", len(l.journal)),
		fmt.Sprintf("Failed units (%d)", len(l.failed)),
	}
	for i, n := range names {
		switch {
		case logSource(i) == l.src:
			names[i] = styleItemOn.Render(" " + n + " ")
		case logSource(i) == srcFailed && len(l.failed) > 0:
			names[i] = styleWarn.Render(" " + n + " ")
		default:
			names[i] = styleMuted.Render(" " + n + " ")
		}
	}
	return strings.Join(names, " ")
}

func (l *logsArea) journalView(rows int) string {
	switch {
	case l.journalErr != nil:
		return styleFail.Render(l.journalErr.Error())
	case len(l.journal) == 0:
		return styleOK.Render("no errors this boot")
	}
	var b strings.Builder
	for i := l.journalTop; i < len(l.journal) && i < l.journalTop+rows; i++ {
		j := l.journal[i]
		fmt.Fprintf(&b, "%s  %s  %s\n", styleMuted.Render(j.Time.Format("01-02 15:04:05")),
			styleWarn.Render(fmt.Sprintf("%-12s", j.Ident)), j.Message)
	}
	return b.String()
}

func (l *logsArea) failedView() string {
	switch {
	case l.failedErr != nil:
		return styleFail.Render(l.failedErr.Error())
	case len(l.failed) == 0:
		return styleOK.Render("no failed units")
	}
	var b strings.Builder
	for i, u := range l.failed {
		name := u.Name
		if u.User {
			name += styleMuted.Render(" (user)")
		}
		line := fmt.Sprintf("%s %s  %s", styleFail.Render("✘"), name, styleMuted.Render(u.Description))
		if i == l.failedSel {
			line = styleTitle.Render("▸ ") + line
		} else {
			line = "  " + line
		}
		b.WriteString(line + "\n")
	}
	return b.String()
}
