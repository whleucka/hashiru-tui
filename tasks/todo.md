# Tasks

Verification for every task: `go test ./...`, `go vet ./...`, `make build`.

## Phase 1: Shell

### Task 1: Sidebar shell with the Area interface
Tabs become a sidebar; the Dashboard, Doctor and Log views move into Overview, Updates and Logs areas, reading one shared State.
- [x] Sidebar lists areas; `j`/`k` (or number keys) select, `enter`/`l` focuses the area, `esc`/`h` returns to the sidebar when the area doesn't use the key
- [x] Status, doctor and log load once and both Overview and Updates show them; `r` reloads
- [x] Existing behaviour survives: doctor expand, log runs/filter/follow, `u` update
**Files:** internal/tui/{app,area,state,view,overview,updates,logs}.go, tests. **Deps:** none. **Size:** L (mostly moves)

### Task 2: Launcher with three kinds
- [x] Interactive: herdr pane focused, else ExecProcess, refreshes state on exit when asked
- [x] Watch: herdr pane unfocused, else ExecProcess
- [x] GUI: detached process through an injected starter (default `setsid`), TUI stays; error shows as a note
**Files:** internal/tui/launch.go, launch_test.go, app.go. **Deps:** 1. **Size:** S

### Task 3: Command palette
- [x] `ctrl+p` (and `:` from the sidebar) opens a filter over "Go to <area>" and every area's actions
- [x] Typing filters by words in any order; `enter` runs; `esc` closes
**Files:** internal/tui/palette.go, palette_test.go, app.go, view.go. **Deps:** 1, 2. **Size:** M

### Checkpoint A
- [x] Tests, vet, build; manual run in a herdr split (fixed: palette placeholder cut to one cell; sidebar footer mixing in the area's keys)

## Phase 2

### Task 4: Pending pacman and AUR updates
- [x] `checkupdates` and `yay -Qua` parsed into package/old/new; exit 2 from checkupdates means none
- [x] Updates lists them; Overview shows counts; missing yay shows "not installed"
**Files:** internal/system/{exec,updates}.go + tests, internal/tui/updates.go, overview.go. **Deps:** 1. **Size:** M

### Task 5: Arch news
- [ ] Fetch archlinux.org/feeds/news/ (timeout), parse RSS titles, dates, links
- [ ] Items newer than the machine's last update are flagged; `enter`/`o` opens the link with xdg-open (GUI kind)
**Files:** internal/system/news.go + test, internal/tui/updates.go. **Deps:** 2, 4. **Size:** S

### Task 6: Keybinds area
- [ ] Parse `hyprctl binds` plain text (modmask → Super/Ctrl/Alt/Shift, key, description; skip submaps and undescribed)
- [ ] List sorted by modifiers; `/` filters as you type
**Files:** internal/system/keybinds.go + test, internal/tui/keybinds.go. **Deps:** 1. **Size:** M

### Task 7: Connectivity area
- [ ] Network: active connections from `nmcli -t`; Bluetooth: powered + connected devices from `bluetoothctl`; Audio: default sink and volume from `wpctl`
- [ ] Actions: nmtui, bluetoothctl (Interactive), blueman-manager, pavucontrol (GUI)
**Files:** internal/system/{network,bluetooth,audio}.go + tests, internal/tui/connectivity.go. **Deps:** 2. **Size:** M

### Checkpoint B
- [ ] Tests, vet, build; manual run

## Phase 3

### Task 8: Overview system summary
- [ ] Host, kernel, uptime, memory, root disk use, battery (from /proc, /sys, statfs; no shelling out)
**Files:** internal/system/sysinfo.go + test, internal/tui/overview.go. **Deps:** 1. **Size:** S

### Task 9: Snapshots area
- [ ] `snapper -c root list` parsed (number, date, type, description, newest first)
- [ ] `enter` shows `snapper status N..0` (changed files); `n` creates one with a description (Interactive: `sudo snapper -c root create -d ...`)
**Files:** internal/system/snapper.go + test, internal/tui/snapshots.go. **Deps:** 2. **Size:** M

### Task 10: Logs sources
- [ ] Logs has sources: Hashiru (existing), Journal errors (`journalctl -p err -b -o short-iso`), Failed units (`systemctl --failed` system + user)
- [ ] Actions: follow journal (Watch), `dmesg -w` (Watch, via sudo)
**Files:** internal/system/journal.go + test, internal/tui/logs.go. **Deps:** 2. **Size:** M

### Task 11: Config area
- [ ] List of Hashiru-changeable files grouped by tier (hashiru.conf; hypr/local.lua + module overrides; kitty, waybar; ~/.zshrc.local), showing which exist
- [ ] `enter` opens `$EDITOR` (Interactive); `hashiru.conf` goes through `hashiru config` so it gets seeded
**Files:** internal/tui/config.go + test. **Deps:** 2. **Size:** S

### Task 12: Wallpaper area
- [ ] Lists images in ~/.config/hypr/wallpapers and ~/Pictures/Wallpapers; shows the current one (`awww query`)
- [ ] `enter` applies with `awww img` and records it as the fixed choice in ~/.config/hashiru/wallpaper; `c` returns to the time-of-day cycle
- [ ] Preview of the selected image through kitty graphics when supported, else none
**Files:** internal/system/wallpaper.go + test, internal/tui/wallpaper.go. **Deps:** 2. **Size:** M

### Checkpoint C
- [ ] Tests, vet, build; manual pass of every area

## Phase 4: Hashiru (/opt/hashiru, commits only, not pushed or applied)

### Task 13: tahoe-wallpaper respects a fixed choice
- [ ] If ~/.config/hashiru/wallpaper names a readable file, the daemon applies it and stops cycling; removing it resumes the cycle
- [ ] shellcheck clean; doctor unaffected
**Files:** stow/hyprland/.config/hypr/scripts/tahoe-wallpaper. **Deps:** 12. **Size:** XS

### Task 14: Keybind and install (STOP: needs a decision)
- [ ] Decide how the binary reaches the machine (no remote yet)
- [ ] SUPER+ALT+Space opens it floating; a stage installs it
**Deps:** 13 + decision.
