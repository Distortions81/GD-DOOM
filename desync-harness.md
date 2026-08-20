# Desync Harness

This repo now has a repeatable demo-desync harness for comparing GD-DOOM against the original Linux DOOM source tree in `../doom-source`.

## Paths

- GD-DOOM repo: `/home/dist/github/GD-DOOM`
- Original DOOM source tree: `/home/dist/github/doom-source`
- Reference binary: `/home/dist/github/doom-source/linuxdoom-1.10/linux/linuxxdoom`
- Harness script: `/home/dist/github/GD-DOOM/scripts/demo_trace_compare.sh`
- Batch harness: `/home/dist/github/GD-DOOM/scripts/demo_trace_compare_batch.sh`
- Comparator: `/home/dist/github/GD-DOOM/cmd/demotracecmp`

## What The Harness Does

`scripts/demo_trace_compare.sh` performs the full compare loop:

1. Cleans and rebuilds a local GD-DOOM trace binary.
2. Cleans and rebuilds `cmd/demotracecmp`.
3. Runs the original DOOM executable with `-tracedemo <demo lump> -tracefile <path>` against an isolated temp dir containing the selected IWAD.
4. Runs GD-DOOM with `-render=false -demo <file> -trace-demo-state <path>`, using its direct, uncapped simulation loop.
5. Compares the resulting JSONL tic traces and stops at the first mismatch.

The comparator already ignores a small set of known non-parity fields, so the reported mismatch is usually actionable.

For external demo sets, `scripts/demo_trace_compare_batch.sh` runs the same loop across a directory of `.lmp` files and writes a tab-separated summary with the first reported mismatch per demo.

## Requirements

- `../doom-source` must exist and contain a built `linuxxdoom`.
- `DOOM1.WAD` must be available in the GD-DOOM repo root unless overridden.
- No desktop display or `xvfb-run` is required: GD-DOOM trace playback is headless and uncapped.

Notes:

- The reference runtime already includes trace support via `-tracedemo` and `-tracefile`.
- By default the harness symlinks the selected `--wad` into an isolated temp dir before launching the reference runtime, so extra IWADs in the repo root do not change which game data gets loaded.
- `-render=false` preserves per-tic demo traces and bypasses Ebiten entirely; it is the required fast path for harness runs.
- The harness does not pass `-demo-exit-on-death`, but trims both traces at the first player-death tic. This matches the reference runtime's terminal replay behavior while preserving a fast, uncapped GD-DOOM run.
- `--stop-after-tics <n>` now trims both the reference and GD-DOOM traces before compare, so short-window debugging does not degrade into a length mismatch.
- The harness always uses the headless `-render=false` fast path. `--headless` and `--no-headless` remain accepted as no-op compatibility options.
- `demotracecmp` mismatch reports now include the normalized failing mobj entry and the matched `gametic` from both sides, which matters because the compare pass sorts mobjs before diffing.

## Default Inputs

- Reference demo lump: `demo1`
- GD-DOOM demo file: `/home/dist/github/GD-DOOM/demos/DOOM1-DEMO1.lmp`
- Output directory: `/home/dist/github/GD-DOOM/tmp/demo-trace-compare`

These defaults are set up to compare the built-in `DEMO1` lump from the original runtime against the extracted `.lmp` file in this repo.

## Basic Usage

From `/home/dist/github/GD-DOOM`:

```bash
scripts/demo_trace_compare.sh
```

Useful overrides:

```bash
scripts/demo_trace_compare.sh --out /tmp/demo-trace-check
scripts/demo_trace_compare.sh --demo-lump demo2 --demo ./demos/DOOM1-DEMO2.lmp
scripts/demo_trace_compare.sh --ref-bin ../doom-source/linuxdoom-1.10/linux/linuxxdoom
scripts/demo_trace_compare.sh --headless
scripts/demo_trace_compare.sh --no-headless
scripts/demo_trace_compare.sh --stop-after-tics 121
scripts/demo_trace_compare.sh -- --width 640 -height 400
```

Late Doom II UV Max batch example:

```bash
scripts/demo_trace_compare_batch.sh --wad ./wads/DOOM2.WAD --out-root /tmp/doom2-uvmax-late
```

## Output Files

The harness writes:

- `reference-<demo>.jsonl`: trace from the original DOOM source
- `reference-<demo>.log`: stdout/stderr from the original runtime
- `gddoom-<demo-file>.jsonl`: GD-DOOM trace
- `gddoom-<demo-file>.log`: GD-DOOM stdout/stderr
- `compare.log`: output from `demotracecmp`

## Verified Result

This setup was first verified with:

```bash
scripts/demo_trace_compare.sh --out /tmp/demo-trace-check
```

That earlier run completed and found the first mismatch at:

```text
mismatch line=1699 path=root.mobj_count
left=202
right=201
```

Artifacts from that verified run:

- `/tmp/demo-trace-check/reference-demo1.jsonl`
- `/tmp/demo-trace-check/reference-demo1.log`
- `/tmp/demo-trace-check/gddoom-DOOM1-DEMO1.lmp.jsonl`
- `/tmp/demo-trace-check/gddoom-DOOM1-DEMO1.lmp.log`
- `/tmp/demo-trace-check/compare.log`

## Current Interpretation

The current `DEMO1` desync is not just a cosmetic field mismatch. At the first failing tic, the reference runtime has one more mobj than GD-DOOM while special counts still match.

After fixing the earlier rocket splash and player-vs-barrel radius-order issue, the current first mismatch moved later. A current run reached:

```text
mismatch line=1860 path=root.mobjs[203].floorz
left=-524288
right=-1572864
```

Current next step:

- Inspect the rocket support/floor selection around gametic 1859 in both JSONL traces.

## Late UV Max Note

The first `doom2-uvmax-late` issue found on `DOOM2-MAP21-UVMAX.lmp` was fixed on `2026-04-16`.

- Old first mismatch: `gametic=120`, exploding `MT_FATSHOT` impact position
- Cause: GD-DOOM split large negative projectile momentum into half-steps, unlike Doom
- Result after fix: trace now matches through tic 121 and the next `MAP21` mismatch moves to `gametic=176` on `root.mobjs[29].movecount`
