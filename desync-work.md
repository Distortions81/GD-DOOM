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

**z drift on `mobjs[6]`** (`gametic=4296`):

- First mismatch: `line=4297 path=root.mobjs[6].z ref=-8912896 gd=-8650752` (delta = 4 units)
- Pattern: z drifts by 4 units per tick starting at tic=4297 (one tick behind)
- Context: player jumps down a large ledge into area with exploding barrels and a mancubus; a lost soul that was chasing the player follows it down
- Doom: lost souls descend via float logic in `P_ZMovement` (`MF_FLOAT`, `MF_NOGRAVITY`), not gravity — `z -= FLOATSPEED` when close enough to target below
- Suspect: `tickMonsterZMovement` not being called or float logic not triggering for the lost soul in GD-DOOM when momx/momy/momz are all zero
- Key path: `tickMonsterMomentum` → if `momx==momy==momz==0` and `z != floorZ` → calls `tickMonsterZMovement`; if stored `floorZ == z`, the call is skipped entirely
