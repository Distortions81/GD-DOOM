# Voice Codec Notes

## Current Direction

The current voice path uses a custom IMA-style 4:1 ADPCM codec with:

- mono audio
- 10 ms frames
- state carried across frames
- periodic seeded resync packets
- a tuned forward predictor
- speech-biased step adaptation

## Tuning Ideas For Voice

### 1. Keep Predictor And Step State Continuous

Do not reset predictor and step index every frame unless the stream actually breaks.

Why:

- reduces boundary artifacts
- improves continuity on vowels and sustained speech
- makes the codec sound less chattery between packets

Implementation notes:

- keep encoder and decoder state alive across chunks
- reset only on discontinuity, silence segmentation, config changes, or explicit resync
- emit occasional seeded packets so late joiners and loss recovery can re-lock

### 2. Bias Step Adaptation For Speech

The standard IMA adaptation curve is generic. For voice, it can help to react:

- slower to short spikes
- faster to sustained energy

Why:

- reduces zipper noise on consonant edges and transient spikes
- avoids over-expanding the quantizer on brief events
- keeps sustained voiced segments from sounding too constrained

Implementation notes:

- tune the index update table, especially the mid-range codes
- keep large-code growth available so louder speech can still open the step size
- validate changes by listening, not just by sample error metrics

### 3. Use A Better Predictor

Plain previous-sample prediction is simple but not ideal for speech.

A better low-cost option is a first-order forward predictor:

`pred = current + alpha * prev_delta`

Example:

`pred = current + 0.75 * prev_delta`

Why:

- tracks voiced speech better
- reduces residual error on smooth segments
- improves quality without changing the 4-bit nibble size

Tradeoff:

- seeded resync packets need to carry extra predictor state such as `prev_delta`

### 4. Speech Band Limiting

The codec should not waste effort on frequencies that matter less for voice.

Useful range:

- high-pass around 80-120 Hz
- low-pass around 3.4-4.5 kHz for narrowband-ish speech

Why:

- removes low rumble and excess top-end noise
- lets the codec spend bits on the speech band
- usually improves subjective intelligibility

### 5. Lower Sample Rate

Voice usually does not need full-band sample rates.

Good candidates:

- 24 kHz for a cleaner wideband voice path
- 16 kHz for a more efficient speech-focused path

Why:

- fewer samples per frame
- lower bandwidth
- less work for the predictor and quantizer

Tradeoff:

- lower rates reduce upper-frequency detail

### 6. Better VAD/DTX

Silence suppression often saves more bandwidth than squeezing active speech harder.

Useful additions:

- better silence detection
- hangover logic so short pauses do not flap on and off
- optional comfort noise at playback

Why:

- reduces wasted packets
- makes speech gating sound more natural

### 7. Mu-law Companding

Mu-law is a companding scheme for audio.

Instead of storing samples linearly, it compresses dynamic range before quantization:

- quiet sounds get relatively more precision
- loud sounds get relatively less precision

Why it helps voice:

- preserves quiet consonants and low-level speech detail better
- makes quantization noise less objectionable in softer speech
- is computationally cheap

Ways to use it here:

- as a preprocessing step before ADPCM
- or as a separate simpler codec

Tradeoffs:

- adds another nonlinearity and another source of distortion
- helps speech more than music
- as a standalone codec it is usually only 2:1 relative to 16-bit PCM

## Recommended Next Steps

If continuing to tune this codec for voice, the highest-value order is:

1. add speech band limiting
2. consider moving to 16 kHz
3. evaluate light mu-law-style companding before ADPCM
4. keep refining predictor and step adaptation by listening tests

## Evaluation Notes

Do not rely only on average sample error.

Use listening tests focused on:

- consonant clarity
- vowel smoothness
- gating behavior around silence
- background hiss / zipper noise
- recovery after packet loss or stream reset
