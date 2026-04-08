package doomruntime

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"gddoom/internal/runtimecfg"
	"gddoom/internal/voicecodec"
)

var frontendVoiceCodecChoices = [...]string{"silk", "g726"}
var frontendVoiceG726BitsChoices = [...]int{2, 3, 4, 5}
var frontendVoiceBitrateChoices = [...]int{8000, 12000, 16000, 20000, 25000, 30000, 40000, 50000, 64000, 80000}
var frontendVoiceSampleRateChoices = [...]int{16000, 24000, 32000, 48000}
var frontendVoicePresetChoices = [...]string{"high", "medium", "low"}

const (
	frontendVoiceMenuRowPreset     = 0
	frontendVoiceMenuRowG726Bits   = 1
	frontendVoiceMenuRowSampleRate = 2
	frontendVoiceMenuRowAGC        = 3
	frontendVoiceMenuRowGate       = 4
	frontendVoiceMenuRowGateThresh = 5
	frontendVoiceMenuRowCount      = 6
)

const (
	defaultFrontendVoiceG726Bits   = 3
	defaultFrontendVoiceBitrate    = voicecodec.SilkDefaultBitrate
	defaultFrontendVoiceSampleRate = 48000
)

var frontendVoiceGateThresholdChoices = [...]float64{
	0.10, 0.15, 0.20, 0.30, 0.40, 0.50, 0.65, 0.80,
	1.00, 1.25, 1.50, 1.75, 2.00, 2.50, 3.00, 3.50, 4.00,
}

func voiceCodecMenuLabel(codec string) string {
	switch strings.TrimSpace(strings.ToLower(codec)) {
	case "silk", "silk_v3", "silkv3":
		return "SILK"
	case "g726", "g726_32", "g72632":
		return "G.726"
	case "pcm", "pcm16", "pcm16_mono":
		return "WAV"
	default:
		return "SILK"
	}
}

func voicePresetLabel(preset string) string {
	switch strings.TrimSpace(strings.ToLower(preset)) {
	case "high":
		return "HIGH"
	case "medium", "med":
		return "MEDIUM"
	case "low":
		return "LOW"
	default:
		return "CUSTOM"
	}
}

func voicePresetChoiceIndex(preset string) int {
	switch strings.TrimSpace(strings.ToLower(preset)) {
	case "high":
		return 0
	case "medium", "med":
		return 1
	case "low":
		return 2
	default:
		return 0
	}
}

func detectVoicePreset(codec string, bitrate, sampleRate int) string {
	if normalizeVoiceCodecChoice(codec) != "silk" {
		return "custom"
	}
	switch {
	case bitrate == 64000 && sampleRate == 48000:
		return "high"
	case bitrate == 40000 && sampleRate == 32000:
		return "medium"
	case bitrate == 25000 && sampleRate == 24000:
		return "low"
	default:
		return "custom"
	}
}

func normalizeVoiceCodecChoice(codec string) string {
	switch strings.TrimSpace(strings.ToLower(codec)) {
	case "silk", "silk_v3", "silkv3":
		return "silk"
	case "g726", "g726_32", "g72632":
		return "g726"
	case "pcm", "pcm16", "pcm16_mono":
		return "pcm"
	default:
		return "silk"
	}
}

func voiceSampleRateMenuLabel(sampleRate int) string {
	if sampleRate <= 0 {
		sampleRate = defaultFrontendVoiceSampleRate
	}
	if sampleRate%1000 == 0 {
		return strconv.Itoa(sampleRate/1000) + " kHz"
	}
	return strconv.Itoa(sampleRate)
}

func voiceG726BitsLabel(bits int) string {
	return fmt.Sprintf("%d BITS/SAMPLE", clampVoiceG726Bits(bits))
}

func clampVoiceBitrate(bitrate int) int {
	if bitrate <= frontendVoiceBitrateChoices[0] {
		return frontendVoiceBitrateChoices[0]
	}
	if bitrate >= frontendVoiceBitrateChoices[len(frontendVoiceBitrateChoices)-1] {
		return frontendVoiceBitrateChoices[len(frontendVoiceBitrateChoices)-1]
	}
	for _, choice := range frontendVoiceBitrateChoices {
		if choice == bitrate {
			return bitrate
		}
	}
	return defaultFrontendVoiceBitrate
}

func voiceBitrateChoiceIndex(bitrate int) int {
	bitrate = clampVoiceBitrate(bitrate)
	for i, choice := range frontendVoiceBitrateChoices {
		if choice == bitrate {
			return i
		}
	}
	return 0
}

func voiceBitrateLabel(bitrate int) string {
	bitrate = clampVoiceBitrate(bitrate)
	if bitrate%1000 == 0 {
		return strconv.Itoa(bitrate/1000) + " KBPS"
	}
	return strconv.Itoa(bitrate) + " BPS"
}

func voiceCodecDetailMenuLabel(codec string) string {
	switch normalizeVoiceCodecChoice(codec) {
	case "silk":
		return "BITRATE"
	case "g726":
		return "BITS/SAMPLE"
	case "pcm":
		return "FORMAT"
	default:
		return "BITRATE"
	}
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
		return defaultFrontendVoiceG726Bits
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
	return frontendVoiceMenuRowCount
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
		return voiceG726BitsLabel(defaultFrontendVoiceG726Bits)
	}
	return voiceG726BitsLabel(sg.opts.VoiceG726BitsPerSample)
}

func (sg *sessionGame) voiceCodecDetailLabel() string {
	if sg == nil {
		return voiceBitrateLabel(defaultFrontendVoiceBitrate)
	}
	switch normalizeVoiceCodecChoice(sg.opts.VoiceCodec) {
	case "silk":
		return voiceBitrateLabel(sg.opts.VoiceBitrate)
	case "g726":
		return voiceG726BitsLabel(sg.opts.VoiceG726BitsPerSample)
	case "pcm":
		return "FIXED 16-BIT"
	default:
		return voiceBitrateLabel(defaultFrontendVoiceBitrate)
	}
}

func (sg *sessionGame) voicePresetLabel() string {
	if sg == nil {
		return voicePresetLabel("high")
	}
	return voicePresetLabel(detectVoicePreset(sg.opts.VoiceCodec, sg.opts.VoiceBitrate, sg.opts.VoiceSampleRate))
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
	return sg.applyVoiceAdvancedSettings(codec, sg.opts.VoiceG726BitsPerSample, sg.opts.VoiceBitrate, sampleRate, sg.opts.VoiceAGCEnabled, sg.opts.VoiceGateEnabled, sg.opts.VoiceGateThreshold)
}

func (sg *sessionGame) frontendChangeVoicePreset(dir int) error {
	if sg == nil || dir == 0 {
		return nil
	}
	curPreset := detectVoicePreset(sg.opts.VoiceCodec, sg.opts.VoiceBitrate, sg.opts.VoiceSampleRate)
	cur := voicePresetChoiceIndex(curPreset)
	if curPreset == "custom" {
		if dir < 0 {
			cur = len(frontendVoicePresetChoices) - 1
		} else {
			cur = 0
		}
	}
	n := len(frontendVoicePresetChoices)
	next := (cur + dir + n) % n
	switch frontendVoicePresetChoices[next] {
	case "high":
		return sg.applyVoiceAdvancedSettings("silk", sg.opts.VoiceG726BitsPerSample, 64000, 48000, sg.opts.VoiceAGCEnabled, sg.opts.VoiceGateEnabled, sg.opts.VoiceGateThreshold)
	case "medium":
		return sg.applyVoiceAdvancedSettings("silk", sg.opts.VoiceG726BitsPerSample, 40000, 32000, sg.opts.VoiceAGCEnabled, sg.opts.VoiceGateEnabled, sg.opts.VoiceGateThreshold)
	case "low":
		return sg.applyVoiceAdvancedSettings("silk", sg.opts.VoiceG726BitsPerSample, 25000, 24000, sg.opts.VoiceAGCEnabled, sg.opts.VoiceGateEnabled, sg.opts.VoiceGateThreshold)
	default:
		return nil
	}
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
	switch normalizeVoiceCodecChoice(sg.opts.VoiceCodec) {
	case "silk":
		cur := voiceBitrateChoiceIndex(sg.opts.VoiceBitrate)
		n := len(frontendVoiceBitrateChoices)
		next := (cur + dir + n) % n
		return sg.applyVoiceAdvancedSettings(sg.opts.VoiceCodec, sg.opts.VoiceG726BitsPerSample, frontendVoiceBitrateChoices[next], sg.opts.VoiceSampleRate, sg.opts.VoiceAGCEnabled, sg.opts.VoiceGateEnabled, sg.opts.VoiceGateThreshold)
	case "g726":
		cur := voiceG726BitsChoiceIndex(sg.opts.VoiceG726BitsPerSample)
		n := len(frontendVoiceG726BitsChoices)
		next := (cur + dir + n) % n
		return sg.applyVoiceAdvancedSettings(sg.opts.VoiceCodec, frontendVoiceG726BitsChoices[next], sg.opts.VoiceBitrate, sg.opts.VoiceSampleRate, sg.opts.VoiceAGCEnabled, sg.opts.VoiceGateEnabled, sg.opts.VoiceGateThreshold)
	default:
		return nil
	}
}

func (sg *sessionGame) frontendToggleVoiceGate() error {
	if sg == nil {
		return nil
	}
	return sg.applyVoiceAdvancedSettings(sg.opts.VoiceCodec, sg.opts.VoiceG726BitsPerSample, sg.opts.VoiceBitrate, sg.opts.VoiceSampleRate, sg.opts.VoiceAGCEnabled, !sg.opts.VoiceGateEnabled, sg.opts.VoiceGateThreshold)
}

func (sg *sessionGame) frontendToggleVoiceAGC() error {
	if sg == nil {
		return nil
	}
	return sg.applyVoiceAdvancedSettings(sg.opts.VoiceCodec, sg.opts.VoiceG726BitsPerSample, sg.opts.VoiceBitrate, sg.opts.VoiceSampleRate, !sg.opts.VoiceAGCEnabled, sg.opts.VoiceGateEnabled, sg.opts.VoiceGateThreshold)
}

func (sg *sessionGame) frontendChangeVoiceGateThreshold(dir int) error {
	if sg == nil || dir == 0 {
		return nil
	}
	cur := voiceGateThresholdChoiceIndex(sg.opts.VoiceGateThreshold)
	n := len(frontendVoiceGateThresholdChoices)
	next := (cur + dir + n) % n
	return sg.applyVoiceAdvancedSettings(sg.opts.VoiceCodec, sg.opts.VoiceG726BitsPerSample, sg.opts.VoiceBitrate, sg.opts.VoiceSampleRate, sg.opts.VoiceAGCEnabled, sg.opts.VoiceGateEnabled, frontendVoiceGateThresholdChoices[next])
}

func (sg *sessionGame) applyVoiceAdvancedSettings(codec string, g726Bits int, bitrate int, sampleRate int, agcEnabled bool, gateEnabled bool, gateThreshold float64) error {
	if sg == nil {
		return nil
	}
	next := runtimecfg.VoiceSettings{
		Codec:         normalizeVoiceCodecChoice(codec),
		G726Bits:      clampVoiceG726Bits(g726Bits),
		Bitrate:       clampVoiceBitrate(bitrate),
		SampleRate:    sampleRate,
		AGCEnabled:    agcEnabled,
		GateEnabled:   gateEnabled,
		GateThreshold: gateThreshold,
	}
	if next.SampleRate <= 0 {
		next.SampleRate = defaultFrontendVoiceSampleRate
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
	sg.opts.VoiceBitrate = next.Bitrate
	sg.opts.VoiceSampleRate = next.SampleRate
	sg.opts.VoiceAGCEnabled = next.AGCEnabled
	sg.opts.VoiceGateEnabled = next.GateEnabled
	sg.opts.VoiceGateThreshold = next.GateThreshold
	if sg.g != nil {
		sg.g.opts.VoiceCodec = next.Codec
		sg.g.opts.VoiceG726BitsPerSample = next.G726Bits
		sg.g.opts.VoiceBitrate = next.Bitrate
		sg.g.opts.VoiceSampleRate = next.SampleRate
		sg.g.opts.VoiceAGCEnabled = next.AGCEnabled
		sg.g.opts.VoiceGateEnabled = next.GateEnabled
		sg.g.opts.VoiceGateThreshold = next.GateThreshold
	}
	return nil
}
