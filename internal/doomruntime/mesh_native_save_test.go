package doomruntime

import (
	"errors"
	"os"
	"reflect"
	"strings"
	"testing"

	"gddoom/internal/doomrand"
	"gddoom/internal/mapdata"
	"gddoom/internal/runtimecfg"
	"gddoom/internal/wad"
)

func nativeSaveFixture(t *testing.T) *NativeCampaign {
	t.Helper()
	c := nativeCampaignFixture(t, "E1M1")
	wf, err := wad.Open(findDOOM1WAD(t))
	if err != nil {
		t.Fatal(err)
	}
	c.session.opts.NewGameLoader = func(name string) (*mapdata.Map, error) { return mapdata.LoadMap(wf, mapdata.MapName(name)) }
	c.session.opts.WADSources = []runtimecfg.WADSource{{Name: "DOOM1.WAD", Hash: "same-test-wad"}}
	return c
}

func TestNativeSaveInteroperatesWithMainAndContinuesSimulation(t *testing.T) {
	c := nativeSaveFixture(t)
	c.SetControls(true, 1.7)
	for i := 0; i < 40; i++ {
		c.Game.Tick(NativeMeshInput{Forward: 1, Fire: i > 30})
	}
	g := c.Game.g
	g.stats.Health, g.stats.Armor, g.stats.Shells = 73, 40, 19
	g.inventory.Weapons[2001], g.inventory.BlueKey = true, true
	g.m.Sectors[0].Light, g.m.Sectors[0].FloorPic = 111, "NUKAGE1"
	g.m.Sidedefs[0].Mid = "STARTAN3"
	c.Game.SetMapActive(true)
	g.showGrid, g.parity.iddt = true, 2
	data, err := c.SaveData("native to main")
	if err != nil {
		t.Fatal(err)
	}
	main := &sessionGame{opts: c.session.opts}
	if err := main.unmarshalSaveGame(data); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(captureLiveRuntimeRoundTripState(main.g), captureLiveRuntimeRoundTripState(g)) {
		t.Fatal("native save changed runtime state when loaded by main")
	}
	if !main.g.alwaysRun || main.g.mode != viewMap || main.g.parity.iddt != 2 || !main.g.showGrid {
		t.Fatal("main lost saved settings/map state")
	}
	mainData, err := main.marshalSaveGame("main to native")
	if err != nil {
		t.Fatal(err)
	}
	c.Game.Tick(NativeMeshInput{Forward: -1})
	if err := c.LoadData(mainData); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(captureLiveRuntimeRoundTripState(c.Game.g), captureLiveRuntimeRoundTripState(main.g)) {
		t.Fatal("main save changed runtime state when loaded by native")
	}
	if c.Game.g.statusFacePatchName() != main.g.statusFacePatchName() {
		t.Fatal("loaded HUD face differs from main")
	}
	if !c.Game.AlwaysRun() || !c.Game.MapActive() || c.TakeMusicRequest() != "D_E1M1" {
		t.Fatal("native load did not restore controls/map/music")
	}
	// Alternate equal command tics, restoring the shared Doom RNG before each host.
	for i := 0; i < 70; i++ {
		menuRNG, playRNG := doomrand.State()
		c.Game.Tick(NativeMeshInput{Forward: 1, Run: true, Fire: i%7 == 0})
		afterMenu, afterPlay := doomrand.State()
		doomrand.SetState(menuRNG, playRNG)
		main.g.capturePrevState()
		buttons := uint8(0)
		if i%7 == 0 {
			buttons = demoButtonAttack
		}
		main.g.stepGameplayFromDemoTic(DemoTic{Forward: int8(forwardMove[1]), Buttons: buttons})
		if main.g.SimChecksum() != c.Game.g.SimChecksum() {
			t.Fatalf("save continuation diverged at tic %d", i)
		}
		gotMenu, gotPlay := doomrand.State()
		if gotMenu != afterMenu || gotPlay != afterPlay {
			t.Fatalf("random sequence diverged at tic %d", i)
		}
	}
}

func TestNativeLoadFailuresPreserveLiveCampaign(t *testing.T) {
	c := nativeSaveFixture(t)
	data, err := c.SaveData("failure tests")
	if err != nil {
		t.Fatal(err)
	}
	original, checksum := c.Game, c.Game.g.SimChecksum()
	originalRNG, originalPlayRNG := doomrand.State()
	broken := append([]byte(nil), data...)
	broken[len(broken)-1] ^= 1
	if err := c.LoadData(broken); err == nil {
		t.Fatal("corrupt save accepted")
	}
	c.session.opts.NewGameLoader = func(string) (*mapdata.Map, error) { return nil, errors.New("map unavailable") }
	if err := c.LoadData(data); err == nil {
		t.Fatal("missing map accepted")
	}
	gotRNG, gotPlayRNG := doomrand.State()
	if c.Game != original || c.Game.g.SimChecksum() != checksum || gotRNG != originalRNG || gotPlayRNG != originalPlayRNG {
		t.Fatal("failed load mutated live campaign")
	}
}

func TestNativeSaveSlotsUseSharedStorageAndRejectIntermission(t *testing.T) {
	c := nativeSaveFixture(t)
	t.Chdir(t.TempDir())
	if err := c.LoadSlot(0); !errors.Is(err, errNoSavedGame) {
		t.Fatalf("empty load=%v", err)
	}
	for _, slot := range []int{0, 2, 12} {
		if err := c.SaveSlot(slot); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := os.Stat(saveGamePath(0)); err != nil {
		t.Fatal(err)
	}
	slots := c.SaveSlots(true)
	ids := []int{}
	for _, slot := range slots {
		ids = append(ids, slot.Slot)
	}
	if !reflect.DeepEqual(ids, []int{0, 2, 12, 13}) || !slots[1].Present || slots[1].Current != "E1M1" || slots[3].Present {
		t.Fatalf("slots=%+v", slots)
	}
	if err := c.LoadSlot(2); err != nil {
		t.Fatal(err)
	}
	c.Game.g.requestLevelExit(false, "exit")
	if err := c.Tick(NativeMeshInput{}, false); err != nil {
		t.Fatal(err)
	}
	if _, err := c.SaveData("intermission"); !errors.Is(err, errSaveGameUnavailable) {
		t.Fatalf("intermission save=%v", err)
	}
}

func TestNativeLoadWarnsForOtherWADAndRestoresSessionOptions(t *testing.T) {
	c := nativeSaveFixture(t)
	c.Game.g.opts.SkillLevel = 2
	c.Game.g.opts.NoMonsters = true
	data, err := c.SaveData("settings")
	if err != nil {
		t.Fatal(err)
	}
	c.session.opts.SkillLevel = 5
	c.session.opts.NoMonsters = false
	c.session.opts.WADSources[0].Hash = "different-wad"
	c.complete = true
	c.session.finale.Active = true
	if err := c.LoadData(data); err != nil {
		t.Fatal(err)
	}
	if c.Phase() != NativeCampaignPlaying || c.session.opts.SkillLevel != 2 || !c.session.opts.NoMonsters || c.Game.SkillLevel() != 2 {
		t.Fatal("load retained old session or campaign stage")
	}
	if !strings.HasPrefix(c.Game.Frame(1).Message, "WAD WARNING:") {
		t.Fatal("different WAD did not warn")
	}
	fresh, err := c.session.opts.NewGameLoader("E1M1")
	if err != nil {
		t.Fatal(err)
	}
	if err := c.Restart(fresh); err != nil {
		t.Fatal(err)
	}
	if c.Game.SkillLevel() != 2 || !c.Game.g.opts.NoMonsters {
		t.Fatal("restart lost loaded session settings")
	}
}
