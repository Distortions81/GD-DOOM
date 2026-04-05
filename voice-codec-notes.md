# Voice Codec Notes

## Current Implementation

The current realtime voice path is built around a custom IMA-style 4:1 ADPCM codec in [`internal/voicecodec/ima41.go`](/home/dist/github/GD-DOOM/internal/voicecodec/ima41.go).

Current properties:

- mono audio
- 48 kHz capture and encode rate
- 10 ms frames
- 4-bit samples packed into fixed-size packets
- codec state carried across active speech frames
- seeded resync packets on stream start and every 50 frames
- expanded predictor seed state: `predictor`, `prevDelta`, `prevDelta2`, `stepIndex`
- a speech-tuned two-slope predictor

At 48 kHz mono with 4-bit samples, the encoded active-speech bitrate is about 192 kbps before transport overhead.

## Codec Behavior

### State Continuity

The encoder and decoder keep predictor and step state continuous across active speech.

This is already implemented and is the right default for this codec because it:

- reduces frame-boundary chatter
- improves continuity on sustained vowels
- lowers the need for each 10 ms frame to re-establish local history

The state is reset only when the stream is intentionally broken:

- startup
- silence segmentation
- audio config changes
- non-contiguous `startSample` on playback

### Seeded Resync Packets

The codec periodically emits seeded packets instead of relying on a single initial state transfer.

Current behavior:

- the first active frame after reset is seeded
- another seeded packet is sent every 50 frames
- at 10 ms per frame, that is every 500 ms

This matters because the predictor is no longer just the last sample. Late recovery needs:

- predictor
- previous delta
- second previous delta
- step index

### Predictor

The current predictor is:

`predicted = current + prevDelta + (prevDelta - prevDelta2) / 4`

That is a small forward predictor with slope damping. In practice it is trying to:

- follow sustained voiced motion
- avoid overshooting harder transients
- keep the implementation cheap enough for realtime relay use

This is more advanced than the older note that discussed only a first-order `prev_delta` term.

### Step Adaptation

The step table remains standard IMA-sized, but the index update table is already speech-biased:

`[-1, -1, -1, 0, 1, 3, 5, 7]`

The practical effect is:

- small codes decay slowly
- mid codes open moderately
- large codes still expand quickly when speech energy rises

That matches the original goal of reducing zipper noise while still tracking louder speech.

## Voice Path Around The Codec

The transport codec is only one part of perceived voice quality. The current session voice path in [`internal/sessionvoice/voice.go`](/home/dist/github/GD-DOOM/internal/sessionvoice/voice.go) and [`internal/sessionvoice/agc.go`](/home/dist/github/GD-DOOM/internal/sessionvoice/agc.go) already does several important things around it.

### Capture-Side Filtering

The raw mic input is high-passed before encoding.

Current behavior:

- explicit capture high-pass at 50 Hz
- AGC/VAD analysis emphasizes roughly the 120 Hz to 4 kHz speech band

This means the old note about adding speech band limiting is now partially implemented on the analysis side, but not yet as a stronger encode-side band-limit pass.

### AGC And Silence Gating

The broadcaster path already applies:

- adaptive gain control
- noise-floor tracking
- voiced/unvoiced detection
- hold logic to avoid rapid gate flapping
- soft gating

When a frame is treated as silence:

- no compressed payload is sent
- the encoder is reset
- the next voiced frame starts with a seeded packet

So the previous "better VAD/DTX" item is also partially implemented already.

### Playback Recovery

The viewer path resets decode state when audio continuity breaks and adds smoothing around discontinuities:

- decoder reset on config change
- decoder reset on `startSample` discontinuity
- buffered audio reset when the playback queue grows too large
- fade-out/fade-in smoothing around resets
- linear resampling from stream rate to device rate

This is important because codec quality alone does not determine whether recovery after jitter or backlog sounds acceptable.

## What Changed Relative To The Older Notes

The earlier version of this file was mostly a tuning wishlist. A few items are now outdated because the code already implements them in some form.

Already implemented or largely implemented:

- continuous predictor and step state across active speech
- periodic seeded resync packets
- improved speech-oriented predictor
- speech-aware step adaptation
- AGC plus VAD/DTX-style silence suppression

Not implemented yet, or only partly implemented:

- lower sample-rate voice mode such as 24 kHz or 16 kHz
- a stronger encode-side low-pass for narrowband or wideband speech shaping
- comfort noise during silence
- mu-law companding ahead of ADPCM
- objective and subjective comparison harnesses for codec tuning

## Current Tradeoffs

The current path favors implementation simplicity and robustness over aggressive bitrate reduction.

Main benefits:

- fixed 10 ms packet cadence
- simple decoder
- no external codec dependency
- deterministic seeded recovery
- quality that should be noticeably better than plain last-sample IMA ADPCM

Main costs:

- 48 kHz mono ADPCM is still fairly bandwidth-heavy for voice
- silence resets discard predictor history by design
- there is no comfort-noise path yet
- there is no narrower speech profile yet for constrained links

## Recommended Next Steps

If the goal is better voice quality per bit, the highest-value next work is:

1. add an explicit encode-side low-pass and decide whether the voice path should target wideband or speech-band operation
2. evaluate a lower-rate mode, with 24 kHz as the least disruptive first cut and 16 kHz as the bandwidth-focused option
3. build repeatable listening tests that compare current ADPCM against candidate predictor and preprocessing changes
4. consider optional comfort noise so silence gating sounds less abrupt for listeners
5. only evaluate mu-law precompanding after the lower-rate and filtering decisions are settled

## Practical Guidance

If bandwidth matters more than preserving upper-frequency detail, dropping sample rate will likely beat further nibble-level tuning.

If quality at the current bitrate matters more, focus on:

- better front-end filtering
- stability of AGC/gating behavior
- listening tests across packet loss and playback resets

The current codec is no longer a blank-slate experiment. The next round of updates should treat it as an implemented speech ADPCM path with specific transport and recovery behavior, not just a generic IMA variant.
