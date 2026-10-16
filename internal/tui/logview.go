package tui

import (
	"fmt"
	"strings"
	"time"

	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/whleucka/hashiru-tui/internal/hashiru"
)

// logView is the Hashiru install log as a list of runs, newest first, and
// the selected run's lines below it.
type logView struct {
	runs []hashiru.Run // oldest first, as parsed
	err  error
	sel  int // index into the newest-first list

	inBody       bool // keys scroll the lines rather than move through runs
	problemsOnly bool // just WARN and ERROR lines
	follow       bool // reread every second and stay at the bottom

	vp            viewport.Model
	width, height int // the whole tab's area
}

func newLogView() logView {
	return logView{vp: viewport.New()}
}

type (
	logMsg struct {
		runs []hashiru.Run
		err  error
	}
	logTickMsg struct{}
)

const (
	logListMax     = 8 // run rows shown at once
	followInterval = time.Second
)

func loadLog(src Source) tea.Cmd {
	return func() tea.Msg {
		runs, err := src.Log()
		return logMsg{runs, err}
	}
}

func followTick() tea.Cmd {
	return tea.Tick(followInterval, func(time.Time) tea.Msg { return logTickMsg{} })
}

// run returns the selected run, or nil.
func (lv *logView) run() *hashiru.Run {
	if lv.sel < 0 || lv.sel >= len(lv.runs) {
		return nil
	}
	return &lv.runs[len(lv.runs)-1-lv.sel]
}

// setRuns swaps in a fresh parse. Following pins the newest run and the
// bottom; otherwise the reading position is kept.
func (lv *logView) setRuns(runs []hashiru.Run, err error) {
	lv.err = err
	if err != nil {
		return
	}
	grew := len(runs) - len(lv.runs)
	lv.runs = runs
	switch {
	case lv.follow:
		lv.sel = 0
	case grew > 0 && lv.sel > 0:
		lv.sel += grew // stay on the same run as new ones arrive on top
	}
	lv.sel = min(lv.sel, max(len(runs)-1, 0))

	offset := lv.vp.YOffset()
	lv.fill()
	if lv.follow {
		lv.vp.GotoBottom()
	} else {
		lv.vp.SetYOffset(offset)
	}
}

func (lv *logView) setSize(w, h int) {
	lv.width, lv.height = w, h
	lv.fill()
}

func (lv *logView) listHeight() int {
	return min(len(lv.runs), logListMax)
}

// fill sizes the viewport and renders the selected run into it.
func (lv *logView) fill() {
	h := lv.height - lv.listHeight() - 2 // a blank line and the run's title
	lv.vp.SetWidth(lv.width)
	lv.vp.SetHeight(max(h, 1))
	lv.vp.SetContent(lv.body())
}

func (lv *logView) selectRun(i int) {
	if i < 0 || i >= len(lv.runs) || i == lv.sel {
		return
	}
	lv.sel = i
	lv.fill()
	lv.vp.GotoTop()
}

// handleKey reports whether it used the key; the rest go back to the shell.
func (lv *logView) handleKey(msg tea.KeyPressMsg) (tea.Cmd, bool) {
	switch msg.String() {
	case "w":
		lv.problemsOnly = !lv.problemsOnly
		lv.fill()
		lv.vp.GotoTop()
		return nil, true
	case "f":
		lv.follow = !lv.follow
		if !lv.follow {
			return nil, true
		}
		lv.selectRun(0)
		lv.vp.GotoBottom()
		return followTick(), true
	}

	if lv.inBody {
		switch msg.String() {
		case "esc", "h", "left":
			lv.inBody = false
		case "g", "home":
			lv.vp.GotoTop()
		case "G", "end":
			lv.vp.GotoBottom()
		case "j", "k", "up", "down", "pgup", "pgdown", "ctrl+d", "ctrl+u", "space":
			var cmd tea.Cmd
			lv.vp, cmd = lv.vp.Update(msg)
			return cmd, true
		default:
			return nil, false
		}
		return nil, true
	}

	if i, ok := moveKey(msg.String(), lv.sel, len(lv.runs), lv.listHeight()); ok {
		lv.selectRun(i)
		return nil, true
	}
	switch msg.String() {
	case "g", "home":
		lv.selectRun(0)
	case "G", "end":
		lv.selectRun(len(lv.runs) - 1)
	case "enter", "l", "right":
		lv.inBody = true
	default:
		return nil, false
	}
	return nil, true
}

func (lv *logView) keys() []Key {
	follow := Key{Keys: "f", Desc: "follow"}
	if lv.follow {
		follow.Desc = "stop following"
	}
	filter := Key{Keys: "w", Desc: "problems only"}
	if lv.problemsOnly {
		filter.Desc = "all lines"
	}
	if lv.inBody {
		return []Key{
			{Keys: "j/k", Desc: "scroll"},
			{Keys: "g/G", Desc: "top/bottom"},
			{Keys: "esc", Desc: "back"},
			filter, follow,
		}
	}
	return []Key{{Keys: "j/k", Desc: "run"}, {Keys: "enter", Desc: "read"}, filter, follow}
}

// --- Rendering -----------------------------------------------------------------

func (lv *logView) view() string {
	switch {
	case lv.err != nil:
		return styleFail.Render(lv.err.Error())
	case lv.runs == nil:
		return styleMuted.Render("reading the install log…")
	case len(lv.runs) == 0:
		return styleMuted.Render("the install log is empty")
	}

	var b strings.Builder
	// Scroll the list so the selection stays in view.
	top := max(0, lv.sel-lv.listHeight()+1)
	for i := top; i < top+lv.listHeight(); i++ {
		b.WriteString(lv.runRow(i) + "\n")
	}

	b.WriteString("\n" + lv.title() + "\n")
	b.WriteString(lv.vp.View())
	return b.String()
}

func (lv *logView) runRow(i int) string {
	r := lv.runs[len(lv.runs)-1-i]
	cursor := "  "
	if i == lv.sel {
		cursor = styleTitle.Render("▸ ")
		if lv.inBody {
			cursor = styleMuted.Render("▸ ")
		}
	}
	row := fmt.Sprintf("%s %s  %s", outcomeMark(r.Outcome), r.Start.Format("2006-01-02 15:04"), describe(r))
	if i == lv.sel {
		row = styleSelected.Render(row)
	}
	return ansi.Truncate(cursor+row+counts(r), lv.width, "…")
}

func (lv *logView) title() string {
	r := lv.run()
	if r == nil {
		return ""
	}
	t := styleTitle.Render(describe(*r)) + "  " + styleMuted.Render(r.Outcome.String())
	if d := duration(*r); d != "" {
		t += styleMuted.Render(" in " + d)
	}
	if lv.follow {
		t += "  " + styleOK.Render("● following")
	}
	if lv.problemsOnly {
		t += "  " + styleWarn.Render("problems only")
	}
	return t
}

func (lv *logView) body() string {
	r := lv.run()
	if r == nil {
		return ""
	}
	var b strings.Builder
	for _, ln := range r.Lines {
		if lv.problemsOnly && ln.Level != "WARN" && ln.Level != "ERROR" {
			continue
		}
		b.WriteString(ansi.Truncate(logLine(ln), lv.width, "…") + "\n")
	}
	if b.Len() == 0 && lv.problemsOnly {
		return styleOK.Render("no warnings or errors in this run")
	}
	return strings.TrimSuffix(b.String(), "\n")
}

var (
	levelStyle = map[string]lipgloss.Style{
		"INFO":  styleMuted,
		"OK":    styleOK,
		"WARN":  styleWarn,
		"ERROR": styleFail,
	}
	styleRaw = lipgloss.NewStyle().Foreground(colorMuted).Faint(true)
)

func logLine(ln hashiru.LogLine) string {
	if ln.Level == "" {
		// Raw output from pacman, git and the like.
		return "                 " + styleRaw.Render(ln.Text)
	}
	level := levelStyle[ln.Level].Render(fmt.Sprintf("%-5s", ln.Level))
	text := ln.Text
	if ln.Level == "WARN" || ln.Level == "ERROR" {
		text = levelStyle[ln.Level].Render(text)
	}
	return styleMuted.Render(ln.Time.Format("15:04:05")) + "  " + level + "  " + text
}

func outcomeMark(o hashiru.Outcome) string {
	switch o {
	case hashiru.Succeeded:
		return styleOK.Render("✔")
	case hashiru.Failed:
		return styleFail.Render("✘")
	case hashiru.Unfinished:
		return styleWarn.Render("…")
	default:
		return styleMuted.Render("·")
	}
}

// describe names a run: an update (it opens with the pull), a run of one stage,
// or an install of several.
func describe(r hashiru.Run) string {
	switch {
	case r.Outcome == hashiru.Loose:
		if len(r.Lines) > 0 {
			return r.Lines[0].Text
		}
		return "outside a run"
	case len(r.Lines) > 0 && strings.HasPrefix(r.Lines[0].Text, "Pulling latest Hashiru"):
		return fmt.Sprintf("update, %d stages", len(r.Stages))
	case len(r.Stages) == 1:
		return "stage " + r.Stages[0].Script
	case len(r.Stages) == 0:
		return "install"
	default:
		return fmt.Sprintf("install, %d stages", len(r.Stages))
	}
}

func counts(r hashiru.Run) string {
	var parts []string
	if r.Warns > 0 {
		parts = append(parts, styleWarn.Render(fmt.Sprintf("%d warn", r.Warns)))
	}
	if r.Errors > 0 && r.Outcome != hashiru.Loose {
		parts = append(parts, styleFail.Render(fmt.Sprintf("%d error", r.Errors)))
	}
	if len(parts) == 0 {
		return ""
	}
	return "  " + strings.Join(parts, " ")
}

// duration is first to last timed line, as Hashiru's fmt_duration prints it.
func duration(r hashiru.Run) string {
	var first, last time.Time
	for _, ln := range r.Lines {
		if ln.Time.IsZero() {
			continue
		}
		if first.IsZero() {
			first = ln.Time
		}
		last = ln.Time
	}
	d := last.Sub(first)
	switch {
	case first.IsZero() || d <= 0:
		return ""
	case d < time.Minute:
		return fmt.Sprintf("%ds", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm %ds", int(d.Minutes()), int(d.Seconds())%60)
	default:
		return fmt.Sprintf("%dh %dm", int(d.Hours()), int(d.Minutes())%60)
	}
}
