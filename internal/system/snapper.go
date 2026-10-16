package system

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"time"
)

// SnapperConfig is the snapper config Hashiru sets up for /.
const SnapperConfig = "root"

// Snapshot is one snapper snapshot.
type Snapshot struct {
	Number      int
	Date        time.Time
	Type        string // single, pre, post
	Description string
	Cleanup     string // number, timeline, or "" (kept until deleted)
}

// SnapshotsArgs lists the root config as JSON. Listing works without root.
var SnapshotsArgs = []string{"--jsonout", "-c", SnapperConfig, "list", "--columns", "number,date,type,description,cleanup"}

// ParseSnapshots reads `snapper --jsonout list`, newest first, without
// snapshot 0 ("current", the live system).
func ParseSnapshots(out []byte) ([]Snapshot, error) {
	var doc map[string][]struct {
		Number      int    `json:"number"`
		Date        string `json:"date"`
		Type        string `json:"type"`
		Description string `json:"description"`
		Cleanup     string `json:"cleanup"`
	}
	if err := json.Unmarshal(out, &doc); err != nil {
		return nil, fmt.Errorf("snapper: %w", err)
	}
	var snaps []Snapshot
	for _, s := range doc[SnapperConfig] {
		if s.Number == 0 {
			continue
		}
		date, _ := time.ParseInLocation(time.DateTime, s.Date, time.Local)
		snaps = append(snaps, Snapshot{
			Number: s.Number, Date: date, Type: s.Type,
			Description: s.Description, Cleanup: s.Cleanup,
		})
	}
	sort.Slice(snaps, func(i, j int) bool { return snaps[i].Number > snaps[j].Number })
	return snaps, nil
}

// Change is one line of `snapper status`: what happened to a path between a
// snapshot and now.
type Change struct {
	Kind byte // '+' created, '-' deleted, 'c' content, '.' metadata only
	Path string
}

// StatusArgs compares snapshot n with the live system.
func StatusArgs(n int) []string {
	return []string{"-c", SnapperConfig, "status", fmt.Sprintf("%d..0", n)}
}

// ParseChanges reads `snapper status`. Each line is six flag columns
// (content/type, permissions, owner, group, xattrs, ACLs), a space, the path.
func ParseChanges(out []byte) []Change {
	var cs []Change
	sc := bufio.NewScanner(bytes.NewReader(out))
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		line := sc.Text()
		if len(line) < 8 || line[6] != ' ' {
			continue
		}
		cs = append(cs, Change{Kind: line[0], Path: line[7:]})
	}
	return cs
}

// Count tallies changes; a metadata-only change counts as modified.
func Count(cs []Change) (created, deleted, modified int) {
	for _, c := range cs {
		switch c.Kind {
		case '+':
			created++
		case '-':
			deleted++
		default:
			modified++
		}
	}
	return
}
