package system

import (
	"strings"
	"testing"
)

func TestParseBinds(t *testing.T) {
	binds := ParseBinds(fixture(t, "hyprctl-binds.txt"))
	if len(binds) < 60 {
		t.Fatalf("only %d binds", len(binds))
	}

	find := func(desc string) *Bind {
		for i := range binds {
			if binds[i].Description == desc {
				return &binds[i]
			}
		}
		return nil
	}
	for desc, want := range map[string]string{
		"Show keybinds":         "Super+/",
		"System monitor (btop)": "Super+Shift+T",
		"Workspace 1":           "Super+1",
		"Dismiss notification":  "Super+,",
	} {
		b := find(desc)
		if b == nil {
			t.Errorf("no bind %q", desc)
			continue
		}
		if got := b.Combo(); got != want {
			t.Errorf("%q: %s, want %s", desc, got, want)
		}
	}

	for _, b := range binds {
		if b.Description == "" {
			t.Errorf("undescribed bind kept: %+v", b)
		}
	}
}

func TestParseBindsSkipsSubmaps(t *testing.T) {
	in := "bindd\n\tmodmask: 0\n\tsubmap: resize\n\tkey: H\n\tdescription: Shrink\n\n" +
		"bindd\n\tmodmask: 72\n\tsubmap: \n\tkey: Space\n\tdescription: System menu\n"
	binds := ParseBinds([]byte(in))
	if len(binds) != 1 || binds[0].Combo() != "Super+Alt+Space" {
		t.Errorf("binds = %+v", binds)
	}
}

func TestMods(t *testing.T) {
	if got := mods(64 + 4 + 8 + 1); got != "Super+Ctrl+Alt+Shift" {
		t.Errorf("mods = %q", got)
	}
	if got := mods(0); got != "" {
		t.Errorf("mods(0) = %q", got)
	}
}

func TestBindMatches(t *testing.T) {
	b := Bind{Mods: "Super+Shift", Key: "T", Description: "System monitor (btop)"}
	for _, q := range []string{"btop", "super shift", "MONITOR t"} {
		if !b.Matches(strings.Fields(strings.ToLower(q))) {
			t.Errorf("%q should match", q)
		}
	}
	if b.Matches([]string{"firefox"}) {
		t.Error("firefox should not match")
	}
}
