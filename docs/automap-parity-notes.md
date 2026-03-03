# Doom Automap Parity Notes

This checklist tracks behavior needed to match vanilla Doom automap (`am_map.c`) more closely.

## Rendering and Visibility

- [ ] Respect per-line discovery (`ML_MAPPED`) in normal automap mode.
- [ ] Implement Computer Area Map behavior (`pw_allmap`): show unrevealed lines in gray.
- [ ] Keep `LINE_NEVERSEE` hidden in normal and allmap modes.
- [ ] Support `IDDT` cheat levels:
- [ ] Level 1: reveal map lines.
- [ ] Level 2: reveal things.
- [ ] Draw one-sided lines as solid walls.
- [ ] Draw secret doors (`ML_SECRET`) as normal walls unless cheat mode is active.
- [ ] Draw teleporter lines (special 39) with teleporter color rule.
- [ ] Draw floor-height delta two-sided lines with floor-change color.
- [ ] Draw ceiling-height delta two-sided lines with ceiling-change color.
- [ ] Draw two-sided lines with no height delta only in cheat mode.
- [ ] Do not add a special "open door" visual style; doors are drawn by geometry/flags.

## Controls and UX Parity

- [ ] Support classic automap enter/exit flow (`TAB`) as optional compatibility mode.
- [ ] Support follow toggle and non-follow pan semantics.
- [ ] Support grid toggle.
- [ ] Support mark and clear-mark controls.
- [ ] Support big-map toggle behavior.
- [ ] Show numbered marks similarly to vanilla.

## Entities and Multiplayer

- [ ] Show things only in `IDDT` level 2 mode.
- [ ] Draw multiplayer arrows/colors when multiple players are present.

## Compatibility Notes

- [ ] Keep current rotating heading/follow-heading mode behind a "modern" toggle.
- [ ] Document that rotating map mode is source-port style, not vanilla Doom automap behavior.

## Validation Plan

- [ ] Build E1M1 parity checks for: normal mode, allmap mode, iddt1, iddt2.
- [ ] Add unit tests for line inclusion rules (`ML_MAPPED`, `LINE_NEVERSEE`, cheat/allmap gates).
- [ ] Add screenshot-based regression checks for key map states.

## Sound Decode Track (Project Root)

- [ ] Inventory sound lumps (`DS*`, `DP*`, etc.) and output a report.
- [ ] Decode DMX sound format to WAV in a root output folder.
- [ ] Add CLI flags for decode mode and output directory.
- [ ] Add parser/decoder tests for malformed and valid sample lumps.
