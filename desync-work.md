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

Late Doom II UV Max batch sweep:
```bash
scripts/demo_trace_compare_batch.sh --wad ./wads/DOOM2.WAD --out-root /tmp/doom2-uvmax-late
```

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

## DOOM2 UV Max Late

### MAP21 — current status

The old early MAP21 desyncs are fixed. The first mismatch frontier moved through:

- `gametic=120`: exploding `MT_FATSHOT` impact position
- `gametic=176`: monster chase / `movecount` drift
- `gametic=356`: teleport-fog / source-height mismatch
- `gametic=476`: revenant attack-state timing
- `gametic=490`: player teleport + telefrag parity and downstream RNG drift

Current result:

- Trace now matches cleanly through at least tic `520`
- Full-demo first mismatch is now at `gametic=527`
- Current first mismatch:
  - `mismatch line=528 path=root.specials[1].crush`
  - Reference special:
    - `kind=floor sector=51 type=187022936 crush=332673568 direction=1 floordestheight=1048576 speed=262144`
  - GD-DOOM special:
    - `kind=floor sector=51 type=3 crush=0 direction=1 floordestheight=1048576 speed=262144`
- This is no longer a monster/player parity issue. The next blocker is special-thinker trace/state parity for the floor mover on sector `51`.

### MAP21 — fixes landed during this pass

- Exact-Doom shotgun-guy state numbers were corrected to Doom's real values:
  - spawn `207-208`
  - see `209-216`
  - missile `217-219`
  - pain `220-221`
- Player teleports now telefrag overlapping shootables at the destination, matching Doom's `P_TeleportMove` stomp behavior
- Telefragged victims now stay pinned on the teleport tic instead of carrying same-tic corpse momentum
- The tic-490 missing `P_Random` was traced to Doom's telefrag kill path, not teleport fog RNG
- Revenant death-state trace base was corrected to Doom's `S_SKEL_DIE1=345`

### MAP21 — useful verification points

- Full 520-tic checkpoint after the telefrag fixes:
  ```bash
  xvfb-run -a ./.tmp/gddoom-demotrace -wad ./wads/DOOM2.WAD -demo ./demos/doom2-uvmax-late/DOOM2-MAP21-UVMAX.lmp -demo-stop-after-tics 520 -trace-demo-state /tmp/map21-gd-520h.jsonl
  ./.tmp/demotracecmp -left /tmp/map21-reference-full.jsonl -right /tmp/map21-gd-520h.jsonl -ignore-transient-fx
  ```
- Expected result there: no content mismatch before the intentional length mismatch from stopping at tic `520`
- Current full-trace compare:
  ```text
  mismatch line=528 path=root.specials[1].crush
  left_gametic=527
  right_gametic=527
  ```
