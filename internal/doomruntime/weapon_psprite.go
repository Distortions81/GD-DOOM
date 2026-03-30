package doomruntime

import (
	"fmt"
	"image"
	"image/color"
	"math"
	"strings"
	"time"

	"gddoom/internal/render/scene"

	"github.com/hajimehoshi/ebiten/v2"
)

const (
	weaponRaiseSpeed = 6
	weaponLowerSpeed = 6
	weaponTopY       = 32
	weaponBottomY    = 128
	weaponBlendSteps = 8
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
		if want := strings.TrimSpace(runtimeDebugEnv("GD_DEBUG_WEAPON_TIC")); want != "" {
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
	if want := strings.TrimSpace(runtimeDebugEnv("GD_DEBUG_WEAPON_TIC")); want != "" {
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
			if want := strings.TrimSpace(runtimeDebugEnv("GD_DEBUG_WEAPON_TIC")); want != "" {
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
		var currentState weaponPspriteState
		var currentTics int
		if flash {
			currentState = g.weaponFlashState
			currentTics = g.weaponFlashTics
		} else {
			currentState = g.weaponState
			currentTics = g.weaponStateTics
		}
		if currentTics != 0 {
			return
		}
		if currentState == weaponStateNone {
			return
		}
		nextDef, ok := weaponPspriteDefs[currentState]
		if !ok {
			return
		}
		state = nextDef.next
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
	return g.weaponSpriteNameForState(g.weaponState)
}

func (g *game) weaponSpriteNameForState(state weaponPspriteState) string {
	if g == nil {
		return ""
	}
	g.ensureWeaponPSprites()
	def, ok := weaponPspriteDefs[state]
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
	return g.weaponFlashSpriteNameForState(g.weaponFlashState)
}

func (g *game) weaponFlashSpriteNameForState(state weaponPspriteState) string {
	if g == nil || state == weaponStateNone {
		return ""
	}
	def, ok := weaponPspriteDefs[state]
	if !ok {
		return ""
	}
	name := def.sprite
	if _, ok := g.opts.SpritePatchBank[name]; ok {
		return name
	}
	return ""
}

func (g *game) renderWeaponOverlayState() (string, string, string, string, float64, float64) {
	if g == nil {
		return "", "", "", "", 0, 0
	}
	alpha := g.weaponRenderAlpha()
	if !g.opts.SourcePortMode {
		alpha = 1
	}
	if alpha < 0 {
		alpha = 0
	}
	if alpha > 1 {
		alpha = 1
	}
	alpha = applyBlendShutter(alpha, weaponSpriteShutterAngle)
	currName := g.weaponSpriteNameForState(g.weaponState)
	prevName := g.weaponSpriteNameForState(g.prevWeaponState)
	currFlash := g.weaponFlashSpriteNameForState(g.weaponFlashState)
	prevFlash := g.weaponFlashSpriteNameForState(g.prevWeaponFlashState)
	if alpha >= 1 {
		return currName, "", currFlash, "", float64(g.weaponPSpriteY), 1
	}
	y := lerp(float64(g.prevWeaponPSpriteY), float64(g.weaponPSpriteY), alpha)
	if prevName == currName && prevFlash == currFlash {
		prevName = ""
		prevFlash = ""
	}
	return currName, prevName, currFlash, prevFlash, y, alpha
}

func (g *game) weaponRenderAlpha() float64 {
	if g == nil {
		return 1
	}
	alpha := g.renderAlpha
	if g.lastUpdate.IsZero() {
		return alpha
	}
	step := g.lastSimInterval.Seconds()
	if step <= 1e-6 {
		ticRate := float64(doomTicsPerSecond)
		if g.simTickScale > 0 {
			ticRate *= g.simTickScale
		}
		if ticRate > 1e-6 {
			step = 1.0 / ticRate
		}
	}
	if step <= 1e-6 {
		return alpha
	}
	a := time.Since(g.lastUpdate).Seconds() / step
	if a < 0 {
		return 0
	}
	if a > 1 {
		return 1
	}
	return a
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
	y := fixedMul(bob, doomFineSine[idx&(doomFineAngles/2-1)]) >> fracBits
	return int(x), int(y)
}

func (g *game) spritePatch(name string) (*ebiten.Image, int, int, int, int, bool) {
	key := strings.ToUpper(strings.TrimSpace(name))
	resolvedKey, p, ok := g.resolveSpritePatchTexture(key)
	if !ok || p.Width <= 0 || p.Height <= 0 || len(p.RGBA) != p.Width*p.Height*4 {
		return nil, 0, 0, 0, 0, false
	}
	if g.spritePatchImg == nil {
		g.spritePatchImg = make(map[string]*ebiten.Image, 256)
	}
	if img, ok := g.spritePatchImg[resolvedKey]; ok {
		return img, p.Width, p.Height, p.OffsetX, p.OffsetY, true
	}
	g.debugImageAlloc("sprite-patch:"+resolvedKey, p.Width, p.Height)
	img := newDebugImage("sprite-patch:"+resolvedKey, p.Width, p.Height)
	img.WritePixels(p.RGBA)
	g.spritePatchImg[resolvedKey] = img
	return img, p.Width, p.Height, p.OffsetX, p.OffsetY, true
}

func (g *game) resolveSpritePatchTexture(name string) (string, WallTexture, bool) {
	key := strings.ToUpper(strings.TrimSpace(name))
	if key == "" || g == nil {
		return "", WallTexture{}, false
	}
	if tex, ok := g.opts.SpritePatchBank[key]; ok && tex.Width > 0 && tex.Height > 0 && len(tex.RGBA) == tex.Width*tex.Height*4 {
		return key, tex, true
	}
	if g.spritePatchResolvedCache != nil {
		if tex, ok := g.spritePatchResolvedCache[key]; ok && tex.Width > 0 && tex.Height > 0 && len(tex.RGBA) == tex.Width*tex.Height*4 {
			return key, tex, true
		}
	}
	if tex, ok := g.blendedSpritePatch(key); ok {
		g.cacheResolvedSpritePatch(key, tex)
		return key, tex, true
	}
	if tex, ok := g.compositeSpritePatch(key); ok {
		g.cacheResolvedSpritePatch(key, tex)
		return key, tex, true
	}
	if base := fallbackSpritePatchKey(key); base != "" {
		return g.resolveSpritePatchTexture(base)
	}
	return "", WallTexture{}, false
}

func (g *game) cacheResolvedSpritePatch(key string, tex WallTexture) {
	if g == nil || key == "" || tex.Width <= 0 || tex.Height <= 0 || len(tex.RGBA) != tex.Width*tex.Height*4 {
		return
	}
	if g.spritePatchResolvedCache == nil {
		g.spritePatchResolvedCache = make(map[string]WallTexture, 256)
	}
	g.spritePatchResolvedCache[key] = tex
}

func blendWallTexture(from, to WallTexture, alpha float64) (WallTexture, bool) {
	if from.Width <= 0 || from.Height <= 0 || to.Width <= 0 || to.Height <= 0 {
		return WallTexture{}, false
	}
	left := min(-from.OffsetX, -to.OffsetX)
	top := min(-from.OffsetY, -to.OffsetY)
	right := max(from.Width-from.OffsetX, to.Width-to.OffsetX)
	bottom := max(from.Height-from.OffsetY, to.Height-to.OffsetY)
	w := right - left
	h := bottom - top
	if w <= 0 || h <= 0 {
		return WallTexture{}, false
	}
	rgba := make([]byte, w*h*4)
	sample := func(tex WallTexture, x, y int) color.NRGBA {
		tx := x + tex.OffsetX
		ty := y + tex.OffsetY
		if tx < 0 || tx >= tex.Width || ty < 0 || ty >= tex.Height {
			return color.NRGBA{}
		}
		i := (ty*tex.Width + tx) * 4
		return color.NRGBA{R: tex.RGBA[i], G: tex.RGBA[i+1], B: tex.RGBA[i+2], A: tex.RGBA[i+3]}
	}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			lx := x + left
			ly := y + top
			c0 := sample(from, lx, ly)
			c1 := sample(to, lx, ly)
			i := (y*w + x) * 4
			rgba[i+0] = uint8(math.Round(float64(c0.R)*(1-alpha) + float64(c1.R)*alpha))
			rgba[i+1] = uint8(math.Round(float64(c0.G)*(1-alpha) + float64(c1.G)*alpha))
			rgba[i+2] = uint8(math.Round(float64(c0.B)*(1-alpha) + float64(c1.B)*alpha))
			rgba[i+3] = uint8(math.Round(float64(c0.A)*(1-alpha) + float64(c1.A)*alpha))
		}
	}
	return WallTexture{
		Width:   w,
		Height:  h,
		OffsetX: -left,
		OffsetY: -top,
		RGBA:    rgba,
	}, true
}

func (g *game) blendedSpritePatch(key string) (WallTexture, bool) {
	if g == nil {
		return WallTexture{}, false
	}
	fromKey, toKey, numer, denom, ok := parseBlendSpritePatchKey(key)
	if !ok || denom <= 0 || numer <= 0 {
		return WallTexture{}, false
	}
	_, from, okFrom := g.resolveSpritePatchTexture(fromKey)
	_, to, okTo := g.resolveSpritePatchTexture(toKey)
	if !okFrom || !okTo {
		return WallTexture{}, false
	}
	alpha := float64(numer) / float64(denom)
	if alpha <= 0 {
		return from, true
	}
	if alpha >= 1 {
		return to, true
	}
	return blendWallTexture(from, to, alpha)
}

func (g *game) compositeSpritePatch(key string) (WallTexture, bool) {
	baseKey, overlayKey, ok := parseCompositeSpritePatchKey(key)
	if !ok {
		return WallTexture{}, false
	}
	_, base, okBase := g.resolveSpritePatchTexture(baseKey)
	if !okBase {
		return WallTexture{}, false
	}
	if overlayKey == "" {
		return base, true
	}
	_, overlay, okOverlay := g.resolveSpritePatchTexture(overlayKey)
	if !okOverlay {
		return base, true
	}
	return compositeWallTextures(base, overlay)
}

func compositeWallTextures(base, overlay WallTexture) (WallTexture, bool) {
	left := min(-base.OffsetX, -overlay.OffsetX)
	top := min(-base.OffsetY, -overlay.OffsetY)
	right := max(base.Width-base.OffsetX, overlay.Width-overlay.OffsetX)
	bottom := max(base.Height-base.OffsetY, overlay.Height-overlay.OffsetY)
	w := right - left
	h := bottom - top
	if w <= 0 || h <= 0 {
		return WallTexture{}, false
	}
	rgba := make([]byte, w*h*4)
	sample := func(tex WallTexture, x, y int) color.NRGBA {
		tx := x + tex.OffsetX
		ty := y + tex.OffsetY
		if tx < 0 || tx >= tex.Width || ty < 0 || ty >= tex.Height {
			return color.NRGBA{}
		}
		i := (ty*tex.Width + tx) * 4
		return color.NRGBA{R: tex.RGBA[i], G: tex.RGBA[i+1], B: tex.RGBA[i+2], A: tex.RGBA[i+3]}
	}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			lx := x + left
			ly := y + top
			c0 := sample(base, lx, ly)
			c1 := sample(overlay, lx, ly)
			a1 := float64(c1.A) / 255.0
			a0 := float64(c0.A) / 255.0
			outA := a1 + a0*(1-a1)
			i := (y*w + x) * 4
			if outA <= 0 {
				continue
			}
			r := (float64(c1.R)*a1 + float64(c0.R)*a0*(1-a1)) / outA
			g := (float64(c1.G)*a1 + float64(c0.G)*a0*(1-a1)) / outA
			b := (float64(c1.B)*a1 + float64(c0.B)*a0*(1-a1)) / outA
			rgba[i+0] = uint8(math.Round(r))
			rgba[i+1] = uint8(math.Round(g))
			rgba[i+2] = uint8(math.Round(b))
			rgba[i+3] = uint8(math.Round(outA * 255))
		}
	}
	return WallTexture{
		Width:   w,
		Height:  h,
		OffsetX: -left,
		OffsetY: -top,
		RGBA:    rgba,
	}, true
}

func parseBlendSpritePatchKey(key string) (from, to string, numer, denom int, ok bool) {
	gt := strings.IndexByte(key, '>')
	hash := strings.IndexByte(key, '#')
	slash := strings.IndexByte(key, '/')
	if gt <= 0 || hash <= gt+1 || slash <= hash+1 || slash >= len(key)-1 {
		return "", "", 0, 0, false
	}
	from = strings.TrimSpace(key[:gt])
	to = strings.TrimSpace(key[gt+1 : hash])
	if from == "" || to == "" {
		return "", "", 0, 0, false
	}
	if _, err := fmt.Sscanf(key[hash+1:], "%d/%d", &numer, &denom); err != nil {
		return "", "", 0, 0, false
	}
	if numer < 0 {
		numer = 0
	}
	if numer > denom {
		numer = denom
	}
	return from, to, numer, denom, denom > 0
}

func parseCompositeSpritePatchKey(key string) (base, overlay string, ok bool) {
	plus := strings.IndexByte(key, '+')
	if plus <= 0 {
		return "", "", false
	}
	base = strings.TrimSpace(key[:plus])
	overlay = strings.TrimSpace(key[plus+1:])
	if base == "" {
		return "", "", false
	}
	return base, overlay, true
}

func weaponCompositePatchToken(base, flash string) string {
	if base == "" {
		return flash
	}
	if flash == "" {
		return base
	}
	return base + "+" + flash
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
	return drawSpritePatchClippedAlpha(screen, img, x, y, sx, sy, ox, oy, clipW, clipH, 1)
}

func (g *game) drawSpritePatchAlpha(screen *ebiten.Image, name string, x, y, sx, sy, alpha float64) bool {
	img, _, _, ox, oy, ok := g.spritePatch(name)
	if !ok {
		return false
	}
	return drawSpritePatchClippedAlpha(screen, img, x, y, sx, sy, ox, oy, screen.Bounds().Dx(), screen.Bounds().Dy(), alpha)
}

func drawSpritePatchClippedAlpha(screen, img *ebiten.Image, x, y, sx, sy float64, ox, oy, clipW, clipH int, alpha float64) bool {
	if screen == nil || img == nil || sx <= 0 || sy <= 0 || clipW <= 0 || clipH <= 0 {
		return false
	}
	if alpha <= 0 {
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
	op.ColorScale.Scale(1, 1, 1, float32(alpha))
	op.GeoM.Scale(sx, sy)
	op.GeoM.Translate(dstX+float64(srcX0)*sx, dstY+float64(srcY0)*sy)
	screen.DrawImage(sub, op)
	return true
}

func quantizeWeaponBlendAlpha(alpha float64) float64 {
	if alpha <= 0 {
		return 0
	}
	if alpha >= 1 {
		return 1
	}
	if weaponBlendSteps <= 1 {
		return alpha
	}
	return math.Round(alpha*weaponBlendSteps) / weaponBlendSteps
}

func (g *game) drawWeaponOverlay(screen *ebiten.Image) {
	if g == nil || g.mode != viewWalk || g.isDead {
		return
	}
	name, prevName, flash, prevFlash, logicalY, alpha := g.renderWeaponOverlayState()
	currComposite := weaponCompositePatchToken(name, flash)
	prevComposite := weaponCompositePatchToken(prevName, prevFlash)
	if currComposite == "" && prevComposite == "" {
		return
	}
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
	if prevComposite != "" && currComposite != "" && alpha > 0 && alpha < 1 {
		alpha = quantizeWeaponBlendAlpha(alpha)
		if alpha <= 0 {
			_ = g.drawSpritePatch(target, prevComposite, x, y, scale, scale)
			return
		}
		if alpha >= 1 {
			_ = g.drawSpritePatch(target, currComposite, x, y, scale, scale)
			return
		}
		blendName := fmt.Sprintf("%s>%s#%d/%d", prevComposite, currComposite, int(math.Round(alpha*weaponBlendSteps)), weaponBlendSteps)
		_ = g.drawSpritePatch(target, blendName, x, y, scale, scale)
	} else if currComposite != "" {
		_ = g.drawSpritePatch(target, currComposite, x, y, scale, scale)
	} else if prevComposite != "" {
		_ = g.drawSpritePatch(target, prevComposite, x, y, scale, scale)
	}
}

func (g *game) precacheWeaponSpritePatches() {
	if g == nil {
		return
	}
	for id := range weaponDefs {
		baseStates, flashStates := collectWeaponPrecacheStates(id)
		baseSprites := weaponPrecacheSpriteNames(g.weaponSpriteNameForState, baseStates)
		flashSprites := weaponPrecacheSpriteNames(g.weaponFlashSpriteNameForState, flashStates)
		for base := range baseSprites {
			_, _, _, _, _, _ = g.spritePatch(base)
		}
		for flash := range flashSprites {
			_, _, _, _, _, _ = g.spritePatch(flash)
		}
		for base := range baseSprites {
			for flash := range flashSprites {
				composite := weaponCompositePatchToken(base, flash)
				if composite == "" {
					continue
				}
				_, _, _, _, _, _ = g.spritePatch(composite)
			}
		}
		for _, edge := range weaponPrecacheEdges(g.weaponSpriteNameForState, baseStates) {
			g.precacheWeaponBlendVariants(edge.from, edge.to, flashSprites)
		}
		for _, edge := range weaponPrecacheEdges(g.weaponFlashSpriteNameForState, flashStates) {
			for base := range baseSprites {
				g.precacheWeaponBlendPair(weaponCompositePatchToken(base, edge.from), weaponCompositePatchToken(base, edge.to))
			}
		}
		for _, edge := range weaponPrecacheActionFlashEdges(id) {
			for flash := range edge.toFlashes {
				g.precacheWeaponBlendPair(weaponCompositePatchToken(edge.from, edge.fromFlash), weaponCompositePatchToken(edge.to, flash))
			}
		}
	}
}

type weaponPrecacheEdge struct {
	from string
	to   string
}

type weaponPrecacheActionEdge struct {
	from      string
	to        string
	fromFlash string
	toFlashes map[string]struct{}
}

func collectWeaponPrecacheStates(id weaponID) ([]weaponPspriteState, []weaponPspriteState) {
	def := weaponInfo(id)
	base := collectWeaponStateChain(def.readystate, def.upstate, def.downstate, def.atkstate)
	flash := collectWeaponStateChain(def.flashstate)
	return base, flash
}

func collectWeaponStateChain(roots ...weaponPspriteState) []weaponPspriteState {
	seen := make(map[weaponPspriteState]struct{}, len(roots)*4)
	out := make([]weaponPspriteState, 0, len(roots)*4)
	for _, root := range roots {
		for state := root; state != weaponStateNone; {
			if _, ok := seen[state]; ok {
				break
			}
			def, ok := weaponPspriteDefs[state]
			if !ok {
				break
			}
			seen[state] = struct{}{}
			out = append(out, state)
			next := def.next
			if next == state {
				break
			}
			state = next
		}
	}
	return out
}

func weaponPrecacheSpriteNames(resolve func(weaponPspriteState) string, states []weaponPspriteState) map[string]struct{} {
	out := make(map[string]struct{}, len(states)+1)
	out[""] = struct{}{}
	for _, state := range states {
		if name := resolve(state); name != "" {
			out[name] = struct{}{}
		}
	}
	return out
}

func weaponPrecacheEdges(resolve func(weaponPspriteState) string, states []weaponPspriteState) []weaponPrecacheEdge {
	out := make([]weaponPrecacheEdge, 0, len(states))
	seen := make(map[weaponPrecacheEdge]struct{}, len(states))
	for _, state := range states {
		def, ok := weaponPspriteDefs[state]
		if !ok || def.next == weaponStateNone || def.next == state {
			continue
		}
		from := resolve(state)
		to := resolve(def.next)
		if from == "" || to == "" || from == to {
			continue
		}
		edge := weaponPrecacheEdge{from: from, to: to}
		if _, ok := seen[edge]; ok {
			continue
		}
		seen[edge] = struct{}{}
		out = append(out, edge)
	}
	return out
}

func weaponActionStartsFlash(action weaponPspriteAction) bool {
	switch action {
	case weaponPspriteActionGunFlash,
		weaponPspriteActionFirePistol,
		weaponPspriteActionFireShotgun,
		weaponPspriteActionFireSuperShotgun,
		weaponPspriteActionFireChaingun,
		weaponPspriteActionFirePlasma,
		weaponPspriteActionFireBFG:
		return true
	default:
		return false
	}
}

func weaponFlashVariantsForState(id weaponID, state weaponPspriteState) map[string]struct{} {
	out := make(map[string]struct{}, 2)
	def := weaponInfo(id)
	if def.flashstate != weaponStateNone {
		if name := weaponPspriteDefs[def.flashstate].sprite; name != "" {
			out[name] = struct{}{}
		}
	}
	switch state {
	case weaponStateChaingunAtk2:
		if name := weaponPspriteDefs[weaponStateChaingunFlash2].sprite; name != "" {
			out[name] = struct{}{}
		}
	case weaponStatePlasmaAtk1:
		if name := weaponPspriteDefs[weaponStatePlasmaFlash2].sprite; name != "" {
			out[name] = struct{}{}
		}
	}
	return out
}

func weaponPrecacheActionFlashEdges(id weaponID) []weaponPrecacheActionEdge {
	baseStates, _ := collectWeaponPrecacheStates(id)
	out := make([]weaponPrecacheActionEdge, 0, len(baseStates)+1)
	seen := make(map[weaponPrecacheEdge]struct{}, len(baseStates)+1)
	appendEdge := func(fromState, toState weaponPspriteState) {
		if toState == weaponStateNone {
			return
		}
		toDef, ok := weaponPspriteDefs[toState]
		if !ok || !weaponActionStartsFlash(toDef.action) {
			return
		}
		from := weaponPspriteDefs[fromState].sprite
		to := toDef.sprite
		if from == "" || to == "" {
			return
		}
		key := weaponPrecacheEdge{from: from, to: to}
		if _, ok := seen[key]; ok {
			return
		}
		flashes := weaponFlashVariantsForState(id, toState)
		if len(flashes) == 0 {
			return
		}
		seen[key] = struct{}{}
		out = append(out, weaponPrecacheActionEdge{from: from, to: to, fromFlash: "", toFlashes: flashes})
	}
	for _, state := range baseStates {
		def, ok := weaponPspriteDefs[state]
		if !ok || def.next == weaponStateNone {
			continue
		}
		appendEdge(state, def.next)
	}
	def := weaponInfo(id)
	if def.readystate != weaponStateNone && def.atkstate != weaponStateNone {
		appendEdge(def.readystate, def.atkstate)
	}
	return out
}

func (g *game) precacheWeaponBlendVariants(from, to string, flashes map[string]struct{}) {
	if from == "" || to == "" {
		return
	}
	if len(flashes) == 0 {
		g.precacheWeaponBlendPair(from, to)
		return
	}
	for flash := range flashes {
		g.precacheWeaponBlendPair(weaponCompositePatchToken(from, flash), weaponCompositePatchToken(to, flash))
	}
}

func (g *game) precacheWeaponBlendPair(from, to string) {
	if g == nil || from == "" || to == "" || from == to {
		return
	}
	for step := 1; step < weaponBlendSteps; step++ {
		key := fmt.Sprintf("%s>%s#%d/%d", from, to, step, weaponBlendSteps)
		_, _, _, _, _, _ = g.spritePatch(key)
	}
}
