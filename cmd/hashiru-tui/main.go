// Command hashiru-tui is a terminal UI for a Hashiru machine: its state, its
// health, and the hashiru commands that change it.
package main

import (
	"errors"
	"fmt"
	"os"
	"runtime/debug"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/whleucka/hashiru-tui/internal/hashiru"
	"github.com/whleucka/hashiru-tui/internal/herdr"
	"github.com/whleucka/hashiru-tui/internal/tui"
)

const usage = `usage: hashiru-tui [area[.tab]] [--run <action-id>]
       hashiru-tui --list-actions
       hashiru-tui --version`

// args is the command line: where to open, or something to print instead.
type args struct {
	area, run   string
	listActions bool
	version     bool
}

func parseArgs(argv []string) (args, error) {
	var a args
	for i := 0; i < len(argv); i++ {
		switch s := argv[i]; {
		case s == "--version" || s == "-V":
			a.version = true
		case s == "--list-actions":
			a.listActions = true
		case s == "--run":
			if i+1 >= len(argv) {
				return a, errors.New("--run needs an action ID")
			}
			i++
			a.run = argv[i]
		case strings.HasPrefix(s, "--run="):
			a.run = strings.TrimPrefix(s, "--run=")
		case s == "-h" || s == "--help":
			return a, errors.New("")
		case strings.HasPrefix(s, "-"):
			return a, fmt.Errorf("unknown flag %s", s)
		case a.area != "":
			return a, fmt.Errorf("one area at a time, not %q and %q", a.area, s)
		default:
			a.area = s
		}
	}
	return a, nil
}

func main() {
	a, err := parseArgs(os.Args[1:])
	if err != nil {
		if err.Error() != "" {
			fmt.Fprintln(os.Stderr, "hashiru-tui:", err)
		}
		fmt.Fprintln(os.Stderr, usage)
		os.Exit(2)
	}
	if a.version {
		// Hashiru's stage compares this with the tag it pins.
		fmt.Println(version(debug.ReadBuildInfo()))
		return
	}

	opts := tui.Options{Source: hashiru.CLI{Bin: os.Getenv("HASHIRU_BIN")}}

	// Inside herdr, commands open in a split beside the TUI, as in
	// mantis-tui. Elsewhere they borrow this terminal until they exit.
	if herdr.Available() {
		opts.RunInPane = herdr.Open
	}

	m := tui.New(opts)
	if a.listActions {
		for _, act := range m.Actions() {
			fmt.Printf("%s\t%s\n", act.ID, act.Title)
		}
		return
	}
	if err := m.Start(a.area, a.run); err != nil {
		fmt.Fprintln(os.Stderr, "hashiru-tui:", err)
		os.Exit(2)
	}

	if _, err := tea.NewProgram(m).Run(); err != nil {
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
