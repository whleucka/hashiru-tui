package hashiru

import (
	"os"
	"strings"
	"testing"
)

func TestParseDoctor(t *testing.T) {
	f, err := os.Open("testdata/doctor.json")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	rep, err := ParseDoctor(f)
	if err != nil {
		t.Fatal(err)
	}
	if rep.Summary.OK != 58 || rep.Summary.Warn != 1 || rep.Summary.Fail != 0 {
		t.Errorf("Summary = %+v", rep.Summary)
	}

	secs := rep.Sections()
	if len(secs) != 8 || secs[0].Name != "Install" {
		t.Fatalf("Sections = %d, first %q", len(secs), secs[0].Name)
	}
	total := 0
	for _, s := range secs {
		total += len(s.Checks)
	}
	if total != len(rep.Checks) {
		t.Errorf("sections hold %d checks, report has %d", total, len(rep.Checks))
	}
}

func TestSectionWorst(t *testing.T) {
	tests := []struct {
		levels []Level
		want   Level
	}{
		{[]Level{OK, Skip}, OK},
		{[]Level{OK, Warn, OK}, Warn},
		{[]Level{Warn, Fail, OK}, Fail},
		{nil, OK},
	}
	for _, tt := range tests {
		var s Section
		for _, l := range tt.levels {
			s.Checks = append(s.Checks, Check{Status: l})
		}
		if got := s.Worst(); got != tt.want {
			t.Errorf("Worst(%v) = %s, want %s", tt.levels, got, tt.want)
		}
	}
}

func TestParseDoctorGarbage(t *testing.T) {
	if _, err := ParseDoctor(strings.NewReader("doctor: 3 ok")); err == nil {
		t.Error("want an error for non-JSON output")
	}
}
