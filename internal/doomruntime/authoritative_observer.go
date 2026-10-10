package doomruntime

import (
	"fmt"
	"time"

	"gddoom/internal/netgame"
)

// Observers consume an existing player's view, but never predict movement or
// send gameplay commands. Their connection identity remains player zero.
func (g *game) updateAuthoritativeObserverAt(now time.Time) error {
	client := g.opts.AuthorityClient
	welcome := client.Welcome()
	if welcome.Epoch == 0 || welcome.PlayerID != 0 {
		return netgame.ErrProtocol
	}
	if g.clientPrediction == nil {
		g.clientPrediction = &ClientPrediction{g: g, epoch: welcome.Epoch}
		g.setHUDMessage("WAITING FOR PLAYERS", 70)
	}
	p := g.clientPrediction
	acknowledge := false
	for range 8 {
		snapshot, ok, err := client.PollSnapshot()
		if err != nil {
			g.setAuthorityConnectionFailure(err)
			return nil
		}
		if !ok {
			break
		}
		if snapshot.Epoch != p.epoch || snapshot.ID == 0 || snapshot.BaselineID != 0 || snapshot.Finalized.HasSequence || snapshot.Finalized.Sequence != 0 ||
			(!snapshot.Finalized.HasTick && snapshot.Finalized.Tick != 0) || (snapshot.Finalized.HasTick && snapshot.Finalized.Tick != snapshot.Tick) {
			return netgame.ErrProtocol
		}
		if p.ready && (snapshot.ID <= p.snapshotID || snapshot.Tick < p.authoritativeTic) {
			continue
		}
		r, err := decodeAuthorityReplica(snapshot.State)
		if err != nil {
			return err
		}
		if r.Tic != snapshot.Tick || r.Viewer < 1 || r.Viewer > 4 || (p.ready && r.SoundCursor < p.soundCursor) {
			return netgame.ErrProtocol
		}
		if err := validateAuthorityReplica(g, r); err != nil {
			return err
		}
		wasReady, sameView := p.ready, p.ready && p.viewer == r.Viewer
		var renderFrom authorityRenderFrame
		if sameView {
			renderFrom = g.captureAuthorityRenderFrame(now)
		}
		g.applyValidatedAuthorityReplica(r)
		if sameView {
			g.beginAuthorityRenderBlend(renderFrom, now)
		} else {
			// Changing followed players is a camera cut, not interpolation from
			// the previous viewer's snapshot timeline.
			g.authorityRender = nil
		}
		p.ready, p.viewer, p.snapshotID = true, r.Viewer, snapshot.ID
		p.authoritativeTic, p.predictedTic = snapshot.Tick, snapshot.Tick
		p.applyAuthoritySounds(r, wasReady)
		g.syncRenderState()
		g.markSimUpdate(now)
		g.clientUpdate.snapshotAt = now
		if !sameView {
			g.setHUDMessage(fmt.Sprintf("SPECTATING PLAYER %d - F12 TO SWITCH", r.Viewer), 105)
		}
		acknowledge = true
	}
	if g.authorityConnectionBlocked() {
		return nil
	}
	if acknowledge {
		if err := client.SendInputs(netgame.InputBatch{Epoch: p.epoch, SnapshotAck: p.snapshotID}); err != nil {
			g.setAuthorityConnectionFailure(err)
		}
	}
	g.publishRuntimeSettingsIfChanged()
	if g.snd != nil {
		g.snd.tick()
	}
	return nil
}

// Spectators share the remote snapshot timeline for horizontal camera motion
// and yaw. Their confirmed body, eye height and collision remain authoritative.
func (g *game) prepareAuthorityObserverCamera() {
	if g.opts.AuthorityClient == nil || g.opts.AuthorityClient.Welcome().PlayerID != 0 || g.clientPrediction == nil || !g.clientPrediction.ready {
		return
	}
	pose := g.authorityRemoteRenderPose(int(g.clientPrediction.viewer), g.p)
	g.renderPX, g.renderPY = float64(pose.x)/fracUnit, float64(pose.y)/fracUnit
	g.renderAngle = pose.angle
	g.State.RenderCamX, g.State.RenderCamY = g.renderPX, g.renderPY
}
