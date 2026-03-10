package automap

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
	huFontStart = '!' // HU_FONTSTART
	huFontEnd   = '_' // HU_FONTEND
	huMsgX      = 0   // HU_MSGX
	huMsgY      = 0   // HU_MSGY
)

func (g *game) drawDoomStatusBar(screen *ebiten.Image) {
	if len(g.opts.StatusPatchBank) == 0 {
		return
	}
	maxBullets, maxShells, maxRockets, maxCells := ammoCaps(g.inventory.Backpack)
	readyAmmo, hasReadyAmmo := g.statusReadyAmmo()
	hud.DrawStatusBar(screen, hud.StatusBarInputs{
		ViewW:        g.viewW,
		ViewH:        g.viewH,
		SourcePort:   g.opts.SourcePortMode,
		Health:       g.stats.Health,
		Armor:        g.stats.Armor,
		ReadyAmmo:    readyAmmo,
		HasReadyAmmo: hasReadyAmmo,
		WeaponOwned: [6]bool{
			g.statusWeaponOwned(2),
			g.statusWeaponOwned(3),
			g.statusWeaponOwned(4),
			g.statusWeaponOwned(5),
			g.statusWeaponOwned(6),
			g.statusWeaponOwned(7),
		},
		KeyOn:     [3]bool{g.inventory.BlueKey, g.inventory.RedKey, g.inventory.YellowKey},
		AmmoCur:   [4]int{g.stats.Bullets, g.stats.Shells, g.stats.Cells, g.stats.Rockets},
		AmmoMax:   [4]int{maxBullets, maxShells, maxCells, maxRockets},
		FacePatch: g.statusFacePatchName(),
	}, func(screen *ebiten.Image, name string, x, y, sx, sy float64) bool {
		g.drawStatusPatch(screen, name, x, y, sx, sy)
		return true
	}, g.drawStatusTallNum, g.drawStatusShortNum, g.drawStatusPercent)
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
	img := ebiten.NewImage(p.Width, p.Height)
	img.WritePixels(p.RGBA)
	g.statusPatchImg[key] = img
	return img, p.Width, p.Height, p.OffsetX, p.OffsetY, true
}

func (g *game) drawStatusPatch(screen *ebiten.Image, name string, x, y, sx, sy float64) {
	img, _, _, ox, oy, ok := g.statusPatch(name)
	if !ok {
		return
	}
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Scale(sx, sy)
	op.GeoM.Translate(x-float64(ox)*sx, y-float64(oy)*sy)
	screen.DrawImage(img, op)
}

func (g *game) drawStatusTallNum(screen *ebiten.Image, value, digits int, rightX, y, sx, sy float64) {
	if value < 0 {
		value = 0
	}
	s := strconv.Itoa(value)
	if len(s) > digits {
		s = s[len(s)-digits:]
	}
	x := rightX
	for i := len(s) - 1; i >= 0; i-- {
		name := "STTNUM" + string(s[i])
		_, w, _, _, _, ok := g.statusPatch(name)
		if !ok {
			continue
		}
		x -= float64(w) * sx
		g.drawStatusPatch(screen, name, x, y, sx, sy)
	}
}

func (g *game) drawStatusShortNum(screen *ebiten.Image, value, digits int, rightX, y, sx, sy float64) {
	if value < 0 {
		value = 0
	}
	s := strconv.Itoa(value)
	if len(s) > digits {
		s = s[len(s)-digits:]
	}
	x := rightX
	for i := len(s) - 1; i >= 0; i-- {
		name := "STYSNUM" + string(s[i])
		_, w, _, _, _, ok := g.statusPatch(name)
		if !ok {
			continue
		}
		x -= float64(w) * sx
		g.drawStatusPatch(screen, name, x, y, sx, sy)
	}
}

func (g *game) drawStatusPercent(screen *ebiten.Image, value int, x, y, sx, sy float64) {
	_, _, _, _, _, ok := g.statusPatch("STTPRCNT")
	if ok {
		g.drawStatusPatch(screen, "STTPRCNT", x, y, sx, sy)
		g.drawStatusTallNum(screen, value, 3, x, y, sx, sy)
		return
	}
	g.drawStatusTallNum(screen, value, 3, x, y, sx, sy)
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
	img := ebiten.NewImage(p.Width, p.Height)
	img.WritePixels(p.RGBA)
	g.messageFontImg[ch] = img
	return img, p.Width, p.Height, p.OffsetX, p.OffsetY, true
}

func (g *game) drawHUDMessage(screen *ebiten.Image, msg string, x, y float64) {
	hud.DrawHUDMessage(screen, hud.MessageInputs{
		ViewW:      g.viewW,
		ViewH:      g.viewH,
		SourcePort: g.opts.SourcePortMode,
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
		return g.inventory.Weapons[2001]
	case 4:
		return g.inventory.Weapons[2002]
	case 5:
		return g.inventory.Weapons[2003]
	case 6:
		return g.inventory.Weapons[2004]
	case 7:
		return g.inventory.Weapons[2006]
	default:
		return false
	}
}

func (g *game) statusReadyAmmo() (int, bool) {
	switch g.inventory.ReadyWeapon {
	case weaponPistol, weaponChaingun:
		return g.stats.Bullets, true
	case weaponShotgun:
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
	g.statusHasAttacker = false
	g.statusDamageCount = 0
	g.statusBonusCount = 0
	g.statusOldWeapons = g.statusOwnedWeapons()
}

func (g *game) statusOwnedWeapons() [8]bool {
	var owned [8]bool
	owned[0] = g.inventory.Weapons[2005] // chainsaw
	owned[1] = true                      // fist
	owned[2] = true                      // pistol
	owned[3] = g.inventory.Weapons[2001] // shotgun
	owned[4] = g.inventory.Weapons[2002] // chaingun
	owned[5] = g.inventory.Weapons[2003] // rocket launcher
	owned[6] = g.inventory.Weapons[2004] // plasma
	owned[7] = g.inventory.Weapons[2006] // BFG
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
	if g.statusDamageCount <= 0 {
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
		if g.invulnerable {
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
