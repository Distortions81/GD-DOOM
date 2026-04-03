package runtimehost

import "errors"

var ErrTerminate = errors.New("runtimehost: terminate")

type Update struct {
	QuitPromptActive    func() bool
	HandleQuitPrompt    func() error
	QuitPromptTriggered func() bool
	RequestQuitPrompt   func()

	TransitionActive        func() bool
	TransitionIsBootHolding func() bool
	SkipRequested           func() bool
	SkipTransitionHold      func()
	TickTransition          func()

	FinaleActive func() bool
	TickFinale   func() bool

	FrontendActive         func() bool
	DemoActive             func() bool
	UpdateRuntimeForDemo   func() error
	AdvanceFrontendAttract func() bool
	TickFrontend           func() error

	IntermissionActive func() bool
	TickIntermission   func() bool
	FinishIntermission func()

	UpdateRuntime            func() error
	HandleRuntimeProgress    func() (bool, error)
	HandleRuntimeTermination func() (bool, error)
}

func RunUpdate(u Update) error {
	if u.QuitPromptActive != nil && u.QuitPromptActive() {
		if u.HandleQuitPrompt != nil {
			return u.HandleQuitPrompt()
		}
		return nil
	}
	if u.QuitPromptTriggered != nil && u.QuitPromptTriggered() {
		if u.RequestQuitPrompt != nil {
			u.RequestQuitPrompt()
		}
		return nil
	}
	if u.TransitionActive != nil && u.TransitionActive() {
		if u.TransitionIsBootHolding != nil && u.TransitionIsBootHolding() && u.SkipRequested != nil && u.SkipRequested() {
			if u.SkipTransitionHold != nil {
				u.SkipTransitionHold()
			}
		}
		if u.TickTransition != nil {
			u.TickTransition()
		}
		return nil
	}
	if u.FinaleActive != nil && u.FinaleActive() {
		if u.TickFinale != nil && u.TickFinale() {
			return ErrTerminate
		}
		return nil
	}
	if u.FrontendActive != nil && u.FrontendActive() {
		if u.DemoActive != nil && u.DemoActive() {
			if u.UpdateRuntimeForDemo != nil {
				if err := u.UpdateRuntimeForDemo(); err != nil {
					return err
				}
			}
			if u.HandleRuntimeProgress != nil {
				handled, err := u.HandleRuntimeProgress()
				if err != nil {
					return err
				}
				if handled {
					return nil
				}
			}
		}
		if u.TickFrontend != nil {
			return u.TickFrontend()
		}
		return nil
	}
	if u.IntermissionActive != nil && u.IntermissionActive() {
		if u.TickIntermission != nil && u.TickIntermission() {
			if u.FinishIntermission != nil {
				u.FinishIntermission()
			}
		}
		return nil
	}

	if u.UpdateRuntime != nil {
		if err := u.UpdateRuntime(); err != nil {
			if u.HandleRuntimeTermination != nil {
				handled, terr := u.HandleRuntimeTermination()
				if terr != nil {
					return terr
				}
				if handled {
					return nil
				}
			}
			return err
		}
	}
	if u.HandleRuntimeProgress != nil {
		handled, err := u.HandleRuntimeProgress()
		if err != nil {
			return err
		}
		if handled {
			return nil
		}
	}
	return nil
}
