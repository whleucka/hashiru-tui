package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/whleucka/hashiru-tui/internal/system"
)

// firmware is fwupd's view of the machine: what can be flashed, and what's
// waiting to be. Updating is fwupdmgr's own job, prompts and all.
type firmware struct {
	e      *env
	fw     system.Firmware
	loaded bool
	sel    int
	offset int
	height int
	detail *fwRow // the row whose details are showing
}

// fwRow is a pending update or a device without one.
type fwRow struct {
	dev system.FirmwareDevice
	up  *system.FirmwareUpdate
}

type firmwareMsg struct{ fw system.Firmware }

const firmwareTimeout = 20 * time.Second

func newFirmware(e *env) *firmware { return &firmware{e: e} }

func (f *firmware) name() string { return "firmware" }

func (f *firmware) label() string {
	if len(f.fw.Updates) > 0 {
		return fmt.Sprintf("Firmware (%d)", len(f.fw.Updates))
	}
	return "Firmware"
}

func (f *firmware) warn() bool { return len(f.fw.Updates) > 0 }

func (f *firmware) update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case areaSizeMsg:
		f.height = msg.height
	case refreshMsg:
		probe := f.e.probe
		return func() tea.Msg {
			ctx, cancel := context.WithTimeout(context.Background(), firmwareTimeout)
			defer cancel()
			return firmwareMsg{system.ReadFirmware(ctx, probe)}
		}
	case firmwareMsg:
		f.fw, f.loaded = msg.fw, true
		f.sel = min(f.sel, max(len(f.rows())-1, 0))
	}
	return nil
}

// rows are the pending updates, then every other device in fwupd's order.
func (f *firmware) rows() []fwRow {
	var rows []fwRow
	pending := map[string]bool{}
	for i := range f.fw.Updates {
		u := &f.fw.Updates[i]
		rows = append(rows, fwRow{dev: u.Device, up: u})
		pending[u.Device.ID] = true
	}
	for _, d := range f.fw.Devices {
		if !pending[d.ID] {
			rows = append(rows, fwRow{dev: d})
		}
	}
	return rows
}

func (f *firmware) actions() []Action {
	return []Action{
		{ID: "hardware.firmware-refresh", Title: "Fetch firmware metadata (fwupdmgr refresh)", Key: "F", Run: f.refresh},
		{ID: "hardware.firmware-update", Title: "Update firmware (fwupdmgr update)", Key: "u", Run: f.install},
	}
}

// refresh fetches new metadata from the LVFS. fwupd asks polkit itself, so
// no sudo; Interactive so its prompt and output show.
func (f *firmware) refresh() tea.Cmd {
	return f.e.launch(Launch{Argv: []string{"fwupdmgr", "refresh"}, Label: "fwupdmgr refresh", Kind: Interactive, Refresh: true})
}

// install runs fwupdmgr update, which asks before each device and about
// rebooting.
func (f *firmware) install() tea.Cmd {
	return f.e.launch(Launch{Argv: []string{"fwupdmgr", "update"}, Label: "fwupdmgr update", Kind: Interactive, Refresh: true})
}

func (f *firmware) keys() []Key {
	if f.detail != nil {
		return []Key{{Keys: "esc", Desc: "back to the list"}}
	}
	return []Key{
		{Keys: "j/k", Desc: "move"},
		{Keys: "enter", Desc: "details"},
		{Keys: "u", Desc: "update"},
		{Keys: "F", Desc: "fetch metadata"},
	}
}

func (f *firmware) handleKey(msg tea.KeyPressMsg) (tea.Cmd, bool) {
	k := msg.String()
	if f.detail != nil {
		if k == "esc" || k == "h" || k == "left" {
			f.detail = nil
			return nil, true
		}
		return nil, false
	}
	rows := f.rows()
	if i, ok := moveKey(k, f.sel, len(rows), f.listRows()); ok {
		f.sel = i
		return nil, true
	}
	switch k {
	case "enter", "l", "right":
		if f.sel < len(rows) {
			r := rows[f.sel]
			f.detail = &r
		}
		return nil, true
	}
	for _, a := range f.actions() {
		if k == a.Key {
			return a.Run(), true
		}
	}
	return nil, false
}

// listRows is how many rows fit under the summary line.
func (f *firmware) listRows() int { return max(f.height-3, 1) }

func (f *firmware) view(width, height int) string {
	switch {
	case f.fw.Missing:
		return styleMuted.Render("fwupdmgr isn't installed (pacman -S fwupd)")
	case f.fw.Err != nil:
		return styleFail.Render("fwupdmgr " + strings.Join(system.FirmwareDevicesArgs, " ") + ": " + f.fw.Err.Error())
	case !f.loaded:
		return styleMuted.Render("asking fwupd…")
	case f.detail != nil:
		return f.detailView()
	}

	var b strings.Builder
	b.WriteString(f.summary() + "\n\n")
	rows := f.rows()
	n := max(height-2, 1)
	if f.sel < f.offset {
		f.offset = f.sel
	}
	if f.sel >= f.offset+n {
		f.offset = f.sel - n + 1
	}
	for i := f.offset; i < len(rows) && i < f.offset+n; i++ {
		r := rows[i]
		cursor, name := "  ", cell(r.dev.Name, 34)
		if i == f.sel {
			cursor, name = styleTitle.Render("▸ "), styleSelected.Render(name)
		}
		b.WriteString(cursor + fwMark(r) + " " + name + "  " + fwVersion(r) + "  " + styleMuted.Render(r.dev.Vendor) + "\n")
	}
	return b.String()
}

func (f *firmware) summary() string {
	updatable := 0
	for _, d := range f.fw.Devices {
		if d.Updatable {
			updatable++
		}
	}
	line := styleMuted.Render(fmt.Sprintf("%d devices, %d updatable · ", len(f.fw.Devices), updatable))
	switch n := len(f.fw.Updates); {
	case f.fw.UpdatesErr != nil:
		line += styleFail.Render("updates: " + f.fw.UpdatesErr.Error())
	case n == 0:
		line += styleOK.Render("up to date") + styleMuted.Render(" (with the metadata fwupd has; F fetches new)")
	case n == 1:
		line += styleWarn.Render("1 update") + styleMuted.Render(" · u to install")
	default:
		line += styleWarn.Render(fmt.Sprintf("%d updates", n)) + styleMuted.Render(" · u to install")
	}
	return line
}

// fwMark is ↑ for a pending update, ● for a device fwupd can flash.
func fwMark(r fwRow) string {
	switch {
	case r.up != nil:
		return styleWarn.Render("↑")
	case r.dev.Updatable:
		return styleOK.Render("●")
	}
	return styleMuted.Render("·")
}

func fwVersion(r fwRow) string {
	if r.up != nil {
		return styleWarn.Render(cell(r.up.From()+" → "+r.up.To(), 24))
	}
	return cell(r.dev.Version, 24)
}

func (f *firmware) detailView() string {
	r := f.detail
	var b strings.Builder
	b.WriteString(styleTitle.Render(r.dev.Name))
	if r.dev.Vendor != "" {
		b.WriteString(styleMuted.Render("  " + r.dev.Vendor))
	}
	b.WriteString("\n")
	if r.dev.Summary != "" {
		b.WriteString(styleMuted.Render(r.dev.Summary) + "\n")
	}
	b.WriteString("\n")
	field := func(k, v string) { b.WriteString(styleKey.Render(k) + v + "\n") }
	if r.up != nil {
		field("version", styleWarn.Render(r.up.From()+" → "+r.up.To()))
	} else {
		field("version", orUnknown(r.dev.Version))
	}
	var notes []string
	if r.dev.NeedsReboot {
		notes = append(notes, "needs a reboot")
	}
	if r.dev.RequireAC {
		notes = append(notes, "only on AC")
	}
	if !r.dev.Updatable {
		notes = append(notes, "fwupd can't update it")
	}
	if len(notes) > 0 {
		field("update", strings.Join(notes, " · "))
	}
	field("flags", styleMuted.Render(strings.Join(r.dev.Flags, " ")))
	if r.up == nil {
		return b.String()
	}
	for _, rel := range r.up.Releases {
		b.WriteString("\n" + styleTitle.Render(rel.Version) + styleMuted.Render("  "+rel.Summary) + "\n")
		if notes := rel.Notes(); notes != "" {
			b.WriteString(notes + "\n")
		}
	}
	return b.String()
}
