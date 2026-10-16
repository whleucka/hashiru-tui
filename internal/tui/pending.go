package tui

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/whleucka/hashiru-tui/internal/system"
)

// pendingLine is the one-line count: "16 pacman · 2 AUR".
func pendingLine(st *State) string {
	switch {
	case st.Pending == nil && st.PendingBusy:
		return styleMuted.Render("checking… (syncs the package databases)")
	case st.Pending == nil:
		return styleMuted.Render("unknown")
	}
	p := st.Pending
	var parts []string
	if p.PacmanErr != nil {
		parts = append(parts, styleFail.Render("pacman: "+p.PacmanErr.Error()))
	} else {
		parts = append(parts, countStyle(len(p.Pacman)).Render(fmt.Sprintf("%d pacman", len(p.Pacman))))
	}
	switch {
	case p.AURMissing:
		parts = append(parts, styleMuted.Render("no yay"))
	case p.AURErr != nil:
		parts = append(parts, styleFail.Render("AUR: "+p.AURErr.Error()))
	default:
		parts = append(parts, countStyle(len(p.AUR)).Render(fmt.Sprintf("%d AUR", len(p.AUR))))
	}
	return strings.Join(parts, styleMuted.Render(" · "))
}

func countStyle(n int) lipgloss.Style {
	if n > 0 {
		return styleWarn
	}
	return styleMuted
}

// pendingRows lists packages, at most limit of them (0 for all), then how
// many more there are.
func pendingRows(st *State, limit int) []string {
	if st.Pending == nil {
		return nil
	}
	type group struct {
		name string
		pkgs []system.Package
	}
	var rows []string
	shown := 0
	for _, g := range []group{{"pacman", st.Pending.Pacman}, {"AUR", st.Pending.AUR}} {
		for _, p := range g.pkgs {
			if limit > 0 && shown == limit {
				break
			}
			rows = append(rows, "  "+cell(p.Name, 24)+" "+
				styleMuted.Render(p.Old+" → "+p.New)+" "+styleMuted.Render(g.name))
			shown++
		}
	}
	if total := len(st.Pending.Pacman) + len(st.Pending.AUR); total > shown {
		rows = append(rows, styleMuted.Render(fmt.Sprintf("  … %d more (p to show all)", total-shown)))
	}
	return rows
}
