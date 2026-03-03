# 2D Floor Visplane Emulation Plan

Goal: replace subsector polygon triangulation fill with a Doom-style clip/span plane pass so floor coverage is robust and closer to vanilla behavior.

Reference implementation in this repo:
- `doom-source/linuxdoom-1.10/r_bsp.c` (`R_Subsector`)
- `doom-source/linuxdoom-1.10/r_segs.c` (`R_RenderSegLoop`)
- `doom-source/linuxdoom-1.10/r_plane.c` (`R_MakeSpans`, `R_DrawPlanes`, `R_MapPlane`)

## Scope and Non-Goals

- In scope:
- Doom-like floor/ceiling clipping model for automap 2D textured floors.
- Deterministic coverage with no missing subsector holes from polygon reconstruction issues.
- Test-backed parity for span generation and texture addressing.
- Not in scope (first pass):
- Full software-renderer lighting parity.
- Sprite clipping parity reuse.
- Replacing the existing pseudo-3D wall renderer.

## Milestone 0: Contract and Instrumentation

- [ ] Add a short architecture note in `internal/render/automap/game.go` near `drawMapFloorTextures2D`.
- [ ] Add debug toggle state for visplane diagnostics (`clip`, `span`, `both`) in automap HUD/debug label.
- [ ] Add counter metrics per frame:
- marked columns
- emitted spans
- rejected spans (degenerate/out-of-bounds)
- [ ] Keep old polygon path behind a temporary runtime toggle for A/B comparison.

## Milestone 1: Data Structures (`visplane-lite`)

- [ ] Add new file: `internal/render/automap/floor_visplane.go`.
- [ ] Define:
- `type floorPlaneKey struct { flat string; floorH int16; light int16 }`
- `type floorVisplane struct { key floorPlaneKey; minX, maxX int; top []int16; bottom []int16 }`
- `type floorSpan struct { y, x1, x2 int; key floorPlaneKey }`
- [ ] Define frame scratch buffers:
- `floorclip []int16` initialized to `viewH`
- `ceilingclip []int16` initialized to `-1`
- [ ] Add reset/init helpers called from `drawMapFloorTextures2D`.

## Milestone 2: Column Marking Pass (Doom-like)

- [ ] Add new file: `internal/render/automap/floor_mark.go`.
- [ ] Implement marking pass that consumes visible wall columns and writes per-column top/bottom ranges for floor planes.
- [ ] Start from existing wall projection math in `internal/render/automap/game.go` (`drawBasic3DView` path) to avoid duplicate projection bugs.
- [ ] Add helper:
- `markFloorColumnRange(plane *floorVisplane, x, top, bottom int, floorclip, ceilingclip []int16)`
- [ ] Explicitly clamp exactly once per column (`0..viewW-1`, `0..viewH-1`).

## Milestone 3: Span Builder

- [ ] Add new file: `internal/render/automap/floor_span.go`.
- [ ] Implement `makeSpans` equivalent to Doom `R_MakeSpans`:
- compare `top/bottom` at `x-1` vs `x`
- open and close runs per scanline
- [ ] Emit spans into a reusable `[]floorSpan` buffer (no per-span heap alloc in steady state).
- [ ] Validate sentinel handling at `minX-1` and `maxX+1`.

## Milestone 4: Span Raster and Flat Sampling

- [ ] Add new file: `internal/render/automap/floor_raster.go`.
- [ ] Implement textured span draw:
- compute world coordinates from screen/sample point
- map to 64x64 flat UV with repeat addressing
- nearest-neighbor sampling from `FlatBank`
- [ ] Keep debug modes:
- textured
- solid plane color
- UV visualization
- [ ] Preserve current `flatImage` cache behavior or replace with byte-level sampler if faster.

## Milestone 5: Integration and Toggle Cleanup

- [ ] Wire visplane pipeline into `drawMapFloorTextures2D` in `internal/render/automap/game.go`.
- [ ] Default to visplane path; keep legacy triangulation path behind hidden debug flag for one iteration.
- [ ] Remove temporary fallback path after parity confidence is reached.
- [ ] Update help text/HUD toggle labels in `internal/render/automap/game.go`.

## Milestone 6: Test Plan

### Unit Tests

- [ ] `internal/render/automap/floor_span_test.go`
- `TestMakeSpans_OpenCloseSimple`
- `TestMakeSpans_HandlesSentinelBoundaries`
- `TestMakeSpans_NoNegativeWidth`
- [ ] `internal/render/automap/floor_mark_test.go`
- `TestMarkFloorColumnRange_ClampsToClipBuffers`
- `TestMarkFloorColumnRange_RejectsInvalidTopBottom`
- [ ] `internal/render/automap/floor_raster_test.go`
- `TestFlatSample_Repeat64`
- `TestRasterSpan_DeterministicAcrossFrames`

### Regression/Behavior Tests

- [ ] Extend `internal/render/automap/map_texture_poly_test.go`:
- `TestFloorVisplane_NoHolesWithMixedDirectionSubsectorSegs`
- [ ] Add image tests in `internal/render/automap/map_texture_image_test.go`:
- `TestFloorVisplaneWritesDebugImage_E1M1LikeSlice`
- `TestFloorVisplaneWritesDebugImage_MultiFlatRegions`

### Golden Artifacts

- [ ] Add/refresh expected outputs under `internal/render/automap/testdata/`:
- `visplane_floor_debug.rgb(.txt/.png)`
- `visplane_floor_multitex_debug.rgb(.txt/.png)`

## Acceptance Criteria

- [ ] No black/unfilled interior holes in floor-rendered visible regions on current problematic map(s).
- [ ] All automap tests pass: `go test ./internal/render/automap`.
- [ ] New visplane unit tests pass and are deterministic.
- [ ] Visual diff against goldens shows only accepted changes.

## Implementation Order (Recommended)

1. `floor_visplane.go` + tests for data reset/sentinels.
2. `floor_span.go` + span tests.
3. `floor_raster.go` + repeat-address tests.
4. Integrate into `drawMapFloorTextures2D`.
5. Add/refresh golden images.
6. Remove temporary legacy path after validation.
