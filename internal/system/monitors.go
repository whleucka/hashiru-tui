package system

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// Monitor is one output as Hyprland reports it.
type Monitor struct {
	Name        string // "eDP-1"
	Description string // "LG Electronics LG ULTRAGEAR 605BNHC03417"
	Width       int    // pixels, in the current mode
	Height      int
	Refresh     float64
	X, Y        int
	Scale       float64
	Transform   int // 0-3: quarter turns; 4-7 flipped
	Disabled    bool
	Focused     bool
	Modes       []string // "1920x1200@60.00", as monitors.lua takes them
	MirrorOf    string   // "" unless mirroring
}

// MonitorsArgs lists every output, disabled ones too.
var MonitorsArgs = []string{"monitors", "all", "-j"}

// ParseMonitors reads `hyprctl monitors all -j`.
func ParseMonitors(out []byte) ([]Monitor, error) {
	var raw []struct {
		Name, Description string
		Width, Height     int
		RefreshRate       float64
		X, Y              int
		Scale             float64
		Transform         int
		Disabled, Focused bool
		AvailableModes    []string
		MirrorOf          string
	}
	if err := json.Unmarshal(out, &raw); err != nil {
		return nil, fmt.Errorf("hyprctl monitors: %w", err)
	}
	ms := make([]Monitor, len(raw))
	for i, r := range raw {
		m := Monitor{
			Name: r.Name, Description: r.Description,
			Width: r.Width, Height: r.Height, Refresh: r.RefreshRate,
			X: r.X, Y: r.Y, Scale: r.Scale, Transform: r.Transform,
			Disabled: r.Disabled, Focused: r.Focused,
		}
		if r.MirrorOf != "none" {
			m.MirrorOf = r.MirrorOf
		}
		for _, mode := range r.AvailableModes {
			m.Modes = append(m.Modes, strings.TrimSuffix(mode, "Hz"))
		}
		ms[i] = m
	}
	return ms, nil
}

// Internal reports whether the output is a laptop's own panel, which
// clamshell.lua turns off and on with the lid.
func Internal(name string) bool { return strings.HasPrefix(name, "eDP-") }

// MonitorOut is one output as monitors.lua describes it: the thing the
// Monitors tab edits and writes.
type MonitorOut struct {
	Name, Desc string
	Mode       string // "preferred" or "WxH@Hz"
	W, H       int    // the mode's pixels, for placing (last known, for preferred)
	X, Y       int
	Scale      float64
	Transform  int
	Disabled   bool
}

// Layout is every connected output, in hyprctl's order.
type Layout []MonitorOut

// LayoutOf is the live layout, as monitors.lua would write it.
func LayoutOf(ms []Monitor) Layout {
	l := make(Layout, len(ms))
	for i, m := range ms {
		l[i] = MonitorOut{
			Name: m.Name, Desc: m.Description,
			Mode: fmt.Sprintf("%dx%d@%.2f", m.Width, m.Height, m.Refresh),
			W:    m.Width, H: m.Height, X: m.X, Y: m.Y,
			Scale: m.Scale, Transform: m.Transform, Disabled: m.Disabled,
		}
	}
	return l
}

// Output is the rule's `output`: the internal panel by name, since every
// laptop calls it eDP-1; anything else by description, so the rule follows
// the display from port to port (as monitors.lua recommends).
func (o MonitorOut) Output() string {
	if Internal(o.Name) || o.Desc == "" {
		return o.Name
	}
	return "desc:" + o.Desc
}

var reMode = regexp.MustCompile(`^(\d+)x(\d+)`)

// SetMode changes the mode; "preferred" keeps the last known size, which is
// as good a guess as there is for placing the others around it.
func (o *MonitorOut) SetMode(mode string) {
	o.Mode = mode
	if m := reMode.FindStringSubmatch(mode); m != nil {
		o.W, _ = strconv.Atoi(m[1])
		o.H, _ = strconv.Atoi(m[2])
	}
}

// Logical is the output's size in the layout: its pixels over its scale,
// turned on its side for a quarter or three-quarter turn.
func (o MonitorOut) Logical() (int, int) {
	s := o.Scale
	if s <= 0 {
		s = 1
	}
	w, h := int(float64(o.W)/s+0.5), int(float64(o.H)/s+0.5)
	if o.Transform%2 == 1 {
		w, h = h, w
	}
	return w, h
}

// Place moves output i to the left of, right of, above or below output j,
// lined up with its top or left edge, then shifts everything so the layout
// starts at 0x0. It returns a new layout.
func Place(l Layout, i int, rel string, j int) Layout {
	l = append(Layout(nil), l...)
	iw, ih := l[i].Logical()
	jw, jh := l[j].Logical()
	x, y := l[j].X, l[j].Y
	switch rel {
	case "left":
		x -= iw
	case "right":
		x += jw
	case "above":
		y -= ih
	case "below":
		y += jh
	}
	l[i].X, l[i].Y = x, y

	minX, minY := 0, 0
	for _, o := range l {
		if !o.Disabled {
			minX, minY = min(minX, o.X), min(minY, o.Y)
		}
	}
	for k := range l {
		l[k].X -= minX
		l[k].Y -= minY
	}
	return l
}

// AnyEnabled reports whether at least one output stays on.
func (l Layout) AnyEnabled() bool {
	for _, o := range l {
		if !o.Disabled {
			return true
		}
	}
	return false
}

// MonitorsHeader starts every monitors.lua hashiru-tui writes, so it can tell
// its own from one written by hand.
const MonitorsHeader = `-- Written by hashiru-tui (Hardware → Monitors). It replaces Hashiru's
-- monitors.lua. Edit it by hand and hashiru-tui asks before writing over
-- it (and keeps a copy in monitors.lua.bak). ` + "`hyprctl monitors all`" + ` prints
-- the names, descriptions and modes.
`

// HasHeader reports whether hashiru-tui wrote the file.
func HasHeader(file []byte) bool { return bytes.HasPrefix(file, []byte(MonitorsHeader)) }

// Rule is one hl.monitor call in a monitors.lua.
type Rule struct {
	Output string // its output = "...": a name, "desc:...", or "" for the catch-all
	Line   string
}

var reRule = regexp.MustCompile(`^\s*hl\.monitor\(\{.*\boutput\s*=\s*"([^"]*)"`)

// ParseRules finds the one-line hl.monitor calls in a monitors.lua. Lines
// commented out aren't rules; multi-line calls aren't seen.
func ParseRules(file []byte) []Rule {
	var rules []Rule
	sc := bufio.NewScanner(bytes.NewReader(file))
	for sc.Scan() {
		line := sc.Text()
		if m := reRule.FindStringSubmatch(line); m != nil {
			rules = append(rules, Rule{Output: m[1], Line: strings.TrimSpace(line)})
		}
	}
	return rules
}

// Matches reports whether a rule's output picks out the monitor: its name,
// or desc: and a prefix of its description, the way Hyprland matches.
func Matches(output string, m Monitor) bool {
	if d, ok := strings.CutPrefix(output, "desc:"); ok {
		return d != "" && strings.HasPrefix(m.Description, d)
	}
	return output != "" && output == m.Name
}

// MonitorsLua writes the layout as a monitors.lua: one rule per connected
// output, then the old file's rules for displays not connected now (as they
// were, so undocking and changing the panel doesn't forget the dock), then
// Hashiru's catch-all so a display never seen before still lights.
func MonitorsLua(l Layout, old []Rule, connected []Monitor) []byte {
	var b strings.Builder
	b.WriteString(MonitorsHeader + "\n")
	for _, o := range l {
		b.WriteString(o.rule() + "\n")
	}

	var kept []string
	for _, r := range old {
		if r.Output == "" {
			continue // the catch-all, written below
		}
		live := false
		for _, m := range connected {
			if Matches(r.Output, m) {
				live = true
				break
			}
		}
		if !live {
			kept = append(kept, r.Line)
		}
	}
	if len(kept) > 0 {
		b.WriteString("\n-- Not connected when this was written; kept as they were.\n")
		b.WriteString(strings.Join(kept, "\n") + "\n")
	}

	b.WriteString("\n-- Anything not named above: preferred mode, placed left to right.\n")
	b.WriteString(`hl.monitor({ output = "", mode = "preferred", position = "auto", scale = 1 })` + "\n")
	return []byte(b.String())
}

// rule is the output's hl.monitor line. The internal panel is never
// written disabled: turning it off and on with the lid is clamshell.lua's.
func (o MonitorOut) rule() string {
	if o.Disabled && !Internal(o.Name) {
		return fmt.Sprintf("hl.monitor({ output = %q, disabled = true })", o.Output())
	}
	s := fmt.Sprintf("hl.monitor({ output = %q, mode = %q, position = \"%dx%d\", scale = %s",
		o.Output(), o.Mode, o.X, o.Y, strconv.FormatFloat(o.Scale, 'f', -1, 64))
	if o.Transform != 0 {
		s += fmt.Sprintf(", transform = %d", o.Transform)
	}
	return s + " })"
}
