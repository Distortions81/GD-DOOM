# 3D Renderer Visplane Parity Plan

Goal: move `doom-basic` 3D walls/floors/ceilings from approximate per-pixel plane sampling to a Doom-style wall-marked, visplane/span-driven pipeline.

## Status Snapshot

- Default (faithful Doom mode): `doom-basic` software 3D renderer is active.
- Implemented: wall-driven clips, visplane split/reuse (`R_CheckPlane`-like), span raster for floors/ceilings.
- Implemented: textured wall columns (mid/top/bottom).
- Remaining: sky parity, masked mid textures, and final edge/pop cleanup.

Reference implementation in this repo:
- `doom-source/linuxdoom-1.10/r_bsp.c` (`R_Subsector`)
- `doom-source/linuxdoom-1.10/r_segs.c` (`R_RenderSegLoop`)
- `doom-source/linuxdoom-1.10/r_plane.c` (`R_FindPlane`, `R_CheckPlane`, `R_MakeSpans`, `R_MapPlane`, `R_DrawPlanes`)

## Current Gaps

- Plane raster still has approximation behavior at some angles/transitions.
- Sky handling is simplified (not full Doom sky plane behavior).
- Masked mid-texture behavior is not yet Doom-faithful.

## Milestone 0: Instrumentation and Safety

- [ ] Add debug overlay for 3D plane bounds (`wallTop`, `wallBottom`, clip arrays when added).
- [ ] Add runtime toggle for old/new 3D plane path while parity is being tuned.
- [ ] Add counters: plane buckets, emitted spans, rejected spans, sky spans.

## Milestone 1: Wall-Driven Clip Arrays

- [x] Introduce 3D `floorclip[]` and `ceilingclip[]` buffers for each column.
- [x] Update wall render loop to mark these buffers per-column like Doom `R_RenderSegLoop`.
- [x] Keep existing wall depth buffer for safety while migrating.

## Milestone 2: Plane Buckets and Span Build

- [x] Define 3D plane key: `height`, `flat`, `light` (+ sky special marker).
- [x] Build plane buckets from wall-marked clip ranges.
- [x] Add `R_MakeSpans`-equivalent scanline transition logic for 3D planes.
- [x] Ensure deterministic ordering and stable sentinel handling.

## Milestone 3: Span Raster (R_MapPlane-like)

- [x] Render plane spans with per-row origin + x-step mapping (not per-pixel reprojection loops).
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
