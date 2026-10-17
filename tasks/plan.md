# Implementation Plan: hashiru-tui v4, devices (SPEC.md "v4")

v3's plan is in [v3/plan.md](v3/plan.md); v2's and v1's beside it.

## Overview

A Devices tab in Hardware (drives, USB, Thunderbolt) and the USB-C
charger's wattage in Power. All in hashiru-tui; the only Hashiru piece it
uses, `usb --reset`, already ships.

## Architecture decisions

- **Probes as in v3:** pure parsers in `internal/system/devices.go` tested on
  output captured here; sysfs reads take a root path. `boltctl` has no JSON,
  so its tree output is parsed loosely (`key: value` under each header).
- **One list, three sections.** Rows carry their kind (drive, partition,
  usb, thunderbolt); keys act on the selected row's kind and do nothing
  (footer note) on the others. The footer shows only the keys that apply to
  the selected row.
- **Who asks for what:** udisks and bolt go through polkit, so mount,
  unmount and power-off run in the background (result in the footer, like
  user units in Services); unlock, bolt's authorize/enroll/forget and
  `usb --reset` are Interactive so their prompts show.
- **Eject is a sequence:** unmount each mounted partition, then power off
  the disk; stop at the first failure and say which step.

## Task list

### Phase 1: Probes
- [x] Task 1: Drives: lsblk JSON → removable disks and partitions
- [x] Task 2: USB from sysfs; Thunderbolt from boltctl
- [x] Task 3: USB-C charger in Power

### Checkpoint A
- [x] Tests, vet, build; fixtures from this machine (a USB stick plugged in for Task 1, the charger in for Task 3)

### Phase 2: Devices tab
- [x] Task 4: Devices tab: three sections, selection, details
- [x] Task 5: Actions: mount, unmount, eject, unlock, open; reset; authorize, store, forget

### Checkpoint B
- [ ] Tests, vet, build; SPEC v4 "Done when" by hand (stick, encrypted stick if at hand, dock, charger); README and site

## Risks and mitigations

| Risk | Impact | Mitigation |
|---|---|---|
| `boltctl list` wording changes | Low | Loose `key: value` parse; unknown keys ignored; fixture test |
| polkit refuses udisks in a non-graphical session (ssh, TTY) | Med | The error is udisksctl's own, shown in the footer; no sudo fallback |
| Ejecting a disk with a busy mount | Med | Unmount fails first; say "busy" and leave the disk powered |
| ucsi reports 0 for current_max on some chargers | Low | No wattage then, just "plugged in (USB-C)" |
| `e` meaning eject here vs enable in Services | Low | Decided in the spec: Devices has no enable; the footer names it |

## Open questions

- None yet.
