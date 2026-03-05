# Music Parity Checklist (Chocolate Doom OPL)

Reference implementation: `_research/chocolate-doom/src/i_oplmusic.c` and `_research/chocolate-doom/src/mus2mid.c`.

## Implemented

- GENMIDI/OP2 header + instrument table parsing (`#OPL_II#`, 128 melodic + 47 percussion).
- OP2 operator register mapping:
  - `0x20`, `0x40`, `0x60`, `0x80`, `0xE0`, `0xC0`.
- Percussion instrument mapping (`MIDI 9` channel to key `35..81` bank entries).
- Percussion out-of-range note filtering on channel 9 (`<35` or `>81` ignored).
- Double-voice (`GENMIDI_FLAG_2VOICE`) note allocation.
- Fixed-note (`GENMIDI_FLAG_FIXED`) handling.
- Base note offset per OP2 voice.
- Fine tuning for second OP2 voice: `(fine_tuning / 2) - 64`.
- MUS pitch wheel byte handling aligned to mus2mid (`raw * 64`) and OPL-side MSB bend usage (`param2 - 64`).
- Frequency generation now uses Chocolate Doom-compatible `frequency_curve` indexing and octave clamp behavior.
- cgo runtime path uses Nuked OPL3 (`internal/sound/opl3_nuked.go`), with buffered register writes (`OPL3_WriteRegBuffered`).
- Driver reset now applies key low-register init parity writes (`0x04`, `0x01`, `0x105`, `0x08`).
- Channel `volume`/`pan` controller changes now refresh active voices immediately.
- Pitch bend now reorders updated voices to the back of allocation order (matching DMX replacement side effect).

## In Progress / Gaps

- Driver/version quirks:
  - Doom 1.666 / Doom2 1.666 / Doom 1.9 voice replacement differences are not implemented.
- MIDI/MUS controller edge behavior:
  - Need exact parity checks for controller clipping/quirks used by DMX path.
- Map playback behavior:
  - Current map music enqueue path is one-shot PCM render; no automatic loop/restart yet.
- Final parity polish:
  - Confirm any remaining octave/timbre differences against Chocolate Doom register traces.

## Next Concrete Steps

1. Add driver-mode switch for Doom 1.666 / Doom2 1.666 / Doom 1.9 voice stealing behavior.
2. Integrate Nuked OPL3 backend and run A/B comparisons against Chocolate Doom.
3. Add parity tests for known tracks (`D_E1M1`, `D_E1M2`, `D_RUNNIN`) using deterministic register traces.
