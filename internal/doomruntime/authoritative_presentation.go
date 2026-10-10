package doomruntime

import (
	"math"
	"time"
)

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
	frames   []authorityRenderFrame
	from, to authorityRenderFrame
	stamp    time.Time
	tic      float64
	alpha    float64
}

const (
	authorityRenderMaxFrames = 8
	authorityRenderDelayTics = 3
	// Ordinary two-tic snapshots start three tics behind the server. A
	// large gap can recover closer to the newest frame, but never extrapolate
	// or accumulate an unbounded presentation delay. Collision stays current.
	authorityRenderMaxDelayTics = 7
)

func newAuthorityRenderTimeline(from, to authorityRenderFrame, now time.Time) *authorityRenderState {
	s := &authorityRenderState{frames: []authorityRenderFrame{from, to}, stamp: now,
		tic: math.Max(float64(from.tic), float64(to.tic-authorityRenderDelayTics))}
	if gap := to.tic - from.tic; gap < authorityRenderDelayTics {
		// Warm up only once. Subsequent packet arrivals never reset this clock.
		s.stamp = now.Add(time.Duration(authorityRenderDelayTics-gap) * time.Second / doomTicsPerSecond)
	}
	s.resetActorCuts(from, to)
	s.selectFrames()
	return s
}

func (s *authorityRenderState) prepare(now time.Time) {
	if len(s.frames) == 0 {
		return
	}
	if elapsed := now.Sub(s.stamp); elapsed > 0 {
		s.tic += elapsed.Seconds() * doomTicsPerSecond
		s.stamp = now
	}
	s.selectFrames()
}

func (s *authorityRenderState) selectFrames() {
	if len(s.frames) == 0 {
		return
	}
	s.tic = math.Max(float64(s.frames[0].tic), math.Min(s.tic, float64(s.frames[len(s.frames)-1].tic)))
	for len(s.frames) > 2 && float64(s.frames[1].tic) <= s.tic {
		s.frames[0] = authorityRenderFrame{}
		s.frames = s.frames[1:]
	}
	s.from, s.to = s.frames[0], s.frames[len(s.frames)-1]
	for i := 1; i < len(s.frames); i++ {
		if float64(s.frames[i].tic) >= s.tic {
			s.from, s.to = s.frames[i-1], s.frames[i]
			break
		}
	}
	s.alpha = 1
	if s.to.tic > s.from.tic {
		s.alpha = (s.tic - float64(s.from.tic)) / float64(s.to.tic-s.from.tic)
	}
}

func (s *authorityRenderState) appendFrame(frame authorityRenderFrame, now time.Time) {
	s.prepare(now)
	last := len(s.frames) - 1
	if last >= 0 && frame.tic < s.frames[last].tic {
		return
	}
	if last >= 0 {
		s.resetActorCuts(s.frames[last], frame)
	}
	if last >= 0 && frame.tic <= s.frames[last].tic {
		if frame.tic == s.frames[last].tic {
			s.frames[last] = frame
		}
	} else {
		s.frames = append(s.frames, frame)
	}
	if len(s.frames) > authorityRenderMaxFrames {
		s.frames[0] = authorityRenderFrame{}
		s.frames = s.frames[1:]
	}
	s.tic = math.Max(s.tic, float64(frame.tic-authorityRenderMaxDelayTics))
	s.selectFrames()
}

func (s *authorityRenderState) resetActorCuts(previous, next authorityRenderFrame) {
	for slot, current := range next.players {
		old, exists := previous.players[slot]
		if exists && old.generation == current.generation && old.movement == current.movement {
			continue
		}
		// Present a discontinuity immediately, then hold its landing pose until
		// the cursor reaches the new timeline. Leaving old identities queued
		// would snap to each new packet and later rewind into delayed movement.
		for i := range s.frames {
			if s.frames[i].tic < next.tic {
				if s.frames[i].players == nil {
					s.frames[i].players = make(map[int]authorityRenderPlayer)
				}
				s.frames[i].players[slot] = current
			}
		}
	}
	// New actors are visible immediately. Seed their first known pose back
	// through the buffered interval so they cannot first follow the newest
	// packet and then rewind when the delayed cursor reaches their birth.
	for order, pose := range next.projectiles {
		if _, exists := previous.projectiles[order]; exists {
			continue
		}
		for i := range s.frames {
			if s.frames[i].tic < next.tic {
				if s.frames[i].projectiles == nil {
					s.frames[i].projectiles = make(map[int64]authorityRenderPose)
				}
				s.frames[i].projectiles[order] = pose
			}
		}
	}
	for index, actor := range next.things {
		if index < len(previous.things) && previous.things[index].kind == actor.kind {
			continue
		}
		for i := range s.frames {
			if s.frames[i].tic < next.tic {
				if index >= len(s.frames[i].things) {
					s.frames[i].things = append(s.frames[i].things, make([]authorityRenderThing, index+1-len(s.frames[i].things))...)
				}
				s.frames[i].things[index] = actor
			}
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
	toFrame, toOK := g.authorityRender.to.players[slot]
	score := g.authorityRules.Scores[slot]
	if !ok || !toOK || from.generation != score.Generation || from.movement != score.MovementEpoch || toFrame.generation != score.Generation || toFrame.movement != score.MovementEpoch {
		return to
	}
	return interpolateAuthorityPose(from.pose, toFrame.pose, g.authorityRender.alpha)
}

func (g *game) authorityThingRenderPose(index int, kind int16, to authorityRenderPose) authorityRenderPose {
	if g.authorityRender == nil || index < 0 || index >= len(g.authorityRender.from.things) || index >= len(g.authorityRender.to.things) {
		return to
	}
	from := g.authorityRender.from.things[index]
	toFrame := g.authorityRender.to.things[index]
	if from.kind != kind || toFrame.kind != kind {
		return to
	}
	return interpolateAuthorityPose(from.pose, toFrame.pose, g.authorityRender.alpha)
}

func (g *game) authorityProjectileRenderPose(p projectile) authorityRenderPose {
	to := authorityRenderPose{x: p.x, y: p.y, z: p.z}
	if g.authorityRender == nil || p.order <= 0 {
		return to
	}
	from, ok := g.authorityRender.from.projectiles[p.order]
	toFrame, toOK := g.authorityRender.to.projectiles[p.order]
	if !ok || !toOK {
		return to
	}
	return interpolateAuthorityPose(from, toFrame, g.authorityRender.alpha)
}

func (g *game) captureAuthorityRenderFrame(now time.Time) authorityRenderFrame {
	if g.authorityRender != nil {
		g.authorityRender.prepare(now)
		frames := g.authorityRender.frames
		if len(frames) > 0 && frames[len(frames)-1].tic == g.worldTic {
			return frames[len(frames)-1]
		}
	}
	return g.snapshotAuthorityRenderFrame()
}

// Save raw authoritative endpoints, never an already interpolated pose. Reusing
// the displayed pose at packet arrival makes velocity depend on packet jitter.
func (g *game) snapshotAuthorityRenderFrame() authorityRenderFrame {
	frame := authorityRenderFrame{tic: g.worldTic, players: make(map[int]authorityRenderPlayer), things: make([]authorityRenderThing, len(g.m.Things)), projectiles: make(map[int64]authorityRenderPose, len(g.projectiles))}
	for _, p := range g.authorityPlayers {
		if g.authorityRules != nil {
			score := g.authorityRules.Scores[p.localSlot]
			frame.players[p.localSlot] = authorityRenderPlayer{playerRenderPose(p.p), score.Generation, score.MovementEpoch}
		}
	}
	for slot, p := range g.remotePlayers {
		if g.authorityRules == nil {
			continue
		}
		score := g.authorityRules.Scores[slot]
		frame.players[slot] = authorityRenderPlayer{playerRenderPose(p.p), score.Generation, score.MovementEpoch}
	}
	for i, thing := range g.m.Things {
		x, y := g.thingPosFixed(i, thing)
		z, _, _ := g.thingSupportState(i, thing)
		pose := authorityRenderPose{x: x, y: y, z: z}
		frame.things[i] = authorityRenderThing{pose, thing.Type}
	}
	for _, p := range g.projectiles {
		frame.projectiles[p.order] = authorityRenderPose{x: p.x, y: p.y, z: p.z}
	}
	return frame
}

func (g *game) beginAuthorityRenderBlend(from authorityRenderFrame, now time.Time) {
	to := g.snapshotAuthorityRenderFrame()
	if g.authorityRender == nil {
		g.authorityRender = newAuthorityRenderTimeline(from, to, now)
		return
	}
	g.authorityRender.appendFrame(to, now)
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
