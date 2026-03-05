package music

import "errors"

const (
	DefaultStreamChunkFrames = 4096
	DefaultStreamLookahead   = OutputSampleRate * 2
)

var errNilStreamDriver = errors.New("music: nil stream driver")

// StreamRenderer incrementally renders parsed events into fixed-size PCM chunks.
type StreamRenderer struct {
	driver *Driver
	events []Event
	idx    int
	wait   int
	waited bool
	done   bool
}

func NewMUSStreamRenderer(driver *Driver, musData []byte) (*StreamRenderer, error) {
	if driver == nil {
		return nil, errNilStreamDriver
	}
	events, err := ParseMUS(musData)
	if err != nil {
		return nil, err
	}
	driver.Reset()
	return &StreamRenderer{
		driver: driver,
		events: events,
	}, nil
}

// NextChunkS16LE renders up to maxFrames of stereo s16 PCM bytes.
// It returns done=true when End has been consumed and no queued wait remains.
func (sr *StreamRenderer) NextChunkS16LE(maxFrames int) (chunk []byte, done bool, err error) {
	if sr == nil || sr.driver == nil {
		return nil, true, errNilStreamDriver
	}
	if maxFrames <= 0 {
		maxFrames = DefaultStreamChunkFrames
	}
	if sr.done {
		return nil, true, nil
	}
	out := make([]int16, 0, maxFrames*2)
	for len(out) < maxFrames*2 {
		if sr.wait > 0 {
			need := maxFrames - (len(out) / 2)
			if need <= 0 {
				break
			}
			n := sr.wait
			if n > need {
				n = need
			}
			if n > 0 {
				out = append(out, sr.driver.generateStereoS16(n)...)
				sr.wait -= n
			}
			if sr.wait > 0 {
				break
			}
		}
		if sr.idx >= len(sr.events) {
			sr.done = true
			break
		}
		ev := sr.events[sr.idx]
		if !sr.waited && ev.DeltaTics > 0 {
			sr.wait = int((uint64(ev.DeltaTics) * uint64(sr.driver.sampleRate)) / uint64(sr.driver.ticRate))
			sr.waited = true
			if sr.wait > 0 {
				continue
			}
		}
		sr.driver.applyEvent(ev)
		sr.idx++
		sr.waited = false
		if ev.Type == EventEnd {
			sr.done = true
			break
		}
	}
	if len(out) == 0 {
		return nil, sr.done, nil
	}
	return PCMInt16ToBytesLE(out), sr.done, nil
}
