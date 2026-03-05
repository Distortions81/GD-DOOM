# Music Parity Checklist (Chocolate Doom OPL)

Reference implementation: `_research/chocolate-doom/src/i_oplmusic.c` and `_research/chocolate-doom/src/mus2mid.c`.

## Implemented

- GENMIDI/OP2 header + instrument table parsing (`#OPL_II#`, 128 melodic + 47 percussion).
- OP2 operator register mapping:
  - `0x20`, `0x40`, `0x60`, `0x80`, `0xE0`, `0xC0`.
- Percussion instrument mapping (`MIDI 9` channel to key `35..81` bank entries).
- Double-voice (`GENMIDI_FLAG_2VOICE`) note allocation.
- Fixed-note (`GENMIDI_FLAG_FIXED`) handling.
- Base note offset per OP2 voice.
- Fine tuning for second OP2 voice: `(fine_tuning / 2) - 64`.
- MUS pitch wheel byte handling aligned to mus2mid (`raw * 64`) and OPL-side MSB bend usage (`param2 - 64`).
- Frequency generation now uses Chocolate Doom-compatible `frequency_curve` indexing and octave clamp behavior.

## In Progress / Gaps

- Driver/version quirks:
  - Doom 1.666 / Doom2 1.666 / Doom 1.9 voice replacement differences are not implemented.
- OPL core:
  - Current runtime uses `BasicOPL3` scaffold.
  - Target parity path should use Nuked OPL3 backend with equivalent register timing/order.
- MIDI/MUS controller edge behavior:
  - Need exact parity checks for controller clipping/quirks used by DMX path.

## Next Concrete Steps

1. Add driver-mode switch for Doom 1.666 / Doom2 1.666 / Doom 1.9 voice stealing behavior.
2. Integrate Nuked OPL3 backend and run A/B comparisons against Chocolate Doom.
3. Add parity tests for known tracks (`D_E1M1`, `D_E1M2`, `D_RUNNIN`) using deterministic register traces.
