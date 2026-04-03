package doomruntime

import (
	"fmt"
	"strings"
	"unicode"

	"gddoom/internal/mapdata"
)

const typedCheatBufferLimit = 24

func normalizeCheatLevel(v int) int {
	if v < 0 {
		return 0
	}
	if v > 3 {
		return 3
	}
	return v
}

func (g *game) applyCheatLevel(level int, announce bool) {
	g.cheatLevel = normalizeCheatLevel(level)
	switch g.cheatLevel {
	case 0:
		if announce {
			g.setHUDMessage("Cheats OFF", 70)
		}
	case 1:
		g.parity.reveal = revealAllMap
		g.parity.iddt = 2
		if announce {
			g.setHUDMessage("IDDT + allmap", 70)
		}
	case 2:
		g.parity.reveal = revealAllMap
		g.parity.iddt = 2
		g.grantIDFA()
		if announce {
			g.setHUDMessage("IDFA", 70)
		}
	case 3:
		g.parity.reveal = revealAllMap
		g.parity.iddt = 2
		g.grantIDKFA()
		g.invulnerable = true
		if announce {
			g.setHUDMessage("IDKFA + IDDQD", 70)
		}
	}
}

func (g *game) grantIDFA() {
	if g.inventory.Weapons == nil {
		g.inventory.Weapons = map[int16]bool{}
	}
	g.inventory.Weapons[2001] = true
	g.inventory.Weapons[2002] = true
	g.inventory.Weapons[2003] = true
	g.inventory.Weapons[2004] = true
	g.inventory.Weapons[2005] = true
	g.inventory.Weapons[2006] = true
	maxBullets, maxShells, maxRockets, maxCells := ammoCaps(g.inventory.Backpack)
	g.stats.Bullets = maxBullets
	g.stats.Shells = maxShells
	g.stats.Rockets = maxRockets
	g.stats.Cells = maxCells
	if g.stats.Health < 100 {
		g.stats.Health = 100
		g.syncPlayerMobjHealth()
	}
	if g.stats.Armor < 200 {
		g.stats.Armor = 200
	}
	g.ensureWeaponHasAmmo()
}

func (g *game) grantIDKFA() {
	g.grantIDFA()
	g.inventory.BlueKey = true
	g.inventory.RedKey = true
	g.inventory.YellowKey = true
}

func (g *game) consumeTypedCheatInput() {
	if g == nil || len(g.input.inputChars) == 0 {
		return
	}
	for _, r := range g.input.inputChars {
		if r > unicode.MaxASCII {
			continue
		}
		r = unicode.ToLower(r)
		if (r < 'a' || r > 'z') && (r < '0' || r > '9') {
			continue
		}
		g.typedCheatBuffer += string(r)
		if len(g.typedCheatBuffer) > typedCheatBufferLimit {
			g.typedCheatBuffer = g.typedCheatBuffer[len(g.typedCheatBuffer)-typedCheatBufferLimit:]
		}
		g.applyTypedCheatSuffix()
	}
}

func (g *game) applyTypedCheatSuffix() {
	switch {
	case g.tryTypedIDCLEV():
	case g.tryTypedIDMUS():
	case g.consumeTypedCheat("idbeholdv"):
		fmt.Println("typed cheat: idbeholdv")
		g.applyBeholdCheat('v')
	case g.consumeTypedCheat("idbeholds"):
		fmt.Println("typed cheat: idbeholds")
		g.applyBeholdCheat('s')
	case g.consumeTypedCheat("idbeholdi"):
		fmt.Println("typed cheat: idbeholdi")
		g.applyBeholdCheat('i')
	case g.consumeTypedCheat("idbeholdr"):
		fmt.Println("typed cheat: idbeholdr")
		g.applyBeholdCheat('r')
	case g.consumeTypedCheat("idbeholda"):
		fmt.Println("typed cheat: idbeholda")
		g.applyBeholdCheat('a')
	case g.consumeTypedCheat("idbeholdl"):
		fmt.Println("typed cheat: idbeholdl")
		g.applyBeholdCheat('l')
	case g.peekTypedCheat("idbehold"):
		fmt.Println("typed cheat: idbehold")
		g.setHUDMessage("inVuln, Str, Inviso, Rad, Allmap, or Lite-amp", 70)
	case g.consumeTypedCheat("iddqd"):
		fmt.Println("typed cheat: iddqd")
		g.invulnerable = !g.invulnerable
		if g.invulnerable {
			g.setHUDMessage("IDDQD", 70)
		} else {
			g.setHUDMessage("IDDQD OFF", 70)
		}
	case g.consumeTypedCheat("idclip"):
		fmt.Println("typed cheat: idclip")
		g.toggleNoClip()
	case g.consumeTypedCheat("idspispopd"):
		fmt.Println("typed cheat: idspispopd")
		g.toggleNoClip()
	case g.consumeTypedCheat("idmypos"):
		fmt.Println("typed cheat: idmypos")
		g.reportPlayerPosition()
	case g.consumeTypedCheat("idchoppers"):
		fmt.Println("typed cheat: idchoppers")
		g.applyIDChoppers()
	case g.consumeTypedCheat("idkfa"):
		fmt.Println("typed cheat: idkfa")
		g.grantIDKFA()
		g.setHUDMessage("IDKFA", 70)
	case g.consumeTypedCheat("idfa"):
		fmt.Println("typed cheat: idfa")
		g.grantIDFA()
		g.setHUDMessage("IDFA", 70)
	case g.consumeTypedCheat("iddt"):
		fmt.Println("typed cheat: iddt")
		g.cycleTypedIDDT()
	}
}

func (g *game) tryTypedIDCLEV() bool {
	if g == nil {
		return false
	}
	const prefix = "idclev"
	if len(g.typedCheatBuffer) < len(prefix)+2 || !strings.HasSuffix(g.typedCheatBuffer[:len(g.typedCheatBuffer)-2], prefix) {
		return false
	}
	suffix := g.typedCheatBuffer[len(g.typedCheatBuffer)-2:]
	if suffix[0] < '0' || suffix[0] > '9' || suffix[1] < '0' || suffix[1] > '9' {
		return false
	}
	target, ok := g.idclevTargetMap(suffix)
	g.typedCheatBuffer = ""
	if !ok {
		fmt.Printf("typed cheat: idclev%s (bad map)\n", suffix)
		g.setHUDMessage("IDCLEV BAD MAP", 70)
		return true
	}
	if err := g.requestCheatWarp(target); err != nil {
		fmt.Printf("typed cheat: idclev%s (load failed: %v)\n", suffix, err)
		g.setHUDMessage("IDCLEV BAD MAP", 70)
		return true
	}
	fmt.Printf("typed cheat: idclev%s -> %s\n", suffix, target)
	g.setHUDMessage(fmt.Sprintf("IDCLEV %s", target), 70)
	return true
}

func (g *game) tryTypedIDMUS() bool {
	if g == nil {
		return false
	}
	const prefix = "idmus"
	if len(g.typedCheatBuffer) < len(prefix)+2 || !strings.HasSuffix(g.typedCheatBuffer[:len(g.typedCheatBuffer)-2], prefix) {
		return false
	}
	suffix := g.typedCheatBuffer[len(g.typedCheatBuffer)-2:]
	if suffix[0] < '0' || suffix[0] > '9' || suffix[1] < '0' || suffix[1] > '9' {
		return false
	}
	g.typedCheatBuffer = ""
	if g.opts.PlayCheatMusic == nil {
		fmt.Printf("typed cheat: idmus%s (music unavailable)\n", suffix)
		g.setHUDMessage("IMPOSSIBLE SELECTION", 70)
		return true
	}
	ok, err := g.opts.PlayCheatMusic(g.currentMapName(), suffix)
	if err != nil {
		fmt.Printf("typed cheat: idmus%s (load failed: %v)\n", suffix, err)
		g.setHUDMessage("IMPOSSIBLE SELECTION", 70)
		return true
	}
	if !ok {
		fmt.Printf("typed cheat: idmus%s (bad selection)\n", suffix)
		g.setHUDMessage("IMPOSSIBLE SELECTION", 70)
		return true
	}
	fmt.Printf("typed cheat: idmus%s\n", suffix)
	g.setHUDMessage("Music Change", 70)
	return true
}

func (g *game) applyBeholdCheat(kind byte) {
	if g == nil {
		return
	}
	switch kind {
	case 'v':
		if g.inventory.InvulnTics <= 0 {
			g.inventory.InvulnTics = 30 * doomTicsPerSecond
		} else {
			g.inventory.InvulnTics = 1
		}
	case 's':
		if !g.inventory.Strength {
			if g.stats.Health < 100 {
				g.stats.Health = 100
				g.syncPlayerMobjHealth()
			}
			g.inventory.Strength = true
			g.inventory.StrengthCount = 1
		} else {
			g.inventory.Strength = false
			g.inventory.StrengthCount = 0
		}
	case 'i':
		if g.inventory.InvisTics <= 0 {
			g.inventory.InvisTics = 60 * doomTicsPerSecond
		} else {
			g.inventory.InvisTics = 1
		}
	case 'r':
		if g.inventory.RadSuitTics <= 0 {
			g.inventory.RadSuitTics = 60 * doomTicsPerSecond
		} else {
			g.inventory.RadSuitTics = 1
		}
	case 'a':
		g.inventory.AllMap = !g.inventory.AllMap
	case 'l':
		if g.inventory.LightAmpTics <= 0 {
			g.inventory.LightAmpTics = 120 * doomTicsPerSecond
		} else {
			g.inventory.LightAmpTics = 1
		}
	default:
		return
	}
	g.setHUDMessage("Power-up Toggled", 70)
}

func (g *game) idclevTargetMap(digits string) (string, bool) {
	if len(digits) != 2 {
		return "", false
	}
	if _, _, ok := episodeMapSlot(mapdata.MapName(g.currentMapName())); ok {
		mapSlot := int(digits[1] - '0')
		if digits[0] < '1' || digits[0] > '9' || mapSlot < 1 || mapSlot > 9 {
			return "", false
		}
		return fmt.Sprintf("E%cM%d", digits[0], mapSlot), true
	}
	if strings.HasPrefix(strings.ToUpper(strings.TrimSpace(string(g.currentMapName()))), "MAP") {
		if digits == "00" {
			return "", false
		}
		return "MAP" + digits, true
	}
	return "", false
}

func (g *game) currentMapName() string {
	if g == nil || g.m == nil {
		return ""
	}
	return strings.ToUpper(strings.TrimSpace(string(g.m.Name)))
}

func (g *game) requestCheatWarp(target string) error {
	if g == nil || g.opts.NewGameLoader == nil {
		return fmt.Errorf("new game loader unavailable")
	}
	m, err := g.opts.NewGameLoader(target)
	if err != nil || m == nil {
		if err == nil {
			err = fmt.Errorf("nil map")
		}
		return err
	}
	g.newGameRequestedMap = m
	if g.opts.SkillLevel > 0 {
		g.newGameRequestedSkill = g.opts.SkillLevel
	} else {
		g.newGameRequestedSkill = 3
	}
	return nil
}

func (g *game) consumeTypedCheat(code string) bool {
	if g == nil || code == "" || !strings.HasSuffix(g.typedCheatBuffer, code) {
		return false
	}
	g.typedCheatBuffer = ""
	return true
}

func (g *game) peekTypedCheat(code string) bool {
	return g != nil && code != "" && strings.HasSuffix(g.typedCheatBuffer, code)
}

func (g *game) cycleTypedIDDT() {
	if g == nil {
		return
	}
	switch {
	case g.parity.reveal != revealAllMap:
		g.parity.reveal = revealAllMap
		g.parity.iddt = 0
	case g.parity.iddt < 2:
		g.parity.iddt++
	default:
		g.parity.reveal = revealNormal
		g.parity.iddt = 0
	}
	if g.parity.reveal == revealNormal {
		g.setHUDMessage("IDDT OFF", 70)
		return
	}
	g.setHUDMessage(fmt.Sprintf("IDDT %d", g.parity.iddt), 70)
}

func (g *game) toggleNoClip() {
	if g == nil {
		return
	}
	g.noClip = !g.noClip
	if g.noClip {
		g.setHUDMessage("IDCLIP ON", 70)
		return
	}
	g.setHUDMessage("IDCLIP OFF", 70)
}

func (g *game) reportPlayerPosition() {
	if g == nil {
		return
	}
	msg := fmt.Sprintf("ang=0x%x;x,y=(0x%x,0x%x)", g.p.angle, uint32(g.p.x), uint32(g.p.y))
	fmt.Println(msg)
	g.setHUDMessage(msg, 70)
}

func (g *game) applyIDChoppers() {
	if g == nil {
		return
	}
	if g.inventory.Weapons == nil {
		g.inventory.Weapons = map[int16]bool{}
	}
	g.inventory.Weapons[2005] = true
	// Match Doom's direct cheat behavior in st_stuff.c: set the power truthy
	// instead of routing through the normal pickup duration.
	g.inventory.InvulnTics = 1
	g.setHUDMessage("... doesn't suck - GM", 70)
}
