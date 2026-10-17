package tui

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/whleucka/hashiru-tui/internal/system"
)

// power is the battery, the power profile, idle timeouts and charge
// thresholds. Reading is native; changing goes through tlp, hashiru charge
// and Hashiru's idle override.
type power struct {
	e      *env
	root   string // where /sys is read from
	home   string
	loaded bool
	st     powerState
	height int
	picker *choice
	ask    *ask
}

// powerState is one reading of everything the tab shows.
type powerState struct {
	bats        []system.BatteryState
	ac, acKnown bool
	charger     system.Charger
	charging    bool // a USB-C supply is online
	platform    string
	tlp         system.TLPStatus
	tlpErr      error
	tlpMissing  bool
	idle        map[string]idleConf // "ac", "battery"
}

// idleConf is one power source's hypridle timeouts and where they're from.
type idleConf struct {
	t        system.Idle
	override bool // ~/.config/hashiru/hypr's, not Hashiru's
	err      error
}

type powerMsg struct{ st powerState }

const powerTimeout = 5 * time.Second

var idleSources = []string{"ac", "battery"}

func newPower(e *env, root, home string) *power { return &power{e: e, root: root, home: home} }

func (p *power) name() string  { return "power" }
func (p *power) label() string { return "Power" }
func (p *power) warn() bool    { return false }

// idlePaths are Hashiru's stowed conf for a power source and the override
// that replaces it.
func (p *power) idlePaths(source string) (stowed, override string) {
	return filepath.Join(p.home, ".config/hypr/hypridle-"+source+".conf"),
		filepath.Join(system.HashiruConfigDir(p.home), "hypr", "hypridle-"+source+".conf")
}

func (p *power) idleScript() string {
	return filepath.Join(p.home, ".config/hypr/scripts/hypridle-battery")
}

func (p *power) update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case areaSizeMsg:
		p.height = msg.height
	case refreshMsg:
		return p.probe()
	case powerMsg:
		p.st, p.loaded = msg.st, true
	case powerDoneMsg:
		return tea.Batch(note(msg.text), p.probe())
	}
	return nil
}

func (p *power) probe() tea.Cmd {
	probe := p.e.probe
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), powerTimeout)
		defer cancel()
		st := powerState{
			bats: system.ReadBatteries(p.root),
			idle: map[string]idleConf{},
		}
		st.ac, st.acKnown = system.OnAC(p.root)
		st.charger, st.charging = system.ReadCharger(p.root)
		st.platform, _ = system.PlatformProfile(p.root)

		out, err := probe(ctx, "tlp-stat", system.TLPStatusArgs...)
		switch {
		case system.Missing(err):
			st.tlpMissing = true
		case err != nil:
			st.tlpErr = err
		default:
			st.tlp = system.ParseTLPStatus(out)
		}

		for _, s := range idleSources {
			stowed, override := p.idlePaths(s)
			ic := idleConf{}
			b, err := os.ReadFile(override)
			if err == nil {
				ic.override = true
			} else {
				b, err = os.ReadFile(stowed)
			}
			ic.t, ic.err = system.ParseIdle(b), err
			st.idle[s] = ic
		}
		return powerMsg{st}
	}
}

func (p *power) actions() []Action {
	return []Action{
		{ID: "hardware.power-profile", Title: "Switch the power profile (tlp)", Key: "p", Run: p.pickProfile},
		{ID: "hardware.idle", Title: "Change the idle timeouts", Key: "i", Run: p.editIdle},
		{ID: "hardware.idle-reset", Title: "Idle timeouts back to Hashiru's", Key: "I", Run: p.resetIdle},
		{ID: "hardware.charge", Title: "Set battery charge thresholds (hashiru charge)", Key: "c", Run: p.editCharge},
		{ID: "hardware.charge-off", Title: "Charge the battery to full again (hashiru charge off)", Key: "C", Run: p.chargeOff},
	}
}

// Typing while a prompt is open, so every key goes to it.
func (p *power) Typing() bool { return p.ask != nil }

func (p *power) keys() []Key {
	switch {
	case p.ask != nil:
		return []Key{{Keys: "enter", Desc: "next"}, {Keys: "esc", Desc: "cancel"}}
	case p.picker != nil:
		return p.picker.keys()
	}
	return []Key{
		{Keys: "p", Desc: "profile"},
		{Keys: "i/I", Desc: "idle / Hashiru's"},
		{Keys: "c/C", Desc: "thresholds / off"},
	}
}

func (p *power) handleKey(msg tea.KeyPressMsg) (tea.Cmd, bool) {
	switch {
	case p.ask != nil:
		cmd, done := p.ask.handleKey(msg)
		if done {
			p.ask = nil
		}
		return cmd, true
	case p.picker != nil:
		cmd, done := p.picker.handleKey(msg.String(), p.height-3)
		if done {
			p.picker = nil
		}
		return cmd, true
	}
	for _, a := range p.actions() {
		if msg.String() == a.Key {
			return a.Run(), true
		}
	}
	return nil, false
}

// pickProfile runs `sudo tlp <profile>`, waybar's cycle button's command,
// in the terminal so sudo can ask.
func (p *power) pickProfile() tea.Cmd {
	if p.st.tlpMissing {
		return note("TLP isn't installed")
	}
	opts := system.TLPProfiles
	sel := 0
	for i, o := range opts {
		if o == p.st.tlp.Profile {
			sel = i
		}
	}
	p.picker = &choice{
		title: "Power profile", about: "sudo tlp <profile>; TLP goes back to AC/battery defaults when the charger changes",
		opts: opts, sel: sel,
		pick: func(i int) tea.Cmd {
			return p.e.launch(Launch{Argv: []string{"sudo", "tlp", opts[i]}, Label: "tlp " + opts[i], Kind: Interactive, Refresh: true})
		},
	}
	return nil
}

// editIdle asks for the lock timeout on AC, then on battery, in minutes.
// The screen-off gap after locking stays what it was. Only a source whose
// timeout changed gets an override.
func (p *power) editIdle() tea.Cmd {
	var steps []askStep
	for _, s := range idleSources {
		name := map[string]string{"ac": "on AC", "battery": "on battery"}[s]
		steps = append(steps, askStep{
			prompt: "lock after, " + name + " (minutes): ",
			value:  fmt.Sprint(p.st.idle[s].t.Lock / 60),
			check:  checkMinutes,
		})
	}
	p.ask = newAsk(steps, func(v []string) tea.Cmd {
		var changed []string
		for i, s := range idleSources {
			was := p.st.idle[s].t
			mins, _ := strconv.Atoi(v[i])
			if mins*60 == was.Lock {
				continue
			}
			gap := max(was.Off-was.Lock, 0)
			if err := p.writeIdle(s, system.Idle{Lock: mins * 60, Off: mins*60 + gap}); err != nil {
				return func() tea.Msg { return noteMsg{text: "idle timeouts", err: err} }
			}
			changed = append(changed, s)
		}
		if len(changed) == 0 {
			return note("idle timeouts unchanged")
		}
		return p.reloadIdle("idle timeouts saved for " + strings.Join(changed, " and "))
	})
	return p.ask.focus()
}

func checkMinutes(v string) error {
	n, err := strconv.Atoi(strings.TrimSpace(v))
	if err != nil || n < 1 || n > 600 {
		return fmt.Errorf("whole minutes, 1 to 600")
	}
	return nil
}

// writeIdle writes the override for source: whichever file is in use now,
// with only its timeouts changed.
func (p *power) writeIdle(source string, t system.Idle) error {
	stowed, override := p.idlePaths(source)
	base, err := os.ReadFile(override)
	if err != nil {
		if base, err = os.ReadFile(stowed); err != nil {
			return err
		}
	}
	out, err := system.RewriteIdle(base, t)
	if err != nil {
		return err
	}
	return writeFile(override, out)
}

// resetIdle removes both overrides, so Hashiru's timeouts apply again.
func (p *power) resetIdle() tea.Cmd {
	removed := 0
	for _, s := range idleSources {
		_, override := p.idlePaths(s)
		if err := os.Remove(override); err == nil {
			removed++
		} else if !os.IsNotExist(err) {
			return func() tea.Msg { return noteMsg{text: "idle timeouts", err: err} }
		}
	}
	if removed == 0 {
		return note("idle timeouts are already Hashiru's")
	}
	return p.reloadIdle("idle timeouts back to Hashiru's")
}

// reloadIdle restarts hypridle on the right file. It's the user's own
// process, so it runs directly, like systemctl --user in Services.
func (p *power) reloadIdle(done string) tea.Cmd {
	probe, script := p.e.probe, p.idleScript()
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), powerTimeout)
		defer cancel()
		if _, err := probe(ctx, script, "reload"); err != nil {
			return noteMsg{text: done + ", but hypridle didn't reload", err: err}
		}
		return powerDoneMsg{done}
	}
}

type powerDoneMsg struct{ text string }

// editCharge asks for start and stop and runs hashiru charge, which needs
// sudo, in the terminal.
func (p *power) editCharge() tea.Cmd {
	var bat *system.BatteryState
	for i := range p.st.bats {
		if p.st.bats[i].Thresholds {
			bat = &p.st.bats[i]
			break
		}
	}
	if bat == nil {
		return note("no battery here takes charge thresholds")
	}
	pct := func(v string) error {
		if n, err := strconv.Atoi(strings.TrimSpace(v)); err != nil || n < 0 || n > 100 {
			return fmt.Errorf("a percent, 0 to 100")
		}
		return nil
	}
	p.ask = newAsk([]askStep{
		{prompt: "start charging below (%): ", value: fmt.Sprint(max(bat.Start, 0)), check: pct},
		{prompt: "stop charging at (%): ", value: fmt.Sprint(bat.Stop), check: pct},
	}, func(v []string) tea.Cmd {
		start, _ := strconv.Atoi(strings.TrimSpace(v[0]))
		stop, _ := strconv.Atoi(strings.TrimSpace(v[1]))
		if start >= stop {
			return note(fmt.Sprintf("%d/%d: start < stop, e.g. 75/80", start, stop))
		}
		return p.e.launch(Launch{
			Argv:  p.e.src.Argv("charge", strconv.Itoa(start), strconv.Itoa(stop)),
			Label: "hashiru charge", Kind: Interactive, Refresh: true,
		})
	})
	return p.ask.focus()
}

func (p *power) chargeOff() tea.Cmd {
	return p.e.launch(Launch{Argv: p.e.src.Argv("charge", "off"), Label: "hashiru charge off", Kind: Interactive, Refresh: true})
}

// minutes is a timeout as people say it: 5 min, or 90s.
func minutes(secs int) string {
	if secs > 0 && secs%60 == 0 {
		return fmt.Sprintf("%d min", secs/60)
	}
	return fmt.Sprintf("%ds", secs)
}

func (p *power) view(width, height int) string {
	switch {
	case !p.loaded:
		return styleMuted.Render("reading the battery…")
	case p.picker != nil:
		return p.picker.view(height)
	case p.ask != nil:
		return p.ask.view()
	}
	st := p.st
	var b strings.Builder
	head := func(s string) { b.WriteString(styleTitle.Render(s) + "\n") }
	field := func(k, v string) { b.WriteString(styleKey.Render(k) + v + "\n") }

	head("Battery")
	if len(st.bats) == 0 {
		b.WriteString(styleMuted.Render("no battery") + "\n")
	}
	for _, bat := range st.bats {
		line := fmt.Sprintf("%d%%  %s", bat.Capacity, strings.ToLower(bat.Status))
		if bat.Health > 0 {
			line += fmt.Sprintf(" · health %.0f%%", bat.Health*100)
			if bat.Cycles > 0 {
				line += fmt.Sprintf(" (%d cycles)", bat.Cycles)
			}
		}
		field(bat.Name, line)
		if bat.Thresholds {
			th := fmt.Sprintf("%d/%d", bat.Start, bat.Stop)
			if bat.Start <= 0 && bat.Stop >= 100 {
				th += styleMuted.Render(" (charges to full)")
			}
			field("thresholds", th)
		}
	}
	switch c := st.charger; {
	case st.charging && c.Watts > 0:
		line := fmt.Sprintf("%dW USB-C", c.Watts)
		if c.Kind != "" && c.Kind != "C" {
			line += " " + strings.ReplaceAll(c.Kind, "_", " ")
		}
		if c.Now > 0 {
			line += styleMuted.Render(fmt.Sprintf("  drawing %dW", c.Now))
		}
		field("charger", line)
	case st.charging:
		field("charger", "USB-C")
	case st.acKnown:
		field("AC", map[bool]string{true: "plugged in", false: "unplugged"}[st.ac])
	}

	b.WriteString("\n")
	head("Profile")
	switch {
	case st.tlpMissing:
		field("TLP", styleMuted.Render("not installed"))
	case st.tlpErr != nil:
		field("TLP", styleFail.Render("tlp-stat -s: "+st.tlpErr.Error()))
	default:
		line := orUnknown(st.tlp.Profile)
		if st.tlp.Source != "" {
			line += " on " + st.tlp.Source
		}
		if !st.tlp.Enabled {
			line += "  " + styleWarn.Render("(TLP isn't enabled)")
		}
		field("TLP", line+styleMuted.Render("  TLP "+st.tlp.Version))
	}
	if st.platform != "" {
		field("platform", st.platform+styleMuted.Render("  (ACPI, what the firmware is running)"))
	}

	b.WriteString("\n")
	head("Idle" + styleMuted.Render("  lock, then screen off"))
	for _, s := range idleSources {
		ic := st.idle[s]
		name := s
		if s == "ac" {
			name = "AC"
		}
		if ic.err != nil {
			field(name, styleFail.Render(ic.err.Error()))
			continue
		}
		from := styleMuted.Render("Hashiru's")
		if ic.override {
			from = styleOK.Render("yours")
		}
		off := ""
		if ic.t.Off > 0 {
			off = "  +" + minutes(ic.t.Off-ic.t.Lock)
		}
		field(name, cell(minutes(ic.t.Lock), 7)+cell(off, 8)+"  "+from)
	}
	return b.String()
}
