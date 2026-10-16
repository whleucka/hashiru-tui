package tui

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// recentMax is how many actions the palette remembers.
const recentMax = 10

// defaultRecentPath is $XDG_STATE_HOME/hashiru-tui/recent, or under
// ~/.local/state when that's unset.
func defaultRecentPath(home string) string {
	dir := os.Getenv("XDG_STATE_HOME")
	if dir == "" {
		dir = filepath.Join(home, ".local", "state")
	}
	return filepath.Join(dir, "hashiru-tui", "recent")
}

// loadRecent reads the IDs of recently run actions, newest first. A missing
// or unreadable file is no recents, never an error: they're a convenience.
func loadRecent(path string) []string {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	return strings.Fields(string(b))
}

// addRecent puts id first and saves the list. Saving is best effort for the
// same reason loading is.
func addRecent(path string, ids []string, id string) []string {
	ids = slices.DeleteFunc(slices.Clone(ids), func(s string) bool { return s == id })
	ids = append([]string{id}, ids...)
	if len(ids) > recentMax {
		ids = ids[:recentMax]
	}
	if path != "" {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err == nil {
			_ = os.WriteFile(path, []byte(strings.Join(ids, "\n")+"\n"), 0o644)
		}
	}
	return ids
}
