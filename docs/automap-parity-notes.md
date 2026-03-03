# Doom Automap Parity Notes

This checklist tracks behavior needed to match vanilla Doom automap (`am_map.c`) more closely.

## Rendering and Visibility

- [x] Respect per-line discovery (`ML_MAPPED`) in normal automap mode.
- [x] Implement Computer Area Map behavior (`pw_allmap`): show unrevealed lines in gray.
- [x] Keep `LINE_NEVERSEE` hidden in normal and allmap modes.
- [x] Support `IDDT` cheat levels:
- [x] Level 1: reveal map lines.
- [x] Level 2: reveal things.
- [x] Draw one-sided lines as solid walls.
- [x] Draw secret doors (`ML_SECRET`) as normal walls unless cheat mode is active.
- [x] Draw teleporter lines (special 39) with teleporter color rule.
- [x] Draw floor-height delta two-sided lines with floor-change color.
- [x] Draw ceiling-height delta two-sided lines with ceiling-change color.
- [x] Draw two-sided lines with no height delta only in cheat mode.
- [ ] Do not add a special "open door" visual style; doors are drawn by geometry/flags.

## Controls and UX Parity

- [x] Support classic automap enter/exit flow (`TAB`) as optional compatibility mode.
- [x] Support follow toggle and non-follow pan semantics.
- [x] Support grid toggle.
- [x] Support mark and clear-mark controls.
- [x] Support big-map toggle behavior.
- [x] Show numbered marks similarly to vanilla.

## Entities and Multiplayer

- [ ] Show things only in `IDDT` level 2 mode.
- [ ] Draw multiplayer arrows/colors when multiple players are present.

## Compatibility Notes

- [x] Keep current rotating heading/follow-heading mode behind a "modern" toggle.
- [ ] Document that rotating map mode is source-port style, not vanilla Doom automap behavior.

## Current Implementation Notes

- Current default runtime is walk/sim mode with automap rendering, not vanilla TAB-entered automap mode.
- Allmap unrevealed-gray behavior is available via `line-color-mode=parity` (runtime toggle `L`).
- Doom-style automap defaults/keybinds are mirrored: follow starts on, grid starts off, marks on `M`, clear on `C`, big-map on `0`.
- North-up map orientation is now default; heading-follow rotation is available as an opt-in toggle (`R`).
- Launch flag `-sourceport-mode` starts with heading-follow rotation enabled (source-port style), while default launch remains Doom-like north-up.
- Startup zoom now uses Doom-style auto zoom (`fit / 0.7`) unless `-zoom` is explicitly provided (>0).
- Non-Doom convenience controls are now gated behind `-sourceport-mode` (`R`, `B`, `O`, `I`, `L`, `HOME`, `P`, `J`, `K`, `U`, `Y`).

## Controls and Visual UX

- HUD now shows active profile (`doom` or `sourceport`) and current automap state (reveal, iddt, grid, marks, color mode).
- Walk mode uses the Doom-emulation 3D software renderer by default (not placeholder text).
- Help panel (`F1`) is profile-aware:
- Doom profile: only parity-safe controls listed; source-port extras explicitly marked disabled.
- Source-port profile: extra controls listed (`R`, `B`, `O`, `I`, `L`, `HOME`).

## Validation Plan

- [ ] Build E1M1 parity checks for: normal mode, allmap mode, iddt1, iddt2. (lower priority)
- [x] Add unit tests for line inclusion rules (`ML_MAPPED`, `LINE_NEVERSEE`, cheat/allmap gates).

## Sound Decode Track (Boot-Time In-Memory)

- [ ] Inventory sound lumps (`DS*`, `DP*`, etc.) and output a report.
- [ ] Decode DMX sound format in-memory during boot/startup.
- [ ] Print startup decode status message (count decoded, failures, skipped lumps).
- [ ] Add parser/decoder tests for malformed and valid sample lumps.
