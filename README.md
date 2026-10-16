# hashiru-tui

A keyboard-driven control center for a [Hashiru](https://hashiru.williamhleucka.com) machine: update it, check it, change it, and reach the tools that manage it. It runs after install only; it is not part of the ISO installer. See [SPEC.md](SPEC.md) for the design.

It delegates where a good tool exists (nmtui, bluetoothctl, journalctl, pavucontrol) and only draws its own screens where none does.

## Areas

| | |
|---|---|
| **Overview** | Host, kernel, uptime, memory, disk, battery (live, every 5s); connections; Hashiru version, pending updates, Arch news, health |
| **Updates** | Hashiru status, release notes (what an update brings, or this version's), pending pacman and AUR packages, Arch news (flagged when newer than your last update), doctor's health checks |
| **Packages** | What Hashiru's manifests install vs what you added: yours, missing (drift), orphans, foreign, Hashiru's; cache size; `pacman -Qi` for any. Search the repos and the AUR and install; remove, remove orphans, clean the cache |
| **Keybinds** | Hyprland's binds from `hyprctl binds`, filterable |
| **Connectivity** | Network devices and wifi, bluetooth, default audio; launches nmtui, bluetoothctl, blueman, pavucontrol |
| **Wallpaper** | Pick a wallpaper (previewed with kitty graphics) or go back to Hashiru's time-of-day cycle |
| **Snapshots** | snapper's snapshots of `/`, what changed since one, take a new one |
| **Config** | The files Hashiru lets you change, grouped by how they apply, each one key from `$EDITOR` |
| **Logs** | Hashiru's install log by run, this boot's journal errors, failed units |

Hashiru's own state comes from its machine-readable output, `hashiru status --porcelain` and `hashiru doctor --json`, and its install log.

## Keys

| Key | |
|---|---|
| `j` `k` / number keys | pick an area |
| `enter` / `esc` | into the area / back to the sidebar |
| `ctrl+p` (or `:` in the sidebar) | command palette: every area and action |
| `r` | refresh |
| `j` `k` / `ctrl+d` `ctrl+u` | move in a list: a row / half a page |
| `[` `]` | previous / next tab, in an area that has them (Packages, Logs) |
| `?` | this area's keys |
| `q` | quit (asks first: `y` quits, any other key stays); `ctrl+c` quits at once |

Each area lists its own keys in the footer. Some to know: `u` update, `a` AUR upgrade, `c` release notes, `s` re-apply one stage (`hashiru install N`) and `n` Arch news (Updates), `S` search, `i` install, `d` remove, `O` remove orphans, `C` clean the cache (Packages), `/` filter (Keybinds, Packages), `w` nmtui and `b` bluetoothctl (Connectivity), `n` new snapshot (Snapshots), `J` `K` `L` follow the journal, kernel or Hashiru log in a pane (Logs).

## Deep links

```bash
hashiru-tui updates               # open on an area (any case)
hashiru-tui --run snapshots.new   # open and run an action
hashiru-tui --list-actions        # every action's ID and title
```

IDs are `<area>.<verb>` and don't change when titles do, so a Hyprland bind or a script can go straight to one. The palette shows each action's ID beside it.

## How tools are launched

- **GUI apps** (blueman, pavucontrol, the browser for news) start detached.
- **Inside herdr**, terminal tools open in a focused pane beside the TUI, whether you type into them (`hashiru update`, nmtui, `$EDITOR`) or watch them (`journalctl -f`).
- **Otherwise** they take over the terminal until they exit, and the TUI refreshes after anything that may have changed the machine.

## Build

```bash
make build   # bin/hashiru-tui
make test
```

`HASHIRU_BIN` points it at a different `hashiru`, e.g. a checkout's `bin/hashiru`.

## Layout

- `internal/hashiru`: runs the hashiru CLI and parses its output.
- `internal/system`: everything else it reads: checkupdates, nmcli, hyprctl, snapper, journalctl, /proc. Commands go through one injected `Run`; parsers are pure and tested against output captured from a real machine.
- `internal/kitty`: kitty graphics through Unicode placeholders.
- `internal/herdr`: `herdr pane split/run/rename`.
- `internal/tui`: the bubbletea app. One `Area` per sidebar entry; tests inject every command, URL and path, so they touch nothing on the machine running them.
