package tui

import (
	"os"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

func TestOverviewSystemSummary(t *testing.T) {
	m := New(Options{Source: fakeSource{}})
	load(t, m)
	out := ansi.Strip(m.View().Content)
	for _, want := range []string{"t14s", "7.2.7-arch1-1", "up 14d", "of 30.5 GiB", "250.0 GiB free of 1000.0 GiB", "32% discharging"} {
		if !strings.Contains(out, want) {
			t.Errorf("Overview missing %q:\n%s", want, out)
		}
	}
}

// fastfetch's facts that say something on a Hashiru machine; not OS, shell
// or terminal, which are always Arch, zsh and kitty.
func TestOverviewMachine(t *testing.T) {
	read := func(name string) string {
		b, err := os.ReadFile("../system/testdata/" + name)
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}
	p := probe{
		"lspci -mm":           read("lspci-mm.txt"),
		"hyprctl version -j":  read("hyprctl-version.json"),
		"hyprctl monitors -j": read("hyprctl-monitors-docked.json"),
		"pacman -Qq":          read("pacman-Qq.txt"),
	}
	m := New(Options{Source: fakeSource{}, Probe: p.run})
	load(t, m)
	out := ansi.Strip(m.View().Content)
	for _, want := range []string{
		"t14s  ThinkPad T14s Gen 6  up 14d",
		"kernel", "7.2.7-arch1-1",
		"AMD Ryzen AI 7 PRO 350 · 16 threads",
		"Radeon 840M / 860M",
		"hyprland", "0.56.2",
		"displays", "eDP-1 1920×1200@60",
		"swap 0 B of 30.5 GiB",
		"health",
		"installed", "packages",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("Overview missing %q:\n%s", want, out)
		}
	}
}

// Outside Hyprland (ssh, a TTY) the rows that need it just aren't there.
func TestOverviewMachineWithoutHyprland(t *testing.T) {
	m := New(Options{Source: fakeSource{}})
	load(t, m)
	out := ansi.Strip(m.View().Content)
	for _, gone := range []string{"hyprland", "displays", "gpu"} {
		if strings.Contains(out, gone) {
			t.Errorf("%q without the tools:\n%s", gone, out)
		}
	}
	if !strings.Contains(out, "kernel") {
		t.Errorf("kernel is from /proc and should show:\n%s", out)
	}
}
