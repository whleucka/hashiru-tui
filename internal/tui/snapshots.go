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

// snapshots lists snapper's snapshots of /, shows what changed since one,
// and takes a new one before trying something risky. No rollback: booting a
// snapshot is grub-btrfs's job, not something to do to a live system.
type snapshots struct {
	e     *env
	snaps []system.Snapshot
	err   error
	sel   int

	diffOf  *system.Snapshot // showing changes since this one
	changes []system.Change
	diffErr error

	naming bool // typing a description for a new snapshot
	input  textinput.Model
	height int
}

type (
	snapsMsg struct {
		snaps []system.Snapshot
		err   error
	}
	changesMsg struct {
		number  int
		changes []system.Change
		err     error
	}
)

// diffMax is how many changed paths are listed; docker logs alone can
// change thousands of files between snapshots.
const diffMax = 500

func newSnapshots(e *env) *snapshots {
	return &snapshots{e: e, input: newInput("description: ", "what you're about to try", 50)}
}

func (s *snapshots) Name() string { return "Snapshots" }

func (s *snapshots) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case areaSizeMsg:
		s.height = msg.height
	case refreshMsg:
		probe := s.e.probe
		return func() tea.Msg {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			out, err := probe(ctx, "snapper", system.SnapshotsArgs...)
			if err != nil {
				return snapsMsg{err: err}
			}
			snaps, err := system.ParseSnapshots(out)
			return snapsMsg{snaps, err}
		}
	case snapsMsg:
		s.snaps, s.err = msg.snaps, msg.err
		s.sel = min(s.sel, max(len(s.snaps)-1, 0))
	case changesMsg:
		if s.diffOf != nil && s.diffOf.Number == msg.number {
			s.changes, s.diffErr = msg.changes, msg.err
		}
	}
	return nil
}

func (s *snapshots) Actions() []Action {
	return []Action{{Title: "Take a snapshot of / now (snapper)", Key: "n", Run: s.startNaming}}
}

func (s *snapshots) startNaming() tea.Cmd {
	s.naming = true
	s.input.SetValue("")
	return s.input.Focus()
}

func (s *snapshots) Help() string {
	switch {
	case s.naming:
		return "enter take snapshot · esc cancel"
	case s.diffOf != nil:
		return "esc back to the list"
	}
	return "j/k move · enter changes since · n new snapshot"
}

func (s *snapshots) HandleKey(msg tea.KeyPressMsg) (tea.Cmd, bool) {
	if s.naming {
		switch msg.String() {
		case "esc":
			s.naming = false
			s.input.Blur()
		case "enter":
			s.naming = false
			s.input.Blur()
			return s.create(strings.TrimSpace(s.input.Value())), true
		default:
			var cmd tea.Cmd
			s.input, cmd = s.input.Update(msg)
			return cmd, true
		}
		return nil, true
	}

	if s.diffOf != nil {
		if msg.String() == "esc" || msg.String() == "h" || msg.String() == "left" {
			s.diffOf, s.changes, s.diffErr = nil, nil, nil
			return nil, true
		}
		return nil, false
	}

	switch msg.String() {
	case "j", "down":
		if s.sel < len(s.snaps)-1 {
			s.sel++
		}
	case "k", "up":
		if s.sel > 0 {
			s.sel--
		}
	case "enter", "l", "right":
		return s.diff(), true
	case "n":
		return s.startNaming(), true
	default:
		return nil, false
	}
	return nil, true
}

func (s *snapshots) diff() tea.Cmd {
	if s.sel >= len(s.snaps) {
		return nil
	}
	snap := s.snaps[s.sel]
	s.diffOf, s.changes, s.diffErr = &snap, nil, nil
	probe := s.e.probe
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		out, err := probe(ctx, "snapper", system.StatusArgs(snap.Number)...)
		return changesMsg{snap.Number, system.ParseChanges(out), err}
	}
}

// create takes a snapshot through sudo in the terminal, where the password
// prompt works. It has no cleanup algorithm, so snapper keeps it until it's
// deleted by hand: it was taken on purpose.
func (s *snapshots) create(desc string) tea.Cmd {
	if desc == "" {
		desc = "hashiru-tui"
	}
	argv := []string{"sudo", "snapper", "-c", system.SnapperConfig, "create", "-d", desc}
	return s.e.launch(Launch{Argv: argv, Label: "snapshot", Kind: Interactive, Refresh: true})
}

func (s *snapshots) View(width, height int) string {
	switch {
	case s.err != nil:
		return styleFail.Render(s.err.Error())
	case s.snaps == nil:
		return styleMuted.Render("reading snapshots…")
	case s.diffOf != nil:
		return s.diffView(height)
	}

	var b strings.Builder
	if s.naming {
		b.WriteString(s.input.View() + "\n\n")
	} else {
		b.WriteString(styleMuted.Render(fmt.Sprintf("%d snapshots of / · n to take one", len(s.snaps))) + "\n\n")
	}
	rows := height - 2
	top := max(0, s.sel-rows+1)
	for i := top; i < len(s.snaps) && i < top+rows; i++ {
		sn := s.snaps[i]
		line := fmt.Sprintf("%5d  %s  %-6s %s", sn.Number, sn.Date.Format("2006-01-02 15:04"), sn.Type, sn.Description)
		if sn.Cleanup == "" {
			line += styleMuted.Render("  (kept)")
		}
		if i == s.sel {
			line = styleTitle.Render("▸ ") + styleSelected.Render(line)
		} else {
			line = "  " + line
		}
		b.WriteString(line + "\n")
	}
	return b.String()
}

func (s *snapshots) diffView(height int) string {
	var b strings.Builder
	b.WriteString(styleTitle.Render(fmt.Sprintf("Changes since #%d", s.diffOf.Number)) +
		styleMuted.Render("  "+s.diffOf.Date.Format("2006-01-02 15:04")+"  "+s.diffOf.Description) + "\n")
	switch {
	case s.diffErr != nil:
		b.WriteString(styleFail.Render(s.diffErr.Error()))
		return b.String()
	case s.changes == nil:
		b.WriteString(styleMuted.Render("comparing… (can take a while for old snapshots)"))
		return b.String()
	}
	created, deleted, modified := system.Count(s.changes)
	b.WriteString(styleOK.Render(fmt.Sprintf("%d created", created)) + styleMuted.Render(" · ") +
		styleFail.Render(fmt.Sprintf("%d deleted", deleted)) + styleMuted.Render(" · ") +
		styleWarn.Render(fmt.Sprintf("%d modified", modified)) + "\n\n")

	for i, c := range s.changes {
		if i == min(diffMax, height-3) {
			b.WriteString(styleMuted.Render(fmt.Sprintf("… and %d more", len(s.changes)-i)))
			break
		}
		kind := styleWarn.Render("~")
		switch c.Kind {
		case '+':
			kind = styleOK.Render("+")
		case '-':
			kind = styleFail.Render("-")
		}
		b.WriteString(kind + " " + c.Path + "\n")
	}
	return b.String()
}
