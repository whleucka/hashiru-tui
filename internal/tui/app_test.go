package tui

import (
	"context"
	"errors"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/whleucka/hashiru-tui/internal/hashiru"
)

type fakeSource struct {
	status    hashiru.Status
	report    hashiru.Report
	statusErr error
	runs      []hashiru.Run
}

func (f fakeSource) Status(context.Context) (hashiru.Status, error) { return f.status, f.statusErr }
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

// load runs Init's commands synchronously and feeds the results back in.
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
	}
	r := []rune(s)[0]
	return tea.KeyPressMsg{Code: r, Text: s}
}

func TestDashboardShowsStatusAndDoctor(t *testing.T) {
	m := New(Options{Source: fakeSource{
		status: hashiru.Status{Version: "v1.13.0", Behind: 2, BehindOK: true, Dirty: true},
		report: sampleReport(),
	}})
	load(t, m)

	out := m.View().Content
	for _, want := range []string{"v1.13.0", "2 commit(s) behind", "dirty", "2 ok", "1 warnings"} {
		if !strings.Contains(out, want) {
			t.Errorf("dashboard missing %q:\n%s", want, out)
		}
	}
}

func TestStatusErrorIsShown(t *testing.T) {
	m := New(Options{Source: fakeSource{statusErr: errors.New("hashiru: not found")}})
	load(t, m)
	if out := m.View().Content; !strings.Contains(out, "hashiru: not found") {
		t.Errorf("error not shown:\n%s", out)
	}
}

func TestDoctorShowsProblemsAndExpands(t *testing.T) {
	m := New(Options{Source: fakeSource{report: sampleReport()}})
	load(t, m)
	m.Update(key("2"))

	out := m.View().Content
	if !strings.Contains(out, "2 commit(s) behind") {
		t.Error("a warning should show without expanding")
	}
	if strings.Contains(out, "repo clean") {
		t.Error("an ok check should be hidden until expanded")
	}

	m.Update(key("enter"))
	if out := m.View().Content; !strings.Contains(out, "repo clean") {
		t.Error("enter should expand the section under the cursor")
	}

	m.Update(key("j"))
	m.Update(key("j")) // past the end: stays on the last section
	if m.cursor != 1 {
		t.Errorf("cursor = %d, want 1", m.cursor)
	}
}

func TestActionsOpenHerdrPanes(t *testing.T) {
	type call struct {
		argv         string
		right, focus bool
	}
	var got []call
	m := New(Options{
		Source: fakeSource{},
		RunInPane: func(_ context.Context, argv []string, _ string, right, focus bool) error {
			got = append(got, call{strings.Join(argv, " "), right, focus})
			return nil
		},
	})
	load(t, m) // 120x40: wide, so panes go right

	for _, k := range []string{"u", "e", "L"} {
		_, cmd := m.Update(key(k))
		drain(m, cmd)
	}
	want := []call{
		{"hashiru update", true, true},
		{"hashiru config", true, true},
		{"hashiru log -f", true, false}, // watched, not typed into
	}
	if len(got) != len(want) {
		t.Fatalf("calls = %+v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("call %d = %+v, want %+v", i, got[i], want[i])
		}
	}
	if !strings.Contains(m.note, "new pane") {
		t.Errorf("note = %q", m.note)
	}
}

func TestPaneGoesDownOnTallTerminals(t *testing.T) {
	var right bool
	m := New(Options{
		Source: fakeSource{},
		RunInPane: func(_ context.Context, _ []string, _ string, r, _ bool) error {
			right = r
			return nil
		},
	})
	m.Update(tea.WindowSizeMsg{Width: 80, Height: 60})
	_, cmd := m.Update(key("u"))
	drain(m, cmd)
	if right {
		t.Error("80x60 is tall: the pane should go below")
	}
}

func TestTabCycles(t *testing.T) {
	m := New(Options{Source: fakeSource{}})
	m.Update(key("tab"))
	if m.tab != tabDoctor {
		t.Fatalf("tab = %v", m.tab)
	}
	m.Update(key("tab"))
	m.Update(key("tab"))
	if m.tab != tabDashboard {
		t.Fatalf("tab = %v, want wrap to Dashboard", m.tab)
	}
}
