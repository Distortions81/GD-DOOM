package doomruntime

import (
	"errors"
	"fmt"
	"math"
	"slices"
	"time"

	"gddoom/internal/demo"
	"gddoom/internal/netgame"
	"gddoom/internal/render/mapview"
)

var (
	ErrPredictionNotReady = errors.New("prediction requires an authoritative baseline")
	ErrPredictionWindow   = errors.New("prediction history is full; wait for an authoritative baseline")
)

// ClientPrediction owns local movement history on the client's game goroutine.
// Networking delivers decoded envelopes; it never invokes this concurrently
// with rendering or input. Prediction keeps the authoritative world frozen and
// changes only the local body/view. Weapons, pickups, damage, triggers, and AI
// remain server-owned and are never replayed.
type ClientPrediction struct {
	g                *game
	epoch            uint64
	viewer           byte
	ready            bool
	snapshotID       uint32
	authoritativeTic uint32
	predictedTic     uint32
	generation       uint32
	movementEpoch    uint32
	soundCursor      uint64
	history          []netgame.Input
	lastInput        netgame.Input
	hasInput         bool
	renderCorrection predictionRenderCorrection
}

const predictionCorrectionDuration = 100 * time.Millisecond

// Reconciliation changes collision immediately. This short, bounded offset
// lets the camera absorb small corrections without also delaying new input.
type predictionRenderCorrection struct {
	x, y, angle float64
	started     time.Time
}

type predictionRenderHistory struct {
	body                     player
	view                     mapview.ViewState
	prevX, prevY             int64
	prevAngle, prevPrevAngle uint32
	renderX, renderY         float64
	renderAngle              uint32
	alpha                    float64
	updated                  time.Time
}

func (p *ClientPrediction) captureRenderHistory() predictionRenderHistory {
	g := p.g
	return predictionRenderHistory{g.p, g.State, g.prevPX, g.prevPY,
		g.prevAngle, g.prevPrevAngle, g.renderPX, g.renderPY, g.renderAngle,
		g.renderAlpha, g.lastUpdate}
}

func (p *ClientPrediction) correctionAt(now time.Time) (x, y, angle float64) {
	c := p.renderCorrection
	if c.started.IsZero() {
		return 0, 0, 0
	}
	remaining := 1 - float64(now.Sub(c.started))/float64(predictionCorrectionDuration)
	remaining = math.Max(0, math.Min(1, remaining))
	return c.x * remaining, c.y * remaining, c.angle * remaining
}

func (p *ClientPrediction) prepareRenderCorrection(now time.Time) {
	x, y, angle := p.correctionAt(now)
	g := p.g
	g.renderPX += x
	g.renderPY += y
	g.renderAngle += uint32(int64(math.Round(angle)))
	g.State.RenderCamX += x
	g.State.RenderCamY += y
}

func (p *ClientPrediction) restoreRenderHistory(from predictionRenderHistory, now time.Time) {
	g := p.g
	dx, dy := g.p.x-from.body.x, g.p.y-from.body.y
	da := g.p.angle - from.body.angle
	// Preserve the current interpolation phase and velocity, translating its
	// endpoints onto the corrected timeline instead of restarting it on receipt.
	g.prevPX, g.prevPY = from.prevX+dx, from.prevY+dy
	g.prevAngle, g.prevPrevAngle = from.prevAngle+da, from.prevPrevAngle+da
	g.lastUpdate, g.renderAlpha = from.updated, from.alpha
	x, y := float64(dx)/fracUnit, float64(dy)/fracUnit
	g.State = from.view
	g.State.CamX += x
	g.State.CamY += y
	g.State.PrevCamX += x
	g.State.PrevCamY += y
	g.State.RenderCamX += x
	g.State.RenderCamY += y
	g.renderPX, g.renderPY = from.renderX+x, from.renderY+y
	g.renderAngle = from.renderAngle + da
	if dx == 0 && dy == 0 && da == 0 {
		// An accurate snapshot must not prolong an earlier correction.
		return
	}
	cx, cy, ca := p.correctionAt(now)
	cx, cy, ca = cx-x, cy-y, ca-float64(int32(da))
	if math.Abs(cx) > 32 || math.Abs(cy) > 32 || math.Abs(ca) > float64(doomAng90) {
		p.renderCorrection = predictionRenderCorrection{}
		g.syncRenderState()
		g.markSimUpdate(now)
		return
	}
	p.renderCorrection = predictionRenderCorrection{cx, cy, ca, now}
}

// newClientPrediction binds an existing renderable game to a server welcome.
// A new map/session epoch requires a new predictor and a matching loaded map.
func newClientPrediction(g *game, welcome netgame.Welcome) (*ClientPrediction, error) {
	if g == nil || g.m == nil || welcome.Epoch == 0 || welcome.PlayerID < 1 || welcome.PlayerID > 4 || welcome.InputLead > netgame.TickRate {
		return nil, fmt.Errorf("invalid prediction world or welcome")
	}
	return &ClientPrediction{g: g, epoch: welcome.Epoch, viewer: welcome.PlayerID, authoritativeTic: welcome.ServerTick, predictedTic: welcome.ServerTick}, nil
}

func (p *ClientPrediction) Ready() bool              { return p.ready }
func (p *ClientPrediction) PredictedTic() uint32     { return p.predictedTic }
func (p *ClientPrediction) AuthoritativeTic() uint32 { return p.authoritativeTic }
func (p *ClientPrediction) SnapshotID() uint32       { return p.snapshotID }

// PendingInputs returns an owned list suitable for building redundant batches.
// A transport may select its newest MaxInputBatch commands from this list.
func (p *ClientPrediction) PendingInputs() []netgame.Input { return slices.Clone(p.history) }

// NextInputTic does not reuse tics sent before a teleport/respawn correction.
// Those future slots can already be queued at the server even after local
// history is discarded. The bool is false when an epoch counter is exhausted.
func (p *ClientPrediction) NextInputTic() (uint32, bool) {
	tic := p.predictedTic
	if p.hasInput && p.lastInput.Tick > tic {
		tic = p.lastInput.Tick
	}
	if tic == math.MaxUint32 {
		return 0, false
	}
	return tic + 1, true
}

// Predict records one numbered command and shows its movement immediately.
// Gaps in scheduled tics are simulated with neutral input; no command is given
// more than one movement step. History and replay work are both capped at 256.
func (p *ClientPrediction) Predict(input netgame.Input) error {
	if !p.ready {
		return ErrPredictionNotReady
	}
	if err := netgame.ValidateInputCommand(input.Command); err != nil {
		return err
	}
	if input.Tick <= p.predictedTic || (p.hasInput && (input.Tick <= p.lastInput.Tick || input.Sequence <= p.lastInput.Sequence)) {
		return netgame.ErrInputSequence
	}
	if len(p.history) >= netgame.MaxInputWindow || uint64(input.Tick)-uint64(p.authoritativeTic) > netgame.MaxInputWindow {
		return ErrPredictionWindow
	}
	p.history = append(p.history, input)
	p.lastInput, p.hasInput = input, true
	p.replayTo(input)
	p.g.State.SetCamera(float64(p.g.p.x)/fracUnit, float64(p.g.p.y)/fracUnit)
	return nil
}

func (p *ClientPrediction) replayTo(input netgame.Input) {
	for p.predictedTic < input.Tick {
		command := demo.Tic{}
		if p.predictedTic+1 == input.Tick {
			command = input.Command
		}
		p.g.predictAuthoritativeMovement(command)
		p.predictedTic++
	}
}

// Reconcile accepts a newer full baseline, expires every finalized command
// (including inputs lost before their deadline), and replays only future intent.
// Stale snapshots are ignored without resetting a newer predicted body. Envelope
// and body validation finish before either the game or prediction history changes.
func (p *ClientPrediction) Reconcile(snapshot netgame.Snapshot) (bool, error) {
	return p.reconcileAt(snapshot, time.Now())
}

func (p *ClientPrediction) reconcileAt(snapshot netgame.Snapshot, now time.Time) (bool, error) {
	if snapshot.Epoch != p.epoch || snapshot.ID == 0 || snapshot.BaselineID != 0 {
		return false, netgame.ErrProtocol
	}
	if p.ready && (snapshot.ID <= p.snapshotID || snapshot.Tick < p.authoritativeTic) {
		return false, nil
	}
	if snapshot.Finalized.HasSequence && !snapshot.Finalized.HasTick ||
		!snapshot.Finalized.HasSequence && snapshot.Finalized.Sequence != 0 ||
		!snapshot.Finalized.HasTick && snapshot.Finalized.Tick != 0 ||
		snapshot.Finalized.HasTick && snapshot.Finalized.Tick != snapshot.Tick ||
		snapshot.Finalized.HasSequence && (!p.hasInput || snapshot.Finalized.Sequence > p.lastInput.Sequence) {
		return false, netgame.ErrProtocol
	}
	r, err := decodeAuthorityReplica(snapshot.State)
	if err != nil {
		return false, err
	}
	if r.Tic != snapshot.Tick || r.Viewer != p.viewer {
		return false, netgame.ErrProtocol
	}
	if err := validateAuthorityReplica(p.g, r); err != nil {
		return false, err
	}
	if p.ready && r.SoundCursor < p.soundCursor {
		return false, netgame.ErrProtocol
	}
	score := r.Rules.Scores[p.viewer]
	discontinuity := p.ready && (score.Generation != p.generation || score.MovementEpoch != p.movementEpoch)
	history := make([]netgame.Input, 0, len(p.history))
	if !discontinuity {
		for _, input := range p.history {
			// Full world state also finalizes time before a joining player's first
			// per-player acknowledgment, so never replay at/before the baseline.
			if input.Tick > snapshot.Tick {
				history = append(history, input)
			}
		}
	}
	var renderFrom authorityRenderFrame
	wasReady := p.ready
	previousTic, wasDead := p.predictedTic, p.g.isDead
	localRenderFrom := p.captureRenderHistory()
	if wasReady {
		renderFrom = p.g.captureAuthorityRenderFrame(now)
	}
	p.g.applyValidatedAuthorityReplica(r)
	if wasReady {
		p.g.beginAuthorityRenderBlend(renderFrom, now)
	}
	p.ready, p.snapshotID = true, snapshot.ID
	p.authoritativeTic, p.predictedTic = snapshot.Tick, snapshot.Tick
	p.generation, p.movementEpoch = score.Generation, score.MovementEpoch
	p.history = history
	for _, input := range history {
		p.replayTo(input)
	}
	p.g.State.SetCamera(float64(p.g.p.x)/fracUnit, float64(p.g.p.y)/fracUnit)
	p.g.syncRenderState()
	if wasReady && !discontinuity && wasDead == p.g.isDead && previousTic == p.predictedTic &&
		abs(p.g.p.x-localRenderFrom.body.x) <= 32*fracUnit && abs(p.g.p.y-localRenderFrom.body.y) <= 32*fracUnit &&
		abs(p.g.p.z-localRenderFrom.body.z) <= 32*fracUnit && abs(int64(int32(p.g.p.angle-localRenderFrom.body.angle))) <= int64(doomAng90) {
		p.restoreRenderHistory(localRenderFrom, now)
	} else {
		p.renderCorrection = predictionRenderCorrection{}
		p.g.markSimUpdate(now)
	}
	p.applyAuthoritySounds(r, wasReady)
	return true, nil
}

// predictAuthoritativeMovement calls the same acceleration, friction, collision,
// gravity and body code as server player think, with world-changing callbacks
// disabled. Its clock is external: g.worldTic remains the last server snapshot.
func (g *game) predictAuthoritativeMovement(tc demo.Tic) {
	previous := g.predictionMovement
	g.predictionMovement = true
	defer func() { g.predictionMovement = previous }()
	cmd, _, _ := demoTicCommand(tc)
	cmd.weaponSlot = 0
	if g.p.justAttacked {
		cmd.forward, cmd.side, cmd.turn, cmd.turnRaw = 0xc800/512, 0, 0, 0
		g.p.justAttacked = false
	}
	g.currentMoveCmd = cmd
	g.updatePlayer(cmd)
	g.tickPlayerViewHeight()
	if g.p.reactionTime > 0 {
		g.p.reactionTime--
	}
	g.tickPlayerBody()
}
