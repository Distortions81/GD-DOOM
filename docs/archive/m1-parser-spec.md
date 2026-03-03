# Milestone 1 Spec: Strict WAD + Map Parser

## Goal
Implement a robust parser-first milestone that loads a single IWAD, parses map lumps, validates strictly, and reports map stats via CLI.

## Scope
In scope:
- IWAD header + lump directory parsing
- Map discovery for `E#M#` and `MAP##`
- Map lump bundle resolution and typed parsing
- Strict fail-fast validation
- CLI summary output

Out of scope:
- PWAD overlays
- Gameplay simulation
- Visual rendering (moved to Milestone 2)

## Required Map Lump Order (relative to map marker)
1. `THINGS`
2. `LINEDEFS`
3. `SIDEDEFS`
4. `VERTEXES`
5. `SEGS`
6. `SSECTORS`
7. `NODES`
8. `SECTORS`
9. `REJECT`
10. `BLOCKMAP`

## Data Rules
- Read little-endian binary data.
- Validate record-size divisibility for structured lumps.
- Trim name fields from 8-byte strings at NUL and trailing spaces.
- Keep parsed coordinates as integer types.

## Validation Rules (hard errors)
- Invalid WAD identification for M1 (must be `IWAD`)
- Truncated WAD header/directory
- Missing map marker
- Missing required map lump set
- Structured lump byte size not divisible by struct size
- Out-of-range references:
  - Linedef vertex indices
  - Linedef sidedef indices
  - Sidedef sector indices
  - Seg vertex/linedef indices
  - Subsector seg ranges
  - Node child indices where determinable

## CLI UX
Command:
`gddoom -wad <path> [-map <E#M#|MAP##>]`

Behavior:
- If `-map` omitted, choose first valid map marker.
- Parse + validate selected map.
- Print summary with map name and counts.
- Exit non-zero on any parse/validation failure.

## Tests
Unit:
- WAD header parse: valid, invalid id, truncated
- Directory parse: valid, truncated, invalid offsets/sizes
- Map discovery: both naming styles
- Lump decoders: endian correctness for each typed lump
- Validation: representative out-of-range and missing-lump cases

Integration:
- `DOOM1.WAD` + `E1M1` parses and validates
- no `-map` selects first map marker
- malformed fixture fails with non-zero exit

## Done Criteria
Milestone 1 is complete when:
1. Parser APIs are stable and tested.
2. CLI behaves per contract and returns correct exit codes.
3. Strict validation catches malformed map data deterministically.
