package tui

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
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
}

// powerState is one reading of everything the tab shows.
type powerState struct {
	bats               []system.BatteryState
	ac, acKnown        bool
	platform           string
	tlp                system.TLPStatus
	tlpErr             error
	tlpMissing         bool
	idle               map[string]idleConf // "ac", "battery"
	canCharge, canIdle bool
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
	}
	return nil
}

func (p *power) probe() tea.Cmd {
	probe, src := p.e.probe, p.e.src
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), powerTimeout)
		defer cancel()
		st := powerState{
			bats: system.ReadBatteries(p.root),
			idle: map[string]idleConf{},
		}
		st.ac, st.acKnown = system.OnAC(p.root)
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

		help := src.Argv("help")
		if out, err := probe(ctx, help[0], help[1:]...); err == nil {
			st.canCharge = system.HasChargeCommand(out)
		}
		if script, err := os.ReadFile(p.idleScript()); err == nil {
			st.canIdle = system.IdleReloadable(script)
		}
		for _, s := range idleSources {
			stowed, override := p.idlePaths(s)
			ic := idleConf{}
			b, err := os.ReadFile(override)
			if err == nil && st.canIdle {
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

func (p *power) actions() []Action { return nil }

func (p *power) keys() []Key { return nil }

func (p *power) handleKey(tea.KeyPressMsg) (tea.Cmd, bool) { return nil, false }

// minutes is a timeout as people say it: 5 min, or 90s.
func minutes(secs int) string {
	if secs > 0 && secs%60 == 0 {
		return fmt.Sprintf("%d min", secs/60)
	}
	return fmt.Sprintf("%ds", secs)
}

// needs is the note beside a control this Hashiru is too old for.
const needs = "needs Hashiru v1.14.0 (hashiru update)"

func (p *power) view(width, height int) string {
	if !p.loaded {
		return styleMuted.Render("reading the battery…")
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
			if !st.canCharge {
				th += "  " + styleMuted.Render(needs+" to set them")
			}
			field("thresholds", th)
		}
	}
	if st.acKnown {
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
	if !st.canIdle {
		b.WriteString(styleMuted.Render(needs+" to change them") + "\n")
	}
	return b.String()
}
