# Voice Codec Notes

Implemented:

- [x] mono audio
- [x] 48 kHz capture and encode rate
- [x] 10 ms analysis/AGC frames
- [x] 5 codec frames bundled into one transport packet
- [x] 50 ms packetization interval during active speech
- [x] 4-bit IMA-style ADPCM voice codec
- [x] codec state carried across active speech
- [x] seeded resync packets on stream start
- [x] periodic seeded resync packets during active speech
- [x] expanded predictor seed state: `predictor`, `prevDelta`, `prevDelta2`, `stepIndex`
- [x] speech-tuned two-slope predictor
- [x] speech-biased step adaptation
- [x] capture high-pass filtering
- [x] AGC
- [x] noise-floor tracking
- [x] voiced/unvoiced detection
- [x] gate hold logic
- [x] smoothed gate attack/release
- [x] short intra-frame gate lookahead before onset
- [x] no-packet transmission during fully silent spans
- [x] encoder reset on silence break
- [x] seeded resume after silence break
- [x] TCP-ordered audio transport without inner per-packet frame index payload
- [x] receiver-side `StartSample` reconstruction from packet order
- [x] decoder reset on playback discontinuity
- [x] fade-out/fade-in smoothing around playback resets
- [x] client rebuffer behavior during silence gaps
- [x] optional rough voice/game sync overlay for debugging

Remaining ideas:

- [ ] mu-law companding ahead of ADPCM
