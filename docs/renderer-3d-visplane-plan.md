# 3D Renderer Visplane Parity Plan

Goal: move `doom-basic` 3D walls/floors/ceilings from approximate per-pixel plane sampling to a Doom-style wall-marked, visplane/span-driven pipeline.

Reference implementation in this repo:
- `doom-source/linuxdoom-1.10/r_bsp.c` (`R_Subsector`)
- `doom-source/linuxdoom-1.10/r_segs.c` (`R_RenderSegLoop`)
- `doom-source/linuxdoom-1.10/r_plane.c` (`R_FindPlane`, `R_CheckPlane`, `R_MakeSpans`, `R_MapPlane`, `R_DrawPlanes`)

## Current Gaps

- No true 3D visplane buckets keyed by `(height, pic, light)`.
- Wall pass does not yet author full Doom-equivalent `floorclip[]/ceilingclip[]`.
- Plane raster still has approximation behavior at some angles/transitions.
- Sky handling is simplified (not full Doom sky plane behavior).

## Milestone 0: Instrumentation and Safety

- [ ] Add debug overlay for 3D plane bounds (`wallTop`, `wallBottom`, clip arrays when added).
- [ ] Add runtime toggle for old/new 3D plane path while parity is being tuned.
- [ ] Add counters: plane buckets, emitted spans, rejected spans, sky spans.

## Milestone 1: Wall-Driven Clip Arrays

- [ ] Introduce 3D `floorclip[]` and `ceilingclip[]` buffers for each column.
- [ ] Update wall render loop to mark these buffers per-column like Doom `R_RenderSegLoop`.
- [ ] Keep existing wall depth buffer for safety while migrating.

## Milestone 2: Plane Buckets and Span Build

- [ ] Define 3D plane key: `height`, `flat`, `light` (+ sky special marker).
- [ ] Build plane buckets from wall-marked clip ranges.
- [ ] Add `R_MakeSpans`-equivalent scanline transition logic for 3D planes.
- [ ] Ensure deterministic ordering and stable sentinel handling.

## Milestone 3: Span Raster (R_MapPlane-like)

- [ ] Render plane spans with per-row origin + x-step mapping (not per-pixel reprojection loops).
- [ ] Keep nearest + repeat texture sampling.
- [ ] Integrate light level lookup model (initially coarse, then Doom-like).
- [ ] Keep sky path separate (angle-based fetch semantics can be approximated first).

## Milestone 4: Sky and Edge Parity

- [ ] Implement dedicated sky plane draw path.
- [ ] Validate clipping at portals, doors, and height changes.
- [ ] Remove angle-dependent floor/ceiling pop artifacts.

## Milestone 5: Cleanup

- [ ] Remove legacy approximation path after confidence.
- [ ] Keep debug overlays behind source-port/debug profile toggles.
- [ ] Update docs/implemented notes.

## Test Plan

### Unit
- [ ] Clip transition tests from wall marks to plane spans.
- [ ] Span builder sentinel and no-negative-width guarantees.
- [ ] Plane key bucketing and ordering stability tests.

### Behavior/Visual
- [ ] Add golden images for known problematic scenes at multiple angles/distances.
- [ ] Add regression checks for “floor appears only when very close” cases.
- [ ] Validate detail levels (`320x200`, `640x400`, `960x600`) for consistency.

## Acceptance Criteria

- [ ] No angle-dependent floor/ceiling pop in current problematic areas.
- [ ] Floor/ceiling visibility stable across sector height transitions.
- [ ] Sky behavior no longer produces incorrect flat artifacts.
- [ ] `go test ./...` remains green.
