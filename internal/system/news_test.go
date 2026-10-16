package system

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestParseNews(t *testing.T) {
	items, err := ParseNews(bytes.NewReader(fixture(t, "news.xml")))
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 10 {
		t.Fatalf("got %d items, want 10", len(items))
	}
	first := items[0]
	if !strings.HasPrefix(first.Title, "Mkinitcpio >=42 requires manual intervention") {
		t.Errorf("title = %q (entities should be decoded)", first.Title)
	}
	if first.Date.Year() != 2026 || first.Date.Month() != time.September || first.Date.Day() != 22 {
		t.Errorf("date = %v", first.Date)
	}
	if !strings.HasPrefix(first.Link, "https://archlinux.org/news/") {
		t.Errorf("link = %q", first.Link)
	}
	for i := 1; i < len(items); i++ {
		if items[i].Date.After(items[i-1].Date) {
			t.Errorf("not newest first at %d", i)
		}
	}
}

func TestParseNewsGarbage(t *testing.T) {
	if _, err := ParseNews(strings.NewReader("<html>captive portal</html>")); err == nil {
		t.Error("want an error for a page that isn't the feed")
	}
}

func TestNewsSince(t *testing.T) {
	items := []NewsItem{
		{Title: "new", Date: time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)},
		{Title: "old", Date: time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)},
	}
	if n := Unread(items, time.Date(2026, 9, 15, 0, 0, 0, 0, time.UTC)); n != 1 {
		t.Errorf("Unread = %d, want 1", n)
	}
	if n := Unread(items, time.Time{}); n != 0 {
		t.Errorf("never updated: Unread = %d, want 0 (nothing to compare against)", n)
	}
}

func TestFetchNews(t *testing.T) {
	get := func(_ context.Context, url string) ([]byte, error) {
		if url != NewsURL {
			t.Errorf("url = %q", url)
		}
		return fixture(t, "news.xml"), nil
	}
	items, err := FetchNews(context.Background(), get)
	if err != nil || len(items) != 10 {
		t.Errorf("items %d, err %v", len(items), err)
	}

	offline := func(context.Context, string) ([]byte, error) { return nil, errors.New("no network") }
	if _, err := FetchNews(context.Background(), offline); err == nil {
		t.Error("want the fetch error")
	}
}
