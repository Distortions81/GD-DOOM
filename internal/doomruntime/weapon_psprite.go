package doomruntime

import (
	"fmt"
	"image"
	"math"
	"os"
	"strings"

	"gddoom/internal/render/scene"

	"github.com/hajimehoshi/ebiten/v2"
)

const (
	weaponRaiseSpeed = 6
	weaponLowerSpeed = 6
	weaponTopY       = 32
	weaponBottomY    = 128
)

type weaponPspriteState int

const (
	weaponStateNone weaponPspriteState = iota
	weaponStateFistReady
	weaponStateFistDown
	weaponStateFistUp
	weaponStateFistAtk1
	weaponStateFistAtk2
	weaponStateFistAtk3
	weaponStateFistAtk4
	weaponStateFistAtk5
	weaponStatePistolReady
	weaponStatePistolDown
	weaponStatePistolUp
	weaponStatePistolAtk1
	weaponStatePistolAtk2
	weaponStatePistolAtk3
	weaponStatePistolAtk4
	weaponStateShotgunReady
	weaponStateShotgunDown
	weaponStateShotgunUp
	weaponStateShotgunAtk1
	weaponStateShotgunAtk2
	weaponStateShotgunAtk3
	weaponStateShotgunAtk4
	weaponStateShotgunAtk5
	weaponStateShotgunAtk6
	weaponStateShotgunAtk7
	weaponStateShotgunAtk8
	weaponStateShotgunAtk9
	weaponStateSuperShotgunReady
	weaponStateSuperShotgunDown
	weaponStateSuperShotgunUp
	weaponStateSuperShotgunAtk1
	weaponStateSuperShotgunAtk2
	weaponStateSuperShotgunAtk3
	weaponStateSuperShotgunAtk4
	weaponStateSuperShotgunAtk5
	weaponStateSuperShotgunAtk6
	weaponStateSuperShotgunAtk7
	weaponStateSuperShotgunAtk8
	weaponStateSuperShotgunAtk9
	weaponStateSuperShotgunAtk10
	weaponStateChaingunReady
	weaponStateChaingunDown
	weaponStateChaingunUp
	weaponStateChaingunAtk1
	weaponStateChaingunAtk2
	weaponStateChaingunAtk3
	weaponStateRocketReady
	weaponStateRocketDown
	weaponStateRocketUp
	weaponStateRocketAtk1
	weaponStateRocketAtk2
	weaponStateRocketAtk3
	weaponStateSawReady
	weaponStateSawReadyB
	weaponStateSawDown
	weaponStateSawUp
	weaponStateSawAtk1
	weaponStateSawAtk2
	weaponStateSawAtk3
	weaponStatePlasmaReady
	weaponStatePlasmaDown
	weaponStatePlasmaUp
	weaponStatePlasmaAtk1
	weaponStatePlasmaAtk2
	weaponStateBFGReady
	weaponStateBFGDown
	weaponStateBFGUp
	weaponStateBFGAtk1
	weaponStateBFGAtk2
	weaponStateBFGAtk3
	weaponStateBFGAtk4
	weaponStatePistolFlash
	weaponStateShotgunFlash1
	weaponStateShotgunFlash2
	weaponStateSuperShotgunFlash1
	weaponStateSuperShotgunFlash2
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

type weaponPspriteAction uint8

const (
	weaponPspriteActionNone weaponPspriteAction = iota
	weaponPspriteActionReady
	weaponPspriteActionLower
	weaponPspriteActionRaise
	weaponPspriteActionPunch
	weaponPspriteActionFirePistol
	weaponPspriteActionFireShotgun
	weaponPspriteActionFireSuperShotgun
	weaponPspriteActionFireChaingun
	weaponPspriteActionGunFlash
	weaponPspriteActionFireMissile
	weaponPspriteActionSaw
	weaponPspriteActionFirePlasma
	weaponPspriteActionBFGSound
	weaponPspriteActionFireBFG
	weaponPspriteActionReFire
	weaponPspriteActionCheckReload
	weaponPspriteActionOpenShotgun2
	weaponPspriteActionLoadShotgun2
	weaponPspriteActionCloseShotgun2
	weaponPspriteActionLight0
	weaponPspriteActionLight1
	weaponPspriteActionLight2
)

type weaponPspriteDef struct {
	sprite string
	tics   int
	next   weaponPspriteState
	action weaponPspriteAction
}

var weaponPspriteDefs = map[weaponPspriteState]weaponPspriteDef{
	weaponStateFistReady:          {sprite: "PUNGA0", tics: 1, next: weaponStateFistReady, action: weaponPspriteActionReady},
	weaponStateFistDown:           {sprite: "PUNGA0", tics: 1, next: weaponStateFistDown, action: weaponPspriteActionLower},
	weaponStateFistUp:             {sprite: "PUNGA0", tics: 1, next: weaponStateFistUp, action: weaponPspriteActionRaise},
	weaponStateFistAtk1:           {sprite: "PUNGB0", tics: 4, next: weaponStateFistAtk2},
	weaponStateFistAtk2:           {sprite: "PUNGC0", tics: 4, next: weaponStateFistAtk3, action: weaponPspriteActionPunch},
	weaponStateFistAtk3:           {sprite: "PUNGD0", tics: 5, next: weaponStateFistAtk4},
	weaponStateFistAtk4:           {sprite: "PUNGC0", tics: 4, next: weaponStateFistAtk5},
	weaponStateFistAtk5:           {sprite: "PUNGB0", tics: 5, next: weaponStateFistReady, action: weaponPspriteActionReFire},
	weaponStatePistolReady:        {sprite: "PISGA0", tics: 1, next: weaponStatePistolReady, action: weaponPspriteActionReady},
	weaponStatePistolDown:         {sprite: "PISGA0", tics: 1, next: weaponStatePistolDown, action: weaponPspriteActionLower},
	weaponStatePistolUp:           {sprite: "PISGA0", tics: 1, next: weaponStatePistolUp, action: weaponPspriteActionRaise},
	weaponStatePistolAtk1:         {sprite: "PISGA0", tics: 4, next: weaponStatePistolAtk2},
	weaponStatePistolAtk2:         {sprite: "PISGB0", tics: 6, next: weaponStatePistolAtk3, action: weaponPspriteActionFirePistol},
	weaponStatePistolAtk3:         {sprite: "PISGC0", tics: 4, next: weaponStatePistolAtk4},
	weaponStatePistolAtk4:         {sprite: "PISGB0", tics: 5, next: weaponStatePistolReady, action: weaponPspriteActionReFire},
	weaponStateShotgunReady:       {sprite: "SHTGA0", tics: 1, next: weaponStateShotgunReady, action: weaponPspriteActionReady},
	weaponStateShotgunDown:        {sprite: "SHTGA0", tics: 1, next: weaponStateShotgunDown, action: weaponPspriteActionLower},
	weaponStateShotgunUp:          {sprite: "SHTGA0", tics: 1, next: weaponStateShotgunUp, action: weaponPspriteActionRaise},
	weaponStateShotgunAtk1:        {sprite: "SHTGA0", tics: 3, next: weaponStateShotgunAtk2},
	weaponStateShotgunAtk2:        {sprite: "SHTGA0", tics: 7, next: weaponStateShotgunAtk3, action: weaponPspriteActionFireShotgun},
	weaponStateShotgunAtk3:        {sprite: "SHTGB0", tics: 5, next: weaponStateShotgunAtk4},
	weaponStateShotgunAtk4:        {sprite: "SHTGC0", tics: 5, next: weaponStateShotgunAtk5},
	weaponStateShotgunAtk5:        {sprite: "SHTGD0", tics: 4, next: weaponStateShotgunAtk6},
	weaponStateShotgunAtk6:        {sprite: "SHTGC0", tics: 5, next: weaponStateShotgunAtk7},
	weaponStateShotgunAtk7:        {sprite: "SHTGB0", tics: 5, next: weaponStateShotgunAtk8},
	weaponStateShotgunAtk8:        {sprite: "SHTGA0", tics: 3, next: weaponStateShotgunAtk9},
	weaponStateShotgunAtk9:        {sprite: "SHTGA0", tics: 7, next: weaponStateShotgunReady, action: weaponPspriteActionReFire},
	weaponStateSuperShotgunReady:  {sprite: "SHT2A0", tics: 1, next: weaponStateSuperShotgunReady, action: weaponPspriteActionReady},
	weaponStateSuperShotgunDown:   {sprite: "SHT2A0", tics: 1, next: weaponStateSuperShotgunDown, action: weaponPspriteActionLower},
	weaponStateSuperShotgunUp:     {sprite: "SHT2A0", tics: 1, next: weaponStateSuperShotgunUp, action: weaponPspriteActionRaise},
	weaponStateSuperShotgunAtk1:   {sprite: "SHT2A0", tics: 3, next: weaponStateSuperShotgunAtk2},
	weaponStateSuperShotgunAtk2:   {sprite: "SHT2A0", tics: 7, next: weaponStateSuperShotgunAtk3, action: weaponPspriteActionFireSuperShotgun},
	weaponStateSuperShotgunAtk3:   {sprite: "SHT2B0", tics: 7, next: weaponStateSuperShotgunAtk4},
	weaponStateSuperShotgunAtk4:   {sprite: "SHT2C0", tics: 7, next: weaponStateSuperShotgunAtk5, action: weaponPspriteActionCheckReload},
	weaponStateSuperShotgunAtk5:   {sprite: "SHT2D0", tics: 7, next: weaponStateSuperShotgunAtk6, action: weaponPspriteActionOpenShotgun2},
	weaponStateSuperShotgunAtk6:   {sprite: "SHT2E0", tics: 7, next: weaponStateSuperShotgunAtk7},
	weaponStateSuperShotgunAtk7:   {sprite: "SHT2F0", tics: 7, next: weaponStateSuperShotgunAtk8, action: weaponPspriteActionLoadShotgun2},
	weaponStateSuperShotgunAtk8:   {sprite: "SHT2G0", tics: 6, next: weaponStateSuperShotgunAtk9},
	weaponStateSuperShotgunAtk9:   {sprite: "SHT2H0", tics: 6, next: weaponStateSuperShotgunAtk10, action: weaponPspriteActionCloseShotgun2},
	weaponStateSuperShotgunAtk10:  {sprite: "SHT2A0", tics: 5, next: weaponStateSuperShotgunReady, action: weaponPspriteActionReFire},
	weaponStateChaingunReady:      {sprite: "CHGGA0", tics: 1, next: weaponStateChaingunReady, action: weaponPspriteActionReady},
	weaponStateChaingunDown:       {sprite: "CHGGA0", tics: 1, next: weaponStateChaingunDown, action: weaponPspriteActionLower},
	weaponStateChaingunUp:         {sprite: "CHGGA0", tics: 1, next: weaponStateChaingunUp, action: weaponPspriteActionRaise},
	weaponStateChaingunAtk1:       {sprite: "CHGGA0", tics: 4, next: weaponStateChaingunAtk2, action: weaponPspriteActionFireChaingun},
	weaponStateChaingunAtk2:       {sprite: "CHGGB0", tics: 4, next: weaponStateChaingunAtk3, action: weaponPspriteActionFireChaingun},
	weaponStateChaingunAtk3:       {sprite: "CHGGB0", tics: 0, next: weaponStateChaingunReady, action: weaponPspriteActionReFire},
	weaponStateRocketReady:        {sprite: "MISGA0", tics: 1, next: weaponStateRocketReady, action: weaponPspriteActionReady},
	weaponStateRocketDown:         {sprite: "MISGA0", tics: 1, next: weaponStateRocketDown, action: weaponPspriteActionLower},
	weaponStateRocketUp:           {sprite: "MISGA0", tics: 1, next: weaponStateRocketUp, action: weaponPspriteActionRaise},
	weaponStateRocketAtk1:         {sprite: "MISGB0", tics: 8, next: weaponStateRocketAtk2, action: weaponPspriteActionGunFlash},
	weaponStateRocketAtk2:         {sprite: "MISGB0", tics: 12, next: weaponStateRocketAtk3, action: weaponPspriteActionFireMissile},
	weaponStateRocketAtk3:         {sprite: "MISGB0", tics: 0, next: weaponStateRocketReady, action: weaponPspriteActionReFire},
	weaponStateSawReady:           {sprite: "SAWGC0", tics: 4, next: weaponStateSawReadyB, action: weaponPspriteActionReady},
	weaponStateSawReadyB:          {sprite: "SAWGD0", tics: 4, next: weaponStateSawReady, action: weaponPspriteActionReady},
	weaponStateSawDown:            {sprite: "SAWGC0", tics: 1, next: weaponStateSawDown, action: weaponPspriteActionLower},
	weaponStateSawUp:              {sprite: "SAWGC0", tics: 1, next: weaponStateSawUp, action: weaponPspriteActionRaise},
	weaponStateSawAtk1:            {sprite: "SAWGA0", tics: 4, next: weaponStateSawAtk2, action: weaponPspriteActionSaw},
	weaponStateSawAtk2:            {sprite: "SAWGB0", tics: 4, next: weaponStateSawAtk3, action: weaponPspriteActionSaw},
	weaponStateSawAtk3:            {sprite: "SAWGB0", tics: 0, next: weaponStateSawReady, action: weaponPspriteActionReFire},
	weaponStatePlasmaReady:        {sprite: "PLSGA0", tics: 1, next: weaponStatePlasmaReady, action: weaponPspriteActionReady},
	weaponStatePlasmaDown:         {sprite: "PLSGA0", tics: 1, next: weaponStatePlasmaDown, action: weaponPspriteActionLower},
	weaponStatePlasmaUp:           {sprite: "PLSGA0", tics: 1, next: weaponStatePlasmaUp, action: weaponPspriteActionRaise},
	weaponStatePlasmaAtk1:         {sprite: "PLSGA0", tics: 3, next: weaponStatePlasmaAtk2, action: weaponPspriteActionFirePlasma},
	weaponStatePlasmaAtk2:         {sprite: "PLSGB0", tics: 20, next: weaponStatePlasmaReady, action: weaponPspriteActionReFire},
	weaponStateBFGReady:           {sprite: "BFGGA0", tics: 1, next: weaponStateBFGReady, action: weaponPspriteActionReady},
	weaponStateBFGDown:            {sprite: "BFGGA0", tics: 1, next: weaponStateBFGDown, action: weaponPspriteActionLower},
	weaponStateBFGUp:              {sprite: "BFGGA0", tics: 1, next: weaponStateBFGUp, action: weaponPspriteActionRaise},
	weaponStateBFGAtk1:            {sprite: "BFGGA0", tics: 20, next: weaponStateBFGAtk2, action: weaponPspriteActionBFGSound},
	weaponStateBFGAtk2:            {sprite: "BFGGB0", tics: 10, next: weaponStateBFGAtk3, action: weaponPspriteActionGunFlash},
	weaponStateBFGAtk3:            {sprite: "BFGGB0", tics: 10, next: weaponStateBFGAtk4, action: weaponPspriteActionFireBFG},
	weaponStateBFGAtk4:            {sprite: "BFGGB0", tics: 20, next: weaponStateBFGReady, action: weaponPspriteActionReFire},
	weaponStatePistolFlash:        {sprite: "PISFA0", tics: 7, next: weaponStateNone, action: weaponPspriteActionLight1},
	weaponStateShotgunFlash1:      {sprite: "SHTFA0", tics: 4, next: weaponStateShotgunFlash2, action: weaponPspriteActionLight1},
	weaponStateShotgunFlash2:      {sprite: "SHTFB0", tics: 3, next: weaponStateNone, action: weaponPspriteActionLight2},
	weaponStateSuperShotgunFlash1: {sprite: "SHT2I0", tics: 5, next: weaponStateSuperShotgunFlash2, action: weaponPspriteActionLight1},
	weaponStateSuperShotgunFlash2: {sprite: "SHT2J0", tics: 4, next: weaponStateNone, action: weaponPspriteActionLight2},
	weaponStateChaingunFlash1:     {sprite: "CHGFA0", tics: 5, next: weaponStateNone, action: weaponPspriteActionLight1},
	weaponStateChaingunFlash2:     {sprite: "CHGFB0", tics: 5, next: weaponStateNone, action: weaponPspriteActionLight2},
	weaponStateRocketFlash1:       {sprite: "MISFA0", tics: 3, next: weaponStateRocketFlash2, action: weaponPspriteActionLight1},
	weaponStateRocketFlash2:       {sprite: "MISFB0", tics: 4, next: weaponStateRocketFlash3},
	weaponStateRocketFlash3:       {sprite: "MISFC0", tics: 4, next: weaponStateRocketFlash4, action: weaponPspriteActionLight2},
	weaponStateRocketFlash4:       {sprite: "MISFD0", tics: 4, next: weaponStateNone, action: weaponPspriteActionLight2},
	weaponStatePlasmaFlash1:       {sprite: "PLSFA0", tics: 4, next: weaponStateNone, action: weaponPspriteActionLight1},
	weaponStatePlasmaFlash2:       {sprite: "PLSFB0", tics: 4, next: weaponStateNone, action: weaponPspriteActionLight1},
	weaponStateBFGFlash1:          {sprite: "BFGFA0", tics: 11, next: weaponStateBFGFlash2, action: weaponPspriteActionLight1},
	weaponStateBFGFlash2:          {sprite: "BFGFB0", tics: 6, next: weaponStateNone, action: weaponPspriteActionLight2},
}

func weaponStateForReady(id weaponID) weaponPspriteState {
	return weaponInfo(id).readystate
}

func weaponStateForAttack(id weaponID) weaponPspriteState {
	return weaponInfo(id).atkstate
}

func flashStartState(id weaponID) weaponPspriteState {
	return weaponInfo(id).flashstate
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
	if !flash {
		if want := strings.TrimSpace(os.Getenv("GD_DEBUG_WEAPON_TIC")); want != "" {
			var wantTic int
			if _, err := fmt.Sscanf(want, "%d", &wantTic); err == nil {
				if g.demoTick-1 >= wantTic-2 && g.demoTick-1 <= wantTic+2 {
					fmt.Printf("gd-weapon-debug gametic=%d phase=pre state=%d tics=%d y=%d ready=%d pending=%d\n",
						g.demoTick-1, state, *tics, g.weaponPSpriteY, g.inventory.ReadyWeapon, g.inventory.PendingWeapon)
				}
			}
		}
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
	g.weaponPSpriteY = weaponTopY
}

func (g *game) startWeaponOverlayFire(id weaponID) {
	g.setWeaponPSpriteState(weaponStateForAttack(id), false)
}

func (g *game) startWeaponFlashState(state weaponPspriteState) {
	g.setWeaponPSpriteState(state, true)
}

func (g *game) bringUpWeapon() {
	g.ensureWeaponDefaults()
	next := g.inventory.PendingWeapon
	if next == 0 {
		next = g.inventory.ReadyWeapon
	}
	if next == 0 {
		next = weaponPistol
	}
	if next == weaponChainsaw {
		g.emitSoundEvent(soundEventSawUp)
	}
	if want := strings.TrimSpace(os.Getenv("GD_DEBUG_WEAPON_TIC")); want != "" {
		var wantTic int
		if _, err := fmt.Sscanf(want, "%d", &wantTic); err == nil {
			if g.demoTick-1 >= wantTic-2 && g.demoTick-1 <= wantTic+2 {
				fmt.Printf("gd-weapon-debug gametic=%d phase=bringup next=%d ready=%d pending=%d y=%d\n",
					g.demoTick-1, next, g.inventory.ReadyWeapon, g.inventory.PendingWeapon, g.weaponPSpriteY)
			}
		}
	}
	g.weaponPSpriteY = weaponBottomY
	g.inventory.PendingWeapon = 0
	g.setWeaponPSpriteState(weaponInfo(next).upstate, false)
}

func (g *game) ensureWeaponPSprites() {
	if g == nil {
		return
	}
	if g.weaponState != weaponStateNone {
		return
	}
	g.ensureWeaponDefaults()
	g.bringUpWeapon()
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
			if want := strings.TrimSpace(os.Getenv("GD_DEBUG_WEAPON_TIC")); want != "" {
				var wantTic int
				if _, err := fmt.Sscanf(want, "%d", &wantTic); err == nil {
					if g.demoTick-1 >= wantTic-2 && g.demoTick-1 <= wantTic+2 {
						fmt.Printf("gd-weapon-debug gametic=%d phase=set state=%d tics=%d y=%d ready=%d pending=%d action=%d next=%d\n",
							g.demoTick-1, state, def.tics, g.weaponPSpriteY, g.inventory.ReadyWeapon, g.inventory.PendingWeapon, def.action, def.next)
					}
				}
			}
		}
		switch def.action {
		case weaponPspriteActionReady:
			g.weaponActionReady(state)
		case weaponPspriteActionLower:
			g.weaponActionLower(state)
		case weaponPspriteActionRaise:
			g.weaponActionRaise(state)
		case weaponPspriteActionPunch:
			g.weaponActionPunch(state)
		case weaponPspriteActionFirePistol:
			g.weaponActionFirePistol(state)
		case weaponPspriteActionFireShotgun:
			g.weaponActionFireShotgun(state)
		case weaponPspriteActionFireSuperShotgun:
			g.weaponActionFireSuperShotgun(state)
		case weaponPspriteActionFireChaingun:
			g.weaponActionFireChaingun(state)
		case weaponPspriteActionGunFlash:
			g.weaponActionGunFlash(state)
		case weaponPspriteActionFireMissile:
			g.weaponActionFireMissile(state)
		case weaponPspriteActionSaw:
			g.weaponActionSaw(state)
		case weaponPspriteActionFirePlasma:
			g.weaponActionFirePlasma(state)
		case weaponPspriteActionBFGSound:
			g.weaponActionBFGSound(state)
		case weaponPspriteActionFireBFG:
			g.weaponActionFireBFG(state)
		case weaponPspriteActionReFire:
			g.weaponActionRefire(state)
		case weaponPspriteActionCheckReload:
			g.weaponActionCheckReload(state)
		case weaponPspriteActionOpenShotgun2:
			g.weaponActionOpenShotgun2(state)
		case weaponPspriteActionLoadShotgun2:
			g.weaponActionLoadShotgun2(state)
		case weaponPspriteActionCloseShotgun2:
			g.weaponActionCloseShotgun2(state)
		case weaponPspriteActionLight0:
			g.weaponActionLight0(state)
		case weaponPspriteActionLight1:
			g.weaponActionLight1(state)
		case weaponPspriteActionLight2:
			g.weaponActionLight2(state)
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
	def, ok := weaponPspriteDefs[g.weaponState]
	if !ok {
		return ""
	}
	name := def.sprite
	if _, ok := g.opts.SpritePatchBank[name]; ok {
		return name
	}
	return ""
}

func (g *game) weaponFlashSpriteName() string {
	if g == nil || g.weaponFlashState == weaponStateNone {
		return ""
	}
	def, ok := weaponPspriteDefs[g.weaponFlashState]
	if !ok {
		return ""
	}
	name := def.sprite
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

func (g *game) weaponBobDoom() (int, int) {
	if g == nil || g.isDead {
		return 0, 0
	}
	bob := fixedMul(g.p.momx, g.p.momx) + fixedMul(g.p.momy, g.p.momy)
	bob >>= 2
	const maxBob = 0x100000
	if bob > maxBob {
		bob = maxBob
	}
	idx := (128 * g.worldTic) & doomFineMask
	x := fixedMul(bob, doomFineSine[idx+doomFineAngles/4]) >> fracBits
	y := fixedMul(bob, doomFineSine[idx]) >> fracBits
	return int(x), int(y)
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
	logicalY := float64(g.weaponPSpriteY)
	if logicalY == 0 {
		logicalY = weaponTopY
	}
	switch g.statusBarDisplayMode() {
	case statusBarDisplayOverlay, statusBarDisplayHidden:
		logicalY -= 8
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
	bx, _ := g.weaponBob()
	x := (1.0 + bx) * scale
	y := float64(rect.Dy()) - (doomLogicalH-logicalY)*scale
	if !g.opts.SourcePortMode {
		const doomBaseYCenter = 100.5
		y = float64(rect.Dy())/2 - (doomBaseYCenter-logicalY)*scale
	}
	_ = g.drawSpritePatch(target, name, x, y, scale, scale)
	if flash := g.weaponFlashSpriteName(); flash != "" {
		_ = g.drawSpritePatch(target, flash, x, y, scale, scale)
	}
}
