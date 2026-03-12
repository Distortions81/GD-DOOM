# Doom Trace Harness Checklist

Goal: compare per-tic Doom-source demo traces against GD-DOOM and stop at the first gameplay desync.

## Already in GD-DOOM

- `-trace-demo-state <path>` writes JSONL trace output during `-demo` playback.
- Trace output currently emits `meta`, `demo`, and per-`tic` records.
- Tic records include RNG indices, game and level counters, player state, active mobjs, and active specials.
- `cmd/demotracecmp` compares two trace files, filters to `kind="tic"`, and stops at the first field-level mismatch.
- Trace emission is covered by `internal/doomruntime/demo_trace_test.go`.

## Remaining Work

### Doom-source trace runner

- [ ] Add a Doom-source CLI mode for traced demo playback.
- [ ] Keep traced demo loading behavior aligned with `-playdemo` / `-timedemo`.
- [ ] Make traced playback run headless without changing tic behavior.
- [ ] Hook trace emission at the tic boundary.
- [ ] Support Doom demo versions `109` and `110`.

### Schema and parity follow-up

- [ ] Review field naming and ordering against Doom-source output and tighten compatibility where comparison needs it.
- [ ] Decide which fields currently ignored by `demotracecmp` should become strict matches.
- [ ] Expand mover and special coverage if a comparison case needs more state.
- [ ] Add a stable cross-engine object identity strategy where raw ordering is not sufficient.

### Validation

- [ ] Produce paired traces for `DOOM.WAD` `DEMO1`.
- [ ] Use `DOOM1.WAD` `DEMO1` as the local fallback bring-up case.
- [ ] Verify traced playback reaches demo end and produces complete paired traces.
- [ ] Add a small golden set that covers doors, lifts, switches, stairs, exits, and combat.
