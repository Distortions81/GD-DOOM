# Masked Mid Render Plan

This document describes a concrete replacement path for expensive masked midtexture rendering.

The immediate target is not perfect visual fidelity. The target is to make common binary masked midtextures cheap enough that maps with many of them stay fast.

## Current Problem

Masked mids are currently expensive because the render path still does a lot of fine-grained work:

- per-column setup
- per-column visibility clipping
- per-pixel hole discovery
- per-pixel cutout coverage checks

This is especially costly when a binary masked midtexture covers many screen columns.

In `E1M1`, the common masked midtextures are:

- `BRNBIGL`
- `BRNBIGC`
- `BRNBIGR`

These are not thin fence textures. They are large masked architectural panels with holes. That means the renderer should optimize for wide masked surfaces with binary coverage, not just sparse grates.

## Design Goals

1. Make common binary masked mids substantially cheaper.
2. Accept simpler lighting for masked mids if it buys a clear speedup.
3. Keep ordering behavior stable enough that sprites and masked mids still compose correctly.
4. Keep the existing path as fallback until the cheap path is proven.

## Non-Goals

- Exact per-column lighting parity for masked mids.
- A full renderer rewrite.
- Solving every masked texture case in the first pass.

## Proposed Direction

Add a new cheap binary masked-mid path with these rules:

- Use one shade for the whole side, or at most one shade per large screen chunk.
- Render from precomputed opaque runs, not from per-pixel mask checks.
- Move work to segment-level or coarse-run-level where possible.
- Fall back to the current masked column renderer for unsupported cases.

## Eligibility

The cheap path should only be used when all of the following are true:

- the linedef is rendering a two-sided middle texture
- the texture is binary masked, not blended
- `tex.alpha == 0`
- the texture has valid opaque-run metadata
- the texture does not require special handling that the cheap path cannot express

Everything else should stay on the current path.

## Shading Strategy

Use a coarse lighting model for masked mids:

- default: one `shadeMul` and `doomRow` for the whole masked side
- optional: split once or a few times for very wide masked-mid ranges if needed

The current path already computes masked-mid shade once from sector light. The replacement should keep that simplification and avoid any per-column lighting work.

## Raster Strategy

The cheap path should be driven by opaque runs, not by masked pixels.

For each visible masked-mid range:

1. Step across screen `x` using the existing wall projection.
2. Group consecutive screen columns into coarse runs where texture-column progression is coherent.
3. For each texture column `tx`, iterate that column's `OpaqueRuns`.
4. Project each opaque texel run into screen-space vertical spans.
5. Fill those spans directly.

The important point is that transparent holes should be skipped because the texture metadata says they are empty, not because the renderer rediscovers that per pixel.

## Clipping Strategy

The replacement should preserve the current clipping rules, but do the work at a coarser level when possible.

- Keep wall and masked occlusion semantics unchanged.
- Keep cutout coverage semantics unchanged.
- Prefer coarse reject tests before any fine per-column work.
- Only fall back to per-column visible-span generation when clipping changes inside a coarse run.

Masked mids should continue to render in their own pass and should not become sprite occluders.

## Implementation Phases

### Phase 1: Instrumentation

Add counters or timing around the current masked-mid path to measure:

- masked-mid segments submitted
- masked-mid columns visited
- visible spans generated
- opaque-run fast-path hits
- per-pixel fallback usage

This should confirm where the time is actually going before deeper changes land.

### Phase 2: Cheap Draw Path

Add a new masked-mid draw path that:

- takes a masked-mid segment or masked-mid cutout item
- uses one shade for the whole segment
- renders from opaque-run metadata
- falls back to the existing renderer if the cheap path cannot handle the case

This phase should change draw cost without changing queueing or ordering logic.

### Phase 3: Coarse Column Grouping

Reduce repeated column work by grouping adjacent screen columns into larger runs when:

- texture-column progression is predictable
- occlusion state is stable enough
- the grouping does not break visible correctness

The first implementation can be conservative. The point is to stop paying full setup cost for every single column on wide masked panels.

### Phase 4: Optional Queue Cleanup

If the cheap draw path is good enough, evaluate whether masked mids should keep using the current cutout-item scheduling shape or move to a dedicated masked-mid queue item with more exact bounds.

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

## Validation

The cheap path should be considered acceptable if it meets all of these:

1. `BRNBIGL/C/R` masked mids render without obvious holes being filled incorrectly.
2. Sprite ordering against masked mids does not regress in common scenes.
3. Binary masked mids spend much less time in per-pixel fallback code.
4. The fallback path still covers unsupported masked textures.

## Known Risks

- Coarse grouping can introduce ordering artifacts on strongly slanted walls.
- Span projection can create off-by-one errors at texel boundaries.
- Over-aggressive clipping reuse can miss local occluder changes inside a run.
- Simplified shading may look flatter than the current path.

These risks are acceptable for the first pass as long as the fallback path remains available.

## Recommended First Patch

The first patch should do the minimum work needed to prove the direction:

1. Add instrumentation for masked-mid cost.
2. Add a cheap constant-shaded masked-mid draw path for binary textures.
3. Use opaque-run metadata directly in that path.
4. Keep the current draw path as fallback.

That should give a measurable performance result before any queue redesign or broader renderer cleanup.
