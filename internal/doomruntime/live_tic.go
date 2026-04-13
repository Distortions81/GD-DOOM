package doomruntime

import (
	"fmt"
	"strings"
	"time"

	"gddoom/internal/demo"
	"gddoom/internal/runtimecfg"

	"github.com/hajimehoshi/ebiten/v2"
)

const (
	coopMaxCatchUpTics    = 4
	coopStartupBufferTics = 2
)

const (
	watchTargetBufferedTics = 2
	watchMaxCatchUpTics     = 4
)

func demoTicCommand(tc DemoTic) (moveCmd, bool, bool) {
	buttons := tc.Buttons
	if buttons&demoButtonSpecial != 0 {
		buttons = 0
	}
	cmd := moveCmd{
		forward:    int64(tc.Forward),
		side:       int64(tc.Side),
		turnRaw:    int64(tc.AngleTurn) << 16,
		weaponSlot: demoButtonWeaponSlot(buttons),
	}
	usePressed := buttons&demoButtonUse != 0
	fireHeld := buttons&demoButtonAttack != 0
	return cmd, usePressed, fireHeld
}

func (g *game) buildOutgoingDemoTic(cmd moveCmd, usePressed, fireHeld bool) demo.Tic {
	buttons := byte(0)
	if usePressed {
		buttons |= demoButtonUse
	}
	if fireHeld {
		buttons |= demoButtonAttack
	}
	if slot := g.demoWeaponSlot; slot != 0 {
		buttons |= demoButtonChange | byte((slot-1)<<demoButtonWeaponShift)
		g.demoWeaponSlot = 0
	}
	return demo.Tic{
		Forward:   clampDemoMove(cmd.forward),
		Side:      clampDemoMove(cmd.side),
		AngleTurn: g.demoAngleTurn(cmd),
		Buttons:   buttons,
	}
}

func (g *game) recordGameplayTic(cmd moveCmd, usePressed, fireHeld bool) {
	if g == nil {
		return
	}
	if g.opts.DemoScript != nil && g.opts.LiveTicSink == nil {
		return
	}
	tc := g.buildOutgoingDemoTic(cmd, usePressed, fireHeld)
	if g.opts.LiveTicSink != nil {
		_ = g.opts.LiveTicSink.BroadcastTic(tc)
	}
	if strings.TrimSpace(g.opts.RecordDemoPath) == "" {
		return
	}
	g.demoRecord = append(g.demoRecord, tc)
}

func (g *game) stepGameplayFromDemoTic(tc DemoTic) {
	cmd, usePressed, fireHeld := demoTicCommand(tc)
	g.runGameplayTic(cmd, usePressed, fireHeld)
	g.discoverLinesAroundPlayer()
	g.State.SetCamera(float64(g.p.x)/fracUnit, float64(g.p.y)/fracUnit)
	g.tickDelayedSounds()
	g.flushSoundEvents()
	g.tickStatusWidgets()
	if g.useFlash > 0 {
		g.useFlash--
	}
	g.tickChatHistory()
	if g.damageFlashTic > 0 {
		g.damageFlashTic--
	}
	if g.bonusFlashTic > 0 {
		g.bonusFlashTic--
	}
	g.tickDelayedSwitchReverts()
	g.markSimUpdate(time.Now())
}

func (g *game) updateWatchMode() error {
	if g == nil {
		return nil
	}
	now := time.Now()
	if err := g.pollChatMessages(); err != nil {
		return fmt.Errorf("watch chat stream: %w", err)
	}
	if g.handleChatInput() {
		// Chat compose owns Enter/Escape/T while it is active.
	} else {
		if g.keyJustPressed(ebiten.KeyF4) {
			g.soundMenuRequested = true
			return nil
		}
		if g.keyJustPressed(ebiten.KeyF10) {
			g.quitPromptRequested = true
			return nil
		}
		if g.keyJustPressed(ebiten.KeyEscape) {
			g.frontendMenuRequested = true
			ebiten.SetCursorMode(ebiten.CursorModeVisible)
			return nil
		}
		if g.bindingJustPressed(bindingAutomap) {
			if g.mode == viewWalk {
				g.mode = viewMap
				g.setHUDMessage("Automap Opened", 35)
			} else {
				g.mode = viewWalk
				g.mouseLookSet = false
				g.mouseLookSuppressTicks = detailMouseSuppressTicks
				g.setHUDMessage("Automap Closed", 35)
			}
		}
		g.edgeInputPass = true
		g.updateParityControls()
		if g.keyJustPressed(ebiten.KeyF5) {
			if g.opts.SourcePortMode {
				g.cycleSourcePortDetailLevel()
			} else {
				g.cycleDetailLevel()
			}
		}
		g.edgeInputPass = false
	}
	ticDur := time.Second / doomTicsPerSecond
	if g.watchTickStamp.IsZero() {
		g.watchTickStamp = now
	}
	g.watchTickAccum += now.Sub(g.watchTickStamp)
	g.watchTickStamp = now

	budget := 0
	if g.worldTic == 0 {
		startupBuffer := max(0, g.opts.WatchStartupBufferTics)
		if startupBuffer == 0 {
			budget = 1
		} else if src, ok := g.opts.LiveTicSource.(runtimecfg.LiveTicBufferedSource); ok && src != nil {
			if src.PendingTics() >= startupBuffer {
				budget = 1
			}
		}
	}
	for g.watchTickAccum >= ticDur {
		g.watchTickAccum -= ticDur
		budget++
	}
	if g.worldTic > 0 {
		if src, ok := g.opts.LiveTicSource.(runtimecfg.LiveTicBufferedSource); ok && src != nil {
			if pending := src.PendingTics(); pending > watchTargetBufferedTics {
				catchUp := pending - watchTargetBufferedTics
				if catchUp > watchMaxCatchUpTics {
					catchUp = watchMaxCatchUpTics
				}
				if catchUp > budget {
					budget = catchUp
				}
			}
		}
	}
	if budget > 8 {
		budget = 8
	}
	for i := 0; i < budget; i++ {
		tc, ok, err := g.opts.LiveTicSource.PollTic()
		if err != nil {
			return fmt.Errorf("watch stream: %w", err)
		}
		if !ok {
			break
		}
		g.capturePrevState()
		g.stepGameplayFromDemoTic(tc)
	}
	g.publishRuntimeSettingsIfChanged()
	return nil
}

func (sg *sessionGame) applyMandatoryWatchKeyframes() error {
	if sg == nil || sg.opts.LiveTicSource == nil {
		return nil
	}
	src, ok := sg.opts.LiveTicSource.(runtimecfg.LiveRuntimeKeyframeSource)
	if !ok || src == nil {
		return nil
	}
	applied := false
	for i := 0; i < 8; i++ {
		kf, ok, err := src.PollRuntimeKeyframe()
		if err != nil {
			return fmt.Errorf("watch keyframe: %w", err)
		}
		if !ok {
			break
		}
		if !kf.MandatoryApply {
			continue
		}
		if err := sg.unmarshalNetplayKeyframe(kf.Blob); err != nil {
			return fmt.Errorf("apply watch keyframe: %w", err)
		}
		applied = true
	}
	if applied && sg.g != nil {
		sg.g.clearPendingSoundState()
	}
	return nil
}

// updateCoopMode runs one game engine Update tick for peer-symmetric co-op.
// It is called instead of the normal walk/map update when g.opts.CoopPeers is set.
func (g *game) updateCoopMode() error {
	if g == nil {
		return nil
	}
	src := g.opts.CoopPeers
	if src == nil {
		return nil
	}

	// Apply roster changes: spawn or remove remote player structs.
	if roster, ok := src.PollRosterUpdate(); ok {
		g.applyCoopRoster(roster.PlayerIDs)
	}

	// Apply any mandatory keyframes received from the server (desync recovery
	// or late-join catch-up) before advancing the sim.
	if err := g.applyCoopKeyframes(); err != nil {
		return fmt.Errorf("coop keyframe: %w", err)
	}

	if err := g.pollChatMessages(); err != nil {
		return fmt.Errorf("coop chat stream: %w", err)
	}
	if g.handleChatInput() {
	} else {
		if g.keyJustPressed(ebiten.KeyF4) {
			g.soundMenuRequested = true
			return nil
		}
		if g.keyJustPressed(ebiten.KeyF10) {
			g.quitPromptRequested = true
			return nil
		}
		if g.keyJustPressed(ebiten.KeyEscape) {
			g.frontendMenuRequested = true
			ebiten.SetCursorMode(ebiten.CursorModeVisible)
			return nil
		}
		if g.bindingJustPressed(bindingAutomap) {
			if g.mode == viewWalk {
				g.mode = viewMap
				g.setHUDMessage("Automap Opened", 35)
			} else {
				g.mode = viewWalk
				g.mouseLookSet = false
				g.mouseLookSuppressTicks = detailMouseSuppressTicks
				g.setHUDMessage("Automap Closed", 35)
			}
		}
		g.edgeInputPass = true
		g.updateParityControls()
		if g.keyJustPressed(ebiten.KeyF5) {
			if g.opts.SourcePortMode {
				g.cycleSourcePortDetailLevel()
			} else {
				g.cycleDetailLevel()
			}
		}
		if g.bindingJustPressed(bindingUse) {
			g.pendingUse = true
		}
		if g.isDead && g.deathRestartJustPressed() {
			g.requestLevelRestart()
		}
		g.edgeInputPass = false
	}

	// Build the local tic from current input and send it upstream.
	localTic := g.buildLocalCoopTic()
	if err := src.SendLocalTic(localTic); err != nil {
		return fmt.Errorf("coop send tic: %w", err)
	}

	// Advance only as many tics as all active peers have available.
	now := time.Now()
	ticDur := time.Second / doomTicsPerSecond
	if g.watchTickStamp.IsZero() {
		g.watchTickStamp = now
	}
	g.watchTickAccum += now.Sub(g.watchTickStamp)
	g.watchTickStamp = now

	budget := 0
	for g.watchTickAccum >= ticDur {
		g.watchTickAccum -= ticDur
		budget++
	}
	// Gate on the lockstep: don't advance beyond what all peers have sent.
	ready := src.ReadyTics()
	if budget > ready {
		budget = ready
	}
	if budget > coopMaxCatchUpTics {
		budget = coopMaxCatchUpTics
	}

	for i := 0; i < budget; i++ {
		g.edgeInputPass = i == 0
		g.capturePrevState()

		// Step local player. For catch-up tics beyond the first, send a
		// neutral tic — input has already been consumed for this frame.
		var tic demo.Tic
		if i == 0 {
			tic = localTic
		} else {
			// neutral: no movement, keep angle
			tic = demo.Tic{}
			if err := src.SendLocalTic(tic); err != nil {
				return fmt.Errorf("coop send catch-up tic: %w", err)
			}
		}
		cmd, usePressed, fireHeld := demoTicCommand(DemoTic(tic))
		g.runGameplayTic(cmd, usePressed, fireHeld)

		// Step each remote player with their received tic.
		for _, rp := range g.remotePlayers {
			tc, ok, err := src.PollPeerTic(byte(rp.slot))
			if err != nil {
				return fmt.Errorf("coop peer tic slot %d: %w", rp.slot, err)
			}
			if ok {
				g.stepRemotePlayer(rp, tc)
			}
		}

		g.syncPeerStartsFromRemotePlayers()
		g.discoverLinesAroundPlayer()
		g.State.SetCamera(float64(g.p.x)/fracUnit, float64(g.p.y)/fracUnit)
		g.tickDelayedSounds()
		g.flushSoundEvents()
		g.tickStatusWidgets()
		if g.useFlash > 0 {
			g.useFlash--
		}
		g.tickChatHistory()
		if g.damageFlashTic > 0 {
			g.damageFlashTic--
		}
		if g.bonusFlashTic > 0 {
			g.bonusFlashTic--
		}
		g.tickDelayedSwitchReverts()
		g.markSimUpdate(time.Now())
	}
	g.edgeInputPass = false

	// Checkpoint: canonical peer (slot 1) sends a hash every checkpointIntervalTics.
	// All peers verify incoming checkpoints and request resync on mismatch.
	g.tickCoopCheckpoint()

	// Keyframe: canonical peer uploads a full snapshot periodically.
	g.tickCoopKeyframe()

	g.publishRuntimeSettingsIfChanged()
	return nil
}

// tickCoopCheckpoint handles periodic desync detection.
// The canonical peer (slot 1) emits a checkpoint every checkpointIntervalTics.
// Non-canonical peers compare any received checkpoint against their local hash.
func (g *game) tickCoopCheckpoint() {
	src := g.opts.CoopPeers
	if src == nil {
		return
	}

	localID := src.LocalPlayerID()

	// Canonical peer sends its hash to the server for relay.
	if localID == 1 && g.worldTic > 0 && g.worldTic%checkpointIntervalTics == 0 {
		_ = src.SendCheckpoint(uint32(g.worldTic), g.SimChecksum())
	}

	// All non-canonical peers check incoming checkpoints.
	if localID != 1 {
		for {
			cp, ok := src.PollCheckpoint()
			if !ok {
				break
			}
			if cp.Tic > uint32(g.worldTic) {
				// Haven't reached this tic yet — ignore for now.
				break
			}
			// We can only verify the checkpoint at the exact tic it was taken.
			// If we've already advanced past it, we trust the state is fine unless
			// we had a prior mismatch. Only verify if we're at that exact tic.
			if cp.Tic == uint32(g.worldTic) {
				local := g.SimChecksum()
				if local != cp.Hash {
					_ = src.SendDesyncNotify(cp.Tic, local)
				}
			}
		}
	}
}

// buildLocalCoopTic constructs a demo.Tic from current walk-mode input.
// It mirrors the input collection in updateWalkMode and quantizes to demo
// precision so all peers see the same values.
func (g *game) buildLocalCoopTic() demo.Tic {
	cmd := moveCmd{}
	usePressed := false
	fireHeld := false
	if !g.chatComposeOpen {
		speed := g.currentRunSpeed()
		strafeMod := g.bindingHeld(bindingStrafeModifier)
		if g.bindingHeld(bindingMoveForward) {
			cmd.forward += forwardMove[speed]
		}
		if g.bindingHeld(bindingMoveBackward) {
			cmd.forward -= forwardMove[speed]
		}
		if g.bindingHeld(bindingStrafeLeft) {
			cmd.side -= sideMove[speed]
		}
		if g.bindingHeld(bindingStrafeRight) {
			cmd.side += sideMove[speed]
		}
		if g.bindingHeld(bindingTurnLeft) {
			if strafeMod {
				cmd.side -= sideMove[speed]
			} else {
				cmd.turn += 1
			}
		}
		if g.bindingHeld(bindingTurnRight) {
			if strafeMod {
				cmd.side += sideMove[speed]
			} else {
				cmd.turn -= 1
			}
		}
		if g.pendingUse {
			usePressed = true
			g.pendingUse = false
		}
		fireHeld = g.bindingHeld(bindingFire)
		if g.opts.MouseLook {
			cmd.turnRaw += g.input.mouseTurnRawAccum
			g.input.mouseTurnRawAccum = 0
		}
		cmd.run = speed == 1
	}
	cmd = quantizeMoveCmdToDemo(cmd)
	return g.buildOutgoingDemoTic(cmd, usePressed, fireHeld)
}

// applyCoopKeyframes drains any incoming keyframes from the server and applies
// mandatory ones immediately (desync recovery / late-join catch-up).
func (g *game) applyCoopKeyframes() error {
	src := g.opts.CoopPeers
	if src == nil || g.opts.LoadKeyframe == nil {
		return nil
	}
	applied := false
	for i := 0; i < 8; i++ {
		blob, mandatory, ok := src.PollKeyframe()
		if !ok {
			break
		}
		if !mandatory {
			continue
		}
		if err := g.opts.LoadKeyframe(blob); err != nil {
			return fmt.Errorf("apply coop keyframe: %w", err)
		}
		applied = true
	}
	if applied {
		g.clearPendingSoundState()
	}
	return nil
}

// tickCoopKeyframe uploads a full game state snapshot to the relay server
// every keyframeIntervalTics. Only the canonical peer (slot 1) does this.
func (g *game) tickCoopKeyframe() {
	src := g.opts.CoopPeers
	if src == nil || g.opts.CaptureKeyframe == nil {
		return
	}
	if src.LocalPlayerID() != 1 {
		return
	}
	if g.worldTic == 0 || g.worldTic%keyframeIntervalTics != 0 {
		return
	}
	blob, err := g.opts.CaptureKeyframe()
	if err != nil || len(blob) == 0 {
		return
	}
	_ = src.SendKeyframe(uint32(g.worldTic), blob)
}

// applyCoopRoster syncs g.remotePlayers to the given set of remote peer IDs.
func (g *game) applyCoopRoster(peerIDs []byte) {
	if g.remotePlayers == nil {
		g.remotePlayers = make(map[int]*remotePlayer)
	}
	// Add newly joined peers.
	active := make(map[int]bool, len(peerIDs))
	for _, id := range peerIDs {
		slot := int(id)
		active[slot] = true
		if _, exists := g.remotePlayers[slot]; !exists {
			p := spawnRemotePlayer(g.m, slot)
			g.remotePlayers[slot] = &remotePlayer{slot: slot, p: p}
		}
	}
	// Remove departed peers.
	for slot := range g.remotePlayers {
		if !active[slot] {
			delete(g.remotePlayers, slot)
		}
	}
	g.syncPeerStartsFromRemotePlayers()
}

// syncPeerStartsFromRemotePlayers updates peerStarts so the automap renders
// remote players at their current simulated positions.
func (g *game) syncPeerStartsFromRemotePlayers() {
	g.peerStarts = g.peerStarts[:0]
	for _, rp := range g.remotePlayers {
		g.peerStarts = append(g.peerStarts, playerStart{
			slot:  rp.slot,
			x:     rp.p.x,
			y:     rp.p.y,
			angle: rp.p.angle,
		})
	}
}
