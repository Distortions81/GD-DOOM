package doomruntime

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"gddoom/internal/doomrand"
	"gddoom/internal/render/hud"

	"github.com/hajimehoshi/ebiten/v2"
)

const (
	huFontStart       = '!' // HU_FONTSTART
	huFontEnd         = '_' // HU_FONTEND
	huMsgX            = 0   // HU_MSGX
	huMsgY            = 0   // HU_MSGY
	statusBarLogicalY = 168.0
)

type statusBarCacheState struct {
	mode         statusBarDisplayMode
	hudScale     float64
	health       int
	armor        int
	readyAmmo    int
	hasReadyAmmo bool
	weaponOwned  [6]bool
	keyOn        [3]bool
	ammoCur      [4]int
	ammoMax      [4]int
	facePatch    string
}

func (g *game) drawDoomStatusBar(screen *ebiten.Image) {
	if !g.statusBarVisible() {
		return
	}
	maxBullets, maxShells, maxRockets, maxCells := ammoCaps(g.inventory.Backpack)
	readyAmmo, hasReadyAmmo := g.statusReadyAmmo()
	state := statusBarCacheState{
		mode:         g.statusBarDisplayMode(),
		hudScale:     g.hudScaleValue(),
		health:       g.stats.Health,
		armor:        g.stats.Armor,
		readyAmmo:    readyAmmo,
		hasReadyAmmo: hasReadyAmmo,
		weaponOwned: [6]bool{
			g.statusWeaponOwned(2),
			g.statusWeaponOwned(3),
			g.statusWeaponOwned(4),
			g.statusWeaponOwned(5),
			g.statusWeaponOwned(6),
			g.statusWeaponOwned(7),
		},
		keyOn:     [3]bool{g.inventory.BlueKey, g.inventory.RedKey, g.inventory.YellowKey},
		ammoCur:   [4]int{g.stats.Bullets, g.stats.Shells, g.stats.Cells, g.stats.Rockets},
		ammoMax:   [4]int{maxBullets, maxShells, maxCells, maxRockets},
		facePatch: g.statusFacePatchName(),
	}
	g.ensureStatusBarCache(state)
	if g.statusBarCacheImg != nil {
		g.drawStatusBarCacheImage(screen, state)
	}
}

func (g *game) ensureStatusBarCache(state statusBarCacheState) {
	if g == nil {
		return
	}
	w := doomLogicalW
	h := doomLogicalH - int(statusBarLogicalY)
	if g.statusBarCacheImg == nil || g.statusBarCacheImg.Bounds().Dx() != w || g.statusBarCacheImg.Bounds().Dy() != h {
		g.statusBarCacheImg = newDebugImage("statusbar:cache", w, h)
		g.statusBarCacheValid = false
	}
	if g.statusBarCacheValid && g.statusBarCacheState == state {
		return
	}
	g.statusBarCacheImg.Clear()
	g.drawStatusBarCached(g.statusBarCacheImg, state)
	g.statusBarCacheState = state
	g.statusBarCacheValid = true
}

func (g *game) drawStatusBarCacheImage(screen *ebiten.Image, state statusBarCacheState) {
	if g == nil || g.statusBarCacheImg == nil {
		return
	}
	op := &ebiten.DrawImageOptions{}
	op.Filter = ebiten.FilterNearest
	switch state.mode {
	case statusBarDisplayOverlay:
		scale := math.Max(1.0, state.hudScale)
		x := (float64(g.viewW) - float64(g.statusBarCacheImg.Bounds().Dx())*scale) * 0.5
		y := float64(g.viewH) - float64(g.statusBarCacheImg.Bounds().Dy())*scale
		op.GeoM.Scale(scale, scale)
		op.GeoM.Translate(x, y)
	default:
		sx, sy, ox, oy := hud.Transform(g.viewW, g.viewH, g.hudUsesLogicalLayout(), state.hudScale)
		op.GeoM.Scale(sx, sy)
		op.GeoM.Translate(ox, oy+statusBarLogicalY*sy)
	}
	screen.DrawImage(g.statusBarCacheImg, op)
}

func (g *game) drawStatusBarCached(screen *ebiten.Image, state statusBarCacheState) {
	g.drawStatusBarLogicalBar(screen, state)
}

func (g *game) drawStatusBarLogicalBar(screen *ebiten.Image, state statusBarCacheState) {
	x0 := 0.0
	y0 := 0.0
	drawPatch := func(name string, x, y float64) {
		g.drawStatusPatch(screen, name, x0+x, y0+(y-statusBarLogicalY), 1, 1)
	}
	drawTallNum := func(value, digits int, rightX, y float64) {
		g.drawStatusTallNum(screen, value, digits, x0+rightX, y0+(y-statusBarLogicalY), 1, 1)
	}
	drawShortNum := func(value, digits int, rightX, y float64) {
		g.drawStatusShortNum(screen, value, digits, x0+rightX, y0+(y-statusBarLogicalY), 1, 1)
	}
	drawPercent := func(value int, x, y float64) {
		g.drawStatusPercent(screen, value, x0+x, y0+(y-statusBarLogicalY), 1, 1)
	}
	drawPatch("STBAR", 0, 168)
	drawPatch("STARMS", 104, 168)
	if state.hasReadyAmmo {
		drawTallNum(state.readyAmmo, 3, 44, 171)
	}
	drawPercent(state.health, 90, 171)
	drawPercent(state.armor, 221, 171)
	for i := 0; i < 6; i++ {
		slot := i + 2
		x := float64(111 + (i%3)*12)
		y := float64(172 + (i/3)*10)
		name := "STGNUM" + string(rune('0'+slot))
		if state.weaponOwned[i] {
			name = "STYSNUM" + string(rune('0'+slot))
		}
		drawPatch(name, x, y)
	}
	keyNames := [3]string{"STKEYS0", "STKEYS2", "STKEYS1"}
	keyY := [3]float64{171, 181, 191}
	for i := 0; i < 3; i++ {
		if state.keyOn[i] {
			drawPatch(keyNames[i], 239, keyY[i])
		}
	}
	curPos := [4][2]float64{{288, 173}, {288, 179}, {288, 191}, {288, 185}}
	maxPos := [4][2]float64{{314, 173}, {314, 179}, {314, 191}, {314, 185}}
	for i := 0; i < 4; i++ {
		drawShortNum(state.ammoCur[i], 3, curPos[i][0], curPos[i][1])
		drawShortNum(state.ammoMax[i], 3, maxPos[i][0], maxPos[i][1])
	}
	drawPatch(state.facePatch, 143, 168)
}

func (g *game) statusPatch(name string) (*ebiten.Image, int, int, int, int, bool) {
	key := strings.ToUpper(strings.TrimSpace(name))
	p, ok := g.opts.StatusPatchBank[key]
	if !ok || p.Width <= 0 || p.Height <= 0 || len(p.RGBA) != p.Width*p.Height*4 {
		return nil, 0, 0, 0, 0, false
	}
	if g.statusPatchImg == nil {
		g.statusPatchImg = make(map[string]*ebiten.Image, 96)
	}
	if img, ok := g.statusPatchImg[key]; ok {
		return img, p.Width, p.Height, p.OffsetX, p.OffsetY, true
	}
	g.debugImageAlloc("status-patch:"+key, p.Width, p.Height)
	img := newDebugImage("statusbar:patch:"+key, p.Width, p.Height)
	img.WritePixels(p.RGBA)
	g.statusPatchImg[key] = img
	return img, p.Width, p.Height, p.OffsetX, p.OffsetY, true
}

func (g *game) drawStatusPatchAlpha(screen *ebiten.Image, name string, x, y, sx, sy, alpha float64) {
	img, _, _, ox, oy, ok := g.statusPatch(name)
	if !ok {
		return
	}
	op := &ebiten.DrawImageOptions{}
	op.Filter = ebiten.FilterNearest
	op.ColorScale.ScaleAlpha(float32(alpha))
	op.GeoM.Scale(sx, sy)
	op.GeoM.Translate(x-float64(ox)*sx, y-float64(oy)*sy)
	screen.DrawImage(img, op)
}

func (g *game) drawStatusPatch(screen *ebiten.Image, name string, x, y, sx, sy float64) {
	g.drawStatusPatchAlpha(screen, name, x, y, sx, sy, 1)
}

type statusDigitDraw struct {
	name string
	x    float64
}

func (g *game) statusDigitWidth(prefix string) (int, bool) {
	_, w, _, _, _, ok := g.statusPatch(prefix + "0")
	if !ok || w <= 0 {
		return 0, false
	}
	return w, true
}

func (g *game) statusDigitDraws(prefix string, value, digits int, rightX, sx float64) []statusDigitDraw {
	if value < 0 {
		value = 0
	}
	s := strconv.Itoa(value)
	if len(s) > digits {
		s = s[len(s)-digits:]
	}
	cellW, ok := g.statusDigitWidth(prefix)
	if !ok {
		return nil
	}
	x := rightX
	draws := make([]statusDigitDraw, 0, len(s))
	for i := len(s) - 1; i >= 0; i-- {
		name := prefix + string(s[i])
		if _, _, _, _, _, ok := g.statusPatch(name); !ok {
			continue
		}
		x -= float64(cellW) * sx
		draws = append(draws, statusDigitDraw{name: name, x: x})
	}
	return draws
}

func (g *game) drawStatusTallNumAlpha(screen *ebiten.Image, value, digits int, rightX, y, sx, sy, alpha float64) {
	for _, draw := range g.statusDigitDraws("STTNUM", value, digits, rightX, sx) {
		g.drawStatusPatchAlpha(screen, draw.name, draw.x, y, sx, sy, alpha)
	}
}

func (g *game) drawStatusTallNum(screen *ebiten.Image, value, digits int, rightX, y, sx, sy float64) {
	g.drawStatusTallNumAlpha(screen, value, digits, rightX, y, sx, sy, 1)
}

func (g *game) drawStatusShortNumAlpha(screen *ebiten.Image, value, digits int, rightX, y, sx, sy, alpha float64) {
	for _, draw := range g.statusDigitDraws("STYSNUM", value, digits, rightX, sx) {
		g.drawStatusPatchAlpha(screen, draw.name, draw.x, y, sx, sy, alpha)
	}
}

func (g *game) drawStatusShortNum(screen *ebiten.Image, value, digits int, rightX, y, sx, sy float64) {
	g.drawStatusShortNumAlpha(screen, value, digits, rightX, y, sx, sy, 1)
}

func (g *game) drawStatusPercentAlpha(screen *ebiten.Image, value int, x, y, sx, sy, alpha float64) {
	_, _, _, _, _, ok := g.statusPatch("STTPRCNT")
	if ok {
		g.drawStatusPatchAlpha(screen, "STTPRCNT", x, y, sx, sy, alpha)
		g.drawStatusTallNumAlpha(screen, value, 3, x, y, sx, sy, alpha)
		return
	}
	g.drawStatusTallNumAlpha(screen, value, 3, x, y, sx, sy, alpha)
}

func (g *game) drawStatusPercent(screen *ebiten.Image, value int, x, y, sx, sy float64) {
	g.drawStatusPercentAlpha(screen, value, x, y, sx, sy, 1)
}

func (g *game) messageFontGlyph(ch rune) (*ebiten.Image, int, int, int, int, bool) {
	if ch >= 'a' && ch <= 'z' {
		ch -= 'a' - 'A'
	}
	p, ok := g.opts.MessageFontBank[ch]
	if !ok || p.Width <= 0 || p.Height <= 0 || len(p.RGBA) != p.Width*p.Height*4 {
		return nil, 0, 0, 0, 0, false
	}
	if g.messageFontImg == nil {
		g.messageFontImg = make(map[rune]*ebiten.Image, 96)
	}
	if img, ok := g.messageFontImg[ch]; ok {
		return img, p.Width, p.Height, p.OffsetX, p.OffsetY, true
	}
	g.debugImageAlloc(fmt.Sprintf("message-font:%d", ch), p.Width, p.Height)
	img := newDebugImage("statusbar:font:"+string(ch), p.Width, p.Height)
	img.WritePixels(p.RGBA)
	g.messageFontImg[ch] = img
	return img, p.Width, p.Height, p.OffsetX, p.OffsetY, true
}

func (g *game) drawHUDMessage(screen *ebiten.Image, msg string, x, y float64) {
	hud.DrawHUDMessage(screen, hud.MessageInputs{
		ViewW:      g.viewW,
		ViewH:      g.viewH,
		SourcePort: g.hudUsesLogicalLayout(),
		HUDScale:   g.hudScaleValue(),
		Message:    msg,
		X:          float64(huMsgX) + x,
		Y:          float64(huMsgY) + y,
	}, g.drawHUTextAt)
}

func (g *game) statusWeaponOwned(slot int) bool {
	switch slot {
	case 2:
		return true
	case 3:
		return g.weaponOwned(weaponShotgun) || g.weaponOwned(weaponSuperShotgun)
	case 4:
		return g.weaponOwned(weaponChaingun)
	case 5:
		return g.weaponOwned(weaponRocketLauncher)
	case 6:
		return g.weaponOwned(weaponPlasma)
	case 7:
		return g.weaponOwned(weaponBFG)
	default:
		return false
	}
}

func (g *game) statusReadyAmmo() (int, bool) {
	switch g.inventory.ReadyWeapon {
	case weaponPistol, weaponChaingun:
		return g.stats.Bullets, true
	case weaponShotgun, weaponSuperShotgun:
		return g.stats.Shells, true
	case weaponRocketLauncher:
		return g.stats.Rockets, true
	case weaponPlasma, weaponBFG:
		return g.stats.Cells, true
	default:
		return 0, false
	}
}

func (g *game) initStatusFaceState() {
	g.statusFaceIndex = 0
	g.statusFaceCount = 0
	g.statusFacePriority = 0
	g.statusOldHealth = -1
	g.statusLastAttack = -1
	g.statusAttackDown = false
	g.statusAttackerThing = -1
	g.statusHasAttacker = false
	g.statusDamageCount = 0
	g.statusBonusCount = 0
	g.statusOldWeapons = g.statusOwnedWeapons()
}

func (g *game) statusOwnedWeapons() [9]bool {
	var owned [9]bool
	owned[0] = g.weaponOwned(weaponChainsaw)
	owned[1] = g.weaponOwned(weaponFist)
	owned[2] = g.weaponOwned(weaponPistol)
	owned[3] = g.weaponOwned(weaponShotgun)
	owned[4] = g.weaponOwned(weaponSuperShotgun)
	owned[5] = g.weaponOwned(weaponChaingun)
	owned[6] = g.weaponOwned(weaponRocketLauncher)
	owned[7] = g.weaponOwned(weaponPlasma)
	owned[8] = g.weaponOwned(weaponBFG)
	return owned
}

func (g *game) tickStatusWidgets() {
	g.statusRandom = doomrand.MRandom()
	g.statusUpdateFaceWidget()
	g.statusOldHealth = g.stats.Health
	if g.statusDamageCount > 0 {
		g.statusDamageCount--
	}
	if g.statusBonusCount > 0 {
		g.statusBonusCount--
	}
	if g.statusDamageCount <= 0 && !g.isDead {
		g.statusAttackerThing = -1
		g.statusHasAttacker = false
	}
}

func (g *game) statusCalcPainOffset() int {
	health := g.stats.Health
	if health > 100 {
		health = 100
	}
	if health < 0 {
		health = 0
	}
	return statusFaceStride * (((100 - health) * statusNumPainFaces) / 101)
}

func (g *game) statusUpdateFaceWidget() {
	priority := g.statusFacePriority

	if priority < 10 {
		if g.stats.Health <= 0 || g.isDead {
			priority = 9
			g.statusFaceIndex = statusDeadFace
			g.statusFaceCount = 1
		}
	}

	if priority < 9 {
		if g.statusBonusCount > 0 {
			doEvilGrin := false
			owned := g.statusOwnedWeapons()
			for i := range owned {
				if g.statusOldWeapons[i] != owned[i] {
					doEvilGrin = true
					g.statusOldWeapons[i] = owned[i]
				}
			}
			if doEvilGrin {
				priority = 8
				g.statusFaceCount = statusEvilGrinCount
				g.statusFaceIndex = g.statusCalcPainOffset() + statusEvilGrinOffset
			}
		}
	}

	if priority < 8 {
		if g.statusDamageCount > 0 && g.statusHasAttacker {
			priority = 7
			if g.stats.Health-g.statusOldHealth > statusMuchPain {
				g.statusFaceCount = statusTurnCount
				g.statusFaceIndex = g.statusCalcPainOffset() + statusOuchOffset
			} else {
				dx := float64(g.statusAttackerX - g.p.x)
				dy := float64(g.statusAttackerY - g.p.y)
				ang := math.Atan2(dy, dx)
				if ang < 0 {
					ang += 2 * math.Pi
				}
				badguyangle := uint32(ang * (4294967296.0 / (2 * math.Pi)))
				var diffang uint32
				turnRight := false
				if badguyangle > g.p.angle {
					diffang = badguyangle - g.p.angle
					turnRight = diffang > statusAng180
				} else {
					diffang = g.p.angle - badguyangle
					turnRight = diffang <= statusAng180
				}
				g.statusFaceCount = statusTurnCount
				g.statusFaceIndex = g.statusCalcPainOffset()
				if diffang < statusAng45 {
					g.statusFaceIndex += statusRampageOffset
				} else if turnRight {
					g.statusFaceIndex += statusTurnOffset
				} else {
					g.statusFaceIndex += statusTurnOffset + 1
				}
			}
		}
	}

	if priority < 7 {
		if g.statusDamageCount > 0 {
			if g.stats.Health-g.statusOldHealth > statusMuchPain {
				priority = 7
				g.statusFaceCount = statusTurnCount
				g.statusFaceIndex = g.statusCalcPainOffset() + statusOuchOffset
			} else {
				priority = 6
				g.statusFaceCount = statusTurnCount
				g.statusFaceIndex = g.statusCalcPainOffset() + statusRampageOffset
			}
		}
	}

	if priority < 6 {
		if g.statusAttackDown {
			if g.statusLastAttack == -1 {
				g.statusLastAttack = statusRampageDelay
			} else {
				g.statusLastAttack--
				if g.statusLastAttack == 0 {
					priority = 5
					g.statusFaceIndex = g.statusCalcPainOffset() + statusRampageOffset
					g.statusFaceCount = 1
					g.statusLastAttack = 1
				}
			}
		} else {
			g.statusLastAttack = -1
		}
	}

	if priority < 5 {
		if g.playerInvulnerable() {
			priority = 4
			g.statusFaceIndex = statusGodFace
			g.statusFaceCount = 1
		}
	}

	if g.statusFaceCount == 0 {
		g.statusFaceIndex = g.statusCalcPainOffset() + (g.statusRandom % 3)
		g.statusFaceCount = statusStraightFaceCount
		priority = 0
	}
	g.statusFaceCount--
	g.statusFacePriority = priority
}

func (g *game) statusFacePatchName() string {
	switch g.statusFaceIndex {
	case statusDeadFace:
		return "STFDEAD0"
	case statusGodFace:
		return "STFGOD0"
	}
	if g.statusFaceIndex < 0 || g.statusFaceIndex >= statusGodFace {
		return "STFST00"
	}
	pain := g.statusFaceIndex / statusFaceStride
	ofs := g.statusFaceIndex % statusFaceStride
	switch {
	case ofs < statusNumStraightFaces:
		return fmt.Sprintf("STFST%d%d", pain, ofs)
	case ofs == statusTurnOffset:
		return fmt.Sprintf("STFTR%d0", pain)
	case ofs == statusTurnOffset+1:
		return fmt.Sprintf("STFTL%d0", pain)
	case ofs == statusOuchOffset:
		return fmt.Sprintf("STFOUCH%d", pain)
	case ofs == statusEvilGrinOffset:
		return fmt.Sprintf("STFEVL%d", pain)
	case ofs == statusRampageOffset:
		return fmt.Sprintf("STFKILL%d", pain)
	default:
		return "STFST00"
	}
}
