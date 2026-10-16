package system

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"sync"
	"time"
)

// Service is one systemd service or timer, from either manager.
type Service struct {
	Name        string // "sshd.service"
	Description string
	Load        string // loaded, masked, ... ("" when the unit isn't loaded)
	Active      string // active, inactive, failed, activating
	Sub         string // running, dead, exited, waiting
	State       string // the unit file's: enabled, disabled, static, masked
	User        bool   // the --user manager's

	// Timers only.
	Next, Last time.Time // zero when there's none
	Activates  string
}

// Timer reports whether s is a timer rather than a service.
func (s Service) Timer() bool { return strings.HasSuffix(s.Name, ".timer") }

// Failed reports whether s is in the failed state.
func (s Service) Failed() bool { return s.Active == "failed" }

// Running reports whether s is up: a running service or a waiting timer.
func (s Service) Running() bool { return s.Active == "active" }

// Enabled reports whether s starts on its own (at boot or login).
func (s Service) Enabled() bool { return strings.HasPrefix(s.State, "enabled") }

// servicesArgs are the three listings, as JSON, which survives descriptions
// with spaces and timers' multi-column times. Prefix "--user" for the user
// manager.
var servicesArgs = [][]string{
	{"list-units", "--type=service,timer", "--all", "--output=json", "--no-pager"},
	{"list-unit-files", "--type=service,timer", "--output=json", "--no-pager"},
	{"list-timers", "--all", "--output=json", "--no-pager"},
}

// ReadServices lists one manager's services and timers: system, or the
// user's with user set.
func ReadServices(ctx context.Context, run Run, user bool) ([]Service, error) {
	var outs [3][]byte
	var errs [3]error
	var wg sync.WaitGroup
	for i, args := range servicesArgs {
		if user {
			args = append([]string{"--user"}, args...)
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			outs[i], errs[i] = run(ctx, "systemctl", args...)
		}()
	}
	wg.Wait()
	for _, err := range errs {
		if err != nil {
			return nil, err
		}
	}
	return MergeServices(outs[0], outs[1], outs[2], user)
}

type (
	unitJSON struct {
		Unit, Load, Active, Sub, Description string
	}
	unitFileJSON struct {
		UnitFile string `json:"unit_file"`
		State    string
	}
	timerJSON struct {
		Unit, Activates string
		Next, Last      int64 // µs since the epoch; 0 for none
	}
)

// MergeServices joins list-units, list-unit-files and list-timers into one
// list sorted by name. A unit file that's enabled, disabled or masked but
// not loaded (sshd, never started) is listed too: it's one to start or
// enable. Left out: references to units that don't exist, and templates
// (getty@.service), which only their instances can run.
func MergeServices(units, files, timers []byte, user bool) ([]Service, error) {
	var us []unitJSON
	var fs []unitFileJSON
	var ts []timerJSON
	for _, j := range []struct {
		name string
		data []byte
		v    any
	}{{"list-units", units, &us}, {"list-unit-files", files, &fs}, {"list-timers", timers, &ts}} {
		if err := json.Unmarshal(j.data, j.v); err != nil {
			return nil, fmt.Errorf("systemctl %s: %w", j.name, err)
		}
	}

	byName := map[string]*Service{}
	var out []*Service
	add := func(s Service) *Service {
		p := &s
		byName[s.Name] = p
		out = append(out, p)
		return p
	}
	for _, u := range us {
		if u.Load == "not-found" || isTemplate(u.Unit) {
			continue
		}
		add(Service{Name: u.Unit, Description: u.Description, Load: u.Load, Active: u.Active, Sub: u.Sub, User: user})
	}
	for _, f := range fs {
		if s, ok := byName[f.UnitFile]; ok {
			s.State = f.State
			continue
		}
		if isTemplate(f.UnitFile) || !slices.Contains([]string{"enabled", "disabled", "masked"}, f.State) {
			continue
		}
		add(Service{Name: f.UnitFile, Active: "inactive", Sub: "dead", State: f.State, User: user})
	}
	for _, t := range ts {
		if s, ok := byName[t.Unit]; ok {
			s.Next, s.Last, s.Activates = usTime(t.Next), usTime(t.Last), t.Activates
		}
	}

	slices.SortFunc(out, func(a, b *Service) int { return strings.Compare(a.Name, b.Name) })
	list := make([]Service, len(out))
	for i, s := range out {
		list[i] = *s
	}
	return list, nil
}

func isTemplate(name string) bool { return strings.Contains(name, "@.") }

func usTime(us int64) time.Time {
	if us <= 0 {
		return time.Time{}
	}
	return time.UnixMicro(us)
}
