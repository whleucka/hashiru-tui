package tui

import (
	"context"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/whleucka/hashiru-tui/internal/system"
)

// monitors is Hyprland's outputs, with changes staged on top until they're
// applied. Applying writes the override monitors.lua and reloads, rather
// than hyprctl eval'ing hl.monitor per change, which clamshell.lua documents
// desyncing from DRM.
type monitors struct {
	e      *env
	dir    string // ~/.config/hashiru/hypr, where the override goes
	tick   func(time.Duration, func(time.Time) tea.Msg) tea.Cmd
	live   []system.Monitor
	err    error
	loaded bool
	staged system.Layout // nil: nothing staged
	sel    int
	height int
	detail bool
	picker *choice

	askHand bool     // asking before replacing a hand-written override
	pending *applied // applied, waiting for y before the timer reverts it
	seq     int      // tells this countdown's ticks from an earlier one's
	asked   system.Layout
}

// applied is a layout written and reloaded, and what to put back.
type applied struct {
	prev    []byte
	existed bool
	left    int // seconds
}

// keepFor is how long an applied layout waits for y. A display left dark
// by a mode it can't show comes back on its own after this.
const keepFor = 15

type (
	monTickMsg struct{ seq int }
	reloadMsg  struct {
		err    error
		revert bool // this reload put the old file back
	}
)

type monitorsMsg struct {
	ms  []system.Monitor
	err error
}

const monitorsTimeout = 5 * time.Second

// monitorScales are the scales the picker offers. Hyprland nudges one that
// doesn't divide the mode evenly; the list shows what it used.
var monitorScales = []float64{1, 1.25, 4.0 / 3, 1.5, 1.6, 5.0 / 3, 2, 2.5, 3}

var monitorPlaces = []string{"left", "right", "above", "below"}

func newMonitors(e *env, home string, tick func(time.Duration, func(time.Time) tea.Msg) tea.Cmd) *monitors {
	return &monitors{e: e, dir: filepath.Join(system.HashiruConfigDir(home), "hypr"), tick: tick}
}

func (m *monitors) file() string { return filepath.Join(m.dir, "monitors.lua") }

func (m *monitors) name() string { return "monitors" }

func (m *monitors) label() string {
	l := "Monitors"
	if m.loaded && m.err == nil {
		l += fmt.Sprintf(" (%d)", len(m.live))
	}
	if m.staged != nil {
		l += " *"
	}
	return l
}

// warn while there are changes that aren't applied.
func (m *monitors) warn() bool { return m.staged != nil }

func (m *monitors) update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case areaSizeMsg:
		m.height = msg.height
	case refreshMsg:
		return m.probe()
	case monTickMsg:
		if m.pending == nil || msg.seq != m.seq {
			return nil // a countdown already answered
		}
		if m.pending.left--; m.pending.left <= 0 {
			return m.revert()
		}
		return m.nextTick()
	case reloadMsg:
		switch {
		case msg.err != nil && !msg.revert:
			// Hyprland didn't take it: put the old file back at once.
			cmd := m.revert()
			return tea.Batch(cmd, func() tea.Msg { return noteMsg{text: "hyprctl reload", err: msg.err} })
		case msg.err != nil:
			return func() tea.Msg { return noteMsg{text: "hyprctl reload (putting the old layout back)", err: msg.err} }
		}
		return m.probe()
	case monitorsMsg:
		var cmd tea.Cmd
		if m.asked != nil && msg.err == nil {
			cmd = scaleNote(m.asked, msg.ms)
			m.asked = nil
		}
		defer func() { m.sel = min(m.sel, max(len(m.live)-1, 0)) }()
		// A display plugged in or out under staged changes: they described
		// a different desk, so they go.
		if m.staged != nil && !sameOutputs(m.live, msg.ms) {
			m.staged = nil
		}
		m.live, m.err, m.loaded = msg.ms, msg.err, true
		return cmd
	}
	return nil
}

// scaleNote says when Hyprland used a scale other than the one asked for,
// which it does when the asked one doesn't divide the mode evenly.
func scaleNote(asked system.Layout, live []system.Monitor) tea.Cmd {
	var odd []string
	for _, mon := range live {
		for _, o := range asked {
			if o.Name == mon.Name && !o.Disabled && scaleText(o.Scale) != scaleText(mon.Scale) {
				odd = append(odd, fmt.Sprintf("%s at %s, not %s", mon.Name, scaleText(mon.Scale), scaleText(o.Scale)))
			}
		}
	}
	if len(odd) == 0 {
		return nil
	}
	return note("Hyprland put " + strings.Join(odd, ", ") + " (the nearest scale that fits its mode)")
}

func (m *monitors) probe() tea.Cmd {
	probe := m.e.probe
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), monitorsTimeout)
		defer cancel()
		out, err := probe(ctx, "hyprctl", system.MonitorsArgs...)
		if err != nil {
			return monitorsMsg{err: err}
		}
		ms, err := system.ParseMonitors(out)
		return monitorsMsg{ms, err}
	}
}

func sameOutputs(a, b []system.Monitor) bool {
	return slices.EqualFunc(a, b, func(x, y system.Monitor) bool { return x.Name == y.Name })
}

// layout is what applying would write: the staged changes, or the live
// layout when there are none.
func (m *monitors) layout() system.Layout {
	if m.staged != nil {
		return m.staged
	}
	return system.LayoutOf(m.live)
}

// stage changes the selected output in a copy of the layout.
func (m *monitors) stage(change func(l system.Layout, i int) system.Layout) tea.Cmd {
	l := change(slices.Clone(m.layout()), m.sel)
	if !l.AnyEnabled() {
		return note("that would leave no display on")
	}
	if slices.Equal(l, system.LayoutOf(m.live)) {
		m.staged = nil // changed back
	} else {
		m.staged = l
	}
	return nil
}

func (m *monitors) actions() []Action {
	return []Action{{ID: "hardware.monitors-apply", Title: "Apply the staged monitor layout (writes monitors.lua)", Key: "a", Run: m.apply}}
}

// Typing while a question is open, so every key (? and [ ] too) answers it.
func (m *monitors) Typing() bool { return m.askHand || m.pending != nil }

// apply writes the staged layout over the override, asking first if the
// override was written by hand.
func (m *monitors) apply() tea.Cmd {
	if m.staged == nil {
		return note("nothing staged to apply")
	}
	prev, err := os.ReadFile(m.file())
	existed := err == nil
	if err != nil && !os.IsNotExist(err) {
		return func() tea.Msg { return noteMsg{text: "read monitors.lua", err: err} }
	}
	if existed && !system.HasHeader(prev) && !m.askHand {
		m.askHand = true
		return nil
	}
	m.askHand = false

	if existed && !system.HasHeader(prev) {
		if err := writeFile(m.file()+".bak", prev); err != nil {
			return func() tea.Msg { return noteMsg{text: "back up monitors.lua", err: err} }
		}
	}
	lua := system.MonitorsLua(m.staged, system.ParseRules(prev), m.live)
	if err := writeFile(m.file(), lua); err != nil {
		return func() tea.Msg { return noteMsg{text: "write monitors.lua", err: err} }
	}
	m.pending = &applied{prev: prev, existed: existed, left: keepFor}
	m.asked = m.staged
	m.seq++
	return tea.Batch(m.reload(false), m.nextTick())
}

func (m *monitors) nextTick() tea.Cmd {
	seq := m.seq
	return m.tick(time.Second, func(time.Time) tea.Msg { return monTickMsg{seq} })
}

func (m *monitors) reload(revert bool) tea.Cmd {
	probe := m.e.probe
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), monitorsTimeout)
		defer cancel()
		_, err := probe(ctx, "hyprctl", "reload")
		return reloadMsg{err: err, revert: revert}
	}
}

// keep ends the countdown with the new layout in place.
func (m *monitors) keep() tea.Cmd {
	m.pending, m.staged = nil, nil
	return tea.Batch(note("layout kept in "+m.file()), m.probe())
}

// revert puts the old override back (or removes ours, if there was none)
// and reloads. The changes stay staged, to adjust and try again.
func (m *monitors) revert() tea.Cmd {
	p := m.pending
	m.pending, m.asked = nil, nil
	if p == nil {
		return nil
	}
	var err error
	if p.existed {
		err = writeFile(m.file(), p.prev)
	} else {
		err = os.Remove(m.file())
	}
	if err != nil {
		return func() tea.Msg { return noteMsg{text: "put monitors.lua back", err: err} }
	}
	return tea.Batch(note("put the old layout back"), m.reload(true))
}

// beforeQuit puts the old layout back if the TUI is quitting mid-countdown,
// so ctrl+c can't keep a layout nobody said yes to.
func (m *monitors) beforeQuit() {
	if m.pending == nil {
		return
	}
	if cmd := m.revert(); cmd != nil {
		ctx, cancel := context.WithTimeout(context.Background(), monitorsTimeout)
		defer cancel()
		_, _ = m.e.probe(ctx, "hyprctl", "reload")
	}
}

// writeFile replaces path in one step: a temp file beside it, renamed over.
func writeFile(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".hashiru-tui-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name()) // a no-op once renamed
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmp.Name(), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}

func (m *monitors) keys() []Key {
	switch {
	case m.askHand:
		return []Key{{Keys: "y", Desc: "replace it"}, {Keys: "any other key", Desc: "cancel"}}
	case m.pending != nil:
		return []Key{{Keys: "y", Desc: "keep"}, {Keys: "any other key", Desc: "put the old one back"}}
	case m.picker != nil:
		return m.picker.keys()
	case m.detail:
		return []Key{{Keys: "esc", Desc: "back to the list"}}
	}
	keys := []Key{
		{Keys: "j/k", Desc: "move"},
		{Keys: "enter", Desc: "details"},
		{Keys: "m", Desc: "mode", Group: "Change (staged)"},
		{Keys: "s", Desc: "scale", Group: "Change (staged)"},
		{Keys: "t", Desc: "rotate", Group: "Change (staged)"},
		{Keys: "p", Desc: "place", Group: "Change (staged)"},
		{Keys: "e/d", Desc: "on/off", Group: "Change (staged)"},
	}
	if m.staged != nil {
		keys = append(keys, Key{Keys: "a", Desc: "apply"}, Key{Keys: "esc", Desc: "drop changes"})
	}
	return keys
}

func (m *monitors) handleKey(msg tea.KeyPressMsg) (tea.Cmd, bool) {
	k := msg.String()
	switch {
	case m.askHand:
		if k == "y" {
			return m.apply(), true
		}
		m.askHand = false
		return nil, true
	case m.pending != nil:
		if k == "y" {
			return m.keep(), true
		}
		return m.revert(), true
	}
	if m.picker != nil {
		cmd, done := m.picker.handleKey(k, m.height-3)
		if done {
			m.picker = nil
		}
		return cmd, true
	}
	if m.detail {
		if k == "esc" || k == "h" || k == "left" {
			m.detail = false
			return nil, true
		}
		return nil, false
	}
	if len(m.live) == 0 {
		return nil, false
	}
	if i, ok := moveKey(k, m.sel, len(m.live), m.height-3); ok {
		m.sel = i
		return nil, true
	}
	switch k {
	case "enter", "l", "right":
		m.detail = true
	case "esc":
		if m.staged == nil {
			return nil, false
		}
		m.staged = nil
	case "a":
		return m.apply(), true
	case "m":
		m.pickMode()
	case "s":
		m.pickScale()
	case "p":
		m.pickPlace()
	case "t":
		return m.stage(func(l system.Layout, i int) system.Layout {
			t := l[i].Transform
			l[i].Transform = t/4*4 + (t%4+1)%4 // a quarter turn, keeping any flip
			return l
		}), true
	case "e", "d":
		if system.Internal(m.live[m.sel].Name) {
			return note("the laptop panel goes off and on with the lid (clamshell.lua)"), true
		}
		off := k == "d"
		return m.stage(func(l system.Layout, i int) system.Layout {
			l[i].Disabled = off
			return l
		}), true
	default:
		return nil, false
	}
	return nil, true
}

func (m *monitors) pickMode() {
	mon := m.live[m.sel]
	opts := append([]string{"preferred"}, mon.Modes...)
	sel := max(slices.Index(opts, m.layout()[m.sel].Mode), 0)
	m.picker = &choice{
		title: "Mode for " + mon.Name, about: "preferred is the display's own pick",
		opts: opts, sel: sel,
		pick: func(i int) tea.Cmd {
			return m.stage(func(l system.Layout, j int) system.Layout {
				l[j].SetMode(opts[i])
				return l
			})
		},
	}
}

func (m *monitors) pickScale() {
	cur := m.layout()[m.sel].Scale
	opts := make([]string, len(monitorScales))
	sel := 0
	for i, s := range monitorScales {
		opts[i] = scaleText(s)
		if scaleText(s) == scaleText(cur) {
			sel = i
		}
	}
	m.picker = &choice{
		title: "Scale for " + m.live[m.sel].Name, about: "Hyprland rounds one that doesn't fit the mode",
		opts: opts, sel: sel,
		pick: func(i int) tea.Cmd {
			return m.stage(func(l system.Layout, j int) system.Layout {
				l[j].Scale = monitorScales[i]
				return l
			})
		},
	}
}

func (m *monitors) pickPlace() {
	l := m.layout()
	var opts []string
	var targets []int
	for j, o := range l {
		if j == m.sel || o.Disabled {
			continue
		}
		for _, rel := range monitorPlaces {
			opts = append(opts, rel+" "+o.Name)
			targets = append(targets, j)
		}
	}
	if len(opts) == 0 {
		return
	}
	m.picker = &choice{
		title: "Place " + m.live[m.sel].Name, about: "lined up with the other's top or left edge",
		opts: opts,
		pick: func(i int) tea.Cmd {
			return m.stage(func(l system.Layout, j int) system.Layout {
				return system.Place(l, j, monitorPlaces[i%len(monitorPlaces)], targets[i])
			})
		},
	}
}

// scaleText is a scale as people write it: 1, 1.25, 1.333.
func scaleText(s float64) string { return strconv.FormatFloat(math.Round(s*1000)/1000, 'f', -1, 64) }

func rotationText(t int) string {
	s := fmt.Sprintf("%d°", t%4*90)
	if t >= 4 {
		s += " flipped"
	}
	return s
}

func onText(disabled bool) string {
	if disabled {
		return "off"
	}
	return "on"
}

func (m *monitors) view(width, height int) string {
	switch {
	case m.err != nil:
		return styleFail.Render("hyprctl " + strings.Join(system.MonitorsArgs, " ") + ": " + m.err.Error())
	case !m.loaded:
		return styleMuted.Render("asking Hyprland…")
	case m.askHand:
		return styleWarn.Render("monitors.lua was written by hand") + "\n" + styleMuted.Render(m.file()) + "\n\n" +
			"Replace it with the staged layout? It's copied to monitors.lua.bak first,\n" +
			"and rules for displays that aren't connected are carried over.\n\n" +
			styleMuted.Render("y replaces it · any other key cancels")
	case m.pending != nil:
		return styleTitle.Render("Keep this layout?") + "\n\n" +
			fmt.Sprintf("y keeps it · any other key puts the old one back · reverting in %ds", m.pending.left) + "\n\n" +
			styleMuted.Render("written to "+m.file())
	case m.picker != nil:
		return m.picker.view(height)
	case m.detail:
		return m.detailView()
	}

	var b strings.Builder
	if m.staged != nil {
		b.WriteString(styleWarn.Render("changes staged, not applied") + styleMuted.Render(" · esc drops them") + "\n\n")
	} else {
		b.WriteString(styleMuted.Render("as Hyprland has them now · changes are staged until applied") + "\n\n")
	}
	live, cur := system.LayoutOf(m.live), m.layout()
	rows := make([][]string, len(m.live))
	for i, mon := range m.live {
		was, now := live[i], cur[i]
		rows[i] = []string{
			mon.Name,
			diff(was.Mode, now.Mode),
			diff(fmt.Sprintf("%dx%d", was.X, was.Y), fmt.Sprintf("%dx%d", now.X, now.Y)),
			diff(scaleText(was.Scale), scaleText(now.Scale)),
			diff(rotationText(was.Transform), rotationText(now.Transform)),
			diff(onText(was.Disabled), onText(now.Disabled)),
			styleMuted.Render(mon.Description),
		}
	}
	// Each column as wide as its widest cell, so a staged "old → new" pushes
	// its column over rather than the ones after it out of line.
	widths := make([]int, len(rows[0])-1)
	for _, r := range rows {
		for c := range widths {
			widths[c] = max(widths[c], ansi.StringWidth(r[c]))
		}
	}
	for i, r := range rows {
		cursor := "  "
		if i == m.sel {
			cursor, r[0] = styleTitle.Render("▸ "), styleSelected.Render(r[0])
		}
		for c, w := range widths {
			r[c] += strings.Repeat(" ", w-ansi.StringWidth(r[c]))
		}
		b.WriteString(cursor + strings.Join(r, "  ") + "\n")
	}
	return b.String()
}

// diff is a column's value, or "old → new" in the warning colour when a
// change is staged.
func diff(was, now string) string {
	if was == now {
		return was
	}
	return styleWarn.Render(was + " → " + now)
}

func (m *monitors) detailView() string {
	mon, o := m.live[m.sel], m.layout()[m.sel]
	var b strings.Builder
	b.WriteString(styleTitle.Render(mon.Name) + styleMuted.Render("  "+mon.Description) + "\n\n")
	field := func(k, v string) { b.WriteString(styleKey.Render(k) + v + "\n") }
	field("rule", styleMuted.Render(`output = "`+o.Output()+`"`))
	field("mode", o.Mode)
	field("position", fmt.Sprintf("%dx%d", o.X, o.Y))
	w, h := o.Logical()
	field("scale", fmt.Sprintf("%s  %s", scaleText(o.Scale), styleMuted.Render(fmt.Sprintf("(%dx%d in the layout)", w, h))))
	field("rotation", rotationText(o.Transform))
	field("state", onText(o.Disabled))
	if mon.MirrorOf != "" {
		field("mirrors", mon.MirrorOf)
	}
	b.WriteString("\n" + styleTitle.Render("Modes") + "\n")
	for _, mode := range mon.Modes {
		mark := "  "
		if mode == system.LayoutOf(m.live)[m.sel].Mode {
			mark = styleOK.Render("● ")
		}
		b.WriteString(mark + mode + "\n")
	}
	return b.String()
}
