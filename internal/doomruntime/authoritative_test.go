package doomruntime

import (
	"reflect"
	"testing"

	"gddoom/internal/demo"
	"gddoom/internal/mapdata"
	"gddoom/internal/wad"
)

func testAuthority(t *testing.T) *Authority {
	t.Helper()
	wf, err := wad.Open(findDOOM1WAD(t))
	if err != nil {
		t.Fatal(err)
	}
	m, err := mapdata.LoadMap(wf, "E1M1")
	if err != nil {
		t.Fatal(err)
	}
	a, err := NewAuthority(m, Options{SkillLevel: 3, NoMonsters: true, SFXVolume: 1})
	if err != nil {
		t.Fatal(err)
	}
	return a
}

func TestAuthorityDrivesTwoPlayersWithoutWaitingForMissingInput(t *testing.T) {
	a := testAuthority(t)
	for _, id := range []byte{2, 1} {
		if err := a.AddPlayer(id); err != nil {
			t.Fatal(err)
		}
	}
	before := a.PlayerStates()
	if len(before) != 2 || before[0].ID != 1 || before[1].ID != 2 {
		t.Fatalf("players are not in canonical order: %+v", before)
	}
	for range 35 {
		if err := a.Step(map[byte]demo.Tic{1: {Forward: 25}}); err != nil {
			t.Fatal(err)
		}
	}
	after := a.PlayerStates()
	if a.Tic() != 35 {
		t.Fatalf("tic=%d, want 35", a.Tic())
	}
	if before[0].X == after[0].X && before[0].Y == after[0].Y {
		t.Fatal("commanded player did not move")
	}
	if before[1].X != after[1].X || before[1].Y != after[1].Y {
		t.Fatal("player with missing input moved")
	}
	for range 35 {
		if err := a.Step(map[byte]demo.Tic{1: {Buttons: demo.ButtonAttack}}); err != nil {
			t.Fatal(err)
		}
	}
	after = a.PlayerStates()
	if after[0].Bullets >= before[0].Bullets || after[1].Bullets != before[1].Bullets {
		t.Fatalf("weapons share ammo or did not fire: before=%+v, after=%+v", before, after)
	}
	if a.g.snd.player != nil || a.g.snd.pcSpeaker != nil {
		t.Fatal("authority opened audio hardware")
	}
}

func TestAuthorityRejectsInvalidInputBeforeAdvancing(t *testing.T) {
	a := testAuthority(t)
	if err := a.Step(nil); err == nil || a.Tic() != 0 {
		t.Fatal("empty authority advanced")
	}
	if err := a.AddPlayer(1); err != nil {
		t.Fatal(err)
	}
	before := a.PlayerStates()
	for _, cmd := range []map[byte]demo.Tic{
		{2: {Forward: 25}},
		{1: {Forward: 51}},
		{1: {Buttons: demo.ButtonSpecial}},
	} {
		if err := a.Step(cmd); err == nil {
			t.Fatalf("accepted invalid command %+v", cmd)
		}
		if a.Tic() != 0 || !reflect.DeepEqual(before, a.PlayerStates()) {
			t.Fatal("rejected command mutated simulation")
		}
	}
	// Doom permits combined strafe input up to 80; match validation and the
	// simulation facade must agree or a valid packet could terminate a match.
	if err := a.Step(map[byte]demo.Tic{1: {Side: 80}}); err != nil {
		t.Fatalf("rejected valid combined strafe input: %v", err)
	}
}

func TestAuthorityMembershipAndDiagnosticOwnership(t *testing.T) {
	a := testAuthority(t)
	for _, id := range []byte{0, 5} {
		if err := a.AddPlayer(id); err == nil {
			t.Fatalf("accepted invalid slot %d", id)
		}
	}
	if err := a.AddPlayer(2); err != nil {
		t.Fatal(err)
	}
	if err := a.AddPlayer(2); err == nil {
		t.Fatal("accepted duplicate slot")
	}
	states := a.PlayerStates()
	states[0].Weapons[0] = "changed"
	if a.PlayerStates()[0].Weapons[0] == "changed" {
		t.Fatal("diagnostic weapon list aliases simulation")
	}
	if err := a.Step(nil); err != nil {
		t.Fatal(err)
	}
	a.RemovePlayer(2)
	if len(a.PlayerStates()) != 0 {
		t.Fatal("removed slot still active")
	}
	if err := a.Step(nil); err == nil || a.Tic() != 1 {
		t.Fatal("empty authority advanced after last player left")
	}
}

func TestAuthorityRejectsLocalOnlyConfiguration(t *testing.T) {
	if _, err := NewAuthority(nil, Options{}); err == nil {
		t.Fatal("accepted nil map")
	}
	m := &mapdata.Map{Sectors: []mapdata.Sector{{}}}
	for _, opts := range []Options{
		{GameMode: "single"},
		{Invulnerable: true},
		{AllCheats: true},
		{CheatLevel: 1},
		{DemoScript: &demo.Script{}},
	} {
		if _, err := NewAuthority(m, opts); err == nil {
			t.Fatalf("accepted incompatible options %+v", opts)
		}
	}
}
