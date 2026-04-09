package audiofx

import (
	"io"
	"math"
	"reflect"

	"gddoom/internal/doomrand"
	"gddoom/internal/media"
	"gddoom/internal/music"

	"github.com/hajimehoshi/ebiten/v2/audio"
)

const (
	doomSoundMaxVolume    = 127
	doomSoundClippingDist = int64(1200 * fracUnit)
	doomSoundCloseDist    = int64(200 * fracUnit)
	doomSoundStereoSwing  = int64(96 * fracUnit)
	doomSoundAttenuator   = (doomSoundClippingDist - doomSoundCloseDist) / fracUnit
	doomSoundNormalSep    = 128
	doomSoundSepRange     = 256
	sourcePortDelayMSUnit = 0.091
	sourcePortEarOffset   = (7 * fracUnit) / 2
	sourcePortRearGainMin = 0.7

	fracUnit = 1 << 16
)

type SpatialOrigin struct {
	X          int64
	Y          int64
	Positioned bool
}

type SpatialPlayer struct {
	ctx        *audio.Context
	volume     float64
	voices     []*spatialVoice
	sourcePort bool
}

type MenuPlayer struct {
	ctx     *audio.Context
	volume  float64
	move    media.PCMSample
	confirm media.PCMSample
	back    media.PCMSample
	quit1   []media.PCMSample
	quit2   []media.PCMSample
	voices  []*menuVoice
}

type pcmBufferSource struct {
	buf []byte
	pos int64
}

type spatialVoice struct {
	player *audio.Player
	src    *pcmBufferSource
	monoA  []int16
	monoB  []int16
	monoC  []int16
}

type menuVoice struct {
	player *audio.Player
	src    *pcmBufferSource
	monoA  []int16
	monoB  []int16
}

var (
	sharedAudioCtx  *audio.Context
	sharedAudioRate int
)

func EnsureSharedAudioContext() *audio.Context {
	rate := music.OutputSampleRate
	if rate <= 0 {
		return nil
	}
	return sharedOrNewAudioContext(rate)
}

func NewMenuPlayer(bank media.SoundBank, volume float64) *MenuPlayer {
	ctx := EnsureSharedAudioContext()
	if ctx == nil {
		return nil
	}
	return &MenuPlayer{
		ctx:     ctx,
		volume:  clampVolume(volume),
		move:    firstMenuSample(bank.MenuCursor, bank.SwitchOn),
		confirm: firstMenuSample(bank.ShootPistol, bank.SwitchOn),
		back:    firstMenuSample(bank.SwitchOff, bank.NoWay),
		quit1: []media.PCMSample{
			firstMenuSample(bank.PlayerDeath, bank.MonsterDeath),
			firstMenuSample(bank.MonsterPainDemon, bank.MonsterPainHumanoid),
			firstMenuSample(bank.MonsterPainHumanoid, bank.Pain),
			firstMenuSample(bank.ImpactRocket, bank.Oof),
			firstMenuSample(bank.PowerUp, bank.SwitchOn),
			firstMenuSample(bank.SeePosit1, bank.SeePosit2),
			firstMenuSample(bank.SeePosit3, bank.SeePosit1),
			firstMenuSample(bank.AttackSgt, bank.ShootShotgun),
		},
		quit2: []media.PCMSample{
			firstMenuSample(bank.ActiveVilAct, bank.SeeVileSit),
			firstMenuSample(bank.PowerUp, bank.ItemUp),
			firstMenuSample(bank.SeeCyberSit, bank.SeeBruiserSit),
			firstMenuSample(bank.ImpactRocket, bank.Oof),
			firstMenuSample(bank.AttackClaw, bank.AttackSkull),
			firstMenuSample(bank.DeathKnight, bank.DeathBaron),
			firstMenuSample(bank.ActiveBSPAct, bank.ActiveDMAct),
			firstMenuSample(bank.AttackSgt, bank.ShootShotgun),
		},
		voices: make([]*menuVoice, 0, maxMenuVoices()),
	}
}

func (p *MenuPlayer) PlayMove() {
	if p != nil {
		p.playSample(p.move)
	}
}

func (p *MenuPlayer) PlayConfirm() {
	if p != nil {
		p.playSample(p.confirm)
	}
}

func (p *MenuPlayer) PlayBack() {
	if p != nil {
		p.playSample(p.back)
	}
}

func (p *MenuPlayer) PlayQuit(commercial bool, seq int) {
	if p == nil {
		return
	}
	set := p.quit1
	if commercial {
		set = p.quit2
	}
	if len(set) == 0 {
		return
	}
	if seq < 0 {
		seq = 0
	}
	p.playSample(set[seq%len(set)])
}

func (p *MenuPlayer) SetVolume(v float64) {
	if p == nil {
		return
	}
	p.volume = clampVolume(v)
}

func (p *MenuPlayer) StopAll() {
	if p == nil {
		return
	}
	for _, voice := range p.voices {
		if voice == nil || voice.player == nil {
			continue
		}
		voice.player.Pause()
		_ = voice.player.Close()
	}
	p.voices = p.voices[:0]
}

func NewSpatialPlayer(sfxVolume float64, sourcePort bool) *SpatialPlayer {
	rate := music.OutputSampleRate
	if rate <= 0 {
		return nil
	}
	ctx := sharedOrNewAudioContext(rate)
	if ctx == nil {
		return nil
	}
	return &SpatialPlayer{
		ctx:        ctx,
		volume:     clampVolume(sfxVolume),
		voices:     make([]*spatialVoice, 0, maxSpatialVoices()),
		sourcePort: sourcePort,
	}
}

func (p *SpatialPlayer) SetVolume(v float64) {
	if p == nil {
		return
	}
	p.volume = clampVolume(v)
}

func (p *SpatialPlayer) PlaySample(sample media.PCMSample) {
	p.PlaySampleSpatialDelayed(sample, SpatialOrigin{}, 0, 0, 0, false, 0)
}

func (p *SpatialPlayer) PlaySampleSpatial(sample media.PCMSample, origin SpatialOrigin, listenerX, listenerY int64, listenerAngle uint32, mapUsesFullClip bool) {
	p.PlaySampleSpatialDelayed(sample, origin, listenerX, listenerY, listenerAngle, mapUsesFullClip, 0)
}

func (p *SpatialPlayer) CanPlaySpatial(origin SpatialOrigin, listenerX, listenerY int64, listenerAngle uint32, mapUsesFullClip bool) bool {
	if p == nil || p.ctx == nil || p.volume <= 0 {
		return false
	}
	leftGain, rightGain, _, _, ok := p.eventStereoMix(origin, listenerX, listenerY, listenerAngle, mapUsesFullClip)
	return ok && (leftGain > 0 || rightGain > 0)
}

func (p *SpatialPlayer) PlaySampleSpatialDelayed(sample media.PCMSample, origin SpatialOrigin, listenerX, listenerY int64, listenerAngle uint32, mapUsesFullClip bool, preDelaySamples float64) {
	if p == nil || p.ctx == nil || p.volume <= 0 {
		return
	}
	if sample.SampleRate <= 0 || len(sample.Data) == 0 {
		return
	}
	leftGain, rightGain, leftDelay, rightDelay, ok := p.eventStereoMix(origin, listenerX, listenerY, listenerAngle, mapUsesFullClip)
	if !ok || (leftGain <= 0 && rightGain <= 0) {
		return
	}
	var mono []int16
	var voice *spatialVoice
	if p.sourcePort && sample.PreparedRate == p.ctx.SampleRate() && len(sample.PreparedMono) > 0 {
		mono = sample.PreparedMono
	} else if !p.sourcePort && sample.FaithfulPreparedRate == p.ctx.SampleRate() && len(sample.FaithfulPreparedMono) > 0 {
		mono = sample.FaithfulPreparedMono
	} else {
		monoLen := len(sample.Data)
		if sample.SampleRate != p.ctx.SampleRate() {
			monoLen = resampledMonoLen(len(sample.Data), sample.SampleRate, p.ctx.SampleRate())
		}
		voice = p.acquireVoice(monoLen * 4)
		if voice == nil {
			return
		}
		voice.monoA = PCMMonoU8ToMonoS16Into(voice.monoA[:0], sample.Data)
		mono = voice.monoA
		if sample.SampleRate != p.ctx.SampleRate() {
			voice.monoB = resampleMonoS16LinearQuantizedInto(voice.monoB[:0], mono, sample.SampleRate, p.ctx.SampleRate())
			mono = voice.monoB
		}
	}
	if len(mono) == 0 {
		return
	}
	if preDelaySamples < 0 {
		preDelaySamples = 0
	}
	leftDelay += preDelaySamples
	rightDelay += preDelaySamples
	if voice == nil {
		voice = p.acquireVoice((len(mono) + int(math.Ceil(max(leftDelay, rightDelay))) + 1) * 4)
		if voice == nil {
			return
		}
		if leftDelay > 0 || rightDelay > 0 {
			pcm := PCMMonoS16ToStereoS16LESpatialDelayedInto(voice.src.buf[:0], mono, leftGain, rightGain, leftDelay, rightDelay)
			voice.src.Reset(pcm)
			if err := voice.player.Rewind(); err != nil {
				_ = voice.player.Close()
				return
			}
			voice.player.SetVolume(1)
			voice.player.Play()
			return
		}
	}
	if p.sourcePort && origin.Positioned {
		far, behind := sourcePortFilterParams(origin, listenerX, listenerY, listenerAngle)
		mono = applySourcePortLowPassInto(voice.monoC[:0], mono, p.ctx.SampleRate(), far, behind)
	}
	pcm := PCMMonoS16ToStereoS16LESpatialInto(voice.src.buf[:0], mono, leftGain, rightGain)
	voice.src.Reset(pcm)
	if err := voice.player.Rewind(); err != nil {
		_ = voice.player.Close()
		return
	}
	voice.player.SetVolume(1)
	voice.player.Play()
}

func (p *SpatialPlayer) eventStereoMix(origin SpatialOrigin, listenerX, listenerY int64, listenerAngle uint32, mapUsesFullClip bool) (float64, float64, float64, float64, bool) {
	baseGain := clampVolume(p.volume)
	if baseGain <= 0 {
		return 0, 0, 0, 0, false
	}
	if !origin.Positioned {
		return baseGain, baseGain, 0, 0, true
	}
	if p == nil {
		return 0, 0, 0, 0, false
	}
	if !p.sourcePort {
		leftGain, rightGain := p.eventStereoGains(origin, listenerX, listenerY, listenerAngle, mapUsesFullClip)
		return leftGain, rightGain, 0, 0, leftGain > 0 || rightGain > 0
	}
	leftDist, rightDist, centerDist := sourcePortEarDistances(origin, listenerX, listenerY, listenerAngle)
	if !mapUsesFullClip && centerDist > float64(doomSoundClippingDist)/fracUnit {
		return 0, 0, 0, 0, false
	}
	centerGain := baseGain * sourcePortDistanceGain(centerDist)
	centerGain *= sourcePortFacingGain(origin, listenerX, listenerY, listenerAngle)
	if centerGain <= 0 {
		return 0, 0, 0, 0, false
	}
	leftWeight := sourcePortEarWeight(leftDist)
	rightWeight := sourcePortEarWeight(rightDist)
	norm := math.Sqrt((leftWeight*leftWeight + rightWeight*rightWeight) * 0.5)
	if norm <= 0 {
		norm = 1
	}
	leftGain := centerGain * (leftWeight / norm)
	rightGain := centerGain * (rightWeight / norm)
	sampleRate := 0
	if p.ctx != nil {
		sampleRate = p.ctx.SampleRate()
	}
	leftDelay, rightDelay := sourcePortSoundDelaySamples(sampleRate, origin, listenerX, listenerY, listenerAngle)
	return leftGain, rightGain, leftDelay, rightDelay, true
}

func sourcePortEarDistances(origin SpatialOrigin, listenerX, listenerY int64, listenerAngle uint32) (float64, float64, float64) {
	if !origin.Positioned {
		return 0, 0, 0
	}
	ang := angleToRadians(listenerAngle)
	lx := float64(listenerX) - float64(sourcePortEarOffset)*math.Sin(ang)
	ly := float64(listenerY) + float64(sourcePortEarOffset)*math.Cos(ang)
	rx := float64(listenerX) + float64(sourcePortEarOffset)*math.Sin(ang)
	ry := float64(listenerY) - float64(sourcePortEarOffset)*math.Cos(ang)
	cx := float64(listenerX)
	cy := float64(listenerY)
	return sourcePortDistanceUnits(origin.X, origin.Y, lx, ly),
		sourcePortDistanceUnits(origin.X, origin.Y, rx, ry),
		sourcePortDistanceUnits(origin.X, origin.Y, cx, cy)
}

func sourcePortSoundDelaySamples(sampleRate int, origin SpatialOrigin, listenerX, listenerY int64, listenerAngle uint32) (float64, float64) {
	if sampleRate <= 0 || !origin.Positioned {
		return 0, 0
	}
	ang := angleToRadians(listenerAngle)
	lx := float64(listenerX) - float64(sourcePortEarOffset)*math.Sin(ang)
	ly := float64(listenerY) + float64(sourcePortEarOffset)*math.Cos(ang)
	rx := float64(listenerX) + float64(sourcePortEarOffset)*math.Sin(ang)
	ry := float64(listenerY) - float64(sourcePortEarOffset)*math.Cos(ang)
	return sourcePortDelaySamplesForPoint(sampleRate, origin.X, origin.Y, lx, ly),
		sourcePortDelaySamplesForPoint(sampleRate, origin.X, origin.Y, rx, ry)
}

func sourcePortDelaySamplesForPoint(sampleRate int, sourceX, sourceY int64, listenerX, listenerY float64) float64 {
	if sampleRate <= 0 {
		return 0
	}
	mapUnits := sourcePortDistanceUnits(sourceX, sourceY, listenerX, listenerY)
	if mapUnits <= 0 {
		return 0
	}
	return (mapUnits * sourcePortDelayMSUnit * float64(sampleRate)) / 1000.0
}

func sourcePortDistanceUnits(sourceX, sourceY int64, listenerX, listenerY float64) float64 {
	dx := (float64(sourceX) - listenerX) / fracUnit
	dy := (float64(sourceY) - listenerY) / fracUnit
	return math.Hypot(dx, dy)
}

func sourcePortDistanceGain(mapUnits float64) float64 {
	closeUnits := float64(doomSoundCloseDist) / fracUnit
	if mapUnits <= closeUnits {
		return 1
	}
	return math.Sqrt(closeUnits / mapUnits)
}

func sourcePortFacingGain(origin SpatialOrigin, listenerX, listenerY int64, listenerAngle uint32) float64 {
	if !origin.Positioned {
		return 1
	}
	dx := float64(origin.X-listenerX) / fracUnit
	dy := float64(origin.Y-listenerY) / fracUnit
	dist := math.Hypot(dx, dy)
	if dist <= 0 {
		return 1
	}
	ang := angleToRadians(listenerAngle)
	forwardX := math.Cos(ang)
	forwardY := math.Sin(ang)
	dot := (dx*forwardX + dy*forwardY) / dist
	if dot < -1 {
		dot = -1
	} else if dot > 1 {
		dot = 1
	}
	return sourcePortRearGainMin + (1-sourcePortRearGainMin)*((dot+1)*0.5)
}

func sourcePortEarWeight(distUnits float64) float64 {
	if distUnits < 1 {
		distUnits = 1
	}
	return 1 / distUnits
}

func sourcePortFilterParams(origin SpatialOrigin, listenerX, listenerY int64, listenerAngle uint32) (float64, float64) {
	if !origin.Positioned {
		return 0, 0
	}
	_, _, centerDist := sourcePortEarDistances(origin, listenerX, listenerY, listenerAngle)
	closeUnits := float64(doomSoundCloseDist) / fracUnit
	clipUnits := float64(doomSoundClippingDist) / fracUnit
	far := 0.0
	if clipUnits > closeUnits && centerDist > closeUnits {
		far = (centerDist - closeUnits) / (clipUnits - closeUnits)
		if far < 0 {
			far = 0
		}
		if far > 1 {
			far = 1
		}
	}
	behind := 1 - sourcePortFacingGain(origin, listenerX, listenerY, listenerAngle)
	if denom := 1 - sourcePortRearGainMin; denom > 0 {
		behind /= denom
	}
	if behind < 0 {
		behind = 0
	}
	if behind > 1 {
		behind = 1
	}
	return far, behind
}

func applySourcePortLowPassInto(dst, src []int16, sampleRate int, far, behind float64) []int16 {
	if len(src) == 0 {
		return nil
	}
	if sampleRate <= 0 {
		return src
	}
	strength := far
	if behind > strength {
		strength = behind
	}
	if strength <= 0 {
		return src
	}
	cutoff := 7000.0 - 5000.0*strength
	if cutoff < 2000 {
		cutoff = 2000
	}
	alpha := math.Exp(-2 * math.Pi * cutoff / float64(sampleRate))
	out := resizePCMInt16Buffer(dst, len(src))
	y := float64(src[0])
	out[0] = src[0]
	for i := 1; i < len(src); i++ {
		x := float64(src[i])
		y = (1-alpha)*x + alpha*y
		out[i] = int16(clampFloat(math.Round(y), -32768, 32767))
	}
	return out
}

func PCMMonoS16ToStereoS16LESpatialDelayedInto(dst []byte, src []int16, leftGain, rightGain float64, leftDelay, rightDelay float64) []byte {
	if len(src) == 0 {
		return dst[:0]
	}
	if leftDelay < 0 {
		leftDelay = 0
	}
	if rightDelay < 0 {
		rightDelay = 0
	}
	samples := len(src) + int(math.Ceil(max(leftDelay, rightDelay))) + 1
	out := resizePCMBuffer(dst, samples*4)
	clear(out)
	for i, base := range src {
		mixDelayedSample(out, i, leftDelay, float64(base)*leftGain, 0)
		mixDelayedSample(out, i, rightDelay, float64(base)*rightGain, 2)
	}
	return out
}

func mixDelayedSample(dst []byte, baseIndex int, delay, sample float64, byteOffset int) {
	pos := float64(baseIndex) + delay
	i0 := int(math.Floor(pos))
	frac := pos - float64(i0)
	if i0 >= 0 {
		addStereoChannelSample(dst, i0, byteOffset, sample*(1-frac))
	}
	addStereoChannelSample(dst, i0+1, byteOffset, sample*frac)
}

func addStereoChannelSample(dst []byte, sampleIndex, byteOffset int, value float64) {
	if sampleIndex < 0 {
		return
	}
	bi := sampleIndex*4 + byteOffset
	if bi < 0 || bi+1 >= len(dst) {
		return
	}
	cur := int16(dst[bi]) | int16(dst[bi+1])<<8
	out := int16(clampFloat(float64(cur)+value, -32768, 32767))
	dst[bi] = byte(out)
	dst[bi+1] = byte(out >> 8)
}

func (p *SpatialPlayer) Tick() {
	if p == nil || len(p.voices) == 0 {
		return
	}
	for _, voice := range p.voices {
		if voice == nil || voice.player == nil || voice.player.IsPlaying() {
			continue
		}
		voice.src.Reset(voice.src.buf[:0])
	}
}

func (p *SpatialPlayer) StopAll() {
	if p == nil || len(p.voices) == 0 {
		return
	}
	for _, voice := range p.voices {
		if voice == nil || voice.player == nil {
			continue
		}
		voice.player.Pause()
		_ = voice.player.Close()
	}
	p.voices = p.voices[:0]
}

func PrepareSoundBankForSourcePort(bank media.SoundBank, dstRate int) media.SoundBank {
	if dstRate <= 0 {
		return bank
	}
	var scratch []int16
	rv := reflect.ValueOf(&bank).Elem()
	for i := 0; i < rv.NumField(); i++ {
		field := rv.Field(i)
		sample, ok := field.Addr().Interface().(*media.PCMSample)
		if !ok || sample == nil {
			continue
		}
		scratch = prepareSampleForSourcePort(sample, dstRate, scratch[:0])
	}
	return bank
}

func PrepareSoundBankForFaithful(bank media.SoundBank, dstRate int) media.SoundBank {
	if dstRate <= 0 {
		return bank
	}
	var scratch []int16
	rv := reflect.ValueOf(&bank).Elem()
	for i := 0; i < rv.NumField(); i++ {
		field := rv.Field(i)
		sample, ok := field.Addr().Interface().(*media.PCMSample)
		if !ok || sample == nil {
			continue
		}
		scratch = prepareSampleForFaithful(sample, dstRate, scratch[:0])
	}
	return bank
}

func ApplySourcePortPresenceBoost(src []int16) []int16 {
	if len(src) == 0 {
		return nil
	}
	out := make([]int16, len(src))
	out[0] = src[0]
	prevHP1 := 0.0
	prevHP2 := 0.0
	prevX := float64(src[0])
	const hpAlpha1 = 0.54
	const hpAlpha2 = 0.36
	const boost1 = 1.15
	const boost2 = 0.65
	for i := 1; i < len(src); i++ {
		x := float64(src[i])
		hp1 := hpAlpha1 * (prevHP1 + x - prevX)
		hp2 := hpAlpha2 * (prevHP2 + hp1 - prevHP1)
		y := x + boost1*hp1 + boost2*hp2
		out[i] = int16(clampFloat(math.Round(y), -32768, 32767))
		prevHP1 = hp1
		prevHP2 = hp2
		prevX = x
	}
	return out
}

func PCMMonoU8ToMonoS16(src []byte) []int16 {
	return PCMMonoU8ToMonoS16Into(nil, src)
}

func PCMMonoU8ToMonoS16Into(dst []int16, src []byte) []int16 {
	if len(src) == 0 {
		return nil
	}
	out := resizePCMInt16Buffer(dst, len(src))
	for i, u := range src {
		out[i] = (int16(u) - 128) << 8
	}
	return out
}

func PCMMonoS16ToStereoS16LESpatial(src []int16, leftGain, rightGain float64) []byte {
	return PCMMonoS16ToStereoS16LESpatialInto(nil, src, leftGain, rightGain)
}

func PCMMonoU8ToStereoS16LESpatialInto(dst []byte, src []byte, leftGain, rightGain float64) []byte {
	out := resizePCMBuffer(dst, len(src)*4)
	oi := 0
	for _, u := range src {
		base := (int16(u) - 128) << 8
		left := int16(clampFloat(float64(base)*leftGain, -32768, 32767))
		right := int16(clampFloat(float64(base)*rightGain, -32768, 32767))
		out[oi] = byte(left)
		out[oi+1] = byte(left >> 8)
		out[oi+2] = byte(right)
		out[oi+3] = byte(right >> 8)
		oi += 4
	}
	return out
}

func PCMMonoS16ToStereoS16LESpatialInto(dst []byte, src []int16, leftGain, rightGain float64) []byte {
	out := resizePCMBuffer(dst, len(src)*4)
	oi := 0
	for _, base := range src {
		left := int16(clampFloat(float64(base)*leftGain, -32768, 32767))
		right := int16(clampFloat(float64(base)*rightGain, -32768, 32767))
		out[oi] = byte(left)
		out[oi+1] = byte(left >> 8)
		out[oi+2] = byte(right)
		out[oi+3] = byte(right >> 8)
		oi += 4
	}
	return out
}

func PCMMonoU8ToStereoS16LESpatial(src []byte, leftGain, rightGain float64) []byte {
	return PCMMonoU8ToStereoS16LESpatialInto(nil, src, leftGain, rightGain)
}

func PCMMonoU8ToStereoS16LEResampled(src []byte, srcRate, dstRate int) []byte {
	return PCMMonoU8ToStereoS16LESpatialResampled(src, srcRate, dstRate, 1, 1)
}

func PCMMonoU8ToStereoS16LESpatialResampled(src []byte, srcRate, dstRate int, leftGain, rightGain float64) []byte {
	if len(src) == 0 || srcRate <= 0 || dstRate <= 0 {
		return nil
	}
	if srcRate == dstRate {
		return PCMMonoU8ToStereoS16LESpatial(src, leftGain, rightGain)
	}
	return PCMMonoS16ToStereoS16LESpatial(resampleMonoS16Linear(PCMMonoU8ToMonoS16(src), srcRate, dstRate), leftGain, rightGain)
}

func DoomAdjustSoundParams(listenerX, listenerY int64, listenerAngle uint32, sourceX, sourceY int64, baseVol int, mapUsesFullClip bool) (vol, sep int, ok bool) {
	adx := abs64(listenerX - sourceX)
	ady := abs64(listenerY - sourceY)
	approxDist := adx + ady - min64(adx, ady)/2
	if !mapUsesFullClip && approxDist > doomSoundClippingDist {
		return 0, doomSoundNormalSep, false
	}
	angle := math.Atan2(float64(sourceY-listenerY), float64(sourceX-listenerX)) - angleToRadians(listenerAngle)
	sep = doomSoundNormalSep - int(math.Round((float64(doomSoundStereoSwing)/float64(fracUnit))*math.Sin(angle)))
	if sep < 0 {
		sep = 0
	}
	if sep > 255 {
		sep = 255
	}
	if approxDist < doomSoundCloseDist {
		vol = baseVol
		return vol, sep, vol > 0
	}
	if mapUsesFullClip {
		if approxDist > doomSoundClippingDist {
			approxDist = doomSoundClippingDist
		}
		vol = 15 + ((baseVol-15)*int((doomSoundClippingDist-approxDist)/fracUnit))/int(doomSoundAttenuator)
	} else {
		vol = (baseVol * int((doomSoundClippingDist-approxDist)/fracUnit)) / int(doomSoundAttenuator)
	}
	return vol, sep, vol > 0
}

func DoomSeparationVolumes(vol, sep int) (left, right int) {
	sep++
	left = vol - (vol*sep*sep)/(doomSoundSepRange*doomSoundSepRange)
	sep -= 257
	right = vol - (vol*sep*sep)/(doomSoundSepRange*doomSoundSepRange)
	if left < 0 {
		left = 0
	}
	if left > doomSoundMaxVolume {
		left = doomSoundMaxVolume
	}
	if right < 0 {
		right = 0
	}
	if right > doomSoundMaxVolume {
		right = doomSoundMaxVolume
	}
	return left, right
}

func DoomSoundMaxVolume() int      { return doomSoundMaxVolume }
func DoomSoundClippingDist() int64 { return doomSoundClippingDist }
func DoomSoundNormalSep() int      { return doomSoundNormalSep }

func SoundVariantIndex(n int) int {
	if n <= 1 {
		return 0
	}
	return doomrand.MRandom() % n
}

func PickFirstAvailable(start int, samples ...media.PCMSample) (media.PCMSample, bool) {
	if len(samples) == 0 {
		return media.PCMSample{}, false
	}
	if start < 0 {
		start = 0
	}
	start %= len(samples)
	for i := 0; i < len(samples); i++ {
		s := samples[(start+i)%len(samples)]
		if len(s.Data) > 0 && s.SampleRate > 0 {
			return s, true
		}
	}
	return media.PCMSample{}, false
}

func (p *SpatialPlayer) eventStereoGains(origin SpatialOrigin, listenerX, listenerY int64, listenerAngle uint32, mapUsesFullClip bool) (float64, float64) {
	baseVol := int(math.Round(p.volume * doomSoundMaxVolume))
	if baseVol < 0 {
		baseVol = 0
	}
	if baseVol > doomSoundMaxVolume {
		baseVol = doomSoundMaxVolume
	}
	if !origin.Positioned {
		gain := float64(baseVol) / doomSoundMaxVolume
		return gain, gain
	}
	vol, sep, ok := DoomAdjustSoundParams(listenerX, listenerY, listenerAngle, origin.X, origin.Y, baseVol, mapUsesFullClip)
	if !ok || vol <= 0 {
		return 0, 0
	}
	left, right := DoomSeparationVolumes(vol, sep)
	return float64(left) / doomSoundMaxVolume, float64(right) / doomSoundMaxVolume
}

func firstMenuSample(samples ...media.PCMSample) media.PCMSample {
	for _, sample := range samples {
		if sample.SampleRate > 0 && len(sample.Data) > 0 {
			return sample
		}
	}
	return media.PCMSample{}
}

func (p *MenuPlayer) playSample(sample media.PCMSample) {
	if p == nil || p.ctx == nil || p.volume <= 0 || sample.SampleRate <= 0 || len(sample.Data) == 0 {
		return
	}
	var mono []int16
	var voice *menuVoice
	if sample.FaithfulPreparedRate == p.ctx.SampleRate() && len(sample.FaithfulPreparedMono) > 0 {
		mono = sample.FaithfulPreparedMono
	} else {
		monoLen := len(sample.Data)
		if sample.SampleRate != p.ctx.SampleRate() {
			monoLen = resampledMonoLen(len(sample.Data), sample.SampleRate, p.ctx.SampleRate())
		}
		voice = p.acquireVoice(monoLen * 4)
		if voice == nil {
			return
		}
		voice.monoA = PCMMonoU8ToMonoS16Into(voice.monoA[:0], sample.Data)
		mono = voice.monoA
		if sample.SampleRate != p.ctx.SampleRate() {
			voice.monoB = resampleMonoS16LinearQuantizedInto(voice.monoB[:0], mono, sample.SampleRate, p.ctx.SampleRate())
			mono = voice.monoB
		}
	}
	if len(mono) == 0 {
		return
	}
	if voice == nil {
		voice = p.acquireVoice(len(mono) * 4)
		if voice == nil {
			return
		}
	}
	pcm := PCMMonoS16ToStereoS16LESpatialInto(voice.src.buf[:0], mono, 1, 1)
	voice.src.Reset(pcm)
	if err := voice.player.Rewind(); err != nil {
		_ = voice.player.Close()
		return
	}
	voice.player.SetVolume(p.volume)
	voice.player.Play()
}

func prepareSampleForSourcePort(sample *media.PCMSample, dstRate int, scratch []int16) []int16 {
	if sample == nil || len(sample.Data) == 0 || sample.SampleRate <= 0 || dstRate <= 0 {
		return scratch
	}
	mono := PCMMonoU8ToMonoS16Into(scratch, sample.Data)
	if sample.SampleRate != dstRate {
		sample.PreparedMono = resampleMonoS16PolyphaseInto(sample.PreparedMono[:0], mono, sample.SampleRate, dstRate)
	} else {
		sample.PreparedMono = resizePCMInt16Buffer(sample.PreparedMono[:0], len(mono))
		copy(sample.PreparedMono, mono)
	}
	sample.PreparedRate = dstRate
	return mono
}

func prepareSampleForFaithful(sample *media.PCMSample, dstRate int, scratch []int16) []int16 {
	if sample == nil || len(sample.Data) == 0 || sample.SampleRate <= 0 || dstRate <= 0 {
		return scratch
	}
	mono := PCMMonoU8ToMonoS16Into(scratch, sample.Data)
	if sample.SampleRate != dstRate {
		sample.FaithfulPreparedMono = resampleMonoS16LinearQuantizedInto(sample.FaithfulPreparedMono[:0], mono, sample.SampleRate, dstRate)
	} else {
		sample.FaithfulPreparedMono = resizePCMInt16Buffer(sample.FaithfulPreparedMono[:0], len(mono))
		copy(sample.FaithfulPreparedMono, mono)
	}
	sample.FaithfulPreparedRate = dstRate
	return mono
}

func sharedOrNewAudioContext(rate int) *audio.Context {
	if sharedAudioCtx != nil {
		if sharedAudioRate == rate {
			return sharedAudioCtx
		}
		return nil
	}
	sharedAudioCtx = audio.NewContext(rate)
	sharedAudioRate = rate
	return sharedAudioCtx
}

func (p *SpatialPlayer) acquireVoice(size int) *spatialVoice {
	for _, voice := range p.voices {
		if voice != nil && voice.player != nil && !voice.player.IsPlaying() {
			voice.src.buf = resizePCMBuffer(voice.src.buf[:0], size)
			return voice
		}
	}
	if len(p.voices) >= maxSpatialVoices() {
		return nil
	}
	src := &pcmBufferSource{buf: make([]byte, size)}
	player, err := p.ctx.NewPlayer(src)
	if err != nil {
		return nil
	}
	voice := &spatialVoice{player: player, src: src}
	p.voices = append(p.voices, voice)
	return voice
}

func (p *MenuPlayer) acquireVoice(size int) *menuVoice {
	for _, voice := range p.voices {
		if voice != nil && voice.player != nil && !voice.player.IsPlaying() {
			voice.src.buf = resizePCMBuffer(voice.src.buf[:0], size)
			return voice
		}
	}
	if len(p.voices) >= maxMenuVoices() {
		return nil
	}
	src := &pcmBufferSource{buf: make([]byte, size)}
	player, err := p.ctx.NewPlayer(src)
	if err != nil {
		return nil
	}
	voice := &menuVoice{player: player, src: src}
	p.voices = append(p.voices, voice)
	return voice
}

func resizePCMBuffer(buf []byte, size int) []byte {
	if cap(buf) >= size {
		return buf[:size]
	}
	return make([]byte, size)
}

func resizePCMInt16Buffer(buf []int16, size int) []int16 {
	if cap(buf) >= size {
		return buf[:size]
	}
	return make([]int16, size)
}

func (s *pcmBufferSource) Reset(buf []byte) {
	if s == nil {
		return
	}
	s.buf = buf
	s.pos = 0
}

func (s *pcmBufferSource) Read(p []byte) (int, error) {
	if s == nil || s.pos >= int64(len(s.buf)) {
		return 0, io.EOF
	}
	n := copy(p, s.buf[s.pos:])
	s.pos += int64(n)
	if n == 0 {
		return 0, io.EOF
	}
	return n, nil
}

func (s *pcmBufferSource) Seek(offset int64, whence int) (int64, error) {
	if s == nil {
		return 0, io.ErrClosedPipe
	}
	var next int64
	switch whence {
	case io.SeekStart:
		next = offset
	case io.SeekCurrent:
		next = s.pos + offset
	case io.SeekEnd:
		next = int64(len(s.buf)) + offset
	default:
		return 0, io.ErrUnexpectedEOF
	}
	if next < 0 {
		return 0, io.ErrUnexpectedEOF
	}
	if next > int64(len(s.buf)) {
		next = int64(len(s.buf))
	}
	s.pos = next
	return s.pos, nil
}

func resampleMonoS16Polyphase(src []int16, srcRate, dstRate int) []int16 {
	return resampleMonoS16PolyphaseInto(nil, src, srcRate, dstRate)
}

func resampleMonoS16PolyphaseInto(dst []int16, src []int16, srcRate, dstRate int) []int16 {
	if len(src) == 0 || srcRate <= 0 || dstRate <= 0 {
		return nil
	}
	if srcRate == dstRate {
		out := resizePCMInt16Buffer(dst, len(src))
		copy(out, src)
		return out
	}
	const phases = 128
	const taps = 24
	center := float64(taps-1) * 0.5
	cutoff := math.Min(1.0, float64(dstRate)/float64(srcRate)) * 0.995
	const kaiserBeta = 8.6
	kaiserDen := besselI0(kaiserBeta)
	kernels := make([][taps]float64, phases)
	for p := 0; p < phases; p++ {
		frac := float64(p) / phases
		sum := 0.0
		for i := 0; i < taps; i++ {
			x := (float64(i) - center) - frac
			ratio := (2*float64(i))/float64(taps-1) - 1
			arg := 1 - ratio*ratio
			if arg < 0 {
				arg = 0
			}
			w := besselI0(kaiserBeta*math.Sqrt(arg)) / kaiserDen
			v := cutoff * sinc(cutoff*x) * w
			kernels[p][i] = v
			sum += v
		}
		if sum != 0 {
			for i := 0; i < taps; i++ {
				kernels[p][i] /= sum
			}
		}
	}
	dstLen := int(math.Ceil(float64(len(src)) * float64(dstRate) / float64(srcRate)))
	if dstLen < 1 {
		dstLen = 1
	}
	out := resizePCMInt16Buffer(dst, dstLen)
	for i := 0; i < dstLen; i++ {
		pos := float64(i) * float64(srcRate) / float64(dstRate)
		base := int(math.Floor(pos))
		frac := pos - float64(base)
		phase := int(math.Round(frac * phases))
		if phase >= phases {
			phase = 0
			base++
		}
		start := base - taps/2 + 1
		acc := 0.0
		for k := 0; k < taps; k++ {
			idx := start + k
			if idx < 0 {
				idx = 0
			} else if idx >= len(src) {
				idx = len(src) - 1
			}
			acc += float64(src[idx]) * kernels[phase][k]
		}
		out[i] = int16(clampFloat(math.Round(acc), -32768, 32767))
	}
	return out
}

func resampleMonoS16Linear(src []int16, srcRate, dstRate int) []int16 {
	return resampleMonoS16LinearInto(nil, src, srcRate, dstRate)
}

func resampleMonoS16LinearInto(dst []int16, src []int16, srcRate, dstRate int) []int16 {
	if len(src) == 0 || srcRate <= 0 || dstRate <= 0 {
		return nil
	}
	if srcRate == dstRate {
		out := resizePCMInt16Buffer(dst, len(src))
		copy(out, src)
		return out
	}
	out := resizePCMInt16Buffer(dst, resampledMonoLen(len(src), srcRate, dstRate))
	if len(src) == 1 {
		for i := range out {
			out[i] = src[0]
		}
		return out
	}
	scale := float64(srcRate) / float64(dstRate)
	last := len(src) - 1
	for i := range out {
		pos := float64(i) * scale
		base := int(pos)
		if base >= last {
			out[i] = src[last]
			continue
		}
		frac := pos - float64(base)
		a := float64(src[base])
		b := float64(src[base+1])
		out[i] = int16(math.Round(a + (b-a)*frac))
	}
	return out
}

func resampleMonoS16LinearQuantized(src []int16, srcRate, dstRate int) []int16 {
	return resampleMonoS16LinearQuantizedInto(nil, src, srcRate, dstRate)
}

func resampleMonoS16LinearQuantizedInto(dst []int16, src []int16, srcRate, dstRate int) []int16 {
	if len(src) == 0 || srcRate <= 0 || dstRate <= 0 {
		return nil
	}
	if srcRate == dstRate {
		out := resizePCMInt16Buffer(dst, len(src))
		copy(out, src)
		return out
	}
	outLen := int((int64(len(src))*int64(dstRate) + int64(srcRate) - 1) / int64(srcRate))
	if outLen <= 0 {
		return nil
	}
	out := resizePCMInt16Buffer(dst, outLen)
	step := (int64(srcRate) << 16) / int64(dstRate)
	pos := int64(0)
	last := len(src) - 1
	for i := 0; i < outLen; i++ {
		idx := int(pos >> 16)
		if idx < 0 {
			idx = 0
		} else if idx > last {
			idx = last
		}
		if idx >= last {
			out[i] = src[last]
		} else {
			frac := int(pos & 0xffff)
			a := int(src[idx])
			b := int(src[idx+1])
			out[i] = int16(((a * (65536 - frac)) + (b * frac) + 32768) >> 16)
		}
		pos += step
	}
	return out
}

func resampledMonoLen(srcLen, srcRate, dstRate int) int {
	if srcLen <= 0 || srcRate <= 0 || dstRate <= 0 {
		return 0
	}
	dstLen := int(math.Ceil(float64(srcLen) * float64(dstRate) / float64(srcRate)))
	if dstLen < 1 {
		return 1
	}
	return dstLen
}

func sinc(x float64) float64 {
	if math.Abs(x) < 1e-12 {
		return 1
	}
	x *= math.Pi
	return math.Sin(x) / x
}

func besselI0(x float64) float64 {
	sum := 1.0
	term := 1.0
	y := (x * x) * 0.25
	for k := 1; k < 32; k++ {
		term *= y / (float64(k) * float64(k))
		sum += term
		if term < 1e-12*sum {
			break
		}
	}
	return sum
}

func clampFloat(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func clampVolume(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

func abs64(v int64) int64 {
	if v < 0 {
		return -v
	}
	return v
}

func min64(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}

func angleToRadians(a uint32) float64 {
	return (float64(a) / float64(uint64(1)<<32)) * 2 * math.Pi
}
