# Doom Trace Harness Checklist

Goal: add a headless Doom-source trace mode that emits per-tic gameplay state for demo playback, then compare it against GD-DOOM to catch first desync.

## Phase 1: Doom-source trace runner

- [ ] Add a Doom-source CLI mode for traced demo playback.
  - Proposed flags: `-tracedemo <demo>` and `-tracefile <path>`
  - Keep demo loading behavior aligned with `-playdemo` / `-timedemo`

- [ ] Make traced demo playback run headless.
  - Avoid normal render loop work where possible
  - Avoid menu/title/demo-page presentation paths
  - Keep gameplay ticking identical to normal demo playback

- [ ] Hook trace emission at the tic boundary.
  - Preferred location: after `G_Ticker()` advances the tic and before `gametic++`
  - Emit exactly one snapshot per completed tic

- [ ] Support Doom demo versions `109` and `110`
  - Important for built-in IWAD demo compatibility

## Phase 2: Minimal snapshot schema

- [ ] Emit a machine-readable trace format.
  - Start with JSONL
  - One JSON object per tic

- [ ] Log minimal deterministic state first:
  - [ ] `gametic`
  - [ ] demo header metadata
  - [ ] RNG indices/state
  - [ ] `gamestate`
  - [ ] `gameaction`
  - [ ] `leveltime`
  - [ ] player position/angle/momentum
  - [ ] player health/armor/weapon

- [ ] Add trace startup metadata.
  - IWAD / demo name
  - demo version
  - episode/map/skill

## Phase 3: Expand trace coverage

- [ ] Dump active mobjs per tic.
  - type
  - x/y/z
  - momx/momy/momz
  - angle
  - health
  - state
  - tics
  - flags

- [ ] Dump sector mover state per tic.
  - doors
  - floors
  - plats
  - ceilings

- [ ] Dump selected sector state.
  - floor height
  - ceiling height
  - light level
  - specials / tags where relevant

- [ ] Add stable object identity strategy.
  - Prefer debug IDs on spawn
  - Avoid raw pointer comparison in the trace format

## Phase 4: GD-DOOM matching trace

- [ ] Add the same snapshot schema to GD-DOOM
- [ ] Add a CLI flag or debug mode to emit per-tic trace
- [ ] Keep field naming and ordering compatible with Doom-source output

## Phase 5: Comparator

- [ ] Add a trace comparator tool
  - Inputs: Doom trace, GD-DOOM trace
  - Stop at first mismatch
  - Print tic number and field-level diff

- [ ] Add summary output:
  - total matched tics
  - first mismatch tic
  - missing/extra entities if applicable

## First test case

- [ ] Primary target: `DOOM.WAD` `DEMO1`
- [ ] Fallback local bring-up target: `DOOM1.WAD` `DEMO1`
- [ ] Verify traced playback reaches demo end and produces a complete trace file

## Nice-to-have later

- [ ] Binary trace format for faster long demos
- [ ] Snapshot hashing for quick mismatch detection
- [ ] Per-system diff filters (player-only, movers-only, mobjs-only)
- [ ] Golden demo set covering doors, lifts, switches, stairs, exits, and combat
