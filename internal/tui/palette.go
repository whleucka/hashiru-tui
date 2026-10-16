package tui

import (
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
	shown  []int // indexes into items that match the filter
	cursor int   // into shown
}

type paletteItem struct {
	title string
	hint  string // the area and key, shown dimmed and matched too
	run   func() tea.Cmd
}

// openPalette collects the items fresh each time, so actions reflect each
// area's current state.
func (m *Model) openPalette() tea.Cmd {
	var items []paletteItem
	for i, a := range m.areas {
		items = append(items, paletteItem{
			title: "Go to " + a.Name(),
			hint:  string(rune('1' + i)),
			run: func() tea.Cmd {
				m.selectArea(i)
				m.inArea = true
				return nil
			},
		})
	}
	for _, a := range m.areas {
		for _, act := range a.Actions() {
			hint := a.Name()
			if act.Key != "" {
				hint += " · " + act.Key
			}
			items = append(items, paletteItem{title: act.Title, hint: hint, run: act.Run})
		}
	}

	w, _ := m.areaSize()
	in := newInput(": ", "type to search areas and actions", w-3)
	m.pal = palette{open: true, input: in, items: items}
	m.pal.filter()
	return m.pal.input.Focus()
}

// filter keeps items containing every typed word, in any order and case.
func (p *palette) filter() {
	words := strings.Fields(strings.ToLower(p.input.Value()))
	p.shown = p.shown[:0]
outer:
	for i, it := range p.items {
		hay := strings.ToLower(it.title + " " + it.hint)
		for _, w := range words {
			if !strings.Contains(hay, w) {
				continue outer
			}
		}
		p.shown = append(p.shown, i)
	}
	p.cursor = min(p.cursor, max(len(p.shown)-1, 0))
}

func (p *palette) visible() []paletteItem {
	out := make([]paletteItem, len(p.shown))
	for i, idx := range p.shown {
		out[i] = p.items[idx]
	}
	return out
}

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
		return p.items[p.shown[p.cursor]].run()
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
		it := p.items[p.shown[i]]
		line := it.title + "  " + styleMuted.Render(it.hint)
		if i == p.cursor {
			line = styleTitle.Render("▸ ") + styleSelected.Render(it.title) + "  " + styleMuted.Render(it.hint)
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

func (p *palette) help() string { return "type to filter · ↑/↓ move · enter run · esc close" }
