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
	Uptime              time.Duration
	MemTotal, MemAvail  uint64 // bytes
	DiskTotal, DiskFree uint64 // bytes, of /
	Battery             *Battery
}

// Battery is the first battery, if the machine has one.
type Battery struct {
	Percent int
	Status  string // Charging, Discharging, Full, Not charging
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
	}
	if f := strings.Fields(read("proc/uptime")); len(f) > 0 {
		secs, _ := strconv.ParseFloat(f[0], 64)
		si.Uptime = time.Duration(secs) * time.Second
	}
	si.MemTotal, si.MemAvail = meminfo([]byte(read("proc/meminfo")))
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
		break
	}
	return si
}

// meminfo returns MemTotal and MemAvailable in bytes.
func meminfo(b []byte) (total, avail uint64) {
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
		}
	}
	return total, avail
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
