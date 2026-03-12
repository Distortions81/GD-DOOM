# GD-DOOM Actions

Single action list for the remaining work in this repo.

## Highest Priority

- Finish 3D renderer parity: remove remaining plane-lighting and depth-order fallbacks, fix angle/transition artifacts, and lock in visual regression scenes.
- Replace mirrored map `Thing` runtime state with Doom-style live actor/runtime objects.
- Finish thinker/tick-order parity needed for deterministic gameplay and future demo support.
- Tighten BSP-aware thing visibility so actor submission matches actual visibility.

## Gameplay and Simulation

- Add save/load parity, including quicksave/quickload flow.
- Close remaining monster parity gaps: animation edge cases, collision, sensory handoff/hearing, teleports, movers, and special timing.
- Complete combat parity for hitscan, projectile timing, knock-back, barrels, difficulty settings, fast monsters, and turbo.
- Finish weapon/psprite parity: raise/lower flow, pending weapon transitions, refire behavior, ammo checks, forced weapon switch, and remaining projectile weapons.
- Add Doom II super shotgun support.

## Rendering and Presentation

- Finish near-camera projectile scale behavior.
- Confirm fullbright handling across all relevant sprite classes.
- Add first-person weapon/player-arm rendering in walk view.
- Add vanilla-like weapon/view bob.
- Add optional DOS-like HUD/aspect presentation adjustments.
- Keep source-port-only rendering behavior gated behind `-sourceport-mode`.

## Automap and Visibility

- Add multiplayer arrow/color behavior.
- Remove non-vanilla open-door styling from strict parity mode.
- Add E1M1 acceptance checks for normal mode, allmap, `iddt1`, and `iddt2`, plus sourceport-only heading-up map behavior.

## Audio and Progression

- Enrich startup sound reporting and support additional compatible sound variants.
- Add remaining monster movement/state sounds and verify per-event routing/timing parity.
- Add a Doom-style software SFX mixer for faithful mode.
- Add a separate higher-quality sourceport mixer/worker path later.
- Finish frontend/demo-attract, intermission, and level-end presentation flow.

## Validation and Performance

- Add repeatable visual captures/goldens for floors, sky, masked walls, and door transitions.
- Keep startup spawn/facing and sky-pan direction in the renderer regression gate.
- Add deterministic replay coverage for weapon, movement, and RNG-heavy combat cases.
- Maintain benchmark/profile baselines for faithful and sourceport modes.
- Continue hot-loop/allocation reductions only where profiling shows real value.

## Supporting Docs

- What is already done: [IMPLEMENTED.md](/home/dist/github/GD-DOOM/docs/IMPLEMENTED.md)
- Launch flags and runtime controls: [launch-params.md](/home/dist/github/GD-DOOM/docs/launch-params.md)
- Mode split policy: [render-modes.md](/home/dist/github/GD-DOOM/docs/render-modes.md)
- Demo trace harness status: [doom-trace-harness-checklist.md](/home/dist/github/GD-DOOM/docs/doom-trace-harness-checklist.md)
- IWAD map-data audit: [map-audit.md](/home/dist/github/GD-DOOM/docs/map-audit.md)
