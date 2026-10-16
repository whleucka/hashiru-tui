package system

import (
	"bufio"
	"bytes"
	"strings"
	"time"
)

// JournalLine is one journal entry in short-iso form.
type JournalLine struct {
	Time    time.Time
	Ident   string // hyprlock, kernel, systemd
	Message string
}

// JournalErrorsArgs is this boot's errors and worse, the latest 200.
var JournalErrorsArgs = []string{"-p", "err", "-b", "-o", "short-iso", "--no-pager", "-n", "200"}

// ParseJournal reads `journalctl -o short-iso`: "TIME HOST IDENT[PID]: MSG".
func ParseJournal(out []byte) []JournalLine {
	var lines []JournalLine
	sc := bufio.NewScanner(bytes.NewReader(out))
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		f := strings.SplitN(sc.Text(), " ", 4)
		if len(f) < 4 {
			continue
		}
		t, err := time.Parse("2006-01-02T15:04:05-07:00", f[0])
		if err != nil {
			continue
		}
		ident, msg, ok := strings.Cut(f[2]+" "+f[3], ": ")
		if !ok {
			continue
		}
		if i := strings.IndexByte(ident, '['); i >= 0 {
			ident = ident[:i]
		}
		lines = append(lines, JournalLine{Time: t, Ident: ident, Message: msg})
	}
	return lines
}

// Unit is a failed systemd unit.
type Unit struct {
	Name, Description string
	User              bool // a --user unit
}

// FailedArgs lists failed units without decoration; add "--user" first for
// the user manager.
var FailedArgs = []string{"--failed", "--plain", "--no-legend"}

// ParseFailed reads `systemctl --failed --plain --no-legend`:
// "UNIT LOAD ACTIVE SUB DESCRIPTION".
func ParseFailed(out []byte, user bool) []Unit {
	var units []Unit
	sc := bufio.NewScanner(bytes.NewReader(out))
	for sc.Scan() {
		f := strings.Fields(sc.Text())
		if len(f) < 4 {
			continue
		}
		units = append(units, Unit{Name: f[0], Description: strings.Join(f[4:], " "), User: user})
	}
	return units
}
