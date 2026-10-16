// Package kitty shows images through the kitty graphics protocol using
// Unicode placeholders: the picture is uploaded once with a virtual placement, then
// drawn by ordinary text cells (U+10EEEE with row/column diacritics, the
// foreground colour carrying the image id). Bubble Tea diffs those like any
// other text, and herdr carries them per pane, so nothing fights the renderer
// the way a direct placement would.
//
// https://sw.kovidgoyal.net/kitty/graphics-protocol/#unicode-placeholders
package kitty

import (
	"fmt"
	"image"
	_ "image/gif" // decoders, registered for Load
	_ "image/jpeg"
	_ "image/png"
	"os"
	"strings"

	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/ansi/kitty"
)

// maxPlaceholder is the length of kitty's row/column diacritics table.
const maxPlaceholder = 297

// Supported reports whether the terminal shows kitty graphics. tmux is ruled
// out: it needs every sequence wrapped for passthrough.
func Supported(getenv func(string) string) bool {
	if getenv("TMUX") != "" {
		return false
	}
	switch getenv("TERM_PROGRAM") {
	case "herdr", "ghostty":
		return true
	}
	return getenv("TERM") == "xterm-kitty" || getenv("KITTY_WINDOW_ID") != ""
}

// Load decodes a JPEG, PNG or GIF. (WebP isn't in the standard library; those
// just get no preview.)
func Load(path string) (image.Image, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	return img, err
}

// Thumbnail scales img down to at most maxW pixels wide, nearest-neighbour:
// a preview a few dozen cells across doesn't need a 2560px upload.
func Thumbnail(img image.Image, maxW int) image.Image {
	b := img.Bounds()
	if b.Dx() <= maxW {
		return img
	}
	w, h := maxW, max(1, b.Dy()*maxW/b.Dx())
	out := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		sy := b.Min.Y + y*b.Dy()/h
		for x := range w {
			out.Set(x, y, img.At(b.Min.X+x*b.Dx()/w, sy))
		}
	}
	return out
}

// Upload transmits img as PNG under id, chunked, without displaying it.
// q=2 keeps the terminal from answering, since replies would arrive as keys.
// Transmission must be set: EncodeGraphics only defaults it after choosing
// what to encode, so leaving it zero sends an empty payload.
func Upload(img image.Image, id int) (string, error) {
	var sb strings.Builder
	err := kitty.EncodeGraphics(&sb, img, &kitty.Options{
		Action: kitty.Transmit, Transmission: kitty.Direct, ID: id, Format: kitty.PNG, Quiet: 2, Chunk: true,
	})
	return sb.String(), err
}

// Place gives image id a virtual placement cols x rows cells, replacing any
// earlier one; the terminal fits the picture in it keeping its aspect.
func Place(id, cols, rows int) string {
	return ansi.KittyGraphics(nil, "a=p", "U=1", fmt.Sprintf("i=%d", id), "p=1",
		fmt.Sprintf("c=%d", cols), fmt.Sprintf("r=%d", rows), "q=2")
}

// Delete frees image id and its placements in the terminal.
func Delete(id int) string {
	return ansi.KittyGraphics(nil, "a=d", "d=I", fmt.Sprintf("i=%d", id), "q=2")
}

// Box is the cell area an iw x ih picture fills inside cols x rows, keeping
// aspect with cells about twice as tall as wide.
func Box(iw, ih, cols, rows int) (int, int) {
	if iw <= 0 || ih <= 0 || cols < 1 || rows < 1 {
		return 0, 0
	}
	cols, rows = min(cols, maxPlaceholder), min(rows, maxPlaceholder)
	scale := min(float64(cols)/float64(iw), float64(rows*2)/float64(ih))
	return max(1, min(cols, int(float64(iw)*scale+0.5))), max(1, min(rows, int(float64(ih)*scale/2+0.5)))
}

// Placeholders draws image id's cols x rows placement, centred in width w.
// The id travels as a 256-colour foreground, so it must be 16-255: those
// survive Bubble Tea's colour handling unchanged.
func Placeholders(id, cols, rows, w int) []string {
	pad := strings.Repeat(" ", max(0, (w-cols)/2))
	fg := fmt.Sprintf("\x1b[38;5;%dm", id)
	lines := make([]string, rows)
	var sb strings.Builder
	for r := range rows {
		sb.Reset()
		sb.WriteString(pad + fg)
		for c := range cols {
			sb.WriteRune(kitty.Placeholder)
			sb.WriteRune(kitty.Diacritic(r))
			sb.WriteRune(kitty.Diacritic(c))
		}
		sb.WriteString("\x1b[0m")
		lines[r] = sb.String()
	}
	return lines
}

// ID maps an upload generation to an id in 16-255, the range that travels
// intact as a 256-colour foreground.
func ID(gen int) int { return 16 + gen%240 }
