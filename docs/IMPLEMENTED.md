# Implemented

Current feature snapshot for this repo.

For launch flags, see [launch-params.md](/home/dist/github/GD-DOOM/docs/launch-params.md).

## Data and Startup

- IWAD loading, directory parsing, and validation.
- Map discovery for both `E#M#` and `MAP##`.
- Parsing and validation for Doom map lumps including `THINGS`, `LINEDEFS`, `SIDEDEFS`, `VERTEXES`, `SEGS`, `SSECTORS`, `NODES`, `SECTORS`, `REJECT`, and `BLOCKMAP`.
- CLI summary/detail output for parsed maps.
- Startup sound import/reporting for `DP*` and `DS*` lumps.
- Config-driven startup defaults via `config.toml` with CLI override precedence and runtime save-back for selected settings.

## Runtime and Rendering

- Ebiten desktop runtime with walk/map toggle (`TAB`).
- Doom profile as the default runtime behavior.
- Sourceport profile via `-sourceport-mode`.
- Doom-emulation software 3D renderer active in walk mode by default.
- 3D wall-driven clip plus visplane/span floor-ceiling rendering.
- Textured wall rendering for mid, top, and bottom wall sections.
- Deferred masked mid-texture rendering for two-sided lines.
- Door panel rendering and visible door ceiling motion in 3D.
- Doom `COLORMAP` sector lighting in the 3D path.
- Fullbright sprite handling.
- 3D detail presets (`320x200`, `640x400`, `960x600`) with runtime cycling.
- Optional Kage/CRT postprocess path.
- Experimental GPU sky path with sourceport-side defaults and selectable sky upscale mode.
- Doom-style aspect correction with a runtime launch override.

## Gameplay, World, and Progression

- Local player start selection with tracking for additional player starts.
- Doom skill selection with THINGS skill-flag filtering and game-mode spawn filtering.
- Level exits and secret exits with in-session map transitions.
- Item pickups for keys, health, armor, ammo, backpack, and weapons.
- Key inventory checks for locked doors.
- Hazard sector damage and radiation suit support.
- Player death state, death overlay, and in-session restart.
- Screen flash feedback for damage and pickups.

## Combat and Monsters

- Basic Doom-style combat foundation with pistol-like hitscan and ammo use.
- Monster health, death handling, and removal.
- Basic monster thinker loop with wake, chase, LOS checks, and cooldown-based attacks.
- Type-specific melee versus ranged monster attack behavior.

## Audio and Music

- MUS playback through the FM synth path.
- Doom-style door sound events.
- Doom-style world sound spatialization with distance falloff and stereo panning.
- Monster alert, idle-active, pain, death, and core attack sound coverage.

## Automap, UI, and Controls

- Doom-style startup zoom behavior with `-zoom` override.
- North-up Doom profile automap defaults.
- Follow, pan, grid, big-map, mark, and clear-mark controls.
- Automap visibility/style rules for `ML_MAPPED`, `LINE_NEVERSEE`, allmap, secret doors, teleporters, floor-change, ceiling-change, and cheat-gated two-sided lines.
- Runtime line discovery around the player.
- `IDDT` level 2 thing rendering path with typed glyphs.
- Collected pickups hidden from thing rendering.
- Profile-aware HUD/help.
- Source-port-only extra controls and overlays gated behind `-sourceport-mode`, including heading-up automap, allmap/`IDDT` convenience toggles, legend display, and thing render mode cycling.
- In-session pause/options screens plus Read This and quit prompt frontend flow.
- BSP-based pseudo-3D visibility traversal.
- Doom-style turn acceleration.
- Sourceport texture animation crossfade generation.

## Demo and Trace Tooling

- Demo playback from Doom v1.10 `.lmp` files.
- Live demo recording to Doom v1.10 `.lmp` output.
- Per-tic JSONL demo-state tracing during demo playback.
- `cmd/demotracecmp` trace comparator for GD-DOOM versus reference traces.

## Tests and Validation

- Unit and integration coverage for parser and validation flows.
- Automap parity rule tests.
- Discovery logic tests.
- Turn acceleration tests.
- Projectile sprite mapping regression tests.
- Demo trace emission tests.
