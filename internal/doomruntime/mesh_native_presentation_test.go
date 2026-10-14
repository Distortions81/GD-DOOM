package doomruntime

import (
	"strings"
	"testing"

	"gddoom/internal/mapdata"
	"gddoom/internal/render/doomtex"
	"gddoom/internal/wad"
)

func loadNativeCombatGame(t *testing.T) *NativeMeshGame {
	t.Helper()
	fixture := loadMeshExperimentGame(t)
	wf, err := wad.Open(findDOOM1WAD(t))
	if err != nil {
		t.Fatal(err)
	}
	set, err := doomtex.LoadFromWAD(wf)
	if err != nil {
		t.Fatal(err)
	}
	opts := fixture.opts
	opts.NoMonsters = false
	opts.Invulnerable = false
	opts.SkillLevel = 3
	opts.SpritePatchBank = make(map[string]WallTexture)
	for _, lump := range wf.Lumps {
		if (len(lump.Name) != 6 && len(lump.Name) != 8) || lump.Name[4] < 'A' || lump.Name[4] > 'Z' || lump.Name[5] < '0' || lump.Name[5] > '8' {
			continue
		}
		rgba, w, h, ox, oy, err := set.BuildPatchRGBA(lump.Name, 0)
		if err != nil {
			continue
		}
		opts.SpritePatchBank[lump.Name] = WallTexture{RGBA: rgba, Width: w, Height: h, OffsetX: ox, OffsetY: oy}
	}
	m, err := mapdata.LoadMap(wf, "E1M1")
	if err != nil {
		t.Fatal(err)
	}
	return NewNativeMeshGame(m, opts)
}

func TestNativeMeshCombatPresentation(t *testing.T) {
	n := loadNativeCombatGame(t)
	for range 24 {
		n.Tick(NativeMeshInput{})
	}
	f := n.Frame(1)
	if len(f.Sprites) == 0 || len(f.WeaponPatches) == 0 || len(f.HUD) == 0 || f.Sky.Width == 0 {
		t.Fatal("native frame missing combat presentation")
	}
	monsters, items := 0, 0
	for _, s := range f.Sprites {
		if strings.HasPrefix(s.Name, "POSS") || strings.HasPrefix(s.Name, "SPOS") || strings.HasPrefix(s.Name, "TROO") {
			monsters++
		} else {
			items++
		}
	}
	if monsters == 0 || items == 0 {
		t.Fatalf("monsters/items missing: %d/%d", monsters, items)
	}
	before := n.g.SimChecksum()
	for _, alpha := range []float64{0, 0.25, 0.75, 1} {
		n.Frame(alpha)
	}
	if n.g.SimChecksum() != before {
		t.Fatal("sprite or HUD extraction changed simulation")
	}
	// Marking a pickup collected must remove the same patch next frame.
	for i, th := range n.g.m.Things {
		if n.g.thingCollected[i] || isMonster(th.Type) || isPlayerStart(th.Type) {
			continue
		}
		ref, ok := n.g.runtimeWorldThingSpriteRef(i, th, n.g.worldTic, 1)
		if !ok || ref == nil {
			continue
		}
		oldCount := len(n.Frame(1).Sprites)
		n.g.thingCollected[i] = true
		if len(n.Frame(1).Sprites) != oldCount-1 {
			t.Fatal("collected pickup still drawn")
		}
		break
	}
	var sounds []NativeSound
	n.SetSoundSink(func(s NativeSound) { sounds = append(sounds, s) })
	bullets := n.g.stats.Bullets
	for range 12 {
		n.Tick(NativeMeshInput{Fire: true})
	}
	if n.g.stats.Bullets >= bullets {
		t.Fatal("native fire did not consume ammunition")
	}
	pistol := false
	for _, s := range sounds {
		if s.Name == "DSPISTOL" {
			pistol = true
		}
	}
	if !pistol {
		t.Fatal("native firing did not emit its shared Doom sound")
	}
	n.g.isDead = true
	if len(n.Frame(1).WeaponPatches) != 0 {
		t.Fatal("dead player retained weapon overlay")
	}
	// The host advances the same overlay counters as the main update loop.
	n.g.damageFlashTic, n.g.bonusFlashTic, n.g.useFlash = 3, 3, 3
	n.Tick(NativeMeshInput{})
	if n.g.damageFlashTic != 2 || n.g.bonusFlashTic != 2 || n.g.useFlash != 2 {
		t.Fatal("native overlay timers did not decay")
	}
}

func TestNativeMeshSoundPanningAndClipping(t *testing.T) {
	n := loadNativeCombatGame(t)
	if err := n.SetPose(0, 0, 41, 0); err != nil {
		t.Fatal(err)
	}
	if v, p := n.SoundParams(NativeSound{}); v != 1 || p != 0 {
		t.Fatal("local sound not centered")
	}
	leftV, left := n.SoundParams(NativeSound{Positioned: true, Y: 300})
	rightV, right := n.SoundParams(NativeSound{Positioned: true, Y: -300})
	if leftV <= 0 || leftV != rightV || left >= 0 || right <= 0 {
		t.Fatalf("incorrect stereo orientation: %v/%v volume %v/%v", left, right, leftV, rightV)
	}
	if v, _ := n.SoundParams(NativeSound{Positioned: true, X: 2000}); v != 0 {
		t.Fatal("distant sound was not clipped")
	}
}

func TestNativeMeshCombatTicsMatchMainEngine(t *testing.T) {
	native := loadNativeCombatGame(t)
	for range 40 {
		native.Tick(NativeMeshInput{Forward: 1, Fire: true})
		native.Frame(0.5)
	}
	want := native.g.SimChecksum()
	// Construct again to reset Doom's global random streams to the same
	// post-spawn state, then drive the main engine's recorded-tic path.
	main := loadNativeCombatGame(t).g
	for range 40 {
		main.capturePrevState()
		main.stepGameplayFromDemoTic(DemoTic{Forward: int8(forwardMove[0]), Buttons: demoButtonAttack})
	}
	if got := main.SimChecksum(); got != want {
		t.Fatalf("native combat diverged from main engine: native=%x main=%x", want, got)
	}
}
