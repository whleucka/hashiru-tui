package system

import (
	"bufio"
	"bytes"
	"encoding/json"
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

// Drive is a removable or hot-plugged disk: a USB stick, an SD card, a
// drive in a dock. Never the disk the system runs from.
type Drive struct {
	Path, Name  string // /dev/sda, sda
	Model, Size string
	Tran        string // usb, thunderbolt, mmc...
	Parts       []Partition
}

// Partition is one filesystem on a drive: a partition, or the whole disk
// when it has no partition table.
type Partition struct {
	Path, Name, Label string
	Size, FSType      string
	Mounts            []string
	// Inner is the unlocked filesystem of a LUKS partition (the
	// /dev/mapper device), nil while it's locked.
	Inner *Partition
}

// Encrypted reports whether it's a LUKS container.
func (p Partition) Encrypted() bool { return p.FSType == "crypto_LUKS" }

// Locked reports whether it's encrypted and not unlocked.
func (p Partition) Locked() bool { return p.Encrypted() && p.Inner == nil }

// Mount is where it's mounted (an unlocked container's filesystem counts),
// or "".
func (p Partition) Mount() string {
	if p.Inner != nil {
		return p.Inner.Mount()
	}
	if len(p.Mounts) > 0 {
		return p.Mounts[0]
	}
	return ""
}

// Mounted reports whether any of its filesystems is mounted, which is what
// stands between it and being pulled out.
func (d Drive) Mounted() bool {
	for _, p := range d.Parts {
		if p.Mount() != "" {
			return true
		}
	}
	return false
}

// DrivesArgs are the lsblk columns ParseDrives reads.
var DrivesArgs = []string{"-J", "-o", "NAME,PATH,LABEL,SIZE,FSTYPE,MOUNTPOINTS,RM,HOTPLUG,TRAN,MODEL,TYPE"}

type lsblkDev struct {
	Name, Path, Size, Type string
	Label, FSType          *string
	Mountpoints            []*string
	RM, Hotplug            bool
	Tran, Model            *string
	Children               []lsblkDev
}

func str(s *string) string {
	if s == nil {
		return ""
	}
	return strings.TrimSpace(*s)
}

func (b lsblkDev) part() Partition {
	p := Partition{Path: b.Path, Name: b.Name, Label: str(b.Label), Size: b.Size, FSType: str(b.FSType)}
	for _, m := range b.Mountpoints {
		if m != nil && *m != "" {
			p.Mounts = append(p.Mounts, *m)
		}
	}
	if p.Encrypted() {
		for _, c := range b.Children {
			if c.Type == "crypt" {
				inner := c.part()
				p.Inner = &inner
			}
		}
	}
	return p
}

// ParseDrives reads `lsblk -J` (DrivesArgs): the disks that are removable or
// hot-plugged, with their filesystems. A fixed disk (the system's) is never
// one, and neither is zram or a loop device.
func ParseDrives(out []byte) ([]Drive, error) {
	var doc struct{ Blockdevices []lsblkDev }
	if err := json.Unmarshal(out, &doc); err != nil {
		return nil, fmt.Errorf("lsblk: %w", err)
	}
	var drives []Drive
	for _, b := range doc.Blockdevices {
		if b.Type != "disk" || !(b.RM || b.Hotplug) || strings.HasPrefix(b.Name, "zram") {
			continue
		}
		d := Drive{Path: b.Path, Name: b.Name, Model: str(b.Model), Size: b.Size, Tran: str(b.Tran)}
		for _, c := range b.Children {
			if c.Type == "part" {
				d.Parts = append(d.Parts, c.part())
			}
		}
		if len(d.Parts) == 0 && str(b.FSType) != "" {
			d.Parts = []Partition{b.part()}
		}
		drives = append(drives, d)
	}
	return drives, nil
}
