# Implementation Plan: hashiru-tui control center (SPEC.md v1)

## Overview

Turn the tab-based Hashiru viewer into the control center SPEC.md describes:
a sidebar of areas, one launcher for other tools, a command palette, and the
v1 areas (Overview, Updates, Keybinds, Connectivity, Wallpaper, Snapshots,
Logs, Config), finishing with the Hashiru side (wallpaper mode, keybind,
installing the binary).

## Architecture decisions

- **Area interface.** Each area is a type implementing `Name`, `Update`
  (every non-key message), `HandleKey` (gets first refusal on keys while
  focused, returns handled), `View(w, h)`, `Help`, and `Actions` (for the
  palette). Adding an area is one file and one line in the registry.
- **Shared state, loaded once.** Status, doctor and the log live in one
  `State` the shell owns. Overview and Updates both read it, so nothing is
  fetched twice.
- **One launcher, three kinds** (Interactive, Watch, GUI), as in SPEC.md.
  herdr pane when present; otherwise ExecProcess for terminal tools and a
  detached `setsid` process for GUI ones. Detaching is injected so tests
  start nothing.
- **Probes are injected, parsers are pure.** Every external command an area
  reads (`checkupdates`, `nmcli`, `hyprctl binds`, `snapper`...) goes through
  one `Exec` function in Options. Parsing is a pure function tested against
  output captured from this machine in `testdata/`.
- **Areas read, the launcher changes.** Anything that changes the machine
  either runs through the launcher (the user sees and confirms it in the
  real tool) or is a single explicit action with a visible note. No silent
  writes.

## Task list

### Phase 1: Shell
- [x] Task 1: Sidebar shell with the Area interface; existing views become Overview, Updates, Logs
- [x] Task 2: Launcher with Interactive / Watch / GUI kinds
- [x] Task 3: Command palette over areas and actions

### Checkpoint A
- [x] `go test ./...` and `make build` pass; run in a herdr split, all three areas work, palette opens and runs an action

### Phase 2: Updates and the everyday areas
- [x] Task 4: Pending pacman and AUR updates in Updates and Overview
- [ ] Task 5: Arch news in Updates
- [ ] Task 6: Keybinds area
- [ ] Task 7: Connectivity area (network, bluetooth, audio)

### Checkpoint B
- [ ] Tests and build pass; manual run against the real machine

### Phase 3: The rest of v1
- [ ] Task 8: Overview system summary (host, kernel, uptime, memory, disk, battery)
- [ ] Task 9: Snapshots area
- [ ] Task 10: Logs sources: journal errors and failed units
- [ ] Task 11: Config area
- [ ] Task 12: Wallpaper area

### Checkpoint C
- [ ] Tests and build pass; full manual pass of every area

### Phase 4: Hashiru side (/opt/hashiru)
- [ ] Task 13: tahoe-wallpaper respects a fixed wallpaper choice
- [ ] Task 14: SUPER+ALT+Space bind and installing the binary (**needs a decision first, see Open questions**)

## Risks and mitigations

| Risk | Impact | Mitigation |
|---|---|---|
| Kitty graphics in a floating window or herdr pane don't render | Med | Wallpaper works without images (names + apply); preview is an extra, off when the terminal doesn't support it |
| `hyprctl binds` output changes between Hyprland versions | Low | Same parser approach as `hypr/scripts/keybinds`; fixture test catches drift |
| Network calls (Arch news, `checkupdates`) slow the UI | Med | Every probe runs off the UI thread with a timeout; areas show "loading" and "unknown" |
| A probe tool is missing (no yay, no bluetooth) | Low | Missing tool is a visible "not installed" line, never an error that blocks the area |
| Task 13/14 change the live desktop when Hashiru next updates | Med | Commits only, not pushed, not applied; you run `hashiru install 45` when ready |

## Open questions

- **Task 14: how the binary reaches the machine.** hashiru-tui has no git
  remote yet, so a stage can't clone or download it. Options: push
  hashiru-tui to GitHub and have a stage `go install` a pinned tag; or vendor
  a built binary into Hashiru (not great in git). The run will stop before
  Task 14 to ask.
