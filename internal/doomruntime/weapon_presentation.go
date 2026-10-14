package doomruntime

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

// Integer gameplay coordinates remain compatible with the simulation oracle.
// Preserve the fractional part and horizontal position separately for drawing.
type weaponPresentationPose struct {
	X, FractionY, BobY int64
	Valid              bool
}

func (g *game) weaponPresentationY(y int, pose weaponPresentationPose) float64 {
	result := float64(y) + float64(pose.FractionY)/fracUnit
	if g.opts.SourcePortMode && g.opts.DisableWeaponBob {
		result -= float64(pose.BobY) / fracUnit
	}
	return result
}

func (g *game) weaponPresentationPosition() (float64, float64) {
	x := func(pose weaponPresentationPose) float64 {
		if g.opts.SourcePortMode && g.opts.DisableWeaponBob || !pose.Valid {
			return 1
		}
		return float64(pose.X) / fracUnit
	}
	alpha := 1.0
	if g.opts.SourcePortMode {
		alpha = applyBlendShutter(math.Max(0, math.Min(1, g.weaponRenderAlpha())), weaponSpriteShutterAngle)
	}
	return lerp(x(g.prevWeaponPose), x(g.weaponPose), alpha), lerp(g.weaponPresentationY(g.prevWeaponPSpriteY, g.prevWeaponPose), g.weaponPresentationY(g.weaponPSpriteY, g.weaponPose), alpha)
}

func (g *game) weaponLightRow(fullbright bool) int {
	if g.playerPowerupInvulnerabilityVisualActive() && len(g.opts.DoomColorMap) >= 33*256 {
		return 32
	}
	if g.playerInfraredBright() && len(g.opts.DoomColorMap) >= 2*256 {
		return 1
	}
	if fullbright || g.opts.DisableDoomLighting {
		return 0
	}
	light := 255
	if g.m != nil {
		sector := g.sectorAt(g.p.x, g.p.y)
		if sector < 0 && len(g.m.Sectors) == 1 {
			sector = 0
		}
		if sector >= 0 && sector < len(g.m.Sectors) {
			light = int(g.m.Sectors[sector].Light)
		}
	}
	if g.opts.SourcePortMode && !g.opts.SourcePortSectorLighting {
		light = 255
	}
	extra := 0
	if g.weaponFlashState != weaponStateNone {
		extra = 1
		switch g.weaponFlashState {
		case weaponStateShotgunFlash2, weaponStateSuperShotgunFlash2, weaponStateChaingunFlash2, weaponStateRocketFlash3, weaponStateRocketFlash4, weaponStateBFGFlash2:
			extra = 2
		}
	}
	// R_DrawPlayerSprites takes scalelight[lightnum][MAXLIGHTSCALE-1].
	lightnum := min(15, max(0, (light>>4)+extra))
	width := max(1, g.walkWeaponViewportRect().Dx())
	if g.opts.SourcePortMode {
		// Modern scales a logical weapon with the window. Lighting must not
		// treat more framebuffer pixels as a smaller projected weapon.
		width = doomLogicalW
	}
	return min(31, max(0, ((15-lightnum)*2*32)/16-47*320/width/2))
}

func weaponPatchFullbright(name string) bool {
	for state, def := range weaponPspriteDefs {
		if state >= weaponStatePistolFlash && def.sprite == name {
			return true
		}
	}
	return false
}

// Light each layer before compositing: the muzzle flash is fullbright while
// the gun still uses its sector light (plus the flash's extralight).
func (g *game) weaponCompositeToken(body, flash string) string {
	lit := func(name string) string {
		if name == "" {
			return ""
		}
		row := g.weaponLightRow(weaponPatchFullbright(name))
		if row == 0 || len(g.opts.DoomPaletteRGBA) < 1024 || len(g.opts.DoomColorMap) < (row+1)*256 {
			return name
		}
		return fmt.Sprintf("L%d@%s", row, name)
	}
	return weaponCompositePatchToken(lit(body), lit(flash))
}

func (g *game) litWeaponPatch(key string) (WallTexture, bool) {
	prefix, name, ok := strings.Cut(key, "@")
	if !ok || !strings.HasPrefix(prefix, "L") || strings.ContainsAny(name, "@+>#/") {
		return WallTexture{}, false
	}
	row, err := strconv.Atoi(prefix[1:])
	if err != nil || row < 0 || len(g.opts.DoomColorMap) < (row+1)*256 || len(g.opts.DoomPaletteRGBA) < 1024 {
		return WallTexture{}, false
	}
	tex, ok := g.opts.SpritePatchBank[name]
	if !ok {
		return WallTexture{}, false
	}
	if len(tex.Indexed) != tex.Width*tex.Height {
		return tex, true
	}
	rgba := make([]byte, len(tex.RGBA))
	for i, idx := range tex.Indexed {
		mapped := int(g.opts.DoomColorMap[row*256+int(idx)])
		copy(rgba[i*4:i*4+3], g.opts.DoomPaletteRGBA[mapped*4:mapped*4+3])
		rgba[i*4+3] = tex.RGBA[i*4+3]
	}
	tex.RGBA, tex.RGBA32, tex.ColMajor = rgba, nil, nil
	return tex, true
}

// Both presentation hosts use the same fullscreen lift. It exposes more of the
// weapon while preserving its original patch offsets and raising/lowering pose.
func (g *game) weaponOverlayLogicalY(y float64) float64 {
	if y == 0 {
		y = weaponTopY
	}
	if g.opts.SourcePortMode && g.statusBarDisplayMode() != statusBarDisplayBottom {
		y -= 8
	}
	return y
}
