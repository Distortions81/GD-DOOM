# Doom Visibility for Vector Renderers

This note records lessons from building an external vector-display renderer
from Doom map data and comparing its output with GD-DOOM. The important lesson
is that Doom is not a conventional polygonal 3D scene. Correct vector output
comes from preserving the decisions made by Doom's 2.5D renderer, rather than
turning map structures into an independent polygon mesh.

## Map topology is not final visibility

The WAD contains vertices, linedefs, sidedefs, sectors, BSP nodes, subsectors,
and SEGs. These describe the map and accelerate rendering, but they are not a
ready-made list of visible 3D polygons.

In particular:

- A linedef is shared map topology, while a SEG is an oriented fragment used
  by the BSP renderer.
- A sector provides floor and ceiling heights and surface properties; it is
  not a single convex floor polygon.
- A subsector is useful for BSP traversal, but its SEG list should not be
  treated as an arbitrary closed polygon and used as the final visibility
  mask.
- One sector can appear in several disconnected subsectors and screen regions.
- Floors and ceilings are emitted as screen-space visplanes, not triangulated
  world-space geometry.

Projecting every linedef, or reconstructing polygons from raw SEG starts,
produces characteristic errors: back sides become visible, lines cross empty
space, pillars show the wrong faces, and floor patterns disappear or bleed
through neighboring areas.

## Follow Doom's wall traversal

A compatible vector renderer should reuse the same high-level sequence as the
classic renderer:

1. Traverse the BSP front-to-back from the camera position.
2. Apply the equivalent of `R_CheckBBox` before visiting the back child.
3. Orient each SEG consistently and reject its back-facing copy.
4. Apply the equivalent of `R_AddLine` to decide whether a wall is solid,
   passable, or visually empty.
5. Maintain Doom's horizontal solid-column coverage while traversing.
6. Preserve visible fragments of a line instead of deleting the entire line
   when only part of it is clipped.

The bounding-box test must be conservative around the near plane. If a BSP
box straddles the camera, clip its projected bounds against the near plane
before deciding that the box is hidden. Rejecting the whole box because some
corners are behind the camera removes valid geometry near screen edges.

Two-sided lines need Doom's height and surface rules. Equal-height portals may
be visually empty, while a change in floor height, ceiling height, flat,
lighting, or middle texture can require a boundary. Treating every two-sided
line as either always visible or always invisible is insufficient.

## Floors and ceilings are visplanes

Floor and ceiling vector textures should follow the same visibility result as
Doom's plane renderer.

The robust model is:

```text
world-aligned vector texture
    -> conservative projection
    -> Doom wall and visplane rasterization
    -> per-pixel visible-plane ownership
    -> keep only runs owned by the matching plane
```

Generate a flat's vector pattern in world coordinates so it remains stable as
the camera moves. Conservative sector or subsector bounds are useful only for
limiting how many repeated pattern strokes are projected. They must not be the
final shape or visibility test.

During the Doom-style plane pass, record both nearest depth and the identity of
the visible floor or ceiling plane at each pixel. Tag each projected flat
vector with its source plane identity. A vector sample is valid only when:

- the matching plane owns that screen pixel; and
- the sample is not behind the stored depth.

Plane identity is ideally the actual visplane instance or its complete key,
including height, flat, lighting, and any other properties used when planes are
merged or split. A sector number is a useful conservative prototype tag, but
it can be too strict when Doom merges identical planes across sectors and too
broad when one sector is split into several visible plane regions.

This ownership test prevents a world-space stroke generated inside a broad
bounding rectangle from appearing in another sector that happens to have the
same depth. A depth-only buffer cannot distinguish those surfaces.

## Why polygon clipping failed

An early approach built a polygon from each subsector's SEG start vertices and
clipped repeated flat strokes against that polygon. It made several assumptions
that do not match the rendering problem:

- the extracted points form the intended closed boundary;
- the winding is consistent and sufficient for clipping;
- one reconstructed region represents the sector's visible floor or ceiling;
- world-space containment is equivalent to current screen-space visibility.

Even when a subsector is convex, those assumptions lose the information that
Doom computes during front-to-back traversal: solid-column clipping, portal
openings, plane marking, and overlapping visplane spans. The replacement is
not a more elaborate polygon reconstruction. The replacement is to consume
Doom's visibility result.

## Quantization and apparent flicker

Projection, depth rasterization, vector rasterization, and the FPGA output all
round continuous coordinates to integer pixels. Comparing only one exact
pixel can therefore hide a line on one frame and reveal it on the next.

A small conservative footprint works well for the final visibility test. For
example, test the 3x3 neighborhood around a vector sample and reject it only
when every plausible pixel is definitely occluded. Keep a modest depth epsilon
as well. This addresses rasterization disagreement without weakening the
scene-level visibility rules.

The tolerance must still respect plane ownership for floor and ceiling cues.
An unowned neighboring pixel should not make a flat vector visible in the
wrong sector.

## Vector-list and FPGA implications

Visibility should reduce work before the vector list reaches constrained
hardware, but correctness must not depend on silently hitting a global vector
cap. Structural lines should be ordered ahead of decorative wall and flat
cues, and any hardware tile or scanline limits should be explicit failsafes.

Useful invariants are:

- clip long vectors into visible runs; do not discard a vector merely because
  one endpoint is off-screen or hidden;
- keep world-space texture alignment deterministic between frames;
- perform visibility before duplicate suppression;
- do not use duplicate removal to conceal incorrect geometry;
- test moving camera traces, because static spawn views miss rounding and BSP
  boundary failures;
- compare host simulation and device output from the same serialized vector
  list before debugging the FPGA rasterizer.

For a framebufferless display, the visibility work belongs on the host or game
side. The FPGA should receive an already valid vector list and concentrate on
bounded binning, scanline rasterization, and output timing.

## Recommended integration direction

The most reliable long-term interface is to export visibility from the Doom
renderer itself, close to the points where walls and visplanes are accepted.
This avoids maintaining a second approximate renderer beside GD-DOOM.

For walls, export the clipped visible SEG ranges and their upper/lower wall
bands. For floors and ceilings, export the visible visplane spans together
with a stable plane key. Vector textures can then be projected or sampled only
inside those accepted regions. Camera pose export remains useful, but camera
pose plus raw WAD geometry alone is not enough to reproduce every renderer
decision reliably.

The general rule is simple: reuse Doom's visibility, then change how the
visible surfaces are drawn.
