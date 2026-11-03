package doomruntime

import (
	"gddoom/internal/demo"
	"gddoom/internal/doomrand"
	"gddoom/internal/mapdata"
	"reflect"
	"testing"
)

func TestBoomRNGSaveAndKeyframeContinuation(t *testing.T) {
	for _, keyframe := range []bool{false, true} {
		m := predictionTestMap()
		m.Things = append(m.Things, mapdata.Thing{Type: 3001, X: 64})
		settings := demo.GameOptions{Seed: 0xfedcba98, Insurance: true, VariableFriction: false, AllowPushers: true, MonstersRemember: true, PlayerBobbing: false}
		opts := Options{Headless: true, BoomGameOptions: &settings, NewGameLoader: func(string) (*mapdata.Map, error) { return cloneMapForRestart(m), nil }}
		sg := &sessionGame{opts: opts, current: m.Name, g: newGame(cloneMapForRestart(m), opts)}
		sg.g.boomLastEnemy = []int{0, -1}
		for i := 0; i < 37; i++ {
			doomrand.PRandom(doomrand.Genlift)
			doomrand.PRandom(doomrand.Spawnthing)
			doomrand.MRandom()
			doomrand.AdvanceLogicTic()
		}
		before := doomrand.Capture()
		var data []byte
		var err error
		if keyframe {
			data, err = sg.marshalNetplayKeyframe()
		} else {
			data, err = sg.marshalSaveGame("Boom RNG")
		}
		if err != nil {
			t.Fatal(err)
		}
		var expected []int
		for i := 0; i < 128; i++ {
			expected = append(expected, doomrand.PRandom(doomrand.Class(i%doomrand.ClassCount)))
			doomrand.AdvanceLogicTic()
		}
		doomrand.Clear()
		if keyframe {
			err = sg.unmarshalNetplayKeyframe(data)
		} else {
			err = sg.unmarshalSaveGame(data)
		}
		if err != nil {
			t.Fatal(err)
		}
		if after := doomrand.Capture(); after != before {
			t.Fatal("loaded state lost seeds, indices, configuration or logic phase")
		}
		if !sg.g.boomMomentumDropoff || sg.g.opts.BoomGameOptions == nil || sg.g.opts.BoomGameOptions.Seed != settings.Seed || sg.g.opts.BoomGameOptions.PlayerBobbing || !reflect.DeepEqual(sg.g.boomLastEnemy, []int{0, -1}) {
			t.Fatal("loaded state lost Boom options or last enemy")
		}
		for i, v := range expected {
			if got := doomrand.PRandom(doomrand.Class(i % doomrand.ClassCount)); got != v {
				t.Fatalf("continuation diverged at %d", i)
			}
			doomrand.AdvanceLogicTic()
		}
	}
	doomrand.Clear()
}
func TestBoomLegacySaveDoesNotInventStreams(t *testing.T) {
	for _, version := range []int{35, 22} {
		f := saveFile{Version: version, RNG: saveRNGState{MenuIndex: 13, PlayIndex: 27}}
		magic := saveGameMagic
		if version == 22 {
			magic = keyframeMagic
		}
		data, err := encodeSnapshot(magic, f)
		if err != nil {
			t.Fatal(err)
		}
		decoded, err := decodeSnapshot(data, magic)
		if err != nil {
			t.Fatal(err)
		}
		doomrand.ConfigureBoom(1993, true)
		restoreSavedRNG(decoded)
		if s := doomrand.Capture(); s.Boom || s.MenuIndex != 13 || s.PlayIndex != 27 {
			t.Fatal("old snapshot invented seeded RNG state")
		}
	}
}
func TestBoomMonsterMemoryRetainsLivingPlayer(t *testing.T) {
	g := newGame(&mapdata.Map{Name: "MAP01", Things: []mapdata.Thing{{Type: 1}, {Type: 3001, X: 64}, {Type: 3004, X: 128}}, Sectors: []mapdata.Sector{{CeilingHeight: 128}}, Linedefs: []mapdata.Linedef{{Special: 227}}}, Options{Headless: true})
	g.setMonsterTargetPlayer(1)
	g.rememberBoomEnemy(1)
	g.setMonsterTargetThing(1, 2)
	if !g.recallBoomEnemy(1) || !g.thingTargetPlayer[1] || g.boomLastEnemy[1] != 0 {
		t.Fatal("did not recall living player")
	}
	g.opts.BoomGameOptions = &demo.GameOptions{MonstersRemember: false}
	g.boomLastEnemy[1] = -1
	if g.recallBoomEnemy(1) {
		t.Fatal("disabled memory recalled an enemy")
	}
}

func TestBoomOptionDisablesEnvironmentalForces(t *testing.T) {
	g := environmentFixtureGame(t, "MAP06")
	g.opts.BoomGameOptions = &demo.GameOptions{AllowPushers: false}
	for _, p := range g.boomPushers {
		g.tickBoomPusher(p)
	}
	if g.p.momx != 0 || g.p.momy != 0 {
		t.Fatal("disabled pushers affected momentum")
	}
	g.opts.BoomGameOptions.AllowPushers = true
	for _, p := range g.boomPushers {
		g.tickBoomPusher(p)
	}
	if g.p.momx == 0 && g.p.momy == 0 {
		t.Fatal("enabled pusher did not apply force")
	}
	g = environmentFixtureGame(t, "MAP01")
	g.opts.BoomGameOptions = &demo.GameOptions{VariableFriction: false}
	for _, f := range g.boomFrictions {
		g.tickBoomFriction(f)
	}
	if storedBoomFriction(g.p.friction) != friction || g.boomPlayerMoveFactor() != boomMoveFactor {
		t.Fatal("disabled friction changed movement")
	}
}
