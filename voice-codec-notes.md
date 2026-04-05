# Voice Codec Notes

## Current Implementation

The current realtime voice path uses a custom IMA-style 4:1 ADPCM codec in [`internal/voicecodec/ima41.go`](/home/dist/github/GD-DOOM/internal/voicecodec/ima41.go), with the surrounding capture/playback path in [`internal/sessionvoice/voice.go`](/home/dist/github/GD-DOOM/internal/sessionvoice/voice.go) and [`internal/sessionvoice/agc.go`](/home/dist/github/GD-DOOM/internal/sessionvoice/agc.go).

Implemented now:

- [x] mono audio
- [x] 48 kHz capture and encode rate
- [x] 10 ms analysis/AGC frames
- [x] 5 codec frames bundled into one transport packet
- [x] 50 ms packetization interval during active speech
- [x] 4-bit ADPCM samples
- [x] codec state carried across active speech
- [x] seeded resync packets on stream start and every 50 ms-frame equivalents
- [x] expanded predictor seed state: `predictor`, `prevDelta`, `prevDelta2`, `stepIndex`
- [x] a speech-tuned two-slope predictor

At 48 kHz mono with 4-bit samples, the active-speech encoded bitrate is still about 192 kbps before transport overhead. Packet bundling reduces packet rate and framing overhead, not the raw coded bitrate.

## Codec Behavior

### State Continuity

The encoder and decoder keep predictor and step state continuous across active speech.

That is still the right default because it:

- reduces frame-boundary chatter
- improves continuity on sustained vowels
- lowers the need for each 10 ms slice to re-establish local history

State resets happen on intentional discontinuities:

- startup
- silence break
- audio config changes
- playback discontinuity detected from `StartSample`

### Seeded Resync Packets

The codec periodically emits seeded packets rather than relying only on the first packet of the stream.

Current behavior:

- the first active packet after reset is seeded
- another seeded packet is sent every 50 ms-frame equivalents
- with the current 5-frame packetization, that lands on a regular active-speech cadence while preserving sub-packet codec state continuity

This matters because late recovery needs more than the last sample:

- predictor
- previous delta
- second previous delta
- step index

### Predictor

The current predictor is:

`predicted = current + prevDelta + (prevDelta - prevDelta2) / 4`

This is a small forward predictor with slope damping. In practice it tries to:

- follow sustained voiced motion
- avoid overshooting harder transients
- stay cheap enough for realtime relay use

### Step Adaptation

The step table is still standard IMA-sized, but the index update table is speech-biased:

`[-1, -1, -1, 0, 1, 3, 5, 7]`

Practical effect:

- small codes decay slowly
- mid codes open moderately
- large codes still expand quickly when speech energy rises

## Voice Path Around The Codec

The codec is only part of perceived voice quality. The current voice path already does a fair amount of shaping around it.

### Capture-Side Filtering

The raw mic input is high-passed before encoding.

Current behavior:

- explicit capture high-pass at 50 Hz
- AGC/VAD analysis emphasizes roughly the 120 Hz to 4 kHz speech band

This is still only partial band limiting. There is not yet a stronger encode-side low-pass that intentionally narrows the coded speech band.

### AGC And Gate Behavior

The broadcaster path now applies:

- adaptive gain control
- noise-floor tracking
- voiced/unvoiced detection
- hold logic to avoid rapid gate flapping
- smoothed gate attack/release
- short intra-frame lookahead so the gate can start opening before the detected onset group

Important current detail:

- fully silent packet windows are not transmitted at all
- the encoder resets during those gaps
- the next voiced packet resumes with seeded codec state

So silence suppression is now more aggressive than the earlier notes implied. There is no comfort noise path. Silence means no audio packets.

### Packetization

The system no longer sends one 10 ms audio chunk per network packet.

Current behavior:

- capture, filtering, AGC, and gating still happen on 10 ms slices
- 5 adjacent slices are packed into one codec packet
- active speech therefore sends about 20 packets per second instead of 100

That reduces:

- per-packet framing overhead
- transport wakeups
- sensitivity to short scheduling jitter

Tradeoff:

- packetization latency is now 50 ms instead of 10 ms

### Playback Recovery

The viewer path resets decode state when audio continuity breaks and smooths around those resets:

- decoder reset on config change
- decoder reset on `StartSample` discontinuity
- buffered audio reset when queued playback grows too large
- fade-out/fade-in smoothing around resets
- linear resampling from stream rate to device rate

Because silence now sends no packets, the client naturally drains and re-buffers around pauses. That is an intentional part of the current design, not just an error recovery path.

## Transport Notes

The current audio relay behavior is simpler than before:

- no idle silence packets
- no inner audio payload header carrying a per-packet frame index
- packet ordering relies on the TCP stream
- `StartSample` is reconstructed on the receiver side from packet order and configured packet size

This keeps idle voice traffic near zero and lets the client catch back up during pauses.

## Status Checklist

Implemented now:

- [x] continuous predictor and step state across active speech
- [x] periodic seeded resync packets
- [x] improved speech-oriented predictor
- [x] speech-aware step adaptation
- [x] AGC plus gate-based silence suppression
- [x] smoothed gate attack/release
- [x] short intra-frame gate lookahead before onset
- [x] packet bundling at 5 x 10 ms per transport packet
- [x] no-packet transmission during fully silent spans
- [x] TCP-ordered audio transport without per-packet inner frame index payload
- [x] client fade/rebuffer recovery on discontinuity
- [x] optional rough voice/game sync overlay for debugging

Ideas or not implemented yet:

- [ ] lower sample-rate voice mode such as 24 kHz or 16 kHz
- [ ] stronger encode-side low-pass for narrowband or wideband speech shaping
- [ ] comfort noise during silence
- [ ] mu-law companding ahead of ADPCM
- [ ] repeatable listening/evaluation harnesses for codec tuning
- [ ] explicit A/V sync control beyond rough buffer tuning and optional overlay readout

## Current Tradeoffs

The current path favors simple robust relay behavior over aggressive bitrate reduction or formal A/V sync.

Main benefits:

- simple decoder
- no external codec dependency
- deterministic seeded recovery
- lower packet rate than the original 10 ms transport
- zero idle bandwidth during silence
- silence periods let the client naturally drain backlog and reduce skip pressure

Main costs:

- 48 kHz mono ADPCM is still fairly bandwidth-heavy for voice
- packetization latency is now 50 ms
- silence resets discard predictor history by design
- there is still no comfort-noise path
- there is still no narrower speech profile for constrained links

## Recommended Next Steps

If the goal is better voice quality per bit, the highest-value next work is:

1. add an explicit encode-side low-pass and decide whether the voice path should target wideband or speech-band operation
2. evaluate a lower-rate mode, with 24 kHz as the least disruptive first cut and 16 kHz as the bandwidth-focused option
3. build repeatable listening tests that compare current ADPCM against candidate predictor and preprocessing changes
4. tune the AGC ceiling and response for quiet microphones using real captures, not only synthetic tests
5. only evaluate mu-law precompanding after the lower-rate and filtering decisions are settled

## Practical Guidance

If bandwidth matters more than preserving upper-frequency detail, dropping sample rate will likely beat further nibble-level tuning.

If quality at the current bitrate matters more, focus on:

- better front-end filtering
- AGC/gate tuning on real microphones
- listening tests across silence resumes and playback resets
- rough voice/game alignment via client buffer sizing rather than hard sync logic

The current codec is no longer a blank-slate experiment. The next round of work should treat it as a working speech ADPCM transport with specific packetization, silence, and recovery behavior.
