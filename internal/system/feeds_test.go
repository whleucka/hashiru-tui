package system

import (
	"bytes"
	"strings"
	"testing"
)

func TestParseKernel(t *testing.T) {
	items, err := ParseKernel(bytes.NewReader(fixture(t, "kdist.xml")))
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 10 {
		t.Fatalf("got %d releases, want 10", len(items))
	}
	if items[0].Title != "7.4-rc1: mainline" {
		t.Errorf("first = %q; want the feed's order, mainline first", items[0].Title)
	}
	stable := items[1]
	if stable.Title != "7.3.1: stable" || !strings.HasSuffix(stable.Link, "/ChangeLog-7.3.1") {
		t.Errorf("stable = %+v; want its ChangeLog as the link", stable)
	}
	if items[0].Link != "https://www.kernel.org/" {
		t.Errorf("mainline has no ChangeLog, so kernel.org: %q", items[0].Link)
	}
	if items[0].Date.IsZero() {
		t.Error("date not parsed (-0000 zone)")
	}
}

func TestParseLWN(t *testing.T) {
	items, err := ParseFeed(bytes.NewReader(fixture(t, "lwn.xml")), "lwn")
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 4 {
		t.Fatalf("got %d items, want 4", len(items))
	}
	for _, it := range items {
		if it.Title == "" || !strings.HasPrefix(it.Link, "https://lwn.net/") || it.Date.IsZero() {
			t.Errorf("incomplete item %+v", it)
		}
	}
}

func TestParseFeedNamesItsErrors(t *testing.T) {
	_, err := ParseFeed(strings.NewReader("<html>captive portal</html>"), "lwn")
	if err == nil || !strings.HasPrefix(err.Error(), "lwn:") {
		t.Errorf("err = %v; want one starting with the feed's name", err)
	}
}

func TestKernelSeries(t *testing.T) {
	for in, want := range map[string]string{
		"7.2.14-arch1-1":      "7.2",
		"7.2.15: stable":      "7.2",
		"7.4-rc1: mainline":   "7.4",
		"6.12.115: longterm":  "6.12",
		"next-20261102: next": "",
		"":                    "",
	} {
		if got := KernelSeries(in); got != want {
			t.Errorf("KernelSeries(%q) = %q, want %q", in, got, want)
		}
	}
}
