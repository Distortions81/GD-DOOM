//go:build !js || !wasm

package audiofx

import "gddoom/internal/media"

func (p *SpatialPlayer) playWASMSoundEffect(sample media.PCMSample, origin SpatialOrigin, listenerX, listenerY int64, mapUsesFullClip bool, group string) bool {
	return false
}

func (p *SpatialPlayer) prewarmWASMSample(sample media.PCMSample) bool { return false }

func (p *SpatialPlayer) prewarmWASMRemaining() int { return 0 }
