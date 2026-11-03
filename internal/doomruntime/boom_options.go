package doomruntime

import (
	"gddoom/internal/demo"
	"gddoom/internal/doomrand"
	"gddoom/internal/mapdata"
)

func demoRNGOptions(m *mapdata.Map, opts Options) (demo.GameOptions, bool) {
	if opts.DemoScript != nil {
		return opts.DemoScript.Header.GameOptions()
	}
	if opts.BoomGameOptions != nil {
		return *opts.BoomGameOptions, true
	}
	return demo.GameOptions{Seed: doomrand.DefaultBoomSeed}, boomMomentumDropoffEnabled(m, opts)
}
func captureBoomOptions(opts Options) []byte {
	if opts.BoomGameOptions == nil {
		return nil
	}
	raw := opts.BoomGameOptions.Bytes()
	return append([]byte(nil), raw[:]...)
}
func restoreBoomOptions(opts *Options, raw []byte) {
	opts.BoomGameOptions = nil
	if len(raw) == 64 {
		var block [64]byte
		copy(block[:], raw)
		settings := demo.DecodeGameOptions(block)
		opts.BoomGameOptions = &settings
	}
}
func restoreSavedRNG(file saveFile) {
	if file.Version == saveGameVersion || file.Version == keyframeVersion {
		doomrand.Restore(file.RNG.Extended)
	} else {
		doomrand.Clear()
		doomrand.SetState(file.RNG.MenuIndex, file.RNG.PlayIndex)
	}
}

// Enemy references use zero for null, positive thing-index+1, negative player
// slots. Sparse allocation preserves old snapshots without memory state.
func (g *game) boomRememberEnabled() bool {
	return g.boomMomentumDropoff && (g.opts.BoomGameOptions == nil || g.opts.BoomGameOptions.MonstersRemember)
}
func (g *game) boomEnemyAlive(ref int) bool {
	if ref < 0 {
		if p := g.authoritativePlayerForSlot(-ref); p != nil {
			return p.stats.Health > 0
		}
		return -ref == g.localSlot && g.stats.Health > 0 && !g.isDead
	}
	i := ref - 1
	return ref > 0 && i < len(g.thingHP) && g.thingHP[i] > 0
}
func (g *game) rememberBoomEnemy(i int) {
	if !g.boomRememberEnabled() {
		return
	}
	if len(g.boomLastEnemy) != len(g.m.Things) {
		next := make([]int, len(g.m.Things))
		copy(next, g.boomLastEnemy)
		g.boomLastEnemy = next
	}
	previous := g.boomLastEnemy[i]
	if previous < 0 && g.boomEnemyAlive(previous) {
		return
	}
	ref := 0
	if i < len(g.thingTargetPlayer) && g.thingTargetPlayer[i] {
		slot := g.localSlot
		if i < len(g.thingTargetPlayerSlot) && g.thingTargetPlayerSlot[i] > 0 {
			slot = g.thingTargetPlayerSlot[i]
		}
		ref = -slot
	} else if i < len(g.thingTargetIdx) && g.thingTargetIdx[i] >= 0 {
		ref = g.thingTargetIdx[i] + 1
	}
	g.boomLastEnemy[i] = ref
}
func (g *game) recallBoomEnemy(i int) bool {
	if !g.boomRememberEnabled() || i < 0 || i >= len(g.boomLastEnemy) {
		return false
	}
	ref := g.boomLastEnemy[i]
	if !g.boomEnemyAlive(ref) {
		return false
	}
	g.boomLastEnemy[i] = 0
	if ref < 0 {
		g.setMonsterTargetPlayer(i)
		if i < len(g.thingTargetPlayerSlot) {
			g.thingTargetPlayerSlot[i] = -ref
		}
	} else {
		g.setMonsterTargetThing(i, ref-1)
	}
	return true
}
func validBoomEnemyMemory(refs []int, n int) bool {
	if len(refs) != 0 && len(refs) != n {
		return false
	}
	for _, ref := range refs {
		if ref < -4 || ref > n {
			return false
		}
	}
	return true
}
