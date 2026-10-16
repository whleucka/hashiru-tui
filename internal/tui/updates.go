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
	notes    notes
}

// pkgPreview is how many pending packages show before "… N more".
const pkgPreview = 8

func newUpdates(e *env) *updates {
	return &updates{e: e, expanded: map[string]bool{}}
}

func (u *updates) Name() string { return "Updates" }

func (u *updates) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case reportMsg:
		if n := len(u.sections()); u.cursor >= n {
			u.cursor = max(n-1, 0)
		}
	case areaSizeMsg:
		u.notes.setSize(msg.width, msg.height)
	case statusMsg:
		if cmd := loadNotes(u.e.probe, msg.s); msg.err == nil && cmd != nil {
			u.notes.busy = true
			return cmd
		}
	case notesMsg:
		u.notes.releases, u.notes.newer, u.notes.err = msg.releases, msg.newer, msg.err
		u.notes.busy, u.notes.loaded = false, true
		u.notes.setSize(u.notes.width, u.notes.height)
		if u.notes.wanted && len(u.notes.releases) > 0 {
			u.notes.open()
		}
	}
	return nil
}

func (u *updates) Actions() []Action {
	return []Action{
		{ID: "updates.update", Title: "Update the system (hashiru update)", Key: "u", Run: u.update},
		{ID: "updates.aur", Title: "Upgrade AUR packages (yay -Sua)", Key: "a", Run: u.upgradeAUR},
		{ID: "updates.news", Title: "Read Arch news", Key: "n", Run: u.openNews},
		{ID: "updates.notes", Title: "Read Hashiru's release notes", Key: "c", Run: func() tea.Cmd { u.readNotes(); return nil }},
	}
}

// readNotes opens the full release notes, or opens them when they arrive if
// they're still loading (as on `hashiru-tui --run updates.notes`).
func (u *updates) readNotes() {
	if len(u.notes.releases) > 0 {
		u.notes.open()
		return
	}
	// Status may not even be back yet, so "not loaded" rather than "busy".
	u.notes.wanted = !u.notes.loaded
}

// openNews opens the newest unread announcement in the browser, or the
// news page when there is none.
func (u *updates) openNews() tea.Cmd {
	return u.e.launch(Launch{Argv: []string{"xdg-open", newsTarget(u.e.state)}, Label: "Arch news", Kind: GUI})
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
	if u.notes.reading {
		return u.notes.handleKey(msg)
	}
	switch msg.String() {
	case "c":
		u.readNotes()
	case "u":
		return u.update(), true
	case "a":
		return u.upgradeAUR(), true
	case "n":
		return u.openNews(), true
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

func (u *updates) Keys() []Key {
	if u.notes.reading {
		return u.notes.keys()
	}
	return []Key{
		{Keys: "u", Desc: "update"},
		{Keys: "c", Desc: "release notes"},
		{Keys: "a", Desc: "AUR"},
		{Keys: "n", Desc: "news"},
		{Keys: "p", Desc: "all packages"},
		{Keys: "j/k", Desc: "section", Group: "Health"},
		{Keys: "enter", Desc: "expand", Group: "Health"},
	}
}

func (u *updates) sections() []hashiru.Section {
	if u.e.state.Report == nil {
		return nil
	}
	return u.e.state.Report.Sections()
}

func (u *updates) View(width, height int) string {
	if u.notes.reading {
		return u.notes.view()
	}
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

	b.WriteString("\n" + u.notes.title() + styleMuted.Render("  c read") + "\n")
	for _, r := range u.notes.rows() {
		b.WriteString(r + "\n")
	}

	b.WriteString("\n" + styleTitle.Render("Packages") + "  " + pendingLine(st) + "\n")
	limit := pkgPreview
	if u.allPkgs {
		limit = 0
	}
	for _, r := range pendingRows(st, limit) {
		b.WriteString(r + "\n")
	}

	b.WriteString("\n" + styleTitle.Render("Arch news") + "\n")
	for _, r := range newsRows(st) {
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

	// Scroll as little as possible to keep the cursor's section, and its
	// checks, on screen: the top stays put until the cursor needs the room.
	cursorEnd := 0
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
		if i == u.cursor {
			cursorEnd = strings.Count(b.String(), "\n")
		}
	}
	return scrollTo(b.String(), cursorEnd, height)
}

// scrollTo drops lines off the top until line end (exclusive) fits in height.
func scrollTo(s string, end, height int) string {
	if skip := end - height; skip > 0 {
		lines := strings.Split(s, "\n")
		return strings.Join(lines[min(skip, len(lines)):], "\n")
	}
	return s
}
