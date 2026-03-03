# GD-DOOM Plan / TODO

Single source of truth for current priorities.

## Current Status

- Parser + validation milestone: done
- Automap baseline milestone: done
- Doom-emulation 3D renderer: in progress (visplane + textured walls landed, parity polish remains)
- Automap parity: mostly done, polish remaining
- Sound decode track: not started

## Next Up (Priority Order)

1. 3D Doom renderer parity polish:
2. sky path parity (dedicated sky draw behavior)
3. masked mid-texture rendering on two-sided lines
4. remaining angle/pop edge cases with regression captures
5. Improve startup sound report detail (sample rates, per-lump error summary)
6. Expand sound parsing support for additional Doom-compatible lump variants as needed

## Parity Polish (Lower Priority)

1. E1M1 acceptance checks across normal/allmap/iddt1/iddt2
2. Multiplayer arrow/color parity
3. Final review of any remaining non-Doom behavior in Doom profile

## Notes

- Implemented feature list lives in `docs/IMPLEMENTED.md`.
- Render mode contract lives in `docs/render-modes.md`.
- Doom profile is the default runtime behavior.
- Source-port convenience behavior must stay behind `-sourceport-mode`.
- Detailed parity checklist lives in `docs/automap-parity-notes.md`.
- 2D floor visplane emulation checklist lives in `docs/plans/floor-visplane-emulation-plan.md`.
- 3D renderer visplane parity checklist lives in `docs/plans/renderer-3d-visplane-plan.md`.
