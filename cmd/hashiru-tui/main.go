// Command hashiru-tui is a terminal UI for a Hashiru machine: its state, its
// health, and the hashiru commands that change it.
package main

import (
	"fmt"
	"os"

	tea "charm.land/bubbletea/v2"

	"github.com/whleucka/hashiru-tui/internal/hashiru"
	"github.com/whleucka/hashiru-tui/internal/herdr"
	"github.com/whleucka/hashiru-tui/internal/tui"
)

func main() {
	opts := tui.Options{Source: hashiru.CLI{Bin: os.Getenv("HASHIRU_BIN")}}

	// Inside herdr, commands open in a split beside the TUI, as in
	// mantis-tui. Elsewhere they borrow this terminal until they exit.
	if herdr.Available() {
		opts.RunInPane = herdr.Open
	}

	if _, err := tea.NewProgram(tui.New(opts)).Run(); err != nil {
		fmt.Fprintln(os.Stderr, "hashiru-tui:", err)
		os.Exit(1)
	}
}
