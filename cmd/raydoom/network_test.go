//go:build raylib && cgo && !js

package main

import (
	"flag"
	"testing"

	"gddoom/internal/demo"
	"gddoom/internal/doomruntime"
)

type nativeWatchTestSource struct{}

func (nativeWatchTestSource) PollTic() (demo.Tic, bool, error) { return demo.Tic{}, false, nil }

func TestNativeWatchMenuDisablesGameMutations(t *testing.T) {
	m := newNativeMenu(doomruntime.Options{LiveTicSource: nativeWatchTestSource{}}, nil, "")
	s := nativeSettings{}
	m.open(menuMain)
	if m.row != 1 {
		t.Fatal("watch main menu did not select Options")
	}
	for _, row := range []int{4, 5, 1} {
		m.update(menuInput{down: true, mouseRow: -1}, &s)
		if m.row != row {
			t.Fatalf("watch navigation selected disabled row %d, want %d", m.row, row)
		}
	}
	for _, row := range []int{0, 2, 3} {
		m.row = row
		if command := m.update(menuInput{confirm: true, mouseRow: -1}, &s); command != menuNoCommand || m.page != menuMain || m.row != 1 {
			t.Fatalf("watch main menu activated disabled row %d", row)
		}
	}
}

func TestNativeNetworkFlagsMatchMainShorthand(t *testing.T) {
	for _, tc := range []struct {
		args             []string
		broadcast, watch string
		invalid          bool
	}{
		{args: []string{}},
		{args: []string{"-broadcast"}, broadcast: "127.0.0.1:6670"},
		{args: []string{"-broadcast", "example"}, broadcast: "example:6670"},
		{args: []string{"-watch", "-watch-session", "123"}, watch: "127.0.0.1:6670"},
		{args: []string{"-watch=localhost:7788", "-watch-session=123"}, watch: "localhost:7788"},
		{args: []string{"-watch"}, invalid: true},
		{args: []string{"-watch", "-watch-session=123", "-broadcast"}, invalid: true},
		{args: []string{"-broadcast", "-demo=DEMO1"}, invalid: true},
		{args: []string{"-broadcast", "-record-demo=example.lmp"}, invalid: true},
		{args: []string{"-broadcast", "-trace-demo-state=example.json"}, invalid: true},
	} {
		fs := flag.NewFlagSet("test", flag.ContinueOnError)
		for _, name := range []string{"broadcast", "watch", "demo", "record-demo", "trace-demo-state"} {
			fs.String(name, "", "")
		}
		fs.Uint64("watch-session", 0, "")
		if err := fs.Parse(nativeNetworkArgs(tc.args)); err != nil {
			t.Fatal(err)
		}
		b, w, err := nativeNetworkFlags(fs)
		if (err != nil) != tc.invalid || !tc.invalid && (b != tc.broadcast || w != tc.watch) {
			t.Fatalf("args=%v: got broadcast=%q watch=%q err=%v", tc.args, b, w, err)
		}
	}
}
