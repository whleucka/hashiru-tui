package system

import (
	"context"
	"os/exec"
	"strings"
	"testing"
)

func TestParseGPUs(t *testing.T) {
	if got := ParseGPUs(fixture(t, "lspci-mm.txt")); strings.Join(got, "|") != "Radeon 840M / 860M" {
		t.Errorf("this machine = %q", got)
	}
	two := `00:02.0 "VGA compatible controller" "Intel Corporation" "Meteor Lake-P [Intel Arc Graphics]" -r08 "Lenovo" "Device 50e4"
01:00.0 "3D controller" "NVIDIA Corporation" "AD107M [GeForce RTX 4060 Max-Q / Mobile]" -ra1 "Lenovo" "Device 50e4"
02:00.0 "Ethernet controller" "Realtek" "RTL8111" "" ""
03:00.0 "VGA compatible controller" "Matrox Electronics Systems Ltd." "MGA G200eW WPCM450" "" ""`
	want := "Intel Arc|GeForce RTX 4060 Max-Q / Mobile|Matrox MGA G200eW WPCM450"
	if got := ParseGPUs([]byte(two)); strings.Join(got, "|") != want {
		t.Errorf("gpus = %q, want %q", got, want)
	}
}

func TestReadMachine(t *testing.T) {
	answers := map[string][]byte{
		"lspci -mm":           fixture(t, "lspci-mm.txt"),
		"hyprctl version -j":  fixture(t, "hyprctl-version.json"),
		"hyprctl monitors -j": fixture(t, "hyprctl-monitors-docked.json"),
		"pacman -Qq":          fixture(t, "pacman-Qq.txt"),
	}
	run := func(_ context.Context, name string, args ...string) ([]byte, error) {
		if out, ok := answers[strings.Join(append([]string{name}, args...), " ")]; ok {
			return out, nil
		}
		return nil, &exec.Error{Name: name, Err: exec.ErrNotFound}
	}
	m := ReadMachine(context.Background(), run)
	if m.Hyprland != "0.56.2" || len(m.GPUs) != 1 || m.Packages < 1000 || len(m.Displays) == 0 {
		t.Errorf("machine = %+v", m)
	}
	for _, d := range m.Displays {
		if d.Disabled {
			t.Errorf("a disabled display: %+v", d)
		}
	}

	// Outside Hyprland, with nothing installed: all empty, no panic.
	if m := ReadMachine(context.Background(), func(_ context.Context, name string, _ ...string) ([]byte, error) {
		return nil, &exec.Error{Name: name, Err: exec.ErrNotFound}
	}); m.Hyprland != "" || m.Displays != nil || m.GPUs != nil || m.Packages != 0 {
		t.Errorf("nothing = %+v", m)
	}
}

func TestMonitorDisplay(t *testing.T) {
	for _, c := range []struct {
		m    Monitor
		want string
	}{
		{Monitor{Name: "DP-9", Width: 2560, Height: 1440, Refresh: 143.97, Scale: 1}, "DP-9 2560×1440@144"},
		{Monitor{Name: "eDP-1", Width: 2880, Height: 1800, Refresh: 120, Scale: 1.25}, "eDP-1 2880×1800@120 ×1.25"},
	} {
		if got := c.m.Display(); got != c.want {
			t.Errorf("Display() = %q, want %q", got, c.want)
		}
	}
}
