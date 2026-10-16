# Tasks (v3)

v2's tasks are in [v2/todo.md](v2/todo.md), v1's in [v1/todo.md](v1/todo.md).

Verification for every hashiru-tui task: `go test ./...`, `go vet ./...`,
`make build`. For Hashiru tasks: `bash -n` on touched scripts, the stage run
twice (`./install.sh N`), `./doctor.sh`.

## Phase 1: Tab deep links, Firmware

### Task 1: `Tabbed` and `area.tab` deep links
- [x] `Tabbed { Tabs() []string; Tab() int; SetTab(int) }`; Packages, Logs and Services implement it
- [x] `hashiru-tui packages.orphans` opens Packages on Orphans; tab part is the tab's short name, any case
- [x] Unknown tab: non-zero exit listing the area's tabs; tab part on an area without tabs: error
- [x] `--run` unchanged; `--list-actions` unchanged
**Files:** internal/tui/{start,area,packages,logs,services}.go, cmd/hashiru-tui/main.go, tests. **Deps:** none. **Size:** S

### Task 2: fwupd probes
- [ ] `system.FirmwareDevices` from `fwupdmgr get-devices --json`: name, vendor, version, updatable, flags
- [ ] `system.FirmwareUpdates` from `fwupdmgr get-updates --json`: device, version from → to, summary, needs reboot
- [ ] Exit codes captured on this machine; "no updates" is an empty list, not an error
- [ ] Fixtures from this T14s in testdata; fwupdmgr missing → a typed "not installed" error
**Files:** internal/system/firmware.go (new), firmware_test.go, testdata. **Deps:** none. **Size:** S

### Task 3: Hardware area with its Firmware tab
- [ ] Area `hardware` in the sidebar (eleventh row, no number key); tab bar with Firmware only for now
- [ ] Pending updates first and flagged, then devices; `enter` shows details
- [ ] `F` `hardware.firmware-refresh` and `u` `hardware.firmware-update` run Interactive through the launcher; re-probe after
- [ ] fwupd missing or daemon down: the tab says so with the command; `r` re-probes this tab
- [ ] Implements `Tabbed`; keys in `Keys()`; actions in palette and `--list-actions`
**Files:** internal/tui/hardware.go, hardware_firmware.go (new), app.go, tests. **Deps:** 1, 2. **Size:** M

### Checkpoint A
- [ ] Tests, vet, build; manual: `hashiru-tui packages.orphans`, `hashiru-tui hardware.firmware`, `F` and `u` in herdr and outside it

## Phase 2: Monitors

### Task 4: Monitor probe, layout model, Lua writer
- [ ] `system.Monitors` from `hyprctl monitors all -j` (fixture from this machine, plus a hand-made two-display fixture)
- [ ] Layout model: output, desc, mode (or preferred), position, scale, transform, disabled; logical size for placement
- [ ] `Place(layout, out, rel, of)` for left/right/above/below
- [ ] `MonitorsLua(layout)`: header marker, externals by `desc:`, `eDP-*` by name, no `disabled` for `eDP-*`, catch-all last; golden tests
- [ ] `HasHeader(file)` tells a generated override from a hand-written one
**Files:** internal/system/monitors.go (new), monitors_test.go, testdata. **Deps:** none. **Size:** M

### Task 5: Monitors tab: list, details, staged edits
- [ ] Rows per output; `enter` details with every mode
- [ ] `m` mode picker, `s` scale picker, `t` rotate, `p` place; staged changes shown as `old → new`; `esc` drops them
- [ ] Refuses a layout with no enabled output
**Files:** internal/tui/hardware_monitors.go (new), hardware.go, tests. **Deps:** 3, 4. **Size:** M

### Task 6: Apply with keep-or-revert
- [ ] `a` `hardware.monitors-apply`: hand-written override → confirm, copy to `monitors.lua.bak`
- [ ] Atomic write, `hyprctl reload`, re-probe; show the scale Hyprland used if different
- [ ] 15 s countdown overlay: `y` keeps; anything else or timeout restores the previous file (or removes it) and reloads
- [ ] Paths from Options; tests drive the countdown with a fake clock and fake reload
**Files:** internal/tui/hardware_monitors.go, a small overlay (reuse the quit-confirm pattern), tests. **Deps:** 5. **Size:** M

### Checkpoint B
- [ ] Tests, vet, build; manual: scale change reverts on timeout; kept change survives a Hyprland restart; `.bak` made; docked/lid-shut if a display is at hand

## Phase 3: Hashiru (in /opt/hashiru; per SPEC-idle-override.md, SPEC-charge-thresholds.md)

### Task 7: idle-override
- [ ] `hypridle-battery`: override-or-stowed resolution per mode; `reload` argument
- [ ] Stowed confs' headers point at the override paths; `doctor.sh` checks hypridle runs on an existing override
- [ ] Spec's "Done when" by hand
**Files:** stow/hyprland/.config/hypr/scripts/hypridle-battery, hypridle-{ac,battery}.conf, doctor.sh. **Deps:** none. **Size:** S

### Task 8: charge-thresholds
- [ ] `hashiru charge [START STOP | off]` with validation that changes nothing on refusal
- [ ] Stage 10 `apply_charge_thresholds`: idempotent, does nothing when the knobs are unset
- [ ] `hashiru.conf.example` knobs; `doctor.sh` checks sysfs matches; `hashiru help` lists it
**Files:** bin/hashiru, scripts/10-base.sh, hashiru.conf.example, doctor.sh. **Deps:** none. **Size:** M

### Task 9: Hashiru release and site docs
- [ ] Release notes in `docs/releases/`; site: configuration (override files, knobs), cli (`charge`), changelog row
- [ ] **Ask before** tagging, pushing the repo and pushing the wiki
**Files:** docs/releases/, ~/projects/hashiru-wiki/public/docs/*. **Deps:** 7, 8. **Size:** S

### Checkpoint C
- [ ] Both specs' "Done when"; `./doctor.sh` clean; `hashiru update` leaves the checkout clean

## Phase 4: Power

### Task 10: Power probes and the hypridle rewriter
- [ ] Batteries from `<root>/sys/class/power_supply/BAT*`: capacity, status, health, cycles, thresholds (absent → unsupported); AC online
- [ ] `platform_profile` and choices; `tlp-stat -s` profile and power source, parsed loosely
- [ ] hypridle conf: read lock/screen-off timeouts; `RewriteTimeouts(conf, lock, off)` changes only `timeout =` lines (golden test against the stowed files)
- [ ] Capability probes: `hashiru help` lists `charge`; `hypridle-battery` supports `reload`
**Files:** internal/system/power.go (new), power_test.go, testdata. **Deps:** none (fixtures can predate Task 7, 8). **Size:** M

### Task 11: Power tab, read-only
- [ ] Battery, profile, idle (AC and battery, override or stowed, labelled), thresholds sections
- [ ] No battery: says so, rest works; old Hashiru: notes beside the controls it can't offer
**Files:** internal/tui/hardware_power.go (new), hardware.go, tests. **Deps:** 3, 10. **Size:** M

### Task 12: Power actions
- [ ] `p` `hardware.power-profile`: picker → `sudo tlp <profile>` Interactive
- [ ] `i` `hardware.idle`: minutes for lock (and screen-off) per source → write override atomically → `hypridle-battery reload`; `I` `hardware.idle-reset` removes overrides and reloads
- [ ] `c` `hardware.charge`: start/stop input, validated → `hashiru charge S E` Interactive; `C` `hardware.charge-off`
- [ ] Re-probe after each; keys in `Keys()`, actions in the palette
**Files:** internal/tui/hardware_power.go, tests. **Deps:** 7, 8, 11. **Size:** M

### Checkpoint D
- [ ] Tests, vet, build; README (Areas, Keys, deep links to tabs); SPEC v3 "Done when"; full manual pass of every area
