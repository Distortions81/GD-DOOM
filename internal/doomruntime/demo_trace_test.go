package doomruntime

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"gddoom/internal/mapdata"
)

func TestDemoTraceWritesMetaDemoAndTics(t *testing.T) {
	base := mustLoadE1M1GameForMapTextureTests(t)
	tracePath := t.TempDir() + "/demo-trace.jsonl"
	g := newGame(base.m, Options{
		Width:   320,
		Height:  200,
		WADHash: "test-wad",
		DemoScript: &DemoScript{
			Path: "demo1",
			Header: DemoHeader{
				Version:      demoVersion109,
				Skill:        2,
				Episode:      1,
				Map:          1,
				PlayerInGame: [4]bool{true},
			},
			Tics: []DemoTic{{Forward: 25}, {Forward: 25}},
		},
		DemoTracePath: tracePath,
	})

	for i := 0; i < 3; i++ {
		if err := g.Update(); err != nil {
			t.Fatalf("update %d: %v", i, err)
		}
	}

	data, err := os.ReadFile(tracePath)
	if err != nil {
		t.Fatalf("read trace: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if got, want := len(lines), 4; got != want {
		t.Fatalf("trace lines=%d want=%d\n%s", got, want, data)
	}
	if !strings.Contains(lines[0], `"kind":"meta"`) {
		t.Fatalf("meta line missing: %s", lines[0])
	}
	if !strings.Contains(lines[1], `"kind":"demo"`) {
		t.Fatalf("demo line missing: %s", lines[1])
	}
	if !strings.Contains(lines[2], `"kind":"tic"`) || !strings.Contains(lines[3], `"kind":"tic"`) {
		t.Fatalf("tic lines missing:\n%s", data)
	}
	if !strings.Contains(lines[2], `"mobjs"`) || !strings.Contains(lines[2], `"specials"`) {
		t.Fatalf("tic payload missing state arrays: %s", lines[2])
	}
	var tic map[string]any
	if err := json.Unmarshal([]byte(lines[2]), &tic); err != nil {
		t.Fatalf("unmarshal tic: %v", err)
	}
	if got := int(tic["rndindex"].(float64)); got != 1 {
		t.Fatalf("rndindex=%d want=1", got)
	}
}

func TestDemoTraceContinuesWhenPlayerDies(t *testing.T) {
	base := mustLoadE1M1GameForMapTextureTests(t)
	tracePath := t.TempDir() + "/demo-trace.jsonl"
	g := newGame(base.m, Options{
		Width:   320,
		Height:  200,
		WADHash: "test-wad",
		DemoScript: &DemoScript{
			Path: "demo1",
			Header: DemoHeader{
				Version:      demoVersion109,
				Skill:        2,
				Episode:      1,
				Map:          1,
				PlayerInGame: [4]bool{true},
			},
			Tics: []DemoTic{{Forward: 25}, {Forward: 25}},
		},
		DemoTracePath: tracePath,
	})
	g.isDead = true

	err := g.Update()
	if err != nil {
		t.Fatalf("Update() err=%v want nil", err)
	}

	data, readErr := os.ReadFile(tracePath)
	if readErr != nil {
		t.Fatalf("read trace: %v", readErr)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if got, want := len(lines), 3; got != want {
		t.Fatalf("trace lines=%d want=%d\n%s", got, want, data)
	}
}

func TestDemoTraceThingReactionDoesNotFallBackToSpawnDefault(t *testing.T) {
	g := &game{
		m: &mapdata.Map{
			Things: []mapdata.Thing{
				{Type: 58},
			},
		},
		thingReactionTics: []int{0},
	}
	if got := demoTraceThingReaction(g, 0); got != 0 {
		t.Fatalf("reactiontime=%d want 0", got)
	}
}

func TestDemoTraceThingTargetUsesConcreteTargetFields(t *testing.T) {
	g := &game{
		m: &mapdata.Map{
			Things: []mapdata.Thing{
				{Type: 3001},
				{Type: 3004},
			},
		},
		thingTargetPlayer: []bool{true, false},
		thingTargetIdx:    []int{-1, 0},
		thingAggro:        []bool{false, false},
	}

	target, targetType := demoTraceThingTarget(g, 0)
	if target != 1 || targetType != 0 {
		t.Fatalf("player target=(%d,%d) want (1,0)", target, targetType)
	}

	target, targetType = demoTraceThingTarget(g, 1)
	if target != 2 || targetType != demoTraceThingType(3001) {
		t.Fatalf("thing target=(%d,%d) want (2,%d)", target, targetType, demoTraceThingType(3001))
	}
}

func TestDemoTraceMonsterPainStateTicsMatchesCurrentFrame(t *testing.T) {
	tests := []struct {
		typ       int16
		remaining int
		want      int
	}{
		{9, 5, 2},
		{9, 3, 3},
		{9, 1, 1},
		{3001, 3, 1},
		{3004, 5, 2},
	}
	for _, tt := range tests {
		got, ok := demoTraceMonsterPainStateTics(tt.typ, tt.remaining)
		if !ok {
			t.Fatalf("type %d remaining %d: helper returned !ok", tt.typ, tt.remaining)
		}
		if got != tt.want {
			t.Fatalf("type %d remaining %d: tics=%d want=%d", tt.typ, tt.remaining, got, tt.want)
		}
	}
}

func TestDemoTraceDoorSpecialKeepsZeroValuedFields(t *testing.T) {
	g := &game{
		doors: map[int]*doorThinker{
			71: {
				sector:       71,
				typ:          doorNormal,
				direction:    1,
				topHeight:    4456448,
				topWait:      150,
				topCountdown: 0,
				speed:        131072,
			},
		},
	}

	specials := g.demoTraceSpecials()
	if got, want := len(specials), 1; got != want {
		t.Fatalf("special count=%d want=%d", got, want)
	}
	if got, ok := specials[0]["type"]; !ok || got != int(doorNormal) {
		t.Fatalf("special type=%v ok=%v want=%d", got, ok, int(doorNormal))
	}
	if got, ok := specials[0]["topcountdown"]; !ok || got != 0 {
		t.Fatalf("topcountdown=%v ok=%v want=0", got, ok)
	}

	data, err := json.Marshal(specials)
	if err != nil {
		t.Fatalf("marshal specials: %v", err)
	}
	s := string(data)
	if !strings.Contains(s, `"type":0`) {
		t.Fatalf("marshaled specials missing type zero field: %s", s)
	}
	if !strings.Contains(s, `"topcountdown":0`) {
		t.Fatalf("marshaled specials missing topcountdown zero field: %s", s)
	}
}

func TestDemoTraceTicKeepsZeroValuedDoorFields(t *testing.T) {
	tracePath := t.TempDir() + "/door-trace.jsonl"
	base := mustLoadE1M1GameForMapTextureTests(t)
	g := newGame(base.m, Options{
		Width:   320,
		Height:  200,
		WADHash: "test-wad",
		DemoScript: &DemoScript{
			Path: "demo1",
			Header: DemoHeader{
				Version:      demoVersion109,
				Skill:        2,
				Episode:      1,
				Map:          1,
				PlayerInGame: [4]bool{true},
			},
			Tics: []DemoTic{{Forward: 0}},
		},
		DemoTracePath: tracePath,
	})
	g.doors = map[int]*doorThinker{
		71: {
			sector:       71,
			typ:          doorNormal,
			direction:    1,
			topHeight:    4456448,
			topWait:      150,
			topCountdown: 0,
			speed:        131072,
		},
	}

	g.writeDemoTraceTic()

	data, err := os.ReadFile(tracePath)
	if err != nil {
		t.Fatalf("read trace: %v", err)
	}
	s := strings.TrimSpace(string(data))
	if !strings.Contains(s, `"type":0`) {
		t.Fatalf("tic line missing type zero field: %s", s)
	}
	if !strings.Contains(s, `"topcountdown":0`) {
		t.Fatalf("tic line missing topcountdown zero field: %s", s)
	}
}

func TestDemoTraceMobjsFollowThinkerInsertionOrder(t *testing.T) {
	g := &game{
		m: &mapdata.Map{
			Things: []mapdata.Thing{
				{Type: 2001},
				{Type: 2007},
			},
		},
		thingCollected:    []bool{false, false},
		thingDropped:      []bool{false, true},
		thingThinkerOrder: []int64{1, 4},
		thingX:            []int64{10, 40},
		thingY:            []int64{0, 0},
		thingZState:       []int64{0, 0},
		thingFloorState:   []int64{0, 0},
		thingCeilState:    []int64{64 * fracUnit, 64 * fracUnit},
		hitscanPuffs: []hitscanPuff{
			{x: 20, y: 0, z: 0, tics: 4, state: 93, kind: hitscanFxPuff, order: 2},
			{x: 30, y: 0, z: 0, tics: 8, state: 92, kind: hitscanFxBlood, order: 3},
		},
	}

	mobjs := g.demoTraceMobjs()
	if got, want := len(mobjs), 5; got != want {
		t.Fatalf("mobj count=%d want=%d", got, want)
	}
	if got := mobjs[1].Type; got != 77 {
		t.Fatalf("mobjs[1].type=%d want=77", got)
	}
	if got := mobjs[2].Type; got != 37 {
		t.Fatalf("mobjs[2].type=%d want=37", got)
	}
	if got := mobjs[3].Type; got != 38 {
		t.Fatalf("mobjs[3].type=%d want=38", got)
	}
	if got := mobjs[4].Type; got != 63 {
		t.Fatalf("mobjs[4].type=%d want=63", got)
	}
}

func TestDemoTraceProjectileImpactRetainsMissileThinkerOrder(t *testing.T) {
	g := &game{
		projectileImpacts: []projectileImpact{
			{
				kind:      projectileFireball,
				x:         20,
				y:         0,
				z:         0,
				tics:      5,
				phaseTics: 5,
				order:     2,
			},
		},
		hitscanPuffs: []hitscanPuff{
			{x: 30, y: 0, z: 0, tics: 4, state: 93, kind: hitscanFxPuff, order: 3},
			{x: 40, y: 0, z: 0, tics: 8, state: 92, kind: hitscanFxBlood, order: 4},
		},
		m: &mapdata.Map{
			Things: []mapdata.Thing{
				{Type: 2001},
			},
		},
		thingCollected:    []bool{false},
		thingThinkerOrder: []int64{5},
		thingX:            []int64{50},
		thingY:            []int64{0},
		thingZState:       []int64{0},
		thingFloorState:   []int64{0},
		thingCeilState:    []int64{64 * fracUnit},
	}

	mobjs := g.demoTraceMobjs()
	if got, want := len(mobjs), 5; got != want {
		t.Fatalf("mobj count=%d want=%d", got, want)
	}
	if got := mobjs[1].Type; got != 31 {
		t.Fatalf("mobjs[1].type=%d want=31", got)
	}
	if got := mobjs[2].Type; got != 37 {
		t.Fatalf("mobjs[2].type=%d want=37", got)
	}
	if got := mobjs[3].Type; got != 38 {
		t.Fatalf("mobjs[3].type=%d want=38", got)
	}
	if got := mobjs[4].Type; got != 77 {
		t.Fatalf("mobjs[4].type=%d want=77", got)
	}
}
