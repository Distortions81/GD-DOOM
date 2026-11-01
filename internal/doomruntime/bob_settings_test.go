package doomruntime

import (
	"math"
	"reflect"
	"testing"

	"gddoom/internal/gameplay"
	"gddoom/internal/mapdata"
	"gddoom/internal/sessionflow"
)

func TestBobPreferencesPersistAndRemainModernOnly(t *testing.T) {
	g := bobTestGame(4, 8*fracUnit)
	g.inventory.ReadyWeapon = weaponPistol
	g.weaponActionReady(weaponStatePistolReady)
	g.capturePrevState()
	g.renderAlpha = 1
	eye, body, y, pose := g.playerViewZ, g.p, g.weaponPSpriteY, g.weaponPose
	g.toggleBobOption(8)
	g.toggleBobOption(9)
	if got := g.renderCameraEyeZ(); got != g.cameraBob.base {
		t.Fatalf("disabled camera bob=%v want base %v", got, g.cameraBob.base)
	}
	x, wy := g.weaponPresentationPosition()
	if x != 1 || math.Abs(wy-weaponTopY) > 1e-9 {
		t.Fatalf("disabled weapon bob=%v,%v", x, wy)
	}
	if g.playerViewZ != eye || g.p != body || g.weaponPSpriteY != y || g.weaponPose != pose {
		t.Fatal("presentation preference changed simulation")
	}
	sg := &sessionGame{g: g}
	sg.capturePersistentSettings()
	sg.applyPersistentSettingsToOptions()
	replacement := &game{opts: Options{SourcePortMode: true}}
	sg.applyPersistentSettingsToGame(replacement)
	if !sg.opts.DisableCameraBob || !sg.opts.DisableWeaponBob || !replacement.opts.DisableCameraBob || !replacement.opts.DisableWeaponBob {
		t.Fatal("map transition lost bob preferences")
	}
	applied := gameplay.ApplyRuntimeSettings(gameplay.PersistentSettings{}, g.runtimeSettingsSnapshot(), true, 2, 4, 5, 1).Settings
	if !applied.DisableCameraBob || !applied.DisableWeaponBob {
		t.Fatal("runtime settings lost preferences")
	}
	g.opts.SourcePortMode = false
	g.toggleBobOption(8)
	g.toggleBobOption(9)
	if !g.opts.DisableCameraBob || !g.opts.DisableWeaponBob || g.renderCameraEyeZ() != g.playerBaseEyeZ() {
		t.Fatal("Faithful mode used or changed Modern preferences")
	}
	if len(frontendOptionRows(false)) != 8 || len(frontendOptionRows(true)) != 10 {
		t.Fatal("incorrect menu rows")
	}
}

func TestBobOptionsRespondToAllMenuInputs(t *testing.T) {
	for _, row := range []int{8, 9} {
		for _, input := range []sessionflow.FrontendInput{{Left: true}, {Right: true}, {Select: true}} {
			result := sessionflow.StepFrontend(sessionflow.Frontend{Active: true, MenuActive: true, Mode: sessionflow.FrontendModeOptions, OptionsOn: row}, input, sessionflow.FrontendConfig{OptionRows: frontendOptionRows(true)})
			if result.ChangeCameraBob != (row == 8) || result.ChangeWeaponBob != (row == 9) {
				t.Fatalf("row %d input %+v result %+v", row, input, result)
			}
		}
	}
	g := &game{opts: Options{SourcePortMode: true}, pauseMenuOptionsOn: 8}
	g.adjustPauseOption(1)
	g.pauseMenuOptionsOn = 9
	g.cyclePauseOption()
	if !g.opts.DisableCameraBob || !g.opts.DisableWeaponBob {
		t.Fatal("pause menu did not change both preferences")
	}
}

func TestWeaponPoseFreezesDuringAttackAndDisablesOnlySway(t *testing.T) {
	g := &game{opts: Options{SourcePortMode: true}, worldTic: 17, renderAlpha: 1, inventory: playerInventory{ReadyWeapon: weaponPistol}}
	g.p.momx = 5 * fracUnit
	g.p.momy = 2 * fracUnit
	g.weaponActionReady(weaponStatePistolReady)
	g.capturePrevState()
	pose, y := g.weaponPose, g.weaponPSpriteY
	g.setWeaponPSpriteState(weaponStatePistolAtk1, false)
	g.p.momx, g.p.momy = 0, 0
	g.worldTic += 30
	if g.weaponPose != pose || g.weaponPSpriteY != y {
		t.Fatal("attack moved stored pose")
	}
	g.opts.DisableWeaponBob = true
	g.weaponActionLower(weaponStatePistolDown)
	x, wy := g.weaponPresentationPosition()
	if x != 1 || math.Abs(wy-(weaponTopY+weaponLowerSpeed)) > 1e-9 {
		t.Fatalf("toggle removed lowering motion: %v,%v", x, wy)
	}
	g.isDead = true
	g.setWeaponPSpriteState(weaponStatePistolDown, false)
	for range 100 {
		g.tickWeaponOverlay()
	}
	if g.weaponPSpriteY != weaponBottomY || g.weaponState != weaponStatePistolDown {
		t.Fatal("death weapon came back up")
	}
}

func TestWeaponPoseSnapshotAndPreviousVersionCompatibility(t *testing.T) {
	pose := weaponPresentationPose{X: -12345, FractionY: 34567, BobY: 4*fracUnit + 34567, Valid: true}
	for _, item := range []struct {
		magic   []byte
		version int
	}{{saveGameMagic, saveGameVersion}, {keyframeMagic, keyframeVersion}} {
		for _, version := range []int{item.version, item.version - 1, item.version - 2, item.version - 3} {
			file := saveFile{Version: version, Game: gameSaveState{WeaponPose: pose}}
			data, err := encodeSnapshot(item.magic, file)
			if err != nil {
				t.Fatal(err)
			}
			got, err := (&sessionGame{}).loadSnapshot(data, item.magic, item.version)
			if err != nil {
				t.Fatal(err)
			}
			want := pose
			if version == 26 || version == 13 {
				want = weaponPresentationPose{}
			}
			if !reflect.DeepEqual(got.Game.WeaponPose, want) {
				t.Fatalf("version %d pose=%+v want %+v", version, got.Game.WeaponPose, want)
			}
		}
	}
}

func TestWeaponLightingSeparatesGunAndFlash(t *testing.T) {
	palette := make([]byte, 1024)
	cm := make([]byte, 33*256)
	for i := 0; i < 256; i++ {
		palette[i*4], palette[i*4+3] = byte(i), 255
	}
	for row := 0; row < 33; row++ {
		for i := 0; i < 256; i++ {
			cm[row*256+i] = byte(i + row)
		}
	}
	g := &game{opts: Options{SourcePortMode: true, SourcePortSectorLighting: true, DoomPaletteRGBA: palette, DoomColorMap: cm, SpritePatchBank: map[string]WallTexture{
		"PISGA0": {Width: 2, Height: 1, Indexed: []byte{7, 7}, RGBA: []byte{7, 0, 0, 255, 7, 0, 0, 255}},
		"PISFA0": {Width: 1, Height: 1, OffsetX: -1, Indexed: []byte{9}, RGBA: []byte{9, 0, 0, 255}},
	}}, m: &mapdata.Map{Sectors: []mapdata.Sector{{Light: 128}}}, viewW: 320, viewH: 200, screenBlocks: 11, weaponFlashState: weaponStatePistolFlash}
	token := g.weaponCompositeToken("PISGA0", "PISFA0")
	_, tex, ok := g.resolveSpritePatchTexture(token)
	if !ok || tex.RGBA[0] != 8 || tex.RGBA[4] != 9 {
		t.Fatalf("gun/flash light = %v (%s)", tex.RGBA, token)
	}
	g.opts.DisableDoomLighting = true
	_, tex, ok = g.resolveSpritePatchTexture(g.weaponCompositeToken("PISGA0", "PISFA0"))
	if !ok || tex.RGBA[0] != 7 || tex.RGBA[4] != 9 {
		t.Fatal("lighting-disabled weapon was still shaded")
	}
}
