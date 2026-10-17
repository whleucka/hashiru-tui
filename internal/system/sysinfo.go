package system

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// SysInfo is the machine at a glance, read straight from /proc and /sys.
type SysInfo struct {
	Host, Kernel        string
	Model               string // "ThinkPad T14s Gen 6", from DMI
	CPU                 string // "AMD Ryzen AI 7 PRO 350"
	Threads             int
	Uptime              time.Duration
	MemTotal, MemAvail  uint64 // bytes
	SwapTotal, SwapFree uint64
	DiskTotal, DiskFree uint64 // bytes, of /
	Battery             *Battery
}

// Battery is the first battery, if the machine has one.
type Battery struct {
	Percent int
	Status  string  // Charging, Discharging, Full, Not charging
	Health  float64 // full charge over design, 0 when it doesn't say
}

// Statfs reports a filesystem's size and the space available to users.
type Statfs func(path string) (total, free uint64, err error)

// RootStatfs is the real Statfs.
func RootStatfs(path string) (uint64, uint64, error) {
	var st syscall.Statfs_t
	if err := syscall.Statfs(path, &st); err != nil {
		return 0, 0, err
	}
	bs := uint64(st.Bsize)
	return st.Blocks * bs, st.Bavail * bs, nil
}

// ReadSysInfo reads under root ("/" for real; a fixture tree in tests).
// Anything unreadable is left zero rather than failing the rest.
func ReadSysInfo(root string, statfs Statfs) SysInfo {
	read := func(p string) string {
		b, _ := os.ReadFile(filepath.Join(root, p))
		return strings.TrimSpace(string(b))
	}
	si := SysInfo{
		Host:   read("proc/sys/kernel/hostname"),
		Kernel: read("proc/sys/kernel/osrelease"),
		Model:  model(read("sys/class/dmi/id/sys_vendor"), read("sys/class/dmi/id/product_name"), read("sys/class/dmi/id/product_version")),
	}
	si.CPU, si.Threads = cpuinfo([]byte(read("proc/cpuinfo")))
	if f := strings.Fields(read("proc/uptime")); len(f) > 0 {
		secs, _ := strconv.ParseFloat(f[0], 64)
		si.Uptime = time.Duration(secs) * time.Second
	}
	si.MemTotal, si.MemAvail, si.SwapTotal, si.SwapFree = meminfo([]byte(read("proc/meminfo")))
	si.DiskTotal, si.DiskFree, _ = statfs("/")

	supplies, _ := filepath.Glob(filepath.Join(root, "sys/class/power_supply/*"))
	for _, dir := range supplies {
		rel, _ := filepath.Rel(root, dir)
		if read(filepath.Join(rel, "type")) != "Battery" {
			continue
		}
		pct, err := strconv.Atoi(read(filepath.Join(rel, "capacity")))
		if err != nil {
			continue
		}
		si.Battery = &Battery{Percent: pct, Status: read(filepath.Join(rel, "status"))}
		for _, b := range ReadBatteries(root) {
			if b.Name == filepath.Base(dir) {
				si.Battery.Health = b.Health
			}
		}
		break
	}
	return si
}

// model is the machine's name as it's sold. Lenovo puts that in
// product_version ("ThinkPad T14s Gen 6") and a part number in
// product_name; everyone else uses product_name. Board makers' placeholders
// mean there isn't one.
func model(vendor, name, version string) string {
	m := name
	if strings.EqualFold(vendor, "LENOVO") && version != "" {
		m = version
	}
	switch strings.ToLower(m) {
	case "", "none", "default string", "to be filled by o.e.m.", "system product name":
		return ""
	}
	return m
}

// cpuinfo is the CPU's name, without the trademarks, clock and built-in
// graphics people don't say, and how many threads it runs.
func cpuinfo(b []byte) (name string, threads int) {
	sc := bufio.NewScanner(bytes.NewReader(b))
	for sc.Scan() {
		k, v, ok := strings.Cut(sc.Text(), ":")
		if !ok {
			continue
		}
		switch strings.TrimSpace(k) {
		case "processor":
			threads++
		case "model name":
			if name == "" {
				name = cpuName(strings.TrimSpace(v))
			}
		}
	}
	return name, threads
}

func cpuName(s string) string {
	for _, cut := range []string{" w/ ", " @ ", " with "} {
		s, _, _ = strings.Cut(s, cut)
	}
	for _, junk := range []string{"(R)", "(TM)", "(tm)", " CPU", " Processor", " processor"} {
		s = strings.ReplaceAll(s, junk, "")
	}
	return strings.Join(strings.Fields(s), " ")
}

// meminfo returns MemTotal, MemAvailable, SwapTotal and SwapFree in bytes.
func meminfo(b []byte) (total, avail, swapTotal, swapFree uint64) {
	sc := bufio.NewScanner(bytes.NewReader(b))
	for sc.Scan() {
		f := strings.Fields(sc.Text())
		if len(f) < 2 {
			continue
		}
		kb, _ := strconv.ParseUint(f[1], 10, 64)
		switch f[0] {
		case "MemTotal:":
			total = kb * 1024
		case "MemAvailable:":
			avail = kb * 1024
		case "SwapTotal:":
			swapTotal = kb * 1024
		case "SwapFree:":
			swapFree = kb * 1024
		}
	}
	return total, avail, swapTotal, swapFree
}

// HumanBytes formats bytes in binary units: "30.5 GiB".
func HumanBytes(n uint64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := uint64(unit), 0
	for m := n / unit; m >= unit && exp < 3; m /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(n)/float64(div), "KMGT"[exp])
}
