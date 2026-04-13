//go:build js && wasm

package audiofx

import (
	"testing"
	"time"
	"unsafe"

	"gddoom/internal/media"

	"github.com/hajimehoshi/ebiten/v2/audio"
)

type fakeWASMBackend struct {
	playing    bool
	paused     int
	rewound    int
	plays      int
	lastVolume float64
	setVolumes int
}

func (f *fakeWASMBackend) Play()                         { f.playing = true; f.plays++ }
func (f *fakeWASMBackend) Pause()                        { f.playing = false; f.paused++ }
func (f *fakeWASMBackend) Rewind() error                 { f.rewound++; return nil }
func (f *fakeWASMBackend) SetBufferSize(_ time.Duration) {}
func (f *fakeWASMBackend) SetVolume(v float64)           { f.lastVolume = v; f.setVolumes++ }
func (f *fakeWASMBackend) IsPlaying() bool               { return f.playing }
func (f *fakeWASMBackend) Close() error                  { f.playing = false; return nil }

func TestPlayWASMSoundEffect_ReusesCachedVoiceWithoutRebuildingPCM(t *testing.T) {
	sample := media.PCMSample{
		SampleRate: 11025,
		Data:       []byte{1, 2, 3, 4},
	}
	key := wasmSampleKey{
		ptr:  uintptr(unsafe.Pointer(unsafe.SliceData(sample.Data))),
		len:  len(sample.Data),
		rate: sample.SampleRate,
	}
	backend := &fakeWASMBackend{}
	buf := []byte{9, 8, 7, 6}
	beforePtr := unsafe.Pointer(unsafe.SliceData(buf))
	voice := &spatialVoice{
		player: backend,
		src:    &pcmBufferSource{buf: buf},
		pinned: true,
		key:    key,
		stamp:  41,
		bucket: wasmVolumeBucketUnset,
	}
	p := &SpatialPlayer{
		ctx:    &audio.Context{},
		volume: 1,
		voices: []*spatialVoice{voice},
	}

	if ok := p.playWASMSoundEffect(sample, SpatialOrigin{}, 0, 0, false, "test"); !ok {
		t.Fatal("playWASMSoundEffect()=false want true")
	}
	afterPtr := unsafe.Pointer(unsafe.SliceData(voice.src.buf))
	if voice != p.voices[0] {
		t.Fatal("cached voice was replaced on cache hit")
	}
	if beforePtr != afterPtr {
		t.Fatal("cached PCM buffer pointer changed on cache hit")
	}
	if len(voice.src.buf) != 4 || voice.src.buf[0] != 9 || voice.src.buf[3] != 6 {
		t.Fatalf("cached PCM buffer changed on cache hit: %v", voice.src.buf)
	}
	if backend.rewound != 1 || backend.paused != 0 || backend.plays != 1 {
		t.Fatalf("backend pause/rewind/play=%d/%d/%d want 0/1/1", backend.paused, backend.rewound, backend.plays)
	}
	if backend.setVolumes != 1 || backend.lastVolume != 1 {
		t.Fatalf("backend volume=%f setCount=%d want 1/1", backend.lastVolume, backend.setVolumes)
	}
	if voice.group != "test" {
		t.Fatalf("voice group=%q want test", voice.group)
	}
	if voice.stamp != 42 {
		t.Fatalf("voice stamp=%d want 42", voice.stamp)
	}
}

func TestPlayWASMSoundEffect_ActiveCachedVoiceRewindsWithoutReplayCall(t *testing.T) {
	sample := media.PCMSample{
		SampleRate: 11025,
		Data:       []byte{1, 2, 3, 4},
	}
	key := wasmSampleKey{
		ptr:  uintptr(unsafe.Pointer(unsafe.SliceData(sample.Data))),
		len:  len(sample.Data),
		rate: sample.SampleRate,
	}
	backend := &fakeWASMBackend{playing: true}
	voice := &spatialVoice{
		player: backend,
		src:    &pcmBufferSource{buf: []byte{9, 8, 7, 6}},
		pinned: true,
		key:    key,
		bucket: wasmVolumeBuckets - 1,
	}
	p := &SpatialPlayer{
		ctx:    &audio.Context{},
		volume: 1,
		voices: []*spatialVoice{voice},
	}

	if ok := p.playWASMSoundEffect(sample, SpatialOrigin{}, 0, 0, false, "test"); !ok {
		t.Fatal("playWASMSoundEffect()=false want true")
	}
	if backend.paused != 0 {
		t.Fatalf("backend paused=%d want 0", backend.paused)
	}
	if backend.rewound != 1 {
		t.Fatalf("backend rewound=%d want 1", backend.rewound)
	}
	if backend.plays != 0 {
		t.Fatalf("backend plays=%d want 0", backend.plays)
	}
	if backend.setVolumes != 0 {
		t.Fatalf("backend setVolumes=%d want 0 when quantized bucket is unchanged", backend.setVolumes)
	}
}

func TestPlayWASMSoundEffect_QuantizedVolumeSkipsRedundantSetVolume(t *testing.T) {
	sample := media.PCMSample{
		SampleRate: 11025,
		Data:       []byte{1, 2, 3, 4},
	}
	key := wasmSampleKey{
		ptr:  uintptr(unsafe.Pointer(unsafe.SliceData(sample.Data))),
		len:  len(sample.Data),
		rate: sample.SampleRate,
	}
	backend := &fakeWASMBackend{}
	voice := &spatialVoice{
		player: backend,
		src:    &pcmBufferSource{buf: []byte{9, 8, 7, 6}},
		pinned: true,
		key:    key,
		bucket: wasmVolumeBuckets - 1,
	}
	p := &SpatialPlayer{
		ctx:    &audio.Context{},
		volume: 1,
		voices: []*spatialVoice{voice},
	}

	if ok := p.playWASMSoundEffect(sample, SpatialOrigin{}, 0, 0, false, "test"); !ok {
		t.Fatal("playWASMSoundEffect()=false want true")
	}
	if backend.setVolumes != 0 {
		t.Fatalf("backend setVolumes=%d want 0", backend.setVolumes)
	}
	if backend.plays != 1 {
		t.Fatalf("backend plays=%d want 1", backend.plays)
	}
}

func TestPlayWASMSoundEffect_AppliesExactGameVolumeAtMaxBucket(t *testing.T) {
	sample := media.PCMSample{
		SampleRate: 11025,
		Data:       []byte{1, 2, 3, 4},
	}
	key := wasmSampleKey{
		ptr:  uintptr(unsafe.Pointer(unsafe.SliceData(sample.Data))),
		len:  len(sample.Data),
		rate: sample.SampleRate,
	}
	backend := &fakeWASMBackend{}
	voice := &spatialVoice{
		player: backend,
		src:    &pcmBufferSource{buf: []byte{9, 8, 7, 6}},
		pinned: true,
		key:    key,
		bucket: wasmVolumeBucketUnset,
	}
	p := &SpatialPlayer{
		ctx:    &audio.Context{},
		volume: 0.4,
		voices: []*spatialVoice{voice},
	}

	if ok := p.playWASMSoundEffect(sample, SpatialOrigin{}, 0, 0, false, "test"); !ok {
		t.Fatal("playWASMSoundEffect()=false want true")
	}
	if backend.setVolumes != 1 {
		t.Fatalf("backend setVolumes=%d want 1", backend.setVolumes)
	}
	if backend.lastVolume != 0.4 {
		t.Fatalf("backend lastVolume=%f want 0.4", backend.lastVolume)
	}
}

func TestSpatialPlayerSetVolume_RescalesPinnedWASMVoice(t *testing.T) {
	backend := &fakeWASMBackend{}
	voice := &spatialVoice{
		player:         backend,
		pinned:         true,
		bucket:         1,
		wasmBucketGain: 0.25,
		wasmAppliedVol: 0.25,
	}
	p := &SpatialPlayer{
		volume: 1,
		voices: []*spatialVoice{voice},
	}

	p.SetVolume(0.4)

	if backend.setVolumes != 1 {
		t.Fatalf("backend setVolumes=%d want 1", backend.setVolumes)
	}
	if backend.lastVolume != 0.1 {
		t.Fatalf("backend lastVolume=%f want 0.1", backend.lastVolume)
	}
	if voice.wasmAppliedVol != 0.1 {
		t.Fatalf("voice.wasmAppliedVol=%f want 0.1", voice.wasmAppliedVol)
	}
}
