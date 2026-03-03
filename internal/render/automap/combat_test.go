package automap

import (
	"testing"

	"gddoom/internal/mapdata"
)

func TestPickHitscanMonsterTarget(t *testing.T) {
	g := &game{
		m: &mapdata.Map{
			Things: []mapdata.Thing{
				{Type: 3004, X: 64, Y: 0},
				{Type: 3004, X: 96, Y: 32},
			},
		},
		thingCollected: []bool{false, false},
		thingHP:        []int{20, 20},
		p:              player{x: 0, y: 0, angle: degToAngle(0)},
	}
	idx, ok := g.pickHitscanMonsterTarget()
	if !ok {
		t.Fatal("expected a target")
	}
	if idx != 0 {
		t.Fatalf("target idx=%d want=0", idx)
	}
}

func TestHandleFireConsumesAmmoAndDamages(t *testing.T) {
	g := &game{
		m: &mapdata.Map{
			Things: []mapdata.Thing{{Type: 3004, X: 64, Y: 0}},
		},
		thingCollected: []bool{false},
		thingHP:        []int{20},
		p:              player{x: 0, y: 0, angle: degToAngle(0)},
		stats:          playerStats{Bullets: 10},
	}
	g.handleFire()
	if g.stats.Bullets != 9 {
		t.Fatalf("bullets=%d want=9", g.stats.Bullets)
	}
	if g.thingHP[0] >= 20 {
		t.Fatalf("monster hp=%d want < 20", g.thingHP[0])
	}
}

func TestHandleFireNoAmmo(t *testing.T) {
	g := &game{
		stats: playerStats{Bullets: 0},
	}
	g.handleFire()
	if g.useText != "No ammo" {
		t.Fatalf("message=%q want=No ammo", g.useText)
	}
}
