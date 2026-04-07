package doomruntime

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"gddoom/internal/runtimecfg"
)

var frontendVoiceCodecChoices = [...]string{"ima", "g726", "pcm"}
var frontendVoiceG726BitsChoices = [...]int{2, 3, 4, 5}
var frontendVoiceSampleRateChoices = [...]int{16000, 24000, 32000, 48000}
var frontendVoiceGateThresholdChoices = [...]float64{
	0.10, 0.15, 0.20, 0.30, 0.40, 0.50, 0.65, 0.80,
	1.00, 1.25, 1.50, 1.75, 2.00, 2.50, 3.00, 3.50, 4.00,
}

func voiceCodecMenuLabel(codec string) string {
	switch strings.TrimSpace(strings.ToLower(codec)) {
	case "g726", "g726_32", "g72632":
		return "G.726"
	case "pcm", "pcm16", "pcm16_mono":
		return "LOSSLESS"
	default:
		return "IMA ADPCM"
	}
}

func normalizeVoiceCodecChoice(codec string) string {
	switch strings.TrimSpace(strings.ToLower(codec)) {
	case "g726", "g726_32", "g72632":
		return "g726"
	case "pcm", "pcm16", "pcm16_mono":
		return "pcm"
	default:
		return "ima"
	}
}

func voiceSampleRateMenuLabel(sampleRate int) string {
	if sampleRate <= 0 {
		sampleRate = frontendVoiceSampleRateChoices[len(frontendVoiceSampleRateChoices)-1]
	}
	if sampleRate%1000 == 0 {
		return strconv.Itoa(sampleRate/1000) + "k"
	}
	return strconv.Itoa(sampleRate)
}

func voiceG726BitsLabel(bits int) string {
	return fmt.Sprintf("%d BITS/SAMPLE", clampVoiceG726Bits(bits))
}

func voiceGateLabel(enabled bool) string {
	if enabled {
		return "ON"
	}
	return "OFF"
}

func voiceAGCLabel(enabled bool) string {
	return voiceGateLabel(enabled)
}

func voiceGateThresholdLabel(threshold float64) string {
	if threshold <= 0 {
		threshold = 1
	}
	return fmt.Sprintf("%0.2f", threshold)
}

func voiceCodecChoiceIndex(codec string) int {
	cur := normalizeVoiceCodecChoice(codec)
	for i, choice := range frontendVoiceCodecChoices {
		if choice == cur {
			return i
		}
	}
	return 0
}

func voiceSampleRateChoiceIndex(sampleRate int) int {
	for i, choice := range frontendVoiceSampleRateChoices {
		if choice == sampleRate {
			return i
		}
	}
	return len(frontendVoiceSampleRateChoices) - 1
}

func voiceGateThresholdChoiceIndex(threshold float64) int {
	if threshold <= 0 {
		threshold = 1
	}
	best := 0
	bestDist := math.Abs(frontendVoiceGateThresholdChoices[0] - threshold)
	for i := 1; i < len(frontendVoiceGateThresholdChoices); i++ {
		if dist := math.Abs(frontendVoiceGateThresholdChoices[i] - threshold); dist < bestDist {
			best = i
			bestDist = dist
		}
	}
	return best
}

func clampVoiceG726Bits(bits int) int {
	switch bits {
	case 2, 3, 4, 5:
		return bits
	default:
		return 4
	}
}

func voiceG726BitsChoiceIndex(bits int) int {
	bits = clampVoiceG726Bits(bits)
	for i, choice := range frontendVoiceG726BitsChoices {
		if choice == bits {
			return i
		}
	}
	return 2
}

func voiceMenuRowCount(codec string) int {
	if normalizeVoiceCodecChoice(codec) == "g726" {
		return frontendVoiceMenuRowCount
	}
	return frontendVoiceMenuRowCount - 1
}

func (sg *sessionGame) voiceCodecLabel() string {
	if sg == nil {
		return voiceCodecMenuLabel("")
	}
	return voiceCodecMenuLabel(sg.opts.VoiceCodec)
}

func (sg *sessionGame) voiceSampleRateLabel() string {
	if sg == nil {
		return voiceSampleRateMenuLabel(0)
	}
	return voiceSampleRateMenuLabel(sg.opts.VoiceSampleRate)
}

func (sg *sessionGame) voiceG726BitsLabel() string {
	if sg == nil {
		return voiceG726BitsLabel(4)
	}
	return voiceG726BitsLabel(sg.opts.VoiceG726BitsPerSample)
}

func (sg *sessionGame) voiceGateLabel() string {
	if sg == nil {
		return voiceGateLabel(true)
	}
	return voiceGateLabel(sg.opts.VoiceGateEnabled)
}

func (sg *sessionGame) voiceAGCLabel() string {
	if sg == nil {
		return voiceAGCLabel(true)
	}
	return voiceAGCLabel(sg.opts.VoiceAGCEnabled)
}

func (sg *sessionGame) voiceGateThresholdLabel() string {
	if sg == nil {
		return voiceGateThresholdLabel(1)
	}
	return voiceGateThresholdLabel(sg.opts.VoiceGateThreshold)
}

func (sg *sessionGame) voiceInputLevelLabel() string {
	return "INPUT LEVEL"
}

func (sg *sessionGame) voiceInputDeviceLabel() string {
	if sg == nil || strings.TrimSpace(sg.opts.VoiceInputDevice) == "" {
		return "USES SYSTEM DEFAULT INPUT"
	}
	return "INPUT: " + strings.ToUpper(strings.TrimSpace(sg.opts.VoiceInputDevice))
}

func (sg *sessionGame) voiceInputLevel() float64 {
	if sg == nil || sg.opts.VoiceInputLevel == nil {
		return 0
	}
	return voiceMeterLevel(sg.opts.VoiceInputLevel())
}

func voiceMeterLevel(level float64) float64 {
	if level < 0 {
		return 0
	}
	if level > 1 {
		level = 1
	}
	if level <= 0 {
		return 0
	}
	const minDB = -60.0
	db := 20 * math.Log10(level)
	if db <= minDB {
		return 0
	}
	if db >= 0 {
		return 1
	}
	return (db - minDB) / -minDB
}

func (sg *sessionGame) voiceInputGateActive() bool {
	if sg == nil || sg.opts.VoiceInputGateActive == nil {
		return false
	}
	return sg.opts.VoiceInputGateActive()
}

func (sg *sessionGame) applyVoiceSettings(codec string, sampleRate int) error {
	return sg.applyVoiceAdvancedSettings(codec, sg.opts.VoiceG726BitsPerSample, sampleRate, sg.opts.VoiceAGCEnabled, sg.opts.VoiceGateEnabled, sg.opts.VoiceGateThreshold)
}

func (sg *sessionGame) frontendChangeVoiceCodec(dir int) error {
	if sg == nil || dir == 0 {
		return nil
	}
	cur := voiceCodecChoiceIndex(sg.opts.VoiceCodec)
	n := len(frontendVoiceCodecChoices)
	next := (cur + dir + n) % n
	return sg.applyVoiceSettings(frontendVoiceCodecChoices[next], sg.opts.VoiceSampleRate)
}

func (sg *sessionGame) frontendChangeVoiceSampleRate(dir int) error {
	if sg == nil || dir == 0 {
		return nil
	}
	cur := voiceSampleRateChoiceIndex(sg.opts.VoiceSampleRate)
	n := len(frontendVoiceSampleRateChoices)
	next := (cur + dir + n) % n
	return sg.applyVoiceSettings(sg.opts.VoiceCodec, frontendVoiceSampleRateChoices[next])
}

func (sg *sessionGame) frontendChangeVoiceG726Bits(dir int) error {
	if sg == nil || dir == 0 {
		return nil
	}
	cur := voiceG726BitsChoiceIndex(sg.opts.VoiceG726BitsPerSample)
	n := len(frontendVoiceG726BitsChoices)
	next := (cur + dir + n) % n
	return sg.applyVoiceAdvancedSettings(sg.opts.VoiceCodec, frontendVoiceG726BitsChoices[next], sg.opts.VoiceSampleRate, sg.opts.VoiceAGCEnabled, sg.opts.VoiceGateEnabled, sg.opts.VoiceGateThreshold)
}

func (sg *sessionGame) frontendToggleVoiceGate() error {
	if sg == nil {
		return nil
	}
	return sg.applyVoiceAdvancedSettings(sg.opts.VoiceCodec, sg.opts.VoiceG726BitsPerSample, sg.opts.VoiceSampleRate, sg.opts.VoiceAGCEnabled, !sg.opts.VoiceGateEnabled, sg.opts.VoiceGateThreshold)
}

func (sg *sessionGame) frontendToggleVoiceAGC() error {
	if sg == nil {
		return nil
	}
	return sg.applyVoiceAdvancedSettings(sg.opts.VoiceCodec, sg.opts.VoiceG726BitsPerSample, sg.opts.VoiceSampleRate, !sg.opts.VoiceAGCEnabled, sg.opts.VoiceGateEnabled, sg.opts.VoiceGateThreshold)
}

func (sg *sessionGame) frontendChangeVoiceGateThreshold(dir int) error {
	if sg == nil || dir == 0 {
		return nil
	}
	cur := voiceGateThresholdChoiceIndex(sg.opts.VoiceGateThreshold)
	n := len(frontendVoiceGateThresholdChoices)
	next := (cur + dir + n) % n
	return sg.applyVoiceAdvancedSettings(sg.opts.VoiceCodec, sg.opts.VoiceG726BitsPerSample, sg.opts.VoiceSampleRate, sg.opts.VoiceAGCEnabled, sg.opts.VoiceGateEnabled, frontendVoiceGateThresholdChoices[next])
}

func (sg *sessionGame) applyVoiceAdvancedSettings(codec string, g726Bits int, sampleRate int, agcEnabled bool, gateEnabled bool, gateThreshold float64) error {
	if sg == nil {
		return nil
	}
	next := runtimecfg.VoiceSettings{
		Codec:         normalizeVoiceCodecChoice(codec),
		G726Bits:      clampVoiceG726Bits(g726Bits),
		SampleRate:    sampleRate,
		AGCEnabled:    agcEnabled,
		GateEnabled:   gateEnabled,
		GateThreshold: gateThreshold,
	}
	if next.SampleRate <= 0 {
		next.SampleRate = frontendVoiceSampleRateChoices[len(frontendVoiceSampleRateChoices)-1]
	}
	if next.GateThreshold <= 0 {
		next.GateThreshold = 1
	}
	if sg.opts.OnVoiceSettingsChanged != nil {
		if err := sg.opts.OnVoiceSettingsChanged(next); err != nil {
			return err
		}
	}
	sg.opts.VoiceCodec = next.Codec
	sg.opts.VoiceG726BitsPerSample = next.G726Bits
	sg.opts.VoiceSampleRate = next.SampleRate
	sg.opts.VoiceAGCEnabled = next.AGCEnabled
	sg.opts.VoiceGateEnabled = next.GateEnabled
	sg.opts.VoiceGateThreshold = next.GateThreshold
	if sg.g != nil {
		sg.g.opts.VoiceCodec = next.Codec
		sg.g.opts.VoiceG726BitsPerSample = next.G726Bits
		sg.g.opts.VoiceSampleRate = next.SampleRate
		sg.g.opts.VoiceAGCEnabled = next.AGCEnabled
		sg.g.opts.VoiceGateEnabled = next.GateEnabled
		sg.g.opts.VoiceGateThreshold = next.GateThreshold
	}
	return nil
}
