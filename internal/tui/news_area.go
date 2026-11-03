package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/whleucka/hashiru-tui/internal/system"
)

// newsTab is one of News's sources.
type newsTab int

const (
	newsArch     newsTab = iota // archlinux.org's announcements (shared state)
	newsSecurity                // the security tracker, against what's installed
	newsKernel                  // kernel.org's releases
	newsLWN                     // LWN's headlines
	newsTabCount
)

const feedTimeout = 15 * time.Second

// newsArea reads the feeds that matter to an Arch machine: one tab each,
// a cursor through the items, enter to open one in the browser.
type newsArea struct {
	e   *env
	tab newsTab
	sel [newsTabCount]int
	top [newsTabCount]int

	kernel    []system.NewsItem
	kernelErr error
	lwn       []system.NewsItem
	lwnErr    error
	exposed   []system.Exposure
	secErr    error
	busy      [newsTabCount]bool

	height int
}

type (
	feedMsg struct {
		tab   newsTab
		items []system.NewsItem
		err   error
	}
	exposureMsg struct {
		exposed []system.Exposure
		err     error
	}
	// newsFocusMsg opens News on a tab (Updates' n).
	newsFocusMsg struct{ tab newsTab }
)

// newsItem is one row, whatever the source.
type newsItem struct {
	line   string   // the row, without the cursor
	link   string   // what enter opens
	detail []string // shown under the list for the selected row
}

func newNews(e *env) *newsArea { return &newsArea{e: e} }

func (n *newsArea) Name() string { return "News" }

func (n *newsArea) Tabs() []string { return []string{"arch", "security", "kernel", "lwn"} }

func (n *newsArea) Tab() int { return int(n.tab) }

func (n *newsArea) SetTab(i int) { n.tab = newsTab(i) }

func (n *newsArea) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case areaSizeMsg:
		n.height = msg.height
	case refreshMsg:
		n.busy[newsSecurity], n.busy[newsKernel], n.busy[newsLWN] = true, true, true
		get, probe := n.e.get, n.e.probe
		return tea.Batch(
			func() tea.Msg {
				ctx, cancel := context.WithTimeout(context.Background(), feedTimeout)
				defer cancel()
				ex, err := system.FetchExposure(ctx, get, probe)
				return exposureMsg{ex, err}
			},
			fetchFeed(newsKernel, func(ctx context.Context) ([]system.NewsItem, error) {
				return system.FetchKernel(ctx, get)
			}),
			fetchFeed(newsLWN, func(ctx context.Context) ([]system.NewsItem, error) {
				return system.FetchFeed(ctx, get, system.LWNURL, "lwn")
			}),
		)
	case feedMsg:
		n.busy[msg.tab] = false
		switch msg.tab {
		case newsKernel:
			n.kernelErr = msg.err
			if msg.err == nil {
				n.kernel = msg.items
			}
		case newsLWN:
			n.lwnErr = msg.err
			if msg.err == nil {
				n.lwn = msg.items
			}
		}
		n.clamp(msg.tab)
	case exposureMsg:
		n.busy[newsSecurity], n.secErr = false, msg.err
		if msg.err == nil {
			n.exposed = msg.exposed
		}
		n.clamp(newsSecurity)
	case newsMsg:
		n.clamp(newsArch)
	case newsFocusMsg:
		n.tab = msg.tab
	}
	return nil
}

func fetchFeed(tab newsTab, fetch func(context.Context) ([]system.NewsItem, error)) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), feedTimeout)
		defer cancel()
		items, err := fetch(ctx)
		return feedMsg{tab, items, err}
	}
}

// clamp keeps a tab's cursor on a row after its list changed.
func (n *newsArea) clamp(t newsTab) {
	n.sel[t] = min(n.sel[t], max(len(n.items(t))-1, 0))
}

func (n *newsArea) Actions() []Action {
	return []Action{
		{ID: "news.open", Title: "Open the selected news item", Key: "o", Run: n.open},
		{ID: "news.copy", Title: "Copy the selected news item's link", Key: "y", Run: n.copyLink},
	}
}

// selected is the row under the cursor, if there is one.
func (n *newsArea) selected() (newsItem, bool) {
	items := n.items(n.tab)
	if i := n.sel[n.tab]; i < len(items) {
		return items[i], true
	}
	return newsItem{}, false
}

func (n *newsArea) open() tea.Cmd {
	it, ok := n.selected()
	if !ok {
		return nil
	}
	return n.e.launch(Launch{Argv: []string{"xdg-open", it.link}, Label: "the link", Kind: GUI})
}

// copyLink puts the link on the clipboard through the terminal (OSC 52),
// so it works over ssh and needs no wl-copy.
func (n *newsArea) copyLink() tea.Cmd {
	it, ok := n.selected()
	if !ok {
		return nil
	}
	return tea.Batch(tea.SetClipboard(it.link), func() tea.Msg { return noteMsg{text: "copied " + it.link} })
}

func (n *newsArea) HandleKey(msg tea.KeyPressMsg) (tea.Cmd, bool) {
	k := msg.String()
	if t, ok := nextTab(k, int(n.tab), int(newsTabCount)); ok {
		n.tab = newsTab(t)
		return nil, true
	}
	if i, ok := moveKey(k, n.sel[n.tab], len(n.items(n.tab)), n.listRows()); ok {
		n.sel[n.tab] = i
		return nil, true
	}
	switch k {
	case "enter", "o":
		return n.open(), true
	case "y":
		return n.copyLink(), true
	}
	return nil, false
}

func (n *newsArea) Keys() []Key {
	return []Key{
		tabKey,
		{Keys: "j/k", Desc: "move"},
		{Keys: "enter/o", Desc: "open in browser"},
		{Keys: "y", Desc: "copy link"},
	}
}

// detailRows is the room under the list for the selected item's link and
// details.
const detailRows = 4

// listRows is how many items fit: the height less the tab bar and the
// detail block.
func (n *newsArea) listRows() int { return max(n.height-3-detailRows, 1) }

func (n *newsArea) View(width, height int) string {
	n.height = height
	var b strings.Builder
	b.WriteString(n.tabBar() + "\n\n")

	items := n.items(n.tab)
	if len(items) == 0 {
		b.WriteString(n.empty())
		return b.String()
	}
	rows := n.listRows()
	sel := n.sel[n.tab]
	top := n.top[n.tab]
	if sel < top {
		top = sel
	}
	if sel >= top+rows {
		top = sel - rows + 1
	}
	n.top[n.tab] = top
	for i := top; i < len(items) && i < top+rows; i++ {
		cur := "  "
		if i == sel {
			cur = styleTitle.Render("▸ ")
		}
		b.WriteString(cut(cur+items[i].line, width) + "\n")
	}

	it := items[sel]
	b.WriteString("\n")
	for _, d := range it.detail {
		b.WriteString(cut("  "+d, width) + "\n")
	}
	b.WriteString(cut("  "+styleMuted.Render(it.link), width) + "\n")
	return b.String()
}

// cut keeps a line to the area's width.
func cut(s string, width int) string {
	if width <= 0 {
		return s
	}
	return ansi.Truncate(s, width, "…")
}

func (n *newsArea) tabBar() string {
	st := n.e.state
	arch := "Arch"
	if u := st.UnreadNews(); u > 0 {
		arch = fmt.Sprintf("Arch (%d new)", u)
	}
	sec := "Security"
	if n.exposed != nil {
		sec = fmt.Sprintf("Security (%d)", len(n.exposed))
	}
	return tabBar([]string{arch, sec, "Kernel", "LWN"}, int(n.tab), func(i int) bool {
		switch newsTab(i) {
		case newsArch:
			return st.UnreadNews() > 0
		case newsSecurity:
			return n.severe() > 0
		}
		return false
	})
}

// severe counts exposures rated High or Critical.
func (n *newsArea) severe() int {
	c := 0
	for _, e := range n.exposed {
		if e.Severity == "High" || e.Severity == "Critical" {
			c++
		}
	}
	return c
}

// empty says why a tab has nothing to show.
func (n *newsArea) empty() string {
	var err error
	busy := n.busy[n.tab]
	switch n.tab {
	case newsArch:
		err, busy = n.e.state.NewsErr, n.e.state.NewsBusy
	case newsSecurity:
		err = n.secErr
	case newsKernel:
		err = n.kernelErr
	case newsLWN:
		err = n.lwnErr
	}
	switch {
	case err != nil:
		return styleFail.Render(err.Error())
	case busy:
		return styleMuted.Render("fetching…")
	case n.tab == newsSecurity:
		return styleOK.Render("no open issues against installed packages")
	}
	return styleMuted.Render("nothing yet (r reloads)")
}

// items are a tab's rows.
func (n *newsArea) items(t newsTab) []newsItem {
	switch t {
	case newsArch:
		return n.archItems()
	case newsSecurity:
		return n.securityItems()
	case newsKernel:
		return n.kernelItems()
	case newsLWN:
		return datedItems(n.lwn, 0)
	}
	return nil
}

// datedItems are feed items as "date  title", the first unread marked.
func datedItems(feed []system.NewsItem, unread int) []newsItem {
	items := make([]newsItem, len(feed))
	for i, it := range feed {
		mark := "  "
		if i < unread {
			mark = styleWarn.Render("● ")
		}
		items[i] = newsItem{
			line: mark + styleMuted.Render(it.Date.Format("2006-01-02")) + "  " + it.Title,
			link: it.Link,
		}
	}
	return items
}

func (n *newsArea) archItems() []newsItem {
	st := n.e.state
	items := datedItems(st.News, st.UnreadNews())
	for i := range items {
		if i < st.UnreadNews() {
			items[i].detail = []string{styleWarn.Render("published since your last update: read it before updating")}
		}
	}
	return items
}

func (n *newsArea) kernelItems() []newsItem {
	var mine string
	if si := n.e.state.Sys; si != nil {
		mine = system.KernelSeries(si.Kernel)
	}
	items := make([]newsItem, len(n.kernel))
	for i, it := range n.kernel {
		version, kind, _ := strings.Cut(it.Title, ": ")
		line := cell(version, 16) + styleMuted.Render(cell(kind, 10)) + "  " + styleMuted.Render(it.Date.Format("2006-01-02"))
		var detail []string
		if mine != "" && system.KernelSeries(version) == mine {
			line += "  " + styleOK.Render("your series")
			detail = []string{"you run " + n.e.state.Sys.Kernel}
		}
		items[i] = newsItem{line: line, link: it.Link, detail: detail}
	}
	return items
}

func (n *newsArea) securityItems() []newsItem {
	items := make([]newsItem, len(n.exposed))
	for i, e := range n.exposed {
		var pkgs []string
		for _, name := range e.Names() {
			pkgs = append(pkgs, name+" "+styleMuted.Render(e.Packages[name]))
		}
		fix := styleMuted.Render("no fix yet")
		if e.Fixed != "" {
			fix = styleOK.Render("fixed in " + e.Fixed + ": update")
		}
		items[i] = newsItem{
			line: severityStyle(e.Severity).Render(cell(e.Severity, 9)) + cell(e.Group, 10) + strings.Join(pkgs, ", ") +
				"  " + styleMuted.Render(e.Type),
			link: e.Link(),
			detail: []string{
				strings.Join(e.Issues, " "),
				fix,
			},
		}
	}
	return items
}

func severityStyle(s string) lipgloss.Style {
	switch s {
	case "Critical", "High":
		return styleFail
	case "Medium":
		return styleWarn
	}
	return styleMuted
}
