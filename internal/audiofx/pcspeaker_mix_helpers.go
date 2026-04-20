package audiofx

import (
	"math"

	"gddoom/internal/sound"
)

var pcSpeakerToneMixPattern = []int{0, 1, 0}

func pcSpeakerToneInterleaveHoldTicks(effectTone sound.PCSpeakerTone, musicTone sound.PCSpeakerTone, tickRate int) int {
	if tickRate <= 0 {
		tickRate = 560
	}
	holdSeconds := 1.0 / pcSpeakerToneInterleaveTargetHz
	lowestHz := math.MaxFloat64
	for _, tone := range [...]sound.PCSpeakerTone{effectTone, musicTone} {
		divisor := tone.ToneDivisor()
		if !tone.Active || divisor == 0 {
			continue
		}
		hz := float64(sound.PCSpeakerPITHz()) / float64(divisor)
		if hz > 0 && hz < lowestHz {
			lowestHz = hz
		}
	}
	if lowestHz != math.MaxFloat64 {
		minSeconds := 1.0 / lowestHz
		if minSeconds > holdSeconds {
			holdSeconds = minSeconds
		}
	}
	hold := int(math.Ceil(holdSeconds * float64(tickRate)))
	if hold < 1 {
		return 1
	}
	return hold
}

func toneAtTick(seq []sound.PCSpeakerTone, seqTickRate int, outTickRate int, tick int) (sound.PCSpeakerTone, bool) {
	if len(seq) == 0 || tick < 0 {
		return sound.PCSpeakerTone{}, false
	}
	seqTickRate = normalizePCSpeakerTickRate(seqTickRate)
	outTickRate = normalizePCSpeakerTickRate(outTickRate)
	idx := int((int64(tick) * int64(seqTickRate)) / int64(outTickRate))
	if idx < 0 || idx >= len(seq) {
		return sound.PCSpeakerTone{}, false
	}
	return seq[idx], true
}

func totalTicksAtRate(seqLen int, seqTickRate int, outTickRate int) int {
	if seqLen <= 0 {
		return 0
	}
	seqTickRate = normalizePCSpeakerTickRate(seqTickRate)
	outTickRate = normalizePCSpeakerTickRate(outTickRate)
	return int(math.Ceil(float64(seqLen) * float64(outTickRate) / float64(seqTickRate)))
}

func normalizePCSpeakerTickRate(rate int) int {
	if rate <= 0 {
		return 140
	}
	return rate
}
