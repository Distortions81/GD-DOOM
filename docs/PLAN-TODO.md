# GD-DOOM Plan / TODO

Single source of truth for current priorities.

## Current Status

- Parser + validation milestone: done
- Automap baseline milestone: done
- Automap parity: mostly done, polish remaining
- Sound decode track: not started

## Next Up (Priority Order)

1. Improve startup sound report detail (sample rates, per-lump error summary)
2. Expand sound parsing support for additional Doom-compatible lump variants as needed
3. Fix CLI map selection when `-map` is omitted (auto-select first valid map)

## Parity Polish (Lower Priority)

1. E1M1 acceptance checks across normal/allmap/iddt1/iddt2
2. Multiplayer arrow/color parity
3. Final review of any remaining non-Doom behavior in Doom profile

## Notes

- Implemented feature list lives in `docs/IMPLEMENTED.md`.
- Doom profile is the default runtime behavior.
- Source-port convenience behavior must stay behind `-sourceport-mode`.
- Detailed parity checklist lives in `docs/automap-parity-notes.md`.
- 2D floor visplane emulation checklist lives in `docs/floor-visplane-emulation-plan.md`.
