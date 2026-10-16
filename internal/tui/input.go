package tui

import "charm.land/bubbles/v2/textinput"

// newInput is a text input with a steady cursor: a blinking one redraws the
// whole screen twice a second for nothing.
func newInput(prompt, placeholder string, width int) textinput.Model {
	in := textinput.New()
	in.Prompt = prompt
	in.Placeholder = placeholder
	st := in.Styles()
	st.Cursor.Blink = false
	in.SetStyles(st)
	in.SetWidth(width) // the placeholder is cut to the width, and 0 means one cell
	return in
}
