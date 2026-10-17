package system

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

// Machine is what Overview shows that takes a command to find out: the
// graphics, the compositor, the displays and how many packages there are.
// It's read once per refresh, not on Overview's tick.
type Machine struct {
	GPUs     []string // "Radeon 840M / 860M"
	Hyprland string   // "0.56.2", "" outside a Hyprland session
	Displays []Monitor
	Packages int
}

// Display is "DP-9 2560×1440@144", with " ×1.25" when it's scaled.
func (m Monitor) Display() string {
	s := fmt.Sprintf("%s %d×%d@%.0f", m.Name, m.Width, m.Height, m.Refresh)
	if m.Scale != 0 && m.Scale != 1 {
		s += fmt.Sprintf(" ×%g", m.Scale)
	}
	return s
}

var (
	LspciArgs           = []string{"-mm"}
	HyprVersionArgs     = []string{"version", "-j"}
	ActiveMonitorsArgs  = []string{"monitors", "-j"}
	PacmanInstalledArgs = []string{"-Qq"}
)

// ReadMachine runs what it needs; anything that fails is left empty.
func ReadMachine(ctx context.Context, run Run) Machine {
	var m Machine
	if out, err := run(ctx, "lspci", LspciArgs...); err == nil {
		m.GPUs = ParseGPUs(out)
	}
	if out, err := run(ctx, "hyprctl", HyprVersionArgs...); err == nil {
		m.Hyprland = ParseHyprVersion(out)
	}
	if out, err := run(ctx, "hyprctl", ActiveMonitorsArgs...); err == nil {
		if mons, err := ParseMonitors(out); err == nil {
			for _, mon := range mons {
				if !mon.Disabled {
					m.Displays = append(m.Displays, mon)
				}
			}
		}
	}
	if out, err := run(ctx, "pacman", PacmanInstalledArgs...); err == nil {
		m.Packages = bytes.Count(out, []byte("\n"))
	}
	return m
}

var reQuoted = regexp.MustCompile(`"([^"]*)"`)

// ParseGPUs reads `lspci -mm`: the display controllers, by the name in the
// brackets ("Krackan [Radeon 840M / 860M Graphics]" is "Radeon 840M /
// 860M"), or the vendor's and the device's when there isn't one.
func ParseGPUs(out []byte) []string {
	var gpus []string
	for _, line := range strings.Split(string(out), "\n") {
		f := reQuoted.FindAllStringSubmatch(line, -1)
		if len(f) < 3 {
			continue
		}
		class, vendor, device := f[0][1], f[1][1], f[2][1]
		if !strings.Contains(class, "VGA") && !strings.Contains(class, "3D") && !strings.Contains(class, "Display") {
			continue
		}
		name := device
		if i, j := strings.Index(device, "["), strings.LastIndex(device, "]"); i >= 0 && j > i {
			name = device[i+1 : j]
		} else if v, _, _ := strings.Cut(vendor, " "); v != "" {
			name = v + " " + device
		}
		gpus = append(gpus, strings.TrimSpace(strings.TrimSuffix(name, " Graphics")))
	}
	return gpus
}

// ParseHyprVersion reads `hyprctl version -j`.
func ParseHyprVersion(out []byte) string {
	var v struct{ Version, Tag string }
	if json.Unmarshal(out, &v) != nil {
		return ""
	}
	if v.Version != "" {
		return v.Version
	}
	return strings.TrimPrefix(v.Tag, "v")
}
