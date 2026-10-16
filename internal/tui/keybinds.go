package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

	"github.com/whleucka/hashiru-tui/internal/system"
)

// keybinds lists Hyprland's binds, searchable.
type keybinds struct {
	e     *env
	binds []system.Bind
	err   error

	filtering bool // keys go to the input
	input     textinput.Model
	offset    int // first row shown
	width     int
	height    int
}

type bindsMsg struct {
	binds []system.Bind
	err   error
}

func newKeybinds(e *env) *keybinds {
	return &keybinds{e: e, input: newInput("/", "filter", 30)}
}

func (k *keybinds) Name() string { return "Keybinds" }

func (k *keybinds) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case areaSizeMsg:
		k.width, k.height = msg.width, msg.height
		k.input.SetWidth(msg.width - 2)
	case refreshMsg:
		probe := k.e.probe
		return func() tea.Msg {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			out, err := probe(ctx, "hyprctl", "binds")
			if err != nil {
				return bindsMsg{err: err}
			}
			return bindsMsg{binds: system.ParseBinds(out)}
		}
	case bindsMsg:
		k.binds, k.err = msg.binds, msg.err
	}
	return nil
}

func (k *keybinds) Actions() []Action { return nil }

func (k *keybinds) Keys() []Key {
	if k.filtering {
		return []Key{
			{Keys: "type", Desc: "to filter"},
			{Keys: "enter", Desc: "keep"},
			{Keys: "esc", Desc: "clear"},
		}
	}
	return []Key{{Keys: "/", Desc: "filter"}, {Keys: "j/k", Desc: "scroll"}}
}

func (k *keybinds) HandleKey(msg tea.KeyPressMsg) (tea.Cmd, bool) {
	if k.filtering {
		switch msg.String() {
		case "esc":
			k.filtering = false
			k.input.SetValue("")
			k.input.Blur()
		case "enter":
			k.filtering = false
			k.input.Blur()
		default:
			var cmd tea.Cmd
			k.input, cmd = k.input.Update(msg)
			k.offset = 0
			return cmd, true
		}
		return nil, true
	}

	switch msg.String() {
	case "/":
		k.filtering = true
		return k.input.Focus(), true
	case "esc":
		if k.input.Value() == "" {
			return nil, false // back to the sidebar
		}
		k.input.SetValue("")
		k.offset = 0
	case "j", "down":
		if k.offset < len(k.shown())-1 {
			k.offset++
		}
	case "k", "up":
		if k.offset > 0 {
			k.offset--
		}
	default:
		return nil, false
	}
	return nil, true
}

func (k *keybinds) shown() []system.Bind {
	words := strings.Fields(strings.ToLower(k.input.Value()))
	if len(words) == 0 {
		return k.binds
	}
	var out []system.Bind
	for _, b := range k.binds {
		if b.Matches(words) {
			out = append(out, b)
		}
	}
	return out
}

func (k *keybinds) View(width, height int) string {
	if k.err != nil {
		return styleFail.Render(k.err.Error())
	}
	if k.binds == nil {
		return styleMuted.Render("reading binds…")
	}

	var b strings.Builder
	shown := k.shown()
	if k.filtering || k.input.Value() != "" {
		b.WriteString(k.input.View())
	} else {
		b.WriteString(styleMuted.Render(fmt.Sprintf("%d binds · / to filter", len(k.binds))))
	}
	b.WriteString("\n\n")

	rows := height - 2
	for i := k.offset; i < len(shown) && i < k.offset+rows; i++ {
		bind := shown[i]
		fmt.Fprintf(&b, "%s %s\n", styleTitle.Render(fmt.Sprintf("%-22s", bind.Combo())), bind.Description)
	}
	if len(shown) == 0 {
		b.WriteString(styleMuted.Render("no binds match"))
	}
	return b.String()
}
