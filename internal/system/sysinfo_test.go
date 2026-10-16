package system

import (
	"errors"
	"testing"
	"time"
)

func TestReadSysInfo(t *testing.T) {
	statfs := func(path string) (total, free uint64, err error) {
		if path != "/" {
			t.Errorf("statfs(%q)", path)
		}
		return 1000 << 30, 250 << 30, nil
	}
	si := ReadSysInfo("testdata/sysroot", statfs)

	if si.Host != "t14s" || si.Kernel != "7.2.7-arch1-1" {
		t.Errorf("host %q kernel %q", si.Host, si.Kernel)
	}
	if si.Uptime < 14*24*time.Hour || si.Uptime > 15*24*time.Hour {
		t.Errorf("uptime = %v", si.Uptime)
	}
	if si.MemTotal != 31967280*1024 || si.MemAvail == 0 || si.MemAvail > si.MemTotal {
		t.Errorf("mem %d / %d", si.MemAvail, si.MemTotal)
	}
	if si.DiskTotal != 1000<<30 || si.DiskFree != 250<<30 {
		t.Errorf("disk %d / %d", si.DiskFree, si.DiskTotal)
	}
	if si.Battery == nil || si.Battery.Percent != 32 || si.Battery.Status != "Discharging" {
		t.Errorf("battery = %+v", si.Battery)
	}
}

func TestReadSysInfoDesktop(t *testing.T) {
	// No battery, and a failing statfs: both just missing, no panic.
	si := ReadSysInfo(t.TempDir(), func(string) (uint64, uint64, error) { return 0, 0, errors.New("nope") })
	if si.Battery != nil || si.DiskTotal != 0 {
		t.Errorf("si = %+v", si)
	}
}

func TestHumanBytes(t *testing.T) {
	for in, want := range map[uint64]string{
		512:             "512 B",
		1536:            "1.5 KiB",
		31967280 * 1024: "30.5 GiB",
		1000 << 30:      "1000.0 GiB",
		3 << 40:         "3.0 TiB",
	} {
		if got := HumanBytes(in); got != want {
			t.Errorf("HumanBytes(%d) = %q, want %q", in, got, want)
		}
	}
}
