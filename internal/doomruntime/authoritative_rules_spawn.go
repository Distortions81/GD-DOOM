package doomruntime

import (
	"fmt"

	"gddoom/internal/doomrand"
)

// authoritativeSpawnStarts defines explicit spawn policy: co-op prefers the
// player's numbered start, then any free co-op start; deathmatch uses only type
// 11 starts. A deathmatch map without starts is rejected instead of silently
// placing opponents at a co-op start.
func (a *Authority) authoritativeSpawnStarts(id byte) []playerStart {
	if a.g.opts.GameMode == gameModeDeathmatch {
		starts := make([]playerStart, 0, 4)
		for i, th := range a.g.m.Things {
			if isDeathmatchStart(th.Type) {
				starts = append(starts, playerStart{index: i, x: int64(th.X) << fracBits, y: int64(th.Y) << fracBits, angle: thingDegToWorldAngle(th.Angle)})
			}
		}
		return starts
	}
	starts := collectPlayerStarts(a.g.m)
	for i, p := range starts {
		if p.slot == int(id) {
			starts[0], starts[i] = starts[i], starts[0]
			break
		}
	}
	return starts
}

func (a *Authority) authoritativeSpawnBody(id byte, start playerStart) (player, bool) {
	g := a.g
	saved := g.captureAuthoritativePlayer()
	defer g.applyAuthoritativePlayer(saved)
	g.applyAuthoritativePlayer(authoritativePlayerState{
		localSlot: int(id), stats: playerStats{Health: 100},
		p: player{x: start.x, y: start.y, angle: start.angle, subsector: -1, sector: -1, viewHeight: playerViewHeight},
	})
	floor, ceiling, _, fits := g.checkPositionForWithPickupTouch(start.x, start.y, false, false)
	if !fits || ceiling-floor < playerHeight {
		return player{}, false
	}
	g.p.floorz, g.p.ceilz, g.p.z = floor, ceiling, floor
	g.refreshPlayerSubsectorCache(g.p.x, g.p.y)
	return g.p, true
}

func (a *Authority) chooseAuthoritativeSpawn(id byte) (player, playerStart, int, error) {
	starts := a.authoritativeSpawnStarts(id)
	if len(starts) == 0 {
		return player{}, playerStart{}, 0, fmt.Errorf("map has no %s player starts", a.g.opts.GameMode)
	}
	bestDistance := int64(-1)
	bestIndex := -1
	var bestBody player
	for offset := range starts {
		i := offset
		if a.g.opts.GameMode == gameModeDeathmatch {
			i = (a.g.authorityRules.SpawnCursor + offset) % len(starts)
		}
		p, fits := a.authoritativeSpawnBody(id, starts[i])
		if !fits {
			continue
		}
		if a.g.opts.GameMode == gameModeCoop {
			return p, starts[i], i, nil
		}
		// Select the free deathmatch spawn farthest from the nearest living
		// opponent. Tie order rotates, including when joining an empty map.
		nearest := int64(1 << 62)
		for otherID, other := range a.players {
			if other == nil || other.isDead || otherID == int(id) {
				continue
			}
			dx, dy := (p.x-other.p.x)>>fracBits, (p.y-other.p.y)>>fracBits
			distance := dx*dx + dy*dy
			if distance < nearest {
				nearest = distance
			}
		}
		if nearest > bestDistance {
			bestDistance, bestIndex, bestBody = nearest, i, p
		}
	}
	if bestIndex >= 0 {
		return bestBody, starts[bestIndex], bestIndex, nil
	}
	return player{}, playerStart{}, 0, ErrAuthoritySpawnBlocked
}

func (a *Authority) spawnAuthoritativePlayer(id byte, respawn bool) error {
	g := a.g
	rules := g.authorityRules
	if rules.Ended {
		return ErrAuthorityMatchEnded
	}
	if rules.Scores[id].Generation == ^uint32(0) {
		return fmt.Errorf("player incarnation exhausted for slot %d", id)
	}
	p, start, spawnIndex, err := a.chooseAuthoritativeSpawn(id)
	if err != nil {
		return err
	}
	saved := g.captureAuthoritativePlayer()
	oldPlayer := a.players[id]
	index := start.index
	thinkerOrder := int64(0)
	if respawn || start.slot != int(id) {
		index = -1
		thinkerOrder = g.allocThinkerOrder()
	}
	g.applyAuthoritativePlayer(authoritativePlayerState{
		p: p, localSlot: int(id), localPlayerThingIndex: index,
		playerBlockOrder: g.allocBlockmapOrder(), authorityPlayerThinkerOrder: thinkerOrder,
		autoWeaponSwitch: g.opts.AutoWeaponSwitch,
	})
	g.initPlayerState()
	if g.opts.GameMode == gameModeDeathmatch {
		g.inventory.BlueKey, g.inventory.RedKey, g.inventory.YellowKey = true, true, true
	} else {
		keys := rules.TeamKeys
		g.inventory.BlueKey, g.inventory.RedKey, g.inventory.YellowKey = keys.Blue, keys.Red, keys.Yellow
		if oldPlayer != nil {
			g.inventory.BlueKey = g.inventory.BlueKey || oldPlayer.inventory.BlueKey
			g.inventory.RedKey = g.inventory.RedKey || oldPlayer.inventory.RedKey
			g.inventory.YellowKey = g.inventory.YellowKey || oldPlayer.inventory.YellowKey
		}
	}
	if respawn && oldPlayer != nil {
		g.playerKillCount, g.playerItemCount, g.secretsFound = oldPlayer.playerKillCount, oldPlayer.playerItemCount, oldPlayer.secretsFound
	}
	g.clearWeaponOverlay()
	g.bringUpWeapon()
	g.initStatusFaceState()
	g.playerViewZ = g.p.z + g.p.viewHeight
	g.prevPX, g.prevPY, g.prevAngle, g.prevPrevAngle = g.p.x, g.p.y, g.p.angle, g.p.angle
	// Player mobjs consume their spawn RNG once, independent of which
	// candidate positions were blocked during the search above.
	_ = doomrand.PRandom() & 3
	state := g.captureAuthoritativePlayer()
	g.applyAuthoritativePlayer(saved)
	g.clearAuthoritativePlayerTargets(int(id))
	if oldPlayer != nil {
		// Existing per-tic roster slices retain this pointer during respawn.
		*oldPlayer = state
	} else {
		a.players[id] = &state
	}
	score := &rules.Scores[id]
	if !respawn {
		score.Frags, score.Deaths = 0, 0
	}
	score.Generation++
	score.DeathRecorded, score.RespawnPending = false, false
	score.DeathTic = 0
	rules.SpawnCursor = (spawnIndex + 1) % len(a.authoritativeSpawnStarts(id))
	a.selectAnchor()
	if respawn {
		g.spawnTeleportFog(p.x, p.y, p.z)
	}
	return nil
}
