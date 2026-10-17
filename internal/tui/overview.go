package tui

import (
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/whleucka/hashiru-tui/internal/system"
)

// overview is the home screen: the machine at a glance.
type overview struct{ e *env }

func newOverview(e *env) *overview { return &overview{e: e} }

func (o *overview) Name() string                              { return "Overview" }
func (o *overview) Update(tea.Msg) tea.Cmd                    { return nil }
func (o *overview) HandleKey(tea.KeyPressMsg) (tea.Cmd, bool) { return nil, false }
func (o *overview) Keys() []Key                               { return nil }
func (o *overview) Actions() []Action                         { return nil }

func (o *overview) View(width, height int) string {
	st := o.e.state
	var b strings.Builder
	row := func(k, v string) { b.WriteString(styleKey.Render(k) + v + "\n") }

	if si := st.Sys; si != nil {
		head := styleTitle.Render(orUnknown(si.Host))
		if si.Model != "" {
			head += "  " + si.Model
		}
		b.WriteString(head + styleMuted.Render("  up "+uptime(si.Uptime)) + "\n")
		row("kernel", orUnknown(si.Kernel))
		if si.CPU != "" {
			row("cpu", fmt.Sprintf("%s · %d threads", si.CPU, si.Threads))
		}
		if m := st.Machine; m != nil {
			if len(m.GPUs) > 0 {
				row("gpu", strings.Join(m.GPUs, " · "))
			}
			if m.Hyprland != "" {
				row("hyprland", m.Hyprland)
			}
			if len(m.Displays) > 0 {
				var ds []string
				for _, d := range m.Displays {
					ds = append(ds, d.Display())
				}
				row("displays", strings.Join(ds, " · "))
			}
		}
		if si.MemTotal > 0 {
			used := si.MemTotal - si.MemAvail
			mem := fmt.Sprintf("%s used of %s", system.HumanBytes(used), system.HumanBytes(si.MemTotal))
			if si.SwapTotal > 0 {
				mem += styleMuted.Render(fmt.Sprintf(" · swap %s of %s", system.HumanBytes(si.SwapTotal-si.SwapFree), system.HumanBytes(si.SwapTotal)))
			}
			row("memory", mem)
		}
		if si.DiskTotal > 0 {
			disk := fmt.Sprintf("%s free of %s", system.HumanBytes(si.DiskFree), system.HumanBytes(si.DiskTotal))
			if si.DiskFree*10 < si.DiskTotal {
				disk = styleWarn.Render(disk)
			}
			row("disk /", disk)
		}
		if bat := si.Battery; bat != nil {
			line := fmt.Sprintf("%d%% %s", bat.Percent, strings.ToLower(bat.Status))
			if bat.Percent <= 15 && bat.Status == "Discharging" {
				line = styleWarn.Render(line)
			}
			if bat.Health > 0 {
				line += styleMuted.Render(fmt.Sprintf(" · %.0f%% health", bat.Health*100))
			}
			row("battery", line)
		}
		if m := st.Machine; m != nil && m.Packages > 0 {
			row("installed", fmt.Sprintf("%d packages", m.Packages))
		}
		b.WriteString("\n")
	}

	b.WriteString(styleTitle.Render("Hashiru") + "\n")
	switch {
	case st.Status == nil && st.StatusBusy:
		row("status", styleMuted.Render("loading… (fetches upstream)"))
	case st.Status == nil && st.StatusErr != nil:
		row("status", styleFail.Render(st.StatusErr.Error()))
	case st.Status != nil:
		s := st.Status
		row("version", orUnknown(s.Version))
		row("updated", when(s.Updated))
		row("upstream", upstreamLine(s))
	}
	if st.Conn != nil {
		b.WriteString("\n" + styleTitle.Render("Connections") + "\n")
		row("network", networkLine(st.Conn))
		row("bluetooth", bluetoothLine(st.Conn))
		row("audio", audioLine(st.Conn))
	}
	b.WriteString("\n" + styleTitle.Render("Updates") + "\n")
	row("packages", pendingLine(st))
	row("arch news", newsLine(st))
	switch {
	case st.Report == nil && st.ReportBusy:
		row("health", styleMuted.Render("running doctor…"))
	case st.Report == nil && st.ReportErr != nil:
		row("health", styleFail.Render(st.ReportErr.Error()))
	case st.Report != nil:
		row("health", summaryLine(st.Report))
	}
	return b.String()
}

// uptime is "14d 13h", "5h 2m" or "12m".
func uptime(d time.Duration) string {
	switch {
	case d >= 24*time.Hour:
		return fmt.Sprintf("%dd %dh", int(d.Hours())/24, int(d.Hours())%24)
	case d >= time.Hour:
		return fmt.Sprintf("%dh %dm", int(d.Hours()), int(d.Minutes())%60)
	default:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
}
