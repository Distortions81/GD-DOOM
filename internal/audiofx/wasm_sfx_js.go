//go:build js && wasm

package audiofx

import (
	"math"
	"unsafe"

	"gddoom/internal/media"
)

const (
	maxConcurrentWASMSounds = 6
	wasmVolumeBuckets       = 4
	wasmVolumeBucketUnset   = ^uint8(0)
)

func (p *SpatialPlayer) playWASMSoundEffect(sample media.PCMSample, origin SpatialOrigin, listenerX, listenerY int64, mapUsesFullClip bool, group string) bool {
	if p == nil || p.ctx == nil || p.sourcePort || sample.SampleRate <= 0 || len(sample.Data) == 0 {
		return false
	}
	attenuation, ok := wasmMonoAttenuation(origin, listenerX, listenerY, mapUsesFullClip)
	if !ok || attenuation <= 0 {
		return true
	}
	bucket, quantizedAttenuation := quantizeWASMVolume(attenuation)
	if quantizedAttenuation <= 0 {
		return true
	}
	if group != "" {
		p.stopGroup(group)
	}
	key := wasmSampleKey{
		ptr:  uintptr(unsafe.Pointer(unsafe.SliceData(sample.Data))),
		len:  len(sample.Data),
		rate: sample.SampleRate,
	}
	voice := p.findWASMCachedVoice(key)
	if voice == nil && p.wasmActiveVoiceCount() >= maxConcurrentWASMSounds {
		return true
	}
	if voice == nil {
		voice = p.acquireWASMCachedVoice(sample, key)
	}
	if voice == nil || voice.player == nil {
		return true
	}
	voice.group = group
	wasPlaying := voice.player.IsPlaying()
	finalGain := clampVolume(p.volume) * quantizedAttenuation
	if err := voice.player.Rewind(); err != nil {
		_ = voice.player.Close()
		return true
	}
	if voice.bucket != bucket || voice.wasmAppliedVol != finalGain {
		voice.player.SetVolume(finalGain)
		voice.bucket = bucket
		voice.wasmBucketGain = quantizedAttenuation
		voice.wasmAppliedVol = finalGain
	}
	if !wasPlaying {
		voice.player.Play()
	}
	return true
}

func wasmMonoAttenuation(origin SpatialOrigin, listenerX, listenerY int64, mapUsesFullClip bool) (float64, bool) {
	baseVol := doomSoundMaxVolume
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

func (p *SpatialPlayer) findWASMCachedVoice(key wasmSampleKey) *spatialVoice {
	for _, voice := range p.voices {
		if voice == nil || voice.player == nil {
			continue
		}
		if voice.pinned && voice.key == key {
			voice.stamp++
			return voice
		}
	}
	return nil
}

func (p *SpatialPlayer) acquireWASMCachedVoice(sample media.PCMSample, key wasmSampleKey) *spatialVoice {
	var candidate *spatialVoice
	var lru *spatialVoice
	for _, voice := range p.voices {
		if voice == nil || voice.player == nil || !voice.pinned {
			continue
		}
		if lru == nil || voice.stamp < lru.stamp {
			lru = voice
		}
	}
	size := wasmCachedPCMSize(sample, p.ctx.SampleRate())
	if len(p.voices) < maxSpatialVoices() {
		src := &pcmBufferSource{buf: make([]byte, size)}
		player, err := p.ctx.NewPlayer(src)
		if err != nil {
			return nil
		}
		candidate = &spatialVoice{player: player, src: src, pinned: true, bucket: wasmVolumeBucketUnset}
		p.voices = append(p.voices, candidate)
	} else {
		candidate = lru
	}
	if candidate == nil {
		return nil
	}
	candidate.player.Pause()
	_ = candidate.player.Rewind()
	candidate.src.buf = buildWASMCachedPCMInto(candidate.src.buf[:0], sample, p.ctx.SampleRate())
	candidate.src.Reset(candidate.src.buf)
	candidate.key = key
	candidate.stamp++
	candidate.group = ""
	candidate.pinned = true
	candidate.bucket = wasmVolumeBucketUnset
	candidate.wasmBucketGain = 0
	candidate.wasmAppliedVol = -1
	return candidate
}

func (p *SpatialPlayer) wasmActiveVoiceCount() int {
	count := 0
	for _, voice := range p.voices {
		if voice != nil && voice.pinned && voice.player != nil && voice.player.IsPlaying() {
			count++
		}
	}
	return count
}

func wasmCachedPCMSize(sample media.PCMSample, dstRate int) int {
	if sample.FaithfulPreparedRate == dstRate && len(sample.FaithfulPreparedMono) > 0 {
		return len(sample.FaithfulPreparedMono) * 4
	}
	return resampledMonoLen(len(sample.Data), sample.SampleRate, dstRate) * 4
}

func buildWASMCachedPCMInto(dst []byte, sample media.PCMSample, dstRate int) []byte {
	if sample.FaithfulPreparedRate == dstRate && len(sample.FaithfulPreparedMono) > 0 {
		return PCMMonoS16ToStereoS16LESpatialInto(dst, sample.FaithfulPreparedMono, 1, 1)
	}
	return PCMMonoU8ToStereoS16LEMonoResampledInto(dst, sample.Data, sample.SampleRate, dstRate)
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

func quantizeWASMVolume(gain float64) (uint8, float64) {
	if gain <= 0 {
		return 0, 0
	}
	if gain >= 1 {
		return wasmVolumeBuckets - 1, 1
	}
	step := 1.0 / float64(wasmVolumeBuckets-1)
	perceptual := math.Sqrt(gain)
	bucket := uint8(math.Round(perceptual / step))
	if bucket >= wasmVolumeBuckets {
		bucket = wasmVolumeBuckets - 1
	}
	if bucket == 0 {
		return 0, 0
	}
	q := float64(bucket) * step
	return bucket, q * q
}
