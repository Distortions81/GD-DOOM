Late Doom II UV Max demos downloaded from DSDA for desync testing against higher-map monster AI.

Files:
- `DOOM2-MAP21-UVMAX.lmp`: Map 21, Looper, 1:58.26, 2020-10-02, `lv21-158.zip`
- `DOOM2-MAP22-UVMAX.lmp`: Map 22, Kinetic, 1:02.20, 2024-03-01, `lv22m102.zip`
- `DOOM2-MAP24-UVMAX.lmp`: Map 24, Kinetic, 4:05.34, 2024-03-01, `lv24m405.zip`
- `DOOM2-MAP26-UVMAX.lmp`: Map 26, Looper, 2:37.89, 2020-11-13, `lv26-237.zip`
- `DOOM2-MAP27-UVMAX.lmp`: Map 27, Looper, 2:57.37, 2020-11-06, `lv27-257.zip`
- `DOOM2-MAP28-UVMAX.lmp`: Map 28, Vile, 2:19.40, 2026-02-28, `lv28m219.zip`
- `DOOM2-MAP29-UVMAX.lmp`: Map 29, Kinetic, 3:51.66, 2024-02-26, `lv29m351.zip`
- `DOOM2-MAP30-UVMAX.lmp`: Map 30, Looper, 0:29.86, 2019-01-08, `lv30-029.zip`

Source ZIPs are preserved under `.zips/`.

Example compare run once `DOOM2.WAD` is available in the repo root:

```bash
scripts/demo_trace_compare.sh \
  --wad ./DOOM2.WAD \
  --demo-lump m28uvmax \
  --demo ./demos/doom2-uvmax-late/DOOM2-MAP28-UVMAX.lmp \
  --out ./tmp/compare-map28
```

Use an 8-character-or-shorter `--demo-lump` name because the original Linux DOOM runtime rejects longer lump bases. The compare harness stages the selected `--demo` for the reference runtime as `<demo-lump>.lmp`, so external DSDA demos work without any extra manual setup.

Batch sweep:

```bash
scripts/demo_trace_compare_batch.sh \
  --wad ./wads/DOOM2.WAD \
  --out-root /tmp/doom2-uvmax-late
```

Current note:

- `DOOM2-MAP21-UVMAX.lmp` had an early `MT_FATSHOT` impact-position desync at `gametic=120`.
- That issue was fixed on `2026-04-16` by matching Doom's signed projectile half-step rule for large negative momentum.
- After that fix, `MAP21`'s next first mismatch moved to `gametic=176` on a monster `movecount` field.
