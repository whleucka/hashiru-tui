package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/whleucka/hashiru-tui/internal/hashiru"
)

// updates is where the machine gets updated and checked: what Hashiru is at,
// and doctor's health checks.
type updates struct {
	e *env
	// Health: which section the cursor is on, and which are expanded.
	cursor   int
	expanded map[string]bool
	allPkgs  bool // list every pending package, not just the first few
}

// pkgPreview is how many pending packages show before "… N more".
const pkgPreview = 8

func newUpdates(e *env) *updates {
	return &updates{e: e, expanded: map[string]bool{}}
}

func (u *updates) Name() string { return "Updates" }

func (u *updates) Update(msg tea.Msg) tea.Cmd {
	if _, ok := msg.(reportMsg); ok {
		if n := len(u.sections()); u.cursor >= n {
			u.cursor = max(n-1, 0)
		}
	}
	return nil
}

func (u *updates) Actions() []Action {
	return []Action{
		{Title: "Update the system (hashiru update)", Key: "u", Run: u.update},
		{Title: "Upgrade AUR packages (yay -Sua)", Key: "a", Run: u.upgradeAUR},
	}
}

// upgradeAUR is separate because hashiru update runs pacman -Syu but never
// upgrades AUR packages; stage 20 only installs the missing ones.
func (u *updates) upgradeAUR() tea.Cmd {
	return u.e.launch(Launch{Argv: []string{"yay", "-Sua"}, Label: "yay -Sua", Kind: Interactive, Refresh: true})
}

func (u *updates) update() tea.Cmd {
	return u.e.launch(Launch{Argv: u.e.src.Argv("update"), Label: "hashiru update", Kind: Interactive, Refresh: true})
}

func (u *updates) HandleKey(msg tea.KeyPressMsg) (tea.Cmd, bool) {
	switch msg.String() {
	case "u":
		return u.update(), true
	case "a":
		return u.upgradeAUR(), true
	case "p":
		u.allPkgs = !u.allPkgs
	case "j", "down":
		if u.cursor < len(u.sections())-1 {
			u.cursor++
		}
	case "k", "up":
		if u.cursor > 0 {
			u.cursor--
		}
	case "enter", "space", "l", "right":
		if s := u.sections(); u.cursor < len(s) {
			name := s[u.cursor].Name
			u.expanded[name] = !u.expanded[name]
		}
	default:
		return nil, false
	}
	return nil, true
}

func (u *updates) Help() string {
	return "u update · a AUR · p all packages · j/k section · enter expand"
}

func (u *updates) sections() []hashiru.Section {
	if u.e.state.Report == nil {
		return nil
	}
	return u.e.state.Report.Sections()
}

func (u *updates) View(width, height int) string {
	st := u.e.state
	var b strings.Builder
	row := func(k, v string) { b.WriteString(styleKey.Render(k) + v + "\n") }

	b.WriteString(styleTitle.Render("Hashiru") + "\n")
	switch {
	case st.Status == nil && st.StatusBusy:
		row("status", styleMuted.Render("loading… (fetches upstream)"))
	case st.Status == nil && st.StatusErr != nil:
		row("status", styleFail.Render(st.StatusErr.Error()))
	case st.Status != nil:
		s := st.Status
		row("version", orUnknown(s.Version))
		row("commit", short(s.Commit))
		if s.Head != "" && s.Head != s.Commit {
			row("checkout", short(s.Head)+styleWarn.Render("  (not what was last installed)"))
		}
		row("installed", when(s.Installed))
		if !s.Updated.IsZero() {
			row("updated", when(s.Updated))
		}
		row("repo", s.Repo)
		row("tree", treeLine(s))
		row("upstream", upstreamLine(s))
	}

	b.WriteString("\n" + styleTitle.Render("Packages") + "  " + pendingLine(st) + "\n")
	limit := pkgPreview
	if u.allPkgs {
		limit = 0
	}
	for _, r := range pendingRows(st, limit) {
		b.WriteString(r + "\n")
	}

	b.WriteString("\n" + styleTitle.Render("Health") + "  ")
	switch {
	case st.Report == nil && st.ReportBusy:
		b.WriteString(styleMuted.Render("running doctor…"))
		return b.String()
	case st.Report == nil && st.ReportErr != nil:
		b.WriteString(styleFail.Render(st.ReportErr.Error()))
		return b.String()
	case st.Report == nil:
		return b.String()
	}
	b.WriteString(summaryLine(st.Report) + "\n")

	for i, sec := range u.sections() {
		cursor := "  "
		name := sec.Name
		if i == u.cursor {
			cursor = styleTitle.Render("▸ ")
			name = styleSelected.Render(name)
		}
		fmt.Fprintf(&b, "%s%s %s %s\n", cursor, mark(sec.Worst()), name,
			styleMuted.Render(fmt.Sprintf("(%d)", len(sec.Checks))))

		// Problems are always shown; the rest only when expanded.
		open := u.expanded[sec.Name]
		for _, c := range sec.Checks {
			if open || c.Status == hashiru.Warn || c.Status == hashiru.Fail {
				fmt.Fprintf(&b, "      %s %s\n", mark(c.Status), c.Message)
			}
		}
	}
	return b.String()
}
