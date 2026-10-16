package hashiru

import (
	"slices"
	"testing"
)

func TestBaseVersion(t *testing.T) {
	for in, want := range map[string]string{"v1.13.0-30-gc9ab1a4": "v1.13.0", "v1.13.0": "v1.13.0", "": ""} {
		if got := BaseVersion(in); got != want {
			t.Errorf("%q: %q, want %q", in, got, want)
		}
	}
}

func TestReleaseTagsSortsByVersion(t *testing.T) {
	ls := []byte("docs/releases/v1.9.0.md\ndocs/releases/v1.10.0.md\ndocs/releases/README.md\ndocs/releases/v1.10.1.md\ndocs/releases/v1.2.md\n")
	if got, want := ReleaseTags(ls), []string{"v1.10.1", "v1.10.0", "v1.9.0", "v1.2"}; !slices.Equal(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestNotesFor(t *testing.T) {
	tags := []string{"v1.14.0", "v1.13.1", "v1.13.0", "v1.12.0"}
	for _, tc := range []struct {
		name      string
		tags      []string
		current   string
		behind    bool
		want      []string
		wantNewer bool
	}{
		{"behind two releases", tags, "v1.13.0", true, []string{"v1.14.0", "v1.13.1"}, true},
		{"up to date", tags, "v1.13.0", false, []string{"v1.13.0"}, false},
		{"behind, no release yet", tags, "v1.14.0", true, []string{"v1.14.0"}, false},
		{"no notes for this version", tags, "v0.9.0", false, nil, false},
	} {
		got, newer := NotesFor(tc.tags, tc.current, tc.behind)
		if !slices.Equal(got, tc.want) || newer != tc.wantNewer {
			t.Errorf("%s: %v %v, want %v %v", tc.name, got, newer, tc.want, tc.wantNewer)
		}
	}
}

func TestReleaseSummary(t *testing.T) {
	r := Release{Tag: "v1.13.0", Body: "\n# v1.13.0\n\nFingerprint release. `hashiru fingerprint on`\nsets it up.\n\n## Fingerprint\n"}
	if got := r.Summary(); got != "Fingerprint release. `hashiru fingerprint on` sets it up." {
		t.Errorf("summary %q", got)
	}
}
