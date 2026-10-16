package herdr

import "testing"

func TestParseSplit(t *testing.T) {
	id, err := parseSplit([]byte(`{"id":"1","result":{"pane":{"pane_id":"w1:t2:p7","tab_id":"w1:t2"}}}`))
	if err != nil || id != "w1:t2:p7" {
		t.Errorf("id %q err %v", id, err)
	}
	for _, bad := range []string{``, `{}`, `{"result":{"pane":{}}}`, `not json`} {
		if _, err := parseSplit([]byte(bad)); err == nil {
			t.Errorf("%q should fail", bad)
		}
	}
}

func TestCommandLine(t *testing.T) {
	got := CommandLine([]string{"hashiru", "update", "45+", "it's"})
	want := `hashiru update 45+ 'it'\''s'`
	if got != want {
		t.Errorf("got  %s\nwant %s", got, want)
	}
}

func TestAvailable(t *testing.T) {
	t.Setenv("HERDR_PANE_ID", "")
	if Available() {
		t.Error("no HERDR_PANE_ID: not available")
	}
}
