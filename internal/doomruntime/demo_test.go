package doomruntime

import (
	"errors"
	"os"
	"testing"

	"gddoom/internal/mapdata"

	"github.com/hajimehoshi/ebiten/v2"
)

func TestParseDemoScriptLMP(t *testing.T) {
	src := []byte{
		demoVersion110, 2, 1, 3, 0, 0, 1, 0, 0,
		1, 0, 0, 0,
		25, 0, 0, demoButtonUse,
		206, 17, 0x80, demoButtonAttack,
		demoMarker,
	}
	d, err := ParseDemoScript(src)
	if err != nil {
		t.Fatalf("parse demo: %v", err)
	}
	if d.Header.Version != demoVersion110 || d.Header.Skill != 2 || d.Header.Episode != 1 || d.Header.Map != 3 || !d.Header.Fast {
		t.Fatalf("unexpected header: %+v", d.Header)
	}
	if got := len(d.Tics); got != 2 {
		t.Fatalf("tic count=%d want=2", got)
	}
	if d.Tics[0].Forward != 25 || d.Tics[0].Buttons != demoButtonUse {
		t.Fatalf("unexpected tic[0]: %+v", d.Tics[0])
	}
	if d.Tics[1].Forward != -50 || d.Tics[1].Side != 17 || d.Tics[1].AngleTurn != -32768 || d.Tics[1].Buttons != demoButtonAttack {
		t.Fatalf("unexpected tic[1]: %+v", d.Tics[1])
	}
}

func TestParseDemoScriptAcceptsVersion109(t *testing.T) {
	src := []byte{
		demoVersion109, 2, 1, 3, 0, 0, 1, 0, 0,
		1, 0, 0, 0,
		25, 0, 0, demoButtonUse,
		demoMarker,
	}
	d, err := ParseDemoScript(src)
	if err != nil {
		t.Fatalf("parse demo: %v", err)
	}
	if d.Header.Version != demoVersion109 {
		t.Fatalf("version=%d want=%d", d.Header.Version, demoVersion109)
	}
}

func TestParseDemoScriptParsesAllHeaderFields(t *testing.T) {
	src := []byte{
		demoVersion110, 4, 3, 7, 1, 1, 1, 1, 2,
		1, 1, 0, 1,
		25, 0, 0, 0,
		demoMarker,
	}
	d, err := ParseDemoScript(src)
	if err != nil {
		t.Fatalf("parse demo: %v", err)
	}
	want := DemoHeader{
		Version:       demoVersion110,
		Skill:         4,
		Episode:       3,
		Map:           7,
		Deathmatch:    true,
		Respawn:       true,
		Fast:          true,
		NoMonsters:    true,
		ConsolePlayer: 2,
		PlayerInGame:  [4]bool{true, true, false, true},
	}
	if d.Header != want {
		t.Fatalf("header=%+v want %+v", d.Header, want)
	}
}

func TestDemoButtonWeaponSlotMatchesDoomButtonPacking(t *testing.T) {
	if got := demoButtonWeaponSlot(demoButtonChange | (2 << demoButtonWeaponShift)); got != 3 {
		t.Fatalf("weapon slot=%d want=3", got)
	}
	if got := demoButtonWeaponSlot(demoButtonAttack | demoButtonUse); got != 0 {
		t.Fatalf("weapon slot without change bit=%d want=0", got)
	}
}

func TestParseDemoScriptErrors(t *testing.T) {
	cases := [][]byte{
		nil,
		{demoVersion110},
		{109, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, demoMarker},
		{demoVersion110, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
		{demoVersion110, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1, 2},
	}
	for _, tc := range cases {
		if _, err := ParseDemoScript(tc); err == nil {
			t.Fatalf("expected parse error for %v", tc)
		}
	}
}

func TestFormatDemoScriptRoundTrip(t *testing.T) {
	in := &DemoScript{
		Header: DemoHeader{
			Version:       demoVersion110,
			Skill:         4,
			Episode:       1,
			Map:           1,
			ConsolePlayer: 0,
			PlayerInGame:  [4]bool{true, false, false, false},
		},
		Tics: []DemoTic{
			{Forward: 25, Side: 0, AngleTurn: 0, Buttons: demoButtonUse},
			{Forward: -50, Side: 17, AngleTurn: -32768, Buttons: demoButtonAttack},
		},
	}
	data, err := FormatDemoScript(in)
	if err != nil {
		t.Fatalf("format demo: %v", err)
	}
	out, err := ParseDemoScript(data)
	if err != nil {
		t.Fatalf("parse roundtrip: %v", err)
	}
	if out.Header != in.Header {
		t.Fatalf("header=%+v want %+v", out.Header, in.Header)
	}
	if len(out.Tics) != len(in.Tics) {
		t.Fatalf("tic count=%d want=%d", len(out.Tics), len(in.Tics))
	}
	for i := range in.Tics {
		if out.Tics[i] != in.Tics[i] {
			t.Fatalf("tic[%d]=%+v want %+v", i, out.Tics[i], in.Tics[i])
		}
	}
}

func TestBuildRecordedDemo(t *testing.T) {
	demo, err := BuildRecordedDemo("MAP01", Options{SkillLevel: 3, GameMode: "single"}, []DemoTic{{Forward: 25}})
	if err != nil {
		t.Fatalf("BuildRecordedDemo() error = %v", err)
	}
	if demo.Header.Version != demoVersion110 || demo.Header.Skill != 2 || demo.Header.Map != 1 || demo.Header.Episode != 0 {
		t.Fatalf("unexpected header: %+v", demo.Header)
	}
	if !demo.Header.PlayerInGame[0] {
		t.Fatalf("player 1 should be active: %+v", demo.Header)
	}
}

func TestNewGameDemoPlaybackIgnoresLaunchGameplayOverrides(t *testing.T) {
	g := newGame(&mapdata.Map{}, Options{
		SkillLevel:       5,
		GameMode:         gameModeDeathmatch,
		ShowNoSkillItems: true,
		ShowAllItems:     true,
		CheatLevel:       3,
		Invulnerable:     true,
		AllCheats:        true,
		DemoScript: &DemoScript{
			Header: DemoHeader{
				Version:       demoVersion110,
				Skill:         1,
				Respawn:       true,
				Fast:          true,
				NoMonsters:    true,
				ConsolePlayer: 0,
				PlayerInGame:  [4]bool{true},
			},
			Tics: []DemoTic{{Forward: 25}},
		},
	})
	if g == nil {
		t.Fatal("expected game")
	}
	if got := g.opts.SkillLevel; got != 2 {
		t.Fatalf("SkillLevel=%d want 2", got)
	}
	if got := g.opts.GameMode; got != gameModeSingle {
		t.Fatalf("GameMode=%q want %q", got, gameModeSingle)
	}
	if !g.opts.FastMonsters || !g.opts.RespawnMonsters || !g.opts.NoMonsters {
		t.Fatalf("demo header flags not applied: fast=%t respawn=%t nomonsters=%t", g.opts.FastMonsters, g.opts.RespawnMonsters, g.opts.NoMonsters)
	}
	if g.opts.ShowNoSkillItems || g.opts.ShowAllItems {
		t.Fatalf("demo playback inherited item filter overrides: shownoskill=%t showall=%t", g.opts.ShowNoSkillItems, g.opts.ShowAllItems)
	}
	if g.cheatLevel != 0 || g.invulnerable || g.opts.AllCheats {
		t.Fatalf("demo playback inherited cheats: cheat=%d invuln=%t allcheats=%t", g.cheatLevel, g.invulnerable, g.opts.AllCheats)
	}
}

func TestDemoUpdateModeTerminatesAtEnd(t *testing.T) {
	g := mustLoadE1M1GameForMapTextureTests(t)
	g.opts.DemoScript = &DemoScript{
		Header: DemoHeader{Version: demoVersion110, Skill: 2, Episode: 1, Map: 1, PlayerInGame: [4]bool{true}},
		Tics: []DemoTic{
			{Forward: 25},
			{Forward: 25},
		},
	}
	g.opts.DemoQuitOnComplete = true
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

func TestDemoUpdateModeTerminatesOnDeathWhenConfigured(t *testing.T) {
	g := mustLoadE1M1GameForMapTextureTests(t)
	g.opts.DemoScript = &DemoScript{
		Header: DemoHeader{Version: demoVersion110, Skill: 2, Episode: 1, Map: 1, PlayerInGame: [4]bool{true}},
		Tics: []DemoTic{
			{Forward: 25},
		},
	}
	g.opts.DemoQuitOnComplete = true
	g.opts.DemoExitOnDeath = true
	g.isDead = true
	err := g.Update()
	if !errors.Is(err, ebiten.Termination) {
		t.Fatalf("expected ebiten.Termination after player death, got %v", err)
	}
}

func TestDemoUpdateModeTerminatesAfterConfiguredTicLimit(t *testing.T) {
	g := mustLoadE1M1GameForMapTextureTests(t)
	g.opts.DemoScript = &DemoScript{
		Header: DemoHeader{Version: demoVersion110, Skill: 2, Episode: 1, Map: 1, PlayerInGame: [4]bool{true}},
		Tics: []DemoTic{
			{Forward: 25},
			{Forward: 25},
			{Forward: 25},
		},
	}
	g.opts.DemoQuitOnComplete = true
	g.opts.DemoStopAfterTics = 2
	startX := g.p.x
	startY := g.p.y
	if err := g.Update(); err != nil {
		t.Fatalf("update 1: %v", err)
	}
	if err := g.Update(); err != nil {
		t.Fatalf("update 2: %v", err)
	}
	if g.p.x == startX && g.p.y == startY {
		t.Fatal("expected demo movement before tic limit")
	}
	err := g.Update()
	if !errors.Is(err, ebiten.Termination) {
		t.Fatalf("expected ebiten.Termination after tic limit, got %v", err)
	}
	if got := g.demoTick; got != 2 {
		t.Fatalf("demoTick=%d want 2", got)
	}
}

func TestSessionGameRebuildPreservesRecordedDemoTics(t *testing.T) {
	sg := &sessionGame{
		opts: Options{RecordDemoPath: "out.lmp"},
		g: &game{
			opts:       Options{RecordDemoPath: "out.lmp"},
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

func TestFreezeDemoRecordFlushesAndStopsRecording(t *testing.T) {
	sg := &sessionGame{
		opts: Options{RecordDemoPath: "out.lmp"},
		g: &game{
			opts:       Options{RecordDemoPath: "out.lmp"},
			demoRecord: []DemoTic{{Forward: 10}, {Forward: 20}},
		},
	}
	sg.freezeDemoRecord()
	if sg.opts.RecordDemoPath != "" {
		t.Fatalf("RecordDemoPath on session should be cleared after freeze, got %q", sg.opts.RecordDemoPath)
	}
	if sg.g.opts.RecordDemoPath != "" {
		t.Fatalf("RecordDemoPath on game should be cleared after freeze, got %q", sg.g.opts.RecordDemoPath)
	}
	if got := len(sg.demoRecord); got != 2 {
		t.Fatalf("demoRecord len=%d want=2", got)
	}
	if got := len(sg.effectiveDemoRecord()); got != 2 {
		t.Fatalf("effective demo tics after freeze=%d want=2", got)
	}
}

func TestFreezeDemoRecordIsNoopWhenNotRecording(t *testing.T) {
	sg := &sessionGame{
		opts: Options{},
		g:    &game{},
	}
	sg.freezeDemoRecord() // must not panic
	if sg.opts.RecordDemoPath != "" {
		t.Fatalf("RecordDemoPath unexpectedly set")
	}
}

func TestRecordDemoTicEncodesWeaponSlot(t *testing.T) {
	g := minimalRecordingGame(t)
	g.demoWeaponSlot = 3 // player pressed key 3 (shotgun slot, 0-based index 2)
	g.recordDemoTic(moveCmd{}, false, false)
	if len(g.demoRecord) != 1 {
		t.Fatalf("expected 1 tic, got %d", len(g.demoRecord))
	}
	tc := g.demoRecord[0]
	if tc.Buttons&demoButtonChange == 0 {
		t.Fatalf("BT_CHANGE not set in buttons=0x%02x", tc.Buttons)
	}
	if got := demoButtonWeaponSlot(tc.Buttons); got != 3 {
		t.Fatalf("weapon slot=%d want=3", got)
	}
	// demoWeaponSlot must be consumed
	if g.demoWeaponSlot != 0 {
		t.Fatalf("demoWeaponSlot not cleared after recording")
	}
}

func TestRecordDemoTicClearsWeaponSlotEvenWhenNotRecording(t *testing.T) {
	g := &game{opts: Options{}} // no RecordDemoPath
	g.demoWeaponSlot = 2
	g.recordDemoTic(moveCmd{}, false, false)
	// slot is not consumed when not recording — that's fine; it will be
	// overwritten next tic. No assertion needed; just must not panic.
}

func TestRecordDemoTicUsesHeldFireBit(t *testing.T) {
	g := minimalRecordingGame(t)
	g.recordDemoTic(moveCmd{}, false, true)
	if len(g.demoRecord) != 1 {
		t.Fatalf("expected 1 tic, got %d", len(g.demoRecord))
	}
	if g.demoRecord[0].Buttons&demoButtonAttack == 0 {
		t.Fatalf("BT_ATTACK not set in buttons=0x%02x", g.demoRecord[0].Buttons)
	}
}

func TestRecordDemoTicUsesCommandAngleTurn(t *testing.T) {
	g := minimalRecordingGame(t)
	g.turnHeld = slowTurnTics
	cmd := moveCmd{turn: 1}
	g.recordDemoTic(cmd, false, false)
	if len(g.demoRecord) != 1 {
		t.Fatalf("expected 1 tic, got %d", len(g.demoRecord))
	}
	if got, want := g.demoRecord[0].AngleTurn, g.demoAngleTurn(cmd); got != want {
		t.Fatalf("AngleTurn=%d want %d", got, want)
	}
}

func minimalRecordingGame(t *testing.T) *game {
	t.Helper()
	g := newGame(&mapdata.Map{}, Options{RecordDemoPath: "out.lmp"})
	if g == nil {
		t.Fatal("newGame returned nil")
	}
	return g
}

func TestSaveDemoScriptRejectsNoTics(t *testing.T) {
	path := t.TempDir() + "/empty.lmp"
	err := SaveDemoScript(path, &DemoScript{})
	if err == nil || err.Error() != "demo has no tics" {
		t.Fatalf("expected no tics error, got %v", err)
	}
	if _, statErr := os.Stat(path); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("empty demo file should not be written, stat err=%v", statErr)
	}
}
