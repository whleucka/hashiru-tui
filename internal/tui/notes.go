package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/whleucka/hashiru-tui/internal/hashiru"
	"github.com/whleucka/hashiru-tui/internal/system"
)

const (
	notesTimeout = 10 * time.Second
	notesMax     = 10 // releases shown when far behind
)

// notes are Hashiru's release notes for the versions this machine is behind,
// or for the one it's on. They're read from the checkout with git rather
// than `hashiru changelog`, which only sees what's been pulled: status has
// just fetched, so @{u} already has the notes for what an update brings.
type notes struct {
	releases []hashiru.Release
	newer    bool // releases are what an update brings, not this version's
	err      error
	busy     bool
	loaded   bool // a load has finished, with notes or without

	reading bool // the full notes fill the area
	wanted  bool // asked for while loading; open on arrival
	vp      viewport.Model
	width   int
	height  int
}

type notesMsg struct {
	releases []hashiru.Release
	newer    bool
	err      error
}

// loadNotes reads the notes for s. Behind, from the upstream ref; otherwise
// from HEAD.
func loadNotes(run system.Run, s hashiru.Status) tea.Cmd {
	if s.Repo == "" {
		return nil
	}
	behind := s.BehindOK && s.Behind > 0
	ref := "HEAD"
	if behind {
		ref = "@{u}"
	}
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), notesTimeout)
		defer cancel()
		git := func(args ...string) ([]byte, error) {
			return run(ctx, "git", append([]string{"-C", s.Repo}, args...)...)
		}
		ls, err := git("ls-tree", "--name-only", ref, hashiru.ReleasesDir+"/")
		if err != nil {
			return notesMsg{err: err}
		}
		tags, newer := hashiru.NotesFor(hashiru.ReleaseTags(ls), hashiru.BaseVersion(s.Version), behind)
		if len(tags) > notesMax {
			tags = tags[:notesMax]
		}
		var rs []hashiru.Release
		for _, t := range tags {
			body, err := git("show", ref+":"+hashiru.ReleasesDir+"/"+t+".md")
			if err != nil {
				return notesMsg{err: err}
			}
			rs = append(rs, hashiru.Release{Tag: t, Body: string(body)})
		}
		return notesMsg{releases: rs, newer: newer}
	}
}

func (n *notes) setSize(w, h int) {
	n.width, n.height = w, h
	n.vp.SetWidth(w)
	n.vp.SetHeight(max(h-2, 1)) // the title and a blank line
	n.vp.SetContent(n.body())
}

func (n *notes) open() {
	n.reading, n.wanted = true, false
	n.setSize(n.width, n.height)
	n.vp.GotoTop()
}

func (n *notes) handleKey(msg tea.KeyPressMsg) (tea.Cmd, bool) {
	switch msg.String() {
	case "esc", "h", "left", "c":
		n.reading = false
	case "g", "home":
		n.vp.GotoTop()
	case "G", "end":
		n.vp.GotoBottom()
	case "j", "k", "up", "down", "pgup", "pgdown", "ctrl+d", "ctrl+u", "space":
		var cmd tea.Cmd
		n.vp, cmd = n.vp.Update(msg)
		return cmd, true
	default:
		return nil, false
	}
	return nil, true
}

func (n *notes) keys() []Key {
	return []Key{
		{Keys: "j/k", Desc: "scroll"},
		{Keys: "g/G", Desc: "top/bottom"},
		{Keys: "esc", Desc: "back"},
	}
}

func (n *notes) title() string {
	if n.newer {
		return styleTitle.Render("Release notes") + styleWarn.Render("  what an update brings")
	}
	return styleTitle.Render("Release notes") + styleMuted.Render("  this version")
}

func (n *notes) view() string {
	return n.title() + "\n\n" + n.vp.View()
}

// body is every release, newest first, with markdown headings picked out.
func (n *notes) body() string {
	wrap := lipgloss.NewStyle().Width(max(n.width, 20))
	var b strings.Builder
	for i, r := range n.releases {
		if i > 0 {
			b.WriteString("\n")
		}
		for _, l := range strings.Split(strings.TrimSpace(r.Body), "\n") {
			if strings.HasPrefix(l, "#") {
				b.WriteString(styleTitle.Render(strings.TrimLeft(l, "# ")) + "\n")
				continue
			}
			b.WriteString(wrap.Render(l) + "\n")
		}
	}
	return b.String()
}

// rows are the summary Updates shows: a line per release.
func (n *notes) rows() []string {
	switch {
	case n.busy && n.releases == nil:
		return []string{styleMuted.Render("  reading…")}
	case n.err != nil:
		return []string{styleMuted.Render("  " + n.err.Error())}
	case len(n.releases) == 0:
		return []string{styleMuted.Render("  none for this version")}
	}
	var out []string
	for _, r := range n.releases {
		out = append(out, fmt.Sprintf("  %s  %s", styleSelected.Render(r.Tag), styleMuted.Render(r.Summary())))
	}
	return out
}
