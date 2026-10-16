# Tasks (v2)

v1's tasks are in [v1/todo.md](v1/todo.md).

Verification for every task: `go test ./...`, `go vet ./...`, `make build`.

## Phase 1: Keys and palette

### Task 1: Keys as data
`Area.Help() string` becomes `Area.Keys() []Key`; the footer renders from it.
- [x] `Key{Keys, Desc, Group}`; every area converted, nothing lost from today's help strings
- [x] Shell's global keys (sidebar, palette, refresh, quit) are one list
- [x] Footer shows the focused area's keys that fit the width, then `? keys`; `TestFooterHelpMatchesFocus` still holds
**Files:** internal/tui/{area,view}.go, every area file, tests. **Deps:** none. **Size:** M (wide, shallow)

### Task 2: `?` keymap popup
- [x] `?` from the sidebar or an area opens a centered popup over the area: area keys by group, then global keys
- [x] `?`, `esc`, `q` close it; it takes every key while open
- [x] While a text input has focus (Keybinds/Logs filter, palette), `?` goes to the input
- [x] Wallpaper hides its kitty preview while the popup is open
**Files:** internal/tui/keyhelp.go (new), app.go, view.go, wallpaper.go, tests. **Deps:** 1. **Size:** M

### Task 3: Palette: IDs, global actions, recents
- [ ] `Action.ID` on every action (`<area>.<verb>`), unique, tested
- [ ] Palette lists global actions too: refresh, show keys, quit
- [ ] Running an action records its ID in `~/.local/state/hashiru-tui/recent` (last 10); with an empty filter, recents come first
- [ ] Path injected through Options; unreadable file means no recents
**Files:** internal/tui/{area,palette,app}.go, recent.go (new), every area's Actions, tests. **Deps:** 1. **Size:** M

### Checkpoint A
- [ ] Tests, vet, build; manual run of the checks in plan.md

## Phase 2: Plumbing

### Task 4: Deep links
- [ ] `hashiru-tui <area>` opens focused on that area (name, case-insensitive); unknown area prints the list, exits 2
- [ ] `hashiru-tui --run <id>` opens, then runs the action once areas have loaded; unknown ID exits 2
- [ ] `hashiru-tui --list-actions` prints `id<TAB>title`, one per line, and exits
- [ ] Usage line and README updated
**Files:** cmd/hashiru-tui/main.go, internal/tui/{app,options}.go, tests. **Deps:** 3. **Size:** S

### Task 5: Changelog in Updates
- [ ] Parse `hashiru changelog` output (`===== vX.Y.Z.md =====` sections) into tag + body
- [ ] Behind: notes for the versions newer than this one (see the risk in plan.md); up to date: the current version's
- [ ] Shown as a collapsible section in Updates; `c` opens the full notes in a pager (Interactive)
**Files:** internal/hashiru/changelog.go + test + testdata, internal/tui/updates.go. **Deps:** none. **Size:** M

### Task 6: Re-apply a stage
- [ ] Stages from `hashiru install --list`
- [ ] `s` in Updates opens a picker; `enter` runs `hashiru install N` Interactive and refreshes after
- [ ] One palette action per stage ("Re-apply stage 45-config", ID `updates.stage-45`)
**Files:** internal/hashiru/cli.go + test, internal/tui/updates.go, tests. **Deps:** 3. **Size:** S

### Task 7: Overview auto-refresh
- [ ] Uptime, memory, battery, disk reload every 5s while Overview is the selected area; nothing else re-runs
- [ ] Ticker injected so tests don't sleep
**Files:** internal/tui/overview.go, overview_test.go. **Deps:** none. **Size:** S

### Checkpoint B
- [ ] Tests, vet, build; manual run of the checks in plan.md

## Phase 3: Packages

### Task 8: Package probes
- [ ] Parse manifests: `pacman/*.txt` under `repo=`, comments and blanks skipped, `aur.txt` marked AUR
- [ ] `pacman -Qqe`, `-Qqm`, `-Qdtq`, `-Sgq` (group expansion); exit 1 with no output means empty
- [ ] Pure `Classify(manifests, explicit, foreign, orphans, groups)` → managed / yours / missing / orphan / foreign, tested on captured output
- [ ] Cache size by walking `/var/cache/pacman/pkg` (root injected)
**Files:** internal/system/packages.go + test + testdata. **Deps:** none. **Size:** M

### Task 9: Packages area
- [ ] New sidebar entry: groups with counts, cache size; `tab` cycles groups, `/` filters
- [ ] `enter` shows `pacman -Qi <pkg>` in a detail pane
- [ ] Missing tool or no repo path is a visible line, not an error
**Files:** internal/tui/packages.go + test, area.go. **Deps:** 1, 8. **Size:** M

### Task 10: Package actions
- [ ] `S` search: query → `pacman -Ss` and `yay -Ssa` results in one list, installed ones marked
- [ ] `i` install (`yay -S`), `x` remove (`sudo pacman -Rns`), `O` remove orphans, `C` clean cache (`paccache -r`); all Interactive, refresh after
- [ ] Palette actions with IDs for the non-package-specific ones
**Files:** internal/system/packages.go (search parser) + test, internal/tui/packages.go + test. **Deps:** 9. **Size:** M

### Checkpoint C
- [ ] Tests, vet, build; manual run of the checks in plan.md

## Phase 4: Services

### Task 11: systemctl probes
- [ ] `systemctl [--user] list-units --type=service,timer --all --plain --no-legend`, `list-unit-files`, `list-timers --all`, parsed
- [ ] Merged into one unit list: name, load/active/sub, enabled, description, next run (timers)
**Files:** internal/system/services.go + test + testdata. **Deps:** none. **Size:** M

### Task 12: Services area
- [ ] New sidebar entry; `u` toggles system/user, `f` failed only, `/` filters
- [ ] `s`/`S`/`R` start/stop/restart, `e`/`d` enable/disable: system via launcher with `sudo`, user directly with a note; refresh after
- [ ] `l` opens `journalctl -u <unit> -f` (Watch)
- [ ] Logs' failed units: `enter` jumps to the unit in Services
**Files:** internal/tui/services.go + test, logs.go, area.go. **Deps:** 1, 11. **Size:** M

### Checkpoint D
- [ ] Tests, vet, build; README updated (areas, keys, deep links); full manual pass
