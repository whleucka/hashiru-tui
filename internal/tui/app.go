// Package tui is the bubbletea front end. It holds no Hashiru logic of its own:
// state comes from package hashiru, and anything that changes the machine runs
// the hashiru CLI in a herdr pane, or in this terminal when there is no herdr.
package tui

import (
	"context"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/whleucka/hashiru-tui/internal/hashiru"
)

// Source is where the TUI gets its state. hashiru.CLI satisfies it; tests use
// a fake.
type Source interface {
	Status(context.Context) (hashiru.Status, error)
	Doctor(context.Context) (hashiru.Report, error)
	Argv(args ...string) []string
}

// Options wires the model to the outside world.
type Options struct {
	Source Source
	// RunInPane runs argv in a new herdr pane named label, to the right or
	// below, focused or not. Nil outside herdr; commands then take over this
	// terminal through tea.ExecProcess instead.
	RunInPane func(ctx context.Context, argv []string, label string, right, focus bool) error
}

type tab int

const (
	tabDashboard tab = iota
	tabDoctor
	tabCount
)

func (t tab) String() string {
	return [...]string{"Dashboard", "Doctor"}[t]
}

// Model is the whole app.
type Model struct {
	opts          Options
	width, height int
	tab           tab

	status     *hashiru.Status
	statusErr  error
	statusBusy bool

	report     *hashiru.Report
	reportErr  error
	reportBusy bool

	// Doctor tab: which section the cursor is on, and which are expanded.
	cursor   int
	expanded map[string]bool

	note    string // one-line feedback in the footer
	noteErr bool
}

// New builds the model. Call Init (bubbletea does) to start loading.
func New(opts Options) *Model {
	return &Model{opts: opts, expanded: map[string]bool{}}
}

// Messages from the commands below back to Update.
type (
	statusMsg struct {
		s   hashiru.Status
		err error
	}
	reportMsg struct {
		r   hashiru.Report
		err error
	}
	noteMsg struct {
		text string
		err  error
	}
	// execDoneMsg arrives when a command that borrowed the terminal exits.
	execDoneMsg struct {
		label string
		err   error
	}
)

// status fetches upstream, which can take ~10s offline, so allow a bit more.
const (
	statusTimeout = 20 * time.Second
	doctorTimeout = 60 * time.Second
	paneTimeout   = 5 * time.Second
)

func (m *Model) Init() tea.Cmd {
	return m.refresh()
}

// refresh reloads status and doctor in parallel.
func (m *Model) refresh() tea.Cmd {
	m.statusBusy, m.reportBusy = true, true
	src := m.opts.Source
	return tea.Batch(
		func() tea.Msg {
			ctx, cancel := context.WithTimeout(context.Background(), statusTimeout)
			defer cancel()
			s, err := src.Status(ctx)
			return statusMsg{s, err}
		},
		func() tea.Msg {
			ctx, cancel := context.WithTimeout(context.Background(), doctorTimeout)
			defer cancel()
			r, err := src.Doctor(ctx)
			return reportMsg{r, err}
		},
	)
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height

	case statusMsg:
		m.statusBusy = false
		m.statusErr = msg.err
		if msg.err == nil {
			m.status = &msg.s
		}

	case reportMsg:
		m.reportBusy = false
		m.reportErr = msg.err
		if msg.err == nil {
			m.report = &msg.r
			m.clampCursor()
		}

	case noteMsg:
		m.setNote(msg.text, msg.err)

	case execDoneMsg:
		// The command may have changed the machine (update, config), so look again.
		if msg.err != nil {
			m.setNote(msg.label+" exited", msg.err)
		} else {
			m.setNote(msg.label+" finished", nil)
		}
		return m, m.refresh()

	case tea.KeyPressMsg:
		return m, m.handleKey(msg)
	}
	return m, nil
}

func (m *Model) setNote(text string, err error) {
	m.note, m.noteErr = text, err != nil
	if err != nil {
		m.note = text + ": " + err.Error()
	}
}

func (m *Model) handleKey(msg tea.KeyPressMsg) tea.Cmd {
	switch msg.String() {
	case "q", "ctrl+c":
		return tea.Quit
	case "tab":
		m.tab = (m.tab + 1) % tabCount
	case "shift+tab":
		m.tab = (m.tab + tabCount - 1) % tabCount
	case "1":
		m.tab = tabDashboard
	case "2":
		m.tab = tabDoctor
	case "r":
		m.note = ""
		return m.refresh()

	// Actions. Update and config want the keyboard (sudo, $EDITOR), so their
	// pane takes focus; a followed log is for watching, so focus stays here.
	case "u":
		return m.run(m.opts.Source.Argv("update"), "hashiru update", true)
	case "e":
		return m.run(m.opts.Source.Argv("config"), "hashiru config", true)
	case "L":
		return m.run(m.opts.Source.Argv("log", "-f"), "hashiru log", false)
	}

	if m.tab == tabDoctor {
		return m.handleDoctorKey(msg)
	}
	return nil
}

func (m *Model) handleDoctorKey(msg tea.KeyPressMsg) tea.Cmd {
	n := len(m.sections())
	switch msg.String() {
	case "j", "down":
		if m.cursor < n-1 {
			m.cursor++
		}
	case "k", "up":
		if m.cursor > 0 {
			m.cursor--
		}
	case "enter", "space", "l", "right":
		if s := m.sections(); m.cursor < len(s) {
			name := s[m.cursor].Name
			m.expanded[name] = !m.expanded[name]
		}
	}
	return nil
}

func (m *Model) sections() []hashiru.Section {
	if m.report == nil {
		return nil
	}
	return m.report.Sections()
}

func (m *Model) clampCursor() {
	if n := len(m.sections()); m.cursor >= n {
		m.cursor = max(n-1, 0)
	}
}
