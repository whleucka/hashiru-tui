# Implementation Plan: hashiru-tui v3, hardware round (SPEC.md "v3")

v2's plan is in [v2/plan.md](v2/plan.md), v1's in [v1/plan.md](v1/plan.md).

## Overview

One Hardware area with Monitors, Power and Firmware tabs, deep links that
reach any area's tab, and the two Hashiru changes Power needs (idle-timeout
overrides, `hashiru charge`). Build order follows the approved capability
map: firmware → monitors → idle-override, charge-thresholds → power.

## Architecture decisions

- **Tabs get an interface.** `Tabbed { Tabs() []string; Tab() int; SetTab(int) }`,
  implemented by Packages, Logs, Services and Hardware. `Start` resolves a positional
  `area.tab` through it, so deep links to tabs are one code path for every
  tabbed area. `--run` still only takes action IDs.
- **Hardware is one area holding three tab models**, each with its own
  probe, keys and actions; the area routes keys to the current tab and
  concatenates their `Keys()` with the tab key. A tab's probe failing is
  shown in that tab only.
- **Monitors: a layout model in `internal/system`**, separate from Hyprland's
  JSON. Probe → layout; staged edits are a copy of the layout; a pure
  `MonitorsLua(layout) []byte` writes the override. Apply = write file,
  `hyprctl reload`, keep-or-revert. No `hyprctl eval` (see SPEC: it desyncs).
- **Every file the TUI writes is under `~/.config/hashiru/hypr/`**, written
  to a temp file and renamed, with the previous content held in memory for
  revert. Paths come from `Options`, so tests use a temp dir.
- **Sysfs and conf reads take a root path** (`/` in production, a temp tree
  in tests), like the existing probes take `Exec`.
- ~~Hashiru capability probes~~ Dropped 2026-10-16: no backwards compat
  with an older Hashiru; the TUI assumes the one it ships with.
- **Root goes through the launcher as Interactive**: `fwupdmgr refresh|update`,
  `sudo tlp <profile>`, `hashiru charge`. Writing the idle override and
  running `hypridle-battery reload` are the user's own files and process, so
  they run directly, like `systemctl --user` in Services.

## Task list

### Phase 1: Tab deep links, Firmware
- [x] Task 1: `Tabbed` and `area.tab` deep links (Packages, Logs)
- [x] Task 2: fwupd probes
- [x] Task 3: Hardware area with its Firmware tab

### Checkpoint A
- [x] Tests, vet, build; manual: `hashiru-tui packages.orphans`, `hashiru-tui hardware.firmware`, `F` and `u` open fwupdmgr in a herdr pane and outside herdr

### Phase 2: Monitors
- [x] Task 4: Monitor probe, layout model, Lua writer
- [x] Task 5: Monitors tab: list, details, staged edits
- [x] Task 6: Apply with keep-or-revert

### Checkpoint B
- [x] Tests, vet, build; manual: change eDP-1's scale, let it revert; change again and keep; restart Hyprland and it holds; a hand-written override gets a `.bak`; docked with an external display if one is at hand

### Phase 3: Hashiru (in /opt/hashiru)
- [x] Task 7: idle-override
- [x] Task 8: charge-thresholds
- [x] Task 9: Hashiru release and site docs

### Checkpoint C
- [x] Each spec's "Done when" by hand; `./doctor.sh` clean; `git status` clean after `hashiru update` on this machine

### Phase 4: Power
- [x] Task 10: Power probes and the hypridle rewriter
- [x] Task 11: Power tab, read-only
- [x] Task 12: Power actions

### Checkpoint D
- [x] Tests, vet, build; README updated for v3; SPEC's v3 "Done when" all hold; full manual pass of every area

## Risks and mitigations

| Risk | Impact | Mitigation |
|---|---|---|
| `hyprctl reload` while docked with the lid shut relights the panel or fights `clamshell.lua` | High | Checkpoint B tests it docked and lid-shut if a display is at hand; generated file leaves `eDP-*` enable/disable to clamshell. If reload misbehaves, ask before touching `clamshell.lua` (spec: ask first) |
| A bad mode blacks out the screen the prompt is on | Med | The revert runs on a timer in the TUI, not on input; a blank screen still reverts in 15 s. Test with a deliberately wrong mode in a nested/headless output if possible |
| Hyprland rounds a scale it can't use | Low | Re-probe after reload, show the scale it used beside the one asked for |
| `fwupdmgr get-updates` exits non-zero for "no updates" | Med | Capture exit codes on this machine in Task 2; treat the "nothing to do" code as an empty list, fixture-tested |
| `tlp-stat -s` wording changes between TLP versions | Low | Parse `key = value` lines loosely; a missing field shows "unknown", never an error |
| Hashiru's two changes ship after the TUI's Power tab | Med | Phase 3 lands (and released, v1.14.0) before Phase 4; no fallback for an older Hashiru |
| Eleven areas, number keys reach ten | Low | Decided: Hardware has no number key |

## Open questions

- ~~Hardware's number key~~ Decided 2026-10-16: none. Hardware is the
  eleventh row, reached by `j`/`k`, the palette and deep links.
- **Task 9 is outward-facing** (tag, GitHub release, pushing the site). It
  will stop and ask before tagging and pushing, as any release does.
