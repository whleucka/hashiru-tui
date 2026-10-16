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

const (
	packagesTimeout = 15 * time.Second
	pkgInfoTimeout  = 15 * time.Second // -Si reads the sync db; yay -Si asks the AUR
)

// pkgGroup is one of the Packages area's lists.
type pkgGroup int

// Shortest and most interesting first: Hashiru's own list is the long one.
const (
	groupYours pkgGroup = iota
	groupMissing
	groupOrphans
	groupForeign
	groupManaged
	groupCount
)

func (g pkgGroup) String() string {
	return [...]string{"Yours", "Missing", "Orphans", "Foreign", "Hashiru"}[g]
}

// pkgRow is one package in a group's list.
type pkgRow struct {
	name string
	note string // the manifest it's in, for Hashiru's and missing ones
	aur  bool
	// installed: -Qi knows it. Missing ones are looked up in the sync db
	// (or the AUR) instead.
	installed bool
}

// packages is the machine's packages as Hashiru sees them: what its
// manifests install, what you added, and what's drifted.
type packages struct {
	e     *env
	cache string // pacman's package cache, sized for the summary
	pkgs  *system.Packages
	busy  bool

	group  pkgGroup
	sel    int
	offset int

	filtering bool
	input     textinput.Model

	infoOf  *pkgRow // showing its pacman -Qi / -Si
	info    string
	infoErr error
	infoTop int
	width   int
	height  int
}

type (
	packagesMsg struct{ p system.Packages }
	pkgInfoMsg  struct {
		name string
		out  string
		err  error
	}
)

func newPackages(e *env, cache string) *packages {
	return &packages{e: e, cache: cache, input: newInput("/", "filter", 30)}
}

func (p *packages) Name() string      { return "Packages" }
func (p *packages) Actions() []Action { return nil }
func (p *packages) Typing() bool      { return p.filtering }

func (p *packages) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case areaSizeMsg:
		p.width, p.height = msg.width, msg.height
		p.input.SetWidth(msg.width - 2)
	case statusMsg:
		// The manifests are in the checkout status names. Without one,
		// everything explicit is just yours.
		return p.load(msg.s.Repo)
	case packagesMsg:
		p.pkgs, p.busy = &msg.p, false
		p.clampSel()
	case pkgInfoMsg:
		if p.infoOf != nil && p.infoOf.name == msg.name {
			p.info, p.infoErr = msg.out, msg.err
		}
	}
	return nil
}

func (p *packages) load(repo string) tea.Cmd {
	p.busy = true
	probe, cache := p.e.probe, p.cache
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), packagesTimeout)
		defer cancel()
		return packagesMsg{system.ReadPackages(ctx, probe, repo, cache)}
	}
}

func (p *packages) rows(g pkgGroup) []pkgRow {
	if p.pkgs == nil {
		return nil
	}
	var out []pkgRow
	names := func(ns []string, installed bool) {
		for _, n := range ns {
			out = append(out, pkgRow{name: n, installed: installed})
		}
	}
	entries := func(es []system.ManifestEntry, installed bool) {
		for _, e := range es {
			out = append(out, pkgRow{name: e.Name, note: e.File, aur: e.AUR, installed: installed})
		}
	}
	switch g {
	case groupYours:
		names(p.pkgs.Yours, true)
	case groupMissing:
		entries(p.pkgs.Missing, false)
	case groupOrphans:
		names(p.pkgs.Orphans, true)
	case groupForeign:
		names(p.pkgs.Foreign, true)
	case groupManaged:
		entries(p.pkgs.Managed, true)
	}
	return out
}

// shown is the current group, filtered by every typed word.
func (p *packages) shown() []pkgRow {
	words := strings.Fields(strings.ToLower(p.input.Value()))
	var out []pkgRow
outer:
	for _, r := range p.rows(p.group) {
		hay := strings.ToLower(r.name + " " + r.note)
		for _, w := range words {
			if !strings.Contains(hay, w) {
				continue outer
			}
		}
		out = append(out, r)
	}
	return out
}

func (p *packages) clampSel() {
	p.sel = min(p.sel, max(len(p.shown())-1, 0))
}

func (p *packages) Keys() []Key {
	switch {
	case p.infoOf != nil:
		return []Key{{Keys: "j/k", Desc: "scroll"}, {Keys: "esc", Desc: "back to the list"}}
	case p.filtering:
		return []Key{{Keys: "type", Desc: "to filter"}, {Keys: "enter", Desc: "keep"}, {Keys: "esc", Desc: "clear"}}
	}
	return []Key{
		{Keys: "[/]", Desc: "group"},
		{Keys: "j/k", Desc: "move"},
		{Keys: "enter", Desc: "info"},
		{Keys: "/", Desc: "filter"},
	}
}

func (p *packages) HandleKey(msg tea.KeyPressMsg) (tea.Cmd, bool) {
	if p.infoOf != nil {
		return p.infoKey(msg)
	}
	if p.filtering {
		switch msg.String() {
		case "esc":
			p.input.SetValue("")
			fallthrough
		case "enter":
			p.filtering = false
			p.input.Blur()
		default:
			var cmd tea.Cmd
			p.input, cmd = p.input.Update(msg)
			p.sel, p.offset = 0, 0
			return cmd, true
		}
		p.clampSel()
		return nil, true
	}

	switch msg.String() {
	case "]":
		p.setGroup((p.group + 1) % groupCount)
	case "[":
		p.setGroup((p.group + groupCount - 1) % groupCount)
	case "/":
		p.filtering = true
		return p.input.Focus(), true
	case "esc":
		if p.input.Value() == "" {
			return nil, false // back to the sidebar
		}
		p.input.SetValue("")
		p.clampSel()
	case "j", "down":
		p.sel = min(p.sel+1, max(len(p.shown())-1, 0))
	case "k", "up":
		p.sel = max(p.sel-1, 0)
	case "g", "home":
		p.sel = 0
	case "G", "end":
		p.sel = max(len(p.shown())-1, 0)
	case "enter", "l", "right":
		return p.showInfo(), true
	default:
		return nil, false
	}
	return nil, true
}

func (p *packages) setGroup(g pkgGroup) {
	p.group, p.sel, p.offset = g, 0, 0
}

func (p *packages) infoKey(msg tea.KeyPressMsg) (tea.Cmd, bool) {
	switch msg.String() {
	case "esc", "h", "left":
		p.infoOf = nil
	case "j", "down":
		p.infoTop = min(p.infoTop+1, max(strings.Count(p.info, "\n")-1, 0))
	case "k", "up":
		p.infoTop = max(p.infoTop-1, 0)
	default:
		return nil, false
	}
	return nil, true
}

// infoArgv is how to describe r: the local db for an installed package; the
// sync db, or the AUR through yay, for one that's missing.
func infoArgv(r pkgRow) []string {
	switch {
	case r.installed:
		return []string{"pacman", "-Qi", r.name}
	case r.aur:
		return []string{"yay", "-Si", r.name}
	}
	return []string{"pacman", "-Si", r.name}
}

func (p *packages) showInfo() tea.Cmd {
	shown := p.shown()
	if p.sel >= len(shown) {
		return nil
	}
	r := shown[p.sel]
	p.infoOf, p.info, p.infoErr, p.infoTop = &r, "", nil, 0
	argv := infoArgv(r)
	probe := p.e.probe
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), pkgInfoTimeout)
		defer cancel()
		out, err := probe(ctx, argv[0], argv[1:]...)
		return pkgInfoMsg{r.name, string(out), err}
	}
}

func (p *packages) View(width, height int) string {
	switch {
	case p.pkgs == nil && p.busy:
		return styleMuted.Render("reading the package database…")
	case p.pkgs == nil:
		return styleMuted.Render("waiting for hashiru status (it names the checkout with the manifests)…")
	case p.pkgs.PacmanErr != nil:
		return styleFail.Render(p.pkgs.PacmanErr.Error())
	case p.infoOf != nil:
		return p.infoView(height)
	}

	var b strings.Builder
	b.WriteString(p.tabs() + "\n")
	b.WriteString(p.summary() + "\n")
	if p.filtering || p.input.Value() != "" {
		b.WriteString(p.input.View())
	}
	b.WriteString("\n")

	shown := p.shown()
	rows := max(height-3, 1)
	// Scroll as little as possible to keep the cursor on screen.
	if p.sel < p.offset {
		p.offset = p.sel
	}
	if p.sel >= p.offset+rows {
		p.offset = p.sel - rows + 1
	}
	for i := p.offset; i < len(shown) && i < p.offset+rows; i++ {
		r := shown[i]
		name, note := fmt.Sprintf("%-32s", r.name), r.note
		if r.aur {
			note += " · AUR"
		}
		cursor := "  "
		if i == p.sel {
			cursor, name = styleTitle.Render("▸ "), styleSelected.Render(name)
		}
		b.WriteString(cursor + name + styleMuted.Render(note) + "\n")
	}
	if len(shown) == 0 {
		b.WriteString(styleMuted.Render("  " + p.empty()))
	}
	return b.String()
}

// tabs is the group bar: each group and its count, the current one lit.
func (p *packages) tabs() string {
	var parts []string
	for g := pkgGroup(0); g < groupCount; g++ {
		label := fmt.Sprintf(" %s (%d) ", g, len(p.rows(g)))
		switch {
		case g == p.group:
			label = styleItemOn.Render(label)
		case g == groupMissing && len(p.rows(g)) > 0:
			label = styleWarn.Render(label)
		default:
			label = styleMuted.Render(label)
		}
		parts = append(parts, label)
	}
	return strings.Join(parts, "")
}

// summary is the line under the tabs: what this group is, and the cache.
func (p *packages) summary() string {
	what := map[pkgGroup]string{
		groupYours:   "installed by you: explicit, in no Hashiru manifest",
		groupMissing: "in a Hashiru manifest but not installed: the next 'hashiru update' installs them",
		groupOrphans: "dependencies nothing needs any more (pacman -Qdt)",
		groupForeign: "not from a sync repo: the AUR, or built by hand (pacman -Qm)",
		groupManaged: "installed from Hashiru's pacman/*.txt",
	}[p.group]
	line := styleMuted.Render(what)
	if p.pkgs.ManifestErr != nil {
		line = styleWarn.Render("no Hashiru manifests (" + p.pkgs.ManifestErr.Error() + "): everything explicit shows as yours")
	}
	cache := "cache " + system.HumanBytes(uint64(p.pkgs.CacheBytes))
	if p.pkgs.CacheErr != nil {
		cache = "cache unknown"
	}
	return line + styleMuted.Render("  ·  "+cache)
}

func (p *packages) empty() string {
	if p.input.Value() != "" {
		return "nothing matches"
	}
	if p.group == groupMissing {
		return "none: every manifest package is installed"
	}
	return "none"
}

func (p *packages) infoView(height int) string {
	var b strings.Builder
	r := p.infoOf
	b.WriteString(styleTitle.Render(r.name) + styleMuted.Render("  "+strings.Join(infoArgv(*r), " ")) + "\n\n")
	switch {
	case p.infoErr != nil:
		b.WriteString(styleFail.Render(p.infoErr.Error()))
		return b.String()
	case p.info == "":
		b.WriteString(styleMuted.Render("reading…"))
		return b.String()
	}
	lines := strings.Split(strings.TrimRight(p.info, "\n"), "\n")
	top := min(p.infoTop, max(len(lines)-1, 0))
	for i := top; i < len(lines) && i < top+height-2; i++ {
		// "Name            : htop": the field in the key colour.
		if k, v, ok := strings.Cut(lines[i], ":"); ok && !strings.HasPrefix(lines[i], " ") {
			b.WriteString(styleMuted.Render(k+":") + v + "\n")
			continue
		}
		b.WriteString(lines[i] + "\n")
	}
	return b.String()
}
