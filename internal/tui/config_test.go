package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

func configModel(t *testing.T, opts Options) (*Model, string) {
	t.Helper()
	home := t.TempDir()
	if err := os.MkdirAll(filepath.Join(home, ".config/hashiru/hypr"), 0o755); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(home, ".config/hashiru/hypr/keybinds.extra.lua"), nil, 0o644)
	opts.Source = fakeSource{}
	opts.Home = home
	m := New(opts)
	load(t, m)
	goTo(t, m, "Config")
	return m, home
}

func TestConfigListsTiersAndMarksExisting(t *testing.T) {
	m, _ := configModel(t, Options{})
	out := ansi.Strip(m.View().Content)
	for _, want := range []string{"hashiru.conf", "Hyprland", "keybinds.extra.lua", "local.lua", "kitty", "style.css", ".zshrc.local"} {
		if !strings.Contains(out, want) {
			t.Errorf("Config missing %q:\n%s", want, out)
		}
	}
	for _, line := range strings.Split(out, "\n") {
		if strings.Contains(line, "keybinds.extra.lua") && !strings.Contains(line, "●") {
			t.Errorf("existing file not marked: %q", line)
		}
		if strings.Contains(line, "autostart.extra.lua") && strings.Contains(line, "●") {
			t.Errorf("missing file marked: %q", line)
		}
	}
}

func TestConfigOpensEditor(t *testing.T) {
	t.Setenv("EDITOR", "nvim")
	var ran []string
	m, home := configModel(t, Options{Exec: recordExec(&ran)})

	press(m, "enter") // the first entry is hashiru.conf, through the CLI
	if len(ran) != 1 || ran[0] != "hashiru config" {
		t.Fatalf("ran %v", ran)
	}

	c := m.area().(*configArea)
	for i, e := range c.entries {
		if strings.HasSuffix(e.path, "waybar/style.css") {
			c.sel = i
		}
	}
	press(m, "enter")
	want := "nvim " + filepath.Join(home, ".config/hashiru/waybar/style.css")
	if len(ran) != 2 || ran[1] != want {
		t.Errorf("ran %v, want %q", ran, want)
	}
	if _, err := os.Stat(filepath.Join(home, ".config/hashiru/waybar")); err != nil {
		t.Error("the directory should exist for the editor to save into")
	}
}
