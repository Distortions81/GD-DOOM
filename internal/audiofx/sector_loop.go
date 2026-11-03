package audiofx

import (
	"gddoom/internal/media"
	"math"
)

// LoopSequencer turns timed refreshes into finite PCM windows with a continuous
// sample phase. A one-shot on the same origin cancels the previous loop; absent
// refreshes expire naturally when the current window ends. No audio goroutine
// reads mutable loop state.
type LoopSequencer struct{ states map[string]loopWindow }
type loopWindow struct {
	tick, timeout, next int
	pitch               float32
	sample              media.PCMSample
}

func (s *LoopSequencer) Clear() { s.states = nil }
func (s *LoopSequencer) Chunk(group string, timeout, tick int, sample media.PCMSample, pitch float32) (media.PCMSample, float32, bool) {
	if timeout <= 0 || group == "" {
		delete(s.states, group)
		return sample, pitch, true
	}
	if len(sample.Data) == 0 || sample.SampleRate <= 0 {
		return sample, pitch, true
	}
	previous, ok := s.states[group]
	offset := 0
	if ok && tick == previous.tick {
		return previous.sample, previous.pitch, false
	}
	if ok && tick == previous.tick+previous.timeout {
		offset = previous.next
		pitch = previous.pitch
	}
	// Sector movement uses the canonical Doom base rate; retain exact durations
	// for other rates too. Pitch is held across refreshes, as in DSDA channels.
	if pitch <= 0 {
		pitch = 1
	}
	count := int(math.Round(float64(sample.SampleRate) * float64(timeout) / 35 * float64(pitch)))
	if count <= 0 {
		return sample, pitch, true
	}
	data := make([]byte, count)
	for i := range data {
		data[i] = sample.Data[(offset+i)%len(sample.Data)]
	}
	window := media.PCMSample{SampleRate: sample.SampleRate, Data: data}
	if s.states == nil {
		s.states = make(map[string]loopWindow)
	}
	s.states[group] = loopWindow{tick: tick, timeout: timeout, next: (offset + count) % len(sample.Data), pitch: pitch, sample: window}
	return window, pitch, true
}
