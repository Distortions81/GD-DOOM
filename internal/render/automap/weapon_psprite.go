package automap

import (
	"math"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
)

type weaponAnimDef struct {
	frames    []string
	frameTics []int
}

func (g *game) tickWeaponOverlay() {
	if g.weaponAnimTics > 0 {
		g.weaponAnimTics--
	}
	if g.weaponFlashTics > 0 {
		g.weaponFlashTics--
	}
}

func (g *game) clearWeaponOverlay() {
	g.weaponAnimTics = 0
	g.weaponAnimTotalTics = 0
	g.weaponFlashTics = 0
	g.weaponFlashTotalTics = 0
}

func (g *game) startWeaponOverlayFire(id weaponID) {
	anim := weaponFireAnim(id)
	flash := weaponFlashAnim(id)
	g.weaponAnimTotalTics = sumPositive(anim.frameTics)
	g.weaponAnimTics = g.weaponAnimTotalTics
	g.weaponFlashTotalTics = sumPositive(flash.frameTics)
	g.weaponFlashTics = g.weaponFlashTotalTics
}

func sumPositive(v []int) int {
	total := 0
	for _, n := range v {
		if n > 0 {
			total += n
		}
	}
	return total
}

func weaponFireAnim(id weaponID) weaponAnimDef {
	switch id {
	case weaponFist:
		return weaponAnimDef{
			frames:    []string{"PUNGB0", "PUNGC0", "PUNGD0", "PUNGC0", "PUNGB0"},
			frameTics: []int{4, 4, 5, 4, 5},
		}
	case weaponPistol:
		return weaponAnimDef{
			frames:    []string{"PISGA0", "PISGB0", "PISGC0", "PISGB0"},
			frameTics: []int{4, 6, 4, 5},
		}
	case weaponShotgun:
		return weaponAnimDef{
			frames:    []string{"SHTGA0", "SHTGA0", "SHTGB0", "SHTGC0", "SHTGD0", "SHTGC0", "SHTGB0", "SHTGA0"},
			frameTics: []int{3, 7, 5, 5, 4, 5, 5, 3},
		}
	case weaponChaingun:
		return weaponAnimDef{
			frames:    []string{"CHGGA0", "CHGGB0", "CHGGB0"},
			frameTics: []int{4, 4, 1},
		}
	case weaponRocketLauncher:
		return weaponAnimDef{
			frames:    []string{"MISGB0", "MISGB0"},
			frameTics: []int{8, 12},
		}
	case weaponPlasma:
		return weaponAnimDef{
			frames:    []string{"PLSGA0", "PLSGB0"},
			frameTics: []int{3, 20},
		}
	case weaponBFG:
		return weaponAnimDef{
			frames:    []string{"BFGGA0", "BFGGB0", "BFGGB0"},
			frameTics: []int{20, 10, 10},
		}
	case weaponChainsaw:
		return weaponAnimDef{
			frames:    []string{"SAWGA0", "SAWGB0", "SAWGB0"},
			frameTics: []int{4, 4, 1},
		}
	default:
		return weaponAnimDef{}
	}
}

func weaponFlashAnim(id weaponID) weaponAnimDef {
	switch id {
	case weaponPistol:
		return weaponAnimDef{
			frames:    []string{"PISFA0"},
			frameTics: []int{7},
		}
	case weaponShotgun:
		return weaponAnimDef{
			frames:    []string{"SHTFA0", "SHTFB0"},
			frameTics: []int{4, 3},
		}
	case weaponChaingun:
		return weaponAnimDef{
			frames:    []string{"CHGFA0", "CHGFB0"},
			frameTics: []int{5, 5},
		}
	case weaponRocketLauncher:
		return weaponAnimDef{
			frames:    []string{"MISFA0", "MISFB0", "MISFC0", "MISFD0"},
			frameTics: []int{3, 4, 4, 4},
		}
	case weaponPlasma:
		return weaponAnimDef{
			frames:    []string{"PLSFA0", "PLSFB0"},
			frameTics: []int{4, 4},
		}
	case weaponBFG:
		return weaponAnimDef{
			frames:    []string{"BFGFA0", "BFGFB0"},
			frameTics: []int{11, 6},
		}
	default:
		return weaponAnimDef{}
	}
}

func weaponReadySpriteName(id weaponID, worldTic int) string {
	switch id {
	case weaponFist:
		return "PUNGA0"
	case weaponPistol:
		return "PISGA0"
	case weaponShotgun:
		return "SHTGA0"
	case weaponChaingun:
		return "CHGGA0"
	case weaponRocketLauncher:
		return "MISGA0"
	case weaponPlasma:
		return "PLSGA0"
	case weaponBFG:
		return "BFGGA0"
	case weaponChainsaw:
		if (worldTic/4)&1 == 0 {
			return "SAWGC0"
		}
		return "SAWGD0"
	default:
		return ""
	}
}

func animFrameName(def weaponAnimDef, tics, total int) string {
	if tics <= 0 || total <= 0 || len(def.frames) == 0 || len(def.frames) != len(def.frameTics) {
		return ""
	}
	elapsed := total - tics
	if elapsed < 0 {
		elapsed = 0
	}
	acc := 0
	for i, ft := range def.frameTics {
		if ft <= 0 {
			continue
		}
		acc += ft
		if elapsed < acc {
			return def.frames[i]
		}
	}
	return def.frames[len(def.frames)-1]
}

func (g *game) weaponSpriteName() string {
	if g == nil {
		return ""
	}
	if name := animFrameName(weaponFireAnim(g.inventory.ReadyWeapon), g.weaponAnimTics, g.weaponAnimTotalTics); name != "" {
		if _, ok := g.opts.SpritePatchBank[name]; ok {
			return name
		}
	}
	name := weaponReadySpriteName(g.inventory.ReadyWeapon, g.worldTic)
	if _, ok := g.opts.SpritePatchBank[name]; ok {
		return name
	}
	return ""
}

func (g *game) weaponFlashSpriteName() string {
	if g == nil {
		return ""
	}
	name := animFrameName(weaponFlashAnim(g.inventory.ReadyWeapon), g.weaponFlashTics, g.weaponFlashTotalTics)
	if _, ok := g.opts.SpritePatchBank[name]; ok {
		return name
	}
	return ""
}

func abs64(v int64) int64 {
	if v < 0 {
		return -v
	}
	return v
}

func (g *game) weaponBob() (float64, float64) {
	if g == nil || g.isDead {
		return 0, 0
	}
	bob := float64(abs64(g.p.momx)+abs64(g.p.momy)) / float64(fracUnit)
	bob *= 0.25
	if bob > 8 {
		bob = 8
	}
	t := (2 * math.Pi * float64(g.worldTic&63)) / 35.0
	return math.Cos(t) * bob * 0.5, math.Sin(t*2) * bob * 0.5
}

func (g *game) spritePatch(name string) (*ebiten.Image, int, int, int, int, bool) {
	key := strings.ToUpper(strings.TrimSpace(name))
	p, ok := g.opts.SpritePatchBank[key]
	if !ok || p.Width <= 0 || p.Height <= 0 || len(p.RGBA) != p.Width*p.Height*4 {
		return nil, 0, 0, 0, 0, false
	}
	if g.spritePatchImg == nil {
		g.spritePatchImg = make(map[string]*ebiten.Image, 256)
	}
	if img, ok := g.spritePatchImg[key]; ok {
		return img, p.Width, p.Height, p.OffsetX, p.OffsetY, true
	}
	img := ebiten.NewImage(p.Width, p.Height)
	img.WritePixels(p.RGBA)
	g.spritePatchImg[key] = img
	return img, p.Width, p.Height, p.OffsetX, p.OffsetY, true
}

func (g *game) drawSpritePatch(screen *ebiten.Image, name string, x, y, sx, sy float64) bool {
	img, _, _, ox, oy, ok := g.spritePatch(name)
	if !ok {
		return false
	}
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Scale(sx, sy)
	op.GeoM.Translate(x-float64(ox)*sx, y-float64(oy)*sy)
	screen.DrawImage(img, op)
	return true
}

func (g *game) drawWeaponOverlay(screen *ebiten.Image) {
	if g == nil || g.mode != viewWalk || g.isDead {
		return
	}
	name := g.weaponSpriteName()
	if name == "" {
		return
	}
	sx, sy, ox, oy := g.hudTransform()
	bx, by := g.weaponBob()
	// Doom psprite coordinates are relative to a 320x200 logical screen.
	x := ox + (1.0+bx)*sx
	y := oy + (32.0+by)*sy
	_ = g.drawSpritePatch(screen, name, x, y, sx, sy)
	if flash := g.weaponFlashSpriteName(); flash != "" {
		_ = g.drawSpritePatch(screen, flash, x, y, sx, sy)
	}
}
