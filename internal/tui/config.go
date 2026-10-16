package tui

import (
	"os"
	"path/filepath"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/whleucka/hashiru-tui/internal/system"
)

// configArea lists the files Hashiru lets a machine change, grouped by how
// they apply, each one keypress from $EDITOR. Everything else Hashiru owns
// outright and puts back on update, which is the point of listing these.
type configArea struct {
	e       *env
	home    string
	entries []configEntry
	sel     int
	height  int
}

type configEntry struct {
	group  string
	path   string // absolute
	note   string
	exists bool
	viaCLI bool // hashiru.conf: `hashiru config` seeds it from the example
}

// hyprModules are the modules hypr/hyprland.lua loads through hashiru.load,
// each of which takes a <module>.lua replacement or a <module>.extra.lua.
var hyprModules = []string{"monitors", "autostart", "keybinds", "windowrules", "layerrules", "clamshell"}

func newConfig(e *env, home string) *configArea {
	c := &configArea{e: e, home: home}
	c.scan()
	return c
}

// scan builds the list and checks which files exist.
func (c *configArea) scan() {
	dir := system.HashiruConfigDir(c.home)
	var es []configEntry
	add := func(group, path, note string) {
		es = append(es, configEntry{group: group, path: path, note: note})
	}
	es = append(es, configEntry{group: "Hashiru", path: filepath.Join(dir, "hashiru.conf"), note: "Hashiru's own settings", viaCLI: true})
	for _, m := range hyprModules {
		add("Hyprland", filepath.Join(dir, "hypr", m+".extra.lua"), "runs after Hashiru's "+m)
	}
	add("Hyprland", filepath.Join(dir, "hypr", "local.lua"), "runs last, wins per setting")
	for _, m := range hyprModules {
		add("Hyprland (replace)", filepath.Join(dir, "hypr", m+".lua"), "replaces Hashiru's "+m+" outright")
	}
	add("Terminal and bar", filepath.Join(dir, "kitty", "local.conf"), "kitty includes every *.conf here")
	add("Terminal and bar", filepath.Join(dir, "waybar", "style.css"), "imported after Hashiru's style")
	add("Shell", filepath.Join(c.home, ".zshrc.local"), "sourced at the end of .zshrc")

	for i := range es {
		_, err := os.Stat(es[i].path)
		es[i].exists = err == nil
	}
	c.entries = es
}

func (c *configArea) Name() string { return "Config" }

func (c *configArea) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case refreshMsg:
		c.scan()
	case areaSizeMsg:
		c.height = msg.height
	}
	return nil
}

func (c *configArea) Actions() []Action {
	acts := []Action{{ID: c.actionID(c.entries[0]), Title: "Edit hashiru.conf (hashiru config)", Run: func() tea.Cmd { return c.open(c.entries[0]) }}}
	for _, e := range c.entries[1:] {
		acts = append(acts, Action{ID: c.actionID(e), Title: "Edit " + c.short(e.path), Run: func() tea.Cmd { return c.open(e) }})
	}
	return acts
}

// actionID names an entry by its path under Hashiru's config directory (or
// home, for .zshrc.local): config.hypr-local-lua, config.zshrc-local.
func (c *configArea) actionID(e configEntry) string {
	rel, err := filepath.Rel(system.HashiruConfigDir(c.home), e.path)
	if err != nil || strings.HasPrefix(rel, "..") {
		rel, _ = filepath.Rel(c.home, e.path)
	}
	slug := strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			return r
		}
		return '-'
	}, strings.ToLower(rel))
	return "config.edit-" + strings.Trim(slug, "-")
}

func (c *configArea) HandleKey(msg tea.KeyPressMsg) (tea.Cmd, bool) {
	if i, ok := moveKey(msg.String(), c.sel, len(c.entries), c.height); ok {
		c.sel = i
		return nil, true
	}
	switch msg.String() {
	case "enter", "e", "l", "right":
		return c.open(c.entries[c.sel]), true
	default:
		return nil, false
	}
}

func (c *configArea) Keys() []Key {
	return []Key{{Keys: "j/k", Desc: "move"}, {Keys: "enter", Desc: "edit"}}
}

// open edits e in the terminal and rescans after, so a new file shows up.
func (c *configArea) open(e configEntry) tea.Cmd {
	if e.viaCLI {
		return c.e.launch(Launch{Argv: c.e.src.Argv("config"), Label: "hashiru config", Kind: Interactive, Refresh: true})
	}
	// The editor can create the file but not its directory.
	if err := os.MkdirAll(filepath.Dir(e.path), 0o755); err != nil {
		return func() tea.Msg { return noteMsg{text: "can't create " + filepath.Dir(e.path), err: err} }
	}
	return c.e.launch(Launch{Argv: []string{editor(), e.path}, Label: c.short(e.path), Kind: Interactive, Refresh: true})
}

// editor is $VISUAL, then $EDITOR, then vi.
func editor() string {
	for _, v := range []string{"VISUAL", "EDITOR"} {
		if e := os.Getenv(v); e != "" {
			return e
		}
	}
	return "vi"
}

// short shows a path from ~.
func (c *configArea) short(p string) string {
	if rel, err := filepath.Rel(c.home, p); err == nil && !strings.HasPrefix(rel, "..") {
		return "~/" + rel
	}
	return p
}

func (c *configArea) View(width, height int) string {
	var b strings.Builder
	b.WriteString(styleMuted.Render("● exists · everything else under ~/.config is Hashiru's and is put back on update") + "\n")
	group := ""
	cursorLine := 0
	for i, e := range c.entries {
		if e.group != group {
			group = e.group
			b.WriteString("\n" + styleTitle.Render(group) + "\n")
		}
		mark := "  "
		if e.exists {
			mark = styleOK.Render("● ")
		}
		line := mark + cell(c.short(e.path), 44) + " " + styleMuted.Render(e.note)
		if i == c.sel {
			line = styleTitle.Render("▸ ") + line
			cursorLine = strings.Count(b.String(), "\n") + 1
		} else {
			line = "  " + line
		}
		b.WriteString(line + "\n")
	}
	return scrollTo(b.String(), cursorLine, height)
}
