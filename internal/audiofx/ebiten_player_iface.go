package audiofx

import "time"

type ebitenPlayer interface {
	Play()
	Pause()
	Rewind() error
	SetBufferSize(time.Duration)
	SetVolume(float64)
	IsPlaying() bool
	Close() error
}
