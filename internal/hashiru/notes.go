package hashiru

import (
	"slices"
	"strconv"
	"strings"
)

// ReleasesDir is where a Hashiru checkout keeps its release notes, one
// vX.Y.Z.md per release.
const ReleasesDir = "docs/releases"

// Release is one release's notes.
type Release struct {
	Tag  string // "v1.13.0"
	Body string // the markdown
}

// Summary is the notes' first paragraph of prose, past the "# vX.Y.Z"
// heading, on one line: the files are hard-wrapped.
func (r Release) Summary() string {
	var para []string
	for _, l := range strings.Split(r.Body, "\n") {
		l = strings.TrimSpace(l)
		switch {
		case l == "" && len(para) > 0:
			return strings.Join(para, " ")
		case l == "", strings.HasPrefix(l, "#") && len(para) == 0:
		default:
			para = append(para, l)
		}
	}
	return strings.Join(para, " ")
}

// BaseVersion is the release a version string is at or after:
// "v1.13.0-30-gc9ab1a4" (git describe) is "v1.13.0".
func BaseVersion(v string) string {
	if i := strings.Index(v, "-"); i >= 0 {
		return v[:i]
	}
	return v
}

// ReleaseTags reads `git ls-tree --name-only <ref> docs/releases/` into the
// tags it has notes for, newest first.
func ReleaseTags(lsTree []byte) []string {
	var tags []string
	for _, l := range strings.Split(string(lsTree), "\n") {
		name := strings.TrimPrefix(strings.TrimSpace(l), ReleasesDir+"/")
		if tag, ok := strings.CutSuffix(name, ".md"); ok && version(tag) != nil {
			tags = append(tags, tag)
		}
	}
	slices.SortFunc(tags, func(a, b string) int { return CompareVersions(b, a) })
	return tags
}

// NotesFor picks which of tags (newest first) to show for a machine at
// current: when behind, the releases newer than it; otherwise, or when no
// release is newer yet, its own. newer says which it was.
func NotesFor(tags []string, current string, behind bool) (show []string, newer bool) {
	if behind {
		for _, t := range tags {
			if CompareVersions(t, current) > 0 {
				show = append(show, t)
			}
		}
		if len(show) > 0 {
			return show, true
		}
	}
	if slices.Contains(tags, current) {
		return []string{current}, false
	}
	return nil, false
}

// CompareVersions orders "vX.Y.Z" tags numerically, so v1.10.0 is after
// v1.9.0. A tag that isn't a version sorts before every one that is.
func CompareVersions(a, b string) int {
	va, vb := version(a), version(b)
	switch {
	case va == nil && vb == nil:
		return strings.Compare(a, b)
	case va == nil:
		return -1
	case vb == nil:
		return 1
	}
	return slices.Compare(va, vb)
}

func version(tag string) []int {
	s, ok := strings.CutPrefix(tag, "v")
	if !ok {
		return nil
	}
	var out []int
	for _, p := range strings.Split(s, ".") {
		n, err := strconv.Atoi(p)
		if err != nil {
			return nil
		}
		out = append(out, n)
	}
	return out
}
