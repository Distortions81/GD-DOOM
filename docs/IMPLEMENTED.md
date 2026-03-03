# Implemented

Snapshot of features currently working in this repo.

## Parser and Data

- IWAD header/directory parsing (`IWAD` validation)
- Map marker discovery for `E#M#` and `MAP##`
- Required map lump bundle parsing:
  - `THINGS`, `LINEDEFS`, `SIDEDEFS`, `VERTEXES`, `SEGS`, `SSECTORS`, `NODES`, `SECTORS`, `REJECT`, `BLOCKMAP`
- Strict map validation (index/reference bounds and structural checks)
- CLI summary output and detailed parse output mode
- Startup sound import from `DP*` (PC speaker) and `DS*` (digital PCM) lumps (in-memory parse + status report)

## Renderer and Runtime

- Ebiten windowed automap renderer
- Doom-style startup zoom behavior (`fit / 0.7`) with `-zoom` override
- Doom profile default behavior (north-up orientation)
- Source-port startup profile via `-sourceport-mode`
- Walk/map mode toggle (`TAB`)
- Non-map placeholder screen (`no game render yet`)
- Doom-style door sound event wiring (`open/close/blaze`) with runtime playback from imported `DS*` lumps
- Level exit special handling with automatic next-map loading (normal + secret exits)

## Automap Features

- Follow toggle and map panning
- Grid toggle
- Big-map toggle
- Mark and clear marks with numbered markers
- Automap line visibility/style rules including:
  - `ML_MAPPED`
  - `LINE_NEVERSEE`
  - allmap unrevealed line handling
  - secret/teleporter/floor-change/ceiling-change handling
  - cheat gate for no-height-delta two-sided lines
- Runtime line discovery/mapping around player
- `IDDT` level 2 thing rendering path
- Typed thing glyph rendering (players/monsters/items/keys/misc)

## Control and UX

- In-app profile-aware HUD and help (`F1`)
- Source-port-only extra toggles gated behind `-sourceport-mode`
- Doom-style turn acceleration behavior (`SLOWTURNTICS` style ramp)

## Tests

- Unit + integration coverage for parser/validation flows
- Automap parity rule tests
- Discovery logic tests
- Turn acceleration tests
