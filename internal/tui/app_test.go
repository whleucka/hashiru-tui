package tui

import (
	"context"
	"errors"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/whleucka/hashiru-tui/internal/hashiru"
)

type fakeSource struct {
	status    hashiru.Status
	report    hashiru.Report
	statusErr error
	runs      []hashiru.Run
	calls     *int // counts Status calls, when set
}

func (f fakeSource) Status(context.Context) (hashiru.Status, error) {
	if f.calls != nil {
		*f.calls++
	}
	return f.status, f.statusErr
}
func (f fakeSource) Doctor(context.Context) (hashiru.Report, error) { return f.report, nil }
func (f fakeSource) Log() ([]hashiru.Run, error)                    { return f.runs, nil }
func (f fakeSource) Argv(args ...string) []string                   { return append([]string{"hashiru"}, args...) }

func sampleReport() hashiru.Report {
	var r hashiru.Report
	r.Checks = []hashiru.Check{
		{Section: "Install", Status: hashiru.OK, Message: "repo clean"},
		{Section: "Install", Status: hashiru.Warn, Message: "2 commit(s) behind"},
		{Section: "Shell", Status: hashiru.OK, Message: "zsh is the login shell"},
	}
	r.Summary.OK, r.Summary.Warn = 2, 1
	return r
}

// load sizes the model and runs Init's commands synchronously.
func load(t *testing.T, m *Model) {
	t.Helper()
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	drain(m, m.Init())
}

// drain runs cmd and any batch it holds, feeding each message to Update.
func drain(m *Model, cmd tea.Cmd) {
	if cmd == nil {
		return
	}
	switch msg := cmd().(type) {
	case tea.BatchMsg:
		for _, c := range msg {
			drain(m, c)
		}
	case nil:
	default:
		_, next := m.Update(msg)
		drain(m, next)
	}
}

func key(s string) tea.KeyPressMsg {
	switch s {
	case "tab":
		return tea.KeyPressMsg{Code: tea.KeyTab}
	case "enter":
		return tea.KeyPressMsg{Code: tea.KeyEnter}
	case "esc":
		return tea.KeyPressMsg{Code: tea.KeyEscape}
	}
	r := []rune(s)[0]
	return tea.KeyPressMsg{Code: r, Text: s}
}

func press(m *Model, keys ...string) {
	for _, k := range keys {
		_, cmd := m.Update(key(k))
		drain(m, cmd)
	}
}

// goTo selects the named area in the sidebar and focuses it.
func goTo(t *testing.T, m *Model, name string) {
	t.Helper()
	for i, a := range m.areas {
		if a.Name() == name {
			m.sel = i
			m.inArea = true
			return
		}
	}
	t.Fatalf("no area %q", name)
}

// areaKey is the number key that selects the named area.
func areaKey(t *testing.T, m *Model, name string) string {
	t.Helper()
	for i, a := range m.areas {
		if a.Name() == name {
			return areaDigit(i)
		}
	}
	t.Fatalf("no area %q", name)
	return ""
}

func TestSidebarListsAreas(t *testing.T) {
	m := New(Options{Source: fakeSource{}})
	load(t, m)
	out := m.View().Content
	for _, name := range []string{"Overview", "Updates", "Logs"} {
		if !strings.Contains(out, name) {
			t.Errorf("sidebar missing %q:\n%s", name, out)
		}
	}
}

func TestSidebarNavigation(t *testing.T) {
	m := New(Options{Source: fakeSource{}})
	load(t, m)

	press(m, "j")
	if m.area().Name() != "Updates" {
		t.Errorf("j: area = %s", m.area().Name())
	}
	press(m, areaKey(t, m, "Logs"))
	if m.area().Name() != "Logs" {
		t.Errorf("number key: area = %s", m.area().Name())
	}
	for range len(m.areas) + 1 {
		press(m, "k") // stops at the top
	}
	if m.sel != 0 {
		t.Errorf("sel = %d", m.sel)
	}

	press(m, "enter")
	if !m.inArea {
		t.Fatal("enter should focus the area")
	}
	press(m, "esc")
	if m.inArea {
		t.Error("esc should return to the sidebar")
	}
}

func TestStateLoadedOnceSharedByAreas(t *testing.T) {
	calls := 0
	m := New(Options{Source: fakeSource{
		status: hashiru.Status{Version: "v1.13.0", Behind: 2, BehindOK: true, Dirty: true},
		report: sampleReport(),
		calls:  &calls,
	}})
	load(t, m)
	if calls != 1 {
		t.Errorf("status fetched %d times, want 1", calls)
	}

	overview := m.View().Content
	for _, want := range []string{"v1.13.0", "2 commit(s) behind", "2 ok", "1 warnings"} {
		if !strings.Contains(overview, want) {
			t.Errorf("Overview missing %q:\n%s", want, overview)
		}
	}

	goTo(t, m, "Updates")
	updates := m.View().Content
	for _, want := range []string{"2 commit(s) behind", "Install", "Shell"} {
		if !strings.Contains(updates, want) {
			t.Errorf("Updates missing %q:\n%s", want, updates)
		}
	}

	press(m, "r")
	if calls != 2 {
		t.Errorf("after r, status fetched %d times, want 2", calls)
	}
}

func TestStatusErrorIsShown(t *testing.T) {
	m := New(Options{Source: fakeSource{statusErr: errors.New("hashiru: not found")}})
	load(t, m)
	if out := m.View().Content; !strings.Contains(out, "hashiru: not found") {
		t.Errorf("error not shown:\n%s", out)
	}
}

func TestUpdatesHealthExpands(t *testing.T) {
	m := New(Options{Source: fakeSource{report: sampleReport()}})
	load(t, m)
	goTo(t, m, "Updates")

	out := m.View().Content
	if strings.Contains(out, "repo clean") {
		t.Error("an ok check should be hidden until expanded")
	}
	press(m, "enter")
	if out := m.View().Content; !strings.Contains(out, "repo clean") {
		t.Error("enter should expand the section under the cursor")
	}
	press(m, "j", "j")
	press(m, "esc")
	if m.inArea {
		t.Error("esc in Updates should return to the sidebar")
	}
}

func TestAreaKeysWorkFromSidebar(t *testing.T) {
	var got []string
	m := New(Options{
		Source: fakeSource{},
		RunInPane: func(_ context.Context, argv []string, _ string, _, _ bool) error {
			got = append(got, strings.Join(argv, " "))
			return nil
		},
	})
	load(t, m)
	press(m, "j") // Updates, still in the sidebar
	press(m, "u")
	if len(got) != 1 || got[0] != "hashiru update" {
		t.Errorf("u from the sidebar ran %v", got)
	}
}

func quits(cmd tea.Cmd) bool {
	if cmd == nil {
		return false
	}
	_, ok := cmd().(tea.QuitMsg)
	return ok
}

func TestQuitAsksFirst(t *testing.T) {
	m := New(Options{Source: fakeSource{}})
	load(t, m)
	if _, cmd := m.Update(key("q")); quits(cmd) {
		t.Fatal("q quit without asking")
	}
	if f := ansi.Strip(m.footer()); !strings.Contains(f, "Quit hashiru-tui?") || !strings.Contains(f, "y quit") {
		t.Errorf("footer doesn't ask: %q", f)
	}
	if _, cmd := m.Update(key("y")); !quits(cmd) {
		t.Error("y after q should quit")
	}
}

func TestQuitAnyOtherKeyStays(t *testing.T) {
	for _, k := range []string{"n", "q", "esc", "j"} {
		m := New(Options{Source: fakeSource{}})
		load(t, m)
		sel := m.sel
		press(m, "q")
		if _, cmd := m.Update(key(k)); quits(cmd) {
			t.Errorf("q then %s quit", k)
		}
		if m.quitting || m.sel != sel {
			t.Errorf("q then %s: still asking %v, sel %d→%d (the key should only answer)", k, m.quitting, sel, m.sel)
		}
	}
}

func TestQuitFromAnArea(t *testing.T) {
	m := New(Options{Source: fakeSource{}})
	load(t, m)
	goTo(t, m, "Config")
	press(m, "q")
	if _, cmd := m.Update(key("y")); !quits(cmd) {
		t.Error("q, y inside an area should quit")
	}
}

func TestCtrlCQuitsAtOnce(t *testing.T) {
	m := New(Options{Source: fakeSource{}})
	load(t, m)
	if _, cmd := m.Update(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl}); !quits(cmd) {
		t.Error("ctrl+c should quit without asking")
	}
}

func TestFooterHelpMatchesFocus(t *testing.T) {
	m := New(Options{Source: fakeSource{}})
	load(t, m)
	press(m, areaKey(t, m, "Logs")) // from the sidebar
	footer := m.footer()
	if strings.Contains(footer, "problems only") {
		t.Errorf("sidebar footer shows the area's keys: %q", footer)
	}
	press(m, "enter")
	if footer := m.footer(); !strings.Contains(footer, "problems only") {
		t.Errorf("area footer missing its keys: %q", footer)
	}
}

func TestKeyLineDropsWholeKeys(t *testing.T) {
	keys := []Key{{Keys: "j/k", Desc: "move"}, {Keys: "enter", Desc: "edit"}, {Keys: "q", Desc: "quit"}}
	if got := keyLine(keys, 80); got != "j/k move · enter edit · q quit" {
		t.Errorf("wide: %q", got)
	}
	// "j/k move · enter edit" is 21 cells; one short of that keeps only the first.
	if got := keyLine(keys, 20); got != "j/k move" {
		t.Errorf("narrow: %q", got)
	}
}

// Every area but Overview has keys, and each one says what it does, so the
// footer and the keymap popup never show a bare key.
func TestAreasDescribeTheirKeys(t *testing.T) {
	m := New(Options{Source: fakeSource{}})
	load(t, m)
	for _, a := range m.areas {
		keys := a.Keys()
		if len(keys) == 0 && a.Name() != "Overview" {
			t.Errorf("%s: no keys", a.Name())
		}
		for _, k := range keys {
			if k.Keys == "" || k.Desc == "" {
				t.Errorf("%s: incomplete key %+v", a.Name(), k)
			}
		}
	}
}

func TestAreaKeysHideTheShellsForTheSameKey(t *testing.T) {
	keys := withGlobal([]Key{{Keys: "esc", Desc: "back to the list"}}, inAreaKeys)
	n := 0
	for _, k := range keys {
		if k.Keys == "esc" {
			n++
		}
	}
	if n != 1 || keys[0].Desc != "back to the list" {
		t.Errorf("esc listed %d times: %+v", n, keys)
	}
}
