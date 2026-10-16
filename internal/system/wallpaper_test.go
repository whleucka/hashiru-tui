package system

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseAwwwQuery(t *testing.T) {
	got := ParseAwwwQuery(fixture(t, "awww-query.txt"))
	want := "/opt/hashiru/stow/hyprland/.config/hypr/wallpapers/26-Tahoe-Beach-Night.jpg"
	if len(got) != 1 || got[0] != want {
		t.Errorf("current = %q, want [%q]", got, want)
	}
	two := ": eDP-1: 2880x1800, scale: 2, currently displaying: image: /a.jpg\n: DP-9: 2560x1440, scale: 1, currently displaying: color: 000000\n"
	if got := ParseAwwwQuery([]byte(two)); len(got) != 1 || got[0] != "/a.jpg" {
		t.Errorf("a colour isn't an image: %q", got)
	}
}

func TestListImages(t *testing.T) {
	a, b := t.TempDir(), t.TempDir()
	for _, f := range []string{"b.jpg", "a.PNG", "notes.txt", "c.webp"} {
		os.WriteFile(filepath.Join(a, f), nil, 0o644)
	}
	os.WriteFile(filepath.Join(b, "d.jpeg"), nil, 0o644)

	got := ListImages([]string{a, b, filepath.Join(a, "missing")})
	want := []string{filepath.Join(a, "a.PNG"), filepath.Join(a, "b.jpg"), filepath.Join(a, "c.webp"), filepath.Join(b, "d.jpeg")}
	if len(got) != len(want) {
		t.Fatalf("got %q", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("%d: %q, want %q", i, got[i], want[i])
		}
	}
}

func TestChoice(t *testing.T) {
	home := t.TempDir()
	if got := ReadChoice(home); got != "" {
		t.Errorf("no choice yet: %q", got)
	}
	if err := WriteChoice(home, "/w/sunset.jpg"); err != nil {
		t.Fatal(err)
	}
	if got := ReadChoice(home); got != "/w/sunset.jpg" {
		t.Errorf("choice = %q", got)
	}
	if err := ClearChoice(home); err != nil {
		t.Fatal(err)
	}
	if got := ReadChoice(home); got != "" {
		t.Errorf("after clear: %q", got)
	}
	if err := ClearChoice(home); err != nil {
		t.Errorf("clearing twice: %v", err)
	}
}

func TestChoiceHonoursXDGConfigHome(t *testing.T) {
	// Hashiru's lib/common.sh: HASHIRU_CONFIG_DIR="${XDG_CONFIG_HOME:-$HOME/.config}/hashiru"
	cfg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", cfg)
	if err := WriteChoice(t.TempDir(), "/w/a.jpg"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(cfg, "hashiru", "wallpaper")); err != nil {
		t.Errorf("choice not under $XDG_CONFIG_HOME/hashiru: %v", err)
	}
	if got := HashiruConfigDir("/home/x"); got != filepath.Join(cfg, "hashiru") {
		t.Errorf("HashiruConfigDir = %q", got)
	}
	t.Setenv("XDG_CONFIG_HOME", "")
	if got := HashiruConfigDir("/home/x"); got != "/home/x/.config/hashiru" {
		t.Errorf("HashiruConfigDir without XDG = %q", got)
	}
}
