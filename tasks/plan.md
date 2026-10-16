# Implementation Plan: hashiru-tui v2, power-user round (SPEC.md "v2")

v1's plan is in [v1/plan.md](v1/plan.md).

## Overview

Make the tool faster to drive and reach into, then add Packages and
Services: keys as data with a `?` popup, a palette with IDs and recents, deep
links, changelog and stage re-apply in Updates, a self-refreshing Overview,
and the two new areas.

## Architecture decisions

- **Keys are data.** `Area.Help() string` becomes `Area.Keys() []Key`
  (`Key{Keys, Desc, Group string}`). The footer and the `?` popup both render
  from it. Global keys are one list in the shell.
- **Overlays are one mechanism.** The palette and the keymap popup are both
  "an overlay takes every key while open". The popup is drawn over the area
  (lipgloss layers), not in place of it, so you still see what the keys act
  on.
- **Actions have IDs.** `Action` gains `ID` (`<area>.<verb>`, lowercase,
  stable). The palette, deep links and `--list-actions` all use it; titles
  can change, IDs don't.
- **Recents live in `~/.local/state/hashiru-tui/recent`**, a few lines of
  IDs. Missing or unreadable means no recents, never an error.
- **Same probe rules as v1.** New commands (`pacman -Q*`, `systemctl`,
  `hashiru changelog`) go through `Exec`; parsers are pure and tested on
  output captured from this machine.
- **Anything that changes the machine runs through the launcher.** Package
  install/remove and system unit changes are Interactive so sudo and pacman
  prompts are visible. Only `systemctl --user` actions run directly.

## Task list

### Phase 1: Keys and palette
- [x] Task 1: Keys as data; the footer renders from them
- [x] Task 2: `?` keymap popup
- [x] Task 3: Action IDs, global actions and recents in the palette

### Checkpoint A
- [ ] Tests, vet, build; manual: `?` in every area, `?` inside a filter types `?`, palette recents survive a restart

### Phase 2: Plumbing
- [x] Task 4: Deep links (`<area>`, `--run`, `--list-actions`)
- [x] Task 5: Changelog in Updates
- [x] Task 6: Re-apply a stage
- [ ] Task 7: Overview refreshes local stats on a timer

### Checkpoint B
- [ ] Tests, vet, build; manual: `hashiru-tui --run snapshots.new` from a shell, stage 45 re-applied from the TUI

### Phase 3: Packages
- [ ] Task 8: Package probes and manifest parsing
- [ ] Task 9: Packages area (groups, filter, info)
- [ ] Task 10: Search, install, remove, orphans, cache

### Checkpoint C
- [ ] Tests, vet, build; manual: groups match `pacman -Qqe` minus the manifests by hand; install and remove a small package

### Phase 4: Services
- [ ] Task 11: systemctl probes (units, unit files, timers; system and user)
- [ ] Task 12: Services area and actions; Logs' failed units jump here

### Checkpoint D
- [ ] Tests, vet, build; README updated for v2; full manual pass of every area

## Risks and mitigations

| Risk | Impact | Mitigation |
|---|---|---|
| Release notes for versions you're behind aren't in the local checkout until the pull | Med | Read them from the fetched ref (`git -C repo show origin/main:...`) if `hashiru changelog` can't; otherwise show "N commits behind" and the current notes. Decide in Task 5 after looking at how Hashiru stores them |
| Manifest entries that are groups (e.g. `base-devel`) don't appear in `pacman -Qqe` | Med | Expand groups with `pacman -Sgq`; fixture test with a group entry |
| `?` collides with a key an area already uses | Low | Task 1 audits every area's keys; none use `?` today |
| Overlay compositing in lipgloss v2 misbehaves with kitty image placeholders on Wallpaper | Low | Popup hides the preview while open; Wallpaper already redraws it on resize |
| Auto-refresh redraws fight with an open overlay or input | Low | Timer only updates Overview's data; views redraw from state as now |
| sudo prompts inside a herdr pane vs. ExecProcess differ | Low | Same Interactive path as `hashiru update`, which already works in both |

## Open questions

- **Services: tag Hashiru's own units?** Showing which units Hashiru enabled
  (reflector, snapper timers, the wallpaper daemon) means either a list in
  Hashiru (e.g. `hashiru status` printing them) or a hardcoded list here that
  drifts. Not in v2 unless you want the Hashiru change; v2 shows all units
  with filters.
