package tui

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/whleucka/hashiru-tui/internal/hashiru"
)

const testFeed = `<?xml version="1.0"?><rss><channel>
<item><title>Mkinitcpio &gt;=42 requires manual intervention</title><link>https://archlinux.org/news/mkinitcpio-42/</link><pubDate>Tue, 13 Oct 2026 09:00:00 +0000</pubDate></item>
<item><title>Old news</title><link>https://archlinux.org/news/old/</link><pubDate>Tue, 01 Sep 2026 09:00:00 +0000</pubDate></item>
</channel></rss>`

func feed(context.Context, string) ([]byte, error) { return []byte(testFeed), nil }

func TestNewsFlaggedSinceLastUpdate(t *testing.T) {
	var opened []string
	m := New(Options{
		Source: fakeSource{status: hashiru.Status{Updated: time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)}},
		Get:    feed,
		Detach: func(argv []string) error { opened = append(opened, strings.Join(argv, " ")); return nil },
	})
	load(t, m)

	if out := m.View().Content; !strings.Contains(out, "1 since your last update") {
		t.Errorf("Overview should flag unread news:\n%s", out)
	}
	goTo(t, m, "Updates")
	out := m.View().Content
	for _, want := range []string{"Arch news", "Mkinitcpio >=42", "Old news"} {
		if !strings.Contains(out, want) {
			t.Errorf("Updates missing %q:\n%s", want, out)
		}
	}

	press(m, "n")
	if len(opened) != 1 || opened[0] != "xdg-open https://archlinux.org/news/mkinitcpio-42/" {
		t.Errorf("n opened %v; want the newest unread item", opened)
	}
}

func TestNewsOffline(t *testing.T) {
	m := New(Options{
		Source: fakeSource{},
		Get:    func(context.Context, string) ([]byte, error) { return nil, errors.New("no network") },
	})
	load(t, m)
	goTo(t, m, "Updates")
	if out := m.View().Content; !strings.Contains(out, "no network") {
		t.Errorf("fetch error not shown:\n%s", out)
	}
}
