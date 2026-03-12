package doomruntime

import (
	"image"
	"math"
	"strings"

	"gddoom/internal/render/scene"

	"github.com/hajimehoshi/ebiten/v2"
)

type weaponPspriteState int

const (
	weaponStateNone weaponPspriteState = iota
	weaponStateFistReady
	weaponStateFistAtk1
	weaponStateFistAtk2
	weaponStateFistAtk3
	weaponStateFistAtk4
	weaponStateFistAtk5
	weaponStatePistolReady
	weaponStatePistolAtk1
	weaponStatePistolAtk2
	weaponStatePistolAtk3
	weaponStatePistolAtk4
	weaponStateShotgunReady
	weaponStateShotgunAtk1
	weaponStateShotgunAtk2
	weaponStateShotgunAtk3
	weaponStateShotgunAtk4
	weaponStateShotgunAtk5
	weaponStateShotgunAtk6
	weaponStateShotgunAtk7
	weaponStateShotgunAtk8
	weaponStateShotgunAtk9
	weaponStateChaingunReady
	weaponStateChaingunAtk1
	weaponStateChaingunAtk2
	weaponStateChaingunAtk3
	weaponStateRocketReady
	weaponStateRocketAtk1
	weaponStateRocketAtk2
	weaponStateRocketAtk3
	weaponStateSawReadyA
	weaponStateSawReadyB
	weaponStateSawAtk1
	weaponStateSawAtk2
	weaponStateSawAtk3
	weaponStatePlasmaReady
	weaponStatePlasmaAtk1
	weaponStatePlasmaAtk2
	weaponStateBFGReady
	weaponStateBFGAtk1
	weaponStateBFGAtk2
	weaponStateBFGAtk3
	weaponStateBFGAtk4
	weaponStatePistolFlash
	weaponStateShotgunFlash1
	weaponStateShotgunFlash2
	weaponStateChaingunFlash1
	weaponStateChaingunFlash2
	weaponStateRocketFlash1
	weaponStateRocketFlash2
	weaponStateRocketFlash3
	weaponStateRocketFlash4
	weaponStatePlasmaFlash1
	weaponStatePlasmaFlash2
	weaponStateBFGFlash1
	weaponStateBFGFlash2
)

type weaponPspriteDef struct {
	sprite string
	tics   int
	next   weaponPspriteState
	action weaponPspriteAction
}

type weaponPspriteAction uint8

const (
	weaponPspriteActionNone weaponPspriteAction = iota
	weaponPspriteActionReady
	weaponPspriteActionRefire
	weaponPspriteActionFire
	weaponPspriteActionGunFlash
	weaponPspriteActionBFGSound
)

var weaponPspriteDefs = map[weaponPspriteState]weaponPspriteDef{
	weaponStateFistReady:      {sprite: "PUNGA0", tics: 1, next: weaponStateFistReady, action: weaponPspriteActionReady},
	weaponStateFistAtk1:       {sprite: "PUNGB0", tics: 4, next: weaponStateFistAtk2},
	weaponStateFistAtk2:       {sprite: "PUNGC0", tics: 4, next: weaponStateFistAtk3, action: weaponPspriteActionFire},
	weaponStateFistAtk3:       {sprite: "PUNGD0", tics: 5, next: weaponStateFistAtk4},
	weaponStateFistAtk4:       {sprite: "PUNGC0", tics: 4, next: weaponStateFistAtk5},
	weaponStateFistAtk5:       {sprite: "PUNGB0", tics: 5, next: weaponStateFistReady, action: weaponPspriteActionRefire},
	weaponStatePistolReady:    {sprite: "PISGA0", tics: 1, next: weaponStatePistolReady, action: weaponPspriteActionReady},
	weaponStatePistolAtk1:     {sprite: "PISGA0", tics: 4, next: weaponStatePistolAtk2},
	weaponStatePistolAtk2:     {sprite: "PISGB0", tics: 6, next: weaponStatePistolAtk3, action: weaponPspriteActionFire},
	weaponStatePistolAtk3:     {sprite: "PISGC0", tics: 4, next: weaponStatePistolAtk4},
	weaponStatePistolAtk4:     {sprite: "PISGB0", tics: 5, next: weaponStatePistolReady, action: weaponPspriteActionRefire},
	weaponStateShotgunReady:   {sprite: "SHTGA0", tics: 1, next: weaponStateShotgunReady, action: weaponPspriteActionReady},
	weaponStateShotgunAtk1:    {sprite: "SHTGA0", tics: 3, next: weaponStateShotgunAtk2},
	weaponStateShotgunAtk2:    {sprite: "SHTGA0", tics: 7, next: weaponStateShotgunAtk3, action: weaponPspriteActionFire},
	weaponStateShotgunAtk3:    {sprite: "SHTGB0", tics: 5, next: weaponStateShotgunAtk4},
	weaponStateShotgunAtk4:    {sprite: "SHTGC0", tics: 5, next: weaponStateShotgunAtk5},
	weaponStateShotgunAtk5:    {sprite: "SHTGD0", tics: 4, next: weaponStateShotgunAtk6},
	weaponStateShotgunAtk6:    {sprite: "SHTGC0", tics: 5, next: weaponStateShotgunAtk7},
	weaponStateShotgunAtk7:    {sprite: "SHTGB0", tics: 5, next: weaponStateShotgunAtk8},
	weaponStateShotgunAtk8:    {sprite: "SHTGA0", tics: 3, next: weaponStateShotgunAtk9},
	weaponStateShotgunAtk9:    {sprite: "SHTGA0", tics: 7, next: weaponStateShotgunReady, action: weaponPspriteActionRefire},
	weaponStateChaingunReady:  {sprite: "CHGGA0", tics: 1, next: weaponStateChaingunReady, action: weaponPspriteActionReady},
	weaponStateChaingunAtk1:   {sprite: "CHGGA0", tics: 4, next: weaponStateChaingunAtk2, action: weaponPspriteActionFire},
	weaponStateChaingunAtk2:   {sprite: "CHGGB0", tics: 4, next: weaponStateChaingunAtk3, action: weaponPspriteActionFire},
	weaponStateChaingunAtk3:   {sprite: "CHGGB0", tics: 0, next: weaponStateChaingunReady, action: weaponPspriteActionRefire},
	weaponStateRocketReady:    {sprite: "MISGA0", tics: 1, next: weaponStateRocketReady, action: weaponPspriteActionReady},
	weaponStateRocketAtk1:     {sprite: "MISGB0", tics: 8, next: weaponStateRocketAtk2, action: weaponPspriteActionGunFlash},
	weaponStateRocketAtk2:     {sprite: "MISGB0", tics: 12, next: weaponStateRocketAtk3, action: weaponPspriteActionFire},
	weaponStateRocketAtk3:     {sprite: "MISGB0", tics: 0, next: weaponStateRocketReady, action: weaponPspriteActionRefire},
	weaponStateSawReadyA:      {sprite: "SAWGC0", tics: 4, next: weaponStateSawReadyB, action: weaponPspriteActionReady},
	weaponStateSawReadyB:      {sprite: "SAWGD0", tics: 4, next: weaponStateSawReadyA, action: weaponPspriteActionReady},
	weaponStateSawAtk1:        {sprite: "SAWGA0", tics: 4, next: weaponStateSawAtk2, action: weaponPspriteActionFire},
	weaponStateSawAtk2:        {sprite: "SAWGB0", tics: 4, next: weaponStateSawAtk3, action: weaponPspriteActionFire},
	weaponStateSawAtk3:        {sprite: "SAWGB0", tics: 0, next: weaponStateSawReadyA, action: weaponPspriteActionRefire},
	weaponStatePlasmaReady:    {sprite: "PLSGA0", tics: 1, next: weaponStatePlasmaReady, action: weaponPspriteActionReady},
	weaponStatePlasmaAtk1:     {sprite: "PLSGA0", tics: 3, next: weaponStatePlasmaAtk2, action: weaponPspriteActionFire},
	weaponStatePlasmaAtk2:     {sprite: "PLSGB0", tics: 20, next: weaponStatePlasmaReady, action: weaponPspriteActionRefire},
	weaponStateBFGReady:       {sprite: "BFGGA0", tics: 1, next: weaponStateBFGReady, action: weaponPspriteActionReady},
	weaponStateBFGAtk1:        {sprite: "BFGGA0", tics: 20, next: weaponStateBFGAtk2, action: weaponPspriteActionBFGSound},
	weaponStateBFGAtk2:        {sprite: "BFGGB0", tics: 10, next: weaponStateBFGAtk3, action: weaponPspriteActionGunFlash},
	weaponStateBFGAtk3:        {sprite: "BFGGB0", tics: 10, next: weaponStateBFGAtk4, action: weaponPspriteActionFire},
	weaponStateBFGAtk4:        {sprite: "BFGGB0", tics: 20, next: weaponStateBFGReady, action: weaponPspriteActionRefire},
	weaponStatePistolFlash:    {sprite: "PISFA0", tics: 7, next: weaponStateNone},
	weaponStateShotgunFlash1:  {sprite: "SHTFA0", tics: 4, next: weaponStateShotgunFlash2},
	weaponStateShotgunFlash2:  {sprite: "SHTFB0", tics: 3, next: weaponStateNone},
	weaponStateChaingunFlash1: {sprite: "CHGFA0", tics: 5, next: weaponStateNone},
	weaponStateChaingunFlash2: {sprite: "CHGFB0", tics: 5, next: weaponStateNone},
	weaponStateRocketFlash1:   {sprite: "MISFA0", tics: 3, next: weaponStateRocketFlash2},
	weaponStateRocketFlash2:   {sprite: "MISFB0", tics: 4, next: weaponStateRocketFlash3},
	weaponStateRocketFlash3:   {sprite: "MISFC0", tics: 4, next: weaponStateRocketFlash4},
	weaponStateRocketFlash4:   {sprite: "MISFD0", tics: 4, next: weaponStateNone},
	weaponStatePlasmaFlash1:   {sprite: "PLSFA0", tics: 4, next: weaponStateNone},
	weaponStatePlasmaFlash2:   {sprite: "PLSFB0", tics: 4, next: weaponStateNone},
	weaponStateBFGFlash1:      {sprite: "BFGFA0", tics: 11, next: weaponStateBFGFlash2},
	weaponStateBFGFlash2:      {sprite: "BFGFB0", tics: 6, next: weaponStateNone},
}

func weaponStateForReady(id weaponID) weaponPspriteState {
	switch id {
	case weaponFist:
		return weaponStateFistReady
	case weaponPistol:
		return weaponStatePistolReady
	case weaponShotgun:
		return weaponStateShotgunReady
	case weaponChaingun:
		return weaponStateChaingunReady
	case weaponRocketLauncher:
		return weaponStateRocketReady
	case weaponPlasma:
		return weaponStatePlasmaReady
	case weaponBFG:
		return weaponStateBFGReady
	case weaponChainsaw:
		return weaponStateSawReadyA
	default:
		return weaponStateNone
	}
}

func weaponStateForAttack(id weaponID) weaponPspriteState {
	switch id {
	case weaponFist:
		return weaponStateFistAtk1
	case weaponPistol:
		return weaponStatePistolAtk1
	case weaponShotgun:
		return weaponStateShotgunAtk1
	case weaponChaingun:
		return weaponStateChaingunAtk1
	case weaponRocketLauncher:
		return weaponStateRocketAtk1
	case weaponPlasma:
		return weaponStatePlasmaAtk1
	case weaponBFG:
		return weaponStateBFGAtk1
	case weaponChainsaw:
		return weaponStateSawAtk1
	default:
		return weaponStateNone
	}
}

func flashStartState(id weaponID) weaponPspriteState {
	switch id {
	case weaponPistol:
		return weaponStatePistolFlash
	case weaponShotgun:
		return weaponStateShotgunFlash1
	case weaponChaingun:
		return weaponStateChaingunFlash1
	case weaponRocketLauncher:
		return weaponStateRocketFlash1
	case weaponPlasma:
		return weaponStatePlasmaFlash1
	case weaponBFG:
		return weaponStateBFGFlash1
	default:
		return weaponStateNone
	}
}

func (g *game) tickWeaponOverlay() {
	g.tickWeaponPSprite(false)
	g.tickWeaponPSprite(true)
}

func (g *game) tickWeaponPSprite(flash bool) {
	var state weaponPspriteState
	var tics *int
	if flash {
		state = g.weaponFlashState
		tics = &g.weaponFlashTics
	} else {
		g.ensureWeaponPSprites()
		state = g.weaponState
		tics = &g.weaponStateTics
	}
	if state == weaponStateNone {
		return
	}
	if *tics == -1 {
		return
	}
	*tics--
	if *tics > 0 {
		return
	}
	def := weaponPspriteDefs[state]
	if flash {
		g.setWeaponPSpriteState(def.next, true)
		return
	}
	g.setWeaponPSpriteState(def.next, false)
}

func (g *game) clearWeaponOverlay() {
	g.weaponState = weaponStateNone
	g.weaponStateTics = 0
	g.weaponFlashState = weaponStateNone
	g.weaponFlashTics = 0
}

func (g *game) startWeaponOverlayFire(id weaponID) {
	g.setWeaponPSpriteState(weaponStateForAttack(id), false)
}

func (g *game) startWeaponFlashState(state weaponPspriteState) {
	g.setWeaponPSpriteState(state, true)
}

func (g *game) ensureWeaponPSprites() {
	if g == nil {
		return
	}
	if g.weaponState != weaponStateNone {
		return
	}
	g.ensureWeaponDefaults()
	g.setWeaponPSpriteState(weaponStateForReady(g.inventory.ReadyWeapon), false)
}

func (g *game) setWeaponPSpriteState(state weaponPspriteState, flash bool) {
	for {
		if state == weaponStateNone {
			if flash {
				g.weaponFlashState = weaponStateNone
				g.weaponFlashTics = 0
			} else {
				g.weaponState = weaponStateNone
				g.weaponStateTics = 0
			}
			return
		}
		def, ok := weaponPspriteDefs[state]
		if !ok {
			if flash {
				g.weaponFlashState = weaponStateNone
				g.weaponFlashTics = 0
			} else {
				g.weaponState = weaponStateNone
				g.weaponStateTics = 0
			}
			return
		}
		if flash {
			g.weaponFlashState = state
			g.weaponFlashTics = def.tics
		} else {
			g.weaponState = state
			g.weaponStateTics = def.tics
		}
		switch def.action {
		case weaponPspriteActionReady:
			g.weaponActionReady(state)
		case weaponPspriteActionRefire:
			g.weaponActionRefire(state)
		case weaponPspriteActionFire:
			g.weaponActionFire(state)
		case weaponPspriteActionGunFlash:
			g.weaponActionGunFlash(state)
		case weaponPspriteActionBFGSound:
			g.weaponActionBFGSound(state)
		}
		if def.tics != 0 {
			return
		}
		state = def.next
	}
}

func weaponReadySpriteName(id weaponID, worldTic int) string {
	if id == weaponChainsaw {
		if (worldTic/4)&1 == 0 {
			return "SAWGC0"
		}
		return "SAWGD0"
	}
	if st := weaponStateForReady(id); st != weaponStateNone {
		return weaponPspriteDefs[st].sprite
	}
	return ""
}

func (g *game) weaponSpriteName() string {
	if g == nil {
		return ""
	}
	g.ensureWeaponPSprites()
	name := weaponPspriteDefs[g.weaponState].sprite
	if _, ok := g.opts.SpritePatchBank[name]; ok {
		return name
	}
	return ""
}

func (g *game) weaponFlashSpriteName() string {
	if g == nil || g.weaponFlashState == weaponStateNone {
		return ""
	}
	name := weaponPspriteDefs[g.weaponFlashState].sprite
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
	if (!ok || p.Width <= 0 || p.Height <= 0 || len(p.RGBA) != p.Width*p.Height*4) && g != nil {
		if base := fallbackSpritePatchKey(key); base != "" {
			if tex, okBase := g.opts.SpritePatchBank[base]; okBase && tex.Width > 0 && tex.Height > 0 && len(tex.RGBA) == tex.Width*tex.Height*4 {
				key = base
				p = tex
				ok = true
			}
		}
	}
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

func fallbackSpritePatchKey(key string) string {
	if key == "" {
		return ""
	}
	gt := strings.IndexByte(key, '>')
	hash := strings.IndexByte(key, '#')
	if gt <= 0 || hash <= gt+1 {
		return ""
	}
	base := strings.TrimSpace(key[:gt])
	if base == "" {
		return ""
	}
	return base
}

func (g *game) drawSpritePatch(screen *ebiten.Image, name string, x, y, sx, sy float64) bool {
	img, _, _, ox, oy, ok := g.spritePatch(name)
	if !ok {
		return false
	}
	return drawSpritePatchClipped(screen, img, x, y, sx, sy, ox, oy, screen.Bounds().Dx(), screen.Bounds().Dy())
}

func drawSpritePatchClipped(screen, img *ebiten.Image, x, y, sx, sy float64, ox, oy, clipW, clipH int) bool {
	if screen == nil || img == nil || sx <= 0 || sy <= 0 || clipW <= 0 || clipH <= 0 {
		return false
	}
	w := img.Bounds().Dx()
	h := img.Bounds().Dy()
	if w <= 0 || h <= 0 {
		return false
	}
	dstX := x - float64(ox)*sx
	dstY := y - float64(oy)*sy
	x0, x1, y0, y1, ok := scene.ClampedSpriteBounds(dstX, dstY, float64(w)*sx, float64(h)*sy, 0, clipH-1, clipW, clipH)
	if !ok {
		return false
	}
	srcX0 := max(0, min(w, int(math.Floor((float64(x0)-dstX)/sx))))
	srcY0 := max(0, min(h, int(math.Floor((float64(y0)-dstY)/sy))))
	srcX1 := max(srcX0+1, min(w, int(math.Ceil((float64(x1+1)-dstX)/sx))))
	srcY1 := max(srcY0+1, min(h, int(math.Ceil((float64(y1+1)-dstY)/sy))))
	if srcX0 >= srcX1 || srcY0 >= srcY1 {
		return false
	}
	sub, ok := img.SubImage(image.Rect(srcX0, srcY0, srcX1, srcY1)).(*ebiten.Image)
	if !ok {
		return false
	}
	op := &ebiten.DrawImageOptions{}
	op.Filter = ebiten.FilterNearest
	op.GeoM.Scale(sx, sy)
	op.GeoM.Translate(dstX+float64(srcX0)*sx, dstY+float64(srcY0)*sy)
	screen.DrawImage(sub, op)
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
	baseY := 32.0
	switch g.statusBarDisplayMode() {
	case statusBarDisplayOverlay, statusBarDisplayHidden:
		baseY = 24.0
	}
	rect := g.walkWeaponViewportRect()
	target := screen
	if rect.Dx() < g.viewW || rect.Dy() < g.viewH || rect.Min.X != 0 || rect.Min.Y != 0 {
		sub, ok := screen.SubImage(rect).(*ebiten.Image)
		if !ok {
			return
		}
		target = sub
	}
	scale := float64(rect.Dx()) / doomLogicalW
	bx, by := g.weaponBob()
	const weaponBottomCheat = -8.0
	const weaponBobBottomReserve = 4.0
	x := (1.0 + bx) * scale
	y := float64(rect.Dy()) - (doomLogicalH-(baseY+by)+weaponBottomCheat+weaponBobBottomReserve)*scale
	_ = g.drawSpritePatch(target, name, x, y, scale, scale)
	if flash := g.weaponFlashSpriteName(); flash != "" {
		_ = g.drawSpritePatch(target, flash, x, y, scale, scale)
	}
}
