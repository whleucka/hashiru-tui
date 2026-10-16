package tui

import (
	"os"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/whleucka/hashiru-tui/internal/hashiru"
)

func pacmanFixtures(t *testing.T) probe {
	t.Helper()
	p := probe{}
	for _, q := range []string{"Qq", "Qqe", "Qqm", "Qdtq"} {
		b, err := os.ReadFile("../system/testdata/pacman-" + q + ".txt")
		if err != nil {
			t.Fatal(err)
		}
		p["pacman -"+q] = string(b)
	}
	return p
}

func packagesModel(t *testing.T, repo string, p probe) *Model {
	t.Helper()
	m := New(Options{Source: fakeSource{status: hashiru.Status{Repo: repo}}, Probe: p.run})
	load(t, m)
	goTo(t, m, "Packages")
	return m
}

func TestPackagesGroups(t *testing.T) {
	m := packagesModel(t, "../system/testdata/repo", pacmanFixtures(t))
	out := ansi.Strip(m.View().Content)
	for _, want := range []string{"Yours (4)", "Missing (0)", "Orphans (1)", "Foreign (7)", "Hashiru (191)", "httpie", "shellcheck", "installed by you"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q:\n%s", want, out)
		}
	}

	press(m, "[", "[") // wraps round past Search to Hashiru's
	out = ansi.Strip(m.View().Content)
	if !strings.Contains(out, "aur.txt · AUR") || !strings.Contains(out, "Hashiru's pacman/*.txt") {
		t.Errorf("Hashiru group:\n%s", out)
	}
	press(m, "]", "]", "]", "]") // Search, Yours, Missing, Orphans
	if out := ansi.Strip(m.View().Content); !strings.Contains(out, "wrk-debug") {
		t.Errorf("orphans:\n%s", out)
	}
}

func TestPackagesFilter(t *testing.T) {
	m := packagesModel(t, "../system/testdata/repo", pacmanFixtures(t))
	press(m, "[", "[") // Hashiru's, the long list
	press(m, "/")
	typeText(m, "chrom")
	out := ansi.Strip(m.View().Content)
	if !strings.Contains(out, "chromium") || strings.Contains(out, "yay-bin ") {
		t.Errorf("filter chrom:\n%s", out)
	}
	if !m.typing() {
		t.Error("filter should report typing, so ? goes to it")
	}
	press(m, "esc")
	if m.area().(*packages).input.Value() != "" || !m.inArea {
		t.Error("esc should clear the filter and stay in the area")
	}
}

func TestPackagesInfo(t *testing.T) {
	p := pacmanFixtures(t)
	p["pacman -Qi httpie"] = "Name            : httpie\nVersion         : 3.2.4-3\nDescription     : human-friendly CLI HTTP client\n"
	m := packagesModel(t, "../system/testdata/repo", p)
	press(m, "enter") // httpie is first of yours
	out := ansi.Strip(m.View().Content)
	if !strings.Contains(out, "pacman -Qi httpie") || !strings.Contains(out, "human-friendly") {
		t.Errorf("info:\n%s", out)
	}
	press(m, "esc")
	if m.area().(*packages).infoOf != nil || !m.inArea {
		t.Error("esc should go back to the list")
	}
}

func TestInfoArgvForMissing(t *testing.T) {
	if got := strings.Join(infoArgv(pkgRow{name: "spotify", aur: true}), " "); got != "yay -Si spotify" {
		t.Errorf("missing AUR: %s", got)
	}
	if got := strings.Join(infoArgv(pkgRow{name: "gimp"}), " "); got != "pacman -Si gimp" {
		t.Errorf("missing repo package: %s", got)
	}
}

func TestPackagesWithoutPacman(t *testing.T) {
	m := packagesModel(t, "../system/testdata/repo", probe{})
	if out := ansi.Strip(m.View().Content); !strings.Contains(out, "pacman") || !strings.Contains(out, "not found") {
		t.Errorf("no pacman:\n%s", out)
	}
}

func TestPackagesWithoutARepo(t *testing.T) {
	m := packagesModel(t, "", pacmanFixtures(t))
	out := ansi.Strip(m.View().Content)
	if !strings.Contains(out, "no Hashiru manifests") || !strings.Contains(out, "Yours (188)") {
		t.Errorf("no repo:\n%s", out)
	}
}
