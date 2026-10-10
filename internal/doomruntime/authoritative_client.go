package doomruntime

import (
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"gddoom/internal/demo"
	"gddoom/internal/netgame"

	"github.com/hajimehoshi/ebiten/v2"
)

type authorityClientUpdateState struct {
	mouseTurnPending int64
	scoreboardHeld   bool
	stamp            time.Time
	accum            time.Duration
	step             time.Duration
	started          bool
	sequence         uint32
	turnHeld         int
	snapshotAt       time.Time
	serverStamp      time.Time
	serverTick       uint32
	chatStamp        time.Time
	chatAccum        time.Duration
}

func (g *game) updateAuthoritativeClient() error {
	g.captureAuthorityScoreboardInput()
	// These controls only change local presentation or request a menu. The
	// legacy parity handler also owns cheats/save/restart and is not used here.
	chatOwnsInput := g.handleChatInput()
	if !g.mouseLookBlocked() && !chatOwnsInput {
		if g.keyJustPressed(ebiten.KeyF4) {
			g.soundMenuRequested = true
		}
		if g.keyJustPressed(ebiten.KeyF10) {
			g.quitPromptRequested = true
		}
		if g.keyJustPressed(ebiten.KeyEscape) {
			g.frontendMenuRequested = true
		}
		if g.bindingJustPressed(bindingAutomap) {
			if g.mode == viewWalk {
				g.mode = viewMap
			} else {
				g.mode = viewWalk
			}
			g.mouseLookSet = false
		}
		if g.keyJustPressed(ebiten.KeyCapsLock) {
			g.alwaysRun = !g.alwaysRun
		}
		if g.keyJustPressed(ebiten.KeyF5) {
			if g.opts.SourcePortMode {
				g.cycleSourcePortDetailLevel()
			} else {
				g.cycleDetailLevel()
			}
		}
		if g.opts.SourcePortMode && g.keyJustPressed(ebiten.KeyBackslash) {
			g.toggleAuthoritativeMouseLook()
		}
		if g.opts.AuthorityClient.Welcome().PlayerID == 0 && g.keyJustPressed(ebiten.KeyF12) {
			if spectator, ok := g.opts.AuthorityClient.(interface{ FollowPlayer(byte) error }); ok {
				if err := spectator.FollowPlayer(0); err != nil {
					g.setHUDMessage("COULD NOT CHANGE SPECTATOR VIEW", 70)
				}
			}
		}
		if g.bindingJustPressed(bindingUse) || (g.isDead && g.deathRestartJustPressed()) {
			g.pendingUse = true
		}
		g.captureAuthoritativeWeaponInput()
	}
	applyRuntimeCursorMode(!g.mouseLookBlocked() && !chatOwnsInput && g.mode == viewWalk && g.shouldCaptureCursor())
	if chatOwnsInput {
		return g.updateAuthoritativeClientAt(time.Now(), nil)
	}
	return g.updateAuthoritativeClientAt(time.Now(), g.buildAuthoritativeClientTic)
}

// updateAuthoritativeClientAt is also the menu/background pump: nil sample
// submits neutral controls while still reconciling and acknowledging snapshots.
// Tests inject both a clock and commands without querying native input devices.
func (g *game) updateAuthoritativeClientAt(now time.Time, sample func() demo.Tic) error {
	if sample == nil {
		g.clientUpdate.scoreboardHeld = false
	}
	client := g.opts.AuthorityClient
	if client == nil {
		return nil
	}
	g.captureAuthoritativeMouseInput(sample != nil)
	if g.authorityFailure != nil {
		return nil
	}
	if err := g.updateAuthoritativeChatAt(now); err != nil {
		return fmt.Errorf("authoritative chat: %w", err)
	}
	if client.Welcome().PlayerID == 0 {
		return g.updateAuthoritativeObserverAt(now)
	}
	if g.opts.DemoScript != nil || g.opts.LiveTicSource != nil || g.opts.LiveTicSink != nil || g.opts.CoopPeers != nil || strings.TrimSpace(g.opts.RecordDemoPath) != "" {
		return fmt.Errorf("authoritative client cannot mix demo/legacy multiplayer input")
	}
	if g.clientPrediction == nil {
		prediction, err := newClientPrediction(g, client.Welcome())
		if err != nil {
			return err
		}
		g.clientPrediction = prediction
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
		applied, err := p.reconcileAt(snapshot, now)
		if err != nil {
			g.setAuthorityConnectionFailure(err)
			return nil
		}
		acknowledge = acknowledge || applied
		if applied {
			g.clientUpdate.observeServerTick(snapshot.Tick, now)
			g.clientUpdate.snapshotAt = now
		}
	}
	if g.authorityConnectionBlocked() {
		g.clearAuthoritativeInputIntent()
		return nil
	}
	clock := &g.clientUpdate
	if !p.Ready() {
		g.clientUpdate.mouseTurnPending = 0
		clock.stamp, clock.accum = now, 0
		return nil
	}
	const ticDuration = time.Second / netgame.TickRate
	if clock.step == 0 {
		clock.step = ticDuration
	}
	// A fresh baseline may overtake prediction after a stall. Only then may
	// we establish a new lead; ordinary latency changes must not skip input
	// slots or simulate several movement steps in one command interval.
	reanchor := !p.hasInput || p.lastInput.Tick <= p.AuthoritativeTic()
	budget := 0
	if !clock.started {
		clock.started, clock.stamp = true, now
		budget = 1
	} else {
		elapsed := now.Sub(clock.stamp)
		clock.stamp = now
		if elapsed > 0 {
			clock.accum += elapsed
		}
		budget = int(clock.accum / clock.step)
		clock.accum %= clock.step
		if budget > 4 {
			budget = 4
		}
	}
	stepped := false
	for i := 0; i < budget; i++ {
		tick, ok := p.NextInputTic()
		if !ok || clock.sequence == math.MaxUint32 {
			return netgame.ErrInputEpochExhausted
		}
		lead := g.authorityInputTarget(now)
		if lead > math.MaxUint32 {
			return netgame.ErrInputEpochExhausted
		}
		if reanchor && uint64(tick) < lead {
			tick = uint32(lead)
		}
		// Do not keep walking into an unknown world after a stalled stream. A
		// fresh baseline reopens the one-second schedule without reusing tics.
		if lead > uint64(p.AuthoritativeTic())+netgame.TickRate || uint64(tick) > uint64(p.AuthoritativeTic())+netgame.TickRate {
			break
		}
		command := demo.Tic{}
		if sample != nil {
			command = sample()
		}
		g.capturePrevState()
		input := netgame.Input{Sequence: clock.sequence + 1, Tick: tick, Command: command}
		if err := p.Predict(input); err != nil {
			if errors.Is(err, ErrPredictionWindow) {
				break
			}
			return fmt.Errorf("predict local input: %w", err)
		}
		clock.sequence++
		reanchor, stepped = false, true
		acknowledge = true
	}
	target := g.authorityInputTarget(now)
	horizon := uint64(p.AuthoritativeTic()) + netgame.TickRate
	if stepped || (budget == 0 && target <= horizon && uint64(p.PredictedTic()) < horizon) {
		// Gently recover the negotiated lead without inserting empty input
		// slots. A one-tic dead band ignores packet arrival phase; the maximum
		// 5% clock adjustment also bounds the change in visible movement speed.
		step := ticDuration
		next := uint64(p.PredictedTic()) + 1
		if target > next+1 {
			step = ticDuration * 20 / 21
		} else if next > target+1 {
			step = ticDuration * 20 / 19
		}
		changed := clock.step != step
		// Keep the interpolation phase continuous when its duration changes.
		if changed {
			clock.accum = clock.accum * step / clock.step
		}
		clock.step = step
		if stepped || changed {
			g.markSimUpdate(now.Add(-clock.accum))
		}
	}
	if acknowledge {
		pending := p.PendingInputs()
		if len(pending) > netgame.MaxInputBatch {
			pending = pending[len(pending)-netgame.MaxInputBatch:]
		}
		if err := client.SendInputs(netgame.InputBatch{Epoch: p.epoch, SnapshotAck: p.SnapshotID(), Inputs: pending}); err != nil {
			return fmt.Errorf("send authoritative input: %w", err)
		}
	}
	g.publishRuntimeSettingsIfChanged()
	if g.snd != nil {
		g.snd.tick()
	}
	return nil
}

func (g *game) authorityInputTarget(now time.Time) uint64 {
	client := g.opts.AuthorityClient
	var rtt time.Duration
	if latency, ok := client.(interface{ RoundTripTime() time.Duration }); ok {
		rtt = latency.RoundTripTime()
	}
	if rtt < 0 {
		rtt = 0
	}
	if rtt > time.Second {
		rtt = time.Second
	}
	// Snapshots travel to us and inputs travel back, so the full round trip
	// belongs in the deadline lead. This only schedules intent; it grants no
	// additional movement steps to any command.
	lead := uint64(client.Welcome().InputLead) + uint64((rtt*time.Duration(netgame.TickRate)+time.Second-1)/time.Second) + 1
	if lead > netgame.TickRate {
		lead = netgame.TickRate
	}
	if now.Sub(g.clientUpdate.snapshotAt) > time.Second {
		return uint64(g.clientPrediction.AuthoritativeTic()) + netgame.TickRate + 1
	}
	return g.clientUpdate.estimatedServerTick(now) + lead
}

func (clock *authorityClientUpdateState) estimatedServerTick(now time.Time) uint64 {
	tick := uint64(clock.serverTick)
	if elapsed := now.Sub(clock.serverStamp); elapsed > 0 {
		tick += uint64(elapsed / (time.Second / netgame.TickRate))
	}
	return tick
}

func (clock *authorityClientUpdateState) observeServerTick(tick uint32, now time.Time) {
	// Packet arrival jitter must not rewind our estimate of the server clock.
	// Retain its phase until a faster baseline advances it. A long silence is
	// a resynchronization boundary, not a clock estimate to extrapolate forever.
	if clock.serverStamp.IsZero() || now.Sub(clock.snapshotAt) > time.Second || uint64(tick) > clock.estimatedServerTick(now) {
		clock.serverTick, clock.serverStamp = tick, now
	}
}

func (g *game) captureAuthoritativeWeaponInput() {
	for slot, action := range []bindingAction{bindingWeapon1, bindingWeapon2, bindingWeapon3, bindingWeapon4, bindingWeapon5, bindingWeapon6, bindingWeapon7} {
		if g.bindingJustPressed(action) {
			g.demoWeaponSlot = slot + 1
		}
	}
	step := 0
	if g.bindingJustPressed(bindingWeaponNext) || g.input.wheelY < 0 {
		step = 1
	}
	if g.bindingJustPressed(bindingWeaponPrev) || g.input.wheelY > 0 {
		step = -1
	}
	if step == 0 {
		return
	}
	order := weaponCycleOrder()
	start := 0
	for i, w := range order {
		if w == g.inventory.ReadyWeapon {
			start = i
			break
		}
	}
	for i := 1; i <= len(order); i++ {
		index := (start + i*step + len(order)*2) % len(order)
		w := order[index]
		if w != g.inventory.ReadyWeapon && g.weaponOwned(w) {
			g.demoWeaponSlot = demoTraceWeaponID(w) + 1
			if w == weaponSuperShotgun {
				g.demoWeaponSlot = 3
			}
			return
		}
	}
}

func (g *game) buildAuthoritativeClientTic() demo.Tic {
	if g.mouseLookBlocked() || g.chatComposeOpen {
		g.clearAuthoritativeInputIntent()
		return demo.Tic{}
	}
	cmd := moveCmd{}
	speed := g.currentRunSpeed()
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
	strafe := g.bindingHeld(bindingStrafeModifier)
	if g.bindingHeld(bindingTurnLeft) {
		if strafe {
			cmd.side -= sideMove[speed]
		} else {
			cmd.turn++
		}
	}
	if g.bindingHeld(bindingTurnRight) {
		if strafe {
			cmd.side += sideMove[speed]
		} else {
			cmd.turn--
		}
	}
	use, fire := g.pendingUse, g.bindingHeld(bindingFire)
	g.pendingUse = false
	if g.input.touchLeftX != 0 || g.input.touchLeftY != 0 || g.input.touchRightX != 0 || g.input.touchRightY != 0 {
		cmd.forward = int64(float64(forwardMove[1]) * (-g.input.touchLeftY))
		cmd.side = int64(float64(sideMove[1]) * g.input.touchLeftX)
		cmd.turn = 0
		cmd.turnRaw -= int64(float64(angleTurn[1]) * g.input.touchRightX)
		fire = fire || g.input.touchRightY < -0.65
		use = use || g.input.touchRightY > 0.65
	}
	if g.opts.MouseLook {
		cmd.turnRaw += g.clientUpdate.mouseTurnPending
	}
	g.clientUpdate.mouseTurnPending = 0
	cmd.run = speed == 1
	// Keyboard acceleration is input sampling state, separate from replay of
	// already encoded raw angle commands and from the server's player state.
	previous := g.turnHeld
	g.turnHeld = g.clientUpdate.turnHeld
	tic := g.buildOutgoingDemoTic(cmd, use, fire)
	g.turnHeld = previous
	if cmd.turn != 0 && !g.isDead && g.p.reactionTime <= 0 {
		g.clientUpdate.turnHeld++
	} else {
		g.clientUpdate.turnHeld = 0
	}
	return tic
}

func (g *game) clearAuthoritativeInputIntent() {
	g.pendingUse, g.mouseLookSet = false, false
	g.demoWeaponSlot, g.clientUpdate.turnHeld = 0, 0
	g.input.mouseTurnRawAccum, g.clientUpdate.mouseTurnPending = 0, 0
}

// The host clears sampled input after every Update, while network commands are
// scheduled at 35 Hz. Retain relative motion across frames with no command, and
// consume it exactly once when the next command is built. Startup/layout mouse
// suppression must also expire here: the authority path never runs updateWalkMode.
func (g *game) captureAuthoritativeMouseInput(active bool) {
	if !active || g.mouseLookBlocked() || g.chatComposeOpen || g.authorityConnectionBlocked() || g.opts.AuthorityClient.Welcome().PlayerID == 0 {
		g.clearAuthoritativeInputIntent()
		return
	}
	if !g.opts.MouseLook {
		g.mouseLookSet = false
		g.input.mouseTurnRawAccum, g.clientUpdate.mouseTurnPending = 0, 0
		return
	}
	if g.mouseLookSuppressTicks > 0 {
		g.mouseLookSuppressTicks--
		g.input.mouseTurnRawAccum, g.clientUpdate.mouseTurnPending = 0, 0
		return
	}
	// A command represents at most a signed half-turn; bounding pending input
	// also prevents a stalled stream from retaining an unbounded cursor jump.
	const limit = int64(math.MaxInt16) << 16
	bound := func(value int64) int64 {
		if value < -limit {
			return -limit
		}
		if value > limit {
			return limit
		}
		return value
	}
	delta := bound(g.input.mouseTurnRawAccum)
	g.clientUpdate.mouseTurnPending = bound(g.clientUpdate.mouseTurnPending + delta)
	g.input.mouseTurnRawAccum = 0
}

func (g *game) toggleAuthoritativeMouseLook() {
	g.opts.MouseLook = !g.opts.MouseLook
	g.mouseLookSet = false
	g.input.mouseTurnRawAccum, g.clientUpdate.mouseTurnPending = 0, 0
	label := "Mouse Look OFF"
	if g.opts.MouseLook {
		label = "Mouse Look ON"
		g.mouseLookSuppressTicks = detailMouseSuppressTicks
	}
	g.setHUDMessage(label, 70)
}
