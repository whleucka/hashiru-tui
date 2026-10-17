package tui

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

// devProbe answers lsblk and boltctl from fixtures: the hand-made sticks
// and this machine's dock, docked.
func devProbe(t *testing.T) probe {
	t.Helper()
	read := func(name string) string {
		b, err := os.ReadFile("../system/testdata/" + name)
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}
	return probe{
		"lsblk":           read("lsblk-sticks.json"),
		"boltctl list":    read("bolt-list-docked.txt"),
		"boltctl domains": read("bolt-domains.txt"),
	}
}

func devicesModel(t *testing.T, p probe) *Model {
	t.Helper()
	m, _ := hardwareModel(t, p)
	if err := m.openTab(m.area(), "devices"); err != nil {
		t.Fatal(err)
	}
	return m
}

func TestHardwareDevices(t *testing.T) {
	m := devicesModel(t, devProbe(t))
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 80})
	out := ansi.Strip(m.View().Content)
	for _, want := range []string{
		"Monitors", "Power", "Devices", "Firmware",
		"Drives", "sda", "DataTraveler 3.0", "28.9G", "KINGSTON", "/run/media/whleucka/KINGSTON",
		"sdb1", "unlocked, not mounted", "WHOLE",
		"USB", "4-2", "USB 3 · 5G", "5-1.4.2.1", "X50 RGB Mechanical Keyboard",
		"Thunderbolt", "security iommu+user", "ThinkPad Thunderbolt 4 Dock", "authorized", "stored",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in\n%s", want, out)
		}
	}
	// Devices sits between Power and Firmware.
	if !(strings.Index(out, "Power") < strings.Index(out, "Devices") && strings.Index(out, "Devices") < strings.Index(out, "Firmware")) {
		t.Errorf("tab order:\n%s", out)
	}
	// Sections in order, partitions under their drive.
	if !(strings.Index(out, "Drives") < strings.Index(out, "sda1") && strings.Index(out, "sda1") < strings.Index(out, "4-2") &&
		strings.Index(out, "4-2") < strings.Index(out, "ThinkPad")) {
		t.Errorf("section order:\n%s", out)
	}
	if m.area().(*hardware).tabs[2].warn() {
		t.Error("an authorized dock isn't a warning")
	}
}

func TestHardwareDevicesWarnsOnAnUnauthorizedDock(t *testing.T) {
	p := devProbe(t)
	p["boltctl list"] = " o Some Dock\n   |- name:          Some Dock\n   |- uuid:          abc\n   `- status:        connected\n"
	m := devicesModel(t, p)
	if !m.area().(*hardware).tabs[2].warn() {
		t.Error("a connected, unauthorized dock should warn")
	}
	if out := ansi.Strip(m.View().Content); !strings.Contains(out, "Some Dock") || !strings.Contains(out, "not stored") {
		t.Errorf("waiting dock:\n%s", out)
	}
}

func TestHardwareDevicesWithoutBolt(t *testing.T) {
	p := devProbe(t)
	delete(p, "boltctl list")
	delete(p, "boltctl domains")
	m := devicesModel(t, p)
	out := ansi.Strip(m.View().Content)
	if !strings.Contains(out, "boltctl isn't installed") || !strings.Contains(out, "DataTraveler") {
		t.Errorf("no bolt: the section says so, the rest works:\n%s", out)
	}

	p["boltctl list"], p["boltctl domains"] = "", ""
	m = devicesModel(t, p)
	if out := ansi.Strip(m.View().Content); !strings.Contains(out, "no Thunderbolt controller") {
		t.Errorf("no controller:\n%s", out)
	}

	p["lsblk"] = "not json"
	m = devicesModel(t, p)
	if out := ansi.Strip(m.View().Content); !strings.Contains(out, "lsblk:") || !strings.Contains(out, "4-2") {
		t.Errorf("lsblk failing:\n%s", out)
	}
}

func TestHardwareDevicesDetails(t *testing.T) {
	m := devicesModel(t, devProbe(t))
	press(m, "j") // sda1
	press(m, "enter")
	out := ansi.Strip(m.View().Content)
	for _, want := range []string{"/dev/sda1", "on /dev/sda DataTraveler 3.0", "KINGSTON", "vfat", "/run/media/whleucka/KINGSTON"} {
		if !strings.Contains(out, want) {
			t.Errorf("partition details missing %q in\n%s", want, out)
		}
	}
	press(m, "esc")

	// The unlocked LUKS partition names its mapper device.
	press(m, "j")
	press(m, "j")
	press(m, "enter")
	if out := ansi.Strip(m.View().Content); !strings.Contains(out, "/dev/mapper/luks-1c2d") || !strings.Contains(out, "secret") {
		t.Errorf("luks details:\n%s", out)
	}
	press(m, "esc")

	// The last row is the dock.
	for range 40 {
		press(m, "j")
	}
	press(m, "enter")
	out = ansi.Strip(m.View().Content)
	for _, want := range []string{"ThinkPad Thunderbolt 4 Dock", "36a78780-0022-688d-ffff-ffffffffffff", "USB4", "40 Gb/s", "authorized whenever"} {
		if !strings.Contains(out, want) {
			t.Errorf("dock details missing %q in\n%s", want, out)
		}
	}
}

func TestHardwareDevicesScrollsToTheSelection(t *testing.T) {
	m := devicesModel(t, devProbe(t))
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 16})
	for range 40 {
		press(m, "j")
	}
	if out := ansi.Strip(m.View().Content); !strings.Contains(out, "ThinkPad Thunderbolt 4 Dock") {
		t.Errorf("the last row is off screen:\n%s", out)
	}
	for range 40 {
		press(m, "k")
	}
	if out := ansi.Strip(m.View().Content); !strings.Contains(out, "Drives") {
		t.Errorf("back at the top, the heading shows:\n%s", out)
	}
}

func TestHardwareDevicesDeepLink(t *testing.T) {
	m := New(Options{Source: fakeSource{}, Probe: devProbe(t).run})
	if err := m.Start("hardware.devices", ""); err != nil {
		t.Fatal(err)
	}
	load(t, m)
	if out := ansi.Strip(m.View().Content); !strings.Contains(out, "DataTraveler") {
		t.Errorf("hardware.devices didn't open Devices:\n%s", out)
	}
}

// devActModel is devicesModel recording what it runs: udisksctl in the
// background (failing any call containing fail), the rest launched.
func devActModel(t *testing.T, p probe, fail string) (m *Model, ran, launched, opened *[]string) {
	t.Helper()
	ran, launched, opened = &[]string{}, &[]string{}, &[]string{}
	m = New(Options{
		Source: fakeSource{},
		Probe: func(ctx context.Context, name string, args ...string) ([]byte, error) {
			if name != "udisksctl" {
				return p.run(ctx, name, args...)
			}
			line := strings.Join(append([]string{name}, args...), " ")
			*ran = append(*ran, line)
			if fail != "" && strings.Contains(line, fail) {
				return nil, errors.New("udisksctl: exit status 1: Error unmounting: target is busy")
			}
			return nil, nil
		},
		RunInPane: func(_ context.Context, argv []string, _ string, _, _ bool) error {
			*launched = append(*launched, strings.Join(argv, " "))
			return nil
		},
		Detach: func(argv []string) error { *opened = append(*opened, strings.Join(argv, " ")); return nil },
	})
	load(t, m)
	goTo(t, m, "Hardware")
	if err := m.openTab(m.area(), "devices"); err != nil {
		t.Fatal(err)
	}
	return m, ran, launched, opened
}

func footer(m *Model) string {
	lines := strings.Split(ansi.Strip(m.View().Content), "\n")
	return lines[len(lines)-1]
}

func TestHardwareDevicesDriveActions(t *testing.T) {
	m, ran, _, opened := devActModel(t, devProbe(t), "")
	// sda: the drive, then sda1, mounted.
	if f := footer(m); !strings.Contains(f, "e eject") || strings.Contains(f, "m mount") {
		t.Errorf("drive row keys: %s", f)
	}
	press(m, "j")
	if f := footer(m); !strings.Contains(f, "u unmount") || !strings.Contains(f, "o open in Thunar") || strings.Contains(f, "R reset") {
		t.Errorf("mounted partition keys: %s", f)
	}
	press(m, "u", "o", "m")
	want := []string{"udisksctl unmount --no-user-interaction -b /dev/sda1"}
	if strings.Join(*ran, "|") != strings.Join(want, "|") {
		t.Errorf("ran %v, want %v", *ran, want)
	}
	if strings.Join(*opened, "|") != "thunar /run/media/whleucka/KINGSTON" {
		t.Errorf("opened %v", *opened)
	}
	if !strings.Contains(m.note, "m mount is for an unmounted partition") {
		t.Errorf("m on a mounted partition: %s", m.note)
	}

	// The unlocked LUKS partition mounts its mapper device.
	*ran = nil
	press(m, "j", "j", "m")
	if strings.Join(*ran, "|") != "udisksctl mount --no-user-interaction -b /dev/mapper/luks-1c2d" {
		t.Errorf("luks mount ran %v", *ran)
	}
}

func TestHardwareDevicesEject(t *testing.T) {
	// sdb's LUKS is unlocked, nothing mounted: lock, then power off. Eject
	// works from a partition row too.
	m, ran, _, _ := devActModel(t, devProbe(t), "")
	press(m, "j", "j", "j", "e")
	want := []string{
		"udisksctl lock --no-user-interaction -b /dev/sdb1",
		"udisksctl power-off --no-user-interaction -b /dev/sdb",
	}
	if strings.Join(*ran, "|") != strings.Join(want, "|") {
		t.Errorf("ran %v, want %v", *ran, want)
	}
	if m.note != "ejected sdb: safe to pull" {
		t.Errorf("note: %s", m.note)
	}

	// A busy mount stops it, says so, and leaves the disk powered.
	m, ran, _, _ = devActModel(t, devProbe(t), "unmount")
	press(m, "e")
	if len(*ran) != 1 || !strings.Contains((*ran)[0], "unmount") {
		t.Errorf("ran %v, want only the unmount", *ran)
	}
	if !strings.Contains(m.note, "unmount sda1: busy") || !m.noteErr {
		t.Errorf("note: %s", m.note)
	}
}

func TestHardwareDevicesUnlockThenMount(t *testing.T) {
	locked := `{"blockdevices":[{"name":"sdb","path":"/dev/sdb","size":"7.5G","rm":true,"hotplug":true,"tran":"usb","model":"Stick","type":"disk",
		"children":[{"name":"sdb1","path":"/dev/sdb1","size":"7.5G","fstype":"crypto_LUKS","mountpoints":[null],"type":"part"}]}]}`
	unlocked := strings.Replace(locked, `"type":"part"}`, `"type":"part","children":[{"name":"luks-9","path":"/dev/mapper/luks-9","size":"7.5G","fstype":"ext4","mountpoints":[null],"type":"crypt"}]}`, 1)
	p := devProbe(t)
	p["lsblk"] = locked
	m, ran, launched, _ := devActModel(t, p, "")
	press(m, "j")
	if f := footer(m); !strings.Contains(f, "m unlock, mount") {
		t.Errorf("locked partition keys: %s", f)
	}
	press(m, "m")
	if strings.Join(*launched, "|") != "udisksctl unlock -b /dev/sdb1" || len(*ran) != 0 {
		t.Errorf("launched %v, ran %v", *launched, *ran)
	}
	// Still locked on the next look (the passphrase is being typed): wait.
	press(m, "r")
	if len(*ran) != 0 {
		t.Errorf("mounted while locked: %v", *ran)
	}
	p["lsblk"] = unlocked
	press(m, "r")
	if strings.Join(*ran, "|") != "udisksctl mount --no-user-interaction -b /dev/mapper/luks-9" {
		t.Errorf("after unlock ran %v", *ran)
	}
	// Only once.
	press(m, "r")
	if len(*ran) != 1 {
		t.Errorf("ran %v", *ran)
	}
}

func TestHardwareDevicesUSBAndBoltActions(t *testing.T) {
	p := devProbe(t)
	p["boltctl list"] = " o Some Dock\n   |- name:          Some Dock\n   |- uuid:          abc\n   `- status:        connected\n"
	m, _, launched, _ := devActModel(t, p, "")
	for range 6 {
		press(m, "j") // past sda, sda1, sdb, sdb1, sdc and its filesystem: the first USB device
	}
	if f := footer(m); !strings.Contains(f, "R reset") || strings.Contains(f, "e eject") {
		t.Errorf("usb row keys: %s", f)
	}
	press(m, "R", "e")
	for range 40 {
		press(m, "j") // the dock
	}
	if f := footer(m); !strings.Contains(f, "a authorize") || !strings.Contains(f, "s store") || strings.Contains(f, "f forget") {
		t.Errorf("waiting dock keys: %s", f)
	}
	press(m, "a", "s", "f")
	want := []string{"usb --reset 1-1", "boltctl authorize abc", "boltctl enroll abc"}
	if strings.Join(*launched, "|") != strings.Join(want, "|") {
		t.Errorf("launched %v, want %v", *launched, want)
	}
	if !strings.Contains(m.note, "f forget is for a stored Thunderbolt device") {
		t.Errorf("f on an unstored dock: %s", m.note)
	}
}

func TestHardwareDevicesPaletteActions(t *testing.T) {
	m, ran, _, _ := devActModel(t, devProbe(t), "")
	ids := map[string]Action{}
	for _, a := range m.Actions() {
		ids[a.ID] = a
	}
	for _, id := range []string{"hardware.drive-mount", "hardware.drive-unmount", "hardware.drive-eject", "hardware.drive-open",
		"hardware.usb-reset", "hardware.bolt-authorize", "hardware.bolt-store", "hardware.bolt-forget"} {
		if _, ok := ids[id]; !ok {
			t.Errorf("no action %s", id)
		}
	}
	drain(m, ids["hardware.drive-eject"].Run())
	if len(*ran) == 0 || !strings.Contains((*ran)[len(*ran)-1], "power-off --no-user-interaction -b /dev/sda") {
		t.Errorf("palette eject ran %v", *ran)
	}
}

func TestHardwareDevicesKeepsTheCursorOnItsDevice(t *testing.T) {
	p := devProbe(t)
	sticks := p["lsblk"]
	p["lsblk"] = `{"blockdevices":[]}`
	m, _, _, _ := devActModel(t, p, "")
	press(m, "j", "enter") // the second USB device
	want := ansi.Strip(m.View().Content)
	press(m, "esc")
	// Sticks plugged in add rows above it; the cursor stays on it.
	p["lsblk"] = sticks
	press(m, "r", "enter")
	if got := ansi.Strip(m.View().Content); got != want {
		t.Errorf("cursor moved:\n%s\nwant\n%s", got, want)
	}
}

// A stick plugged in shows up on the next tick, without r; not while the
// details are showing.
func TestHardwareDevicesTickFindsANewStick(t *testing.T) {
	p := devProbe(t)
	sticks := p["lsblk"]
	p["lsblk"] = `{"blockdevices":[]}`
	m, _, _, _ := devActModel(t, p, "")
	p["lsblk"] = sticks
	press(m, "enter")
	_, cmd := m.Update(sysTickMsg{})
	drain(m, cmd)
	press(m, "esc")
	if out := ansi.Strip(m.View().Content); strings.Contains(out, "DataTraveler 3.0  ") && strings.Contains(out, "KINGSTON") {
		t.Errorf("reloaded under the details:\n%s", out)
	}
	_, cmd = m.Update(sysTickMsg{})
	drain(m, cmd)
	if out := ansi.Strip(m.View().Content); !strings.Contains(out, "KINGSTON") {
		t.Errorf("the tick didn't find the stick:\n%s", out)
	}
}
