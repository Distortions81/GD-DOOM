//go:build integration

package doomruntime

import (
	"encoding/json"
	"fmt"
	"os"
	"testing"

	"gddoom/internal/mapdata"
	"gddoom/internal/wad"
)

type teleportProbeCase struct {
	Name, Map, Actor                        string
	Source, Destination, Special            int
	Back, Blocked, Missing                  bool
	X, NextX, NextY, Z, Y, MomX, MomY, MomZ int64
	Angle                                   uint32
	Ticks                                   int
}

// TestBoomTeleportCapture exercises spawned monsters and the live movement /
// crossing paths. It exports integer state for scripts/verify_boom_teleports.py.
func TestBoomTeleportCapture(t *testing.T) {
	path := os.Getenv("GD_TELEPORT_CASES")
	if path == "" {
		t.Skip("run scripts/verify_boom_teleports.py")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var cases []teleportProbeCase
	if err = json.Unmarshal(data, &cases); err != nil {
		t.Fatal(err)
	}
	wf, err := wad.Open(os.Getenv("GD_TELEPORT_WAD"))
	if err != nil {
		t.Fatal(err)
	}
	out, err := os.Create(os.Getenv("GD_TELEPORT_OUT"))
	if err != nil {
		t.Fatal(err)
	}
	defer out.Close()
	fmt.Fprintln(out, "name,moved,x,y,z,floor,ceil,angle,momx,momy,momz,reaction,special,blocker_alive,fogs")
	for _, c := range cases {
		m, err := mapdata.LoadMap(wf, mapdata.MapName(c.Map))
		if err != nil {
			t.Fatal(err)
		}
		if c.Actor == "caco" {
			m.Things[1].Type = 3005
		}
		g := newGame(m, Options{SkillLevel: 3})
		g.lineSpecial[c.Source] = uint16(c.Special)
		if c.Missing {
			g.m.Linedefs[c.Source].Tag = 999
		}
		if c.Actor == "player" || c.Actor == "missile" {
			g.setThingPosFixed(1, -704*fracUnit, 0)
		}
		if c.Blocked {
			bx, by := int64(448*fracUnit), -c.Y
			if c.Special >= 262 && c.Special <= 265 {
				by = c.Y
			}
			if c.Special >= 207 && c.Special <= 210 || c.Special >= 268 {
				bx, by = 640*fracUnit, 128*fracUnit
			}
			g.setThingPosFixed(2, bx, by)
			f, ceil, _ := g.teleportDestinationHeights(bx, by)
			g.setThingSupportState(2, f, f, ceil)
		}
		x := c.X
		next := c.NextX
		if c.Back {
			x, next = next, x
		}
		var moved bool
		var z, floor, ceil, mx, my, mz int64
		angle := c.Angle
		reaction := 7
		if c.Actor == "player" {
			g.setPlayerPosFixed(x, c.Y)
			g.p.z, g.p.floorz, g.p.ceilz = c.Z, 0, 256*fracUnit
			g.p.angle, g.p.momx, g.p.momy, g.p.momz, g.p.reactionTime = c.Angle, c.MomX, c.MomY, c.MomZ, 7
			if c.Special == 209 || c.Special == 210 {
				g.useSpecialLineForActorIndex(c.Source, b2i(c.Back), true, -1)
				moved = true
			} else {
				moved = g.tryMove(next, c.NextY)
			}
			x, z, floor, ceil, angle, mx, my, mz, reaction = g.p.x, g.p.z, g.p.floorz, g.p.ceilz, g.p.angle, g.p.momx, g.p.momy, g.p.momz, g.p.reactionTime
			c.Y = g.p.y
		} else if c.Actor == "missile" {
			if c.Special == 209 || c.Special == 210 {
				g.useSpecialLineForActorIndex(c.Source, b2i(c.Back), false, -1)
			} else {
				g.checkProjectileWalkSpecialLines(x, c.Y, next, c.NextY, projectile{kind: projectilePlasmaBall, sourceType: 68, radius: 13 * fracUnit})
				x, c.Y = next, c.NextY
			}
			moved = true
			z, floor, ceil, mx, my, mz = c.Z, 0, 256*fracUnit, c.MomX, c.MomY, c.MomZ
		} else {
			th := g.m.Things[1]
			g.setThingPosFixed(1, x, c.Y)
			g.setThingSupportState(1, c.Z, 0, 256*fracUnit)
			g.setThingWorldAngle(1, c.Angle)
			g.setThingMomentum(1, c.MomX, c.MomY, c.MomZ)
			g.thingReactionTics[1] = 7
			if c.Special == 209 || c.Special == 210 {
				g.useSpecialLineForActorIndex(c.Source, b2i(c.Back), false, 1)
				for tick := 0; tick < c.Ticks; tick++ {
					g.tickMonsterMomentum(1, th)
				}
				moved = true
			} else if c.Ticks > 0 {
				for tick := 0; tick < c.Ticks; tick++ {
					g.tickMonsterMomentum(1, th)
				}
				moved = true
			} else {
				// This is the successful P_TryMove commit used by tickMonsterMomentum,
				// stopped immediately after crossing to inspect unmodified exit state.
				f, ce, lines, ok := g.tryMoveProbeMonster(1, th.Type, next, c.NextY)
				moved = ok
				if ok {
					g.setThingPosFixed(1, next, c.NextY)
					g.setThingSupportState(1, c.Z, f, ce)
					g.checkWalkSpecialLinesForActorWithCandidates(x, c.Y, next, c.NextY, 1, false, lines)
				}
			}
			x, c.Y = g.thingPosFixed(1, th)
			z, floor, ceil = g.thingSupportState(1, th)
			angle = g.thingWorldAngle(1, th)
			mx, my, mz = g.thingMomX[1], g.thingMomY[1], g.thingMomZ[1]
			reaction = g.thingReactionTics[1]
		}
		fmt.Fprintf(out, "%s,%d,%d,%d,%d,%d,%d,%d,%d,%d,%d,%d,%d,%d,%d\n", c.Name, b2i(moved), x, c.Y, z, floor, ceil, angle, mx, my, mz, reaction, g.lineSpecial[c.Source], b2i(g.thingHP[2] > 0), len(g.hitscanPuffs))
	}
}
