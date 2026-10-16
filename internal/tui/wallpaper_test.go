package tui

import (
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/whleucka/hashiru-tui/internal/system"
)

func wallModel(t *testing.T, opts Options) (*Model, string, *[]string) {
	t.Helper()
	home := t.TempDir()
	dir := filepath.Join(home, ".config/hypr/wallpapers")
	os.MkdirAll(dir, 0o755)
	for _, n := range []string{"Dawn.png", "Night.png"} {
		f, _ := os.Create(filepath.Join(dir, n))
		png.Encode(f, image.NewRGBA(image.Rect(0, 0, 16, 9)))
		f.Close()
	}
	var opened []string
	opts.Source = fakeSource{}
	opts.Home = home
	opts.Probe = probe{"awww": ": DP-9: 2560x1440, scale: 1, currently displaying: image: " + filepath.Join(dir, "Night.png") + "\n"}.run
	opts.Detach = func(argv []string) error { opened = append(opened, strings.Join(argv, " ")); return nil }
	m := New(opts)
	load(t, m)
	goTo(t, m, "Wallpaper")
	return m, home, &opened
}

func TestWallpaperListAndCurrent(t *testing.T) {
	m, _, _ := wallModel(t, Options{})
	out := ansi.Strip(m.View().Content)
	if !strings.Contains(out, "Dawn") || !strings.Contains(out, "time-of-day cycle") {
		t.Errorf("view:\n%s", out)
	}
	for _, line := range strings.Split(out, "\n") {
		if strings.Contains(line, "Night") && !strings.Contains(line, "●") {
			t.Errorf("the current wallpaper should be marked: %q", line)
		}
	}
}

func TestWallpaperFixAndCycle(t *testing.T) {
	m, home, opened := wallModel(t, Options{})
	dawn := filepath.Join(home, ".config/hypr/wallpapers/Dawn.png")

	press(m, "enter") // Dawn is first
	if got := system.ReadChoice(home); got != dawn {
		t.Errorf("choice = %q, want %q", got, dawn)
	}
	if len(*opened) != 1 || !strings.HasPrefix((*opened)[0], "awww img "+dawn) {
		t.Errorf("opened %v", *opened)
	}
	if out := ansi.Strip(m.View().Content); !strings.Contains(out, "fixed") {
		t.Errorf("mode should say fixed:\n%s", out)
	}

	press(m, "c")
	if got := system.ReadChoice(home); got != "" {
		t.Errorf("c should clear the choice, got %q", got)
	}
	if len(*opened) != 2 || (*opened)[1] != filepath.Join(home, ".config/hypr/scripts/tahoe-wallpaper") {
		t.Errorf("opened %v", *opened)
	}
}

func TestWallpaperPreview(t *testing.T) {
	m, _, _ := wallModel(t, Options{Graphics: graphicsOn})
	if out := m.View().Content; !strings.Contains(out, "\U0010EEEE") {
		t.Errorf("want kitty placeholders for the preview:\n%s", ansi.Strip(out))
	}
}

func TestWallpaperCurrentThroughSymlink(t *testing.T) {
	// Hashiru stows ~/.config/hypr/wallpapers as a symlink into its checkout,
	// and awww reports the resolved path.
	home, repo := t.TempDir(), t.TempDir()
	os.WriteFile(filepath.Join(repo, "Night.jpg"), nil, 0o644)
	os.MkdirAll(filepath.Join(home, ".config/hypr"), 0o755)
	os.Symlink(repo, filepath.Join(home, ".config/hypr/wallpapers"))

	m := New(Options{
		Source: fakeSource{},
		Home:   home,
		Probe:  probe{"awww": ": DP-9: 2560x1440, scale: 1, currently displaying: image: " + filepath.Join(repo, "Night.jpg") + "\n"}.run,
	})
	load(t, m)
	goTo(t, m, "Wallpaper")
	for _, line := range strings.Split(ansi.Strip(m.View().Content), "\n") {
		if strings.Contains(line, "Night") && !strings.Contains(line, "●") {
			t.Errorf("current wallpaper not marked through the symlink: %q", line)
		}
	}
}

func TestWallpaperPreviewRowsAlign(t *testing.T) {
	// Every placeholder row must start in the same column, beside a name or
	// not: kitty assembles the picture from the cells' positions.
	m, _, _ := wallModel(t, Options{Graphics: graphicsOn})
	col := -1
	rows := 0
	for _, line := range strings.Split(m.View().Content, "\n") {
		i := strings.Index(line, "\U0010EEEE")
		if i < 0 {
			continue
		}
		rows++
		w := ansi.StringWidth(line[:i])
		if col == -1 {
			col = w
		} else if w != col {
			t.Errorf("placeholder row %d starts at column %d, want %d", rows, w, col)
		}
	}
	if rows < 3 {
		t.Fatalf("only %d preview rows; the test needs rows beside and below the list", rows)
	}
}
