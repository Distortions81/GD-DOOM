package doomruntime

// newHeadlessSoundSystem retains sound-event bookkeeping, including vanilla
// cosmetic RNG use, without opening an audio device. A server can receive normal
// client defaults (including a nonzero volume) without constructing a player.
func newHeadlessSoundSystem(opts Options) *soundSystem {
	s := newSoundSystem(SoundBank{}, nil, nil, 0, 0, sourcePortAudioEnabled(opts), opts.SFXPitchShift, opts.PCSpeakerVariant)
	s.vanillaVolume = vanillaSFXVolume(opts.SFXVolume)
	return s
}
