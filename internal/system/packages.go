package system

import (
	"bufio"
	"bytes"
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"sync"
)

// PacmanCache is where pacman keeps downloaded packages.
const PacmanCache = "/var/cache/pacman/pkg"

// ManifestEntry is one line of a Hashiru package manifest, pacman/<file>.txt.
type ManifestEntry struct {
	Name string
	File string // "apps.txt"
	AUR  bool   // from aur.txt, installed with yay
}

// ParseManifest reads one manifest the way Hashiru's install_packages does:
// a package per line, skipping blanks and lines starting with #.
func ParseManifest(file string, data []byte) []ManifestEntry {
	var out []ManifestEntry
	sc := bufio.NewScanner(bytes.NewReader(data))
	for sc.Scan() {
		l := strings.TrimSpace(sc.Text())
		if l == "" || strings.HasPrefix(l, "#") {
			continue
		}
		out = append(out, ManifestEntry{Name: l, File: file, AUR: file == "aur.txt"})
	}
	return out
}

// ReadManifests reads every pacman/*.txt in a Hashiru checkout, in name order.
func ReadManifests(repo string) ([]ManifestEntry, error) {
	files, err := filepath.Glob(filepath.Join(repo, "pacman", "*.txt"))
	if err != nil {
		return nil, err
	}
	if len(files) == 0 {
		return nil, fs.ErrNotExist
	}
	sort.Strings(files)
	var out []ManifestEntry
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			return nil, err
		}
		out = append(out, ParseManifest(filepath.Base(f), data)...)
	}
	return out, nil
}

// Packages is the machine's packages as Hashiru sees them.
type Packages struct {
	Managed []ManifestEntry // in a manifest and installed
	Missing []ManifestEntry // in a manifest, not installed: drift
	Yours   []string        // explicitly installed, in no manifest
	Orphans []string        // dependencies nothing needs any more
	Foreign []string        // not from a sync repo: AUR, or built by hand

	ManifestErr error // no repo, or no manifests in it
	PacmanErr   error // pacman missing or failing; nothing else is known

	CacheBytes int64
	CacheErr   error
}

// PacmanQuery is the local package database, as the lists pacman prints.
type PacmanQuery struct {
	Installed []string            // -Qq: everything, dependencies too
	Explicit  []string            // -Qqe
	Foreign   []string            // -Qqm
	Orphans   []string            // -Qdtq
	Groups    map[string][]string // -Sgq, for manifest names that aren't packages
}

// Classify sorts the machine's packages against Hashiru's manifests. A
// manifest name that's a group counts as installed when all its members
// are, and its members count as Hashiru's.
func Classify(manifests []ManifestEntry, q PacmanQuery) Packages {
	installed := set(q.Installed)
	owned := map[string]bool{}
	var p Packages
	for _, e := range manifests {
		members, isGroup := q.Groups[e.Name]
		switch {
		case installed[e.Name]:
			owned[e.Name] = true
			p.Managed = append(p.Managed, e)
		case isGroup && len(members) > 0 && all(members, installed):
			for _, m := range members {
				owned[m] = true
			}
			p.Managed = append(p.Managed, e)
		default:
			p.Missing = append(p.Missing, e)
		}
	}
	for _, name := range q.Explicit {
		if !owned[name] {
			p.Yours = append(p.Yours, name)
		}
	}
	p.Orphans, p.Foreign = q.Orphans, q.Foreign
	return p
}

// ReadPackages asks pacman about the local database (no root, no network)
// and sorts it against the manifests in repo. cache is sized too, normally
// PacmanCache.
func ReadPackages(ctx context.Context, run Run, repo, cache string) Packages {
	var q PacmanQuery
	var errs [4]error
	var wg sync.WaitGroup
	for i, c := range []struct {
		dst  *[]string
		args []string
	}{
		{&q.Installed, []string{"-Qq"}},
		{&q.Explicit, []string{"-Qqe"}},
		{&q.Foreign, []string{"-Qqm"}},
		{&q.Orphans, []string{"-Qdtq"}},
	} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			*c.dst, errs[i] = pacmanList(ctx, run, c.args...)
		}()
	}

	var p Packages
	var manifests []ManifestEntry
	wg.Add(1)
	go func() {
		defer wg.Done()
		p.CacheBytes, p.CacheErr = dirSize(cache)
	}()
	if repo == "" {
		p.ManifestErr = fs.ErrNotExist
	} else {
		manifests, p.ManifestErr = ReadManifests(repo)
	}
	wg.Wait()

	for _, err := range errs {
		if err != nil {
			return Packages{PacmanErr: err, ManifestErr: p.ManifestErr, CacheBytes: p.CacheBytes, CacheErr: p.CacheErr}
		}
	}

	// Only names that aren't installed can be groups worth expanding, and
	// there are normally none, so this is usually no calls at all.
	installed := set(q.Installed)
	q.Groups = map[string][]string{}
	for _, e := range manifests {
		if !installed[e.Name] {
			if members, err := pacmanList(ctx, run, "-Sgq", e.Name); err == nil && len(members) > 0 {
				q.Groups[e.Name] = members
			}
		}
	}

	c := Classify(manifests, q)
	c.ManifestErr, c.CacheBytes, c.CacheErr = p.ManifestErr, p.CacheBytes, p.CacheErr
	return c
}

// pacmanList runs a pacman query that prints a name per line. pacman exits
// 1 with nothing printed when nothing matches (no orphans, no foreign
// packages), which is an empty list, not an error.
func pacmanList(ctx context.Context, run Run, args ...string) ([]string, error) {
	out, err := run(ctx, "pacman", args...)
	if code, ok := exitCode(err); ok && code == 1 && len(bytes.TrimSpace(out)) == 0 {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return strings.Fields(string(out)), nil
}

// dirSize adds up the regular files directly in dir. pacman's cache is flat.
func dirSize(dir string) (int64, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0, err
	}
	var n int64
	for _, e := range entries {
		if !e.Type().IsRegular() {
			continue
		}
		if info, err := e.Info(); err == nil {
			n += info.Size()
		}
	}
	return n, nil
}

func set(names []string) map[string]bool {
	s := make(map[string]bool, len(names))
	for _, n := range names {
		s[n] = true
	}
	return s
}

func all(names []string, in map[string]bool) bool {
	return !slices.ContainsFunc(names, func(n string) bool { return !in[n] })
}
