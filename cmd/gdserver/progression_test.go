package main

import (
	"bytes"
	"math"
	"path/filepath"
	"testing"

	"gddoom/internal/demo"
	"gddoom/internal/doomruntime"
	"gddoom/internal/mapdata"
	"gddoom/internal/netgame"
	"gddoom/internal/wad"
)

func progressionWAD(t *testing.T) *wad.File {
	t.Helper()
	f, err := wad.Open(filepath.Join("..", "..", "DOOM1.WAD"))
	if err != nil {
		t.Fatal(err)
	}
	return f
}

func TestDeathmatchDefaultProgressionAndExplicitRotation(t *testing.T) {
	f := progressionWAD(t)
	for _, tc := range []struct {
		name, current, want string
		secret              bool
		rotation            []mapdata.MapName
		index, wantIndex    int
	}{
		{name: "normal exit", current: "E1M1", want: "E1M2", index: -1, wantIndex: -1},
		{name: "secret exit", current: "E1M3", secret: true, want: "E1M9", index: -1, wantIndex: -1},
		{name: "secret return", current: "E1M9", want: "E1M4", index: -1, wantIndex: -1},
		{name: "episode finale", current: "E1M8", index: -1, wantIndex: -1},
		{name: "explicit arena", current: "E1M1", want: "E1M1", rotation: []mapdata.MapName{"E1M1"}},
		{name: "explicit rotation", current: "E1M1", want: "E1M3", rotation: []mapdata.MapName{"E1M1", "E1M3"}, wantIndex: 1},
		{name: "rotation overrides secret", current: "E1M3", secret: true, want: "E1M1", rotation: []mapdata.MapName{"E1M1", "E1M3"}, index: 1},
		{name: "rotation overrides finale", current: "E1M8", want: "E1M1", rotation: []mapdata.MapName{"E1M8", "E1M1"}, wantIndex: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			next, index, err := nextServerMap(f, mapdata.MapName(tc.current), tc.secret, true, tc.rotation, tc.index)
			if err != nil || string(next) != tc.want || index != tc.wantIndex {
				t.Fatalf("next=%s index=%d err=%v; want %s/%d", next, index, err, tc.want, tc.wantIndex)
			}
		})
	}
	// An exhausted custom one-map pack should finish cleanly, while an invalid
	// current map still reports a configuration error instead of hiding it.
	single := &wad.File{Lumps: []wad.Lump{{Name: "E1M1"}}}
	for _, name := range []string{"THINGS", "LINEDEFS", "SIDEDEFS", "VERTEXES", "SEGS", "SSECTORS", "NODES", "SECTORS", "REJECT", "BLOCKMAP"} {
		single.Lumps = append(single.Lumps, wad.Lump{Name: name})
	}
	if next, _, err := nextServerMap(single, "E1M1", false, true, nil, -1); err != nil || next != "" {
		t.Fatalf("exhausted pack: next=%s err=%v", next, err)
	}
	if _, _, err := nextServerMap(single, "E1M2", false, true, nil, -1); err == nil {
		t.Fatal("unknown current map silently completed")
	}
}

func TestDeathmatchRealExitUseAdvancesBothPlayersWithoutRotation(t *testing.T) {
	f := progressionWAD(t)
	m, err := mapdata.LoadMap(f, "E1M1")
	if err != nil {
		t.Fatal(err)
	}
	// Fixture setup skips walking E1M1: move the first deathmatch start beside
	// its real exit switch. Exit activation still comes from a scheduled Use
	// command; neither the authority nor its completion flags are mutated.
	placed := false
	for _, line := range m.Linedefs {
		if line.Special != 11 {
			continue
		}
		v1, v2 := m.Vertexes[line.V1], m.Vertexes[line.V2]
		dx, dy := float64(v2.X-v1.X), float64(v2.Y-v1.Y)
		length := math.Hypot(dx, dy)
		for i := range m.Things {
			if m.Things[i].Type != 11 {
				continue
			}
			m.Things[i].X = int16(math.Round(float64(v1.X+v2.X)/2 + dy*24/length))
			m.Things[i].Y = int16(math.Round(float64(v1.Y+v2.Y)/2 - dx*24/length))
			m.Things[i].Angle = int16(math.Mod(math.Atan2(dx, -dy)*180/math.Pi+360, 360))
			placed = true
			break
		}
		break
	}
	if !placed {
		t.Fatal("real map has no exit/deathmatch fixture")
	}
	a, err := doomruntime.NewAuthority(m, doomruntime.Options{GameMode: "deathmatch", SkillLevel: 3, NoMonsters: true})
	if err != nil {
		t.Fatal(err)
	}
	match, err := netgame.NewMatch(a, a, netgame.MatchConfig{Epoch: 1, Compatibility: "exit-test", PlayerLimit: 2, FutureTicks: 8, DisconnectTicks: 70, SnapshotInterval: 1})
	if err != nil {
		t.Fatal(err)
	}
	ids := make([]netgame.ConnectionID, 2)
	for i := range ids {
		id, _, err := match.Join(netgame.Hello{Compatibility: "exit-test", Name: []string{"Alice", "Bob"}[i]})
		if err != nil {
			t.Fatal(err)
		}
		ids[i] = id
	}
	// Reborn players suppress a held Use button until it has been released.
	if _, err := match.Step(); err != nil {
		t.Fatal(err)
	}
	if err := match.Submit(ids[0], netgame.InputBatch{Epoch: 1, Inputs: []netgame.Input{{Sequence: 1, Tick: 2, Command: demo.Tic{Buttons: demo.ButtonUse}}}}); err != nil {
		t.Fatal(err)
	}
	result, err := match.Step()
	if err != nil || !result.Completed || result.CompletionReason != "map_exit" || len(result.Snapshots) != 2 {
		t.Fatalf("scheduled exit did not complete both players' map: complete=%v reason=%q snapshots=%d players=%+v err=%v", result.Completed, result.CompletionReason, len(result.Snapshots), a.PlayerStates(), err)
	}
	nextName, _, err := nextServerMap(f, a.MapName(), a.SecretExit(), true, nil, -1)
	if err != nil || nextName != "E1M2" {
		t.Fatalf("deathmatch exit repeated map: next=%s err=%v", nextName, err)
	}
	next, err := mapdata.LoadMap(f, nextName)
	if err != nil {
		t.Fatal(err)
	}
	if err := a.AdvanceMap(next); err != nil {
		t.Fatal(err)
	}
	welcomes, err := match.ResetEpoch(2, "next-map-test")
	if err != nil {
		t.Fatal(err)
	}
	result, err = match.Step()
	if err != nil || result.Completed || len(result.Snapshots) != 2 {
		t.Fatalf("new map did not resume both players: complete=%v snapshots=%d err=%v", result.Completed, len(result.Snapshots), err)
	}
	for i, id := range ids {
		viewer := byte(i + 1)
		baseline, err := a.Snapshot(viewer)
		if err != nil {
			t.Fatal(err)
		}
		snapshot := result.Snapshots[id]
		if welcomes[id].PlayerID != viewer || welcomes[id].Epoch != 2 || a.MapName() != "E1M2" || snapshot.Epoch != 2 || snapshot.Tick != a.Tic() || !bytes.Equal(snapshot.State, baseline) {
			t.Fatalf("player %d lost slot or current next-map baseline: %+v map=%s tic=%d", viewer, welcomes[id], a.MapName(), snapshot.Tick)
		}
	}
}
