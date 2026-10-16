package system

import (
	"bufio"
	"bytes"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

var imageExts = map[string]bool{".jpg": true, ".jpeg": true, ".png": true, ".webp": true, ".gif": true}

// ListImages lists the images in dirs, each directory's sorted by name. A
// directory that doesn't exist is skipped.
func ListImages(dirs []string) []string {
	var out []string
	for _, d := range dirs {
		entries, err := os.ReadDir(d)
		if err != nil {
			continue
		}
		var names []string
		for _, e := range entries {
			if !e.IsDir() && imageExts[strings.ToLower(filepath.Ext(e.Name()))] {
				names = append(names, e.Name())
			}
		}
		sort.Strings(names)
		for _, n := range names {
			out = append(out, filepath.Join(d, n))
		}
	}
	return out
}

// ParseAwwwQuery reads `awww query` and returns the image on each output
// that shows one: ": DP-9: 2560x1440, scale: 1, currently displaying:
// image: /path".
func ParseAwwwQuery(out []byte) []string {
	var paths []string
	sc := bufio.NewScanner(bytes.NewReader(out))
	for sc.Scan() {
		if _, p, ok := strings.Cut(sc.Text(), "currently displaying: image: "); ok {
			paths = append(paths, strings.TrimSpace(p))
		}
	}
	return paths
}

// choicePath is where a fixed wallpaper is recorded. Hashiru's
// tahoe-wallpaper daemon reads it: when it names a file, the daemon shows
// that and stops cycling through the time-of-day set.
func choicePath(home string) string {
	return filepath.Join(home, ".config", "hashiru", "wallpaper")
}

// ReadChoice is the fixed wallpaper, or "" when cycling.
func ReadChoice(home string) string {
	b, err := os.ReadFile(choicePath(home))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}

// WriteChoice fixes the wallpaper to path.
func WriteChoice(home, path string) error {
	p := choicePath(home)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	return os.WriteFile(p, []byte(path+"\n"), 0o644)
}

// ClearChoice goes back to the time-of-day cycle.
func ClearChoice(home string) error {
	err := os.Remove(choicePath(home))
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	return err
}
