# Current verification checkpoint — 2026-10-01

The [documented exclusions](demos/COMPET-N-exclusions.md) list each confirmed original-engine glitch or crash outside the fix scope.

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

The latest complete quick report is `tmp/compet-n-continue-smoke-v96/summary.json`.
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
at `tmp/compet-n-continue-repository-v96/summary.json`; this replay includes the dependency updates.
The complete Go suite passes
after the latest corrections and the dependency updates from `d5ca1ac`, and
eight Python corpus checks pass. This checkpoint follows pushed base `4a8909d` and includes grounded-effect movement,
lethal-tic use ordering, commercial finales and newborn missile ordering.
**3,502** eligible recordings have completed
attempts; **1,132** have no completed attempt yet.
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

Stationary effects resting on the floor now skip Z movement as in
P_MobjThinker. Three respawn recordings match in full: MAP02 RE02-120
(2,914 tics), MAP02 RE02-147 (4,407 tics) and MAP05 RE05-315 (7,703 tics),
including gameplay RNG. A use command on a lethal tic now waits until
P_DeathThink on the following tic before requesting rebirth; MAP05 LV05-027
matches its 915-tic first-death comparison window and RNG (1,847 input tics).
Commercial finales now preserve the pending next map and carryover, freeze
level thinkers, and use the original held-button gate after finalecount 50.
Both long recordings now match state and gameplay RNG through their original
first-death boundaries: 2939fa01 (36,513 compared tics; 64,962 input tics) and
5355UV01 (57,967 compared tics; 133,445 input tics). The fresh v81 repository
replay also passes all 25 recordings. Seven of the nine short MAP06 follow-ups
passed on v81; the two remaining projectile floor-height failures are fixed
by running newborn missiles at their original thinker-list position before
later-created floor movers. NS06-113 and NS06-118 now match their complete
2,652- and 2,863-tic comparison windows and gameplay RNG on v83. The ordering
regression fails before the correction and passes afterward; the full Go suite
passes on v83. Five older MAP07 failures also pass their complete comparison
windows and gameplay RNG on v83; earlier corrections may contribute to those
passes. The latest complete quick refresh passes all 68 recordings, including state and gameplay
RNG. The fresh 25-recording repository replay also passes state and gameplay
RNG in the latest complete phase. The 15 previously failing gameplay frontiers
all pass on v83, including the 96,025-tic TY064534 comparison.

The three newly discovered gameplay failures pass on v86: MAP14 pa14-043
(1,804 tics), E2M5 N2M5-040 (1,701 tics), and MAP16 NM16-041 (1,888 tics),
including independent gameplay RNG audits. Corrections preserve a pain state
installed by self-damage during an attack, execute nested spawn/look/chase
actions before the next ordinary thinker, and remove the port's artificial
64-effect eviction so live effects reach their natural terminal state.
Focused regressions reproduce each failure; the full Go suite passes on v86.


The v89 fixes make pending monster attacks aim at the player's corpse height,
track original MF_COUNTKILL/MF_COUNTITEM totals and cumulative player counts,
preserve actor height clips when a closing door rolls back, and retain collision
actors spawned directly by the Icon of Sin under `-nomonsters`. All 11 targeted
ordinary failures pass complete comparison windows and independent gameplay RNG.
An additional NS11-135 recording also passes. Regression tests reproduce the
corpse-height, blocked-door and runtime-collision failures; item/kill counting and
nonzero save/keyframe round trips are verified. The full Go suite passes on v89.
Saves/keyframes advance to formats 26/13 for the four persistent count fields.
MAP20 LV20-115 also matches all 2,881 compared tics and gameplay RNG on v90:
a raised imp's RUN1 action returns after directly reacquiring the player,
without another chase probe. The v91 player physics correction applies the
original corpse friction exception while cached support differs from its own
subsector floor, including the death tic. The source-backed regression fails
before the correction and passes afterward; the full Go suite passes on v91.
All 14 ordinary gameplay frontiers in the v91 targeted batch pass complete
comparison windows and independent gameplay RNG; its remaining error is the
accepted original-reference E4M1 crash. The v91 controls are recorded as they
complete. The superseded v89 sweep retains 197 completed comparisons: 194
matches, two ordinary mismatches (now fixed), and one accepted original crash.
The v92 walk-trigger correction rejects a destination bounding box beyond a
line endpoint before testing side changes. MAP27 NS27-145's Mancubus fireball
previously crossed only the line extension and incorrectly activated two lifts.
The reproducing regression fails before the correction and passes afterward;
all 3,137 compared demo tics and gameplay RNG now match. The full Go suite passes
on v92. The v91 sweep was superseded with 169 completed results: 167 matches
and two ordinary mismatches. MAP27 is fixed. E3M1 N3M1TRY's reported blood ceiling height difference
at tic 4452 is caused by ordering otherwise identical blood effects at different
Z heights. The raw collections and original GDB show the new blood spawns at
23 units after the crusher quarters the player's height on a lethal hit; Go
used the living center at 44 units. The v93 correction reads the post-damage
player height. Its regression reproduces the old wrong height, and all 4,453
compared tics through first death now match state and gameplay RNG. The full Go
suite passes on v93. Both new target recordings match on v93.

The v94 correction preserves the shared floor/ceiling opening and float Z
adjustment left by a nested Lost Soul chase during a moving-sector height clip.
Original GDB confirms the nested P_TryMove call; regressions cover both a
successful step and a rejected step that still permits floating. E3M6 R3M6-214
now matches all 4,777 compared tics and gameplay RNG on v95.
The v95 correction makes a raised Arachnotron enter RUN1 before its initial
sight delay, executing A_Chase and reacquiring the target as original Doom does.
Regressions cover visible and sound-only reacquisition; original GDB records
RUN1 -> spawn -> sight at the failure tic. MAP23 RE23-207 now matches all 4,958
compared tics and gameplay RNG. Both regressions reproduce the old failures,
and the complete Go suite passes on v95.

The superseded v95 execution manifest freezes 4,602 queued recordings, 17 explicitly
verified current-runtime passes, and 15 accepted limitations, accounting for
all 4,634 eligible inputs. Later control passes overlap this frozen queue.
There were 1,706 never-tested inputs when the queue was selected.
The superseded v93 sweep retains 212 completed comparisons: 204 matches, six
strict mismatches, and two original-reference errors. Two ordinary mismatches
are fixed by the v94/v95 corrections above. Four E3M7 recordings were confirmed
as excluded platform-to-door structure corruption: manual line 352 (special 1)
treats sector 23's active T_PlatRaise as a door and writes direction=-1 into
platform.wait. GDB watchpoints, strict state failures and later RNG differences
are retained for each recording. Two additional E4M1 inputs crash the original
reference before GD-DOOM playback; their precise causes remain unclassified.
Those six accepted limitations were classified after the immutable v93 queue
was selected, so it still preserves its original 4,611 queued, 14 covered and
nine accepted partition. None of these excluded results is counted as a match.
The superseded v95 sweep retains 552 completed results: 539 matches, eight
strict mismatches, and five original-reference crashes. The two ordinary
mismatches are fixed on v96: direct chainsaw commands now select wp_chainsaw,
and a crushed barrel keeps its permanent S_GIBS timer rather than resuming its
old explosion. MAP28 RE28-243 matches all 5,873 tics and gameplay RNG;
MAP23 LV23-257 matches 2,012 compared tics through the original first death
(6,417 input tics) and gameplay RNG. Both reproducing regressions and the
complete Go suite pass on v96. Four additional E3M7 platform-to-door
corruptions, five original E4M1 crashes, and two E3M5 unused uninitialized
ceiling-field differences were verified separately. The current policy lists
26 exclusions; their raw mismatches/errors remain recorded. See
[the exclusion ledger](demos/COMPET-N-exclusions.md) for recording names and evidence.

The current v96 execution manifest freezes 4,594 queued recordings, 14 explicitly
verified current-runtime passes, and 26 accepted limitations, accounting for
all 4,634 eligible inputs. Later control passes overlap this frozen queue.
There were 1,154 never-tested inputs when the queue was selected.
The superseded v92 sweep preserves 30 completed state/RNG matches and its
4,610 queued, 15 explicitly covered, and nine accepted frozen partition.
The superseded v91 manifest preserved its 4,563 queued, 62 explicitly covered,
and nine accepted partition; its execution manifest was not edited.

A third E4M1 recording, r4m1-137, crashes the original reference during tracing
before GD-DOOM playback. That error is retained as an accepted limitation.

The old v86 sweep was superseded with 509 completed comparisons: 496 matches
and 13 strict mismatches. Its raw results and terminal receipt are retained.
The superseded v89 queue froze 4,585 recordings plus 42 verified current-runtime matches
and seven accepted limitations, accounting for all 4,634 eligible inputs.
Later control passes overlap the frozen queue. E1M4-111 was subsequently
classified as the same excluded platform-to-door structure corruption: GDB
shows manual line 564 overwriting platform.wait at tic 2157. Its retained
strict mismatch and matching gameplay RNG are not a passing comparison.

The obsolete v58 corpus batch was stopped with 452 completed comparisons;
its terminal receipt distinguishes it from a completed sweep. The v82 sweep
was also superseded after 257 completed comparisons (256 matches and one
original-reference error). The v84 sweep was superseded with 542 completed
comparisons: 535 matches, five mismatches and two original-reference crashes.
The superseded v86 queue contained 4,571 recordings, with 57 verified current-runtime matches
and six accepted limitations explicitly listed at selection time; together
they account for the original 4,634 eligible recordings. Later quick-set passes
can overlap this frozen queue. Four workers run the current v96 streaming comparisons.
Never-tested
recordings are first, followed by previous failures and older passes, including
recorded original-engine errors. New batches pin the in-place trace trimmer
as well as the harness and comparator; eight Python checks verify equivalent
comparison windows and immutable tool snapshots. In-place truncation avoids
copying multi-gigabyte trace prefixes, and GD-DOOM exits after writing the
original first-death tic. The runner now preserves manifest order through
filtering and executor submission; the scheduling regression fails before this
correction and passes afterward. The briefly started alphabetical v83 batch was
superseded with its three completed comparisons preserved.

The user explicitly excluded NoClip emulation and other extreme original-engine
glitches or crashes on 2026-10-01. Accepted limitations are recorded separately
in `evaluation_policy` in `demos/COMPET-N-results.json`, including original
reference crashes, E1M4/E3M7 door/platform structure corruption and E3M5's unused
uninitialized ceiling field, plus MAP27's timed door using uninitialized
destination/wait fields when reopened. A second E4M1 original crash was
classified after the immutable v86 execution manifest was selected.
Their raw strict mismatch/error statuses stay
intact and are not passing comparisons. Ordinary unexplained gameplay
differences and GD-DOOM/harness crashes remain in scope; the comparator is unchanged.

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
Save format is **26** and netplay keyframe format **13**; earlier formats are
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
