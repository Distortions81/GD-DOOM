package doomruntime

// collectWASMSoundOrder keeps the strongest occurrence of each sound kind in
// first-occurrence order. Both local tics and replicated event batches use this
// policy so a network burst cannot repeatedly rewind the same browser voice.
func (g *game) collectWASMSoundOrder(count int, at func(int) (soundEvent, queuedSoundOrigin)) {
	g.resetWASMSoundOrder()
	fullClip := g.m != nil && soundMapUsesFullClip(g.m.Name)
	for idx := 0; idx < count; idx++ {
		ev, origin := at(idx)
		strength := vanillaSoundStrength(g.snd, origin, g.p.x, g.p.y, g.p.angle, fullClip)
		if g.wasmSoundSeen[ev] {
			if strength > g.wasmSoundBestStrength[ev] {
				g.wasmSoundBestIdx[ev] = idx
				g.wasmSoundBestStrength[ev] = strength
			}
			continue
		}
		g.wasmSoundSeen[ev] = true
		g.wasmSoundBestIdx[ev] = idx
		g.wasmSoundBestStrength[ev] = strength
		g.wasmSoundOrder[g.wasmSoundOrderCount] = ev
		g.wasmSoundOrderCount++
	}
}

func (g *game) resetWASMSoundOrder() {
	for i := 0; i < g.wasmSoundOrderCount; i++ {
		ev := g.wasmSoundOrder[i]
		g.wasmSoundSeen[ev] = false
		g.wasmSoundBestIdx[ev] = 0
		g.wasmSoundBestStrength[ev] = 0
	}
	g.wasmSoundOrderCount = 0
}

// visitWASMSoundBatch applies the existing browser voice-start budgets, giving
// weapons, pickups and movers priority over monster vocals. Indexes refer to
// the original events so callers retain their pitch, identity and origin.
func (g *game) visitWASMSoundBatch(count int, at func(int) (soundEvent, queuedSoundOrigin), play func(int, soundEvent)) {
	g.collectWASMSoundOrder(count, at)
	defer g.resetWASMSoundOrder()
	total, vocals := 0, 0
	for pass := 0; pass < 2; pass++ {
		for i := 0; i < g.wasmSoundOrderCount; i++ {
			ev := g.wasmSoundOrder[i]
			vocal := isMonsterVocalSound(ev)
			if (pass == 0 && vocal) || (pass == 1 && !vocal) {
				continue
			}
			if total >= maxSoundEventsPerFlush() {
				return
			}
			if vocal {
				if vocals >= maxMonsterVocalSoundsPerFlush() {
					continue
				}
				vocals++
			}
			play(g.wasmSoundBestIdx[ev], ev)
			total++
		}
	}
}
