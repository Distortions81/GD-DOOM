//go:build js && wasm

package audiofx

import (
	"math"
	"unsafe"

	"gddoom/internal/media"
)

func (p *SpatialPlayer) playWASMSoundEffect(sample media.PCMSample, origin SpatialOrigin, listenerX, listenerY int64, mapUsesFullClip bool, group string) bool {
	if p == nil || p.ctx == nil || p.sourcePort || sample.SampleRate <= 0 || len(sample.Data) == 0 {
		return false
	}
	gain, ok := p.wasmMonoGain(origin, listenerX, listenerY, mapUsesFullClip)
	if !ok || gain <= 0 {
		return true
	}
	if group != "" {
		p.stopGroup(group)
	}
	voice := p.acquireWASMCachedVoice(sample)
	if voice == nil || voice.player == nil {
		return true
	}
	voice.group = group
	voice.player.Pause()
	if err := voice.player.Rewind(); err != nil {
		_ = voice.player.Close()
		return true
	}
	voice.player.SetVolume(gain)
	voice.player.Play()
	return true
}

func (p *SpatialPlayer) wasmMonoGain(origin SpatialOrigin, listenerX, listenerY int64, mapUsesFullClip bool) (float64, bool) {
	baseVol := int(math.Round(clampVolume(p.volume) * doomSoundMaxVolume))
	if baseVol <= 0 {
		return 0, false
	}
	if !origin.Positioned {
		return float64(baseVol) / doomSoundMaxVolume, true
	}
	adx := abs64(listenerX - origin.X)
	ady := abs64(listenerY - origin.Y)
	approxDist := adx + ady - min64(adx, ady)/2
	if !mapUsesFullClip && approxDist > doomSoundClippingDist {
		return 0, false
	}
	var vol int
	if approxDist < doomSoundCloseDist {
		vol = baseVol
	} else if mapUsesFullClip {
		if approxDist > doomSoundClippingDist {
			approxDist = doomSoundClippingDist
		}
		vol = 15 + ((baseVol-15)*int((doomSoundClippingDist-approxDist)/fracUnit))/int(doomSoundAttenuator)
	} else {
		vol = (baseVol * int((doomSoundClippingDist-approxDist)/fracUnit)) / int(doomSoundAttenuator)
	}
	if vol <= 0 {
		return 0, false
	}
	if vol > doomSoundMaxVolume {
		vol = doomSoundMaxVolume
	}
	return float64(vol) / doomSoundMaxVolume, true
}

func (p *SpatialPlayer) acquireWASMCachedVoice(sample media.PCMSample) *spatialVoice {
	key := wasmSampleKey{
		ptr:  uintptr(unsafe.Pointer(unsafe.SliceData(sample.Data))),
		len:  len(sample.Data),
		rate: sample.SampleRate,
	}
	var candidate *spatialVoice
	var lru *spatialVoice
	for _, voice := range p.voices {
		if voice == nil || voice.player == nil {
			continue
		}
		if voice.pinned && voice.key == key {
			voice.stamp++
			return voice
		}
		if voice.pinned && (lru == nil || voice.stamp < lru.stamp) {
			lru = voice
		}
	}
	if len(p.voices) < maxSpatialVoices() {
		size := resampledMonoLen(len(sample.Data), sample.SampleRate, p.ctx.SampleRate()) * 4
		src := &pcmBufferSource{buf: make([]byte, size)}
		player, err := p.ctx.NewPlayer(src)
		if err != nil {
			return nil
		}
		candidate = &spatialVoice{player: player, src: src, pinned: true}
		p.voices = append(p.voices, candidate)
	} else {
		candidate = lru
	}
	if candidate == nil {
		return nil
	}
	candidate.player.Pause()
	_ = candidate.player.Rewind()
	candidate.src.buf = PCMMonoU8ToStereoS16LEMonoResampledInto(candidate.src.buf[:0], sample.Data, sample.SampleRate, p.ctx.SampleRate())
	candidate.src.Reset(candidate.src.buf)
	candidate.key = key
	candidate.stamp++
	candidate.group = ""
	candidate.pinned = true
	return candidate
}

func PCMMonoU8ToStereoS16LEMonoResampledInto(dst []byte, src []byte, srcRate, dstRate int) []byte {
	if len(src) == 0 || srcRate <= 0 || dstRate <= 0 {
		return dst[:0]
	}
	outLen := resampledMonoLen(len(src), srcRate, dstRate)
	out := resizePCMBuffer(dst, outLen*4)
	if srcRate == dstRate {
		oi := 0
		for _, u := range src {
			base := (int16(u) - 128) << 8
			out[oi] = byte(base)
			out[oi+1] = byte(base >> 8)
			out[oi+2] = byte(base)
			out[oi+3] = byte(base >> 8)
			oi += 4
		}
		return out
	}
	step := (int64(srcRate) << 16) / int64(dstRate)
	pos := int64(0)
	last := len(src) - 1
	oi := 0
	for i := 0; i < outLen; i++ {
		idx := int(pos >> 16)
		if idx < 0 {
			idx = 0
		} else if idx > last {
			idx = last
		}
		base := (int16(src[idx]) - 128) << 8
		out[oi] = byte(base)
		out[oi+1] = byte(base >> 8)
		out[oi+2] = byte(base)
		out[oi+3] = byte(base >> 8)
		oi += 4
		pos += step
	}
	return out
}
