# Info: see desync-harness.md

# Demo Desync Status

Current stock-demo status after the recent `doom-source` parity work on monster walk-special handling and lost-soul death behavior.

Fresh no-render playback rerun on `2026-03-30`:

- `DOOM1 demo1`: completed, `tics=5026`, `map=E1M5`, `player_dead=true`
- `DOOM1 demo2`: completed, `tics=3836`, `map=E1M3`, `player_dead=true`
- `DOOM1 demo3`: completed, `tics=2134`, `map=E1M7`, `player_dead=true`
- `DOOM2 demo1`: completed, `tics=1205`, `map=MAP11`, `player_dead=true`
- `DOOM2 demo2`: completed, `tics=2001`, `map=MAP05`, `player_dead=true`
- `DOOM2 demo3`: completed, `tics=4471`, `map=MAP26`, `player_dead=true`

Those fresh runs used GD-DOOM's `-render=false` demo path only. The trace-compare sections below still reflect the latest full `doom-source` comparison runs, not a same-turn full retrace of all six demos.

## DOOM1

### demo1

- Status: clean
- Result: `traces match lines=4973`
- Fresh no-render replay: `completed tics=5026 map=E1M5`
- Notes: fixed the sector-71 normal-door thinker lifetime mismatch at `gametic 306`; the close-complete path now defers thinker retirement by one prune cycle, while open-only door completions still remove immediately.

### demo2

- Status: clean
- Result: `traces match lines=3778`
- Fresh no-render replay: `completed tics=3836 map=E1M3`
- Fixed in: `3ab5506`, `9570e96`
- Notes: still clean after the current chase/harness changes.

### demo3

- Status: clean
- Result: `traces match lines=2079`
- Fresh no-render replay: `completed tics=2134 map=E1M7`
- Fixed in: `2adb2c6`
- Notes: still clean after the later `DOOM2-DEMO3` fixes.

## DOOM2

### demo1

- Status: clean
- Result: `traces match lines=1063`
- Fresh no-render replay: `completed tics=1205 map=MAP11`
- Notes: fixed a real hitscan parity bug at `gametic 791`. A shotgun pellet was incorrectly seeing an imp through a blocking line because `doomPointOnDivlineSide` did not match Doom's shifted fixed-point side test in `PIT_AddThingIntercepts`. The fresh rerun is now fully trace-clean.

### demo2

- Status: clean
- Result: `traces match lines=1949`
- Fresh no-render replay: `completed tics=2001 map=MAP05`
- Fixed in: `2adb2c6`
- Notes: still clean after the current monster walk-special and lost-soul death fixes.

### demo3

- Status: desync
- Fresh no-render replay: `completed tics=4471 map=MAP26`
- All prior blockers fixed (false Cacodemon teleport on line 555, lost-soul death/gravity, Mancubus chase reacquire, etc.)
- Fresh full end-to-end compare run (2026-04-02) after the line-555 nil-candidate fix:
  - First mismatch: `line=3245 path=root.mobj_count ref=237 gd=238` at `gametic=3244`
  - GD-DOOM has one extra `MT_CHAINGUN` (type 73) in sector 37 at `(32177617, 29496857)` — a dropped weapon
  - The mobj first appears at `gametic 326` (dropped by a killed chaingunner); both ref and GD have it then
  - Ref reports `state=880 (S_MGUN), tics=-1`; GD-DOOM reports `state=4, tics=8` from spawn onward
  - At `gametic 3244` Doom removes it (player walks within pickup range, ~33 map units); GD-DOOM never removes it

- Root cause: `canTouchPickup` z check is correct — it mirrors Doom's `P_TouchSpecialThing` guard. The z check is not the bug.
- Traced the actual divergence: the dropped chaingun's `thingZState` in GD-DOOM stays at +8 units from drop (gt=326) all the way to gt=3215. In Doom, at gt=3244 the chaingun `z=-244` — it has been **riding the sector floor down** as a moving floor descends.
- GD-DOOM never updates the chaingun's `thingZState` as the sector floor moves. The floor under the chaingun in sector 37 descends from -8 to -244 over the course of the demo, but `heightClipAroundSector` is not firing for the chaingun's position, so it is left at +8 the entire time.
- At gt=3216 GD-DOOM marks the chaingun as collected via `heightClipThing` → the dropped-item crush path (`thingCollected[i] = true` when `tmceil - tmfloor < thingHeight`). That is also wrong behavior — Doom removes crushed dropped items via `P_RemoveMobj`, not by marking them collected.
- The floor at the chaingun's XY in sector 37 is a moving platform. `heightClipAroundSector` for that sector is either not being called, or the chaingun's blockmap cell falls outside the sector's block box so it is skipped.

## Next Issue

- Investigate why `heightClipAroundSector` does not update the dropped chaingun's `thingZState` as sector 37's floor descends
- Check whether the chaingun's blockmap cell is inside `sectorBlockBox(37)` and whether `heightClipThing` is ever called for it during the floor's descent
- Separately: the crushed-dropped-item path in `heightClipThing` marks the item `collected` instead of removing it — this should use removal, not collection, to match `P_ChangeSector` / `PIT_ChangeSector` behavior
