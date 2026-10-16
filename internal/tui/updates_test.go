package tui

import (
	"strings"
	"testing"
)

func TestPendingUpdatesShown(t *testing.T) {
	m := New(Options{
		Source: fakeSource{},
		Probe: probe{
			"checkupdates": "curl 8.22.0-1 -> 8.23.0-1\nmesa 1:26.0-1 -> 1:26.1-1\n",
			"yay":          "spotify 1:1.2-2 -> 1:1.3-2 [3d]\n",
		}.run,
	})
	load(t, m)

	if out := m.View().Content; !strings.Contains(out, "2 pacman") || !strings.Contains(out, "1 AUR") {
		t.Errorf("Overview missing counts:\n%s", out)
	}
	goTo(t, m, "Updates")
	out := m.View().Content
	for _, want := range []string{"curl", "8.22.0-1 → 8.23.0-1", "spotify", "AUR"} {
		if !strings.Contains(out, want) {
			t.Errorf("Updates missing %q:\n%s", want, out)
		}
	}
}

func TestPendingWithoutYay(t *testing.T) {
	m := New(Options{Source: fakeSource{}, Probe: probe{"checkupdates": ""}.run})
	load(t, m)
	if out := m.View().Content; !strings.Contains(out, "0 pacman") || !strings.Contains(out, "no yay") {
		t.Errorf("Overview:\n%s", out)
	}
}

func TestAURUpgradeIsSeparate(t *testing.T) {
	// hashiru update runs pacman -Syu but never upgrades AUR packages.
	var ran []string
	m := New(Options{
		Source: fakeSource{},
		Exec:   recordExec(&ran),
	})
	load(t, m)
	goTo(t, m, "Updates")
	press(m, "a")
	if len(ran) != 1 || ran[0] != "yay -Sua" {
		t.Errorf("a ran %v", ran)
	}
}
