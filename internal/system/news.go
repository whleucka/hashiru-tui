package system

import (
	"bytes"
	"context"
	"encoding/xml"
	"errors"
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

// FetchNews gets and parses the feed.
func FetchNews(ctx context.Context, get Get) ([]NewsItem, error) {
	body, err := get(ctx, NewsURL)
	if err != nil {
		return nil, err
	}
	return ParseNews(bytes.NewReader(body))
}

// ParseNews reads the RSS feed, newest first.
func ParseNews(r io.Reader) ([]NewsItem, error) {
	var feed struct {
		Items []struct {
			Title   string `xml:"title"`
			Link    string `xml:"link"`
			PubDate string `xml:"pubDate"`
		} `xml:"channel>item"`
	}
	if err := xml.NewDecoder(r).Decode(&feed); err != nil {
		return nil, fmt.Errorf("arch news: %w", err)
	}
	if len(feed.Items) == 0 {
		return nil, errors.New("arch news: no items (not the feed?)")
	}
	items := make([]NewsItem, 0, len(feed.Items))
	for _, it := range feed.Items {
		date, _ := time.Parse(time.RFC1123Z, strings.TrimSpace(it.PubDate))
		items = append(items, NewsItem{
			Title: strings.TrimSpace(it.Title),
			Link:  strings.TrimSpace(it.Link),
			Date:  date,
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
