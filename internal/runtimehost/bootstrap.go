package runtimehost

type Bootstrap struct {
	BuildRuntime          func()
	InitMusicPlayback     func()
	ShouldStartInFrontend func() bool
	StartFrontend         func()
	StartMapMusic         func()
	CaptureSettings       func()
	ShouldShowBootSplash  func() bool
	QueueBootSplash       func()
	InitPost              func()
}

func RunBootstrap(b Bootstrap) {
	if b.BuildRuntime != nil {
		b.BuildRuntime()
	}
	if b.InitMusicPlayback != nil {
		b.InitMusicPlayback()
	}
	if b.ShouldStartInFrontend != nil && b.ShouldStartInFrontend() {
		if b.StartFrontend != nil {
			b.StartFrontend()
		}
	} else if b.StartMapMusic != nil {
		b.StartMapMusic()
	}
	if b.CaptureSettings != nil {
		b.CaptureSettings()
	}
	if b.ShouldShowBootSplash != nil && b.ShouldShowBootSplash() {
		if b.QueueBootSplash != nil {
			b.QueueBootSplash()
		}
	}
	if b.InitPost != nil {
		b.InitPost()
	}
}
