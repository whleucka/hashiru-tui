package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/whleucka/hashiru-tui/internal/system"
)

// searchTimeout covers yay asking the AUR over the network.
const searchTimeout = 30 * time.Second

type searchMsg struct{ s system.Search }

// Everything here changes the machine, so it all runs through the
// launcher as Interactive: pacman's and yay's own prompts (sudo, "Proceed
// with installation?") are where you confirm, and the TUI refreshes after.

func (p *packages) Actions() []Action {
	return []Action{
		{ID: "packages.search", Title: "Search the repos and the AUR", Key: "S", Run: p.startSearch},
		{ID: "packages.remove-orphans", Title: "Remove orphaned packages (pacman -Rns)", Key: "O", Run: p.removeOrphans},
		{ID: "packages.clean-cache", Title: "Clean the package cache (paccache -r)", Key: "C", Run: p.cleanCache},
	}
}

// startSearch goes to the Search tab with the query box focused.
func (p *packages) startSearch() tea.Cmd {
	p.group, p.sel, p.offset = groupSearch, 0, 0
	p.querying = true
	return p.query.Focus()
}

func (p *packages) runSearch(query string) tea.Cmd {
	if strings.TrimSpace(query) == "" {
		return nil
	}
	p.searchBusy = true
	probe := p.e.probe
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), searchTimeout)
		defer cancel()
		return searchMsg{system.SearchPackages(ctx, probe, query)}
	}
}

// searchRows are the results, the repos' first: they're what pacman
// installs without building anything.
func (p *packages) searchRows() []pkgRow {
	if p.search == nil {
		return nil
	}
	var out []pkgRow
	for _, f := range append(append([]system.Found{}, p.search.Repo...), p.search.AUR...) {
		out = append(out, pkgRow{
			name:      f.Name,
			note:      f.Repo + " " + f.Version,
			desc:      f.Description,
			aur:       f.AUR(),
			installed: f.Installed,
		})
	}
	return out
}

func (p *packages) selected() (pkgRow, bool) {
	shown := p.shown()
	if p.sel >= len(shown) {
		return pkgRow{}, false
	}
	return shown[p.sel], true
}

// installArgv: pacman for a repo package, yay for an AUR one.
func installArgv(r pkgRow) []string {
	if r.aur {
		return []string{"yay", "-S", r.name}
	}
	return []string{"sudo", "pacman", "-S", r.name}
}

func (p *packages) install() tea.Cmd {
	r, ok := p.selected()
	switch {
	case !ok:
		return nil
	case r.installed:
		return note(r.name + " is already installed")
	}
	return p.e.launch(Launch{Argv: installArgv(r), Label: "install " + r.name, Kind: Interactive, Refresh: true})
}

func (p *packages) remove() tea.Cmd {
	r, ok := p.selected()
	switch {
	case !ok:
		return nil
	case !r.installed:
		return note(r.name + " isn't installed")
	}
	label := "remove " + r.name
	if p.group == groupManaged {
		label += " (Hashiru's: the next update puts it back)"
	}
	return p.e.launch(Launch{Argv: []string{"sudo", "pacman", "-Rns", r.name}, Label: label, Kind: Interactive, Refresh: true})
}

func (p *packages) removeOrphans() tea.Cmd {
	if p.pkgs == nil || len(p.pkgs.Orphans) == 0 {
		return note("no orphans")
	}
	argv := append([]string{"sudo", "pacman", "-Rns"}, p.pkgs.Orphans...)
	return p.e.launch(Launch{Argv: argv, Label: fmt.Sprintf("remove %d orphans", len(p.pkgs.Orphans)), Kind: Interactive, Refresh: true})
}

// cleanCache keeps the last three versions of each package, paccache's
// default, so a downgrade is still possible.
func (p *packages) cleanCache() tea.Cmd {
	return p.e.launch(Launch{Argv: []string{"sudo", "paccache", "-r"}, Label: "clean the cache", Kind: Interactive, Refresh: true})
}

func note(text string) tea.Cmd {
	return func() tea.Msg { return noteMsg{text: text} }
}

// searchStatus is the line under the query: counts, or why there are none.
func (p *packages) searchStatus() string {
	s := p.search
	switch {
	case p.searchBusy:
		return styleMuted.Render("searching the repos and the AUR…")
	case s == nil:
		return styleMuted.Render("S or / to search the repos and the AUR · i installs the selection")
	}
	parts := []string{fmt.Sprintf("%d in the repos", len(s.Repo))}
	if s.RepoErr != nil {
		parts[0] = styleFail.Render("repos: " + s.RepoErr.Error())
	}
	switch {
	case s.AURMissing:
		parts = append(parts, "no yay, so no AUR")
	case s.AURErr != nil:
		parts = append(parts, styleFail.Render("AUR: "+s.AURErr.Error()))
	default:
		parts = append(parts, fmt.Sprintf("%d in the AUR", len(s.AUR)))
	}
	return styleMuted.Render(strings.Join(parts, " · "))
}
