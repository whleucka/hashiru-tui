# Tasks (v4)

v3's tasks are in [v3/todo.md](v3/todo.md).

Verification for every task: `go test ./...`, `go vet ./...`, `make build`.

## Phase 1: Probes

### Task 1: Drives
- [x] `system.ParseDrives` from lsblk JSON: disks with `rm` or `hotplug`, their partitions; path, label, size, fstype, mount points, transport, model; LUKS (`crypto_LUKS`) flagged with its unlocked child
- [x] Never the system disk (not rm, not hotplug); zram and loop skipped
- [x] Fixtures: this machine with none; three sticks hand-made in lsblk's shape (plain, unlocked LUKS, no partition table)
- [x] A real stick's lsblk output, after the reboot into 7.2.10 (the running kernel's modules are gone, so usb_storage can't load)
**Files:** internal/system/devices.go (new), devices_test.go, testdata. **Size:** S

### Task 2: USB and Thunderbolt
- [x] `system.USBDevices(root)`: bus-port, vendor:product, manufacturer, product, speed, parent hub; sorted as a tree
- [x] `ParseBolt(list, domains)`: devices (name, vendor, uuid, generation, status, authorized, stored) and the security level; fixtures from this machine (the stored dock)
**Files:** internal/system/devices.go, devices_test.go, testdata (a sysfs tree under sysroot). **Size:** M

### Task 3: USB-C charger in Power
- [x] `system.Charger(root)`: the online `type=USB` supply's watts (voltage_max × current_max) and PD/PPS
- [x] Power's AC line: "charger 92W USB-C PD  drawing 65W", or "plugged in" without one
- [x] Fixture captured with the charger in
**Files:** internal/system/power.go, power_test.go, internal/tui/hardware_power.go, tests. **Size:** S

### Checkpoint A
- [x] Tests, vet, build; fixtures from this machine

## Phase 2: Devices tab

### Task 4: Devices tab
- [x] Tab `devices` between Power and Firmware; Drives, USB, Thunderbolt sections in one list
- [x] Rows: drive/partition with mount point; USB indented under hubs with speed; Thunderbolt with status and stored
- [x] Warns on a connected, unauthorized Thunderbolt device; missing bolt or controller says so
- [x] `enter` on a row shows its details (lsblk fields, sysfs ids, boltctl fields)
**Files:** internal/tui/hardware_devices.go (new), hardware.go, tests. **Size:** M

### Task 5: Actions
- [ ] Drives: `m` mount, `u` unmount, `e` eject (unmount all, power off), LUKS unlock Interactive then mount, `enter` on a mounted partition opens Thunar
- [ ] USB: `R` → `usb --reset <id>` Interactive
- [ ] Thunderbolt: `a` authorize, `s` store (enroll), `f` forget, Interactive
- [ ] Keys shown only for the selected row's kind; actions in the palette with IDs (`hardware.drive-mount`, ...)
**Files:** internal/tui/hardware_devices.go, tests. **Size:** M

### Checkpoint B
- [ ] SPEC v4 "Done when" by hand; README and site's tui.html; release when you say
