# GD-DOOM Plan / TODO

Single source of truth for current priorities.

## Current Status

- Parser + validation milestone: done
- Automap baseline milestone: done
- Doom-emulation 3D renderer: in progress (visplane path + textured walls + colormap lighting + masked mids landed, parity polish remains)
- Automap parity: mostly done, polish remaining
- Sound decode track: partial (startup decode/report is in, deeper compatibility pass pending)
- IWAD map audit: in place; no current actionable engine parity gaps from scanned Doom/Doom II map data

## Next Up (Priority Order)

1. 3D renderer parity correctness:
2. replace temporary per-pixel plane sector-light lookup with Doom-style visplane/lighting behavior
3. replace plane depth-buffer workaround with Doom-style visplane clipping/ordering semantics
4. projectile scale parity near camera (remaining edge-case tuning)
5. capture regression scenes and convert to repeatable visual checks
4. BSP-aware thing visibility/culling pass:
5. tighten thing/monster/item submission against BSP visibility windows (not just screen clip)
6. optional perf-only: evaluate blockmap-assisted broad-phase culling for dense scenes
7. performance track (high resolution):
8. profile 4K (`3840x2160`) in sourceport and faithful modes, keep profiles under `profiles/`
9. continue wall/visplane hot-loop reductions (especially column depth writes + span paths)
10. reduce per-frame allocations in hot draw paths via scratch reuse/prealloc
11. content parity + presentation polish:
12. verify animation timing parity after crossfade generation (no cadence drift)
13. replace mirrored map `Thing` runtime state with Doom-style live actor/runtime-object state (`x/y/z`, momentum, flags, thinker-owned state) to improve parity and demo determinism
14. keep startup spawn position/facing and sky-pan direction as mandatory regression checks after render edits
15. finish remaining monster sensory parity (target wake handoff / remaining hearing edge cases)
16. audio parity follow-up:
17. add remaining monster movement/state sounds (`hoof`, `metal`, similar action-triggered sounds)
18. verify per-event sound routing/timing parity (doors, switches, shots, impacts, monsters)
19. enrich startup sound report detail (rates/formats/errors)
20. implement the sound worker/channel refactor for request-driven playback and future sourceport travel-delay support
    See [sound-system-worker-plan.md](/home/dist/github/GD-DOOM/docs/plans/sound-system-worker-plan.md)
20. expand decode support for additional Doom-compatible variants if encountered

## Parity Polish (Lower Priority)

1. E1M1 acceptance checks across normal/allmap/iddt1/iddt2
2. Multiplayer arrow/color parity
3. Final review of any remaining non-Doom behavior in Doom profile

## Verification Gate (After Render Changes)

1. Launch `E1M1` and verify startup player location/facing is unchanged.
2. In walk view, rotate left/right and verify sky panning direction is not reversed.
3. Run demo benchmark pass and compare with recent profile baseline.

## Notes

- Implemented feature list lives in `docs/IMPLEMENTED.md`.
- Render mode contract lives in `docs/render-modes.md`.
- Doom profile is the default runtime behavior.
- Source-port convenience behavior must stay behind `-sourceport-mode`.
- Detailed parity checklist lives in `docs/automap-parity-notes.md`.
- Focused faithfulness/missing-feature checklist lives in `docs/plans/faithfulness-gap-checklist.md`.
- 2D floor visplane emulation checklist lives in `docs/plans/floor-visplane-emulation-plan.md`.
- 3D renderer visplane parity checklist lives in `docs/plans/renderer-3d-visplane-plan.md`.
