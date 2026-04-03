# Info: see desync-harness.md

## Quick Run Commands

No-render replay (just check it completes):
```bash
.tmp/gddoom-demotrace -wad DOOM2.WAD -render=false -demo ./demos/DOOM2-DEMO3.lmp
```

Full trace compare against doom-source (slow, finds first mismatch):
```bash
rm -f .tmp/gddoom-demotrace && go build -o .tmp/gddoom-demotrace . && scripts/demo_trace_compare.sh --wad DOOM2.WAD --demo-lump demo3 --demo ./demos/DOOM2-DEMO3.lmp --out /tmp/d2demo3-compare
```

Always delete the binary before rebuilding to avoid stale binary issues with the script's freshness check.

# Demo Desync Status

Fresh no-render playback on `2026-04-02`:

- `DOOM1 demo1`: completed, `tics=5026`, `map=E1M5`, `player_dead=true`
- `DOOM1 demo2`: completed, `tics=3836`, `map=E1M3`, `player_dead=true`
- `DOOM1 demo3`: completed, `tics=2134`, `map=E1M7`, `player_dead=true`
- `DOOM2 demo1`: completed, `tics=1205`, `map=MAP11`, `player_dead=true`
- `DOOM2 demo2`: completed, `tics=2001`, `map=MAP05`, `player_dead=true`
- `DOOM2 demo3`: completed, `tics=4471`, `map=MAP26`, `player_dead=true`

## DOOM1

All three demos: **clean** (traces match, no mismatch).

## DOOM2

### demo1 — clean
### demo2 — clean

### demo3 — desync (z drift)

- `mobj_count` matches throughout entire demo
- First mismatch: `line=4297 path=root.mobjs[6].z ref=-8912896 gd=-8650752` (~tic=4296, z drift of ~4 units on a monster)

## Next Issue

**Monster z-movement miss on state transition** (`gametic=4296`, type=18 zombie, sector 6):

- Both ref and GD start tic=4295 with `z=-132, momz=-8` (identical)
- At tic=4296: ref `z=-136` (moved by -4), GD `z=-132` (no change), both `momz=0` after
- State transitions 590→587 (`A_Fall`) during this tic
- Root cause: `checkPositionForActor` for the zombie returns `ok=true` and the line loop raises `tmfloor` above the true subsector floor, causing `tickMonsterZMovement` to snap `z = floorZ` instead of letting the monster fall
- Note: using `subsectorFloorCeilAt` unconditionally in `heightClipThing` fixes this but breaks monster step-up logic (monsters walk up ledges that are too large). Needs a targeted fix that only applies when the thing is actually blocked.
