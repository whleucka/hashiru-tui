package tui

import (
	"strings"

	tea "charm.land/bubbletea/v2"
)

// overview is the home screen: the machine at a glance.
type overview struct{ e *env }

func newOverview(e *env) *overview { return &overview{e: e} }

func (o *overview) Name() string                              { return "Overview" }
func (o *overview) Update(tea.Msg) tea.Cmd                    { return nil }
func (o *overview) HandleKey(tea.KeyPressMsg) (tea.Cmd, bool) { return nil, false }
func (o *overview) Help() string                              { return "" }
func (o *overview) Actions() []Action                         { return nil }

func (o *overview) View(width, height int) string {
	st := o.e.state
	var b strings.Builder
	row := func(k, v string) { b.WriteString(styleKey.Render(k) + v + "\n") }

	b.WriteString(styleTitle.Render("Hashiru") + "\n")
	switch {
	case st.Status == nil && st.StatusBusy:
		row("status", styleMuted.Render("loading… (fetches upstream)"))
	case st.Status == nil && st.StatusErr != nil:
		row("status", styleFail.Render(st.StatusErr.Error()))
	case st.Status != nil:
		s := st.Status
		row("version", orUnknown(s.Version))
		row("updated", when(s.Updated))
		row("upstream", upstreamLine(s))
	}
	row("packages", pendingLine(st))
	row("arch news", newsLine(st))
	switch {
	case st.Report == nil && st.ReportBusy:
		row("health", styleMuted.Render("running doctor…"))
	case st.Report == nil && st.ReportErr != nil:
		row("health", styleFail.Render(st.ReportErr.Error()))
	case st.Report != nil:
		row("health", summaryLine(st.Report))
	}
	return b.String()
}
