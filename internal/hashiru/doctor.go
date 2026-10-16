package hashiru

import (
	"encoding/json"
	"fmt"
	"io"
)

// Level is how a doctor check came out.
type Level string

const (
	OK   Level = "ok"
	Warn Level = "warn"
	Fail Level = "fail"
	Skip Level = "skip" // the machine opted out; neither good nor bad
)

// Check is one line of doctor output.
type Check struct {
	Section string `json:"section"`
	Status  Level  `json:"status"`
	Message string `json:"message"`
}

// Report is `hashiru doctor --json`.
type Report struct {
	Checks  []Check `json:"checks"`
	Summary struct {
		OK   int `json:"ok"`
		Warn int `json:"warn"`
		Fail int `json:"fail"`
	} `json:"summary"`
}

// Section is a run of checks under one heading, in doctor's order.
type Section struct {
	Name   string
	Checks []Check
}

// Worst is the most serious level in the section: fail, then warn, then ok.
func (s Section) Worst() Level {
	worst := OK
	for _, c := range s.Checks {
		switch c.Status {
		case Fail:
			return Fail
		case Warn:
			worst = Warn
		}
	}
	return worst
}

// Sections groups the checks by section, keeping doctor's order.
func (r Report) Sections() []Section {
	var out []Section
	for _, c := range r.Checks {
		if n := len(out); n == 0 || out[n-1].Name != c.Section {
			out = append(out, Section{Name: c.Section})
		}
		last := &out[len(out)-1]
		last.Checks = append(last.Checks, c)
	}
	return out
}

// ParseDoctor decodes doctor's JSON.
func ParseDoctor(r io.Reader) (Report, error) {
	var rep Report
	if err := json.NewDecoder(r).Decode(&rep); err != nil {
		return rep, fmt.Errorf("doctor: %w", err)
	}
	return rep, nil
}
