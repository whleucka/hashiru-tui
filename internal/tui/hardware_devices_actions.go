package tui

import (
	"context"
	"errors"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
)

// devAct is one of the Devices tab's actions: the rows it applies to, and
// what it does to one.
type devAct struct {
	id, title string
	key, desc string
	what      string // the rows it's for: "a mounted partition"
	applies   func(devRow) bool
	run       func(devRow) tea.Cmd
}

// devDoneMsg is a background udisks action finishing.
type devDoneMsg struct {
	text string
	err  error
}

// devActTimeout is long: unmounting a stick flushes what's still being
// written to it.
const devActTimeout = 2 * time.Minute

func isPart(r devRow) bool    { return r.kind == kindPart }
func isMounted(r devRow) bool { return isPart(r) && r.part.Mount() != "" }

func (d *devices) acts() []devAct {
	return []devAct{
		{id: "hardware.drive-mount", title: "Mount the selected partition (udisksctl)", key: "m", desc: "mount",
			what: "an unmounted partition", applies: func(r devRow) bool { return isPart(r) && !isMounted(r) }, run: d.mount},
		{id: "hardware.drive-unmount", title: "Unmount the selected partition (udisksctl)", key: "u", desc: "unmount",
			what: "a mounted partition", applies: isMounted, run: d.unmount},
		{id: "hardware.drive-eject", title: "Eject the selected drive: unmount and power it off", key: "e", desc: "eject",
			what: "a drive", applies: func(r devRow) bool { return r.kind == kindDrive || isPart(r) }, run: d.eject},
		{id: "hardware.drive-open", title: "Open the selected partition in Thunar", key: "o", desc: "open in Thunar",
			what: "a mounted partition", applies: isMounted, run: d.open},
		{id: "hardware.usb-reset", title: "Reset the selected USB device (usb --reset)", key: "R", desc: "reset",
			what: "a USB device", applies: func(r devRow) bool { return r.kind == kindUSB }, run: d.reset},
		{id: "hardware.bolt-authorize", title: "Authorize the selected Thunderbolt device, this once (boltctl)", key: "a", desc: "authorize",
			what: "a Thunderbolt device waiting to be authorized", applies: func(r devRow) bool { return r.kind == kindBolt && r.bolt.NeedsAuth() },
			run: d.bolt("authorize")},
		{id: "hardware.bolt-store", title: "Trust the selected Thunderbolt device from now on (boltctl enroll)", key: "s", desc: "store",
			what: "a connected Thunderbolt device that isn't stored", applies: func(r devRow) bool { return r.kind == kindBolt && r.bolt.Connected() && !r.bolt.Stored },
			run: d.bolt("enroll")},
		{id: "hardware.bolt-forget", title: "Forget the selected Thunderbolt device (boltctl forget)", key: "f", desc: "forget",
			what: "a stored Thunderbolt device", applies: func(r devRow) bool { return r.kind == kindBolt && r.bolt.Stored },
			run: d.bolt("forget")},
	}
}

// actions are for the palette: each acts on the selected row, or says what
// it's for.
func (d *devices) actions() []Action {
	var acts []Action
	for _, a := range d.acts() {
		acts = append(acts, Action{ID: a.id, Title: a.title, Key: a.key, Run: func() tea.Cmd { return d.do(a) }})
	}
	return acts
}

func (d *devices) do(a devAct) tea.Cmd {
	r, ok := d.selected()
	if !ok || !a.applies(r) {
		return note(a.key + " " + a.desc + " is for " + a.what)
	}
	d.unlockThen = ""
	return a.run(r)
}

// mount mounts a partition, or the filesystem inside an unlocked LUKS one.
// A locked one is unlocked first, in the terminal so its passphrase can be
// asked, and mounted when the list shows it unlocked.
func (d *devices) mount(r devRow) tea.Cmd {
	p := r.part
	if p.Locked() {
		d.unlockThen = p.Path
		return d.e.launch(Launch{Argv: []string{"udisksctl", "unlock", "-b", p.Path}, Label: "unlock " + p.Name, Kind: Interactive, Refresh: true})
	}
	dev := p.Path
	if p.Inner != nil {
		dev = p.Inner.Path
	}
	return d.udisks("mounted "+p.Name, devStep{"mount " + p.Name, udisksctl("mount", dev)})
}

// mountUnlocked mounts what was just unlocked, once the list shows it.
func (d *devices) mountUnlocked() tea.Cmd {
	if d.unlockThen == "" {
		return nil
	}
	for _, r := range d.rows() {
		if r.kind != kindPart || r.part.Path != d.unlockThen {
			continue
		}
		if r.part.Locked() {
			return nil // still asking, or it was cancelled
		}
		d.unlockThen = ""
		if r.part.Mount() != "" {
			return nil
		}
		return d.mount(r)
	}
	d.unlockThen = "" // pulled out
	return nil
}

func (d *devices) unmount(r devRow) tea.Cmd {
	p := r.part
	dev := p.Path
	if p.Inner != nil {
		dev = p.Inner.Path
	}
	return d.udisks("unmounted "+p.Name, devStep{"unmount " + p.Name, udisksctl("unmount", dev)})
}

// eject makes the drive safe to pull: unmount everything on it, lock what's
// unlocked, then power it off. It stops at the first step that fails.
func (d *devices) eject(r devRow) tea.Cmd {
	dr := r.drive
	var steps []devStep
	for _, p := range dr.Parts {
		switch {
		case p.Inner != nil:
			if p.Inner.Mount() != "" {
				steps = append(steps, devStep{"unmount " + p.Name, udisksctl("unmount", p.Inner.Path)})
			}
			steps = append(steps, devStep{"lock " + p.Name, udisksctl("lock", p.Path)})
		case p.Mount() != "":
			steps = append(steps, devStep{"unmount " + p.Name, udisksctl("unmount", p.Path)})
		}
	}
	steps = append(steps, devStep{"power off " + dr.Name, udisksctl("power-off", dr.Path)})
	return d.udisks("ejected "+dr.Name+": safe to pull", steps...)
}

func (d *devices) open(r devRow) tea.Cmd {
	return d.e.launch(Launch{Argv: []string{"thunar", r.part.Mount()}, Label: "Thunar", Kind: GUI})
}

// reset is Hashiru's usb script, which asks sudo itself.
func (d *devices) reset(r devRow) tea.Cmd {
	return d.e.launch(Launch{Argv: []string{"usb", "--reset", r.usb.ID}, Label: "reset " + r.usb.ID, Kind: Interactive, Refresh: true})
}

// bolt runs a boltctl verb on the device; bolt asks polkit, in the terminal.
func (d *devices) bolt(verb string) func(devRow) tea.Cmd {
	return func(r devRow) tea.Cmd {
		return d.e.launch(Launch{Argv: []string{"boltctl", verb, r.bolt.UUID}, Label: "boltctl " + verb + " " + r.bolt.Name, Kind: Interactive, Refresh: true})
	}
}

// devStep is one udisksctl call, and what to call it if it fails.
type devStep struct {
	what string
	argv []string
}

// udisksctl never prompts: polkit lets the active session do these, and a
// refusal is better as an error than as a prompt nobody sees.
func udisksctl(verb, dev string) []string {
	return []string{"udisksctl", verb, "--no-user-interaction", "-b", dev}
}

// udisks runs steps in the background, in order, the result in the footer.
func (d *devices) udisks(done string, steps ...devStep) tea.Cmd {
	probe := d.e.probe
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), devActTimeout)
		defer cancel()
		for _, s := range steps {
			if _, err := probe(ctx, s.argv[0], s.argv[1:]...); err != nil {
				if strings.Contains(err.Error(), "busy") {
					err = errors.New("busy: something still has files open on it")
				}
				return devDoneMsg{text: s.what, err: err}
			}
		}
		return devDoneMsg{text: done}
	}
}
