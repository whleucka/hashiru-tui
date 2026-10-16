package system

import (
	"bufio"
	"bytes"
	"context"
	"strings"
	"sync"
)

// Package is one pending upgrade.
type Package struct {
	Name, Old, New string
}

// Updates is what's waiting to be installed.
type Updates struct {
	Pacman    []Package
	PacmanErr error

	AUR        []Package
	AURErr     error
	AURMissing bool // no yay
}

// Pending asks checkupdates and yay, in parallel. checkupdates syncs a copy
// of the databases in a temporary directory, so it needs no root and leaves
// pacman's own databases alone.
func Pending(ctx context.Context, run Run) Updates {
	var u Updates
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		out, err := run(ctx, "checkupdates")
		if code, ok := exitCode(err); ok && code == 2 {
			err = nil // 2 is "nothing to update"
		}
		u.Pacman, u.PacmanErr = ParseUpdates(out), err
	}()
	go func() {
		defer wg.Done()
		out, err := run(ctx, "yay", "-Qua")
		switch code, ok := exitCode(err); {
		case missing(err):
			u.AURMissing, err = true, nil
		case ok && code == 1 && len(bytes.TrimSpace(out)) == 0:
			err = nil // yay exits 1 when there's nothing to list
		}
		u.AUR, u.AURErr = ParseUpdates(out), err
	}()
	wg.Wait()
	return u
}

// ParseUpdates reads "name old -> new" lines, as checkupdates and yay -Qua
// print them. Anything after the new version (yay's "[3d14h]") is ignored.
func ParseUpdates(out []byte) []Package {
	var pkgs []Package
	sc := bufio.NewScanner(bytes.NewReader(out))
	for sc.Scan() {
		f := strings.Fields(sc.Text())
		if len(f) < 4 || f[2] != "->" {
			continue
		}
		pkgs = append(pkgs, Package{Name: f[0], Old: f[1], New: f[3]})
	}
	return pkgs
}
