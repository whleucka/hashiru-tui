package hashiru

import (
	"bufio"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// LogPath is where Hashiru writes its install log (lib/common.sh's
// HASHIRU_LOG), honouring XDG_DATA_HOME the way the bash side does.
func LogPath() string {
	data := os.Getenv("XDG_DATA_HOME")
	if data == "" {
		home, _ := os.UserHomeDir()
		data = filepath.Join(home, ".local", "share")
	}
	return filepath.Join(data, "hashiru", "install.log")
}

// LogLine is one line of the install log. Lines Hashiru wrote itself look like
// "[2026-10-14 15:15:21] [INFO] message"; anything else is raw output from a
// command it ran (pacman, git), with no time and no level.
type LogLine struct {
	Time  time.Time
	Level string // INFO, OK, WARN, ERROR, or "" for raw output
	Text  string
}

// StageRun is one stage within a run, from its "[i/N] Running" line.
type StageRun struct {
	Script   string // e.g. "45-config.sh"
	Done     bool   // reached "[i/N] Completed"
	Duration string // as Hashiru printed it, e.g. "1m 4s"
}

// Outcome is how a run ended.
type Outcome int

const (
	Unfinished Outcome = iota // no completion line and no error: interrupted, or still running
	Succeeded
	Failed
	Loose // not a run: CLI messages logged between runs
)

func (o Outcome) String() string {
	return [...]string{"unfinished", "ok", "failed", "outside a run"}[o]
}

// Run is one install or update: everything from a RUN START marker up to its
// completion line. Lines logged outside a run (a refused `hashiru update`, a
// mistyped flag) are gathered into Loose runs, so they don't make a finished
// run look failed.
type Run struct {
	Start   time.Time
	Lines   []LogLine
	Stages  []StageRun
	Outcome Outcome
	Warns   int
	Errors  int
}

const runMarker = "===== HASHIRU RUN START ====="

var (
	lineRe    = regexp.MustCompile(`^\[(\d{4}-\d\d-\d\d \d\d:\d\d:\d\d)\] \[([A-Z]+)\] (.*)$`)
	runningRe = regexp.MustCompile(`^\[\d+/\d+\] Running: (\S+)`)
	doneRe    = regexp.MustCompile(`^\[\d+/\d+\] Completed: (\S+) \(([^)]*)\)`)
)

// ParseLine splits one log line into its parts.
func ParseLine(s string) LogLine {
	m := lineRe.FindStringSubmatch(s)
	if m == nil {
		return LogLine{Text: s}
	}
	t, err := time.ParseInLocation(time.DateTime, m[1], time.Local)
	if err != nil {
		return LogLine{Text: s}
	}
	return LogLine{Time: t, Level: m[2], Text: m[3]}
}

// ParseLog splits the log into runs, oldest first.
func ParseLog(r io.Reader) ([]Run, error) {
	var runs []Run
	var cur *Run

	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024) // pacman can print long lines
	for sc.Scan() {
		ln := ParseLine(sc.Text())

		switch {
		case ln.Level == "INFO" && ln.Text == runMarker:
			// `hashiru update` logs its pull just before install.sh writes the
			// marker; those lines are this run's opening, not a separate event.
			if cur != nil && cur.Outcome == Loose && !cur.idleBefore(ln) {
				cur.Outcome = Unfinished
				if cur.Errors > 0 {
					cur.Outcome = Failed
				}
				continue
			}
			runs = append(runs, Run{Start: ln.Time})
			cur = &runs[len(runs)-1]
			continue
		case cur == nil || (cur.Outcome == Succeeded || cur.Outcome == Loose) && cur.idleBefore(ln):
			// After a finished run, or apart from earlier loose lines: a new
			// loose entry.
			runs = append(runs, Run{Start: ln.Time, Outcome: Loose})
			cur = &runs[len(runs)-1]
		}
		cur.add(ln)
	}
	return runs, sc.Err()
}

// looseGap is how long after a finished run a line has to come to be a
// separate event. `hashiru update` logs its stamp and "Update complete" a
// second or two after install.sh's own completion line, so those stay in the
// run; a refused update the next day does not.
const looseGap = time.Minute

// idleBefore reports whether ln came long enough after the run's last timed
// line to be unrelated to it. Raw output has no time and never is.
func (r *Run) idleBefore(ln LogLine) bool {
	if ln.Time.IsZero() {
		return false
	}
	for i := len(r.Lines) - 1; i >= 0; i-- {
		if t := r.Lines[i].Time; !t.IsZero() {
			return ln.Time.Sub(t) > looseGap
		}
	}
	return false
}

func (r *Run) add(ln LogLine) {
	if r.Start.IsZero() {
		r.Start = ln.Time
	}
	r.Lines = append(r.Lines, ln)

	switch ln.Level {
	case "WARN":
		r.Warns++
	case "ERROR":
		r.Errors++
		if r.Outcome == Unfinished {
			r.Outcome = Failed
		}
	}

	if m := runningRe.FindStringSubmatch(ln.Text); m != nil {
		r.Stages = append(r.Stages, StageRun{Script: m[1]})
	}
	if m := doneRe.FindStringSubmatch(ln.Text); m != nil {
		for i := range r.Stages {
			if r.Stages[i].Script == m[1] {
				r.Stages[i].Done, r.Stages[i].Duration = true, m[2]
			}
		}
	}
	// install.sh's last word. A stage that failed and was retried still logged
	// an ERROR, but the run as a whole got there.
	if ln.Level == "OK" && strings.HasPrefix(ln.Text, "Hashiru installation complete") {
		r.Outcome = Succeeded
	}
}

// ReadLog parses the log at path.
func ReadLog(path string) ([]Run, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return ParseLog(f)
}
