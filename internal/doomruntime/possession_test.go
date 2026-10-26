package doomruntime

import (
	"math"
	"testing"
	"time"

	"gddoom/internal/mapdata"
)

func possessionTestGame(typ int16) *game {
	m := predictionTestMap()
	for i := range m.Vertexes {
		m.Vertexes[i].X *= 8
		m.Vertexes[i].Y *= 8
	}
	m.Things = append(m.Things, mapdata.Thing{Type: typ, X: 128, Flags: 7}, mapdata.Thing{Type: 3004, X: 384, Flags: 7})
	return newGame(m, Options{Headless: true, SkillLevel: 3, PlayAsMonster: true, SourcePortMode: true})
}

func possessTestBody(t *testing.T, g *game) {
	t.Helper()
	g.possessionCommand(moveCmd{}, false, true)
	if g.possession == nil || g.possession.thing != 1 {
		t.Fatal("aim + fire did not possess the first monster")
	}
}

func TestPossessionTransfersCommandsAndReturnsWithoutChangingMarine(t *testing.T) {
	g := possessionTestGame(3001)
	marine, stats, inv := g.p, g.stats, g.inventory
	hp := g.thingHP[1]
	possessTestBody(t, g)
	// Holding the selecting click must not immediately launch an attack.
	g.possessionCommand(moveCmd{}, false, true)
	g.tickThingThinker(1, g.m.Things[1])
	if len(g.projectiles) != 0 {
		t.Fatal("selecting click fired a projectile")
	}
	g.possessionCommand(moveCmd{forward: 50}, false, false)
	g.tickThingThinker(1, g.m.Things[1])
	x, y := g.thingPosFixed(1, g.m.Things[1])
	if x <= 128*fracUnit || math.Abs(float64(y)) > fracUnit/100 {
		t.Fatalf("monster did not move forward: %d,%d", x, y)
	}
	if g.p != marine || g.stats != stats || g.inventory.ReadyWeapon != inv.ReadyWeapon {
		t.Fatal("possession changed marine state")
	}
	if g.thingHP[1] != hp {
		t.Fatal("possession damaged its target")
	}
	g.possessionCommand(moveCmd{}, true, false)
	if g.possession != nil || g.p != marine {
		t.Fatal("Use did not return to the original marine")
	}
	rx, ry := g.thingPosFixed(1, g.m.Things[1])
	if rx != x || ry != y {
		t.Fatal("return reset the monster position")
	}
}

func TestPossessionFireShootsAlongAimAndUsesMonsterSource(t *testing.T) {
	g := possessionTestGame(3001)
	possessTestBody(t, g)
	g.possessionCommand(moveCmd{}, false, false)
	g.tickThingThinker(1, g.m.Things[1])
	g.possessionCommand(moveCmd{}, false, true)
	g.tickThingThinker(1, g.m.Things[1])
	if len(g.projectiles) != 1 {
		t.Fatalf("projectiles=%d want 1", len(g.projectiles))
	}
	p := g.projectiles[0]
	if p.sourcePlayer || p.sourceThing != 1 || p.sourceType != 3001 || p.vx <= 0 || math.Abs(float64(p.vy)) > fracUnit/100 {
		t.Fatalf("wrong monster projectile: %+v", p)
	}
	for range 30 {
		g.tickProjectileByOrder(p.order)
	}
	if g.thingHP[2] >= 20 {
		t.Fatal("monster fireball did not damage the aimed enemy")
	}
	if g.stats.Bullets != 50 {
		t.Fatal("monster attack consumed marine ammunition")
	}
}

func TestPossessionMeleeAndHitscanDamageTargets(t *testing.T) {
	for _, typ := range []int16{3002, 9, 3005} {
		t.Run(monsterTestTypeName(typ), func(t *testing.T) {
			g := possessionTestGame(typ)
			possessTestBody(t, g)
			if typ != 9 {
				g.setThingPosFixed(2, 166*fracUnit, 0)
			}
			for range 8 {
				g.firePossessedMonster(1, g.m.Things[1])
			}
			if g.thingHP[2] >= 20 {
				t.Fatal("monster attack did not damage target")
			}
			if g.stats.Health != 100 {
				t.Fatal("aimed monster attack damaged the marine behind it")
			}
		})
	}
}

func monsterTestTypeName(typ int16) string {
	switch typ {
	case 3002:
		return "pinky"
	case 3005:
		return "cacodemon"
	default:
		return "shotgunner"
	}
}

func TestPossessionCannotPassWallsOrPossessBarrels(t *testing.T) {
	g := possessionTestGame(3001)
	g.lines = append(g.lines, physLine{idx: 0, x1: 64 * fracUnit, y1: -1024 * fracUnit, x2: 64 * fracUnit, y2: 1024 * fracUnit, dx: 0, dy: 2048 * fracUnit, sideNum0: 0, sideNum1: -1})
	g.possessionCommand(moveCmd{}, false, true)
	if g.possession != nil {
		t.Fatal("possessed through a solid wall")
	}
	g = possessionTestGame(2035)
	g.possessionCommand(moveCmd{}, false, true)
	if g.possession != nil {
		t.Fatal("possessed a barrel")
	}
}

func TestPossessedBodyUsesMonsterCollisionAndStopsAI(t *testing.T) {
	g := possessionTestGame(3001)
	possessTestBody(t, g)
	g.possessionCommand(moveCmd{}, false, false)
	for range 40 {
		g.tickThingThinker(1, g.m.Things[1])
	}
	x, y := g.thingPosFixed(1, g.m.Things[1])
	if x != 128*fracUnit || y != 0 || g.stats.Health != 100 {
		t.Fatal("possessed body ran autonomous AI")
	}
	g.setThingPosFixed(1, 1000*fracUnit, 0)
	g.possessionCommand(moveCmd{forward: 50}, false, false)
	for range 30 {
		g.tickThingThinker(1, g.m.Things[1])
	}
	x, _ = g.thingPosFixed(1, g.m.Things[1])
	if x+monsterRadius(3001) > 1024*fracUnit {
		t.Fatal("controlled body crossed the arena wall")
	}
}

func TestPossessionCameraUsesBodyAndHidesOwnSprite(t *testing.T) {
	g := possessionTestGame(3001)
	possessTestBody(t, g)
	g.prepareRenderStateAt(time.Now())
	if g.renderPX != 128 || g.renderPY != 0 {
		t.Fatalf("camera=%f,%f want 128,0", g.renderPX, g.renderPY)
	}
	if eye, ok := g.possessionEyeZ(); !ok || eye != float64(monsterHeight(3001)*3/4)/fracUnit {
		t.Fatalf("eye=%f ok=%v", eye, ok)
	}
	if _, _, ok := g.monsterSpriteRefForView(1, g.m.Things[1], 0, 128, 0); ok {
		t.Fatal("controlled body renders over its own camera")
	}
}

func TestPossessionReturnsOnDeathAndSaveLoad(t *testing.T) {
	for _, reason := range []string{"monster-death", "marine-death", "load"} {
		t.Run(reason, func(t *testing.T) {
			g := possessionTestGame(3001)
			possessTestBody(t, g)
			switch reason {
			case "monster-death":
				g.damageMonsterFrom(1, 1000, true, -1, 0, 0, false)
			case "marine-death":
				g.isDead = true
			case "load":
				restoreGameSaveState(g, captureGameSaveState(g))
			}
			g.finishPossessionTic()
			if g.possession != nil {
				t.Fatal("possession remained active")
			}
		})
	}
}

func TestPossessionDisabledForRecordedAndNetworkGames(t *testing.T) {
	for _, name := range []string{"record", "demo", "coop", "authority"} {
		t.Run(name, func(t *testing.T) {
			g := possessionTestGame(3001)
			switch name {
			case "record":
				g.opts.RecordDemoPath = "test.lmp"
			case "demo":
				g.opts.DemoScript = &DemoScript{}
			case "coop":
				g.opts.GameMode = "coop"
			case "authority":
				g.authorityPlayers = []*authoritativePlayerState{{}}
			}
			_, _, fire := g.possessionCommand(moveCmd{}, false, true)
			if !fire || g.possession != nil || g.possessionModeEnabled() {
				t.Fatal("sandbox intercepted a normal demo/network command")
			}
		})
	}
}

func TestTypedIDMonsterTogglesModeAndReleasesBody(t *testing.T) {
	g := possessionTestGame(3001)
	possessTestBody(t, g)
	g.input.inputChars = []rune("idmonster")
	g.consumeTypedCheatInput()
	if g.opts.PlayAsMonster || g.possession != nil {
		t.Fatal("IDMONSTER did not disable possession")
	}
	g.input.inputChars = []rune("idmonster")
	g.consumeTypedCheatInput()
	if !g.possessionModeEnabled() {
		t.Fatal("IDMONSTER did not enable possession")
	}
}

func TestPossessionRunsThroughGameplayTics(t *testing.T) {
	g := possessionTestGame(3001)
	marineX, marineY := g.p.x, g.p.y
	g.runGameplayTic(moveCmd{}, false, true)
	if g.possession == nil {
		t.Fatal("gameplay fire failed to possess")
	}
	g.runGameplayTic(moveCmd{forward: 50}, false, false)
	g.runGameplayTic(moveCmd{}, false, true)
	if g.stats.Bullets != 50 || len(g.projectiles) != 1 {
		t.Fatal("gameplay did not route attack to monster")
	}
	if g.p.x != marineX || g.p.y != marineY {
		t.Fatal("monster input moved marine")
	}
	g.runGameplayTic(moveCmd{}, true, false)
	if g.possession != nil {
		t.Fatal("gameplay Use failed to return")
	}
}

func TestPossessionAllMonsterAttacks(t *testing.T) {
	for _, typ := range []int16{3001, 3002, 58, 3003, 69, 3004, 9, 65, 84, 7, 3005, 3006, 66, 67, 68, 16, 64, 71} {
		g := possessionTestGame(typ)
		possessTestBody(t, g)
		g.firePossessedMonster(1, g.m.Things[1])
	}
}
