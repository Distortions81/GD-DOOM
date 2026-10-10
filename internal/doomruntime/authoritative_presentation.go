package doomruntime

import "time"

// Snapshot interpolation is presentation state. Collision continues to use the
// most recent authoritative positions; a stalled stream never extrapolates.
type authorityRenderPose struct {
	x, y, z int64
	angle   uint32
}
type authorityRenderPlayer struct {
	pose                 authorityRenderPose
	generation, movement uint32
}
type authorityRenderThing struct {
	pose authorityRenderPose
	kind int16
}
type authorityRenderFrame struct {
	tic         int
	players     map[int]authorityRenderPlayer
	things      []authorityRenderThing
	projectiles map[int64]authorityRenderPose
}
type authorityRenderState struct {
	from     authorityRenderFrame
	received time.Time
	duration time.Duration
	alpha    float64
}

func (s *authorityRenderState) prepare(now time.Time) {
	s.alpha = 1
	if s.duration > 0 {
		s.alpha = float64(now.Sub(s.received)) / float64(s.duration)
		if s.alpha < 0 {
			s.alpha = 0
		}
		if s.alpha > 1 {
			s.alpha = 1
		}
	}
}

func playerRenderPose(p player) authorityRenderPose {
	return authorityRenderPose{p.x, p.y, p.z, p.angle}
}
func interpolateAuthorityPose(from, to authorityRenderPose, alpha float64) authorityRenderPose {
	// Large spatial jumps are teleports or newly spawned world actors. Players
	// additionally use explicit incarnation/teleport identities below.
	const maximumBlendDistance = 128 * fracUnit
	if abs(from.x-to.x) > maximumBlendDistance || abs(from.y-to.y) > maximumBlendDistance || abs(from.z-to.z) > maximumBlendDistance {
		return to
	}
	return authorityRenderPose{lerpFixed(from.x, to.x, alpha), lerpFixed(from.y, to.y, alpha), lerpFixed(from.z, to.z, alpha), lerpAngle(from.angle, to.angle, alpha)}
}

func (g *game) authorityRemoteRenderPose(slot int, p player) authorityRenderPose {
	to := playerRenderPose(p)
	if g.authorityRender == nil || g.authorityRules == nil {
		return to
	}
	from, ok := g.authorityRender.from.players[slot]
	score := g.authorityRules.Scores[slot]
	if !ok || from.generation != score.Generation || from.movement != score.MovementEpoch {
		return to
	}
	return interpolateAuthorityPose(from.pose, to, g.authorityRender.alpha)
}

func (g *game) authorityThingRenderPose(index int, kind int16, to authorityRenderPose) authorityRenderPose {
	if g.authorityRender == nil || index < 0 || index >= len(g.authorityRender.from.things) {
		return to
	}
	from := g.authorityRender.from.things[index]
	if from.kind != kind {
		return to
	}
	return interpolateAuthorityPose(from.pose, to, g.authorityRender.alpha)
}

func (g *game) authorityProjectileRenderPose(p projectile) authorityRenderPose {
	to := authorityRenderPose{x: p.x, y: p.y, z: p.z}
	if g.authorityRender == nil || p.order <= 0 {
		return to
	}
	from, ok := g.authorityRender.from.projectiles[p.order]
	if !ok {
		return to
	}
	return interpolateAuthorityPose(from, to, g.authorityRender.alpha)
}

func (g *game) captureAuthorityRenderFrame(now time.Time) authorityRenderFrame {
	if g.authorityRender != nil {
		g.authorityRender.prepare(now)
	}
	frame := authorityRenderFrame{tic: g.worldTic, players: make(map[int]authorityRenderPlayer), things: make([]authorityRenderThing, len(g.m.Things)), projectiles: make(map[int64]authorityRenderPose, len(g.projectiles))}
	for slot, p := range g.remotePlayers {
		if g.authorityRules == nil {
			continue
		}
		score := g.authorityRules.Scores[slot]
		frame.players[slot] = authorityRenderPlayer{g.authorityRemoteRenderPose(slot, p.p), score.Generation, score.MovementEpoch}
	}
	for i, thing := range g.m.Things {
		x, y := g.thingPosFixed(i, thing)
		z, _, _ := g.thingSupportState(i, thing)
		pose := g.authorityThingRenderPose(i, thing.Type, authorityRenderPose{x: x, y: y, z: z})
		frame.things[i] = authorityRenderThing{pose, thing.Type}
	}
	for _, p := range g.projectiles {
		frame.projectiles[p.order] = g.authorityProjectileRenderPose(p)
	}
	return frame
}

func (g *game) beginAuthorityRenderBlend(from authorityRenderFrame, now time.Time) {
	duration := time.Duration(g.worldTic-from.tic) * time.Second / doomTicsPerSecond
	if duration < time.Second/doomTicsPerSecond {
		duration = time.Second / doomTicsPerSecond
	}
	if duration > 200*time.Millisecond {
		duration = 200 * time.Millisecond
	}
	g.authorityRender = &authorityRenderState{from: from, received: now, duration: duration}
}

// Sprite pose follows server state. It never runs a player thinker or advances
// weapon state. Corpses remain corpses even if an intermediate snapshot is lost.
func (g *game) authoritativePlayerFrame(slot int) (byte, bool) {
	p := g.authoritativePlayerForSlot(slot)
	if p == nil {
		return 0, false
	}
	if p.isDead {
		return 'N', true
	}
	switch p.playerMobjState {
	case doomStatePlayerAttack1:
		return 'E', true
	case doomStatePlayerAttack2:
		return 'F', true
	case doomStatePlayerPain1, doomStatePlayerPain2:
		return 'G', true
	}
	if p.p.momx == 0 && p.p.momy == 0 {
		return 'A', true
	}
	return byte('A' + (g.worldTic/playerWalkFrameTics)%4), true
}
