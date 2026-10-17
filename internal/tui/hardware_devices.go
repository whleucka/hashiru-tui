package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/whleucka/hashiru-tui/internal/system"
)

// devices is what's plugged in: removable drives, USB and Thunderbolt, in
// one list. Keys act on the selected row's kind.
type devices struct {
	e      *env
	root   string // where /sys is read from
	st     devState
	loaded bool
	sel    int
	offset int
	height int
	detail *devRow // the row whose details are showing
	// unlockThen is the LUKS partition being unlocked, to mount once the
	// list shows it unlocked.
	unlockThen string
}

// devState is one reading of everything the tab shows.
type devState struct {
	drives    []system.Drive
	drivesErr error
	usb       []system.USBDevice
	bolt      []system.BoltDevice
	security  string // the Thunderbolt domain's, "" with no controller
	boltErr   error
	noBolt    bool // boltctl isn't installed
}

type devKind int

const (
	kindDrive devKind = iota
	kindPart
	kindUSB
	kindBolt
)

// devRow is one selectable row: a drive, one of its partitions, a USB
// device or a Thunderbolt device.
type devRow struct {
	kind  devKind
	drive system.Drive
	part  system.Partition
	usb   system.USBDevice
	bolt  system.BoltDevice
}

type devicesMsg struct{ st devState }

const devicesTimeout = 5 * time.Second

func newDevices(e *env, root string) *devices { return &devices{e: e, root: root} }

func (d *devices) name() string  { return "devices" }
func (d *devices) label() string { return "Devices" }

// warn is a Thunderbolt device that's plugged in and waiting to be
// authorized: the dock that's connected and doing nothing.
func (d *devices) warn() bool {
	for _, b := range d.st.bolt {
		if b.NeedsAuth() {
			return true
		}
	}
	return false
}

func (d *devices) update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case areaSizeMsg:
		d.height = msg.height
	case refreshMsg:
		return d.probe()
	case devicesMsg:
		key := ""
		if r, ok := d.selected(); ok {
			key = r.key()
		}
		d.st, d.loaded = msg.st, true
		d.reselect(key)
		return d.mountUnlocked()
	case devDoneMsg:
		return tea.Batch(func() tea.Msg { return noteMsg{text: msg.text, err: msg.err} }, d.probe())
	}
	return nil
}

// key names the device a row is, so the cursor stays on it when a reload
// adds or takes away rows above it.
func (r devRow) key() string {
	switch r.kind {
	case kindDrive:
		return "drive " + r.drive.Path
	case kindPart:
		return "part " + r.part.Path
	case kindUSB:
		return "usb " + r.usb.ID
	}
	return "bolt " + r.bolt.UUID
}

func (d *devices) selected() (devRow, bool) {
	rows := d.rows()
	if d.sel < len(rows) {
		return rows[d.sel], true
	}
	return devRow{}, false
}

// reselect puts the cursor back on the device with key, or keeps its place
// when that's gone.
func (d *devices) reselect(key string) {
	rows := d.rows()
	for i, r := range rows {
		if r.key() == key {
			d.sel = i
			return
		}
	}
	d.sel = min(d.sel, max(len(rows)-1, 0))
}

func (d *devices) probe() tea.Cmd {
	probe, root := d.e.probe, d.root
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), devicesTimeout)
		defer cancel()
		st := devState{usb: system.USBDevices(root)}

		out, err := probe(ctx, "lsblk", system.DrivesArgs...)
		if err == nil {
			st.drives, err = system.ParseDrives(out)
		}
		st.drivesErr = err

		out, err = probe(ctx, "boltctl", system.BoltListArgs...)
		switch {
		case system.Missing(err):
			st.noBolt = true
		case err != nil:
			st.boltErr = err
		default:
			st.bolt = system.ParseBolt(out)
			if dom, err := probe(ctx, "boltctl", system.BoltDomainsArgs...); err == nil {
				st.security = system.BoltSecurity(dom)
			}
		}
		return devicesMsg{st}
	}
}

// rows are the drives (each followed by its partitions), the USB devices
// as a tree, then Thunderbolt.
func (d *devices) rows() []devRow {
	var rows []devRow
	for _, dr := range d.st.drives {
		rows = append(rows, devRow{kind: kindDrive, drive: dr})
		for _, p := range dr.Parts {
			rows = append(rows, devRow{kind: kindPart, drive: dr, part: p})
		}
	}
	for _, u := range d.st.usb {
		rows = append(rows, devRow{kind: kindUSB, usb: u})
	}
	for _, b := range d.st.bolt {
		rows = append(rows, devRow{kind: kindBolt, bolt: b})
	}
	return rows
}

// keys are the ones for the selected row: a partition's aren't a USB
// device's.
func (d *devices) keys() []Key {
	if d.detail != nil {
		return []Key{{Keys: "esc", Desc: "back to the list"}}
	}
	keys := []Key{{Keys: "j/k", Desc: "move"}, {Keys: "enter", Desc: "details"}}
	r, ok := d.selected()
	if !ok {
		return keys
	}
	for _, a := range d.acts() {
		if a.applies(r) {
			desc := a.desc
			if a.key == "m" && r.part.Locked() {
				desc = "unlock, mount"
			}
			keys = append(keys, Key{Keys: a.key, Desc: desc})
		}
	}
	return keys
}

func (d *devices) handleKey(msg tea.KeyPressMsg) (tea.Cmd, bool) {
	k := msg.String()
	if d.detail != nil {
		if k == "esc" || k == "h" || k == "left" {
			d.detail = nil
			return nil, true
		}
		return nil, false
	}
	rows := d.rows()
	if i, ok := moveKey(k, d.sel, len(rows), max(d.height-2, 1)); ok {
		d.sel = i
		return nil, true
	}
	switch k {
	case "enter", "l", "right":
		if d.sel < len(rows) {
			r := rows[d.sel]
			d.detail = &r
		}
		return nil, true
	}
	for _, a := range d.acts() {
		if k == a.key {
			return d.do(a), true
		}
	}
	return nil, false
}

// devLine is one line of the list: a row (row >= 0) or a section's
// heading or note (row -1).
type devLine struct {
	row  int
	text func(selected bool) string
}

func (d *devices) lines() []devLine {
	var lines []devLine
	fixed := func(s string) devLine { return devLine{row: -1, text: func(bool) string { return s }} }
	heading := func(title, note string) {
		if len(lines) > 0 {
			lines = append(lines, fixed(""))
		}
		lines = append(lines, fixed(styleTitle.Render(title)+styleMuted.Render(note)))
	}
	i := 0
	add := func(r devRow) {
		lines = append(lines, devLine{row: i, text: func(sel bool) string { return devRowText(r, sel) }})
		i++
	}
	rows := d.rows()
	next := func(k devKind) {
		for i < len(rows) && (rows[i].kind == k || k == kindDrive && rows[i].kind == kindPart) {
			add(rows[i])
		}
	}

	heading("Drives", "")
	switch {
	case d.st.drivesErr != nil:
		lines = append(lines, fixed("  "+styleFail.Render("lsblk: "+d.st.drivesErr.Error())))
	case len(d.st.drives) == 0:
		lines = append(lines, fixed(styleMuted.Render("  no removable drives")))
	}
	next(kindDrive)

	heading("USB", "")
	if len(d.st.usb) == 0 {
		lines = append(lines, fixed(styleMuted.Render("  no USB devices")))
	}
	next(kindUSB)

	note := ""
	if d.st.security != "" {
		note = "  security " + d.st.security
	}
	heading("Thunderbolt", note)
	switch {
	case d.st.noBolt:
		lines = append(lines, fixed(styleMuted.Render("  boltctl isn't installed (pacman -S bolt)")))
	case d.st.boltErr != nil:
		lines = append(lines, fixed("  "+styleFail.Render("boltctl list: "+d.st.boltErr.Error())))
	case len(d.st.bolt) == 0 && d.st.security == "":
		lines = append(lines, fixed(styleMuted.Render("  no Thunderbolt controller")))
	case len(d.st.bolt) == 0:
		lines = append(lines, fixed(styleMuted.Render("  no Thunderbolt devices")))
	}
	next(kindBolt)
	return lines
}

func devRowText(r devRow, sel bool) string {
	cursor := "  "
	if sel {
		cursor = styleTitle.Render("▸ ")
	}
	name := func(s string, w int) string {
		if sel {
			return styleSelected.Render(cell(s, w))
		}
		return cell(s, w)
	}
	switch r.kind {
	case kindDrive:
		return cursor + name(r.drive.Name, 10) + "  " + cell(r.drive.Model, 28) + "  " + cell(r.drive.Size, 7) + "  " + styleMuted.Render(r.drive.Tran)
	case kindPart:
		p := r.part
		label := p.Label
		if label == "" {
			label = styleMuted.Render("no label")
		}
		return cursor + "  " + name(p.Name, 8) + "  " + cell(label, 28) + "  " + cell(p.Size, 7) + "  " + cell(p.FSType, 11) + "  " + partState(p)
	case kindUSB:
		u := r.usb
		indent := strings.Repeat("  ", u.Depth)
		title := u.Name
		if title == "" {
			title = u.VendorProduct
		}
		if u.Hub {
			title = styleMuted.Render(title)
		}
		return cursor + name(indent+u.ID, 18) + "  " + cell(title, 36) + "  " + cell(u.SpeedText(), 12) + "  " + styleMuted.Render(u.Maker)
	}
	b := r.bolt
	stored := styleMuted.Render("not stored")
	if b.Stored {
		stored = "stored"
	}
	return cursor + name(b.Name, 30) + "  " + cell(b.Vendor, 10) + "  " + cell(b.Generation, 14) + "  " + boltStatus(b) + "  " + stored
}

// partState is where a partition is mounted, or why it isn't.
func partState(p system.Partition) string {
	switch {
	case p.Locked():
		return styleWarn.Render("locked")
	case p.Mount() != "":
		return styleOK.Render(p.Mount())
	case p.Encrypted():
		return styleMuted.Render("unlocked, not mounted")
	}
	return styleMuted.Render("not mounted")
}

func boltStatus(b system.BoltDevice) string {
	s := cell(b.Status, 13)
	switch {
	case b.NeedsAuth():
		return styleWarn.Render(s)
	case b.Connected():
		return styleOK.Render(s)
	}
	return styleMuted.Render(s)
}

func (d *devices) view(width, height int) string {
	switch {
	case !d.loaded:
		return styleMuted.Render("looking for devices…")
	case d.detail != nil:
		return d.detailView()
	}
	lines := d.lines()
	// Scroll by lines, keeping the selected row's on screen and, at the
	// top, its section's heading too.
	at := 0
	for i, l := range lines {
		if l.row == d.sel {
			at = i
		}
	}
	n := max(height, 1)
	if at < d.offset {
		d.offset = at
	}
	if at >= d.offset+n {
		d.offset = at - n + 1
	}
	if d.sel == 0 {
		d.offset = 0
	}
	var b strings.Builder
	for i := d.offset; i < len(lines) && i < d.offset+n; i++ {
		b.WriteString(lines[i].text(lines[i].row == d.sel && lines[i].row >= 0) + "\n")
	}
	return b.String()
}

func (d *devices) detailView() string {
	r := d.detail
	var b strings.Builder
	field := func(k, v string) {
		if v != "" {
			b.WriteString(styleKey.Render(k) + v + "\n")
		}
	}
	switch r.kind {
	case kindDrive:
		b.WriteString(styleTitle.Render(orUnknown(r.drive.Model)) + styleMuted.Render("  "+r.drive.Path) + "\n\n")
		field("size", r.drive.Size)
		field("connected", r.drive.Tran)
		field("partitions", fmt.Sprint(len(r.drive.Parts)))
		if r.drive.Mounted() {
			field("state", styleWarn.Render("mounted, not safe to pull"))
		} else {
			field("state", "nothing mounted")
		}
	case kindPart:
		p := r.part
		b.WriteString(styleTitle.Render(p.Path) + styleMuted.Render("  on "+r.drive.Path+" "+r.drive.Model) + "\n\n")
		field("label", p.Label)
		field("size", p.Size)
		field("filesystem", p.FSType)
		if p.Inner != nil {
			field("unlocked as", p.Inner.Path)
			field("inside", p.Inner.FSType)
			field("label", p.Inner.Label)
		}
		field("state", partState(p))
		if mounts := allMounts(p); len(mounts) > 1 {
			field("also at", strings.Join(mounts[1:], " "))
		}
	case kindUSB:
		u := r.usb
		b.WriteString(styleTitle.Render(orUnknown(u.Name)) + styleMuted.Render("  "+u.Maker) + "\n\n")
		field("port", u.ID)
		field("id", u.VendorProduct)
		if u.Speed > 0 {
			field("speed", fmt.Sprintf("%s (%d Mb/s)", u.SpeedText(), u.Speed))
		}
		if u.Hub {
			field("kind", "hub")
		}
	case kindBolt:
		t := r.bolt
		b.WriteString(styleTitle.Render(t.Name) + styleMuted.Render("  "+t.Vendor) + "\n\n")
		field("uuid", t.UUID)
		field("generation", t.Generation)
		field("status", boltStatus(t))
		field("speed", t.Speed)
		if t.Stored {
			field("stored", "yes: authorized whenever it's plugged in")
		} else {
			field("stored", "no")
		}
		if t.NeedsAuth() {
			b.WriteString("\n" + styleWarn.Render("plugged in but not authorized, so it does nothing yet") + "\n")
		}
	}
	return b.String()
}

func allMounts(p system.Partition) []string {
	if p.Inner != nil {
		return p.Inner.Mounts
	}
	return p.Mounts
}
