# hashiru-tui

A terminal UI for a [Hashiru](https://hashiru.williamhleucka.com) machine. It runs after install only; it is not part of the ISO installer.

It holds no Hashiru logic of its own. Its state comes from the CLI's machine-readable output:

- `hashiru status --porcelain`: key=value lines (release stamp, checkout, upstream)
- `hashiru doctor --json`: every check, grouped by section

Anything that changes the machine (`update`, `config`) runs the real `hashiru` command. Inside herdr that happens in a split beside the TUI, as in mantis-tui. Outside herdr, the command takes over the terminal until it exits.

## Keys

| Key | |
|---|---|
| `1` `2` / `tab` | Dashboard, Doctor |
| `r` | refresh |
| `u` | `hashiru update` (pane, focused) |
| `e` | `hashiru config` (pane, focused) |
| `L` | `hashiru log -f` (pane, focus stays here) |
| `j` `k` `enter` | Doctor: move, expand a section |
| `q` | quit |

## Build

```bash
make build   # bin/hashiru-tui
make test
```

`HASHIRU_BIN` points it at a different `hashiru`, e.g. a checkout's `bin/hashiru`.

## Layout

- `internal/hashiru`: runs the CLI and parses its output. No TUI imports.
- `internal/herdr`: `herdr pane split/run/rename`.
- `internal/tui`: the bubbletea model. Takes a `Source` and an optional `RunInPane`, so tests need neither hashiru nor herdr.

## Roadmap

- **Log**: the install log split into runs at `HASHIRU RUN START`, filter by level, follow
- **Stages**: `scripts/NN-*.sh` with last-run result; re-run one (`45`) or from one (`45+`)
- **Changelog**: `docs/releases/*.md` rendered with glamour; shows what's new when behind
- **Update preview**: incoming commits and release notes before confirming `u`
- **Config**: `hashiru.conf` values and which `~/.config/hashiru/` overrides exist
- **Packages**, **Fingerprint**, command palette
- Ship it: `hashiru tui`, installed by a Hashiru stage
