package tui

import (
	"slices"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
)

// palette is the command palette: every area and every area's actions, found
// by typing a few words of the name.
type palette struct {
	open   bool
	input  textinput.Model
	items  []paletteItem
	recent []paletteItem // shown first while nothing is typed
	shown  []paletteItem // what matches the filter, in order
	cursor int           // into shown
}

type paletteItem struct {
	title  string
	hint   string // the key and ID, shown dimmed and matched too
	id     string // "" for "Go to <area>"
	recent bool   // listed under recents
	run    func() tea.Cmd
}

// shellActions are the shell's own, in the palette beside the areas'.
func (m *Model) shellActions() []Action {
	return []Action{
		{ID: "shell.refresh", Title: "Refresh everything", Key: "r", Run: m.refresh},
		{ID: "shell.keys", Title: "Show keys for this area", Key: "?", Run: func() tea.Cmd { return m.setKeyHelp(true) }},
		{ID: "shell.quit", Title: "Quit", Key: "q", Run: func() tea.Cmd { return tea.Quit }},
	}
}

// allActions is every action, the areas' then the shell's. Deep links and
// the palette both look here.
func (m *Model) allActions() []Action {
	var acts []Action
	for _, a := range m.areas {
		acts = append(acts, a.Actions()...)
	}
	return append(acts, m.shellActions()...)
}

// openPalette collects the items fresh each time, so actions reflect each
// area's current state. With nothing typed, recently run actions come first.
func (m *Model) openPalette() tea.Cmd {
	var gotos, acts []paletteItem
	for i, a := range m.areas {
		gotos = append(gotos, paletteItem{
			title: "Go to " + a.Name(),
			hint:  string(rune('1' + i)),
			run: func() tea.Cmd {
				m.selectArea(i)
				m.inArea = true
				return nil
			},
		})
	}
	for _, act := range m.allActions() {
		hint := act.ID
		if act.Key != "" {
			hint = act.Key + " · " + hint
		}
		acts = append(acts, paletteItem{title: act.Title, hint: hint, id: act.ID, run: m.recordRun(act)})
	}

	var recent []paletteItem
	for _, id := range m.recent {
		if i := slices.IndexFunc(acts, func(it paletteItem) bool { return it.id == id }); i >= 0 {
			it := acts[i]
			it.recent = true
			recent = append(recent, it)
		}
	}

	w, _ := m.areaSize()
	in := newInput(": ", "type to search areas and actions", w-3)
	m.pal = palette{open: true, input: in, items: append(gotos, acts...), recent: recent}
	m.pal.filter()
	return m.pal.input.Focus()
}

// recordRun wraps an action so running it from the palette remembers it.
func (m *Model) recordRun(act Action) func() tea.Cmd {
	return func() tea.Cmd {
		m.recent = addRecent(m.opts.RecentPath, m.recent, act.ID)
		return act.Run()
	}
}

// filter keeps items containing every typed word, in any order and case.
func (p *palette) filter() {
	words := strings.Fields(strings.ToLower(p.input.Value()))
	p.shown = p.shown[:0]
	if len(words) == 0 {
		p.shown = append(p.shown, p.recent...)
	}
outer:
	for _, it := range p.items {
		hay := strings.ToLower(it.title + " " + it.hint)
		for _, w := range words {
			if !strings.Contains(hay, w) {
				continue outer
			}
		}
		p.shown = append(p.shown, it)
	}
	p.cursor = min(p.cursor, max(len(p.shown)-1, 0))
}

func (p *palette) visible() []paletteItem { return p.shown }

// handlePaletteKey takes every key while the palette is open.
func (m *Model) handlePaletteKey(msg tea.KeyPressMsg) tea.Cmd {
	p := &m.pal
	switch msg.String() {
	case "esc", "ctrl+p":
		p.open = false
		return nil
	case "enter":
		p.open = false
		if len(p.shown) == 0 {
			return nil
		}
		return p.shown[p.cursor].run()
	case "down", "ctrl+n", "ctrl+j":
		if p.cursor < len(p.shown)-1 {
			p.cursor++
		}
		return nil
	case "up", "ctrl+k":
		if p.cursor > 0 {
			p.cursor--
		}
		return nil
	}
	var cmd tea.Cmd
	p.input, cmd = p.input.Update(msg)
	p.filter()
	return cmd
}

func (p *palette) view(width, height int) string {
	var b strings.Builder
	b.WriteString(p.input.View() + "\n\n")
	rows := height - 2
	top := max(0, p.cursor-rows+1)
	for i := top; i < len(p.shown) && i < top+rows; i++ {
		it := p.shown[i]
		hint := it.hint
		if it.recent {
			hint = "recent · " + hint
		}
		line := it.title + "  " + styleMuted.Render(hint)
		if i == p.cursor {
			line = styleTitle.Render("▸ ") + styleSelected.Render(it.title) + "  " + styleMuted.Render(hint)
		} else {
			line = "  " + line
		}
		b.WriteString(line + "\n")
	}
	if len(p.shown) == 0 {
		b.WriteString(styleMuted.Render("  nothing matches"))
	}
	return b.String()
}

func (p *palette) keys() []Key {
	return []Key{
		{Keys: "type", Desc: "to filter"},
		{Keys: "↑/↓", Desc: "move"},
		{Keys: "enter", Desc: "run"},
		{Keys: "esc", Desc: "close"},
	}
}
