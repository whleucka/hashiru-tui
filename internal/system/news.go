package system

import (
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"
)

// NewsURL is Arch's news feed: the announcements that say "read this before
// you update", which bite people who update blindly.
const NewsURL = "https://archlinux.org/feeds/news/"

// NewsItem is one announcement.
type NewsItem struct {
	Title string
	Link  string
	Date  time.Time
}

// Get fetches a URL's body.
type Get func(ctx context.Context, url string) ([]byte, error)

// HTTPGet is the real Get.
func HTTPGet(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s: %s", url, resp.Status)
	}
	return io.ReadAll(io.LimitReader(resp.Body, 1<<20))
}

// FetchNews gets and parses Arch's feed.
func FetchNews(ctx context.Context, get Get) ([]NewsItem, error) {
	return FetchFeed(ctx, get, NewsURL, "arch news")
}

// FetchFeed gets and parses an RSS 2.0 feed; name labels its errors.
func FetchFeed(ctx context.Context, get Get, url, name string) ([]NewsItem, error) {
	body, err := get(ctx, url)
	if err != nil {
		return nil, err
	}
	return ParseFeed(bytes.NewReader(body), name)
}

// ParseNews reads Arch's RSS feed, newest first.
func ParseNews(r io.Reader) ([]NewsItem, error) { return ParseFeed(r, "arch news") }

// rssItem is the part of an RSS 2.0 item every feed here uses.
type rssItem struct {
	Title       string `xml:"title"`
	Link        string `xml:"link"`
	PubDate     string `xml:"pubDate"`
	Description string `xml:"description"`
}

func readRSS(r io.Reader, name string) ([]rssItem, error) {
	var feed struct {
		Items []rssItem `xml:"channel>item"`
	}
	if err := xml.NewDecoder(r).Decode(&feed); err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}
	if len(feed.Items) == 0 {
		return nil, fmt.Errorf("%s: no items (not the feed?)", name)
	}
	return feed.Items, nil
}

// pubDate reads RSS's date. Feeds differ on "+0000" or "-0000" and the
// weekday's spelling, so a couple of layouts.
func pubDate(s string) time.Time {
	s = strings.TrimSpace(s)
	for _, layout := range []string{time.RFC1123Z, time.RFC1123, "Mon, 2 Jan 2006 15:04:05 -0700"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t
		}
	}
	return time.Time{}
}

// ParseFeed reads an RSS 2.0 feed, newest first.
func ParseFeed(r io.Reader, name string) ([]NewsItem, error) {
	raw, err := readRSS(r, name)
	if err != nil {
		return nil, err
	}
	items := make([]NewsItem, 0, len(raw))
	for _, it := range raw {
		items = append(items, NewsItem{
			Title: strings.TrimSpace(it.Title),
			Link:  strings.TrimSpace(it.Link),
			Date:  pubDate(it.PubDate),
		})
	}
	sort.SliceStable(items, func(i, j int) bool { return items[i].Date.After(items[j].Date) })
	return items, nil
}

// Unread counts items published after since, the machine's last update. With
// no last update there is nothing to compare against, so none are unread.
func Unread(items []NewsItem, since time.Time) int {
	if since.IsZero() {
		return 0
	}
	n := 0
	for _, it := range items {
		if it.Date.After(since) {
			n++
		}
	}
	return n
}
