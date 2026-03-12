package runtimehost

import "github.com/hajimehoshi/ebiten/v2"

type Draw struct {
	Prepare   func()
	HasGame   func() bool
	DrawEmpty func(*ebiten.Image)

	TransitionActive      func() bool
	TransitionNeedsResize func() bool
	InvalidateTransition  func()
	EnsureTransitionReady func()
	TransitionInitialized func() bool
	DrawTransitionFrame   func(*ebiten.Image)
	ClearTransition       func()

	IntermissionActive func() bool
	DrawIntermission   func(*ebiten.Image)
	FrontendActive     func() bool
	DrawFrontend       func(*ebiten.Image)
	FinaleActive       func() bool
	DrawFinale         func(*ebiten.Image)
	DrawGameplay       func(*ebiten.Image)

	QuitPromptActive func() bool
	DrawQuitPrompt   func(*ebiten.Image)
	CaptureLastFrame func(*ebiten.Image)
}

func RunDraw(screen *ebiten.Image, d Draw) {
	if d.Prepare != nil {
		d.Prepare()
	}
	if d.HasGame != nil && !d.HasGame() {
		if d.DrawEmpty != nil {
			d.DrawEmpty(screen)
		}
		return
	}
	if d.TransitionActive != nil && d.TransitionActive() {
		if d.TransitionNeedsResize != nil && d.TransitionNeedsResize() && d.InvalidateTransition != nil {
			d.InvalidateTransition()
		}
		if d.EnsureTransitionReady != nil {
			d.EnsureTransitionReady()
		}
		if d.TransitionInitialized != nil && d.TransitionInitialized() {
			if d.DrawTransitionFrame != nil {
				d.DrawTransitionFrame(screen)
			}
			if d.QuitPromptActive != nil && d.QuitPromptActive() && d.DrawQuitPrompt != nil {
				d.DrawQuitPrompt(screen)
			}
			return
		}
		if d.ClearTransition != nil {
			d.ClearTransition()
		}
	}
	if d.IntermissionActive != nil && d.IntermissionActive() {
		if d.DrawIntermission != nil {
			d.DrawIntermission(screen)
		}
		if d.QuitPromptActive != nil && d.QuitPromptActive() && d.DrawQuitPrompt != nil {
			d.DrawQuitPrompt(screen)
		}
		if d.CaptureLastFrame != nil {
			d.CaptureLastFrame(screen)
		}
		return
	}
	if d.FrontendActive != nil && d.FrontendActive() {
		if d.DrawFrontend != nil {
			d.DrawFrontend(screen)
		}
		if d.QuitPromptActive != nil && d.QuitPromptActive() && d.DrawQuitPrompt != nil {
			d.DrawQuitPrompt(screen)
		}
		if d.CaptureLastFrame != nil {
			d.CaptureLastFrame(screen)
		}
		return
	}
	if d.FinaleActive != nil && d.FinaleActive() {
		if d.DrawFinale != nil {
			d.DrawFinale(screen)
		}
		if d.QuitPromptActive != nil && d.QuitPromptActive() && d.DrawQuitPrompt != nil {
			d.DrawQuitPrompt(screen)
		}
		if d.CaptureLastFrame != nil {
			d.CaptureLastFrame(screen)
		}
		return
	}
	if d.DrawGameplay != nil {
		d.DrawGameplay(screen)
	}
}
