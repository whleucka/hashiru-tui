package tui

import (
	"strings"

	tea "charm.land/bubbletea/v2"
)

// choice is a short list to pick one thing from, drawn in place of what it
// was opened over: a mode, a scale, where to put a display.
type choice struct {
	title string
	about string // muted, beside the title
	opts  []string
	sel   int
	pick  func(int) tea.Cmd
}

// handleKey moves, picks or cancels; done is whether the choice is over.
func (c *choice) handleKey(k string, page int) (cmd tea.Cmd, done bool) {
	switch k {
	case "esc", "h", "left":
		return nil, true
	case "enter", "l", "right":
		if c.sel < len(c.opts) {
			return c.pick(c.sel), true
		}
		return nil, true
	}
	c.sel, _ = moveKey(k, c.sel, len(c.opts), page)
	return nil, false
}

func (c *choice) keys() []Key {
	return []Key{{Keys: "j/k", Desc: "move"}, {Keys: "enter", Desc: "pick"}, {Keys: "esc", Desc: "cancel"}}
}

func (c *choice) view(height int) string {
	var b strings.Builder
	b.WriteString(styleTitle.Render(c.title))
	if c.about != "" {
		b.WriteString(styleMuted.Render("  " + c.about))
	}
	b.WriteString("\n\n")
	rows := max(height-2, 1)
	top := max(0, c.sel-rows+1)
	for i := top; i < len(c.opts) && i < top+rows; i++ {
		if i == c.sel {
			b.WriteString(styleTitle.Render("▸ ") + styleSelected.Render(c.opts[i]) + "\n")
		} else {
			b.WriteString("  " + c.opts[i] + "\n")
		}
	}
	return b.String()
}
