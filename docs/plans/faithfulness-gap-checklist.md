# Faithfulness Gap Checklist

Focused list of work still needed to match vanilla Doom behavior, plus clearly missing features.

## 3D Rendering Parity

- [x] Sky parity/direction issues verified fixed in current build.
- [x] Masked wall/door rendering issues verified fixed in current build.
- [x] Door-related floor/ceiling shading artifact verified fixed in current build.
- [ ] Finish near-camera projectile scale behavior so sprite size progression matches expected Doom look.
- [ ] Confirm fullbright handling across all relevant sprite classes (attacks, pickups, decorative lights).
- [ ] Add first-person weapon/player-arm rendering in walk view.
- [ ] Add vanilla-like view bob behavior for weapon/player view.
- [ ] Add optional HUD adjustment for DOS-like aspect ratio presentation.

## Visibility and Culling

- [ ] Tighten BSP-aware thing visibility so monsters/items are submitted only when truly visible.
- [x] Billboard visibility regressions currently not reproducible.
- [ ] Optional perf-only: evaluate blockmap-assisted broad-phase culling if profiling shows BSP culling is insufficient.

## Automap Parity Gaps

- [ ] Ensure thing rendering is strictly `IDDT` level 2 only.
- [ ] Add multiplayer arrow/color behavior parity when multiple players are present.
- [ ] Remove/avoid any non-vanilla "open door style" rendering in strict parity mode.
- [ ] Keep source-port-only behaviors fully gated behind `-sourceport-mode`.

## Gameplay/Simulation Gaps

- [ ] Add save/load system parity including quicksave/quickload flow.
- [ ] Implement thinker-system parity (tick order, scheduling, and per-thinker behavior consistency).
- [ ] Replace mirrored map `Thing` runtime state with Doom-style live actor/runtime-object state so movement, drops, AI, and demo sync all use the same fixed-point source of truth.
- [ ] Audit monster animation-state parity in all attack/death edge cases.
- [x] Implement correct monster pain-state behavior (state selection, timing/tics, and interruption rules).
- [ ] Add monster collision parity (blocking/push behavior against player/world/actors).
- [ ] Add explosive barrel gameplay parity (collision, damage, and explosion chain behavior).
- [ ] Validate door/special timing parity in additional maps beyond current smoke tests.
- [ ] Expand projectile behavior parity checks (spawn cadence, collision, and impact timing).
- [x] Cover Doom/Doom II linedef trigger semantics for normal IWAD gameplay specials.
- [ ] Validate exact mover behavior parity for floors/plats/ceilings/stairs/donut edge cases.
- [ ] Finish monster-only teleport activation parity and validate teleporter timing/destination details.
- [x] Animate switches after activation.
- [ ] Implement sector light effects parity (flash/pulse/strobe/flicker behavior).
- [ ] Complete hitscan parity with Doom (damage/spread/tracing behavior).
- [ ] Add projectile/hitscan knock-back parity.
- [x] Add bullet impact "puff/poof" effects for hitscan impacts.
- [x] Use sprite-based visuals for bullet puffs and blood effects (instead of temporary placeholders).
- [ ] Verify key/lock interaction parity across all key-required specials.
- [ ] Complete remaining monster AI sensory parity (target handoff and any remaining hearing edge cases).
- [ ] Validate full difficulty parity including Nightmare semantics.
- [ ] Validate and expose fast-monsters mode parity behavior.
- [ ] Add optional turbo mode for faster player movement.

## Controls and Weapon Handling Parity

- [x] Implement baseline Doom-style F-key bindings (help/detail/messages/quit + placeholders for save/load/quicksave/quickload/end game).
- [x] Add number-key weapon hotkeys with vanilla slot semantics.
- [x] Add next/previous weapon cycling (mouse wheel and/or dedicated buttons).
- [x] Add auto weapon-switch toggle behavior parity.
- [x] Add run toggle option parity.

## Audio and Music Parity

- [ ] Enrich startup sound import report detail (rates, formats, per-lump errors).
- [ ] Expand decode support for additional Doom-compatible sound variants if encountered.
- [ ] Refactor SFX playback around a worker/channel model so active sounds can be scheduled and updated cleanly without changing Doom-faithful defaults.
  See [sound-system-worker-plan.md](/home/dist/github/GD-DOOM/docs/plans/sound-system-worker-plan.md).
- [x] Decode and play MUS music format via the OPL3 music path.
- [x] Add weapon firing sounds and bullet impact sounds.
- [x] Add monster pain sounds.
- [x] Add projectile travel/launch sounds.
- [x] Add monster wake and idle/active sounds.
- [ ] Add remaining monster movement/state sounds where Doom uses them (`hoof`, `metal`, and similar action-triggered sounds).
- [ ] Verify per-event sound routing/timing parity (doors, switches, shots, impacts, monsters).

## Intermission and Progression Presentation

- [ ] Add main menu flow with demo-attract playback behind/around menu states.
- [ ] Add intermission splash/state flow.
- [ ] Add level-end text flow where applicable.
- [ ] Add intermission animation special cases.
- [ ] Add intermission sounds/music transitions.

## Validation and Regression Infrastructure

- [ ] Add repeatable visual parity captures for known problematic scenes (sky, masked walls, door transitions).
- [ ] Add E1M1 acceptance checks for normal/allmap/iddt1/iddt2 automap states.
- [x] Add IWAD map audit for malformed specials / risky parity data (`docs/map-audit.md`).
- [x] Startup spawn position/facing regression currently not reproducible.
- [ ] Keep sky panning direction as a required regression check after render changes.
- [ ] Maintain demo benchmark baselines for faithful and sourceport modes.
