package tui

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

// fwProbe answers fwupdmgr from this machine's devices and the hand-made
// pending updates.
func fwProbe(t *testing.T) probe {
	t.Helper()
	devs, err1 := os.ReadFile("../system/testdata/fwupdmgr-get-devices.json")
	ups, err2 := os.ReadFile("../system/testdata/fwupdmgr-get-updates.json")
	if err1 != nil || err2 != nil {
		t.Fatal(err1, err2)
	}
	return probe{
		"fwupdmgr get-devices --json": string(devs),
		"fwupdmgr get-updates --json": string(ups),
	}
}

func hardwareModel(t *testing.T, p probe) (*Model, *[]string) {
	t.Helper()
	var launched []string
	m := New(Options{
		Source: fakeSource{},
		Probe:  p.run,
		RunInPane: func(_ context.Context, argv []string, _ string, _, _ bool) error {
			launched = append(launched, strings.Join(argv, " "))
			return nil
		},
	})
	load(t, m)
	goTo(t, m, "Hardware")
	return m, &launched
}

// firmwareModel is hardwareModel on the Firmware tab.
func firmwareModel(t *testing.T, p probe) (*Model, *[]string) {
	t.Helper()
	m, launched := hardwareModel(t, p)
	if err := m.openTab(m.area(), "firmware"); err != nil {
		t.Fatal(err)
	}
	return m, launched
}

func TestHardwareFirmware(t *testing.T) {
	m, _ := firmwareModel(t, fwProbe(t))
	out := ansi.Strip(m.View().Content)
	for _, want := range []string{
		"Firmware (2)", "2 updates", "System Firmware", "65549 → 0.1.15",
		"Integrated Camera", "35.12 → 35.14", "Fingerprint Sensor", "KIOXIA",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in\n%s", want, out)
		}
	}
	// Pending updates come first.
	if strings.Index(out, "System Firmware") > strings.Index(out, "Fingerprint Sensor") {
		t.Errorf("updates aren't first:\n%s", out)
	}
	// A device with an update isn't listed twice.
	if strings.Count(out, "Integrated Camera") != 1 {
		t.Errorf("camera listed %d times", strings.Count(out, "Integrated Camera"))
	}
}

func TestHardwareFirmwareDetails(t *testing.T) {
	m, _ := firmwareModel(t, fwProbe(t))
	press(m, "enter")
	out := ansi.Strip(m.View().Content)
	for _, want := range []string{"System Firmware", "LENOVO", "65549 → 0.1.15", "needs a reboot", "on AC", "• Fixed an issue where the system might hang on resume.", "0.1.14"} {
		if !strings.Contains(out, want) {
			t.Errorf("details missing %q in\n%s", want, out)
		}
	}
	press(m, "esc")
	if out := ansi.Strip(m.View().Content); !strings.Contains(out, "Fingerprint Sensor") {
		t.Errorf("esc didn't go back to the list:\n%s", out)
	}
}

func TestHardwareFirmwareActions(t *testing.T) {
	m, launched := firmwareModel(t, fwProbe(t))
	press(m, "F")
	press(m, "u")
	if want := []string{"fwupdmgr refresh", "fwupdmgr update"}; strings.Join(*launched, "|") != strings.Join(want, "|") {
		t.Errorf("launched %v, want %v", *launched, want)
	}
	ids := map[string]bool{}
	for _, a := range m.Actions() {
		ids[a.ID] = true
	}
	for _, id := range []string{"hardware.firmware-refresh", "hardware.firmware-update"} {
		if !ids[id] {
			t.Errorf("no action %s", id)
		}
	}
}

func TestHardwareFirmwareMissingAndDown(t *testing.T) {
	m, _ := firmwareModel(t, probe{})
	if out := ansi.Strip(m.View().Content); !strings.Contains(out, "fwupdmgr isn't installed") {
		t.Errorf("missing fwupd:\n%s", out)
	}
	m, _ = firmwareModel(t, probe{"fwupdmgr get-devices --json": "not json"})
	if out := ansi.Strip(m.View().Content); !strings.Contains(out, "fwupdmgr") || !strings.Contains(out, "get-devices") {
		t.Errorf("fwupd failing should name the command:\n%s", out)
	}
}

func TestHardwareDeepLinkAndSidebar(t *testing.T) {
	m := New(Options{Source: fakeSource{}, Probe: fwProbe(t).run})
	if err := m.Start("hardware.firmware", ""); err != nil {
		t.Fatal(err)
	}
	load(t, m)
	if m.area().Name() != "Hardware" {
		t.Fatalf("opened on %s", m.area().Name())
	}
	// The eleventh area has no number key, and no junk in its place.
	for _, line := range strings.Split(ansi.Strip(m.sidebar()), "\n") {
		if strings.Contains(line, "Hardware") && strings.TrimSpace(line) != "Hardware" {
			t.Errorf("sidebar row %q", line)
		}
	}
}
