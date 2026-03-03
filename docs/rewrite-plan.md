# GD-DOOM Go Rewrite Plan

## Summary
Rewrite this repo into a Go codebase with phased delivery:

1. Milestone 1 (parser-first): robust IWAD loader + map lump parsing + strict validation + CLI inspection output.
2. Milestone 2 (immediate next): Ebiten desktop app to render map vectors (DOOM-style automap geometry).

## Decisions Locked
- Phased rewrite
- Desktop target via Ebiten
- Geometry-correct rendering first (not full automap styling in first renderer)
- Single IWAD input in Milestone 1
- Support both `E#M#` and `MAP##` map names
- Strict fail-fast validation
- In-repo WAD parser (no third-party parser dependency)
- CLI flags: `-wad`, `-map`
- Coordinate strategy: parse integers, convert to float64 in renderer only

## Repository Structure
- `cmd/gddoom/main.go`
- `internal/wad/{types.go,reader.go,errors.go}`
- `internal/mapdata/{types.go,loader.go,validate.go}`
- `internal/render/automap/{camera.go,geom.go,scene.go,ebiten_game.go}`
- `internal/app/{run_parse.go,run_render.go}`
- `docs/{m1-parser-spec.md,m2-automap-spec.md}`
- `testdata/` (synthetic malformed WAD fixtures)

## API Targets
### `internal/wad`
- `type Header struct { Identification string; NumLumps int32; InfoTableOfs int32 }`
- `type Lump struct { Name string; FilePos int32; Size int32; Index int }`
- `type File struct { Path string; Header Header; Lumps []Lump }`
- `func Open(path string) (*File, error)`
- `func (f *File) LumpByName(name string) (Lump, bool)`
- `func (f *File) LumpData(l Lump) ([]byte, error)`

### `internal/mapdata`
- `type MapName string`
- `type Vertex struct { X int16; Y int16 }`
- `type Linedef struct { V1, V2 uint16; Flags uint16; Special uint16; Tag uint16; SideNum [2]int16 }`
- `type Sidedef struct { TextureOffset int16; RowOffset int16; Top, Bottom, Mid string; Sector uint16 }`
- `type Sector struct { FloorHeight int16; CeilingHeight int16; FloorPic, CeilingPic string; Light int16; Special int16; Tag int16 }`
- `type Map struct { Name MapName; Things []Thing; Vertexes []Vertex; Linedefs []Linedef; Sidedefs []Sidedef; Sectors []Sector; Segs []Seg; SubSectors []SubSector; Nodes []Node; Reject []byte; Blockmap []int16 }`
- `func LoadMap(f *wad.File, name MapName) (*Map, error)`
- `func FirstMapName(f *wad.File) (MapName, error)`
- `func Validate(m *Map) error`

### CLI Contract
- `gddoom -wad <path> [-map <E#M#|MAP##>]`
- If `-map` omitted, auto-select first valid map marker.
- Output map name + counts per section.
- Non-zero exit on validation errors.

### Renderer Contract
- `func RunAutomap(m *mapdata.Map, opts automap.Options) error`
- `type Options struct { Width int; Height int; StartZoom float64; LineColorMode string }`

## Milestones
- Milestone 1 details: `docs/m1-parser-spec.md`
- Milestone 2 details: `docs/m2-automap-spec.md`
- Milestone 3+ (future): automap styling parity, multi-WAD overlays, gameplay systems.

## Current Tracking Notes
- Automap parity checklist: `docs/automap-parity-notes.md`
- Includes vanilla automap visibility/color rules (`ML_MAPPED`, `LINE_NEVERSEE`, `pw_allmap`, `IDDT`) and control parity notes.
- Also includes sound decode track notes for project-root output flow.

## Next Execution Order
1. Implement automap line visibility/color parity from `docs/automap-parity-notes.md`.
2. Add parity validation checks (E1M1 mode checks + focused logic tests).
3. Implement sound lump inventory and DMX-to-WAV decode flow in CLI.
