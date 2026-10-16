package tui

import (
	"context"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/whleucka/hashiru-tui/internal/kitty"
	"github.com/whleucka/hashiru-tui/internal/system"
)

// graphicsMode says whether to draw image previews with kitty graphics.
type graphicsMode int

const (
	graphicsAuto graphicsMode = iota // from the environment
	graphicsOn
	graphicsOff
)

// wallpaper picks the desktop wallpaper. Applying one fixes it, which
// Hashiru's tahoe-wallpaper daemon honours instead of cycling; c goes back to
// the time-of-day cycle.
type wallpaper struct {
	e        *env
	home     string
	graphics bool

	images  []string
	current []string // shown on some output now
	choice  string   // fixed wallpaper, "" when cycling
	sel     int

	width, height int

	// Preview: the upload generation, the id on screen, its pixel size and
	// placement.
	gen, shown   int
	iw, ih       int
	cols, rows   int
	previewOf    string
	covered      bool // an overlay is open; leave the preview out
	previewError string
}

type (
	wallMsg struct {
		images, current []string
		choice          string
	}
	wallUploadMsg struct {
		gen, id int
		path    string
		seq     string
		w, h    int
		err     error
	}
)

const (
	wallListWidth = 34
	thumbWidth    = 640 // pixels; plenty for a preview a few dozen cells wide
)

func newWallpaper(e *env, home string, graphics bool) *wallpaper {
	return &wallpaper{e: e, home: home, graphics: graphics}
}

func (w *wallpaper) dirs() []string {
	return []string{
		filepath.Join(w.home, ".config", "hypr", "wallpapers"), // Hashiru's set
		filepath.Join(w.home, "Pictures", "Wallpapers"),
	}
}

func (w *wallpaper) Name() string { return "Wallpaper" }

func (w *wallpaper) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case areaSizeMsg:
		w.width, w.height = msg.width, msg.height
		return w.place()
	case refreshMsg:
		return w.load()
	case wallMsg:
		w.images, w.current, w.choice = msg.images, msg.current, msg.choice
		w.sel = min(w.sel, max(len(w.images)-1, 0))
		return w.preview()
	case wallUploadMsg:
		return w.uploaded(msg)
	case overlayMsg:
		w.covered = msg.open
	}
	return nil
}

func (w *wallpaper) load() tea.Cmd {
	probe, home, dirs := w.e.probe, w.home, w.dirs()
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		out, _ := probe(ctx, "awww", "query")
		current := system.ParseAwwwQuery(out)
		for i, p := range current {
			current[i] = resolve(p)
		}
		return wallMsg{
			images:  system.ListImages(dirs),
			current: current,
			choice:  system.ReadChoice(home),
		}
	}
}

func (w *wallpaper) Actions() []Action {
	return []Action{
		{ID: "wallpaper.cycle", Title: "Wallpaper: back to the time-of-day cycle", Key: "c", Run: w.cycle},
	}
}

func (w *wallpaper) HandleKey(msg tea.KeyPressMsg) (tea.Cmd, bool) {
	if i, ok := moveKey(msg.String(), w.sel, len(w.images), w.height-3); ok {
		w.sel = i
		return w.preview(), true
	}
	switch msg.String() {
	case "enter", "l", "right":
		return w.apply(), true
	case "c":
		return w.cycle(), true
	default:
		return nil, false
	}
}

func (w *wallpaper) Keys() []Key {
	return []Key{
		{Keys: "j/k", Desc: "choose"},
		{Keys: "enter", Desc: "set"},
		{Keys: "c", Desc: "time-of-day cycle"},
	}
}

// apply fixes the wallpaper to the selection and shows it now.
func (w *wallpaper) apply() tea.Cmd {
	if w.sel >= len(w.images) {
		return nil
	}
	path := w.images[w.sel]
	if err := system.WriteChoice(w.home, path); err != nil {
		return func() tea.Msg { return noteMsg{text: "couldn't save the choice", err: err} }
	}
	w.choice = path
	w.current = []string{resolve(path)}
	return w.e.launch(Launch{
		Argv:  []string{"awww", "img", path, "--transition-type", "fade", "--transition-duration", "1"},
		Label: "wallpaper " + filepath.Base(path), Kind: GUI,
	})
}

// cycle clears the choice and has tahoe-wallpaper put the current period's
// picture back.
func (w *wallpaper) cycle() tea.Cmd {
	if err := system.ClearChoice(w.home); err != nil {
		return func() tea.Msg { return noteMsg{text: "couldn't clear the choice", err: err} }
	}
	w.choice = ""
	script := filepath.Join(w.home, ".config", "hypr", "scripts", "tahoe-wallpaper")
	return tea.Batch(
		w.e.launch(Launch{Argv: []string{script}, Label: "time-of-day wallpaper", Kind: GUI}),
		w.load(),
	)
}

// --- Preview -------------------------------------------------------------------

// previewBox is the cell area the preview may use.
func (w *wallpaper) previewBox() (int, int) {
	return max(w.width-wallListWidth-2, 0), max(w.height-3, 0)
}

// preview starts uploading the selection, if graphics are on and it isn't
// already the one shown.
func (w *wallpaper) preview() tea.Cmd {
	if !w.graphics || w.sel >= len(w.images) || w.images[w.sel] == w.previewOf {
		return nil
	}
	path := w.images[w.sel]
	w.previewOf, w.previewError = path, ""
	w.gen++
	gen, id := w.gen, kitty.ID(w.gen)
	return func() tea.Msg { // decoding and PNG-encoding is too slow for Update
		img, err := kitty.Load(path)
		if err != nil {
			return wallUploadMsg{gen: gen, path: path, err: err}
		}
		img = kitty.Thumbnail(img, thumbWidth)
		seq, err := kitty.Upload(img, id)
		b := img.Bounds()
		return wallUploadMsg{gen: gen, id: id, path: path, seq: seq, w: b.Dx(), h: b.Dy(), err: err}
	}
}

// uploaded writes the upload, its placement and the old picture's delete as
// one sequence, so the terminal sees them in order.
func (w *wallpaper) uploaded(msg wallUploadMsg) tea.Cmd {
	if msg.gen != w.gen {
		return nil // superseded: never sent, nothing to free
	}
	if msg.err != nil {
		w.previewError = "no preview: " + msg.err.Error()
		return nil
	}
	seq := msg.seq
	if w.shown != 0 && w.shown != msg.id {
		seq += kitty.Delete(w.shown)
	}
	w.shown, w.iw, w.ih = msg.id, msg.w, msg.h
	pc, pr := w.previewBox()
	w.cols, w.rows = kitty.Box(w.iw, w.ih, pc, pr)
	return tea.Raw(seq + kitty.Place(w.shown, w.cols, w.rows))
}

// place re-places the shown picture after a resize.
func (w *wallpaper) place() tea.Cmd {
	if w.shown == 0 {
		return nil
	}
	pc, pr := w.previewBox()
	c, r := kitty.Box(w.iw, w.ih, pc, pr)
	if c == w.cols && r == w.rows {
		return nil
	}
	w.cols, w.rows = c, r
	return tea.Raw(kitty.Place(w.shown, c, r))
}

// --- View ----------------------------------------------------------------------

func (w *wallpaper) View(width, height int) string {
	mode := styleMuted.Render("time-of-day cycle (tahoe-wallpaper)")
	if w.choice != "" {
		mode = styleWarn.Render("fixed: "+filepath.Base(w.choice)) + styleMuted.Render("  c to cycle again")
	}
	var b strings.Builder
	b.WriteString(styleKey.Render("mode") + mode + "\n\n")
	if len(w.images) == 0 {
		b.WriteString(styleMuted.Render("no images in " + strings.Join(w.dirs(), " or ")))
		return b.String()
	}

	var list []string
	for i, p := range w.images {
		mark := "  "
		if slices.Contains(w.current, resolve(p)) {
			mark = styleOK.Render("● ")
		}
		name := strings.TrimSuffix(filepath.Base(p), filepath.Ext(p))
		line := mark + fmt.Sprintf("%-*s", wallListWidth-6, name)
		if i == w.sel {
			line = styleTitle.Render("▸ ") + styleSelected.Render(line)
		} else {
			line = "  " + line
		}
		list = append(list, line)
	}
	rows := height - 3
	top := max(0, w.sel-rows+1)
	list = list[top:min(len(list), top+rows)]

	var right []string
	switch {
	case w.previewError != "":
		right = []string{styleMuted.Render(w.previewError)}
	case w.shown != 0 && w.cols > 0 && !w.covered:
		pw, _ := w.previewBox()
		right = kitty.Placeholders(w.shown, w.cols, w.rows, pw)
	}

	for i := 0; i < max(len(list), len(right)); i++ {
		// Every row is exactly wallListWidth cells, beside a name or not:
		// kitty assembles the picture from where its placeholder cells sit.
		l := ""
		if i < len(list) {
			l = ansi.Truncate(list[i], wallListWidth, "…")
		}
		l += strings.Repeat(" ", wallListWidth-ansi.StringWidth(l))
		if i < len(right) {
			l += "  " + right[i]
		}
		b.WriteString(l + "\n")
	}
	return b.String()
}

// resolve follows symlinks, so a stowed path and awww's resolved one compare
// equal. A path that can't be resolved is compared as it is.
func resolve(p string) string {
	if r, err := filepath.EvalSymlinks(p); err == nil {
		return r
	}
	return p
}
