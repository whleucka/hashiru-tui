package tui

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/whleucka/hashiru-tui/internal/system"
)

// feeds serves each feed's fixture by URL; a URL missing from the map
// fails like a dead network.
func feeds(t *testing.T, files map[string]string) system.Get {
	t.Helper()
	return func(_ context.Context, url string) ([]byte, error) {
		if url == system.NewsURL {
			return []byte(testFeed), nil
		}
		name, ok := files[url]
		if !ok {
			return nil, errors.New("no network")
		}
		return os.ReadFile(filepath.Join("..", "system", "testdata", name))
	}
}

// pacmanProbe answers pacman -Q and vercmp (by string order, enough here).
func pacmanProbe(_ context.Context, name string, args ...string) ([]byte, error) {
	switch name {
	case "pacman":
		return []byte("pam 1.7.3-1\nvim 9.2.1178-1\nlibtiff 4.7.2-2\n"), nil
	case "vercmp":
		switch {
		case args[0] < args[1]:
			return []byte("-1"), nil
		case args[0] > args[1]:
			return []byte("1"), nil
		}
		return []byte("0"), nil
	}
	return nil, errors.New("unexpected " + name)
}

func newsModel(t *testing.T, files map[string]string) *Model {
	t.Helper()
	m := New(Options{Source: fakeSource{}, Get: feeds(t, files), Probe: pacmanProbe})
	load(t, m)
	goTo(t, m, "News")
	return m
}

func TestNewsSecurityTab(t *testing.T) {
	m := newsModel(t, map[string]string{system.VulnerableURL: "vulnerable.json"})
	if err := m.Start("news.security", ""); err != nil {
		t.Fatal(err)
	}
	out := ansi.Strip(m.View().Content)
	// pam (High) first; vim's group is fixed in an older version than the
	// one installed, so it's not listed.
	for _, want := range []string{"Security (2)", "▸ High     AVG-2901  pam 1.7.3-1", "AVG-2842  libtiff", "no fix yet", "https://security.archlinux.org/AVG-2901"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "vim") {
		t.Errorf("vim already has the fix:\n%s", out)
	}
}

func TestNewsKernelMarksYourSeries(t *testing.T) {
	m := New(Options{
		Source:  fakeSource{},
		Get:     feeds(t, map[string]string{system.KernelURL: "kdist.xml"}),
		Probe:   pacmanProbe,
		SysRoot: filepath.Join("..", "system", "testdata", "sysroot"),
	})
	load(t, m)
	goTo(t, m, "News")
	press(m, "]", "]") // Kernel
	press(m, "j", "j") // 7.2.15: stable
	out := ansi.Strip(m.View().Content)
	kernel, _ := os.ReadFile(filepath.Join("..", "system", "testdata", "sysroot", "proc", "sys", "kernel", "osrelease"))
	series := system.KernelSeries(string(kernel))
	for _, line := range strings.Split(out, "\n") {
		if strings.Contains(line, "your series") && !strings.Contains(line, series+".") {
			t.Errorf("marked a release outside %s: %q", series, line)
		}
	}
	if !strings.Contains(out, "your series") {
		t.Errorf("nothing marked for %s:\n%s", series, out)
	}
}

func TestNewsCopyLink(t *testing.T) {
	m := newsModel(t, map[string]string{system.LWNURL: "lwn.xml"})
	press(m, "[") // wraps to LWN
	_, cmd := m.Update(key("y"))
	if cmd == nil {
		t.Fatal("y did nothing")
	}
	drain(m, cmd)
	if !strings.Contains(m.note, "copied https://lwn.net/Articles/") {
		t.Errorf("note = %q", m.note)
	}
}

func TestNewsFeedErrorStaysInItsTab(t *testing.T) {
	m := newsModel(t, nil) // only Arch's feed answers
	press(m, "]", "]", "]")
	if out := m.View().Content; !strings.Contains(out, "no network") {
		t.Errorf("LWN's error not shown:\n%s", out)
	}
	press(m, "]") // back to Arch, which loaded
	if out := m.View().Content; !strings.Contains(out, "Mkinitcpio") {
		t.Errorf("Arch tab broken by another feed's error:\n%s", out)
	}
}
