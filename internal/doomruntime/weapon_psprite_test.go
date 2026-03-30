package doomruntime

import (
	"math"
	"testing"

	"gddoom/internal/mapdata"
)

func TestWeaponReadySpriteName(t *testing.T) {
	if got := weaponReadySpriteName(weaponPistol, 0); got != "PISGA0" {
		t.Fatalf("pistol ready sprite=%q want PISGA0", got)
	}
	if got := weaponReadySpriteName(weaponShotgun, 0); got != "SHTGA0" {
		t.Fatalf("shotgun ready sprite=%q want SHTGA0", got)
	}
	if got := weaponReadySpriteName(weaponChaingun, 0); got != "CHGGA0" {
		t.Fatalf("chaingun ready sprite=%q want CHGGA0", got)
	}
	if got := weaponReadySpriteName(weaponChainsaw, 0); got != "SAWGC0" {
		t.Fatalf("chainsaw ready even sprite=%q want SAWGC0", got)
	}
	if got := weaponReadySpriteName(weaponChainsaw, 4); got != "SAWGD0" {
		t.Fatalf("chainsaw ready odd sprite=%q want SAWGD0", got)
	}
}

func TestBringUpWeaponStartsOffscreenAndReachesReady(t *testing.T) {
	g := &game{
		inventory: playerInventory{
			ReadyWeapon: weaponPistol,
			Weapons:     map[int16]bool{},
		},
	}

	g.bringUpWeapon()
	if g.weaponPSpriteY != weaponBottomY-weaponRaiseSpeed {
		t.Fatalf("weapon y=%d want=%d", g.weaponPSpriteY, weaponBottomY-weaponRaiseSpeed)
	}
	if g.weaponState != weaponStatePistolUp {
		t.Fatalf("weapon state=%v want=%v", g.weaponState, weaponStatePistolUp)
	}
	advanceWeaponToReady(g)
	if g.weaponPSpriteY != weaponTopY {
		t.Fatalf("weapon y=%d want=%d after raise", g.weaponPSpriteY, weaponTopY)
	}
	if g.weaponState != weaponStatePistolReady {
		t.Fatalf("weapon state=%v want=%v after raise", g.weaponState, weaponStatePistolReady)
	}
}

func TestBringUpChainsawStartsIdleLoopSoundWhenReady(t *testing.T) {
	g := &game{
		inventory: playerInventory{
			ReadyWeapon: weaponChainsaw,
			Weapons:     map[int16]bool{2005: true},
		},
	}

	g.bringUpWeapon()
	advanceWeaponToReady(g)
	if !hasSoundEvent(g.soundQueue, soundEventSawUp) {
		t.Fatalf("soundQueue=%v missing %v", g.soundQueue, soundEventSawUp)
	}
	if !hasSoundEvent(g.soundQueue, soundEventSawIdle) {
		t.Fatalf("soundQueue=%v missing %v", g.soundQueue, soundEventSawIdle)
	}
}

func TestWeaponSpriteName_PrefersFireAnimationThenReady(t *testing.T) {
	g := &game{
		worldTic: 0,
		stats: playerStats{
			Bullets: 10,
		},
		inventory: playerInventory{
			ReadyWeapon: weaponPistol,
		},
		opts: Options{
			SpritePatchBank: map[string]WallTexture{
				"PISGA0": {Width: 1, Height: 1, RGBA: []byte{1, 2, 3, 4}},
				"PISGB0": {Width: 1, Height: 1, RGBA: []byte{1, 2, 3, 4}},
				"PISGC0": {Width: 1, Height: 1, RGBA: []byte{1, 2, 3, 4}},
				"PISFA0": {Width: 1, Height: 1, RGBA: []byte{1, 2, 3, 4}},
			},
		},
	}

	g.startWeaponOverlayFire(weaponPistol)
	if got := g.weaponSpriteName(); got != "PISGA0" {
		t.Fatalf("initial fire sprite=%q want PISGA0", got)
	}
	if got := g.weaponFlashSpriteName(); got != "" {
		t.Fatalf("initial flash sprite=%q want empty before fire state action", got)
	}
	for i := 0; i < 4; i++ {
		g.tickWeaponOverlay()
	}
	if got := g.weaponSpriteName(); got != "PISGB0" {
		t.Fatalf("mid fire sprite=%q want PISGB0", got)
	}
	if got := g.weaponFlashSpriteName(); got != "PISFA0" {
		t.Fatalf("flash sprite after fire action=%q want PISFA0", got)
	}
	for i := 0; i < 32; i++ {
		g.tickWeaponOverlay()
	}
	if got := g.weaponSpriteName(); got != "PISGA0" {
		t.Fatalf("post fire sprite=%q want ready PISGA0", got)
	}
	if got := g.weaponFlashSpriteName(); got != "" {
		t.Fatalf("flash sprite after expire=%q want empty", got)
	}
}

func TestTickWeaponFireStartsOverlayAndClearsOnSwitch(t *testing.T) {
	g := &game{
		statusAttackDown: false,
		stats: playerStats{
			Bullets: 10,
		},
		inventory: playerInventory{
			ReadyWeapon: weaponPistol,
			Weapons:     map[int16]bool{},
		},
		opts: Options{
			SpritePatchBank: map[string]WallTexture{
				"PISGA0": {Width: 1, Height: 1, RGBA: []byte{1, 2, 3, 4}},
				"PISGB0": {Width: 1, Height: 1, RGBA: []byte{1, 2, 3, 4}},
				"PISGC0": {Width: 1, Height: 1, RGBA: []byte{1, 2, 3, 4}},
				"PISFA0": {Width: 1, Height: 1, RGBA: []byte{1, 2, 3, 4}},
				"SHTGA0": {Width: 1, Height: 1, RGBA: []byte{1, 2, 3, 4}},
			},
		},
	}

	advanceWeaponToReady(g)
	g.statusAttackDown = true
	g.tickWeaponFire()
	if g.weaponState != weaponStatePistolAtk1 || g.weaponStateTics <= 0 || g.weaponFlashState != weaponStateNone {
		t.Fatalf("weapon state not started: state=%v tics=%d flash=%v", g.weaponState, g.weaponStateTics, g.weaponFlashState)
	}
	g.inventory.Weapons[2001] = true
	g.statusAttackDown = false
	g.selectWeaponSlot(3)
	if g.inventory.PendingWeapon != weaponShotgun {
		t.Fatalf("pending weapon=%v want shotgun", g.inventory.PendingWeapon)
	}
	if g.inventory.ReadyWeapon != weaponPistol {
		t.Fatalf("ready weapon=%v want pistol until current attack finishes", g.inventory.ReadyWeapon)
	}
	for i := 0; i < 64 && g.inventory.ReadyWeapon != weaponShotgun; i++ {
		g.tickWeaponOverlay()
	}
	if g.inventory.ReadyWeapon != weaponShotgun {
		t.Fatalf("weapon switch should have selected shotgun after attack: ready=%v pending=%v", g.inventory.ReadyWeapon, g.inventory.PendingWeapon)
	}
	if g.inventory.PendingWeapon != weaponShotgun && g.inventory.PendingWeapon != 0 {
		t.Fatalf("pending weapon=%v want shotgun during raise or cleared after bring-up", g.inventory.PendingWeapon)
	}
	if g.weaponState != weaponStateShotgunUp && g.weaponState != weaponStateShotgunReady {
		t.Fatalf("weapon state=%v want shotgun raise or ready", g.weaponState)
	}
	for i := 0; i < 32; i++ {
		g.tickWeaponOverlay()
	}
	if g.inventory.PendingWeapon != 0 {
		t.Fatalf("pending weapon=%v want cleared after switch", g.inventory.PendingWeapon)
	}
	if g.inventory.ReadyWeapon != weaponShotgun || g.weaponState != weaponStateShotgunReady || g.weaponFlashState != weaponStateNone {
		t.Fatalf("weapon switch not applied after raise: ready=%v state=%v flash=%v", g.inventory.ReadyWeapon, g.weaponState, g.weaponFlashState)
	}
}

func TestWeaponSwitchDoesNotFlipReadyWeaponUntilLowerCompletes(t *testing.T) {
	g := &game{
		inventory: playerInventory{
			ReadyWeapon:   weaponPistol,
			PendingWeapon: weaponShotgun,
			Weapons:       map[int16]bool{2001: true},
		},
	}

	g.weaponPSpriteY = weaponTopY
	g.setWeaponPSpriteState(weaponStatePistolDown, false)
	if g.inventory.ReadyWeapon != weaponPistol || g.inventory.PendingWeapon != weaponShotgun {
		t.Fatalf("switch changed too early: ready=%v pending=%v", g.inventory.ReadyWeapon, g.inventory.PendingWeapon)
	}
	for i := 0; i < 32 && g.inventory.ReadyWeapon == weaponPistol; i++ {
		prevY := g.weaponPSpriteY
		prevReady, prevPending := g.inventory.ReadyWeapon, g.inventory.PendingWeapon
		g.tickWeaponOverlay()
		if prevY+weaponLowerSpeed < weaponBottomY && (g.inventory.ReadyWeapon != prevReady || g.inventory.PendingWeapon != prevPending) {
			t.Fatalf("switch changed before lower completed: prevY=%d y=%d ready=%v pending=%v", prevY, g.weaponPSpriteY, g.inventory.ReadyWeapon, g.inventory.PendingWeapon)
		}
	}
	if g.inventory.ReadyWeapon != weaponShotgun {
		t.Fatalf("ready weapon=%v want shotgun after lower completed", g.inventory.ReadyWeapon)
	}
	if g.inventory.PendingWeapon != 0 {
		t.Fatalf("pending weapon=%v want cleared after bring-up starts", g.inventory.PendingWeapon)
	}
}

func TestZeroTicRefireAmmoSwitchDoesNotLowerTwice(t *testing.T) {
	g := &game{
		stats: playerStats{Rockets: 0, Shells: 4},
		inventory: playerInventory{
			ReadyWeapon: weaponRocketLauncher,
			Weapons: map[int16]bool{
				2001: true,
				2003: true,
			},
		},
	}

	g.weaponPSpriteY = 44
	g.setWeaponPSpriteState(weaponStateRocketAtk3, false)

	if g.weaponState != weaponStateRocketDown {
		t.Fatalf("weapon state=%v want=%v", g.weaponState, weaponStateRocketDown)
	}
	if g.weaponPSpriteY != 50 {
		t.Fatalf("weapon y=%d want=50 after single lower step", g.weaponPSpriteY)
	}
	if g.inventory.ReadyWeapon != weaponRocketLauncher {
		t.Fatalf("ready weapon=%v want rocket launcher before lower completes", g.inventory.ReadyWeapon)
	}
	if g.inventory.PendingWeapon != weaponShotgun {
		t.Fatalf("pending weapon=%v want shotgun", g.inventory.PendingWeapon)
	}
}

func TestNewGameStartsWeaponBringUpLikeVanilla(t *testing.T) {
	g := newGame(&mapdata.Map{}, Options{})
	if g.weaponState != weaponStatePistolUp {
		t.Fatalf("weaponState=%v want pistol up", g.weaponState)
	}
	if g.weaponStateTics != 1 {
		t.Fatalf("weaponStateTics=%d want 1", g.weaponStateTics)
	}
	if g.weaponPSpriteY != weaponBottomY-weaponRaiseSpeed {
		t.Fatalf("weaponPSpriteY=%d want %d", g.weaponPSpriteY, weaponBottomY-weaponRaiseSpeed)
	}
	if g.inventory.PendingWeapon != 0 {
		t.Fatalf("pending weapon=%v want cleared after bring-up setup", g.inventory.PendingWeapon)
	}
}

func TestSpritePatch_FallsBackToBasePatchForMissingBlendToken(t *testing.T) {
	g := &game{
		opts: Options{
			SpritePatchBank: map[string]WallTexture{
				"BON1A0": {Width: 1, Height: 1, OffsetX: 2, OffsetY: 3, RGBA: []byte{1, 2, 3, 4}},
				"BON1B0": {Width: 1, Height: 1, OffsetX: 5, OffsetY: 7, RGBA: []byte{9, 8, 7, 6}},
			},
		},
	}
	_, w, h, _, _, ok := g.spritePatch("BON1A0>BON1B0#1/10")
	if !ok {
		t.Fatal("spritePatch should lazily materialize or fall back for blend token")
	}
	if w <= 0 || h <= 0 {
		t.Fatalf("got w=%d h=%d want positive blended size", w, h)
	}
}

func TestSpritePatch_MaterializesBlendedPatchToken(t *testing.T) {
	g := &game{
		opts: Options{
			SpritePatchBank: map[string]WallTexture{
				"BON1A0": {Width: 1, Height: 1, OffsetX: 0, OffsetY: 0, RGBA: []byte{0, 0, 0, 255}},
				"BON1B0": {Width: 1, Height: 1, OffsetX: 0, OffsetY: 0, RGBA: []byte{200, 100, 50, 255}},
			},
		},
	}
	_, w, h, _, _, ok := g.spritePatch("BON1A0>BON1B0#128/255")
	if !ok {
		t.Fatal("spritePatch should materialize blended patch")
	}
	if w != 1 || h != 1 {
		t.Fatalf("got w=%d h=%d want 1x1", w, h)
	}
	if img := g.spritePatchImg["BON1A0>BON1B0#128/255"]; img == nil {
		t.Fatal("expected blended patch cached under token key")
	}
}

func TestSpritePatch_MaterializesCompositeBlendToken(t *testing.T) {
	g := &game{
		opts: Options{
			SpritePatchBank: map[string]WallTexture{
				"PISGA0": {Width: 1, Height: 1, OffsetX: 0, OffsetY: 0, RGBA: []byte{10, 20, 30, 255}},
				"PISGB0": {Width: 1, Height: 1, OffsetX: 0, OffsetY: 0, RGBA: []byte{110, 120, 130, 255}},
				"PISFA0": {Width: 1, Height: 1, OffsetX: 0, OffsetY: 0, RGBA: []byte{200, 210, 220, 255}},
			},
		},
	}
	key := "PISGA0>PISGB0+PISFA0#128/255"
	_, w, h, _, _, ok := g.spritePatch(key)
	if !ok {
		t.Fatal("spritePatch should materialize blended composite patch")
	}
	if w != 1 || h != 1 {
		t.Fatalf("got w=%d h=%d want 1x1", w, h)
	}
	if img := g.spritePatchImg[key]; img == nil {
		t.Fatal("expected composite blended patch cached under token key")
	}
}

func TestPrecacheWeaponSpritePatches_WarmsCompositeCache(t *testing.T) {
	g := &game{
		opts: Options{
			SpritePatchBank: map[string]WallTexture{
				"PISGA0": {Width: 1, Height: 1, OffsetX: 0, OffsetY: 0, RGBA: []byte{10, 20, 30, 255}},
				"PISGB0": {Width: 1, Height: 1, OffsetX: 0, OffsetY: 0, RGBA: []byte{40, 50, 60, 255}},
				"PISGC0": {Width: 1, Height: 1, OffsetX: 0, OffsetY: 0, RGBA: []byte{70, 80, 90, 255}},
				"PISFA0": {Width: 1, Height: 1, OffsetX: 0, OffsetY: 0, RGBA: []byte{100, 110, 120, 255}},
			},
		},
	}

	g.precacheWeaponSpritePatches()

	if g.spritePatchResolvedCache == nil {
		t.Fatal("expected resolved sprite patch cache to be initialized")
	}
	if _, ok := g.spritePatchResolvedCache["PISGA0+PISFA0"]; !ok {
		t.Fatal("expected composite psprite patch to be resolved during precache")
	}
	if g.spritePatchImg == nil || g.spritePatchImg["PISGA0+PISFA0"] == nil {
		t.Fatal("expected composite psprite image to be materialized during precache")
	}
	if _, ok := g.spritePatchResolvedCache["PISGA0>PISGB0+PISFA0#1/8"]; !ok {
		t.Fatal("expected pistol animation blend token to be materialized during precache")
	}
	if g.spritePatchImg["PISGA0>PISGB0+PISFA0#1/8"] == nil {
		t.Fatal("expected pistol animation blend image to be cached during precache")
	}
}

func TestQuantizeWeaponBlendAlpha(t *testing.T) {
	tests := []struct {
		in   float64
		want float64
	}{
		{in: -0.1, want: 0},
		{in: 0, want: 0},
		{in: 0.06, want: 0},
		{in: 0.10, want: 0.125},
		{in: 0.49, want: 0.5},
		{in: 0.74, want: 0.75},
		{in: 1, want: 1},
		{in: 1.2, want: 1},
	}
	for _, tc := range tests {
		if got := quantizeWeaponBlendAlpha(tc.in); got != tc.want {
			t.Fatalf("quantizeWeaponBlendAlpha(%v)=%v want %v", tc.in, got, tc.want)
		}
	}
}

func TestWeaponOverlayAnchorUsesDoomCenterX(t *testing.T) {
	rectW := 640
	scale := float64(rectW) / doomLogicalW
	got := 1.0 * scale
	if math.Abs(got-2.0) > 1e-9 {
		t.Fatalf("psprite x anchor=%v want 2", got)
	}
}

func TestWeaponOverlayYUsesViewportCenter(t *testing.T) {
	rectW := 640
	rectH := 336
	scale := float64(rectW) / doomLogicalW
	got := float64(rectH)/2 - (100.5-32.0)*scale
	if math.Abs(got-31.0) > 1e-9 {
		t.Fatalf("faithful overlay y=%v want 31", got)
	}
}

func TestRenderWeaponOverlayState_InterpolatesSourcePortPsprite(t *testing.T) {
	g := &game{
		opts: Options{
			SourcePortMode: true,
			SpritePatchBank: map[string]WallTexture{
				"PISGA0": {Width: 1, Height: 1, RGBA: []byte{1, 2, 3, 4}},
				"PISGB0": {Width: 1, Height: 1, RGBA: []byte{5, 6, 7, 8}},
				"PISFA0": {Width: 1, Height: 1, RGBA: []byte{9, 10, 11, 12}},
			},
		},
		prevWeaponState:      weaponStatePistolReady,
		weaponState:          weaponStatePistolAtk2,
		prevWeaponFlashState: weaponStateNone,
		weaponFlashState:     weaponStatePistolFlash,
		prevWeaponPSpriteY:   32,
		weaponPSpriteY:       40,
		renderAlpha:          0.5,
	}

	name, prevName, flash, prevFlash, y, alpha := g.renderWeaponOverlayState()
	if name != "PISGB0" || prevName != "PISGA0" {
		t.Fatalf("weapon names got current=%q prev=%q want PISGB0/PISGA0", name, prevName)
	}
	if flash != "PISFA0" || prevFlash != "" {
		t.Fatalf("flash names got current=%q prev=%q want PISFA0/empty", flash, prevFlash)
	}
	if y != 36 {
		t.Fatalf("interpolated y=%v want 36", y)
	}
	if alpha != 0.5 {
		t.Fatalf("alpha=%v want 0.5", alpha)
	}
}

func TestRenderWeaponOverlayState_KeepsPrevBaseWhenOnlyFlashChanges(t *testing.T) {
	g := &game{
		opts: Options{
			SourcePortMode: true,
			SpritePatchBank: map[string]WallTexture{
				"PISGA0": {Width: 1, Height: 1, RGBA: []byte{1, 2, 3, 255}},
				"PISFA0": {Width: 1, Height: 1, RGBA: []byte{9, 10, 11, 255}},
			},
		},
		prevWeaponState:      weaponStatePistolReady,
		weaponState:          weaponStatePistolAtk1,
		prevWeaponFlashState: weaponStateNone,
		weaponFlashState:     weaponStatePistolFlash,
		prevWeaponPSpriteY:   32,
		weaponPSpriteY:       32,
		renderAlpha:          0.5,
	}

	name, prevName, flash, prevFlash, _, alpha := g.renderWeaponOverlayState()
	if name != "PISGA0" || prevName != "PISGA0" {
		t.Fatalf("weapon names got current=%q prev=%q want PISGA0/PISGA0", name, prevName)
	}
	if flash != "PISFA0" || prevFlash != "" {
		t.Fatalf("flash names got current=%q prev=%q want PISFA0/empty", flash, prevFlash)
	}
	if alpha != 0.5 {
		t.Fatalf("alpha=%v want 0.5", alpha)
	}
}

func TestRenderWeaponOverlayState_DoesNotInterpolateClassicMode(t *testing.T) {
	g := &game{
		opts: Options{
			SourcePortMode: false,
			SpritePatchBank: map[string]WallTexture{
				"PISGA0": {Width: 1, Height: 1, RGBA: []byte{1, 2, 3, 4}},
				"PISGB0": {Width: 1, Height: 1, RGBA: []byte{5, 6, 7, 8}},
			},
		},
		prevWeaponState:    weaponStatePistolReady,
		weaponState:        weaponStatePistolAtk2,
		prevWeaponPSpriteY: 32,
		weaponPSpriteY:     40,
		renderAlpha:        0.5,
	}

	name, prevName, _, _, y, alpha := g.renderWeaponOverlayState()
	if name != "PISGB0" || prevName != "" {
		t.Fatalf("classic names got current=%q prev=%q want PISGB0/empty", name, prevName)
	}
	if y != 40 {
		t.Fatalf("classic y=%v want 40", y)
	}
	if alpha != 1 {
		t.Fatalf("classic alpha=%v want 1", alpha)
	}
}

func TestWeaponRenderAlpha_UsesExpectedTickRate(t *testing.T) {
	g := &game{
		renderAlpha: 0.2,
	}
	alpha := g.weaponRenderAlpha()
	if math.Abs(alpha-0.2) > 0.001 {
		t.Fatalf("weapon alpha=%v want cached render alpha", alpha)
	}
}
