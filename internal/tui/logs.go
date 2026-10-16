package tui

import tea "charm.land/bubbletea/v2"

// logsArea shows logs. For now that is Hashiru's install log; the journal and
// failed units join it as further sources.
type logsArea struct {
	e  *env
	lv logView
}

func newLogs(e *env) *logsArea { return &logsArea{e: e, lv: newLogView()} }

func (l *logsArea) Name() string { return "Logs" }

func (l *logsArea) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case areaSizeMsg:
		l.lv.setSize(msg.width, msg.height)
	case refreshMsg:
		return loadLog(l.e.src)
	case logMsg:
		l.lv.setRuns(msg.runs, msg.err)
	case logTickMsg:
		if !l.lv.follow {
			return nil // stopped since the tick was scheduled
		}
		return tea.Batch(loadLog(l.e.src), followTick())
	}
	return nil
}

func (l *logsArea) Actions() []Action {
	return []Action{
		{Title: "Follow the Hashiru log in a pane (hashiru log -f)", Key: "L", Run: l.followInPane},
	}
}

func (l *logsArea) followInPane() tea.Cmd {
	return l.e.run(l.e.src.Argv("log", "-f"), "hashiru log", false)
}

func (l *logsArea) HandleKey(msg tea.KeyPressMsg) (tea.Cmd, bool) {
	if msg.String() == "L" {
		return l.followInPane(), true
	}
	return l.lv.handleKey(msg)
}

func (l *logsArea) Help() string { return l.lv.help() + " · L pane" }

func (l *logsArea) View(width, height int) string { return l.lv.view() }
