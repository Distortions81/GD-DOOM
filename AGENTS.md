# GD-DOOM Agent Notes

## Known Regressions To Watch

- Startup/player location regression:
  Changes in renderer or camera code can accidentally start the run at the wrong location/orientation. Treat startup spawn position and facing as a regression check after render changes.

- Sky direction regression:
  Current issue: sky appears to move backwards relative to camera turn in walk view. Any sky rendering/parity work should verify horizontal panning direction against Doom behavior.

## Quick Verification After Render Changes

- Launch a known map (for example `E1M1`) and confirm initial player location/facing is correct.
- In walk view, rotate left/right and confirm sky panning direction matches expected Doom behavior (not reversed).
