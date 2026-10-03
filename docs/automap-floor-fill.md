# Textured automap floor fill

The E1M3 normal exit room (sector 7) disappears from the textured automap
because its linedefs do not form closed rings by vertex index. Line 933 ends
eight units past the start of line 927, which lies on line 933. The old ring
tracer discards the unclosed outer chain and retains three inner outlines.
This removes the room and incorrectly treats those inner outlines as floors.

The minimap now scans directed original linedef sides directly. Crossings
change the winding count; intervals with nonzero winding receive the sector's
floor texture. Reverse sides of internal lines cancel. Endpoint IDs, ring
closure, and cached triangulation no longer decide whether the room exists.
Texture coordinates, lighting, and automap reveal policy remain unchanged.
Map geometry and gameplay BSP data are not modified.

This is a 2D fill fix. It does not establish complete 3D plane geometry for
maps with open sectors or rendering tricks. Fill stays between finite wall
crossings; a lone side does not imply an infinite textured half-plane.

## Source-port comparison

- [DSDA-Doom `gl_preprocess.c`](https://github.com/kraflab/dsda-doom/blob/master/prboom2/src/gl_preprocess.c):
  `gld_PrecalculateSector` tessellates sector contours with GLU, removing
  same-sector internal lines and duplicate lines. `gld_CarveFlats` and
  `gld_FlatConvexCarver` construct subsectors from ancestor BSP partitions and
  oriented SEGs. `gld_ProcessTexturedMap` requests subsector geometry for the
  textured automap; GL nodes provide complete subsector boundaries when present.
- [GZDoom map loader](https://github.com/ZDoom/gzdoom/blob/master/src/maploader/maploader.cpp):
  builds GL nodes for textured automaps while retaining original nodes for
  gameplay point queries. Its [render preparation](https://github.com/ZDoom/gzdoom/blob/master/src/maploader/renderinfo.cpp)
  handles open-sector rendering tricks, miniseg references, and holes.
- [glBSP specification](https://glbsp.sourceforge.net/specs.php): GL nodes add
  vertices and invisible minisegs so subsectors have complete, ordered, closed
  boundaries. Ordinary Doom SEGs omit implicit boundaries and some old node
  builds cannot be treated as exact convex polygons.

A local comparison with the DSDA-Doom carving approach restored the room but
also filled exterior space with this WAD's ordinary nodes. E1M3 subsector 44
contains one SEG and its BSP cell extends behind the diagonal wall: the point
(-509.125, -1915.25) belongs to that leaf despite lying outside the room's wall
boundary. Original BSP membership alone is therefore insufficient to validate
the textured floor's outline. A complete 3D solution should use proven GL-node
construction and rendering-sector handling rather than add acceptance
heuristics to the existing triangle cache.

## Validation

The real-map regression samples the exit room, pillar holes, and exterior at
three rotations. It also verifies texture sampling, reveal policy, and that
preparing the floor edges leaves the map data unchanged. Synthetic raster
tests cover junction overhangs, internal opposite sides, overlapping regions,
and the legacy even-odd ring input.

```sh
xvfb-run -a go test ./internal/render/mapview ./internal/doomruntime \
  -run 'Test(FloorDirected|FloorRing|AutomapE1M3ExitRoom)' -v

GD_GEOMETRY_WADS="$PWD/DOOM1.WAD,$PWD/wads/DOOMU.WAD,$PWD/wads/DOOM2.WAD" \
  xvfb-run -a go test ./internal/doomruntime -run '^TestAutomapBoundaryMaps$' -v

GD_AUTOMAP_INTEGRATION=1 GD_AUTOMAP_CAPTURE_DIR=/tmp/gddoom-automap-captures \
  xvfb-run -a go test -tags integration ./internal/doomruntime \
  -run '^TestAutomapE1M3Presentation$' -v
```

The opt-in audit checks bounded row interiors against independent winding
queries on original sidedefs across 77 local map variants. Open boundary rows
are counted separately because a nonzero winding on one side of an isolated
wall does not prove a bounded floor region. The framebuffer test exercises the
actual 1280 by 800 in-game minimap draw.
