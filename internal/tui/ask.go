package tui

import (
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
)

// ask is a few questions in a row, each a line of text: minutes for each
// power source, a start and a stop. Each answer is checked before the next
// question; the last one hands them all to done.
type ask struct {
	steps []askStep
	i     int
	vals  []string
	err   error
	input textinput.Model
	done  func([]string) tea.Cmd
}

type askStep struct {
	prompt string
	value  string // what's there to begin with
	check  func(string) error
}

func newAsk(steps []askStep, done func([]string) tea.Cmd) *ask {
	a := &ask{steps: steps, done: done}
	a.load()
	return a
}

func (a *ask) load() {
	st := a.steps[a.i]
	a.input = newInput(st.prompt, "", 8)
	a.input.SetValue(st.value)
	a.input.CursorEnd()
}

func (a *ask) focus() tea.Cmd { return a.input.Focus() }

// handleKey answers, moves on, or cancels; over is whether the asking is.
func (a *ask) handleKey(msg tea.KeyPressMsg) (cmd tea.Cmd, over bool) {
	switch msg.String() {
	case "esc":
		return nil, true
	case "enter":
		v := strings.TrimSpace(a.input.Value())
		if check := a.steps[a.i].check; check != nil {
			if a.err = check(v); a.err != nil {
				return nil, false
			}
		}
		a.vals = append(a.vals, v)
		if a.i++; a.i == len(a.steps) {
			return a.done(a.vals), true
		}
		a.load()
		return a.focus(), false
	}
	a.input, cmd = a.input.Update(msg)
	return cmd, false
}

func (a *ask) view() string {
	var b strings.Builder
	for i, v := range a.vals {
		b.WriteString(styleMuted.Render(a.steps[i].prompt+v) + "\n")
	}
	b.WriteString(a.input.View() + "\n")
	if a.err != nil {
		b.WriteString(styleFail.Render(a.err.Error()) + "\n")
	}
	b.WriteString("\n" + styleMuted.Render("enter for the next · esc cancels"))
	return b.String()
}
