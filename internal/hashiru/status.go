// Package hashiru reads the state of a Hashiru machine through the hashiru CLI.
//
// It never reimplements what the CLI does. Everything comes from the two
// machine-readable commands, `hashiru status --porcelain` and
// `hashiru doctor --json`, so the bash side stays the single source of truth.
package hashiru

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"
)

// Status is `hashiru status --porcelain`.
type Status struct {
	Version   string
	Commit    string // what /etc/hashiru-release says was installed
	Head      string // what the checkout is at now
	Installed time.Time
	Updated   time.Time // zero if never updated
	Repo      string
	Dirty     bool // tracked changes; `hashiru update` refuses while true
	Untracked int
	Behind    int  // commits behind upstream; only meaningful if BehindOK
	BehindOK  bool // false when there is no upstream or no network
}

// ParseStatus reads key=value lines. Unknown keys are ignored, so a newer
// hashiru that adds keys doesn't break an older TUI.
func ParseStatus(r io.Reader) (Status, error) {
	var s Status
	sc := bufio.NewScanner(r)
	for sc.Scan() {
		key, value, ok := strings.Cut(sc.Text(), "=")
		if !ok {
			continue
		}
		var err error
		switch key {
		case "version":
			s.Version = value
		case "commit":
			s.Commit = value
		case "head":
			s.Head = value
		case "repo":
			s.Repo = value
		case "installed":
			s.Installed, err = parseTime(value)
		case "updated":
			s.Updated, err = parseTime(value)
		case "dirty":
			s.Dirty = value == "1"
		case "untracked":
			s.Untracked, err = parseInt(value)
		case "behind":
			if value != "" {
				s.Behind, err = parseInt(value)
				s.BehindOK = err == nil
			}
		}
		if err != nil {
			return s, fmt.Errorf("status: %s: %w", key, err)
		}
	}
	return s, sc.Err()
}

// parseTime accepts an empty value as "unknown".
func parseTime(v string) (time.Time, error) {
	if v == "" {
		return time.Time{}, nil
	}
	return time.Parse(time.RFC3339, v)
}

func parseInt(v string) (int, error) {
	if v == "" {
		return 0, nil
	}
	return strconv.Atoi(v)
}
