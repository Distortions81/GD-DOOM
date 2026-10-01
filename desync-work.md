# Current verification checkpoint — 2026-10-01

The dedicated `desync-work` branch and expanded recovered-demo work are merged
into `main`, pushed at `18c31ad`. The earlier merged checkpoint passes all
25 repository recordings (69,398 compared tics) and the Go test suite.

**Broader external corpus:** the COMPET-N snapshot supplies 4,634 unique
single-player Doom I/II recordings across all 68 maps. Its initial quick
selection found 31 matches, 35 state mismatches, and two finale replay errors.
The latest complete quick phase plus targeted follow-ups now verifies
**68/68**, with strict gameplay RNG matching for every pass and no replay
errors. **0 recordings still fail**. See
[demos/COMPET-N.md](demos/COMPET-N.md) for preparation, fixes, and exact frontiers;
[demos/COMPET-N-results.json](demos/COMPET-N-results.json) records per-phase input
hashes and results.

The latest complete quick report is `tmp/compet-n-continue-smoke-v60/summary.json`.
The latest completed UV-Max phase is `tmp/compet-n-continue-combat-v34/summary.json`:
**67 matches and 1 state mismatch** including targeted
follow-ups. The quick and combat selections
cover 136 distinct recordings, with **135 latest verified passes and
1 state mismatch** across their recorded phases. The original E4M8
reference crash is retained separately; the active combat manifest substitutes
another E4M8 recording. A default E3M5 mismatch concerns an unused,
uninitialized reference ceiling field; its diagnostic matches all 7,004 tics
without changing comparator normalization.

The latest completed repository replay is **25/25**, with strict RNG matching,
at `tmp/compet-n-continue-repository-v75/summary.json`; this v75 replay includes the dependency updates.
The complete Go suite passes
after the latest corrections and the dependency updates from `d5ca1ac`, and
five Python corpus checks pass. This checkpoint adds tagged-door, teleport-order and slide-origin corrections
on top of `3d6bb39`.
**1,223** eligible recordings have completed
attempts; **3,411** have no completed attempt yet.
The full manifest remains unswept, and these passes do not establish universal
demo compatibility.

Recent targeted fixes preserve monster-only teleport triggers on player
crossings, reject dead projectile shooters before immediate damage-wake chase,
and wrap BSP coordinate differences like the original fixed-point arithmetic.
MAP08 Nightmare, E2M4 Nightmare and E4M8 Nightmare now match their complete
comparison windows and gameplay RNG. Teleport blocklinks traversal also clears
MAP24 UV-Max in full (9,312 compared tics and gameplay RNG).

Tagged doors now reject sectors with other active or stopped movers, clearing
MAP15 UV-Max in full (12,146 tics and RNG). Sector-first teleport destination
selection clears the fourth breadth batch's MAP06 Nightmare run (2,400 tics
and RNG). Using the nudged slide ray origin also clears all three early MAP03
movement failures in full.
E4M6's complete 6,622-tic replay also matches state and RNG; the fourth breadth
batch now has 272/272 verified matches.

Reference capture now streams through a pipe to bypass its 2 GiB file limit;
a long recording produced all 133,445 tics and 9,325,412,691 valid JSON bytes.
This is a capture check, not a passing port comparison. New batches use immutable
replay harnesses and comparators with recorded hashes. Two v74 repository checks
interrupted by a live harness edit were rerun successfully.

Continuation fixes include intermission command/transition continuity,
platform stop/reactivation, pickup and sector blockmap ordering, exact floor
texture/special inheritance timing, infinite death states and respawn provenance,
held-fire weapon switching, retained dead melee targets, damage-wake chase
ordering, targetless environmental damage, melee impact specials and punch puff
lifetime, SS wake/refire behavior, Commander Keen behavior, raised-floor
vertical movement, missile player/corpse ordering and sky-wall handling, pillar
autoaim identity, original arch-vile resurrection, and Cyberdemon damage-wake
chasing, original chainsaw lunge/reach and melee aiming, and blocked opening
door destination restoration, diagonal slide rounding, and unlimited missile
flight lifetime, original coordinate wraparound, and shared attack-range
state for Revenant puffs, zero-tic Revenant attack actions, and mutable
blockmap iteration during radius damage. MAP13 UV-Max now matches all
11,319 compared tics and gameplay RNG; preserving pending weapon switches
when the held weapon is selected also passes MAP08's full 5,163-tic comparison.
Crushed corpse collisions now respect S_GIBS immediately rather than the
previous death-animation phase, retaining raised ghost collision behavior.
MAP17 UV-Max now matches all 9,178 compared tics and gameplay RNG.
Crusher blood also tests actor collisions despite its MF_NOBLOCKMAP flag,
passing E3M4 and E4M7 UV-Max (9,285 and 5,752 tics). Revenant fist impacts
face once, passing MAP28 UV-Max (2,094 tics). Each replay matches gameplay RNG.
Charging skull collisions also follow original blockmap cell and link order
between monsters and the player, passing E4M5 and E3M2 UV-Max
(4,502 and 4,281 compared tics, including gameplay RNG).
Shared skull slams during A_Chase movement also pass MAP10 UV-Max
(10,048 compared tics and gameplay RNG). Non-player objects now respect
ML_BLOCKMONSTERS during pickup height clipping and blood movement.
Newly crushed bodies are re-clipped by later moving planes within the same tic.
Fast mode retains original walking steps and halves demon/spectre run,
attack, and pain states. Nightmare effects and missiles also report their
original zero spawn reaction delay, without changing comparator normalization.
Ammo pickups also double on the easiest difficulty and Nightmare, including
weapon supplies, dropped clips, and backpacks. Fast demon pain traces now
select the frame using the halved durations. Only the original three monster
projectile types accelerate in fast modes, and the generic post-attack chase
gate skips direction selection and movement in those modes. Both new UV-Fast
recordings now pass their complete traces and gameplay RNG. Repeated crusher
stop crossings also preserve the saved direction for the next restart.
Charging Lost Souls also slam during moving-sector height clips, clearing
the MAP11 Nightmare and new MAP06 speedrun recordings in full. The latest
Nightmare sweep and follow-ups have 68 of 68 matching recordings.
Missile floor impacts wait for the normal thinker after the spawn half-step,
and Arch-vile blasts preserve the fire's last linked subsector until A_Fire.
Floating height tracking also uses retained monster corpses, clearing MAP28
pacifist and E2M5 Nightmare in full. Missile death retains the local XY
split-step remainder, clearing MAP23 Nightmare; hitscan boundary nudges also
update the intersecting ray and impacts, clearing MAP25 Nightmare. Runtime
spawns now initialize crusher bookkeeping too, clearing E1M7 Nightmare and
two new E1M5/E4M8 recordings in full. Spider Mastermind damage wake executes
the immediate chase action, clearing MAP28 Nightmare in full. Intercept
arithmetic preserves fixed_t overflow too, clearing five MAP32 Nightmare
recordings without changing comparison normalization. Moving-sector height
checks follow relinked skulls through their new blocklinks, advancing MAP17
Nightmare's first state difference from tic 2,547 to 3,782. Generic monster
damage wake applies to all walking families with a see state, including the
Arch-vile's corpse search before ordinary chase bookkeeping. MAP17 Nightmare
now matches all 3,897 compared tics and gameplay RNG. Radiation-suit leakage
rolls every tic on 20-damage floors before the damage pulse check, advancing
MAP29 Nightmare from tic 1,761 to 1,951 and clearing MAP12 UV-Max in full
(8,275 compared tics and gameplay RNG). Nested skull reset/chase moves
preserve original shared probe state, clearing MAP29 Nightmare in full
(2,711 compared tics and gameplay RNG). The v63 Nightmare refresh is complete; newer follow-ups verify all 68 recordings. The completed v60 refresh
caught an E3M9 moving-floor regression: player-start markers had duplicate
physical links sharing the live player's order. Start markers now have no
map-mobj links, matching P_SpawnMapThing; its floor-step regression passes
and its full v65 replay matches all 2,411 compared tics and gameplay RNG. E2M6 diagnostics also verify that skull collision
cleanup re-reads Doom's shared current mover after a damage wake, resetting
the Demon victim while preserving the initiating skull's charge and vertical
momentum. E2M6 Nightmare now matches all 2,659 compared tics and gameplay RNG.
MAP23 UV-Max also passes in full (3,602 compared tics and gameplay RNG).
Charging skulls also collide with dead
barrels throughout their solid explosion animation, clearing E3M4 Nightmare
in full (1,922 compared tics and gameplay RNG).
Save format is **25** and netplay keyframe format **12**; earlier formats are
incompatible. Source and snapshot regressions pass.

The expanded follow-up work from `codex/desync-extra-demos` is also integrated
into local main, including fix commit `8831c48`. The corpus now
includes **25 distinct recordings**: the original 19, the additional
`lv26-237.lmp` speedrun from the MAP26 ZIP, and five recordings recovered from
branch history. Every unique LMP blob in available branch history and every
archived LMP has a byte-identical extracted representative. The eight original
UV-Max runs were already tested; the previous MAP26 input was the ZIP's
`lv26-239.lmp`, not its `lv26-237.lmp` recording.

The full suite inventories all **25 repository demos**, selects each matching
IWAD, builds the port and comparator, generates fresh original-game traces,
and checks gameplay RNG independently of the normalized state comparator.
It also rejects ZIP archives containing unextracted recordings:

```bash
xvfb-run -a env GOCACHE=/tmp/gddoom-go-cache python3 scripts/demo_trace_compare_all.py \
  --jobs 2 --out-root tmp/demo-trace-all
```

The original binary defaults to
`../doom-source/linuxdoom-1.10/linux/linuxxdoom`; override it with `--ref-bin`.
The runtime requires a display at initialization, even with rendering disabled.
The harness handles Xvfb. Traces end at the original replay termination or first
player death; a pass does not claim comparison of recorded tics after death.
Comparator passes ignore documented trace-only differences, including several
state/flag fields. Gameplay `prndindex` must also match at every compared tic.
Reports contain input hashes, individual logs, `summary.json`, and `summary.tsv`.

The completed expanded sweep passes **25/25 demos**, totaling **69,398 compared
tics**. Every normalized state comparison passes, and gameplay `prndindex`
matches at every compared tic. There are no known remaining divergences in
these repository replay windows. This is evidence for this demo corpus;
it does not prove universal compatibility for other demos or ignored fields.

The latest report is `tmp/desync-expanded-final/summary.json`, with a compact
`summary.tsv` alongside it. This fresh sweep includes all six recovered demos
and the additional fixes below. Generated traces and reports are not checked in.
The full Go test suite passes under Xvfb. Focused regression tests cover
arch-vile fire, projectile timing, pickup behavior, boss-brain spawn buckets,
plane clipping, player thinker order, delayed boss exit, missile subsector
links, lethal player thrust/RNG, and binary snapshot state.

The added MAP26 recording initially exposed two separate issues:

- At tic 2003, a blocked plasma spawn crossed a BSP partition with its directly
  advanced coordinates. Doom retains its original subsector link when
  `P_TryMove` fails. Missiles and their impact states now retain that link;
  successful moves update it, and snapshots preserve it.
- At tic 2971, the lethal hitscan came from more than 64 units below the player.
  `P_DamageMobj` consumes a random draw and may reverse and quadruple thrust
  before armor/death processing. The player path now matches this behavior,
  with explicit inflictor heights for projectile and radius damage.

The existing comparator's normalization was not loosened for either issue.

Integration was first verified in a temporary checkout, reported at
`tmp/desync-main-integration/summary.json`. The earlier merge into local main
also applied cleanly. Its source matches the validated integration checkout,
and its complete Go test suite and fresh repository replay sweep both pass.
The integrated fixes were committed and pushed before the external corpus work.

Corrections verified so far include projectile explosion thinker timing,
arachnotron impact frames, invisible-target RNG, revenant tracer targets,
repeating crusher timing, same-tic target reacquisition, slide-move pickups,
one-use triggers consumed while sectors are busy, projectile blockmap traversal,
lost-soul death flags and corpse collisions, duplicate backpack ammo, duplicate
plasma ammo, arachnotron death frame durations, arch-vile fire and blast thrust,
lost-soul spawn-fit checks, damaged cacodemon wake actions, and saturated
soulsphere/megasphere consumption. Player movement now follows its map-spawn
position in the ordered thinker list. Brain markers do not enter the blockmap.
MAP30 now models the original brain
wake timing, ordered cubes and teleport fire, immediate spawned-monster chase,
runtime actor queries, brain damage/pain, and death explosion/exit states.
Projectile order/countdowns, tracer targets, fire, and cubes are persisted.
The expanded fixes update save format version 20 to 21 and keyframe format
version 7 to 8 to preserve linked subsectors; older snapshots are not compatible.
The snapshot regressions and the complete Go test suite pass.

## Historical notes

The following notes predate this checkpoint; their old mismatch frontiers and
"clean" labels are superseded by the full-suite results above.

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
