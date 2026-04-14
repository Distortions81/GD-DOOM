# Masked Mid Render Plan

This document describes a concrete replacement strategy for expensive masked midtexture rendering.

The immediate target is not perfect visual fidelity. The target is to make the common masked-mid shapes in Doom cheap enough that maps with many of them stay fast.

## Current Problem

Masked mids are currently expensive because the render path still does a lot of fine-grained work:

- per-column setup
- per-column visibility clipping
- per-pixel hole discovery
- per-pixel cutout coverage checks

This is especially costly when a masked midtexture covers many screen columns.

In `E1M1`, the common masked midtextures are:

- `BRNBIGL`
- `BRNBIGC`
- `BRNBIGR`

These are not thin fence textures. They are large masked architectural panels with holes and large opaque regions. That means the renderer should optimize for wide masked surfaces that are mostly solid, not just sparse grates.

## Design Goals

1. Make the common wide-panel masked mids substantially cheaper.
2. Also provide a separate cheap path for sparse fence or grate textures.
3. Accept simpler lighting for masked mids if it buys a clear speedup.
4. Keep ordering behavior stable enough that sprites and masked mids still compose correctly.
5. Keep the existing path as fallback until the cheap paths are proven.

## Non-Goals

- Exact per-column lighting parity for masked mids.
- A full renderer rewrite.
- Solving every masked texture case in the first pass.

## Proposed Direction

Add two alternate masked-mid draw paths:

1. A large-opaque-region path for wide architectural masked mids such as `BRNBIGL/C/R`.
2. A sparse-fence path for iron grates and similar textures with low opaque coverage.

Both paths should:

- use one shade for the whole side, or at most one shade per large screen chunk
- render from precomputed opaque metadata, not from per-pixel mask checks
- move work to segment-level or coarse-run-level where possible
- fall back to the current masked column renderer for unsupported cases

## Texture Shape Classes

The renderer should stop treating all masked mids as one category. For optimization purposes, split them into two broad shape classes.

### Class A: Large Opaque Regions

Traits:

- high opaque coverage
- large contiguous opaque runs in many texture columns
- transparent holes are important, but occupy less total area than the solid body
- examples: `BRNBIGL`, `BRNBIGC`, `BRNBIGR`

Optimization goal:

- avoid paying full per-column setup cost for a texture that is visually mostly a wall
- skip holes because metadata says they are empty, not because the renderer rediscovered them per pixel

### Class B: Sparse Fences / Grates

Traits:

- lower opaque coverage
- many narrow opaque runs
- transparency dominates visual area
- examples: iron bars, fence-like masked textures

Optimization goal:

- reject empty space very aggressively
- avoid touching screen pixels for transparent gaps
- use texture metadata to jump directly to the few opaque texel runs that matter

## Eligibility

Any alternate masked-mid path should only be used when all of the following are true:

- the linedef is rendering a two-sided middle texture
- `tex.alpha == 0`
- the texture has valid opaque-run metadata
- the texture does not require special handling that the cheap path cannot express

Then classify the texture:

- use the large-opaque-region path when opaque coverage is high enough, or when hole fragmentation is low enough, that the texture behaves like a mostly-solid masked panel
- use the sparse-fence path when opaque coverage is low and empty space dominates
- keep the current path for blended textures, ambiguous shapes, or cases where metadata is insufficient

The exact threshold can be tuned later. The first pass can use a conservative heuristic derived from `OpaqueRuns`, `OpaqueRunOffs`, or a precomputed texture coverage metric.

## Shading Strategy

Use a coarse lighting model for masked mids:

- default: one `shadeMul` and `doomRow` for the whole masked side
- optional: split once or a few times for very wide masked-mid ranges if needed

The current path already computes masked-mid shade once from sector light. The replacements should keep that simplification and avoid any per-column lighting work.

## Raster Strategy

The two alternate paths should share queueing and ordering, but differ in how they rasterize texture coverage.

### Path A: Large Opaque Regions

This path is for wide, mostly-solid masked panels.

For each visible masked-mid range:

1. Step across screen `x` using the existing wall projection.
2. Group consecutive screen columns into coarse runs where texture-column progression is coherent and clipping state is stable enough.
3. For each coarse run, iterate the relevant texture columns and their opaque runs.
4. Project each opaque texel run into screen-space vertical spans.
5. Fill those spans directly.
6. Fall back to the current column path if the clipping state changes inside the coarse run.

The important point is that the renderer should treat the texture as a mostly-solid panel with holes cut out of it.

### Path B: Sparse Fences / Grates

This path is for textures where most texels are transparent.

For each visible masked-mid range:

1. Step across screen `x` using the existing wall projection.
2. For each texture column `tx`, iterate only that column's opaque texel runs.
3. Project those runs into screen-space spans.
4. Skip all transparent gaps without any per-pixel hole discovery.
5. Prefer early reject tests for columns or spans that are fully clipped or already covered.

The important point is that this path is optimized for touching very little of the screen, because the texture itself is mostly empty.

## Clipping Strategy

The alternate paths should preserve the current clipping rules, but do the work at a coarser level when possible.

- Keep wall and masked occlusion semantics unchanged.
- Keep cutout coverage semantics unchanged.
- Prefer coarse reject tests before any fine per-column work.
- Path A should only fall back to per-column visible-span generation when clipping changes inside a coarse run.
- Path B can stay column-oriented, but should still avoid rediscovering transparency per pixel.

Masked mids should continue to render in their own pass and should not become sprite occluders.

## Implementation Phases

### Phase 1: Instrumentation

Add counters or timing around the current masked-mid path to measure:

- masked-mid segments submitted
- masked-mid columns visited
- visible spans generated
- large-opaque-path hits
- sparse-fence-path hits
- per-pixel fallback usage
- texture-shape classifications

This should confirm where the time is actually going before deeper changes land.

### Phase 2: Texture Classification

Add a lightweight texture-shape classification step:

- precompute or cache opaque coverage and fragmentation signals per wall texture
- classify masked-mid textures into large-opaque-region, sparse-fence, or fallback
- keep the heuristic conservative in the first pass

This phase should not change ordering or clipping behavior.

### Phase 3: Large-Opaque Draw Path

Add a masked-mid draw path for Class A textures that:

- takes a masked-mid segment or masked-mid cutout item
- uses one shade for the whole segment
- renders from opaque-run metadata
- groups adjacent screen columns into larger coherent runs
- falls back to the existing renderer if the cheap path cannot handle the case

This phase should change draw cost without changing queueing or ordering logic.

### Phase 4: Sparse-Fence Draw Path

Add a masked-mid draw path for Class B textures that:

- takes a masked-mid segment or masked-mid cutout item
- uses one shade for the whole segment
- iterates opaque runs directly per texture column
- aggressively rejects fully clipped or covered spans
- falls back to the existing renderer if the cheap path cannot handle the case

This phase should change draw cost without changing queueing or ordering logic.

### Phase 5: Coarse Grouping Refinement

Reduce repeated column work by grouping adjacent screen columns into larger runs when:

- texture-column progression is predictable
- occlusion state is stable enough
- the grouping does not break visible correctness

The first implementation can be conservative and may only apply to Class A. The point is to stop paying full setup cost for every single column on wide masked panels.

### Phase 6: Optional Queue Cleanup

If the alternate draw paths are good enough, evaluate whether masked mids should keep using the current cutout-item scheduling shape or move to a dedicated masked-mid queue item with more exact bounds.

This is a follow-up step, not required for the first performance win.

## Code Areas To Change

Primary renderer code:

- [internal/doomruntime/game.go](/home/dist/github/GD-DOOM/internal/doomruntime/game.go:5191)
- [internal/doomruntime/game.go](/home/dist/github/GD-DOOM/internal/doomruntime/game.go:7977)
- [internal/doomruntime/game.go](/home/dist/github/GD-DOOM/internal/doomruntime/game.go:8132)
- [internal/doomruntime/game.go](/home/dist/github/GD-DOOM/internal/doomruntime/game.go:6420)
- [internal/doomruntime/game.go](/home/dist/github/GD-DOOM/internal/doomruntime/game.go:6547)

Current tests that should stay relevant:

- [internal/doomruntime/masked_mid_test.go](/home/dist/github/GD-DOOM/internal/doomruntime/masked_mid_test.go:1)
- [internal/doomruntime/wall_column_test.go](/home/dist/github/GD-DOOM/internal/doomruntime/wall_column_test.go:135)

Additional tests to add:

- texture-shape classification tests for large-opaque and sparse-fence samples
- masked-mid segment draw tests that verify holes stay open in wide opaque panels
- sparse-fence draw tests that verify transparent gaps are skipped
- fallback tests for blended or unsupported masked textures

## Validation

The alternate paths should be considered acceptable if they meet all of these:

1. `BRNBIGL/C/R` masked mids render without obvious holes being filled incorrectly.
2. Fence or grate masked mids still preserve narrow transparent gaps.
3. Sprite ordering against masked mids does not regress in common scenes.
4. Wide masked panels spend much less time in per-column and per-pixel fallback code.
5. The fallback path still covers blended or unsupported masked textures.

## Known Risks

- Misclassification can send a texture to the wrong alternate path and reduce either quality or speed.
- Coarse grouping can introduce ordering artifacts on strongly slanted walls.
- Span projection can create off-by-one errors at texel boundaries.
- Over-aggressive clipping reuse can miss local occluder changes inside a run.
- Simplified shading may look flatter than the current path.

These risks are acceptable for the first pass as long as the fallback path remains available and the classification stays conservative.

## Recommended First Patch

The first patch should do the minimum work needed to prove the direction:

1. Add instrumentation for masked-mid cost.
2. Add conservative texture-shape classification for masked mids.
3. Add a constant-shaded large-opaque-region masked-mid path for `BRNBIG`-style textures.
4. Keep the current draw path as fallback.

## Recommended Second Patch

After the first patch proves the large-opaque path:

1. Add the sparse-fence masked-mid path.
2. Tune classification thresholds using instrumentation.
3. Expand tests to cover both texture classes.

That should give a measurable performance result before any queue redesign or broader renderer cleanup.
