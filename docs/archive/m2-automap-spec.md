# Milestone 2 Spec: Ebiten Geometry-Correct Automap

## Goal
Render parsed DOOM map geometry (linedefs) in a desktop Ebiten window with correct world-to-screen projection and interactive pan/zoom.

## Scope
In scope:
- Ebiten desktop app path
- Map bounds computation
- Initial fit-to-window transform
- Pan and zoom controls
- Linedef vector drawing (`Linedefs -> Vertexes`)
- Simple debug overlay (map name, zoom, basic counts)

Out of scope:
- Full original automap styling and color semantics
- Fog-of-war / discovered line logic
- Gameplay logic

## Rendering Requirements
- Use parsed integer coordinates from `mapdata`.
- Convert coordinates to float64 for rendering math.
- Preserve map orientation consistently (define y-axis transform once and document it).
- Handle degenerate/zero-length lines without panics.

## Input/Controls
Minimum controls:
- Pan: arrow keys or WASD
- Zoom in/out: `=` and `-` (or equivalent)
- Reset view: `0`
- Exit: `Esc`

## Interface
- `func RunAutomap(m *mapdata.Map, opts automap.Options) error`
- `type Options struct { Width int; Height int; StartZoom float64; LineColorMode string }`

## Visual Acceptance
Manual checks:
1. `E1M1` renders non-empty geometry and visible level layout.
2. Camera pan/zoom remains stable and does not invert unexpectedly.
3. Different maps render successfully with correct extents.

## Next (Post-M2)
- Add line semantics for wall styling (one-sided, secret, height changes).
- Add PWAD overlay support.
- Expand toward gameplay systems.

## Follow-On Parity Doc
- Detailed vanilla parity checklist now lives at `docs/automap-parity-notes.md`.
- Use it as the implementation source for map discovery rules, cheat/allmap behavior, and control parity.
