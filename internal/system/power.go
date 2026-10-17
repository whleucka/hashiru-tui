package system

import (
	"bufio"
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// BatteryState is one battery in full, from sysfs, for the Power tab
// (Overview reads the short Battery in sysinfo.go).
type BatteryState struct {
	Name     string // BAT0
	Capacity int    // percent
	Status   string // Charging, Discharging, Not charging, Full
	Health   float64
	Cycles   int // -1 when the battery doesn't say

	// Thresholds is whether it takes charge thresholds; Start and Stop are
	// what they are now.
	Thresholds  bool
	Start, Stop int
}

// readInt is a sysfs number, or -1.
func readInt(path string) int {
	b, err := os.ReadFile(path)
	if err != nil {
		return -1
	}
	n, err := strconv.Atoi(strings.TrimSpace(string(b)))
	if err != nil {
		return -1
	}
	return n
}

func readLine(path string) string {
	b, _ := os.ReadFile(path)
	return strings.TrimSpace(string(b))
}

// ReadBatteries lists the batteries under root's /sys/class/power_supply.
func ReadBatteries(root string) []BatteryState {
	dirs, _ := filepath.Glob(filepath.Join(root, "sys/class/power_supply/BAT*"))
	var bats []BatteryState
	for _, d := range dirs {
		b := BatteryState{
			Name:     filepath.Base(d),
			Capacity: readInt(filepath.Join(d, "capacity")),
			Status:   readLine(filepath.Join(d, "status")),
			Cycles:   readInt(filepath.Join(d, "cycle_count")),
			Start:    readInt(filepath.Join(d, "charge_control_start_threshold")),
			Stop:     readInt(filepath.Join(d, "charge_control_end_threshold")),
		}
		b.Thresholds = b.Stop >= 0
		// Some batteries report 0 cycles rather than nothing.
		if b.Cycles == 0 {
			b.Cycles = -1
		}
		// Watt-hours or amp-hours, whichever the battery reports.
		for _, unit := range []string{"energy", "charge"} {
			full, design := readInt(filepath.Join(d, unit+"_full")), readInt(filepath.Join(d, unit+"_full_design"))
			if full > 0 && design > 0 {
				b.Health = float64(full) / float64(design)
				break
			}
		}
		bats = append(bats, b)
	}
	return bats
}

// OnAC reports whether a mains supply is online; known is false on a
// machine with none (a desktop with no power_supply, or one sysfs hides).
func OnAC(root string) (on, known bool) {
	dirs, _ := filepath.Glob(filepath.Join(root, "sys/class/power_supply/*"))
	for _, d := range dirs {
		if readLine(filepath.Join(d, "type")) != "Mains" {
			continue
		}
		known = true
		if readLine(filepath.Join(d, "online")) == "1" {
			return true, true
		}
	}
	return false, known
}

// Charger is the USB-C supply charging the machine.
type Charger struct {
	Watts int    // what it offers: the negotiated voltage × its current limit
	Now   int    // what it's drawing, when the supply says
	Kind  string // the USB type in use: C, PD, PD_PPS
}

// ReadCharger finds the online USB-C power supply (ucsi's, type USB), or
// ok is false: unplugged, or a barrel charger, which only shows as AC.
func ReadCharger(root string) (c Charger, ok bool) {
	dirs, _ := filepath.Glob(filepath.Join(root, "sys/class/power_supply/ucsi-source-psy-*"))
	for _, d := range dirs {
		if readLine(filepath.Join(d, "type")) != "USB" || readLine(filepath.Join(d, "online")) != "1" {
			continue
		}
		volts := float64(readInt(filepath.Join(d, "voltage_max"))) / 1e6
		c.Watts = int(volts*float64(readInt(filepath.Join(d, "current_max")))/1e6 + 0.5)
		c.Now = int(float64(readInt(filepath.Join(d, "voltage_now")))*float64(readInt(filepath.Join(d, "current_now")))/1e12 + 0.5)
		// usb_type lists every type with the one in use bracketed: "C [PD] PD_PPS".
		for _, f := range strings.Fields(readLine(filepath.Join(d, "usb_type"))) {
			if strings.HasPrefix(f, "[") {
				c.Kind = strings.Trim(f, "[]")
			}
		}
		c.Watts, c.Now = max(c.Watts, 0), max(c.Now, 0) // a value the supply leaves unset reads -1
		return c, true
	}
	return Charger{}, false
}

// PlatformProfile is the ACPI platform profile and the ones the firmware
// offers, or "" and nil without one.
func PlatformProfile(root string) (string, []string) {
	cur := readLine(filepath.Join(root, "sys/firmware/acpi/platform_profile"))
	if cur == "" {
		return "", nil
	}
	return cur, strings.Fields(readLine(filepath.Join(root, "sys/firmware/acpi/platform_profile_choices")))
}

// TLPStatus is what `tlp-stat -s` reports. It reads without root.
type TLPStatus struct {
	Version string
	Enabled bool
	Profile string // performance, balanced, power-saver
	Source  string // AC, battery
}

// TLPProfiles are the profiles `tlp <profile>` takes (TLP 1.8 and later).
var TLPProfiles = []string{"performance", "balanced", "power-saver"}

var reTLPVersion = regexp.MustCompile(`^--- TLP (\S+)`)

// ParseTLPStatus reads the `key = value` lines of `tlp-stat -s`, loosely:
// a line it doesn't know is skipped, a field it can't find is "".
func ParseTLPStatus(out []byte) TLPStatus {
	var s TLPStatus
	sc := bufio.NewScanner(bytes.NewReader(out))
	for sc.Scan() {
		line := sc.Text()
		if m := reTLPVersion.FindStringSubmatch(line); m != nil {
			s.Version = m[1]
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		k, v = strings.TrimSpace(k), strings.TrimSpace(v)
		switch k {
		case "tlp":
			s.Enabled = strings.HasPrefix(v, "enabled")
		case "TLP profile":
			s.Profile, _, _ = strings.Cut(v, "/") // "balanced/BAT"
		case "Power source":
			s.Source = v
		}
	}
	return s
}

// TLPStatusArgs: system info and status only, no root needed.
var TLPStatusArgs = []string{"-s"}

// Idle is hypridle's two timeouts, in seconds: locking and turning the
// screen off. 0 is none.
type Idle struct{ Lock, Off int }

var reTimeout = regexp.MustCompile(`^(\s*timeout\s*=\s*)\d+(.*)$`)

// listener is one listener block: its timeout line and what it does.
type listener struct {
	timeoutLine int
	timeout     int
	action      string
}

// listeners finds the conf's listener blocks, in order.
func listeners(lines []string) []listener {
	var ls []listener
	var cur *listener
	for i, raw := range lines {
		line := strings.TrimSpace(raw)
		switch {
		case strings.HasPrefix(line, "#"):
		case strings.HasPrefix(line, "listener") && strings.HasSuffix(line, "{"):
			cur = &listener{timeoutLine: -1}
		case cur != nil && line == "}":
			ls = append(ls, *cur)
			cur = nil
		case cur != nil && reTimeout.MatchString(raw):
			cur.timeoutLine = i
			_, v, _ := strings.Cut(line, "=")
			cur.timeout, _ = strconv.Atoi(strings.TrimSpace(v))
		case cur != nil && strings.HasPrefix(line, "on-timeout"):
			cur.action = line
		}
	}
	return ls
}

func (l listener) locks() bool {
	return strings.Contains(l.action, "lock-session") || strings.Contains(l.action, "hyprlock")
}
func (l listener) blanks() bool { return strings.Contains(l.action, "dpms") }

// ParseIdle reads the lock and screen-off timeouts out of a hypridle conf:
// the listener that locks and the one that turns displays off.
func ParseIdle(conf []byte) Idle {
	var t Idle
	for _, l := range listeners(strings.Split(string(conf), "\n")) {
		switch {
		case l.locks() && t.Lock == 0:
			t.Lock = l.timeout
		case l.blanks() && t.Off == 0:
			t.Off = l.timeout
		}
	}
	return t
}

// RewriteIdle is conf with the lock and screen-off listeners' timeouts set
// to t, and every other line as it was.
func RewriteIdle(conf []byte, t Idle) ([]byte, error) {
	lines := strings.Split(string(conf), "\n")
	set := func(i, secs int) {
		lines[i] = reTimeout.ReplaceAllString(lines[i], "${1}"+strconv.Itoa(secs)+"${2}")
	}
	var lock, off bool
	for _, l := range listeners(lines) {
		if l.timeoutLine < 0 {
			continue
		}
		switch {
		case l.locks() && !lock:
			set(l.timeoutLine, t.Lock)
			lock = true
		case l.blanks() && !off:
			set(l.timeoutLine, t.Off)
			off = true
		}
	}
	if !lock || !off {
		return nil, errors.New("hypridle conf: no lock and screen-off listeners to change")
	}
	return []byte(strings.Join(lines, "\n")), nil
}
