package music

import (
	"errors"
	"unsafe"
)

var errNilStreamDriver = errors.New("music: nil stream driver")

func DefaultStreamChunkFrames() int {
	return streamChunkFrames()
}

func DefaultStreamLookahead() int {
	return streamLookaheadFrames()
}

func DefaultStreamChunkFramesForBackend(backend Backend) int {
	return streamChunkFramesForBackend(ResolveBackend(backend))
}

func DefaultStreamLookaheadForBackend(backend Backend) int {
	return streamLookaheadFramesForBackend(ResolveBackend(backend))
}

// StreamRenderer incrementally renders parsed events into fixed-size PCM chunks.
type StreamRenderer struct {
	driver  eventRenderer
	events  []Event
	idx     int
	wait    int
	waited  bool
	done    bool
	pcmBuf  []int16
	byteBuf []byte
}

func newStreamRendererFromParsed(driver eventRenderer, parsed *ParsedMUS) (*StreamRenderer, error) {
	if driver == nil {
		return nil, errNilStreamDriver
	}
	if parsed == nil {
		return nil, errors.New("music: nil parsed MUS")
	}
	driver.Reset()
	return &StreamRenderer{
		driver: driver,
		events: parsed.events,
	}, nil
}

func NewParsedMUSStreamRenderer(driver eventRenderer, parsed *ParsedMUS) (*StreamRenderer, error) {
	return newStreamRendererFromParsed(driver, parsed)
}

func NewMUSStreamRenderer(driver eventRenderer, musData []byte) (*StreamRenderer, error) {
	parsed, err := ParseMUSData(musData)
	if err != nil {
		return nil, err
	}
	return NewParsedMUSStreamRenderer(driver, parsed)
}

// NextChunkS16LE renders up to maxFrames of stereo s16 PCM bytes.
// It returns done=true when End has been consumed and no queued wait remains.
func (sr *StreamRenderer) NextChunkS16LE(maxFrames int) (chunk []byte, done bool, err error) {
	if sr == nil || sr.driver == nil {
		return nil, true, errNilStreamDriver
	}
	if maxFrames <= 0 {
		maxFrames = DefaultStreamChunkFrames()
	}
	if sr.done {
		return nil, true, nil
	}
	if cap(sr.pcmBuf) < maxFrames*2 {
		sr.pcmBuf = make([]int16, 0, maxFrames*2)
	}
	out := sr.pcmBuf[:0]
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
				out = append(out, sr.driver.GenerateStereoS16(n)...)
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
			sr.wait = int((uint64(ev.DeltaTics) * uint64(sr.driver.SampleRate())) / uint64(sr.driver.TicRate()))
			sr.waited = true
			if sr.wait > 0 {
				continue
			}
		}
		sr.driver.ApplyEvent(ev)
		sr.idx++
		sr.waited = false
		if ev.Type == EventEnd {
			sr.done = true
			break
		}
	}
	if len(out) == 0 {
		sr.pcmBuf = out
		return nil, sr.done, nil
	}
	sr.pcmBuf = out
	if nativeLittleEndian() {
		return pcmInt16ViewAsBytesLE(out), sr.done, nil
	}
	sr.byteBuf = PCMInt16ToBytesLEInto(sr.byteBuf[:0], out)
	return sr.byteBuf, sr.done, nil
}

func pcmInt16ViewAsBytesLE(samples []int16) []byte {
	if len(samples) == 0 {
		return nil
	}
	return unsafe.Slice((*byte)(unsafe.Pointer(unsafe.SliceData(samples))), len(samples)*2)
}

func nativeLittleEndian() bool {
	var probe uint16 = 1
	return *(*byte)(unsafe.Pointer(&probe)) == 1
}

func estimatedPCMBytesForEvents(events []Event, sampleRate int, ticRate int) int {
	if sampleRate <= 0 {
		sampleRate = OutputSampleRate
	}
	if ticRate <= 0 {
		ticRate = defaultTicRate
	}
	var total uint64
	for _, ev := range events {
		if ev.DeltaTics == 0 {
			continue
		}
		frames := (uint64(ev.DeltaTics) * uint64(sampleRate)) / uint64(ticRate)
		total += frames * 4
	}
	if total > uint64(^uint(0)>>1) {
		return int(^uint(0) >> 1)
	}
	return int(total)
}

func renderParsedMUSS16LE(driver eventRenderer, parsed *ParsedMUS) ([]byte, error) {
	stream, err := NewParsedMUSStreamRenderer(driver, parsed)
	if err != nil {
		return nil, err
	}
	pcm := make([]byte, 0, parsed.estimatedPCMBytes)
	for {
		chunk, done, err := stream.NextChunkS16LE(DefaultStreamChunkFrames())
		if err != nil {
			return nil, err
		}
		if len(chunk) > 0 {
			pcm = append(pcm, chunk...)
		}
		if done {
			return pcm, nil
		}
	}
}
