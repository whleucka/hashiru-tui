package hashiru

import (
	"os"
	"testing"
)

func TestParseStages(t *testing.T) {
	out, err := os.ReadFile("testdata/install-list.txt")
	if err != nil {
		t.Fatal(err)
	}
	stages := ParseStages(out)
	if len(stages) != 9 {
		t.Fatalf("got %d stages: %+v", len(stages), stages)
	}
	if got := stages[5]; got != (Stage{Num: "45", Name: "config", File: "45-config.sh"}) {
		t.Errorf("stage 45: %+v", got)
	}
	if got := ParseStages([]byte("Available stages:\n  notastage.sh\n  x-y.sh\n")); got != nil {
		t.Errorf("junk parsed as %+v", got)
	}
}
