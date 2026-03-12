# OPL3 Listening Checklist

Use this checklist when tuning the pure-Go OPL3 backend against the Nuked backend.

## Setup

Run the same WAD, map music, and output device twice:

```bash
go run ./cmd/gddoom -wad DOOM1.WAD -map E1M1 -opl3-backend=purego
go run ./cmd/gddoom -wad DOOM1.WAD -map E1M1 -opl3-backend=nuked
```

Optional bank override:

```bash
go run ./cmd/gddoom -wad DOOM1.WAD -map E1M1 -opl3-backend=purego -opl-bank=GENMIDI.op2
```

## Tracks

- `D_E1M1`: compare melodic lead presence, note-on bite, and stereo balance.
- `D_E1M4`: compare sustained brass/body tone, decay slope, and release tail smoothness.
- `D_E1M8`: compare harsher/brighter patches, feedback edge, and percussion articulation.

## What To Listen For

- Attack: note starts should speak quickly without sounding clicky or flattened.
- Decay and sustain: held notes should settle into a stable body instead of collapsing or staying too loud.
- Release: note-offs should fade naturally without abrupt dropouts.
- FM bite: bright leads should keep their edge without turning brittle or hollow.
- Additive layers: doubled voices should sound fuller, not phasey or obviously detuned from the Nuked reference.
- Vibrato and tremolo: modulation should be audible but not exaggerated.
- Percussion: drum hits should stay distinct and not smear into pitched voices.

## Acceptance

- Automated `internal/music` comparison tests pass.
- Automated `internal/sound` behavior tests pass.
- No obvious regression is audible on the three reference tracks above when switching between `purego` and `nuked`.
