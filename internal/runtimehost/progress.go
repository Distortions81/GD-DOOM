package runtimehost

type ProgressSignals struct {
	HasNewGame    bool
	HasQuitPrompt bool
	HasReadThis   bool
	HasRestart    bool
}

type ProgressHandlers struct {
	OnNewGame    func() error
	OnQuitPrompt func() error
	OnReadThis   func() error
	OnRestart    func() error
}

func HandleProgress(signals ProgressSignals, handlers ProgressHandlers) (bool, error) {
	switch {
	case signals.HasNewGame:
		if handlers.OnNewGame != nil {
			return true, handlers.OnNewGame()
		}
		return true, nil
	case signals.HasQuitPrompt:
		if handlers.OnQuitPrompt != nil {
			return true, handlers.OnQuitPrompt()
		}
		return true, nil
	case signals.HasReadThis:
		if handlers.OnReadThis != nil {
			return true, handlers.OnReadThis()
		}
		return true, nil
	case signals.HasRestart:
		if handlers.OnRestart != nil {
			return true, handlers.OnRestart()
		}
		return true, nil
	default:
		return false, nil
	}
}
