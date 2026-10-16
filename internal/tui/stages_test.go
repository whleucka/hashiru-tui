package tui

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

func stagesModel(t *testing.T, ran *[]string) *Model {
	t.Helper()
	list, err := os.ReadFile("../hashiru/testdata/install-list.txt")
	if err != nil {
		t.Fatal(err)
	}
	m := New(Options{
		Source: fakeSource{},
		Probe:  probe{"hashiru install --list": string(list)}.run,
		RunInPane: func(_ context.Context, argv []string, _ string, _, _ bool) error {
			*ran = append(*ran, strings.Join(argv, " "))
			return nil
		},
	})
	load(t, m)
	goTo(t, m, "Updates")
	return m
}

func TestStagePickerReappliesOne(t *testing.T) {
	var ran []string
	m := stagesModel(t, &ran)
	press(m, "s")
	out := ansi.Strip(m.View().Content)
	if !strings.Contains(out, "Re-apply a stage") || !strings.Contains(out, "45-config.sh  hashiru install 45") {
		t.Fatalf("picker:\n%s", out)
	}
	press(m, "j", "j", "j", "j", "j", "enter") // 10 15 20 30 35 [45]
	if len(ran) != 1 || ran[0] != "hashiru install 45" {
		t.Errorf("ran %v, want hashiru install 45", ran)
	}
	if m.area().(*updates).stage.open {
		t.Error("picker still open after running")
	}
}

func TestStagePickerEscCancels(t *testing.T) {
	var ran []string
	m := stagesModel(t, &ran)
	press(m, "s", "esc")
	if m.area().(*updates).stage.open || len(ran) != 0 || !m.inArea {
		t.Errorf("esc: open %v, ran %v, in area %v", m.area().(*updates).stage.open, ran, m.inArea)
	}
}

func TestStageActionsInThePaletteAndDeepLinks(t *testing.T) {
	var ran []string
	m := stagesModel(t, &ran)
	m.Update(ctrlP())
	typeText(m, "stage 50")
	press(m, "enter")
	if len(ran) != 1 || ran[0] != "hashiru install 50" {
		t.Errorf("palette ran %v", ran)
	}
	if err := m.Start("", "updates.stage-99"); err != nil {
		t.Errorf("deep link to a stage: %v", err)
	}
}

func TestNoHashiruNoStages(t *testing.T) {
	m := New(Options{Source: fakeSource{}}) // every probe "not installed"
	load(t, m)
	goTo(t, m, "Updates")
	press(m, "s")
	if m.area().(*updates).stage.open {
		t.Error("picker opened with no stages")
	}
	for _, a := range m.Actions() {
		if strings.HasPrefix(a.ID, "updates.stage-") {
			t.Errorf("stage action without stages: %s", a.ID)
		}
	}
}
