package system

import (
	"context"
	"errors"
	"os/exec"
	"testing"
)

func TestParseFirmwareDevices(t *testing.T) {
	devs, err := ParseFirmwareDevices(fixture(t, "fwupdmgr-get-devices.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(devs) != 26 {
		t.Fatalf("got %d devices, want 26", len(devs))
	}
	var sys *FirmwareDevice
	updatable := 0
	for i, d := range devs {
		if d.Name == "System Firmware" {
			sys = &devs[i]
		}
		if d.Updatable {
			updatable++
		}
	}
	if updatable != 16 {
		t.Errorf("%d updatable, want 16", updatable)
	}
	if sys == nil {
		t.Fatal("no System Firmware")
	}
	if sys.Vendor != "LENOVO" || sys.Version != "65549" || !sys.NeedsReboot || !sys.RequireAC || !sys.Updatable {
		t.Errorf("System Firmware = %+v", *sys)
	}
	// A device with no version or vendor is listed, not dropped.
	if devs[0].Name != "B140UAN04.7" || devs[0].Version != "" || devs[0].Updatable {
		t.Errorf("first = %+v", devs[0])
	}
}

func TestParseFirmwareUpdates(t *testing.T) {
	ups, err := ParseFirmwareUpdates(fixture(t, "fwupdmgr-get-updates.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(ups) != 2 {
		t.Fatalf("got %d updates, want 2", len(ups))
	}
	u := ups[0]
	if u.Device.Name != "System Firmware" || u.From() != "65549" || u.To() != "0.1.15" {
		t.Errorf("update = %s %s → %s", u.Device.Name, u.From(), u.To())
	}
	if !u.Device.NeedsReboot || len(u.Releases) != 2 {
		t.Errorf("update = %+v", u)
	}
	if want := "Lenovo ThinkPad T14s Gen 6 System Firmware\n\n• Fixed an issue where the system might hang on resume."; u.Releases[0].Notes() != want {
		t.Errorf("notes = %q, want %q", u.Releases[0].Notes(), want)
	}
}

func TestParseFirmwareUpdatesNone(t *testing.T) {
	ups, err := ParseFirmwareUpdates([]byte("{\n  \"Devices\": [\n  ]\n}\n"))
	if err != nil || len(ups) != 0 {
		t.Errorf("got %v, %v; want none, no error", ups, err)
	}
	if _, err := ParseFirmwareDevices([]byte("not json")); err == nil {
		t.Error("bad JSON: no error")
	}
}

func TestNoFirmwareUpdates(t *testing.T) {
	two := exec.Command("sh", "-c", "exit 2").Run()
	one := exec.Command("sh", "-c", "exit 1").Run()
	if !NoFirmwareUpdates(two) {
		t.Error("exit 2 is fwupd's nothing to do")
	}
	if NoFirmwareUpdates(one) || NoFirmwareUpdates(errors.New("x")) || NoFirmwareUpdates(nil) {
		t.Error("only exit 2 means nothing to do")
	}
}

// fwupdRun answers fwupdmgr by subcommand.
type fwupdRun map[string]struct {
	out  string
	code int // -1: not installed
}

func (f fwupdRun) Run(_ context.Context, name string, args ...string) ([]byte, error) {
	r := f[args[0]]
	switch {
	case name != "fwupdmgr" || r.code == -1:
		return nil, &exec.Error{Name: name, Err: exec.ErrNotFound}
	case r.code != 0:
		return []byte(r.out), exitErr(r.code)
	}
	return []byte(r.out), nil
}

func TestReadFirmware(t *testing.T) {
	devs := string(fixture(t, "fwupdmgr-get-devices.json"))
	ups := string(fixture(t, "fwupdmgr-get-updates.json"))
	ctx := context.Background()

	f := ReadFirmware(ctx, fwupdRun{"get-devices": {out: devs}, "get-updates": {out: ups}}.Run)
	if f.Err != nil || f.UpdatesErr != nil || f.Missing || len(f.Devices) != 26 || len(f.Updates) != 2 {
		t.Errorf("both = %d devices, %d updates, %v / %v", len(f.Devices), len(f.Updates), f.Err, f.UpdatesErr)
	}

	f = ReadFirmware(ctx, fwupdRun{"get-devices": {out: devs}, "get-updates": {code: 2}}.Run)
	if f.UpdatesErr != nil || len(f.Updates) != 0 || len(f.Devices) != 26 {
		t.Errorf("nothing to do = %d updates, %v", len(f.Updates), f.UpdatesErr)
	}

	f = ReadFirmware(ctx, fwupdRun{"get-devices": {out: devs}, "get-updates": {code: 1}}.Run)
	if f.UpdatesErr == nil || len(f.Devices) != 26 {
		t.Errorf("updates failing keeps the devices: %d, %v", len(f.Devices), f.UpdatesErr)
	}

	f = ReadFirmware(ctx, fwupdRun{"get-devices": {code: 1}}.Run)
	if f.Err == nil || f.Missing {
		t.Errorf("daemon down = %v, missing %v", f.Err, f.Missing)
	}

	f = ReadFirmware(ctx, fwupdRun{"get-devices": {code: -1}}.Run)
	if !f.Missing || f.Err != nil {
		t.Errorf("not installed = %v, missing %v", f.Err, f.Missing)
	}
}
