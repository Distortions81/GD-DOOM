package doomruntime

import (
	"fmt"
	"math"

	"gddoom/internal/render/levelmesh"
)

func nativeTexture(tex *WallTexture) levelmesh.Texture {
	if tex == nil {
		return levelmesh.Texture{}
	}
	return levelmesh.Texture{RGBA: tex.RGBA, Width: tex.Width, Height: tex.Height, Indexed: tex.Indexed}
}

func (n *NativeMeshGame) buildPresentation() {
	g := n.g
	n.sprites = n.sprites[:0]
	add := func(ref *spriteRenderRef, x, y, z int64, sector int, flip, floorAnchor, shadow bool) {
		if ref == nil || ref.tex == nil {
			return
		}
		tex := ref.tex
		if tex.Width <= 0 || tex.Height <= 0 {
			return
		}
		fy := 1.0
		// Projection already stretches geometry vertically by Doom's pixel
		// aspect. Circular pickup patches opt out of that stretch.
		if g.geometryAspectActive && ref.aspectExempt {
			fy = 1 / g.geometryAspectY
		}
		oy := float64(tex.OffsetY)
		if floorAnchor {
			oy = float64(tex.Height)
		}
		n.sprites = append(n.sprites, levelmesh.Sprite{
			Texture: n.fixedWorldTexture(nativeTexture(tex)), Name: ref.key,
			X: float64(x) / fracUnit, Y: float64(y) / fracUnit, Z: float64(z) / fracUnit,
			OffsetX: float64(tex.OffsetX), OffsetY: oy, ScaleY: fy,
			Light: n.Light(sector), Flip: flip, Fullbright: ref.fullBright, Shadow: shadow,
		})
	}
	tickUnits, unitsPerTic := g.worldThingAnimTickUnits()
	for i, th := range g.m.Things {
		if g.thingCollected[i] || isPlayerStart(th.Type) {
			continue
		}
		sector := g.thingSectorCached(i, th)
		if isMonster(th.Type) {
			if !g.monsterVisibleAfterDeath(i, th.Type) {
				continue
			}
			x, y, z := g.thingRenderPosFixed(i, th, g.renderAlpha)
			ref, flip, _ := g.monsterSpriteRefForView(i, th, g.worldTic, g.renderPX, g.renderPY)
			add(ref, x, y, z, sector, flip, false, monsterUsesShadow(th.Type))
		} else {
			x, y := g.thingPosFixed(i, th)
			ref, _ := g.runtimeWorldThingSpriteRef(i, th, tickUnits, unitsPerTic)
			add(ref, x, y, g.thingFloorZ(x, y), sector, false, true, false)
		}
	}
	for _, p := range g.projectiles {
		x, y, z := g.projectileRenderPosFixed(p, g.renderAlpha)
		ref, _ := g.projectileSpriteRef(p.kind, p.frame)
		add(ref, x, y, z, g.sectorAt(x, y), false, false, false)
	}
	for _, fx := range g.projectileImpacts {
		ref, _ := g.projectileImpactSpriteRef(fx.kind, fx.phase)
		add(ref, fx.x, fx.y, fx.z, g.sectorAt(fx.x, fx.y), false, false, false)
	}
	for _, p := range g.hitscanPuffs {
		if p.hidden {
			continue
		}
		ref, _ := g.hitscanEffectSpriteRef(p)
		add(ref, p.x, p.y, p.z, g.sectorAt(p.x, p.y), false, false, false)
	}
	for _, p := range g.bossSpawnCubes {
		ref, _ := g.bossCubeSpriteRef(g.worldTic)
		add(ref, p.x, p.y, p.z, g.sectorAt(p.x, p.y), false, false, false)
	}
	for _, p := range g.bossSpawnFires {
		ref, _ := g.bossSpawnFireSpriteRef(32 - p.tics)
		add(ref, p.x, p.y, p.z, g.sectorAt(p.x, p.y), false, false, false)
	}
	state := g.statusBarState()
	if !n.haveHUD || n.hudState != state {
		n.hud = n.hud[:0]
		for _, draw := range g.statusBarPatchDraws(state) {
			tex, ok := g.opts.StatusPatchBank[draw.name]
			if !ok {
				continue
			}
			n.hud = append(n.hud, levelmesh.Patch{Texture: nativeTexture(&tex), X: draw.x - float64(tex.OffsetX), Y: draw.y - float64(tex.OffsetY), W: float64(tex.Width), H: float64(tex.Height)})
		}
		n.hudState, n.haveHUD = state, true
	}
	n.weapon = n.weapon[:0]
	if g.isDead {
		return
	}
	name, prevName, flash, prevFlash, y, alpha := g.renderWeaponOverlayState()
	current, previous := weaponCompositePatchToken(name, flash), weaponCompositePatchToken(prevName, prevFlash)
	token := current
	if token == "" {
		token = previous
	}
	if previous != "" && current != "" && alpha > 0 && alpha < 1 {
		alpha = quantizeWeaponBlendAlpha(alpha)
		switch {
		case alpha <= 0:
			token = previous
		case alpha >= 1:
			token = current
		default:
			token = fmt.Sprintf("%s>%s#%d/%d", previous, current, int(math.Round(alpha*weaponBlendSteps)), weaponBlendSteps)
		}
	}
	_, tex, ok := g.resolveSpritePatchTexture(token)
	if !ok {
		return
	}
	if y == 0 {
		y = weaponTopY
	}
	bx, _ := g.weaponBob()
	n.weapon = append(n.weapon, levelmesh.Patch{Texture: nativeTexture(&tex), X: 1 + bx - float64(tex.OffsetX), Y: y - float64(tex.OffsetY), W: float64(tex.Width), H: float64(tex.Height)})
}
