package system

import "testing"

func TestParseJournal(t *testing.T) {
	lines := ParseJournal(fixture(t, "journal-err.txt"))
	if len(lines) != 50 {
		t.Fatalf("got %d lines", len(lines))
	}
	first := lines[0]
	if first.Ident != "hyprlock" || first.Time.Hour() != 11 {
		t.Errorf("first = %+v", first)
	}
	if first.Message != "pam_unix(hyprlock:auth): auth could not identify password for [user]" {
		t.Errorf("message = %q", first.Message)
	}

	// "-- No entries --" and continuation lines don't become entries.
	if got := ParseJournal([]byte("-- No entries --\n")); len(got) != 0 {
		t.Errorf("no entries = %+v", got)
	}
	k := ParseJournal([]byte("2026-10-15T08:00:00-06:00 t14s kernel: ACPI BIOS Error\n"))
	if len(k) != 1 || k[0].Ident != "kernel" || k[0].Message != "ACPI BIOS Error" {
		t.Errorf("kernel line = %+v", k)
	}
}

func TestParseFailed(t *testing.T) {
	units := ParseFailed(fixture(t, "failed-system.txt"), false)
	if len(units) != 1 {
		t.Fatalf("units = %+v", units)
	}
	want := Unit{Name: "reflector.service", Description: "Refresh Pacman mirrorlist with Reflector."}
	if units[0] != want {
		t.Errorf("unit = %+v, want %+v", units[0], want)
	}
	user := ParseFailed([]byte("waybar.service loaded failed failed Highly customizable Wayland bar\n"), true)
	if len(user) != 1 || !user[0].User {
		t.Errorf("user units = %+v", user)
	}
	if got := ParseFailed(nil, false); len(got) != 0 {
		t.Errorf("none = %+v", got)
	}
}
