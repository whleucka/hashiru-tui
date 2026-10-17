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

func TestReadSysInfoMachine(t *testing.T) {
	si := ReadSysInfo("testdata/sysroot", func(string) (uint64, uint64, error) { return 0, 0, nil })
	if si.Model != "ThinkPad T14s Gen 6" {
		t.Errorf("model = %q", si.Model)
	}
	if si.CPU != "AMD Ryzen AI 7 PRO 350" || si.Threads != 16 {
		t.Errorf("cpu = %q, %d threads", si.CPU, si.Threads)
	}
	if si.SwapTotal != 31967228*1024 || si.SwapFree != si.SwapTotal {
		t.Errorf("swap %d / %d", si.SwapFree, si.SwapTotal)
	}
	if si.Battery == nil || si.Battery.Health < 0.8 || si.Battery.Health > 1 {
		t.Errorf("battery = %+v", si.Battery)
	}
}

func TestModelAndCPUNames(t *testing.T) {
	for _, c := range []struct{ vendor, name, version, want string }{
		{"LENOVO", "21TB000AUS", "ThinkPad T14s Gen 6", "ThinkPad T14s Gen 6"},
		{"Dell Inc.", "XPS 13 9340", "", "XPS 13 9340"},
		{"ASUS", "System Product Name", "System Version", ""},
		{"", "", "", ""},
	} {
		if got := model(c.vendor, c.name, c.version); got != c.want {
			t.Errorf("model(%q, %q, %q) = %q, want %q", c.vendor, c.name, c.version, got, c.want)
		}
	}
	for in, want := range map[string]string{
		"AMD Ryzen AI 7 PRO 350 w/ Radeon 860M":    "AMD Ryzen AI 7 PRO 350",
		"Intel(R) Core(TM) i7-8565U CPU @ 1.80GHz": "Intel Core i7-8565U",
		"AMD Ryzen 7 5800X 8-Core Processor":       "AMD Ryzen 7 5800X 8-Core",
		"13th Gen Intel(R) Core(TM) i7-1365U":      "13th Gen Intel Core i7-1365U",
	} {
		if got := cpuName(in); got != want {
			t.Errorf("cpuName(%q) = %q, want %q", in, got, want)
		}
	}
}
