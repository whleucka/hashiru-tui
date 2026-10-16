package system

import (
	"context"
	"slices"
	"strings"
	"testing"
)

func find(t *testing.T, ss []Service, name string) Service {
	t.Helper()
	i := slices.IndexFunc(ss, func(s Service) bool { return s.Name == name })
	if i < 0 {
		t.Fatalf("no %s", name)
	}
	return ss[i]
}

func TestMergeServicesOnThisMachine(t *testing.T) {
	ss, err := MergeServices(fixture(t, "systemctl-units.json"), fixture(t, "systemctl-unit-files.json"), fixture(t, "systemctl-timers.json"), false)
	if err != nil {
		t.Fatal(err)
	}
	bt := find(t, ss, "bluetooth.service")
	if !bt.Running() || bt.Sub != "running" || bt.Description != "Bluetooth service" || bt.State == "" {
		t.Errorf("bluetooth: %+v", bt)
	}
	timer := find(t, ss, "snapper-timeline.timer")
	if !timer.Timer() || timer.Next.IsZero() || timer.Last.IsZero() || timer.Activates != "snapper-timeline.service" {
		t.Errorf("timer: %+v", timer)
	}
	// Disabled and never loaded, but there to enable.
	if a := find(t, ss, "avahi-daemon.service"); a.Active != "inactive" || a.State != "disabled" {
		t.Errorf("unloaded unit file: %+v", a)
	}
	for _, s := range ss {
		if s.Load == "not-found" || strings.Contains(s.Name, "@.") {
			t.Errorf("listed %s (%s)", s.Name, s.Load)
		}
	}
	if !slices.IsSortedFunc(ss, func(a, b Service) int { return strings.Compare(a.Name, b.Name) }) {
		t.Error("not sorted by name")
	}
}

func TestMergeServicesFailedAndEnabled(t *testing.T) {
	units := `[{"unit":"broken.service","load":"loaded","active":"failed","sub":"failed","description":"Broken"}]`
	files := `[{"unit_file":"broken.service","state":"enabled","preset":null}]`
	ss, err := MergeServices([]byte(units), []byte(files), []byte(`[]`), true)
	if err != nil {
		t.Fatal(err)
	}
	if b := ss[0]; !b.Failed() || !b.Enabled() || !b.User || b.Running() {
		t.Errorf("%+v", b)
	}
}

func TestMergeServicesBadJSON(t *testing.T) {
	if _, err := MergeServices([]byte("UNIT LOAD"), []byte("[]"), []byte("[]"), false); err == nil || !strings.Contains(err.Error(), "list-units") {
		t.Errorf("err = %v", err)
	}
}

func TestReadServicesUser(t *testing.T) {
	run := cmdRun{}
	for i, f := range []string{"systemctl-user-units.json", "systemctl-user-unit-files.json", "systemctl-user-timers.json"} {
		run["systemctl --user "+strings.Join(servicesArgs[i], " ")] = struct {
			out  string
			code int
		}{out: string(fixture(t, f))}
	}
	ss, err := ReadServices(context.Background(), run.Run, true)
	if err != nil {
		t.Fatal(err)
	}
	if d := find(t, ss, "dbus-broker.service"); !d.User || !d.Running() {
		t.Errorf("user dbus: %+v", d)
	}
	if _, err := ReadServices(context.Background(), cmdRun{}.Run, false); err == nil {
		t.Error("no systemctl: no error")
	}
}
