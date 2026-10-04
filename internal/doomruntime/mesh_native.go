package doomruntime

import (
	"fmt"
	"math"

	"gddoom/internal/mapdata"
	"gddoom/internal/render/levelmesh"
)

// NativeMeshGame lets an alternate window/render backend drive the existing
// Doom simulation without calling Ebiten's input, drawing or window loop.
// It is a single-level geometry experiment, not a second gameplay engine.
type NativeMeshGame struct{ g *game }

type NativeMeshInput struct {
	Forward, Side, Turn int     // -1, 0, +1; positive side/turn mean right/left.
	YawDelta            float64 // Radians, positive counterclockwise.
	Run, Use, Fire      bool
	WeaponSlot          int
}

type NativeMeshFrame struct {
	Triangles     []levelmesh.Triangle // Valid until the next Frame call.
	Camera        levelmesh.Camera
	WorldTic      int
	Health, Armor int
	Ammo          int
	Weapon        string
	Exited, Dead  bool
}

func NewNativeMeshGame(m *mapdata.Map, opts Options) *NativeMeshGame {
	opts.MeshRenderer = "textured"
	opts.SourcePortMode = true
	opts.SFXVolume = 0 // This geometry host does not initialize an audio backend.
	g := newGame(m, opts)
	g.syncRenderState()
	return &NativeMeshGame{g: g}
}

// Tick advances exactly one Doom tic (35 Hz). The external host owns timing.
func (n *NativeMeshGame) Tick(in NativeMeshInput) {
	g := n.g
	if g.levelExitRequested {
		return
	}
	g.capturePrevState()
	speed := 0
	if in.Run {
		speed = 1
	}
	cmd := moveCmd{
		forward: int64(max(-1, min(1, in.Forward))) * forwardMove[speed],
		side:    int64(max(-1, min(1, in.Side))) * sideMove[speed],
		turn:    max(-1, min(1, in.Turn)),
		turnRaw: int64(in.YawDelta * (4294967296 / (2 * math.Pi))),
		run:     in.Run, weaponSlot: in.WeaponSlot,
	}
	g.runGameplayTic(cmd, in.Use, in.Fire)
	g.tickStatusWidgets()
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
	r := g.ensureMeshExperiment()
	g.buildMeshExperimentGeometry(r)
	def := weaponInfo(g.inventory.ReadyWeapon)
	return NativeMeshFrame{
		Triangles: r.triangles,
		Camera:    levelmesh.Camera{X: g.renderPX, Y: g.renderPY, Z: g.playerEyeZ(), Yaw: angleToRadians(g.renderAngle)},
		WorldTic:  g.worldTic, Health: g.stats.Health, Armor: g.stats.Armor,
		Ammo: weaponAmmoCount(g.stats, def.ammo), Weapon: def.name,
		Exited: g.levelExitRequested, Dead: g.isDead,
	}
}

func (n *NativeMeshGame) Texture(t levelmesh.Triangle) levelmesh.Texture {
	return n.g.meshMaterial(n.g.ensureMeshExperiment(), t)
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
	g.syncRenderState()
	return nil
}
