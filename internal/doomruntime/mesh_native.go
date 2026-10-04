package doomruntime

import (
	"fmt"
	"image/color"
	"math"

	"gddoom/internal/mapdata"
	"gddoom/internal/render/hud"
	"gddoom/internal/render/levelmesh"
	"gddoom/internal/render/mapview"
)

// NativeMeshGame lets an alternate window/render backend drive the existing
// Doom simulation without calling Ebiten's input, drawing or window loop.
// Gameplay remains shared; this adapter exposes a single level to the host.
type NativeMeshGame struct {
	g             *game
	lightRamp     [32]float32
	lightRows     int
	sprites       []levelmesh.Sprite
	hud           []levelmesh.Patch
	weapon        []levelmesh.Patch
	hudState      statusBarCacheState
	haveHUD       bool
	mapPixels     []byte
	mapSegments   []mapview.Segment
	mapPatches    []levelmesh.Patch
	mapLabels     []NativeMapLabel
	fixedTextures map[nativeFixedTextureKey][]byte
	fuzzSpans     []levelmesh.FuzzSpan
	fuzzColors    map[int]nativeFuzzPalette
	fuzzRemaps    map[int]levelmesh.Texture
}

type NativeMeshInput struct {
	Forward, Side, Turn int     // Directions; side may reach +/-2 with a strafe modifier.
	YawDelta            float64 // Radians, positive counterclockwise.
	Run, Use, Fire      bool
	Map                 *NativeMapInput
	WeaponCycle         int
	WeaponSlot          int
}

type NativeMeshFrame struct {
	Triangles               []levelmesh.Triangle // Valid until the next Frame call.
	Camera                  levelmesh.Camera
	WorldTic                int
	Health, Armor           int
	Ammo                    int
	Weapon                  string
	Exited, Dead            bool
	Fullbright              bool // Active light-amplification powerup, including its blink.
	FixedColormap           bool // Invulnerability powerup's inverse palette, including its blink.
	Sprites                 []levelmesh.Sprite
	HUD, WeaponPatches      []levelmesh.Patch
	HUDMode                 int // Shared status-bar display mode: bottom, overlay, hidden.
	HUDScale                float64
	Sky                     levelmesh.Texture
	Message                 string
	DamageFlash, BonusFlash int
	FlashOverlay            color.RGBA // Shared status counters and powerup palette priority.
}

func NewNativeMeshGame(m *mapdata.Map, opts Options) *NativeMeshGame {
	return newNativeMeshGame(m, opts, true)
}
func newNativeMeshGame(m *mapdata.Map, opts Options, resetRNG bool) *NativeMeshGame {
	opts.MeshRenderer = "textured"
	opts.SourcePortMode = true
	opts.SFXVolume = 0 // The host attaches native audio without initializing Ebiten audio.
	g := newGameWithRNG(m, opts, resetRNG)
	g.syncRenderState()
	n := &NativeMeshGame{g: g, lightRows: doomShadeRows()}
	for row := range n.lightRamp {
		// Snapshot the shared source-port policy, including its integer ramp
		// fallback when a WAD includes extra inverse/fullbright colormap rows.
		n.lightRamp[row] = float32(doomShadeMulFromRow(row)) / 256
	}
	return n
}

// Tick advances exactly one Doom tic (35 Hz). The external host owns timing.
func (n *NativeMeshGame) Tick(in NativeMeshInput) {
	g := n.g
	if g.chatComposeOpen {
		in = NativeMeshInput{Map: in.Map}
	}
	if g.levelExitRequested {
		return
	}
	g.capturePrevState()
	var mapUpdate mapview.UpdateResult
	if in.Map != nil {
		mapUpdate = n.updateMap(*in.Map)
	}
	speed := 0
	if in.Run {
		speed = 1
	}
	cmd := moveCmd{
		forward: int64(max(-1, min(1, in.Forward))) * forwardMove[speed],
		side:    int64(max(-2, min(2, in.Side))) * sideMove[speed],
		turn:    max(-1, min(1, in.Turn)),
		turnRaw: int64(in.YawDelta * (4294967296 / (2 * math.Pi))),
		run:     in.Run, weaponSlot: in.WeaponSlot,
	}
	if in.WeaponCycle != 0 {
		g.cycleWeapon(in.WeaponCycle)
	}
	if in.WeaponSlot != 0 {
		g.demoWeaponSlot = in.WeaponSlot
	}
	g.runLocalGameplayTic(cmd, in.Use, in.Fire)
	g.tickStatusWidgets()
	g.tickChatHistory()
	g.discoverLinesAroundPlayer()
	if in.Map == nil || mapUpdate.SyncCameraToPlayer {
		g.State.SetCamera(float64(g.p.x)/fracUnit, float64(g.p.y)/fracUnit)
	} else {
		g.State.Pan(mapUpdate.PanDX, mapUpdate.PanDY)
	}
	if g.useFlash > 0 && g.hudMessagesEnabled {
		g.useFlash--
	}
	if g.damageFlashTic > 0 {
		g.damageFlashTic--
	}
	if g.bonusFlashTic > 0 {
		g.bonusFlashTic--
	}
	g.tickDelayedSounds()
	g.tickDelayedSwitchReverts()
	g.flushSoundEvents()
}

func (n *NativeMeshGame) Frame(alpha float64) NativeMeshFrame {
	g := n.g
	alpha = math.Max(0, math.Min(1, alpha))
	g.renderAlpha = alpha
	g.renderPX = lerp(float64(g.prevPX)/fracUnit, float64(g.p.x)/fracUnit, alpha)
	g.renderPY = lerp(float64(g.prevPY)/fracUnit, float64(g.p.y)/fracUnit, alpha)
	g.renderAngle = g.renderCameraAngle(alpha)
	g.beginSourcePortSpectreFuzzFrame(alpha)
	r := g.ensureMeshExperiment()
	g.buildMeshExperimentGeometry(r)
	def := weaponInfo(g.inventory.ReadyWeapon)
	n.buildPresentation()
	var sky levelmesh.Texture
	if _, tex, ok := g.runtimeSkyTextureEntryForMap(g.m.Name); ok {
		sky = nativeTexture(tex)
	}
	message := ""
	_, fixedColormap := g.playerFixedColormapRow()
	if g.useFlash > 0 {
		message = g.useText
	}
	return NativeMeshFrame{
		Triangles: r.triangles,
		Camera:    levelmesh.Camera{X: g.renderPX, Y: g.renderPY, Z: g.playerEyeZ(), Yaw: angleToRadians(g.renderAngle)},
		WorldTic:  g.worldTic, Health: g.stats.Health, Armor: g.stats.Armor,
		Ammo: weaponAmmoCount(g.stats, def.ammo), Weapon: def.name,
		Exited: g.levelExitRequested, Dead: g.isDead,
		Fullbright:    g.playerInfraredBright(),
		FixedColormap: fixedColormap,
		Sprites:       n.sprites, HUD: n.hud, WeaponPatches: n.weapon, Sky: sky,
		HUDMode: n.HUDMode(), HUDScale: g.hudScaleValue(),
		Message: message, DamageFlash: g.damageFlashTic, BonusFlash: g.bonusFlashTic,
		FlashOverlay: hud.FlashOverlayColor(g.statusDamageCount, g.statusBonusCount, g.inventory.StrengthCount, g.inventory.RadSuitTics),
	}
}

func (n *NativeMeshGame) LightRamp() [32]float32 { return n.lightRamp }
func (n *NativeMeshGame) LightRows() int         { return n.lightRows }

func (n *NativeMeshGame) Texture(t levelmesh.Triangle) levelmesh.Texture {
	return n.fixedWorldTexture(n.g.meshMaterial(n.g.ensureMeshExperiment(), t))
}

func (n *NativeMeshGame) Light(sector int) float64 {
	g := n.g
	if sector < 0 || sector >= len(g.m.Sectors) {
		return 1
	}
	return float64(sectorLightMul(g.sectorLightForRender(sector, &g.m.Sectors[sector]))) / 256
}

// SetPose is an inspection aid; normal movement still uses Doom collision.
func (n *NativeMeshGame) SetPose(x, y, eye, yaw float64) error {
	for _, v := range []float64{x, y, eye, yaw} {
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return fmt.Errorf("camera coordinates must be finite")
		}
	}
	if math.Abs(x) > 32767 || math.Abs(y) > 32767 || math.Abs(eye) > 32767 {
		return fmt.Errorf("camera is outside Doom coordinate range")
	}
	g := n.g
	g.setPlayerPosFixed(int64(math.Round(x*fracUnit)), int64(math.Round(y*fracUnit)))
	if floor, ceil, ok := g.subsectorFloorCeilAt(g.p.x, g.p.y); ok {
		g.p.floorz, g.p.ceilz = floor, ceil
	}
	g.p.z = int64(math.Round((eye - 41) * fracUnit))
	g.p.angle = uint32(int64(math.Remainder(yaw, 2*math.Pi) * 4294967296 / (2 * math.Pi)))
	g.playerViewZ = int64(math.Round(eye * fracUnit))
	g.p.momx, g.p.momy, g.p.momz = 0, 0, 0
	g.p.viewHeight, g.p.deltaViewHeight = playerViewHeight, 0
	g.State.SetCamera(x, y)
	g.discoverLinesAroundPlayer()
	g.syncRenderState()
	return nil
}

func (n *NativeMeshGame) IsDead() bool { return n.g.isDead }
