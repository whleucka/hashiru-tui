package tui

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/whleucka/hashiru-tui/internal/hashiru"
)

type launched struct{ argv, label string }

func actionsModel(t *testing.T) (*Model, *[]launched) {
	t.Helper()
	p := pacmanFixtures(t)
	ss, _ := os.ReadFile("../system/testdata/pacman-Ss.txt")
	ssa, _ := os.ReadFile("../system/testdata/yay-Ssa.txt")
	p["pacman -Ss lazy"] = string(ss)
	p["yay -Ssa lazy"] = string(ssa)
	var ran []launched
	m := New(Options{
		Source: fakeSource{status: hashiru.Status{Repo: "../system/testdata/repo"}},
		Probe:  p.run,
		RunInPane: func(_ context.Context, argv []string, label string, _, _ bool) error {
			ran = append(ran, launched{strings.Join(argv, " "), label})
			return nil
		},
	})
	load(t, m)
	goTo(t, m, "Packages")
	return m, &ran
}

func TestSearchAndInstall(t *testing.T) {
	m, ran := actionsModel(t)
	press(m, "S")
	pk := m.area().(*packages)
	if pk.group != groupSearch || !m.typing() {
		t.Fatalf("S: group %v, typing %v", pk.group, m.typing())
	}
	typeText(m, "lazy")
	press(m, "enter")
	out := ansi.Strip(m.View().Content)
	for _, want := range []string{"2 in the repos · 7 in the AUR", "lazygit", "extra 0.66.1-1", "installed", "aur 0.3.0-1", "A terminal-based SSH"} {
		if !strings.Contains(out, want) {
			t.Errorf("results missing %q:\n%s", want, out)
		}
	}

	press(m, "i") // lazygit, from the repos, is installed already
	if len(*ran) != 0 || !strings.Contains(m.note, "already installed") {
		t.Errorf("installed one: ran %v, note %q", *ran, m.note)
	}
	press(m, "j", "j", "i") // lazygit, htop, [lazyssh] from the AUR
	if len(*ran) != 1 || (*ran)[0].argv != "yay -S lazyssh" {
		t.Errorf("ran %v, want yay -S lazyssh", *ran)
	}
}

func TestInstallFromTheRepos(t *testing.T) {
	if got := strings.Join(installArgv(pkgRow{name: "btop"}), " "); got != "sudo pacman -S btop" {
		t.Errorf("repo install: %s", got)
	}
}

func TestRemoveOrphansAndCache(t *testing.T) {
	m, ran := actionsModel(t)
	press(m, "d") // httpie, first of yours
	press(m, "O", "C")
	want := []string{"sudo pacman -Rns httpie", "sudo pacman -Rns wrk-debug", "sudo paccache -r"}
	if len(*ran) != 3 {
		t.Fatalf("ran %v", *ran)
	}
	for i, w := range want {
		if (*ran)[i].argv != w {
			t.Errorf("%d: ran %q, want %q", i, (*ran)[i].argv, w)
		}
	}
}

func TestRemovingHashirusOwnSaysItComesBack(t *testing.T) {
	m, ran := actionsModel(t)
	press(m, "[", "[", "d") // Hashiru's, the first of them
	if len(*ran) != 1 || !strings.Contains((*ran)[0].label, "the next update puts it back") {
		t.Errorf("ran %+v", *ran)
	}
}

func TestQuestionMarkTypesIntoTheSearch(t *testing.T) {
	m, _ := actionsModel(t)
	press(m, "S", "?")
	if m.keyHelp || m.area().(*packages).query.Value() != "?" {
		t.Errorf("popup %v, query %q", m.keyHelp, m.area().(*packages).query.Value())
	}
}

func TestDeepLinkToSearch(t *testing.T) {
	m := New(Options{Source: fakeSource{}})
	if err := m.Start("", "packages.search"); err != nil {
		t.Fatal(err)
	}
	load(t, m)
	if p := m.area().(*packages); m.area().Name() != "Packages" || p.group != groupSearch || !p.querying {
		t.Errorf("opened on %s, group %v, querying %v", m.area().Name(), p.group, p.querying)
	}
}
