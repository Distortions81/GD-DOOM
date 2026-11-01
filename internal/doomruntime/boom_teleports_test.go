package doomruntime

import (
	"testing"

	"gddoom/internal/mapdata"
	"gddoom/internal/wad"
)

func teleportFixtureGame(t *testing.T) (*game, int, int) {
	t.Helper()
	f, err := wad.Open("../../testdata/boom/teleports.wad")
	if err != nil {
		t.Fatal(err)
	}
	m, err := mapdata.LoadMap(f, "MAP01")
	if err != nil {
		t.Fatal(err)
	}
	g := newGame(m, Options{SkillLevel: 3})
	source, dest := -1, -1
	for i, l := range m.Linedefs {
		if l.Special == 244 {
			source = i
		} else if l.Tag == 17 {
			dest = i
		}
	}
	if source < 0 || dest < 0 {
		t.Fatal("fixture missing teleport lines")
	}
	return g, source, dest
}

func TestSilentLineTeleportCrossingMonsterAndPlayerGates(t *testing.T) {
	for _, special := range []uint16{207, 208, 243, 244, 262, 263, 264, 265, 266, 267, 268, 269} {
		if !walkSpecialAllowedForNonPlayer(special) {
			t.Fatalf("monster cannot cross %d", special)
		}
		for _, isPlayer := range []bool{true, false} {
			g, source, _ := teleportFixtureGame(t)
			g.lineSpecial[source] = special
			oldX, oldY := int64(-449*fracUnit), int64(64*fracUnit)
			next := int64(-447 * fracUnit)
			if isPlayer {
				g.setThingPosFixed(1, -704*fracUnit, 0)
				g.setPlayerPosFixed(oldX, oldY)
				g.p.z = 13 * fracUnit
				g.p.momx = 7 * fracUnit
				g.p.momz = fracUnit
				g.p.reactionTime = 7
				if !g.tryMove(next, oldY) {
					t.Fatal("fixture move blocked")
				}
			} else {
				th := g.m.Things[1]
				g.setThingPosFixed(1, oldX, oldY)
				g.setThingSupportState(1, 13*fracUnit, 0, 256*fracUnit)
				g.setThingMomentum(1, 7*fracUnit, 0, fracUnit)
				g.tickMonsterMomentum(1, th)
			}
			x := g.p.x
			if !isPlayer {
				x, _ = g.thingPosFixed(1, g.m.Things[1])
			}
			allowed := !isPlayer || !mapdata.LookupLineSpecial(special).Teleport.MonsterOnly
			if (x > 0) != allowed {
				t.Fatalf("special %d player=%v allowed=%v x=%d", special, isPlayer, allowed, x)
			}
			wantSpecial := special
			if allowed && !mapdata.LookupLineSpecial(special).Repeat {
				wantSpecial = 0
			}
			if g.lineSpecial[source] != wantSpecial {
				t.Fatalf("special %d player=%v consumed incorrectly: %d", special, isPlayer, g.lineSpecial[source])
			}
			if len(g.hitscanPuffs) != 0 {
				t.Fatal("silent teleport spawned fog")
			}
			if isPlayer && (g.p.momz != fracUnit || g.p.reactionTime != 7) {
				t.Fatal("silent teleport changed vertical momentum/freeze")
			}
		}
	}
}

func TestSilentLineTeleportRejectsBackAndMissingDestination(t *testing.T) {
	for _, special := range []uint16{243, 244, 262, 263, 264, 265, 266, 267, 268, 269} {
		for _, back := range []bool{true, false} {
			g, source, _ := teleportFixtureGame(t)
			g.p.z = 13 * fracUnit
			if !back {
				g.m.Linedefs[source].Tag = 999
			}
			if g.activateTeleportLine(source, b2i(back), *mapdata.LookupLineSpecial(special).Teleport, 1, false) {
				t.Fatalf("accepted invalid teleport %d back=%v", special, back)
			}
		}
	}
}

func TestBoomMomentumDropoffKeepsAIAndVanillaRestrictions(t *testing.T) {
	g, _, _ := teleportFixtureGame(t)
	g.setThingPosFixed(1, 448*fracUnit, 64*fracUnit)
	g.setThingSupportState(1, 77*fracUnit, 32*fracUnit, 256*fracUnit)
	if _, _, _, ok := g.tryMoveProbeMonster(1, 3001, 447*fracUnit, 64*fracUnit); ok {
		t.Fatal("AI walking must still refuse the ledge")
	}
	if _, _, _, ok := g.tryMoveProbeMonsterWithDropoff(1, 3001, 447*fracUnit, 64*fracUnit, true); !ok {
		t.Fatal("Boom momentum must pass the exit ledge")
	}
	if !boomMomentumDropoffEnabled(g.m, Options{}) {
		t.Fatal("Boom map not detected")
	}
	if boomMomentumDropoffEnabled(g.m, Options{DemoScript: &DemoScript{Header: DemoHeader{Version: 109}}}) {
		t.Fatal("vanilla demo changed movement rules")
	}
	if !boomMomentumDropoffEnabled(&mapdata.Map{}, Options{DemoScript: &DemoScript{Header: DemoHeader{Version: 202}}}) {
		t.Fatal("Boom demo must allow momentum dropoff")
	}
	if boomMomentumDropoffEnabled(&mapdata.Map{}, Options{}) {
		t.Fatal("plain Doom map changed movement rules")
	}
}

func TestSilentLineTeleportFirstExitAndSaveRestore(t *testing.T) {
	g, source, dest := teleportFixtureGame(t)
	// A second, clear exit must not be tried after the first eligible one blocks.
	line := g.m.Linedefs[dest]
	line.V1 = uint16(len(g.m.Vertexes))
	line.V2 = line.V1 + 1
	g.m.Vertexes = append(g.m.Vertexes, mapdata.Vertex{X: 640, Y: 256}, mapdata.Vertex{X: 640, Y: -256})
	g.m.Linedefs = append(g.m.Linedefs, line)
	g.setThingPosFixed(1, -447*fracUnit, 64*fracUnit)
	g.setThingSupportState(1, 13*fracUnit, 0, 256*fracUnit)
	g.setThingMomentum(1, 7*fracUnit, -3*fracUnit, fracUnit)
	g.setThingPosFixed(2, 448*fracUnit, -64*fracUnit)
	g.setThingSupportState(2, 32*fracUnit, 32*fracUnit, 256*fracUnit)
	info := *mapdata.LookupLineSpecial(244).Teleport
	if g.activateTeleportLine(source, 0, info, 1, false) {
		t.Fatal("blocked first exit incorrectly fell through")
	}
	g.setThingPosFixed(2, -704*fracUnit, 192*fracUnit)
	if !g.activateTeleportLine(source, 0, info, 1, false) {
		t.Fatal("clear exit failed")
	}
	saved := captureGameSaveState(g)
	restored, _, _ := teleportFixtureGame(t)
	restoreGameSaveState(restored, saved)
	before, after := g.m.Things[1], restored.m.Things[1]
	x, y := g.thingPosFixed(1, before)
	rx, ry := restored.thingPosFixed(1, after)
	z, f, c := g.thingSupportState(1, before)
	rz, rf, rc := restored.thingSupportState(1, after)
	if x != rx || y != ry || z != rz || f != rf || c != rc || g.thingWorldAngle(1, before) != restored.thingWorldAngle(1, after) || g.thingMomX[1] != restored.thingMomX[1] || g.thingMomY[1] != restored.thingMomY[1] || g.thingMomZ[1] != restored.thingMomZ[1] {
		t.Fatal("teleported monster state lost on restore")
	}
}

func TestSilentLineTeleportZeroTagRejected(t *testing.T) {
	for _, special := range []uint16{243, 244, 262, 263, 264, 265, 266, 267, 268, 269} {
		g, source, dest := teleportFixtureGame(t)
		g.m.Linedefs[source].Tag = 0
		g.m.Linedefs[dest].Tag = 0
		if g.activateTeleportLine(source, 0, *mapdata.LookupLineSpecial(special).Teleport, 1, false) {
			t.Fatalf("accepted zero tag %d", special)
		}
	}
}

func TestSilentSwitchTeleportUsesActualMonsterAndSwitchLifetime(t *testing.T) {
	for _, special := range []uint16{209, 210} {
		g, source, _ := teleportFixtureGame(t)
		g.lineSpecial[source] = special
		sd := int(g.m.Linedefs[source].SideNum[0])
		g.m.Sidedefs[sd].Mid = "SW1BRCOM"
		g.setThingPosFixed(1, -449*fracUnit, 64*fracUnit)
		g.setThingSupportState(1, 13*fracUnit, 0, 256*fracUnit)
		g.setThingMomentum(1, 7*fracUnit, -3*fracUnit, fracUnit)
		playerX, playerY := g.p.x, g.p.y
		if !g.useSpecialLineForActorIndex(source, 0, false, 1) {
			t.Fatalf("monster could not use %d", special)
		}
		x, y := g.thingPosFixed(1, g.m.Things[1])
		if x != 640*fracUnit || y != 128*fracUnit || g.p.x != playerX || g.p.y != playerY {
			t.Fatal("monster switch teleported wrong actor")
		}
		if g.m.Sidedefs[sd].Mid != "SW2BRCOM" {
			t.Fatal("silent teleport switch did not animate")
		}
		if special == 209 && g.lineSpecial[source] != 0 {
			t.Fatal("successful S1 silent teleport not consumed")
		}
		if special == 210 {
			if g.lineSpecial[source] != 210 || len(g.delayedSwitchReverts) != 1 {
				t.Fatal("SR switch not retained")
			}
			for tick := 0; tick < switchResetTics; tick++ {
				g.tickDelayedSwitchReverts()
			}
			if g.m.Sidedefs[sd].Mid != "SW1BRCOM" {
				t.Fatal("SR silent teleport button did not restore")
			}
		}
		for _, secret := range []bool{false, true} {
			h, line, _ := teleportFixtureGame(t)
			h.lineSpecial[line] = special
			h.setThingPosFixed(1, -449*fracUnit, 64*fracUnit)
			side := 1
			if secret {
				h.m.Linedefs[line].Flags |= mlSecret
				side = 0
			}
			if h.useSpecialLineForActorIndex(line, side, false, 1) {
				t.Fatal("monster used secret/back-side teleport switch")
			}
		}
	}
}
