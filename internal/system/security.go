package system

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// VulnerableURL is Arch's security tracker: every issue group (AVG) still
// open against a package. Arch stopped publishing advisories (ASAs) in May
// 2025, but the tracker's groups are still kept up.
const VulnerableURL = "https://security.archlinux.org/issues/vulnerable.json"

// trackerURL is where a group's page lives: trackerURL + "AVG-2901".
const trackerURL = "https://security.archlinux.org/"

// Exposure is an open issue group that hits a package on this machine.
type Exposure struct {
	Group    string            // AVG-2901
	Packages map[string]string // the installed ones it names → their version
	Severity string            // Critical, High, Medium, Low, Unknown
	Type     string            // "arbitrary code execution"
	Issues   []string          // CVE ids
	Fixed    string            // the version that fixes it; "" when none yet
}

// Link is the group's page on the tracker.
func (e Exposure) Link() string { return trackerURL + e.Group }

// Names are the installed packages it hits, sorted.
func (e Exposure) Names() []string {
	names := make([]string, 0, len(e.Packages))
	for n := range e.Packages {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

// TrackerGroup is one entry in vulnerable.json.
type TrackerGroup struct {
	Name     string   `json:"name"`
	Packages []string `json:"packages"`
	Severity string   `json:"severity"`
	Type     string   `json:"type"`
	Fixed    *string  `json:"fixed"`
	Issues   []string `json:"issues"`
}

// ParseVulnerable reads the tracker's open groups.
func ParseVulnerable(data []byte) ([]TrackerGroup, error) {
	var groups []TrackerGroup
	if err := json.Unmarshal(data, &groups); err != nil {
		return nil, fmt.Errorf("security tracker: %w", err)
	}
	return groups, nil
}

// ParseInstalled reads `pacman -Q`: name → version.
func ParseInstalled(out []byte) map[string]string {
	pkgs := map[string]string{}
	for _, line := range strings.Split(string(out), "\n") {
		if f := strings.Fields(line); len(f) == 2 {
			pkgs[f[0]] = f[1]
		}
	}
	return pkgs
}

// Vercmp compares two package versions the way pacman does: <0, 0 or >0.
type Vercmp func(ctx context.Context, a, b string) (int, error)

// PacmanVercmp is the real Vercmp: pacman's own vercmp, since epochs and
// pkgrels make version order easy to get subtly wrong.
func PacmanVercmp(run Run) Vercmp {
	return func(ctx context.Context, a, b string) (int, error) {
		out, err := run(ctx, "vercmp", a, b)
		if err != nil {
			return 0, err
		}
		return strconv.Atoi(strings.TrimSpace(string(out)))
	}
}

// Exposed keeps the groups that hit an installed package, dropping those
// whose fix this machine already has (the tracker leaves some open long
// after the fix ships). Worst first, then newest group.
func Exposed(ctx context.Context, groups []TrackerGroup, installed map[string]string, cmp Vercmp) ([]Exposure, error) {
	var out []Exposure
	for _, g := range groups {
		hit := map[string]string{}
		for _, p := range g.Packages {
			v, ok := installed[p]
			if !ok {
				continue
			}
			if g.Fixed != nil && *g.Fixed != "" {
				c, err := cmp(ctx, v, *g.Fixed)
				if err != nil {
					return nil, err
				}
				if c >= 0 {
					continue // already has the fix
				}
			}
			hit[p] = v
		}
		if len(hit) == 0 {
			continue
		}
		e := Exposure{Group: g.Name, Packages: hit, Severity: g.Severity, Type: g.Type, Issues: g.Issues}
		if g.Fixed != nil {
			e.Fixed = *g.Fixed
		}
		out = append(out, e)
	}
	sort.SliceStable(out, func(i, j int) bool {
		a, b := severityRank(out[i].Severity), severityRank(out[j].Severity)
		if a != b {
			return a < b
		}
		return groupNumber(out[i].Group) > groupNumber(out[j].Group)
	})
	return out, nil
}

func severityRank(s string) int {
	switch s {
	case "Critical":
		return 0
	case "High":
		return 1
	case "Medium":
		return 2
	case "Low":
		return 3
	}
	return 4
}

func groupNumber(g string) int {
	n, _ := strconv.Atoi(strings.TrimPrefix(g, "AVG-"))
	return n
}

// FetchExposure gets the tracker's open groups and matches them against
// what's installed.
func FetchExposure(ctx context.Context, get Get, run Run) ([]Exposure, error) {
	body, err := get(ctx, VulnerableURL)
	if err != nil {
		return nil, err
	}
	groups, err := ParseVulnerable(body)
	if err != nil {
		return nil, err
	}
	q, err := run(ctx, "pacman", "-Q")
	if err != nil {
		return nil, err
	}
	return Exposed(ctx, groups, ParseInstalled(q), PacmanVercmp(run))
}
