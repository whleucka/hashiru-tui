package tui

import (
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
