package system

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// fakeVercmp orders the versions this test uses.
func fakeVercmp(_ context.Context, a, b string) (int, error) {
	return strings.Compare(a, b), nil
}

func TestExposed(t *testing.T) {
	groups, err := ParseVulnerable(fixture(t, "vulnerable.json"))
	if err != nil {
		t.Fatal(err)
	}
	installed := ParseInstalled([]byte("vim 9.2.1178-1\nlibtiff 4.7.2-2\npam 1.7.3-1\nlibxml2 2.15.4-1\nopenssl 3.6.5-1\nxdg-utils 1.2.1-2\n"))
	got, err := Exposed(context.Background(), groups, installed, fakeVercmp)
	if err != nil {
		t.Fatal(err)
	}
	var order []string
	for _, e := range got {
		order = append(order, e.Group)
	}
	// vim's group is "fixed" in 9.0.1225, older than what's installed: gone.
	// The not-installed Critical one: gone. High, then Medium, Low, Unknown;
	// newest group first within a severity.
	want := "AVG-2901 AVG-2898 AVG-2765 AVG-1420 AVG-2882 AVG-2842"
	if strings.Join(order, " ") != want {
		t.Errorf("order = %v\nwant    %s", order, want)
	}
	pam := got[0]
	if pam.Packages["pam"] != "1.7.3-1" || pam.Severity != "High" || pam.Type != "arbitrary filesystem access" || len(pam.Issues) == 0 {
		t.Errorf("pam = %+v", pam)
	}
	if pam.Link() != "https://security.archlinux.org/AVG-2901" {
		t.Errorf("link = %q", pam.Link())
	}
}

func TestExposedFixAvailable(t *testing.T) {
	fixed := "2.0-1"
	groups := []TrackerGroup{{Name: "AVG-1", Packages: []string{"foo"}, Severity: "High", Fixed: &fixed}}
	got, err := Exposed(context.Background(), groups, map[string]string{"foo": "1.0-1"}, fakeVercmp)
	if err != nil || len(got) != 1 || got[0].Fixed != "2.0-1" {
		t.Errorf("an older install than the fix is exposed, with the fix named: %+v %v", got, err)
	}
}

func TestExposedVercmpError(t *testing.T) {
	fixed := "2.0-1"
	groups := []TrackerGroup{{Name: "AVG-1", Packages: []string{"foo"}, Fixed: &fixed}}
	broken := func(context.Context, string, string) (int, error) { return 0, errors.New("no vercmp") }
	if _, err := Exposed(context.Background(), groups, map[string]string{"foo": "1.0-1"}, broken); err == nil {
		t.Error("want vercmp's error")
	}
}

func TestPacmanVercmp(t *testing.T) {
	run := func(_ context.Context, name string, args ...string) ([]byte, error) {
		if name != "vercmp" || len(args) != 2 {
			t.Fatalf("ran %s %v", name, args)
		}
		return []byte("-1\n"), nil
	}
	if c, err := PacmanVercmp(run)(context.Background(), "1:1.0", "2.0"); err != nil || c != -1 {
		t.Errorf("= %d, %v", c, err)
	}
}
