package system

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"slices"
	"strings"
)

// FirmwareDevice is one device fwupd knows about.
type FirmwareDevice struct {
	ID      string
	Name    string
	Vendor  string // "" when fwupd doesn't say
	Version string // "" when fwupd doesn't say
	Summary string
	Flags   []string

	Updatable   bool // fwupd can flash it
	NeedsReboot bool // the update applies on the next boot
	RequireAC   bool // only on mains power
}

// FirmwareRelease is one version fwupd could install on a device.
type FirmwareRelease struct {
	Version     string
	Summary     string
	Description string // fwupd's (AppStream) markup
	Flags       []string
}

// FirmwareUpdate is a device with releases newer than what it runs, newest
// first.
type FirmwareUpdate struct {
	Device   FirmwareDevice
	Releases []FirmwareRelease
}

// From is the version the device runs.
func (u FirmwareUpdate) From() string { return u.Device.Version }

// To is the newest release, which is what `fwupdmgr update` installs.
func (u FirmwareUpdate) To() string {
	if len(u.Releases) == 0 {
		return ""
	}
	return u.Releases[0].Version
}

// Neither needs root: fwupd answers anyone on the system bus.
var (
	FirmwareDevicesArgs = []string{"get-devices", "--json"}
	FirmwareUpdatesArgs = []string{"get-updates", "--json"}
)

// Firmware is what fwupd reports. A missing fwupdmgr is flagged rather than
// reported as an error; an error listing updates leaves the devices.
type Firmware struct {
	Devices    []FirmwareDevice
	Updates    []FirmwareUpdate
	Err        error // devices couldn't be listed (daemon down, ...)
	UpdatesErr error
	Missing    bool
}

// ReadFirmware lists fwupd's devices and pending updates. It reads the
// metadata fwupd already has; `fwupdmgr refresh` is what fetches new.
func ReadFirmware(ctx context.Context, run Run) Firmware {
	var f Firmware
	out, err := run(ctx, "fwupdmgr", FirmwareDevicesArgs...)
	if f.Missing = missing(err); err != nil {
		if !f.Missing {
			f.Err = err
		}
		return f
	}
	if f.Devices, f.Err = ParseFirmwareDevices(out); f.Err != nil {
		return f
	}
	out, err = run(ctx, "fwupdmgr", FirmwareUpdatesArgs...)
	switch {
	case NoFirmwareUpdates(err):
	case err != nil:
		f.UpdatesErr = err
	default:
		f.Updates, f.UpdatesErr = ParseFirmwareUpdates(out)
	}
	return f
}

// NoFirmwareUpdates reports whether err is fwupdmgr's "nothing to do"
// (exit 2), which some versions return for get-updates with nothing
// pending instead of an empty list.
func NoFirmwareUpdates(err error) bool {
	code, ok := exitCode(err)
	return ok && code == 2
}

type fwupdDevice struct {
	DeviceID string `json:"DeviceId"`
	Name     string
	Vendor   string
	Version  string
	Summary  string
	Flags    []string
	Releases []struct {
		Version     string
		Summary     string
		Description string
		Flags       []string
	}
}

func (d fwupdDevice) device() FirmwareDevice {
	return FirmwareDevice{
		ID: d.DeviceID, Name: d.Name, Vendor: d.Vendor, Version: d.Version,
		Summary: d.Summary, Flags: d.Flags,
		Updatable:   slices.Contains(d.Flags, "updatable"),
		NeedsReboot: slices.Contains(d.Flags, "needs-reboot"),
		RequireAC:   slices.Contains(d.Flags, "require-ac"),
	}
}

func parseFwupd(out []byte) ([]fwupdDevice, error) {
	var doc struct{ Devices []fwupdDevice }
	if err := json.Unmarshal(out, &doc); err != nil {
		return nil, fmt.Errorf("fwupdmgr: %w", err)
	}
	return doc.Devices, nil
}

// ParseFirmwareDevices reads `fwupdmgr get-devices --json`, in fwupd's order.
func ParseFirmwareDevices(out []byte) ([]FirmwareDevice, error) {
	ds, err := parseFwupd(out)
	if err != nil {
		return nil, err
	}
	devs := make([]FirmwareDevice, len(ds))
	for i, d := range ds {
		devs[i] = d.device()
	}
	return devs, nil
}

// ParseFirmwareUpdates reads `fwupdmgr get-updates --json`: each device with
// something newer, its releases as fwupd orders them (newest first).
func ParseFirmwareUpdates(out []byte) ([]FirmwareUpdate, error) {
	ds, err := parseFwupd(out)
	if err != nil {
		return nil, err
	}
	var ups []FirmwareUpdate
	for _, d := range ds {
		u := FirmwareUpdate{Device: d.device()}
		for _, r := range d.Releases {
			u.Releases = append(u.Releases, FirmwareRelease{
				Version: r.Version, Summary: r.Summary,
				Description: r.Description, Flags: r.Flags,
			})
		}
		ups = append(ups, u)
	}
	return ups, nil
}

var (
	reTag   = regexp.MustCompile(`<[^>]*>`)
	reBlank = regexp.MustCompile(`\n{3,}`)
)

// Notes is the release's description as plain text: paragraphs apart,
// list items as bullets.
func (r FirmwareRelease) Notes() string {
	s := r.Description
	s = strings.NewReplacer("<li>", "\n• ", "</p>", "\n\n", "<ul>", "\n", "</ul>", "\n").Replace(s)
	s = reTag.ReplaceAllString(s, "")
	s = strings.NewReplacer("&amp;", "&", "&lt;", "<", "&gt;", ">", "&quot;", `"`, "&apos;", "'").Replace(s)
	lines := strings.Split(s, "\n")
	for i, l := range lines {
		lines[i] = strings.TrimSpace(l)
	}
	return strings.TrimSpace(reBlank.ReplaceAllString(strings.Join(lines, "\n"), "\n\n"))
}
