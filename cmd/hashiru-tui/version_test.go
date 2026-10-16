package main

import (
	"runtime/debug"
	"testing"
)

func TestVersion(t *testing.T) {
	tagged := &debug.BuildInfo{Main: debug.Module{Version: "v0.1.0"}}
	if got := version(tagged, true); got != "v0.1.0" {
		t.Errorf("go install @v0.1.0: %q", got)
	}
	local := &debug.BuildInfo{Main: debug.Module{Version: "(devel)"}}
	if got := version(local, true); got != "devel" {
		t.Errorf("local build: %q", got)
	}
	if got := version(nil, false); got != "devel" {
		t.Errorf("no build info: %q", got)
	}
}
