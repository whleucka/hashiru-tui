package tui

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

var svcArgs = []string{
	"list-units --type=service,timer --all --output=json --no-pager",
	"list-unit-files --type=service,timer --output=json --no-pager",
	"list-timers --all --output=json --no-pager",
}

const brokenUnits = `[{"unit":"broken.service","load":"loaded","active":"failed","sub":"failed","description":"Broken thing"}]`

// svcProbe answers systemctl from this machine's fixtures; the user
// manager gets a failed unit too.
func svcProbe(t *testing.T) probe {
	t.Helper()
	p := probe{}
	for i, f := range []string{"units", "unit-files", "timers"} {
		sys, err1 := os.ReadFile("../system/testdata/systemctl-" + f + ".json")
		user, err2 := os.ReadFile("../system/testdata/systemctl-user-" + f + ".json")
		if err1 != nil || err2 != nil {
			t.Fatal(err1, err2)
		}
		p["systemctl "+svcArgs[i]] = string(sys)
		p["systemctl --user "+svcArgs[i]] = string(user)
	}
	p["systemctl --user "+svcArgs[0]] = brokenUnits
	p["systemctl --user "+svcArgs[1]] = `[]`
	return p
}

type svcRec struct {
	launched []string
	quiet    []string
}

func servicesModel(t *testing.T, p probe) (*Model, *svcRec) {
	t.Helper()
	rec := &svcRec{}
	m := New(Options{
		Source: fakeSource{},
		Probe: func(ctx context.Context, name string, args ...string) ([]byte, error) {
			line := strings.Join(append([]string{name}, args...), " ")
			if strings.Contains(line, " start ") || strings.Contains(line, " restart ") || strings.Contains(line, " stop ") {
				rec.quiet = append(rec.quiet, line)
				return nil, nil
			}
			return p.run(ctx, name, args...)
		},
		RunInPane: func(_ context.Context, argv []string, _ string, _, _ bool) error {
			rec.launched = append(rec.launched, strings.Join(argv, " "))
			return nil
		},
	})
	load(t, m)
	goTo(t, m, "Services")
	return m, rec
}

func TestServicesTabs(t *testing.T) {
	m, _ := servicesModel(t, svcProbe(t))
	out := ansi.Strip(m.View().Content)
	for _, want := range []string{"System (", "User (1)", "Failed (1)", "bluetooth.service", "active running", "Bluetooth service"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q:\n%s", want, out)
		}
	}
	press(m, "/")
	typeText(m, "snapper-timeline")
	if out := ansi.Strip(m.View().Content); !strings.Contains(out, "snapper-timeline.timer") || !strings.Contains(out, "next ") {
		t.Errorf("timer row:\n%s", out)
	}
	press(m, "esc", "[") // wraps to Failed
	if out := ansi.Strip(m.View().Content); !strings.Contains(out, "broken.service") || !strings.Contains(out, "failed failed") {
		t.Errorf("failed tab:\n%s", out)
	}
}

func TestSystemUnitChangesGoThroughSudo(t *testing.T) {
	m, rec := servicesModel(t, svcProbe(t))
	press(m, "/")
	typeText(m, "bluetooth.service")
	press(m, "enter", "R", "e", "d")
	want := []string{
		"sudo systemctl restart bluetooth.service",
		"sudo systemctl enable bluetooth.service",
		"sudo systemctl disable bluetooth.service",
	}
	if strings.Join(rec.launched, "|") != strings.Join(want, "|") || rec.quiet != nil {
		t.Errorf("launched %v, quiet %v", rec.launched, rec.quiet)
	}
}

func TestUserUnitChangesRunQuietly(t *testing.T) {
	m, rec := servicesModel(t, svcProbe(t))
	press(m, "]", "s") // User tab: broken.service
	if len(rec.quiet) != 1 || rec.quiet[0] != "systemctl --user start broken.service" || rec.launched != nil {
		t.Errorf("quiet %v, launched %v", rec.quiet, rec.launched)
	}
	if !strings.Contains(m.note, "start broken.service: done") {
		t.Errorf("note %q", m.note)
	}
}

func TestServiceStatusAndLog(t *testing.T) {
	p := svcProbe(t)
	m, rec := servicesModel(t, p)
	// status exits 3 for a unit that isn't running, and still prints it.
	m.env.probe = func(ctx context.Context, name string, args ...string) ([]byte, error) {
		if strings.Join(args, " ") == "--user status --no-pager --full broken.service" {
			return []byte("× broken.service - Broken thing\n     Active: failed\n"), codeErr(3)
		}
		return p.run(ctx, name, args...)
	}
	press(m, "]", "enter")
	out := ansi.Strip(m.View().Content)
	if !strings.Contains(out, "Active: failed") || strings.Contains(out, "exit") {
		t.Errorf("status:\n%s", out)
	}
	press(m, "l")
	if len(rec.launched) != 1 || rec.launched[0] != "journalctl --user -u broken.service -f" {
		t.Errorf("follow: %v", rec.launched)
	}
}

func TestZeroSelectsTheTenthArea(t *testing.T) {
	m := New(Options{Source: fakeSource{}})
	load(t, m)
	press(m, "0")
	if m.area().Name() != "Services" {
		t.Errorf("0 → %s", m.area().Name())
	}
	if out := ansi.Strip(m.View().Content); !strings.Contains(out, "0 Services") {
		t.Errorf("sidebar:\n%s", out)
	}
}

func TestServicesWithoutSystemctl(t *testing.T) {
	m, _ := servicesModel(t, probe{})
	if out := ansi.Strip(m.View().Content); !strings.Contains(out, "systemctl") {
		t.Errorf("no systemctl:\n%s", out)
	}
}

type codeErr int

func (c codeErr) Error() string { return "exit status" }
func (c codeErr) ExitCode() int { return int(c) }
