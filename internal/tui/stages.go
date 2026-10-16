package tui

import (
	"context"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/whleucka/hashiru-tui/internal/hashiru"
)

// stagesTimeout: `install --list` answers before install.sh's network
// check, so it's instant; this only guards against something odd.
const stagesTimeout = 3 * time.Second

// stagePicker re-applies one of install.sh's stages with `hashiru install
// N`: re-run config (45) or snapper (50) without a whole update. It doesn't
// pull, so it works on a dirty checkout too.
type stagePicker struct {
	stages []hashiru.Stage
	open   bool
	sel    int
}

type stagesMsg struct{ stages []hashiru.Stage }

// readStages lists the stages. Unknown (no hashiru, an error) is no stages:
// the picker and its actions just aren't there.
func (u *updates) readStages() []hashiru.Stage {
	argv := u.e.src.Argv("install", "--list")
	ctx, cancel := context.WithTimeout(context.Background(), stagesTimeout)
	defer cancel()
	out, err := u.e.probe(ctx, argv[0], argv[1:]...)
	if err != nil {
		return nil
	}
	return hashiru.ParseStages(out)
}

func (u *updates) loadStages() tea.Cmd {
	return func() tea.Msg { return stagesMsg{u.readStages()} }
}

func (u *updates) stageActions() []Action {
	var acts []Action
	for _, s := range u.stage.stages {
		acts = append(acts, Action{
			ID:    "updates.stage-" + s.Num,
			Title: "Re-apply stage " + strings.TrimSuffix(s.File, ".sh") + " (hashiru install " + s.Num + ")",
			Run:   func() tea.Cmd { return u.applyStage(s) },
		})
	}
	return acts
}

func (u *updates) applyStage(s hashiru.Stage) tea.Cmd {
	return u.e.launch(Launch{
		Argv:    u.e.src.Argv("install", s.Num),
		Label:   "hashiru install " + s.Num,
		Kind:    Interactive,
		Refresh: true,
	})
}

func (p *stagePicker) handleKey(u *updates, msg tea.KeyPressMsg) (tea.Cmd, bool) {
	switch msg.String() {
	case "esc", "h", "left", "s":
		p.open = false
	case "j", "down", "k", "up", "ctrl+d", "ctrl+u":
		p.sel, _ = moveKey(msg.String(), p.sel, len(p.stages), u.notes.height-2)
	case "enter", "l", "right":
		p.open = false
		if p.sel < len(p.stages) {
			return u.applyStage(p.stages[p.sel]), true
		}
	default:
		return nil, false
	}
	return nil, true
}

func (p *stagePicker) keys() []Key {
	return []Key{
		{Keys: "j/k", Desc: "move"},
		{Keys: "enter", Desc: "re-apply"},
		{Keys: "esc", Desc: "cancel"},
	}
}

func (p *stagePicker) view(u *updates) string {
	var b strings.Builder
	b.WriteString(styleTitle.Render("Re-apply a stage") + styleMuted.Render("  runs install.sh for just that stage; doesn't pull") + "\n\n")
	for i, s := range p.stages {
		cmd := styleMuted.Render("  " + strings.Join(u.e.src.Argv("install", s.Num), " "))
		if i == p.sel {
			b.WriteString(styleTitle.Render("▸ ") + styleSelected.Render(s.File) + cmd + "\n")
		} else {
			b.WriteString("  " + s.File + cmd + "\n")
		}
	}
	return b.String()
}
