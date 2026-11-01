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

type environmentProbeCase struct {
	Name, Map, Actor, Mode    string
	X, Y, Z, MomX, MomY, MomZ int64
	Forward, Side             int64
	Tics, Disable, Enable     int
	Noclip                    bool
}

func TestBoomEnvironmentCapture(t *testing.T) {
	path := os.Getenv("GD_ENV_CASES")
	if path == "" {
		t.Skip("run scripts/verify_boom_environment.py")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var cases []environmentProbeCase
	if err = json.Unmarshal(data, &cases); err != nil {
		t.Fatal(err)
	}
	wf, err := wad.Open(os.Getenv("GD_ENV_WAD"))
	if err != nil {
		t.Fatal(err)
	}
	out, err := os.Create(os.Getenv("GD_ENV_OUT"))
	if err != nil {
		t.Fatal(err)
	}
	defer out.Close()
	fmt.Fprintln(out, "name,tic,x,y,z,momx,momy,momz,friction,movefactor")
	for _, c := range cases {
		m, e := mapdata.LoadMap(wf, mapdata.MapName(c.Map))
		if e != nil {
			t.Fatal(e)
		}
		if c.Actor == "caco" {
			m.Things[1].Type = 3005
		}
		g := newGame(m, Options{SkillLevel: 3})
		g.noClip = c.Noclip
		original := make([]int16, len(m.Sectors))
		for i, s := range m.Sectors {
			original[i] = s.Special
		}
		setFlags := func(on bool) {
			for i := range g.m.Sectors {
				if on {
					g.m.Sectors[i].Special = original[i]
				} else {
					g.m.Sectors[i].Special &^= 0x300
				}
			}
		}
		if c.Enable >= 0 {
			setFlags(false)
		}
		floor, ceil, ok := g.teleportDestinationHeights(c.X, c.Y)
		if !ok {
			t.Fatal("bad fixture position")
		}
		g.thingStateTics[1] = 100000
		if c.Actor == "player" {
			g.setPlayerPosFixed(c.X, c.Y)
			g.p.z, g.p.floorz, g.p.ceilz = c.Z, floor, ceil
			g.p.momx, g.p.momy, g.p.momz = c.MomX, c.MomY, c.MomZ
			g.p.viewHeight = playerViewHeight
			g.playerViewZ = c.Z + playerViewHeight
		} else {
			g.setThingPosFixed(1, c.X, c.Y)
			g.setThingSupportState(1, c.Z, floor, ceil)
			g.setThingMomentum(1, c.MomX, c.MomY, c.MomZ)
		}
		for tic := 0; tic <= c.Tics; tic++ {
			if tic > 0 {
				if tic-1 == c.Disable {
					setFlags(false)
				}
				if tic-1 == c.Enable {
					setFlags(true)
				}
				if c.Mode == "force" {
					for _, f := range g.boomFrictions {
						g.tickBoomFriction(f)
					}
					for _, p := range g.boomPushers {
						g.tickBoomPusher(p)
					}
				} else {
					g.runGameplayTic(moveCmd{forward: c.Forward, side: c.Side}, false, false)
				}
			}
			x, y, z, mx, my, mz := g.p.x, g.p.y, g.p.z, g.p.momx, g.p.momy, g.p.momz
			f, mf := storedBoomFriction(g.p.friction), g.p.movefactor
			if c.Actor != "player" {
				th := g.m.Things[1]
				x, y = g.thingPosFixed(1, th)
				z, _, _ = g.thingSupportState(1, th)
				mx, my, mz = g.thingMomX[1], g.thingMomY[1], g.thingMomZ[1]
				f, mf = friction, 0
			}
			fmt.Fprintf(out, "%s,%d,%d,%d,%d,%d,%d,%d,%d,%d\n", c.Name, tic, x, y, z, mx, my, mz, f, mf)
		}
	}
}
