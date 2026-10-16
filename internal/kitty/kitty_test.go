package kitty

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func env(m map[string]string) func(string) string { return func(k string) string { return m[k] } }

func TestSupported(t *testing.T) {
	for _, tt := range []struct {
		env  map[string]string
		want bool
	}{
		{map[string]string{"TERM": "xterm-kitty"}, true},
		{map[string]string{"KITTY_WINDOW_ID": "3"}, true},
		{map[string]string{"TERM_PROGRAM": "herdr"}, true},
		{map[string]string{"TERM": "xterm-kitty", "TMUX": "/tmp/tmux"}, false},
		{map[string]string{"TERM": "xterm-256color"}, false},
	} {
		if got := Supported(env(tt.env)); got != tt.want {
			t.Errorf("Supported(%v) = %v", tt.env, got)
		}
	}
}

func TestBox(t *testing.T) {
	// 16:9 into 40x20 cells (cells ~twice as tall as wide): width-bound.
	if c, r := Box(1600, 900, 40, 20); c != 40 || r != 11 {
		t.Errorf("Box = %dx%d, want 40x11", c, r)
	}
	if c, r := Box(0, 0, 40, 20); c != 0 || r != 0 {
		t.Errorf("empty image: %dx%d", c, r)
	}
}

func TestPlaceholders(t *testing.T) {
	lines := Placeholders(42, 3, 2, 7)
	if len(lines) != 2 {
		t.Fatalf("rows = %d", len(lines))
	}
	if !strings.HasPrefix(lines[0], "  \x1b[38;5;42m") {
		t.Errorf("want 2 cells of centring then the id as colour: %q", lines[0])
	}
	if n := strings.Count(lines[0], "\U0010EEEE"); n != 3 {
		t.Errorf("placeholder cells = %d, want 3", n)
	}
}

func TestThumbnail(t *testing.T) {
	big := image.NewRGBA(image.Rect(0, 0, 2000, 1000))
	big.Set(1999, 999, color.White)
	small := Thumbnail(big, 400)
	if b := small.Bounds(); b.Dx() != 400 || b.Dy() != 200 {
		t.Errorf("thumbnail = %v", b)
	}
	if b := Thumbnail(image.NewRGBA(image.Rect(0, 0, 100, 50)), 400).Bounds(); b.Dx() != 100 {
		t.Errorf("small images aren't enlarged: %v", b)
	}
}

func TestLoad(t *testing.T) {
	p := filepath.Join(t.TempDir(), "w.png")
	f, _ := os.Create(p)
	png.Encode(f, image.NewRGBA(image.Rect(0, 0, 8, 4)))
	f.Close()
	img, err := Load(p)
	if err != nil || img.Bounds().Dx() != 8 {
		t.Errorf("Load = %v, %v", img, err)
	}
	if _, err := Load(filepath.Join(t.TempDir(), "nope.webp")); err == nil {
		t.Error("want an error")
	}
}

func TestUpload(t *testing.T) {
	seq, err := Upload(image.NewRGBA(image.Rect(0, 0, 4, 4)), 42)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(seq, "\x1b_G") || !strings.Contains(seq, "i=42") {
		t.Errorf("upload = %.40q", seq)
	}
}
