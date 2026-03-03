# GD-DOOM Plan / TODO

Single source of truth for current priorities.

## Current Status

- Parser + validation milestone: done
- Automap baseline milestone: done
- Automap parity: mostly done, polish remaining
- Sound decode track: not started

## Next Up (Priority Order)

1. Extend startup sound import beyond `DP*` (PC-speaker) to additional Doom sound lump formats as needed
2. Improve startup sound report detail (sample rates, per-lump error summary)
3. Fix CLI map selection when `-map` is omitted (auto-select first valid map)

## Parity Polish (Lower Priority)

1. E1M1 acceptance checks across normal/allmap/iddt1/iddt2
2. Screenshot-based automap regression checks
3. Multiplayer arrow/color parity
4. Final review of any remaining non-Doom behavior in Doom profile

## Notes

- Implemented feature list lives in `docs/IMPLEMENTED.md`.
- Doom profile is the default runtime behavior.
- Source-port convenience behavior must stay behind `-sourceport-mode`.
- Detailed parity checklist lives in `docs/automap-parity-notes.md`.
