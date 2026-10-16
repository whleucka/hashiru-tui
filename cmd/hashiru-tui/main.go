// Command hashiru-tui is a terminal UI for a Hashiru machine: its state, its
// health, and the hashiru commands that change it.
package main

import (
	"fmt"
	"os"
	"runtime/debug"

	tea "charm.land/bubbletea/v2"

	"github.com/whleucka/hashiru-tui/internal/hashiru"
	"github.com/whleucka/hashiru-tui/internal/herdr"
	"github.com/whleucka/hashiru-tui/internal/tui"
)

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "--version", "-V":
			// Hashiru's stage compares this with the tag it pins.
			fmt.Println(version(debug.ReadBuildInfo()))
			return
		default:
			fmt.Fprintln(os.Stderr, "usage: hashiru-tui [--version]")
			os.Exit(2)
		}
	}

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

// version is the module version `go install ...@vX.Y.Z` stamps into the
// binary, or "devel" for a local build.
func version(info *debug.BuildInfo, ok bool) string {
	if !ok || info == nil || info.Main.Version == "" || info.Main.Version == "(devel)" {
		return "devel"
	}
	return info.Main.Version
}
