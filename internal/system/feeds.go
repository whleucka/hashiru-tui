package system

import (
	"bytes"
	"context"
	"io"
	"regexp"
	"strings"
)

// The News area's other feeds. Arch's own is NewsURL.
const (
	KernelURL = "https://www.kernel.org/feeds/kdist.xml"
	LWNURL    = "https://lwn.net/headlines/rss"
)

// FetchKernel gets kernel.org's release feed.
func FetchKernel(ctx context.Context, get Get) ([]NewsItem, error) {
	body, err := get(ctx, KernelURL)
	if err != nil {
		return nil, err
	}
	return ParseKernel(bytes.NewReader(body))
}

var changelogHref = regexp.MustCompile(`href="([^"]*/ChangeLog-[^"]*)"`)

// ParseKernel reads kernel.org's releases in the feed's order (mainline,
// stable, then the longterm lines, newest first), which says more than
// sorting by date would. Every item links to kernel.org's front page, so
// a release's ChangeLog is the better link when it has one.
func ParseKernel(r io.Reader) ([]NewsItem, error) {
	raw, err := readRSS(r, "kernel.org")
	if err != nil {
		return nil, err
	}
	items := make([]NewsItem, 0, len(raw))
	for _, it := range raw {
		link := strings.TrimSpace(it.Link)
		if m := changelogHref.FindStringSubmatch(it.Description); m != nil {
			link = m[1]
		}
		items = append(items, NewsItem{Title: strings.TrimSpace(it.Title), Link: link, Date: pubDate(it.PubDate)})
	}
	return items, nil
}

// KernelSeries is a version's major.minor ("7.2" of "7.2.14-arch1-1" and of
// "7.2.15: stable"), for finding the release line this machine runs.
func KernelSeries(v string) string {
	v = strings.TrimSpace(v)
	end := strings.IndexFunc(v, func(r rune) bool { return r != '.' && (r < '0' || r > '9') })
	if end >= 0 {
		v = v[:end]
	}
	parts := strings.SplitN(v, ".", 3)
	if len(parts) < 2 || parts[0] == "" || parts[1] == "" {
		return ""
	}
	return parts[0] + "." + parts[1]
}
