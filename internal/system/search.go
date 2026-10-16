package system

import (
	"bufio"
	"bytes"
	"context"
	"strings"
	"sync"
)

// searchMax caps AUR results: a short query matches thousands.
const searchMax = 200

// Found is one search result.
type Found struct {
	Repo        string // "extra", "aur"
	Name        string
	Version     string
	Description string
	Installed   bool
}

// AUR reports whether f comes from the AUR (installed with yay) rather than
// a sync repo (pacman).
func (f Found) AUR() bool { return f.Repo == "aur" }

// Search is what a query found, repos and AUR apart, since either can fail
// alone (no network for the AUR, no yay).
type Search struct {
	Query      string
	Repo       []Found
	RepoErr    error
	AUR        []Found
	AURErr     error
	AURMissing bool // no yay
}

// SearchPackages asks pacman (the local copy of the sync databases) and yay
// (the AUR, only: -a) at once.
func SearchPackages(ctx context.Context, run Run, query string) Search {
	s := Search{Query: query}
	words := strings.Fields(query)
	if len(words) == 0 {
		return s
	}
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		out, err := run(ctx, "pacman", append([]string{"-Ss"}, words...)...)
		if code, ok := exitCode(err); ok && code == 1 && len(bytes.TrimSpace(out)) == 0 {
			err = nil // nothing found
		}
		s.Repo, s.RepoErr = ParseSearch(out), err
	}()
	go func() {
		defer wg.Done()
		out, err := run(ctx, "yay", append([]string{"-Ssa"}, words...)...)
		switch code, ok := exitCode(err); {
		case missing(err):
			s.AURMissing, err = true, nil
		case ok && code == 1 && len(bytes.TrimSpace(out)) == 0:
			err = nil
		}
		s.AUR, s.AURErr = ParseSearch(out), err
		if len(s.AUR) > searchMax {
			s.AUR = s.AUR[:searchMax]
		}
	}()
	wg.Wait()
	return s
}

// ParseSearch reads pacman -Ss and yay -Ss output: a "repo/name version ..."
// line, then the description indented under it. pacman marks installed
// packages "[installed]" (or "[installed: 1.0]" when older); yay says
// "(Installed)".
func ParseSearch(out []byte) []Found {
	var found []Found
	sc := bufio.NewScanner(bytes.NewReader(out))
	for sc.Scan() {
		line := sc.Text()
		if strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t") {
			if n := len(found); n > 0 && found[n-1].Description == "" {
				found[n-1].Description = strings.TrimSpace(line)
			}
			continue
		}
		f := strings.Fields(line)
		if len(f) < 2 {
			continue
		}
		repo, name, ok := strings.Cut(f[0], "/")
		if !ok {
			continue
		}
		rest := strings.Join(f[2:], " ")
		found = append(found, Found{
			Repo:      repo,
			Name:      name,
			Version:   f[1],
			Installed: strings.Contains(rest, "[installed") || strings.Contains(rest, "(Installed"),
		})
	}
	return found
}
