package automap

import (
	"errors"
	"os"
	"strings"
	"testing"

	"gddoom/internal/mapdata"

	"github.com/hajimehoshi/ebiten/v2"
)

func TestParseDemoScriptV1(t *testing.T) {
	src := `
# comment
gddoom-demo-v1
25 0 0 0 0 0 0
50 -10 1 0 1 1 0
0 0 0 -65536 0 0 1
`
	d, err := ParseDemoScript(src)
	if err != nil {
		t.Fatalf("parse demo: %v", err)
	}
	if got := len(d.Tics); got != 3 {
		t.Fatalf("tic count=%d want=3", got)
	}
	if d.Tics[1].Forward != 50 || d.Tics[1].Side != -10 || d.Tics[1].Turn != 1 || !d.Tics[1].Run || !d.Tics[1].Use {
		t.Fatalf("unexpected tic[1]: %+v", d.Tics[1])
	}
	if d.Tics[2].TurnRaw != -65536 || !d.Tics[2].Fire {
		t.Fatalf("unexpected tic[2]: %+v", d.Tics[2])
	}
}

func TestParseDemoScriptErrors(t *testing.T) {
	cases := []string{
		"",
		"bad-header\n1 2 3 4 0 0 0\n",
		"gddoom-demo-v1\n1 2 3\n",
		"gddoom-demo-v1\n1 2 3 4 2 0 0\n",
	}
	for _, tc := range cases {
		if _, err := ParseDemoScript(tc); err == nil {
			t.Fatalf("expected parse error for %q", tc)
		}
	}
}

func TestParseDemoScriptHeaderCaseInsensitive(t *testing.T) {
	d, err := ParseDemoScript("GDDOOM-DEMO-V1\n0 0 0 0 0 0 0\n")
	if err != nil {
		t.Fatalf("parse demo: %v", err)
	}
	if len(d.Tics) != 1 {
		t.Fatalf("tic count=%d want=1", len(d.Tics))
	}
}

func TestParseDemoScriptRejectsNoTics(t *testing.T) {
	_, err := ParseDemoScript("gddoom-demo-v1\n# no tics\n")
	if err == nil || !strings.Contains(err.Error(), "no tics") {
		t.Fatalf("expected no tics error, got: %v", err)
	}
}

func TestFormatDemoScriptRoundTrip(t *testing.T) {
	in := []DemoTic{
		{Forward: 25, Side: 0, Turn: 1, TurnRaw: 0, Run: false, Use: false, Fire: false},
		{Forward: -50, Side: 17, Turn: -1, TurnRaw: -65536, Run: true, Use: true, Fire: true},
	}
	txt := FormatDemoScript(in)
	out, err := ParseDemoScript(txt)
	if err != nil {
		t.Fatalf("parse roundtrip: %v", err)
	}
	if len(out.Tics) != len(in) {
		t.Fatalf("tic count=%d want=%d", len(out.Tics), len(in))
	}
	for i := range in {
		if out.Tics[i] != in[i] {
			t.Fatalf("tic[%d]=%+v want %+v", i, out.Tics[i], in[i])
		}
	}
}

func TestDemoUpdateModeTerminatesAtEnd(t *testing.T) {
	g := mustLoadE1M1GameForMapTextureTests(t)
	g.opts.DemoScript = &DemoScript{
		Tics: []DemoTic{
			{Forward: 25},
			{Forward: 25},
		},
	}
	startX := g.p.x
	startY := g.p.y
	if err := g.Update(); err != nil {
		t.Fatalf("update 1: %v", err)
	}
	if err := g.Update(); err != nil {
		t.Fatalf("update 2: %v", err)
	}
	if g.p.x == startX && g.p.y == startY {
		t.Fatal("expected demo movement to update player position")
	}
	err := g.Update()
	if !errors.Is(err, ebiten.Termination) {
		t.Fatalf("expected ebiten.Termination after demo end, got %v", err)
	}
}

func TestSessionGameRebuildPreservesRecordedDemoTics(t *testing.T) {
	sg := &sessionGame{
		opts: Options{RecordDemoPath: "out.demo"},
		g: &game{
			opts:       Options{RecordDemoPath: "out.demo"},
			demoRecord: []DemoTic{{Forward: 10}, {Forward: 20}},
		},
	}
	sg.rebuildGameWithPersistentSettings(&mapdata.Map{})
	if got := len(sg.demoRecord); got != 2 {
		t.Fatalf("collected demo tics=%d want=2", got)
	}
	if sg.g == nil {
		t.Fatal("expected rebuilt game")
	}
	sg.g.demoRecord = append(sg.g.demoRecord, DemoTic{Forward: 30})
	if got := len(sg.effectiveDemoRecord()); got != 3 {
		t.Fatalf("effective demo tics=%d want=3", got)
	}
}

func TestSaveDemoScriptRejectsNoTics(t *testing.T) {
	path := t.TempDir() + "/empty.demo"
	err := SaveDemoScript(path, nil)
	if err == nil || !strings.Contains(err.Error(), "no tics") {
		t.Fatalf("expected no tics error, got %v", err)
	}
	if _, statErr := os.Stat(path); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("empty demo file should not be written, stat err=%v", statErr)
	}
}
