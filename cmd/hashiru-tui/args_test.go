package main

import "testing"

func TestParseArgs(t *testing.T) {
	for _, tc := range []struct {
		argv []string
		want args
	}{
		{nil, args{}},
		{[]string{"updates"}, args{area: "updates"}},
		{[]string{"--run", "snapshots.new"}, args{run: "snapshots.new"}},
		{[]string{"--run=snapshots.new"}, args{run: "snapshots.new"}},
		{[]string{"logs", "--run", "logs.follow-journal"}, args{area: "logs", run: "logs.follow-journal"}},
		{[]string{"--list-actions"}, args{listActions: true}},
		{[]string{"-V"}, args{version: true}},
	} {
		got, err := parseArgs(tc.argv)
		if err != nil || got != tc.want {
			t.Errorf("%q: got %+v, %v; want %+v", tc.argv, got, err, tc.want)
		}
	}
	for _, bad := range [][]string{{"--run"}, {"--bogus"}, {"updates", "logs"}, {"--help"}} {
		if _, err := parseArgs(bad); err == nil {
			t.Errorf("%q: no error", bad)
		}
	}
}
