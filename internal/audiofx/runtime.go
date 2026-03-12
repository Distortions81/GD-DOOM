package audiofx

import (
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
	doomSoundCloseDist    = int64(160 * fracUnit)
	doomSoundStereoSwing  = int64(96 * fracUnit)
	doomSoundAttenuator   = (doomSoundClippingDist - doomSoundCloseDist) / fracUnit
	doomSoundNormalSep    = 128
	doomSoundSepRange     = 256

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
	players    []*audio.Player
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
	players []*audio.Player
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
		players: make([]*audio.Player, 0, 8),
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
	for _, player := range p.players {
		if player == nil {
			continue
		}
		player.Pause()
		_ = player.Close()
	}
	p.players = p.players[:0]
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
	p.PlaySampleSpatial(sample, SpatialOrigin{}, 0, 0, 0, false)
}

func (p *SpatialPlayer) PlaySampleSpatial(sample media.PCMSample, origin SpatialOrigin, listenerX, listenerY int64, listenerAngle uint32, mapUsesFullClip bool) {
	if p == nil || p.ctx == nil || p.volume <= 0 {
		return
	}
	if sample.SampleRate <= 0 || len(sample.Data) == 0 {
		return
	}
	leftGain, rightGain := p.eventStereoGains(origin, listenerX, listenerY, listenerAngle, mapUsesFullClip)
	var pcm []byte
	if p.sourcePort && sample.PreparedRate == p.ctx.SampleRate() && len(sample.PreparedMono) > 0 {
		pcm = PCMMonoS16ToStereoS16LESpatial(sample.PreparedMono, leftGain, rightGain)
	} else if sample.SampleRate != p.ctx.SampleRate() {
		pcm = PCMMonoU8ToStereoS16LESpatialResampled(sample.Data, sample.SampleRate, p.ctx.SampleRate(), leftGain, rightGain)
	} else {
		pcm = PCMMonoU8ToStereoS16LESpatial(sample.Data, leftGain, rightGain)
	}
	if len(pcm) == 0 {
		return
	}
	player := audio.NewPlayerFromBytes(p.ctx, pcm)
	player.SetVolume(1)
	player.Play()
	p.players = append(p.players, player)
}

func (p *SpatialPlayer) Tick() {
	if p == nil || len(p.players) == 0 {
		return
	}
	keep := p.players[:0]
	for _, player := range p.players {
		if player.IsPlaying() {
			keep = append(keep, player)
			continue
		}
		_ = player.Close()
	}
	p.players = keep
}

func (p *SpatialPlayer) StopAll() {
	if p == nil || len(p.players) == 0 {
		return
	}
	for _, player := range p.players {
		if player == nil {
			continue
		}
		player.Pause()
		_ = player.Close()
	}
	p.players = p.players[:0]
}

func PrepareSoundBankForSourcePort(bank media.SoundBank, dstRate int) media.SoundBank {
	if dstRate <= 0 {
		return bank
	}
	rv := reflect.ValueOf(&bank).Elem()
	for i := 0; i < rv.NumField(); i++ {
		field := rv.Field(i)
		sample, ok := field.Addr().Interface().(*media.PCMSample)
		if !ok || sample == nil {
			continue
		}
		prepareSampleForSourcePort(sample, dstRate)
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
	if len(src) == 0 {
		return nil
	}
	out := make([]int16, len(src))
	for i, u := range src {
		out[i] = (int16(u) - 128) << 8
	}
	return out
}

func PCMMonoS16ToStereoS16LESpatial(src []int16, leftGain, rightGain float64) []byte {
	out := make([]byte, len(src)*4)
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
	return PCMMonoS16ToStereoS16LESpatial(PCMMonoU8ToMonoS16(src), leftGain, rightGain)
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
	return PCMMonoS16ToStereoS16LESpatial(resampleMonoS16Polyphase(PCMMonoU8ToMonoS16(src), srcRate, dstRate), leftGain, rightGain)
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
	var pcm []byte
	if sample.SampleRate == p.ctx.SampleRate() {
		pcm = PCMMonoU8ToStereoS16LESpatial(sample.Data, 1, 1)
	} else {
		pcm = PCMMonoU8ToStereoS16LEResampled(sample.Data, sample.SampleRate, p.ctx.SampleRate())
	}
	if len(pcm) == 0 {
		return
	}
	player := audio.NewPlayerFromBytes(p.ctx, pcm)
	player.SetVolume(p.volume)
	player.Play()
	p.players = append(p.players, player)
}

func prepareSampleForSourcePort(sample *media.PCMSample, dstRate int) {
	if sample == nil || len(sample.Data) == 0 || sample.SampleRate <= 0 || dstRate <= 0 {
		return
	}
	mono := PCMMonoU8ToMonoS16(sample.Data)
	if sample.SampleRate != dstRate {
		mono = resampleMonoS16Polyphase(mono, sample.SampleRate, dstRate)
	}
	mono = ApplySourcePortPresenceBoost(mono)
	sample.PreparedRate = dstRate
	sample.PreparedMono = mono
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

func resampleMonoS16Polyphase(src []int16, srcRate, dstRate int) []int16 {
	if len(src) == 0 || srcRate <= 0 || dstRate <= 0 {
		return nil
	}
	if srcRate == dstRate {
		out := make([]int16, len(src))
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
	out := make([]int16, dstLen)
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
