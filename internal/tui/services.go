package tui

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

	"github.com/whleucka/hashiru-tui/internal/system"
)

const (
	servicesTimeout = 10 * time.Second
	userActTimeout  = 30 * time.Second // a user unit's start can take a moment
)

// svcTab is one of the Services area's lists.
type svcTab int

const (
	tabSystem svcTab = iota
	tabUser
	tabFailed // both managers'
	svcTabCount
)

// services lists systemd's services and timers, system and user, and
// starts, stops and enables them. System units change through sudo in the
// terminal, where the password prompt works; user units need no prompt,
// so they change in the background with a note.
type services struct {
	e *env

	system, user       []system.Service
	systemErr, userErr error
	loaded             bool

	tab    svcTab
	sel    int
	offset int

	filtering bool
	input     textinput.Model

	statusOf  *system.Service // showing its systemctl status
	status    string
	statusErr error
	statusTop int

	height int
}

type (
	servicesMsg struct {
		system, user       []system.Service
		systemErr, userErr error
	}
	svcStatusMsg struct {
		name, out string
		err       error
	}
	// svcFocusMsg selects a unit: Logs sends it for a failed one.
	svcFocusMsg struct {
		name string
		user bool
	}
	// svcDoneMsg is a user unit's action finishing.
	svcDoneMsg struct {
		what string
		err  error
	}
)

func newServices(e *env) *services {
	return &services{e: e, input: newInput("/", "filter", 30)}
}

func (s *services) Name() string { return "Services" }
func (s *services) Typing() bool { return s.filtering }

func (s *services) Actions() []Action {
	return []Action{{ID: "services.failed", Title: "Show failed units", Run: func() tea.Cmd {
		s.tab, s.sel, s.offset = tabFailed, 0, 0
		return nil
	}}}
}

func (s *services) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case areaSizeMsg:
		s.height = msg.height
		s.input.SetWidth(msg.width - 2)
	case refreshMsg:
		return s.load()
	case servicesMsg:
		s.system, s.user, s.systemErr, s.userErr, s.loaded = msg.system, msg.user, msg.systemErr, msg.userErr, true
		s.sel = min(s.sel, max(len(s.shown())-1, 0))
	case svcStatusMsg:
		if s.statusOf != nil && s.statusOf.Name == msg.name {
			s.status, s.statusErr = msg.out, msg.err
		}
	case svcFocusMsg:
		s.focus(msg.name, msg.user)
	case svcDoneMsg:
		return tea.Batch(s.load(), func() tea.Msg { return noteMsg{text: msg.what, err: msg.err} })
	}
	return nil
}

func (s *services) load() tea.Cmd {
	probe := s.e.probe
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), servicesTimeout)
		defer cancel()
		var m servicesMsg
		m.system, m.systemErr = system.ReadServices(ctx, probe, false)
		m.user, m.userErr = system.ReadServices(ctx, probe, true)
		return m
	}
}

func (s *services) Tabs() []string { return []string{"system", "user", "failed"} }

func (s *services) Tab() int { return int(s.tab) }

func (s *services) SetTab(i int) { s.tab, s.sel, s.offset = svcTab(i), 0, 0 }

// focus selects a unit on the Failed tab, or wherever it's listed.
func (s *services) focus(name string, user bool) {
	s.filtering = false
	s.input.SetValue("")
	for _, tab := range []svcTab{tabFailed, tabSystem, tabUser} {
		s.tab = tab
		for i, u := range s.shown() {
			if u.Name == name && u.User == user {
				s.sel, s.offset = i, 0
				return
			}
		}
	}
	s.tab, s.sel = tabFailed, 0
}

func (s *services) rows(t svcTab) []system.Service {
	switch t {
	case tabSystem:
		return s.system
	case tabUser:
		return s.user
	}
	var out []system.Service
	for _, u := range append(append([]system.Service{}, s.system...), s.user...) {
		if u.Failed() {
			out = append(out, u)
		}
	}
	return out
}

func (s *services) shown() []system.Service {
	words := strings.Fields(strings.ToLower(s.input.Value()))
	var out []system.Service
outer:
	for _, u := range s.rows(s.tab) {
		hay := strings.ToLower(u.Name + " " + u.Description + " " + u.Active + " " + u.Sub + " " + u.State)
		for _, w := range words {
			if !strings.Contains(hay, w) {
				continue outer
			}
		}
		out = append(out, u)
	}
	return out
}

func (s *services) selected() (system.Service, bool) {
	shown := s.shown()
	if s.sel >= len(shown) {
		return system.Service{}, false
	}
	return shown[s.sel], true
}

const svcChange = "Change"

func (s *services) Keys() []Key {
	switch {
	case s.statusOf != nil:
		return []Key{{Keys: "j/k", Desc: "scroll"}, {Keys: "esc", Desc: "back to the list"}, {Keys: "l", Desc: "follow its log"}}
	case s.filtering:
		return []Key{{Keys: "type", Desc: "to filter"}, {Keys: "enter", Desc: "keep"}, {Keys: "esc", Desc: "clear"}}
	}
	return []Key{
		tabKey,
		{Keys: "j/k", Desc: "move"},
		{Keys: "enter", Desc: "status"},
		{Keys: "/", Desc: "filter"},
		{Keys: "l", Desc: "follow its log"},
		{Keys: "s", Desc: "start", Group: svcChange},
		{Keys: "S", Desc: "stop", Group: svcChange},
		{Keys: "R", Desc: "restart", Group: svcChange},
		{Keys: "e", Desc: "enable", Group: svcChange},
		{Keys: "d", Desc: "disable", Group: svcChange},
	}
}

func (s *services) HandleKey(msg tea.KeyPressMsg) (tea.Cmd, bool) {
	k := msg.String()
	if s.statusOf != nil {
		switch k {
		case "esc", "h", "left":
			s.statusOf = nil
		case "l":
			return s.follow(*s.statusOf), true
		default:
			i, ok := moveKey(k, s.statusTop, strings.Count(s.status, "\n"), s.height-2)
			s.statusTop = i
			return nil, ok
		}
		return nil, true
	}
	if s.filtering {
		switch k {
		case "esc":
			s.input.SetValue("")
			fallthrough
		case "enter":
			s.filtering = false
			s.input.Blur()
			s.sel = min(s.sel, max(len(s.shown())-1, 0))
		default:
			var cmd tea.Cmd
			s.input, cmd = s.input.Update(msg)
			s.sel, s.offset = 0, 0
			return cmd, true
		}
		return nil, true
	}

	if t, ok := nextTab(k, int(s.tab), int(svcTabCount)); ok {
		s.tab, s.sel, s.offset = svcTab(t), 0, 0
		return nil, true
	}
	if i, ok := moveKey(k, s.sel, len(s.shown()), s.height-3); ok {
		s.sel = i
		return nil, true
	}
	switch k {
	case "/":
		s.filtering = true
		return s.input.Focus(), true
	case "esc":
		if s.input.Value() == "" {
			return nil, false // back to the sidebar
		}
		s.input.SetValue("")
	case "enter":
		return s.showStatus(), true
	case "l":
		if u, ok := s.selected(); ok {
			return s.follow(u), true
		}
	case "s":
		return s.act("start"), true
	case "S":
		return s.act("stop"), true
	case "R":
		return s.act("restart"), true
	case "e":
		return s.act("enable"), true
	case "d":
		return s.act("disable"), true
	default:
		return nil, false
	}
	return nil, true
}

// systemctl is argv for systemctl on u's manager.
func systemctl(u system.Service, args ...string) []string {
	if u.User {
		return append([]string{"systemctl", "--user"}, args...)
	}
	return append([]string{"systemctl"}, args...)
}

// act runs verb on the selected unit: through sudo in the terminal for a
// system unit, quietly for a user one.
func (s *services) act(verb string) tea.Cmd {
	u, ok := s.selected()
	if !ok {
		return nil
	}
	argv := systemctl(u, verb, u.Name)
	what := verb + " " + u.Name
	if !u.User {
		return s.e.launch(Launch{Argv: append([]string{"sudo"}, argv...), Label: what, Kind: Interactive, Refresh: true})
	}
	probe := s.e.probe
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), userActTimeout)
		defer cancel()
		_, err := probe(ctx, argv[0], argv[1:]...)
		if err == nil {
			what += ": done"
		}
		return svcDoneMsg{what: what, err: err}
	}
}

// follow opens the unit's journal, live, in a pane (or this terminal).
func (s *services) follow(u system.Service) tea.Cmd {
	argv := []string{"journalctl", "-u", u.Name, "-f"}
	if u.User {
		argv = []string{"journalctl", "--user", "-u", u.Name, "-f"}
	}
	return s.e.launch(Launch{Argv: argv, Label: u.Name, Kind: Watch})
}

func (s *services) showStatus() tea.Cmd {
	u, ok := s.selected()
	if !ok {
		return nil
	}
	s.statusOf, s.status, s.statusErr, s.statusTop = &u, "", nil, 0
	argv := systemctl(u, "status", "--no-pager", "--full", u.Name)
	probe := s.e.probe
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), servicesTimeout)
		defer cancel()
		out, err := probe(ctx, argv[0], argv[1:]...)
		// status exits 3 for a unit that isn't running, and still prints it.
		if code, isExit := exitCodeOf(err); isExit && code == 3 && len(out) > 0 {
			err = nil
		}
		return svcStatusMsg{u.Name, string(out), err}
	}
}

// exitCodeOf is err's exit code, if it's a command exiting.
func exitCodeOf(err error) (int, bool) {
	var e interface{ ExitCode() int }
	if errors.As(err, &e) {
		return e.ExitCode(), true
	}
	return 0, false
}

func (s *services) View(width, height int) string {
	switch {
	case !s.loaded:
		return styleMuted.Render("reading systemd's units…")
	case s.statusOf != nil:
		return s.statusView(height)
	}

	var b strings.Builder
	b.WriteString(s.tabs() + "\n")
	switch err := s.tabErr(); {
	case err != nil:
		b.WriteString(styleFail.Render(err.Error()))
	case s.filtering || s.input.Value() != "":
		b.WriteString(s.input.View())
	default:
		b.WriteString(styleMuted.Render(s.summary()))
	}
	b.WriteString("\n\n")

	shown := s.shown()
	rows := max(height-3, 1)
	if s.sel < s.offset {
		s.offset = s.sel
	}
	if s.sel >= s.offset+rows {
		s.offset = s.sel - rows + 1
	}
	for i := s.offset; i < len(shown) && i < s.offset+rows; i++ {
		u := shown[i]
		cursor, name := "  ", cellMid(u.Name, 36)
		if i == s.sel {
			cursor, name = styleTitle.Render("▸ "), styleSelected.Render(name)
		}
		b.WriteString(cursor + svcMark(u) + " " + name + "  " +
			svcState(u) + "  " + styleMuted.Render(cell(u.State, 9)) + "  " + styleMuted.Render(svcAbout(u)) + "\n")
	}
	if len(shown) == 0 {
		empty := "none"
		switch {
		case s.input.Value() != "":
			empty = "nothing matches"
		case s.tab == tabFailed:
			empty = "nothing has failed"
		}
		b.WriteString(styleMuted.Render("  " + empty))
	}
	return b.String()
}

func (s *services) tabs() string {
	labels := []string{
		fmt.Sprintf("System (%d)", len(s.system)),
		fmt.Sprintf("User (%d)", len(s.user)),
		fmt.Sprintf("Failed (%d)", len(s.rows(tabFailed))),
	}
	return tabBar(labels, int(s.tab), func(i int) bool {
		return svcTab(i) == tabFailed && len(s.rows(tabFailed)) > 0
	})
}

func (s *services) tabErr() error {
	switch s.tab {
	case tabSystem:
		return s.systemErr
	case tabUser:
		return s.userErr
	}
	return nil
}

func (s *services) summary() string {
	running := 0
	for _, u := range s.rows(s.tab) {
		if u.Running() {
			running++
		}
	}
	how := "sudo systemctl, in the terminal"
	if s.tab == tabUser {
		how = "systemctl --user, no password"
	}
	return fmt.Sprintf("%d running · changes run %s", running, how)
}

func svcMark(u system.Service) string {
	switch {
	case u.Failed():
		return styleFail.Render("✘")
	case u.Running():
		return styleOK.Render("●")
	}
	return styleMuted.Render("○")
}

func svcState(u system.Service) string {
	st := cell(u.Active+" "+u.Sub, 18)
	switch {
	case u.Failed():
		return styleFail.Render(st)
	case u.Running():
		return styleOK.Render(st)
	}
	return styleMuted.Render(st)
}

// svcAbout is the last column: a timer's next run, else the description.
func svcAbout(u system.Service) string {
	if u.Timer() && !u.Next.IsZero() {
		return "next " + until(u.Next) + " · " + u.Description
	}
	return u.Description
}

// until is how long until t, briefly: "53m", "6h", "2d".
func until(t time.Time) string {
	d := time.Until(t)
	switch {
	case d < time.Minute:
		return "now"
	case d < time.Hour:
		return fmt.Sprintf("in %dm", int(d.Minutes()))
	case d < 48*time.Hour:
		return fmt.Sprintf("in %dh", int(d.Hours()))
	}
	return fmt.Sprintf("in %dd", int(d.Hours()/24))
}

func (s *services) statusView(height int) string {
	var b strings.Builder
	u := s.statusOf
	b.WriteString(styleTitle.Render(u.Name) + styleMuted.Render("  "+strings.Join(systemctl(*u, "status", u.Name), " ")) + "\n\n")
	switch {
	case s.statusErr != nil:
		b.WriteString(styleFail.Render(s.statusErr.Error()))
		return b.String()
	case s.status == "":
		b.WriteString(styleMuted.Render("reading…"))
		return b.String()
	}
	lines := strings.Split(strings.TrimRight(s.status, "\n"), "\n")
	top := min(s.statusTop, max(len(lines)-1, 0))
	for i := top; i < len(lines) && i < top+height-2; i++ {
		b.WriteString(lines[i] + "\n")
	}
	return b.String()
}
