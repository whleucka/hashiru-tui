// Package tui is the bubbletea front end: a sidebar of areas, each a view over
// part of the system, and one way to launch the tools that change it.
package tui

import (
	"context"
	"os"
	"os/exec"
	"strconv"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/whleucka/hashiru-tui/internal/hashiru"
	"github.com/whleucka/hashiru-tui/internal/kitty"
	"github.com/whleucka/hashiru-tui/internal/system"
)

// Source is where Hashiru's own state comes from. hashiru.CLI satisfies it;
// tests use a fake.
type Source interface {
	Status(context.Context) (hashiru.Status, error)
	Doctor(context.Context) (hashiru.Report, error)
	Log() ([]hashiru.Run, error)
	Argv(args ...string) []string
}

// Options wires the model to the outside world.
type Options struct {
	Source Source
	// RunInPane runs argv in a new herdr pane named label, to the right or
	// below, focused or not. Nil outside herdr; commands then take over this
	// terminal through tea.ExecProcess instead.
	RunInPane func(ctx context.Context, argv []string, label string, right, focus bool) error
	// Detach starts a GUI app in the background. Defaults to a setsid'd process.
	Detach func(argv []string) error
	// Exec hands the terminal to a command. Defaults to tea.ExecProcess.
	Exec func(*exec.Cmd, tea.ExecCallback) tea.Cmd
	// Probe runs the read-only commands areas show (checkupdates, nmcli...).
	// Defaults to running them for real.
	Probe system.Run
	// Get fetches a URL (Arch news). Defaults to a real HTTP GET.
	Get system.Get
	// SysRoot is where /proc and /sys are read from; Statfs sizes /.
	SysRoot string
	Statfs  system.Statfs
	// Home is the user's home directory, for the Config area.
	Home string
	// Graphics is whether to draw image previews; auto asks the environment.
	Graphics graphicsMode
	// Tick schedules a message after a delay, for Overview's live stats.
	// Defaults to tea.Tick.
	Tick func(time.Duration, func(time.Time) tea.Msg) tea.Cmd
	// PacmanCache is pacman's package cache, sized in Packages. Defaults to
	// /var/cache/pacman/pkg.
	PacmanCache string
	// RecentPath is where the palette keeps recently run actions. Defaults
	// to $XDG_STATE_HOME/hashiru-tui/recent.
	RecentPath string
}

// defaultProbe is a variable so tests can keep every model off the machine.
var (
	defaultProbe system.Run = system.Command
	defaultGet   system.Get = system.HTTPGet

	defaultTick        = tea.Tick
	defaultPacmanCache = system.PacmanCache

	defaultHome     = os.UserHomeDir
	defaultGraphics = func() bool { return kitty.Supported(os.Getenv) }

	defaultSysRoot               = "/"
	defaultStatfs  system.Statfs = system.RootStatfs
)

// Model is the shell: sidebar, the selected area, and the footer.
type Model struct {
	opts          Options
	width, height int

	env    *env
	state  *State
	areas  []Area
	sel    int  // selected area
	inArea bool // keys go to the area first rather than the sidebar

	pal     palette
	keyHelp bool     // the ? popup is open
	recent  []string // IDs of actions run from the palette, newest first
	// quitting: q was pressed and the footer asks to confirm. Only y quits;
	// any other key stays, so a stray q doesn't close the window.
	quitting bool
	// startRun is the action a deep link asked for; it runs on the first
	// window size, so what it launches has a laid-out screen to return to.
	startRun *Action

	note    string // one-line feedback in the footer
	noteErr bool
}

// New builds the model. Call Init (bubbletea does) to start loading.
func New(opts Options) *Model {
	if opts.Detach == nil {
		opts.Detach = detach
	}
	if opts.Exec == nil {
		opts.Exec = tea.ExecProcess
	}
	if opts.Probe == nil {
		opts.Probe = defaultProbe
	}
	if opts.Get == nil {
		opts.Get = defaultGet
	}
	if opts.SysRoot == "" {
		opts.SysRoot = defaultSysRoot
	}
	if opts.Statfs == nil {
		opts.Statfs = defaultStatfs
	}
	if opts.Home == "" {
		opts.Home, _ = defaultHome()
	}
	if opts.PacmanCache == "" {
		opts.PacmanCache = defaultPacmanCache
	}
	if opts.Tick == nil {
		opts.Tick = defaultTick
	}
	if opts.RecentPath == "" {
		opts.RecentPath = defaultRecentPath(opts.Home)
	}
	m := &Model{opts: opts, state: &State{}, recent: loadRecent(opts.RecentPath)}
	m.env = &env{
		src:     opts.Source,
		probe:   opts.Probe,
		state:   m.state,
		launch:  m.launch,
		refresh: m.refresh,
	}
	m.areas = newAreas(m.env, opts, opts.Graphics == graphicsOn ||
		opts.Graphics == graphicsAuto && defaultGraphics())
	return m
}

type (
	noteMsg struct {
		text string
		err  error
	}
	// execDoneMsg arrives when a command that borrowed the terminal exits.
	execDoneMsg struct {
		label   string
		err     error
		refresh bool
	}
	// refreshMsg asks areas to reload whatever they load themselves.
	refreshMsg struct{}
)

// sysInterval is how often Overview's local stats reload while it's shown.
const sysInterval = 5 * time.Second

type sysTickMsg struct{}

func (m *Model) Init() tea.Cmd {
	// The ticker starts here, not in refresh, so r doesn't start another.
	return tea.Batch(m.refresh(), m.sysTick())
}

func (m *Model) sysTick() tea.Cmd {
	return m.opts.Tick(sysInterval, func(time.Time) tea.Msg { return sysTickMsg{} })
}

// refresh reloads the shared state and lets every area reload its own.
func (m *Model) refresh() tea.Cmd {
	return tea.Batch(m.state.load(m.opts), m.broadcast(refreshMsg{}))
}

func (m *Model) area() Area { return m.areas[m.sel] }

// broadcast hands msg to every area.
func (m *Model) broadcast(msg tea.Msg) tea.Cmd {
	cmds := make([]tea.Cmd, 0, len(m.areas))
	for _, a := range m.areas {
		cmds = append(cmds, a.Update(msg))
	}
	return tea.Batch(cmds...)
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		w, h := m.areaSize()
		cmd := m.broadcast(areaSizeMsg{w, h})
		if act := m.startRun; act != nil {
			m.startRun = nil
			cmd = tea.Batch(cmd, act.Run())
		}
		return m, cmd

	case tea.KeyPressMsg:
		return m, m.handleKey(msg)

	case noteMsg:
		m.setNote(msg.text, msg.err)
		return m, nil

	case sysTickMsg:
		// Only while Overview is on screen; nothing else re-runs.
		if _, ok := m.area().(*overview); ok && !m.pal.open {
			return m, tea.Batch(readSys(m.opts), m.sysTick())
		}
		return m, m.sysTick()

	case execDoneMsg:
		if msg.err != nil {
			m.setNote(msg.label+" exited", msg.err)
		} else {
			m.setNote(msg.label+" finished", nil)
		}
		if msg.refresh {
			return m, m.refresh() // it may have changed the machine
		}
		return m, nil
	}

	m.state.apply(msg)
	return m, m.broadcast(msg)
}

func (m *Model) setNote(text string, err error) {
	m.note, m.noteErr = text, err != nil
	if err != nil {
		m.note = text + ": " + err.Error()
	}
}

func (m *Model) handleKey(msg tea.KeyPressMsg) tea.Cmd {
	switch msg.String() {
	case "ctrl+c":
		return tea.Quit
	}
	if m.quitting {
		m.quitting = false
		if msg.String() == "y" {
			return tea.Quit
		}
		return nil // the key only answers the question
	}
	if m.keyHelp {
		return m.handleKeyHelpKey(msg)
	}
	if msg.String() == "ctrl+p" && !m.pal.open {
		return m.openPalette()
	}
	if m.pal.open {
		return m.handlePaletteKey(msg)
	}
	if msg.String() == "?" && !m.typing() {
		return m.setKeyHelp(true)
	}

	if m.inArea {
		if cmd, ok := m.area().HandleKey(msg); ok {
			return cmd
		}
		switch msg.String() {
		case "esc", "h", "left":
			m.inArea = false
			return nil
		}
		cmd, _ := m.shellKey(msg)
		return cmd
	}

	switch msg.String() {
	case "j", "down":
		m.selectArea(m.sel + 1)
		return nil
	case "k", "up":
		m.selectArea(m.sel - 1)
		return nil
	case "enter", "l", "right":
		m.inArea = true
		return nil
	case ":":
		return m.openPalette()
	}
	if cmd, ok := m.shellKey(msg); ok {
		return cmd
	}
	// The selected area's own action keys work from the sidebar too.
	cmd, _ := m.area().HandleKey(msg)
	return cmd
}

// shellKey handles the keys that mean the same everywhere, reporting whether
// msg was one.
func (m *Model) shellKey(msg tea.KeyPressMsg) (tea.Cmd, bool) {
	switch s := msg.String(); s {
	case "q":
		m.quitting = true
		return nil, true
	case "r":
		m.note = ""
		return m.refresh(), true
	case "tab":
		m.selectArea((m.sel + 1) % len(m.areas))
		return nil, true
	case "shift+tab":
		m.selectArea((m.sel + len(m.areas) - 1) % len(m.areas))
		return nil, true
	default:
		if n, err := strconv.Atoi(s); err == nil && n >= 1 && n <= len(m.areas) {
			m.selectArea(n - 1)
			m.inArea = false
			return nil, true
		}
	}
	return nil, false
}

func (m *Model) selectArea(i int) {
	if i >= 0 && i < len(m.areas) {
		m.sel = i
	}
}
