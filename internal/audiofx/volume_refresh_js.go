//go:build js && wasm

package audiofx

func (p *SpatialPlayer) refreshPlatformVolumes() {
	if p == nil {
		return
	}
	base := clampVolume(p.volume)
	for _, voice := range p.voices {
		if voice == nil || voice.player == nil || !voice.pinned || voice.bucket == wasmVolumeBucketUnset {
			continue
		}
		next := base * voice.wasmBucketGain
		if voice.wasmAppliedVol == next {
			continue
		}
		voice.player.SetVolume(next)
		voice.wasmAppliedVol = next
	}
}
