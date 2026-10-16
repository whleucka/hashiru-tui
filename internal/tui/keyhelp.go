package tui

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// typer is an area with a text input that can have focus. While it does, ?
// is a character for the input, not the keymap popup.
type typer interface {
	Typing() bool
}

// overlayMsg tells areas something now covers them (or no longer does), so
// one that draws outside the text layer, like Wallpaper's kitty preview, can
// get out of the way.
type overlayMsg struct{ open bool }

var keyHelpKeys = []Key{{Keys: "? / esc", Desc: "close"}}

var styleKeyHelp = lipgloss.NewStyle().
	Border(lipgloss.RoundedBorder()).
	BorderForeground(colorMuted).
	Padding(0, 1)

func (m *Model) typing() bool {
	t, ok := m.area().(typer)
	return ok && m.inArea && t.Typing()
}

func (m *Model) setKeyHelp(open bool) tea.Cmd {
	m.keyHelp = open
	return m.broadcast(overlayMsg{open})
}

// handleKeyHelpKey takes every key while the popup is open.
func (m *Model) handleKeyHelpKey(msg tea.KeyPressMsg) tea.Cmd {
	switch msg.String() {
	case "?", "esc", "q":
		return m.setKeyHelp(false)
	}
	return nil
}

// keyHelpView is the popup's box: the selected area's keys by group, then
// the shell's for where the focus is. From the sidebar it still shows the
// area's, as a preview of what entering it gives you.
func (m *Model) keyHelpView() string {
	global := sidebarKeys
	if m.inArea {
		global = inAreaKeys
	}
	keys := withGlobal(m.area().Keys(), global)

	width := 0
	for _, k := range keys {
		width = max(width, ansi.StringWidth(k.Keys))
	}

	var groups []string
	byGroup := map[string][]Key{}
	for _, k := range keys {
		g := k.Group
		if g == "" {
			g = m.area().Name()
		}
		if _, seen := byGroup[g]; !seen {
			groups = append(groups, g)
		}
		byGroup[g] = append(byGroup[g], k)
	}

	var b strings.Builder
	for i, g := range groups {
		if i > 0 {
			b.WriteString("\n")
		}
		b.WriteString(styleTitle.Render(g) + "\n")
		for _, k := range byGroup[g] {
			pad := strings.Repeat(" ", width-ansi.StringWidth(k.Keys))
			b.WriteString("  " + styleSelected.Render(k.Keys) + pad + "  " + k.Desc + "\n")
		}
	}
	return styleKeyHelp.Render(strings.TrimSuffix(b.String(), "\n"))
}

// overKeyHelp draws the popup centered over the area part of body.
func (m *Model) overKeyHelp(body string, areaW, areaH int) string {
	box := clip(m.keyHelpView(), areaW, areaH)
	bw, bh := lipgloss.Width(box), lipgloss.Height(box)
	left := sidebarWidth + 3 // sidebar, its border and padding, the gap
	// The window's size, not body's: an area with short lines makes body
	// narrower than where the popup goes.
	c := lipgloss.NewCanvas(max(m.width, lipgloss.Width(body)), max(areaH, lipgloss.Height(body)))
	// Through a Compositor: a Layer drawn straight onto the canvas ignores
	// its X and Y and fills the canvas.
	c.Compose(lipgloss.NewCompositor(
		lipgloss.NewLayer(body),
		lipgloss.NewLayer(box).X(left+max(0, (areaW-bw)/2)).Y(max(0, (areaH-bh)/2)).Z(1),
	))
	return c.Render()
}
