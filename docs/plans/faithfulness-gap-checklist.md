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
- [ ] Audit monster animation-state parity in all attack/death edge cases.
- [ ] Add monster collision parity (blocking/push behavior against player/world/actors).
- [ ] Add explosive barrel gameplay parity (collision, damage, and explosion chain behavior).
- [ ] Validate door/special timing parity in additional maps beyond current smoke tests.
- [ ] Expand projectile behavior parity checks (spawn cadence, collision, and impact timing).
- [ ] Implement missing specials (including lift behaviors).
- [ ] Implement teleporter special parity (activation, destination handling, and timing).
- [x] Animate switches after activation.
- [ ] Implement sector light effects parity (flash/pulse/strobe/flicker behavior).
- [ ] Complete hitscan parity with Doom (damage/spread/tracing behavior).
- [ ] Add projectile/hitscan knock-back parity.
- [ ] Add bullet impact "puff/poof" effects for hitscan impacts.
- [ ] Verify key/lock interaction parity across all key-required specials.
- [ ] Complete monster AI sensory parity (including sound/hearing-driven wake/alert behavior).
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
- [ ] Decode and play MUS music format.
- [ ] Add weapon firing sounds and bullet impact sounds.
- [ ] Add monster pain sounds.
- [ ] Add projectile travel/launch sounds.
- [ ] Add monster wake and idle/active sounds.
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
- [x] Startup spawn position/facing regression currently not reproducible.
- [ ] Keep sky panning direction as a required regression check after render changes.
- [ ] Maintain demo benchmark baselines for faithful and sourceport modes.
