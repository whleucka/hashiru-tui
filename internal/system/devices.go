package system

import (
	"bufio"
	"bytes"
	"fmt"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

// USBDevice is one USB device, from sysfs.
type USBDevice struct {
	ID            string // bus-port: "5-1.4.2", what `usb --reset` takes
	VendorProduct string // "0951:1666"
	Maker, Name   string
	Speed         int // Mb/s: 1.5 shows as 1, 12, 480, 5000, 10000...
	Depth         int // hubs between it and the root port
	Hub           bool
}

// SpeedText is the link speed as people name it.
func (d USBDevice) SpeedText() string {
	switch {
	case d.Speed <= 0:
		return ""
	case d.Speed <= 12:
		return "USB 1"
	case d.Speed <= 480:
		return "USB 2"
	}
	return fmt.Sprintf("USB 3 · %dG", d.Speed/1000)
}

var reUSBID = regexp.MustCompile(`^\d+-\d+(\.\d+)*$`)

// USBDevices lists root's /sys/bus/usb/devices as a tree: each device right
// after the hub it hangs off, ports in order. Interfaces (with a colon) and
// root hubs (usbN) aren't devices you plug in, so they're left out.
func USBDevices(root string) []USBDevice {
	dirs, _ := filepath.Glob(filepath.Join(root, "sys/bus/usb/devices/*"))
	var devs []USBDevice
	for _, d := range dirs {
		id := filepath.Base(d)
		if !reUSBID.MatchString(id) {
			continue
		}
		speed, _ := strconv.ParseFloat(readLine(filepath.Join(d, "speed")), 64)
		devs = append(devs, USBDevice{
			ID:            id,
			VendorProduct: readLine(filepath.Join(d, "idVendor")) + ":" + readLine(filepath.Join(d, "idProduct")),
			Maker:         readLine(filepath.Join(d, "manufacturer")),
			Name:          readLine(filepath.Join(d, "product")),
			Speed:         int(speed),
			Depth:         strings.Count(id, "."),
			Hub:           readLine(filepath.Join(d, "bDeviceClass")) == "09",
		})
	}
	slices.SortFunc(devs, func(a, b USBDevice) int { return slices.Compare(usbPath(a.ID), usbPath(b.ID)) })
	return devs
}

// usbPath is "5-1.4.2" as numbers, [5 1 4 2], so 5-1.10 sorts after 5-1.9.
func usbPath(id string) []int {
	var p []int
	for _, f := range strings.FieldsFunc(id, func(r rune) bool { return r == '-' || r == '.' }) {
		n, _ := strconv.Atoi(f)
		p = append(p, n)
	}
	return p
}

// BoltDevice is one Thunderbolt device bolt knows: connected now, or stored
// from before.
type BoltDevice struct {
	Name, Vendor string
	UUID         string
	Generation   string // "USB4", "Thunderbolt 3"
	Status       string // disconnected, connected, authorizing, authorized, auth-error...
	Stored       bool   // enrolled: authorized automatically when it's plugged in
	Speed        string // "40 Gb/s = 2 lanes * 20 Gb/s" while connected
}

// Connected reports whether the device is plugged in now.
func (d BoltDevice) Connected() bool {
	return d.Status != "disconnected" && d.Status != "unknown" && d.Status != ""
}

// NeedsAuth reports whether it's plugged in and waiting to be authorized:
// the dock that's connected and doing nothing.
func (d BoltDevice) NeedsAuth() bool { return d.Status == "connected" || d.Status == "auth-error" }

// reBoltKey is one `key: value` line of boltctl's tree, with the nesting
// (the "|  " runs) before the branch.
var reBoltKey = regexp.MustCompile("^ +((?:\\|  )*)[|`]- ([a-z ]+):\\s*(.*)$")

// ParseBolt reads `boltctl list`. There's no JSON, so it reads the tree
// loosely: a device per "o" or "*" header, its top-level keys, and the
// link speed nested under status. Keys it doesn't know are skipped.
func ParseBolt(out []byte) []BoltDevice {
	var devs []BoltDevice
	var cur *BoltDevice
	sc := bufio.NewScanner(bytes.NewReader(out))
	for sc.Scan() {
		line := sc.Text()
		if strings.HasPrefix(line, " o ") || strings.HasPrefix(line, " * ") {
			devs = append(devs, BoltDevice{Name: strings.TrimSpace(line[3:])})
			cur = &devs[len(devs)-1]
			continue
		}
		m := reBoltKey.FindStringSubmatch(line)
		if m == nil || cur == nil {
			continue
		}
		nested, key, val := m[1] != "", m[2], strings.TrimSpace(m[3])
		switch {
		case nested && key == "rx speed":
			cur.Speed = val
		case nested:
		case key == "name":
			cur.Name = val
		case key == "vendor":
			cur.Vendor = val
		case key == "uuid":
			cur.UUID = val
		case key == "generation":
			cur.Generation = val
		case key == "status":
			cur.Status = val
		case key == "stored":
			cur.Stored = val != "" && val != "no"
		}
	}
	return devs
}

// BoltSecurity is the first domain's security level from `boltctl domains`
// ("iommu+user": devices need authorizing), or "".
func BoltSecurity(out []byte) string {
	sc := bufio.NewScanner(bytes.NewReader(out))
	for sc.Scan() {
		if m := reBoltKey.FindStringSubmatch(sc.Text()); m != nil && m[2] == "security" {
			return strings.TrimSpace(m[3])
		}
	}
	return ""
}

// BoltArgs are boltctl's listings; neither needs root.
var (
	BoltListArgs    = []string{"list"}
	BoltDomainsArgs = []string{"domains"}
)
