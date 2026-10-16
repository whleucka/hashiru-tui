package system

import (
	"strings"
	"testing"
)

func TestParseSnapshots(t *testing.T) {
	snaps, err := ParseSnapshots(fixture(t, "snapper-list.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(snaps) != 61 {
		t.Fatalf("got %d snapshots, want 61 (62 less 'current')", len(snaps))
	}
	if snaps[0].Number != 584 || snaps[0].Type != "single" || snaps[0].Description != "timeline" {
		t.Errorf("newest = %+v", snaps[0])
	}
	if snaps[0].Date.IsZero() || snaps[0].Date.Hour() != 20 {
		t.Errorf("date = %v", snaps[0].Date)
	}
	last := snaps[len(snaps)-1]
	if last.Number != 2 || last.Type != "pre" || !strings.HasPrefix(last.Description, "pacman -S") {
		t.Errorf("oldest = %+v", last)
	}
}

func TestParseSnapshotsBadJSON(t *testing.T) {
	if _, err := ParseSnapshots([]byte("Unknown config.")); err == nil {
		t.Error("want an error")
	}
}

func TestParseChanges(t *testing.T) {
	in := "+..... /etc/new.conf\nc..... /etc/pacman.conf\n-..... /usr/bin/gone\n.....x /var/xattr\n"
	cs := ParseChanges([]byte(in))
	if len(cs) != 4 {
		t.Fatalf("changes = %+v", cs)
	}
	want := []Change{{'+', "/etc/new.conf"}, {'c', "/etc/pacman.conf"}, {'-', "/usr/bin/gone"}, {'.', "/var/xattr"}}
	for i := range want {
		if cs[i] != want[i] {
			t.Errorf("change %d = %+v, want %+v", i, cs[i], want[i])
		}
	}
	created, deleted, modified := Count(cs)
	if created != 1 || deleted != 1 || modified != 2 {
		t.Errorf("counts %d %d %d", created, deleted, modified)
	}

	if real := ParseChanges(fixture(t, "snapper-status.txt")); len(real) == 0 {
		t.Error("the captured status parsed to nothing")
	}
}
