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
| Terminal, to watch | `journalctl -f`, btop | new pane, unfocused | `tea.ExecProcess` |
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
- Action: update everything. `hashiru update` already runs `pacman -Syu` in
  stage 10, so one action covers all three.
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
- **Drop:** the Stages tab idea. It's maintainer work.

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
