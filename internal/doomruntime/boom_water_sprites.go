package doomruntime

import "math"

// Sprite lighting uses R_AddSprites' average of the fake front plane lights.
func (g *game) spriteSectorLightMul(sec int) uint32 {
	if g.heightView(sec, false) != nil {
		return uint32(sectorLightMul(g.boomSpriteLightLevel(sec)))
	}
	return g.sectorLightMulCached(sec)
}

// boomSpriteClip mirrors R_ProjectSprite's whole-object rejection and
// R_DrawSprite's height-sector clipping. gz is physical; gzt is patch-top Z.
// Clip bounds here are inclusive, unlike DSDA's exclusive cliptop/clipbot.
func (g *game) boomSpriteClip(x, y, gz int64, gzt, eye, depth, focal float64, viewH, top, bottom int) (int, int, bool) {
	control := g.heightControl(g.sectorAt(x, y))
	if control < 0 || !g.boomHeightFrameValid || !g.billboardClippingEnabled() {
		return top, bottom, top <= bottom
	}
	modelFloor, modelCeil, _ := g.sectorPhysicalRenderSnapshot(control)
	f, c, z := float64(modelFloor)/fracUnit, float64(modelCeil)/fracUnit, float64(gz)/fracUnit
	cameraControl := g.heightControl(g.sectorAt(floorFixed(g.renderPX), floorFixed(g.renderPY)))
	below, above, atOrBelow, atOrAbove := false, false, false, false
	cameraAboveFloor := false
	if cameraControl >= 0 {
		cf, cc, _ := g.sectorPhysicalRenderSnapshot(cameraControl)
		below = floorFixed(eye) < cf
		above = floorFixed(eye) > cc
		atOrBelow = floorFixed(eye) <= cf
		atOrAbove = floorFixed(eye) >= cc
		cameraAboveFloor = floorFixed(eye) > cf
	}
	if (below && z >= f) || (!below && gzt < f) || (above && gzt < c && eye >= c) || (!above && z >= c) {
		return top, bottom, false
	}
	clip := func(height float64, bottomClip bool) {
		h := int(math.Floor(float64(viewH)/2 - (height-eye)*focal/depth))
		if h < 0 || h >= viewH {
			return
		}
		if bottomClip {
			bottom = min(bottom, h-1)
		} else {
			top = max(top, h+1)
		}
	}
	if f > z {
		if f <= eye || cameraAboveFloor {
			clip(f, true)
		} else if atOrBelow {
			clip(f, false)
		}
	}
	if c < gzt {
		clip(c, atOrAbove)
	}
	return top, bottom, top <= bottom
}

func (g *game) boomSpriteLightLevel(sec int) int16 {
	// Boom 2.01/2.02 demo compatibility retains the original sprite-light bug.
	if g.opts.DemoScript != nil && (g.opts.DemoScript.Header.Version == 201 || g.opts.DemoScript.Header.Version == 202) {
		return g.sectorLightLevelCached(sec)
	}
	if v := g.heightView(sec, false); v != nil {
		return int16((int(v.floorLight) + int(v.ceilingLight)) / 2)
	}
	return g.sectorLightLevelCached(sec)
}
func (g *game) boomSpriteLightRow(sec int, fullbright bool, depth float64) int {
	if fullbright || !g.boomHeightFrameValid || g.opts.SourcePortMode {
		return 0
	}
	return doomWallLightRow(g.boomSpriteLightLevel(sec), 0, depth, float64(g.viewW)/2)
}
