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

- Root cause: `canTouchPickup` in `pickups.go:229-242` has the correct z check structure — it mirrors Doom's `P_TouchSpecialThing` (`delta > toucher->height || delta < -8*FRACUNIT`). The check itself is not wrong.
- The actual bug is that GD-DOOM's **player z is wrong** at gametic 3244: player floorz is reported as -248 units while the dropped chaingun sits at z=8 units in the same sector 37 — 256 unit delta, far beyond `playerHeight=56`
- In Doom the player z would be at or near the sector floor (8 units), making the delta ~0 and the z check pass; in GD-DOOM the player is 256 units below the sector floor it is supposedly standing on
- This points to a **player Z / floor-tracking bug**: the player's z or floorz is not being updated to the sector's current floor height, likely because sector 37's floor is a moving floor (lift/platform) and GD-DOOM is not updating the player's z as the floor moves

## Next Issue

- Investigate why player z is -248 units when standing on sector 37 floor at +8 units at `gametic 3244`
- Likely a moving-floor (lift/platform) tracking bug: sector 37 floor moves and the player z does not follow it up
- Check how GD-DOOM updates player z when the floor under the player changes mid-tic
