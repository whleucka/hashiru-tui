package system

import (
	"strings"
	"testing"
)

func TestUSBDevices(t *testing.T) {
	devs := USBDevices("testdata/sysroot")
	if len(devs) != 20 {
		t.Fatalf("got %d devices", len(devs))
	}
	byID := map[string]USBDevice{}
	var order []string
	for _, d := range devs {
		byID[d.ID] = d
		order = append(order, d.ID)
	}
	stick := byID["4-2"]
	if stick.Name != "DataTraveler 3.0" || stick.Maker != "Kingston" || stick.VendorProduct != "0951:1666" ||
		stick.Speed != 5000 || stick.Depth != 0 || stick.Hub {
		t.Errorf("stick = %+v", stick)
	}
	if kb := byID["5-1.4.2.1"]; kb.Depth != 3 || kb.Speed != 12 || kb.Name != "X50 RGB Mechanical Keyboard" {
		t.Errorf("keyboard = %+v", kb)
	}
	if hub := byID["5-1.4"]; !hub.Hub || hub.Name != "USB2.0 Hub" {
		t.Errorf("hub = %+v (name trimmed?)", hub)
	}
	// A tree: each device right after its hub, ports in numeric order.
	if got := strings.Join(order[5:10], " "); got != "5-1 5-1.1 5-1.2 5-1.4 5-1.4.2" {
		t.Errorf("order = %s", got)
	}
	if got := USBDevices(t.TempDir()); len(got) != 0 {
		t.Errorf("no usb: %+v", got)
	}
}

func TestUSBSpeed(t *testing.T) {
	for speed, want := range map[int]string{1: "USB 1", 12: "USB 1", 480: "USB 2", 5000: "USB 3 · 5G", 10000: "USB 3 · 10G", 20000: "USB 3 · 20G", 0: ""} {
		if got := (USBDevice{Speed: speed}).SpeedText(); got != want {
			t.Errorf("speed %d = %q, want %q", speed, got, want)
		}
	}
}

func TestParseBolt(t *testing.T) {
	docked := ParseBolt(fixture(t, "bolt-list-docked.txt"))
	if len(docked) != 1 {
		t.Fatalf("devices = %+v", docked)
	}
	d := docked[0]
	if d.Name != "ThinkPad Thunderbolt 4 Dock" || d.Vendor != "Lenovo" || d.UUID != "36a78780-0022-688d-ffff-ffffffffffff" ||
		d.Generation != "USB4" || d.Status != "authorized" || !d.Stored || d.Speed != "40 Gb/s = 2 lanes * 20 Gb/s" {
		t.Errorf("docked = %+v", d)
	}
	if !d.Connected() || d.NeedsAuth() {
		t.Errorf("docked: connected %v, needs auth %v", d.Connected(), d.NeedsAuth())
	}

	gone := ParseBolt(fixture(t, "bolt-list-undocked.txt"))[0]
	if gone.Status != "disconnected" || gone.Connected() || !gone.Stored {
		t.Errorf("undocked = %+v", gone)
	}

	waiting := ParseBolt([]byte(" o Some Dock\n   |- name:          Some Dock\n   |- uuid:          abc\n   `- status:        connected\n"))[0]
	if !waiting.NeedsAuth() || waiting.Stored {
		t.Errorf("waiting = %+v", waiting)
	}
	if got := ParseBolt(nil); len(got) != 0 {
		t.Errorf("empty = %+v", got)
	}
}

func TestBoltSecurity(t *testing.T) {
	if got := BoltSecurity(fixture(t, "bolt-domains.txt")); got != "iommu+user" {
		t.Errorf("security = %q", got)
	}
	if got := BoltSecurity(nil); got != "" {
		t.Errorf("none = %q", got)
	}
}

func TestParseDrives(t *testing.T) {
	drives, err := ParseDrives(fixture(t, "lsblk-sticks.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(drives) != 3 {
		t.Fatalf("drives = %+v", drives)
	}
	stick := drives[0]
	if stick.Path != "/dev/sda" || stick.Model != "DataTraveler 3.0" || stick.Tran != "usb" || stick.Size != "28.9G" || len(stick.Parts) != 1 {
		t.Errorf("stick = %+v", stick)
	}
	if p := stick.Parts[0]; p.Path != "/dev/sda1" || p.Label != "KINGSTON" || p.FSType != "vfat" ||
		p.Mount() != "/run/media/whleucka/KINGSTON" || p.Locked() || p.Encrypted() {
		t.Errorf("stick part = %+v", p)
	}
	if !stick.Mounted() {
		t.Error("sda has a mounted partition")
	}

	// LUKS: the partition is encrypted; unlocked, its filesystem is the
	// mapper device, which is what gets mounted.
	enc := drives[1].Parts[0]
	if !enc.Encrypted() || enc.Locked() || enc.Inner == nil || enc.Inner.Path != "/dev/mapper/luks-1c2d" ||
		enc.Inner.Label != "secret" || enc.Mount() != "" {
		t.Errorf("luks = %+v (inner %+v)", enc, enc.Inner)
	}
	if drives[1].Mounted() {
		t.Error("sdb has nothing mounted (null mountpoints)")
	}

	// No partition table: the disk is its own filesystem.
	whole := drives[2]
	if len(whole.Parts) != 1 || whole.Parts[0].Path != "/dev/sdc" || whole.Parts[0].FSType != "exfat" || whole.Parts[0].Label != "WHOLE" {
		t.Errorf("whole-disk = %+v", whole)
	}
}

func TestParseDrivesSkipsTheSystemDisk(t *testing.T) {
	drives, err := ParseDrives(fixture(t, "lsblk-undocked-nostick.json"))
	if err != nil || len(drives) != 0 {
		t.Errorf("this machine with no stick: %+v, %v", drives, err)
	}
	if _, err := ParseDrives([]byte("lsblk: bad")); err == nil {
		t.Error("bad JSON: no error")
	}
}

func TestLockedLUKS(t *testing.T) {
	drives, _ := ParseDrives([]byte(`{"blockdevices":[{"name":"sdb","path":"/dev/sdb","size":"7.5G","rm":true,"hotplug":true,"tran":"usb","type":"disk",
		"children":[{"name":"sdb1","path":"/dev/sdb1","size":"7.5G","fstype":"crypto_LUKS","mountpoints":[null],"type":"part"}]}]}`))
	if p := drives[0].Parts[0]; !p.Encrypted() || !p.Locked() || p.Inner != nil {
		t.Errorf("locked = %+v", p)
	}
}
