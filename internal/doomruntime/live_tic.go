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
	if g.keyJustPressed(ebiten.KeyF4) || g.keyJustPressed(ebiten.KeyF10) {
		g.quitPromptRequested = true
		return nil
	}
	if g.keyJustPressed(ebiten.KeyEscape) {
		g.frontendMenuRequested = true
		ebiten.SetCursorMode(ebiten.CursorModeVisible)
		return nil
	}
	if g.keyJustPressed(ebiten.KeyTab) {
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
