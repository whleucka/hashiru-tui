# hashiru-tui: spec

A keyboard-driven control center for a Hashiru machine: the place to update it,
check it, change it and reach the tools that manage it.

Decided 2026-10-15.

## Audience and stance

- **For:** the maintainer, and people who already know Arch. Dense, fast, no
  hand-holding text. Every screen shows the command it is wrapping, so it
  teaches by exposing rather than explaining.
- **Not:** a reimplementation of Hashiru, or of any tool that already works.

**Delegate where a good tool exists; build only where none does.** btop, nmtui,
bluetoothctl and journalctl are fine. The TUI makes them one keypress away and
shows a summary. It builds a native screen only where there's nothing good
(wallpaper, keybinds, snapshots) or where a Hashiru-shaped view adds something
(updates, doctor).

## Launching

Both, with the same binary:

- `hashiru-tui` from a terminal (later also `hashiru tui`).
- **`SUPER+ALT+Space`** opens it in a floating kitty window, the way
  `SUPER+SHIFT+U` floats `update-system` today. Like omarchy's walker menu for
  things other than apps: `SUPER+Space` launches apps, `SUPER+ALT+Space` runs the
  system. The bind goes in Hashiru (`stow/hyprland/.config/hypr/keybinds.lua`),
  not here, and lands with build step 7, once there is a binary to point at.

## Running other tools

One runner, three cases, picked per action:

| Kind | Example | Inside herdr | Otherwise |
|---|---|---|---|
| Terminal, interactive | nmtui, bluetoothctl, `hashiru update`, `$EDITOR` | new pane, focused | `tea.ExecProcess` (takes over this window until it exits) |
| Terminal, to watch | `journalctl -f`, btop | new pane, focused (was unfocused until 2026-10-16: you open a pane to look at it) | `tea.ExecProcess` |
| GUI | blueman-manager, pavucontrol | detached (`setsid`), the TUI stays | same |

herdr is an extra, never required. A floating window opened by the keybind is
not in herdr, so the ExecProcess path is the primary one and has to feel good.

## Layout

A sidebar of areas on the left, the selected area on the right, plus a command
palette (`:` / `ctrl+p`) that searches every area *and* every action ("pair
bluetooth", "edit kitty", "snapshot now"). Tabs don't scale past four or five
areas.

## v1 areas

**Overview** (home): an at-a-glance view. Hostname, kernel, uptime;
Hashiru version and whether it's behind; pending pacman/AUR updates; doctor
summary; battery, disk, memory; network and bluetooth status. Each line is a
link to its area. (In place of a "getting started" checklist, which suits new
users more than this audience.)

**Updates**
- Pending: pacman (`checkupdates`, which doesn't need root and doesn't touch the
  sync db), AUR (`yay -Qua`), Hashiru (`status --porcelain` behind count).
- **Arch news**: latest items from archlinux.org's news feed, flagged when one is
  newer than the last update. The thing that bites people who update blindly.
- Actions: `hashiru update` (stage 10 runs `pacman -Syu`, so it covers
  pacman and Hashiru), and `yay -Sua` separately: no stage upgrades AUR
  packages, stage 20 only installs missing ones.
- Doctor lives here as "health", with its sections and problems (already built).

**Wallpaper** (native)
- Thumbnail grid of `~/.config/hypr/wallpapers` plus a user directory, drawn
  with kitty graphics. Enter applies it with `awww img`.
- Mode: **time-of-day cycle** (today's `tahoe-wallpaper daemon`) or **fixed**.
  Needs a Hashiru change: the daemon has to read a mode/choice from
  `~/.config/hashiru/`, or it puts the Tahoe set back within 5 minutes.

**Keybinds** (native)
- Searchable list from `hyprctl binds`. The plain-text output, not `-j`: the
  JSON is malformed for Lua-registered binds on Hyprland 0.56 (see
  `hypr/scripts/keybinds`). Grouped by modifier, filtered as you type.
- Later: jump to the bind's line in `keybinds.lua` or the override file.

**Connectivity** (delegated, with summaries)
- Network: current connection, IP, wifi strength (`nmcli`); open `nmtui`.
- Bluetooth: powered, connected devices (`bluetoothctl`); open `bluetoothctl`,
  or `blueman-manager` as GUI. Worth adding `bluetui` to Hashiru's packages.
- Audio: default sink/source and volume (`wpctl`); open `pavucontrol`.

**Snapshots** (native, read-mostly)
- List from `snapper -c root list` (readable without root here): number, date,
  type, description. Diff a snapshot against now (`snapper status N..0`).
- Action: create a snapshot with a description, before trying something risky.
- No rollback in the TUI. Booting a snapshot is grub-btrfs's job, and doing it
  from a TUI on a live system is the wrong place for it.

**Logs** (mixed)
- Sources: Hashiru install log (parser already built), `journalctl -p err -b`,
  failed units (`systemctl --failed`), `dmesg` (may need root).
- Native list for errors and failed units; `journalctl -f` / `-u <unit>` open
  through the runner.

**Config**: a list of the things Hashiru lets you change, each opening
`$EDITOR`: `hashiru.conf`, `hypr/local.lua`, kitty and waybar overrides,
`~/.zshrc.local`. It makes the three override tiers visible without reading the
docs. Grouped by tier.

## Later

Monitors (`hyprctl monitors`, written to `~/.config/hashiru/`), power (tlp,
profiles, battery health, hypridle timeouts), packages (search, orphans, cache),
firmware (`fwupdmgr`), services, theme/fonts, notifications (mako history,
do-not-disturb), clipboard, default apps, fingerprint (wraps
`hashiru fingerprint`), USB/Thunderbolt, SSH keys, docs search.

## What changes in the current code

- **Keep:** `internal/hashiru` (status, doctor, log parser), `internal/herdr`,
  the runner, the Dashboard and Doctor views (they become Overview and
  Updates → health).
- **Change:** tabs become a sidebar; the runner grows the three kinds above.
- **Drop:** the Stages tab idea. It's maintainer work. (v2 brings back re-applying one stage; see below.)

## Build order

1. Shell: sidebar, area interface, runner with three kinds, palette.
2. Overview + Updates (move what's built, add pacman/AUR counts and Arch news).
3. Keybinds: small, native, immediately useful.
4. Connectivity: summaries plus launchers.
5. Wallpaper, with the Hashiru daemon change.
6. Snapshots, Logs, Config.
7. Hashiru side: the keybind, installing the binary, `hashiru tui`.

## Shipping

Hashiru installs the binary itself (decided 2026-10-15); no AUR package. Whether
a stage builds it from a pinned tag or downloads a release build is settled at
step 7.

## v2: power-user round

Decided 2026-10-16. Same audience and stance; v2 makes the tool faster to
drive and reach into, then adds the two areas a power user misses most.
Order: keys and palette, plumbing, Packages, Services. Hardware (monitors,
power, firmware) is its own later round, since it needs Hashiru changes.

### Keys you can see

- **`?` opens a keymap popup** over the current screen: the focused area's
  keys, grouped, then the global ones. `?`, `esc` or `q` closes it. While a
  text input has focus (`/` filter, palette), `?` types a `?`.
- Areas describe their keys as data (key, what it does, group) instead of a
  help string. The footer is built from the same list, so the two can't
  disagree; it shows what fits and ends with `? keys`.

### Palette

- Stays the way to reach everything. Gains the global actions (refresh, show
  keys, quit) and puts recently run actions first.
- Every action gets a stable ID (`updates.update`, `snapshots.new`), shown
  dimmed in the palette and used by deep links.

### Plumbing

- **Deep links.** `hashiru-tui <area>` opens on that area;
  `hashiru-tui --run <action-id>` opens and runs it;
  `hashiru-tui --list-actions` prints every ID. So a Hyprland bind or a
  script can go straight to "update" or "new snapshot".
- **Changelog before updating.** Updates shows the release notes
  (`hashiru changelog`) for the versions this machine is behind, or the
  current one's when it's up to date.
- **Re-apply a stage.** A stage picker (`hashiru install --list`) runs
  `hashiru install N` Interactive. This reverses v1's "drop Stages": that was
  a maintainer's view of stages; re-applying config (45) or snapper (50) in
  one key is a user's need.
- **Overview refreshes itself**: the cheap local stats (uptime, memory,
  battery, disk) every few seconds. Network probes still only on `r`.

### Packages (native)

The view nothing else gives: what Hashiru owns versus what you added.
- **Groups:** Hashiru-managed (in `pacman/*.txt` under the `repo=` from
  `status --porcelain`), yours (explicitly installed, not in a manifest),
  missing (in a manifest, not installed: drift), orphans (`pacman -Qdtq`),
  foreign (`-Qqm`). Package cache size from `/var/cache/pacman/pkg`.
- `/` filters; `enter` shows `pacman -Qi`.
- **Search** the repos and the AUR (`pacman -Ss`, `yay -Ssa`).
- **Actions, all through the launcher so the real prompts show:** install
  (`yay -S`), remove (`sudo pacman -Rns`), remove orphans, clean the cache
  (`paccache -r`).

### Services (native list, delegated actions)

- System and `--user` units and timers: state, enabled, description; next
  run for timers. `/` filters; one key toggles system/user, another shows
  only failed.
- Actions: start, stop, restart, enable, disable. System units go through the
  launcher as `sudo systemctl ...` so the password prompt is visible; user
  units run directly with a note. `journalctl -u` opens as Watch.
- Logs' failed units jump here.

## v3: hardware round

Decided 2026-10-16. The round v2 put off: monitors, power, firmware. Same
audience and stance. All three go in **one Hardware area** with `[` `]` tabs
(Monitors, Power, Firmware), so the sidebar grows by one row, not three.

### Capability map

| Module id | Where | Responsibility | Depends on |
|---|---|---|---|
| `firmware` | here | fwupd devices, pending updates, refresh metadata, update | — |
| `monitors` | here | live outputs; change mode, scale, rotation, placement; apply by writing the override `monitors.lua` | — |
| `idle-override` | Hashiru | `~/.config/hashiru/hypr/hypridle-{ac,battery}.conf` replace the stowed ones; a reload command | — |
| `charge-thresholds` | Hashiru | `HASHIRU_CHARGE_START/STOP` in `hashiru.conf`, `hashiru charge`, applied to `/etc/tlp.d` | — |
| `power` | here | battery health, TLP profile and switch, idle timeouts, charge thresholds | `idle-override`, `charge-thresholds` |

Build order: `firmware` (creates the Hardware area) → `monitors` →
`idle-override`, `charge-thresholds` → `power`. The two Hashiru modules'
contracts are specced in Hashiru (`/opt/hashiru/SPEC-idle-override.md`,
`SPEC-charge-thresholds.md`); this file only says how the TUI uses them.
Each Hashiru module ships in a Hashiru release before `power` relies on it,
and `power` degrades (read-only, with a note) on a Hashiru that predates it.

### Hardware area

- Area ID `hardware`; `hashiru-tui hardware` opens it. Tabs as in Packages and
  Logs: `[` `]`, the tab bar on top, the last tab remembered for the session.
- **Deep links reach a tab** (decided 2026-10-16), in every area with tabs,
  not just this one: `hashiru-tui hardware.power`, `hashiru-tui packages.orphans`,
  `hashiru-tui logs.journal`. The tab part is the tab's short, stable name
  (`yours`, `journal`, `failed`; the title's first word, lowercased), any
  case accepted; labels in the tab bar can change, names don't. An unknown tab fails like an unknown area, listing the
  area's tabs. An area without tabs given a tab part is an error too.
  `--run` keeps action IDs; a positional `area.tab` is never an action.
- Action IDs are `hardware.<verb>` (below). Everything that needs root goes
  through the launcher as Interactive, so sudo and the tool's own prompts show.
- `r` re-probes the current tab only.

### Monitors (native)

The one place a TUI beats editing Lua by hand: seeing the outputs and their
real modes while changing them.

- **List** from `hyprctl monitors all -j`: name, description, mode, position,
  scale, rotation, enabled, focused. `enter` shows the rest, including every
  available mode.
- **Change, staged:** `m` mode (picker: `preferred` plus `availableModes`),
  `s` scale (picker of common scales; after applying, show the scale Hyprland
  actually used if it differs), `t` rotate (0/90/180/270), `p` place (left of,
  right of, above, below another output; position computed from its logical
  size). Staged changes show as a diff on the row (`2560x1440@60 → preferred`).
  `esc` drops them.
- **Apply (`a`, `hardware.monitors-apply`)** writes
  `~/.config/hashiru/hypr/monitors.lua` from the staged layout and runs
  `hyprctl reload`. Then a **keep-or-revert prompt with a 15 s countdown**:
  `y` keeps, anything else or the timeout puts the previous file back (or
  removes it, if there wasn't one) and reloads again. That guards the
  black-screen case `monitors.lua` warns about (a dock offering fewer modes).
- **Why write-and-reload, not `hyprctl eval 'hl.monitor(...)'` per change:**
  `clamshell.lua` documents that `hl.monitor` accumulates rules and desyncs
  from DRM when called repeatedly. One file, one reload, is the state Hyprland
  will also boot into, so "live" and "saved" can't drift.
- **The generated file:** a header saying hashiru-tui wrote it, one
  `hl.monitor` per output, then Hashiru's catch-all (`output = ""`) last so a
  display never seen before still lights. External displays are matched by
  `desc:` (follows the display between ports, as `monitors.lua` recommends);
  the internal panel (`eDP-*`) by name. Disabled outputs are written with
  `disabled = true` except `eDP-*`, which is left to `clamshell.lua`.
- **A hand-written override is never silently replaced.** If the file exists
  without the hashiru-tui header, apply asks first and copies it to
  `monitors.lua.bak` before writing.
- Refuses to stage a layout with no enabled output.

### Power (native, delegated writes)

- **Battery**, per `/sys/class/power_supply/BAT*`: charge, status, health
  (`energy_full / energy_full_design`, or the `charge_*` pair), cycle count,
  current thresholds; AC online. No battery: the section says so and the rest
  of the tab still works.
- **Profile:** `platform_profile` and its choices, TLP's profile and power
  source from `tlp-stat -s`. `p` (`hardware.power-profile`) picks one and runs
  `sudo tlp <profile>`, the same command as waybar's cycle button.
- **Idle:** lock and screen-off timeouts for AC and battery, read from the
  override when there is one, else the stowed file, labelled which. `i`
  (`hardware.idle`) edits them in minutes; saving writes the override (a copy
  of the stowed file with only the `listener` timeouts changed) and runs
  Hashiru's reload. `I` (`hardware.idle-reset`) removes the override and
  reloads. Screen-off stays "lock + 30 s" unless set separately.
- **Charge thresholds:** `c` (`hardware.charge`) asks start and stop
  (0–100, start < stop) and runs `hashiru charge START STOP` Interactive;
  `C` (`hardware.charge-off`) runs `hashiru charge off`. Hidden when the
  battery has no `charge_control_*` files; read-only with a note when
  `hashiru` has no `charge` subcommand.

### Firmware (delegated)

- **Devices** from `fwupdmgr get-devices --json`: name, vendor, version,
  updatable. **Pending** from `fwupdmgr get-updates --json`, shown first and
  flagged; `enter` shows a device's or release's details (version from → to,
  summary, whether it needs a reboot).
- `F` (`hardware.firmware-refresh`) runs `fwupdmgr refresh` Interactive;
  `u` (`hardware.firmware-update`) runs `fwupdmgr update` Interactive, which
  keeps fwupd's own confirm and reboot prompts. `u` is the same key as
  Updates' update.
- fwupd not installed or the daemon not answering: the tab says so with the
  command that failed; no other tab is affected.

### Engineering (same as v1 and v2)

- Probes go through `system.Exec`, in `internal/system/{monitors,power,firmware}.go`;
  parsers are pure and tested against output captured from this machine
  (`internal/system/testdata/`). File reads (sysfs, hypridle confs) take a
  root path so tests use a temp dir.
- The Lua writer and the hypridle rewriter are pure functions with golden
  tests: same layout in, same file out; a stowed hypridle file with changed
  timeouts in, only the `timeout =` lines different out.
- `make test` and `make race` pass; `make fmt` clean; every new key is in the
  area's `Keys()` so the footer and `?` show it.
- **Ask first:** any change to `clamshell.lua` or the catch-all; adding a Go
  dependency. **Never:** write under `/etc` or into the stowed tree from the
  TUI; run root commands outside the launcher.

### Done when

- `hashiru-tui hardware` opens on the area with three tabs; `[` `]` cycle them.
  `hashiru-tui hardware.power` and `hashiru-tui packages.orphans` open on that
  tab; `hashiru-tui hardware.nope` exits non-zero listing the tabs.
- Firmware lists this machine's devices and an empty pending list without
  error; `u` and `F` open fwupdmgr in a pane (herdr) or take over the terminal.
- Monitors: changing eDP-1's scale and applying writes the override, reloads,
  and reverts on its own after 15 s without `y`; with `y` the file stays and
  survives a Hyprland restart. A hand-written override gets a `.bak`.
- Power shows 87–88 % health and 57 cycles on this T14s (matching sysfs),
  switches profile through sudo, edits idle timeouts so hypridle restarts on the override
  (`ps -o args= -C hypridle` names the override file), and sets 75/80
  thresholds that read back from sysfs and survive `hashiru update`.
- Every new action is in the palette and `--list-actions`.
