# Large original-game demo corpus

The [COMPET-N public archive](https://compet-n.gamers.org/public/compet-n/)
provides a complete [2019-01-21 snapshot ZIP](https://compet-n.gamers.org/public/compet-n/compet-n_2019-01-21.zip).
Downloaded and inspected on 2026-09-30, it is 165,741,921 bytes (158 MiB).
Its SHA-256 is pinned in `scripts/fetch_compet_n.py`.
[Original COMPET-N rules](https://compet-n.gamers.org/index.php?page=compet-n_rules)
require the original DOS executables; the same page also describes the newer,
separate Competition Doom rules.

The filtered snapshot contains **4,634 unique single-player version-109 demos**:

| Game | Demos | Starting maps | IWAD |
| --- | ---: | ---: | --- |
| Ultimate Doom | 2,408 | All 36, E1M1–E4M9 | `wads/DOOMU.WAD` |
| Doom II | 2,226 | All 32, MAP01–MAP32 | `wads/DOOM2.WAD` |

The nine included categories are UV Speed, UV Max, Nightmare Speed,
Nightmare 100% Secrets, Tyson, Pacifist, UV Fast, UV Respawn, and No Monsters.
The unique command streams total 30,010,645 tics, or 238.18 recorded hours.
Category overlap is retained in provenance while byte-identical demos within
each game are deduplicated. Every ZIP's LMP members are examined, including
extra attempts beyond the recording named by the ZIP.

The importer excludes other IWADs/PWADs, co-op, multiplayer, built/miscellaneous
categories, and multi-level movies. It validates the header and end marker,
routes every Doom I episode to Ultimate Doom (the root `DOOM1.WAD` is shareware),
and generates safe filenames without extracting archive paths. Twelve eligible
archive entries are unreadable: two malformed ZIPs and ten recordings compressed
using methods unsupported by Python's ZIP reader. Their exact names and reasons
are saved in each manifest. This snapshot does not include newer submissions.

## Prepare and run

```bash
python3 -B scripts/fetch_compet_n.py
# Or reuse the downloaded snapshot:
python3 -B scripts/fetch_compet_n.py \
  --archive tmp/demo-pack-research/compet-n_2019-01-21.zip

xvfb-run -a env GOCACHE=/tmp/gddoom-go-cache \
  python3 -B scripts/demo_trace_compare_all.py \
  --manifest tmp/compet-n/smoke.json --jobs 2 \
  --discard-matching-traces --out-root tmp/compet-n-smoke
```

The downloaded archive, 4,634 extracted recordings, provenance, and manifests
live in ignored `tmp/compet-n/`; binary demo assets are not added to Git.
The first download already used `tmp/demo-pack-research/`; the second command
reuses that copy without downloading again.

Three manifests provide progressively broader selections:

- `smoke.json`: 68 recordings, one per starting map, preferring the shortest
  UV Speed recording. MAP30 falls back to another eligible category.
- `combat.json`: 68 recordings, one shortest UV Max run per starting map.
- `manifest.json`: all 4,634 eligible unique recordings.

Use `--manifest tmp/compet-n/combat.json` or `--manifest tmp/compet-n/manifest.json`
with a separate `--out-root` to run the larger selections. The existing 25-demo
repository suite still runs when `--manifest` is omitted. Each input hash is
validated before replay. Comparisons use the same state normalization and
independent strict gameplay RNG audit as the repository suite. Matches discard
their large JSONL traces only when requested; logs and results remain, and
mismatch traces are retained. `--compress-retained-traces` stores complete retained
traces as `.jsonl.gz` after the full comparison and RNG audit; it preserves all
tics. Decompress a trace with `gzip -dc TRACE.jsonl.gz > /tmp/TRACE.jsonl` to
inspect it with the comparator. Summaries are written to `summary.json` and
`summary.tsv` when the selected sweep finishes.
The runner also verifies the port trace's starting map. A trace replaced by an
attract demo after an episode finale is reported as `replay-error`, rather than
interpreted as a simulation mismatch against the requested map.

Recorded hours describe input duration, not an estimated replay runtime or
the number of tics successfully compared. Replay windows end at original
termination or first player death, and ignored comparator fields are unchanged.

Corpus tooling checks:

```bash
python3 -B scripts/test_demo_corpus.py
```

## Verification — 2026-09-30

The initial quick sweep found 31 matches, 35 state mismatches, and two replay
errors. The latest complete quick sweep is `tmp/compet-n-continue-smoke-v60/summary.json`.
Together with the newer targeted replays recorded in
[COMPET-N-results.json](COMPET-N-results.json), the quick selection now has
**68 strict matches and 0 state mismatches**, with no replay errors.
Every passing recording independently matches gameplay RNG at every compared tic.
These are results for particular recordings, not a count of independent bugs.

| Game | Quick-set match | State mismatch |
| --- | ---: | ---: |
| Ultimate Doom | 36 | 0 |
| Doom II | 32 | 0 |
| Total | **68** | **0** |

The complete 4,634-recording manifest has not been swept. The two selections
cover 136 distinct recordings, plus a separately retained reference crash.
Coverage also includes completed recordings from the new UV and Nightmare batches:
**1223 recordings have completed attempts**, with
**3,411 without completed attempts**. The most recent completed combat phase
plus targeted follow-ups has **67 strict matches and 1 state mismatch**. Across the latest
quick results and that combat phase, **135 pass and 1 fail**.
Binary hashes and per-recording follow-up provenance distinguish the phases;
the combat phase can precede later quick-set fixes.

The original repository suite passes **25/25**, totaling **69,398 compared tics**,
in `tmp/compet-n-continue-repository-v75/summary.json`. This completed v75 replay uses the updated dependencies.
The complete Go suite passes after the
latest source corrections and the dependency updates integrated from `d5ca1ac`;
all five Python corpus checks pass. This checkpoint adds tagged-door, teleport-order and slide-origin corrections
on top of `3d6bb39`.
The complete archive sweep remains in progress.

The reference trace now streams through a host pipe to avoid the original
32-bit executable's 2 GiB regular-file limit. A separate capture verified all
133,445 tics of a long recording: 9,325,412,691 bytes, with every line valid JSON.
This verifies reference capture only. Each new batch pins immutable copies of
its replay harness and comparator and records their hashes, so source edits
cannot change queued comparisons. Two interrupted v74 repository checks were
rerun successfully with the pinned harness.

Recent follow-ups reject tagged doors while another sector mover is active,
clearing MAP15 UV-Max in full (12,146 tics and RNG). Teleport destinations are
selected by sector index before Thing order, clearing the fourth breadth batch's
MAP06 Nightmare recording in full (2,400 tics and RNG). Slide rays also use the
nudged block-boundary origin for intercepts; all three early MAP03 movement
failures now match their complete comparison windows and gameplay RNG.
E4M6's full 6,622-tic replay also matches state and RNG after the same slide
correction, bringing the fourth breadth batch to 272/272 verified matches.

The source-aligned corrections cover:

- Demo command lifetime through episode finales, initial attack/use latches,
  and Doom I plasma/BFG availability outside shareware.
- Intermission command consumption and button edges, the queued `ga_worlddone`
  tic, and uninterrupted command stream, trace writer, RNG, inventory, and
  psprites through level loading.
- BSP node rounding, use-ray block-boundary nudging, reverse crossed-special
  order, all applicable crossings, and teleport clearing of remaining crossings.
- Door completion and restoration timing, and consumption of one-shot tagged
  door switches after activation.
- Partial collision openings, rejected plane restoration and reclipping, and
  floor minima seeded with the moving sector's own height.
- Pickup touches during `P_CheckPosition`, strict XY touch boundaries, and
  player/item ordering within the sector's blockmap walk. Later item height
  changes do not cause an extra speculative pickup scan.
- Platform stop/reactivation before sector-busy checks, immediate texture
  changes, correct preservation of sector damage effects, and texture-based
  raises using actual WAD texture heights.
- Lower-and-change floors inheriting their neighboring model's texture and
  special on completion; raise-24-and-change floors copying them immediately
  without restoring already discovered secrets at completion.
- Original blockmap/solid flags, dead Lost Soul decoration expiry, and blood
  and boss-brain pools staying outside the blockmap.
- Original light timers and RNG, boss-death actions on entering the final
  death frame, and E1M8 damage/exit ordering.
- Crusher damage on four-tic pulses, obstruction slowdown, blood movement/RNG,
  and the original lowerAndCrush ceiling action's `crush=false` flag.
- Infinite terminal death states and Nightmare/respawn timers, random gates,
  original map spawn points, teleporter fog, and reaction delays.
- Empty ready weapons waiting for a fire attempt before selecting a fallback;
  held-fire weapon switching retaining `attackdown` through `A_ReFire`.
- Retained dead targets in melee checks, melee-before-missile damage-wake
  ordering, and the SS soldier's immediate run action when damage wakes it.
- Environmental damage preserving an absent target. Pain recovery enters
  `RUN1` and executes its turn/reacquisition before returning to `A_Look`.
- Fist and chainsaw traces activating impact specials; punches start their
  puff in `S_PUFF3` after consuming the ordinary spawn RNG.
- Missiles visiting the player in the original blockmap link order alongside
  solid corpses, and retaining the earlier sky opening when a later solid wall
  rejects their movement. E3M9 now matches all 2,328 compared tics and RNG.
- Tall green pillars retaining their map thing identity, so they cannot attract
  autoaim or take barrel damage. MAP17 now matches all 4,475 tics and RNG.
- Arch-vile corpse search using the next step and blockmap order, original
  heal/raise timings, null revived targets, and crushed-corpse ghost dimensions.
  MAP29 now matches all 3,578 tics and RNG.
- Cyberdemons executing the initial chase action when damage wakes their first
  standing frame. E4M6 now matches all 3,310 tics and RNG.
- Chainsaw hits forcing the original next-tic forward command and disabling
  turn/strafe, single-ray melee aiming, and saw reach adding one fixed quantum.
  MAP01 UV-Max now matches all 1,571 tics and RNG.
- Opening doors restoring and reclipping blocked final snaps when their
  destination is below the current ceiling, while still completing the mover.
  MAP05 UV-Max now matches all 4,772 tics and RNG.
- Diagonal slide collisions using the original line-side rounding rather than
  the distinct path-traversal divline test. MAP13's first difference advanced
  from tic 1,459 to 2,500 before the missile lifetime correction.
- Missiles retaining their original unlimited flight lifetime; missed shots
  no longer create artificial timeout explosions or consume their extra RNG.
- Missile coordinates wrapping to signed 32-bit values before collision and
  BSP support lookup, including shots that travel far outside the blockmap.
- Revenant trail puffs using the most recent aiming or shooting range, as
  P_SpawnPuff does in the original engine. A preceding punch starts these puffs
  in S_PUFF3; the shared range survives saves, keyframes, and level changes.
- Revenant zero-tic attack startup actions facing the target before the visible
  windup action. Invisible targets consume both original pairs of spread RNG.
  MAP13 UV-Max now matches its complete 11,319-tic comparison and gameplay RNG.
- Explosion damage following mutable blockmap successors after each callback;
  waking and moving a monster can revisit earlier actors, as in original Doom.
  MAP08 now matches gameplay RNG throughout its comparison window.
- Selecting the held weapon preserving an already queued switch, including
  while its psprite lowers. MAP08 UV-Max now matches all 5,163 tics and RNG.
- Crushed corpses losing solidity on entering S_GIBS, even before their old
  death animation reaches A_Fall. This applies to walking actors, missiles,
  and charging skulls while preserving solidity on a raised ghost's next death.
  MAP17 UV-Max now matches all 9,178 compared tics and gameplay RNG.
- Crusher blood testing solid actors and the player during XY movement; its
  MF_NOBLOCKMAP flag controls registration rather than collision immunity.
  E3M4 and E4M7 UV-Max now match all 9,285 and 5,752 compared tics and RNG.
- Revenant fist impact facing an invisible target exactly once inside A_SkelFist.
  MAP28 UV-Max now matches all 2,094 compared tics and gameplay RNG.
- Charging Lost Souls visiting the player in original blockmap cell and link
  order alongside other actors. E4M5's first divergence hit a Spectre in Doom
  but incorrectly damaged the player in the port. E4M5 and E3M2 now pass
  all 4,502 and 4,281 compared tics and gameplay RNG. Regressions check
  both link orders within one cell and traversal across adjacent cells.
- Lost Souls retaining their charge collision during A_Chase movement after
  taking damage. Shared slam handling clears MAP10's full 10,048-tic comparison
  and independently matches gameplay RNG.
- Missile spawn half-steps checking the updated height without exploding
  at the floor until normal thinker movement, preserving impact order and RNG.
  MAP13 Nightmare matches all 2,680 compared tics and gameplay RNG.
- Arch-vile fire retaining its last linked subsector when A_VileAttack directly
  changes XY, until the next A_Fire action relinks it. MAP14 Nightmare and
  pacifist plus MAP17 speed pass their complete comparisons and gameplay RNG.
- Missiles retaining the remainder of a split XY move after exploding, using
  ordinary collision rules before their death-state tick. MAP23 Nightmare
  matches all 920 compared tics and gameplay RNG.
- Moving-sector height checks following current blocklinks after a skull
  relinks into another cell, including visits outside the initial sector box.
  MAP17 Nightmare's first state difference advances from tic 2,547 to 3,782,
  before the generic damage-wake correction clears all 3,897 compared tics
  and independently matches gameplay RNG.
- Intercept products, sums and coordinate differences wrapping to signed
  32-bit fixed_t values, matching original overflow on long sight rays. The
  five MAP32 Nightmare recordings pass complete comparisons and gameplay RNG.
- Every generic walking monster with a see state executing its immediate
  entry action when damage retargets its first idle frame. Arch-viles search
  for a raisable corpse before ordinary chase bookkeeping. MAP28 Nightmare matches all 1,693 compared
  tics and gameplay RNG.
- Radiation-suit leakage rolling every tic on 20-damage floors, before the
  32-tic damage-pulse check. MAP12 UV-Max matches all 8,275 compared tics and
  gameplay RNG. MAP29 Nightmare's first divergence advances
  from tic 1,761 to 1,951 before the nested movement correction clears it.
- Monster movement probes preserving original shared float state across
  nested skull reset/chase actions. A failed outer move uses the nested
  probe to adjust height and succeeds without an extra chase step. MAP29
  Nightmare matches all 2,711 compared tics and gameplay RNG. The v64 refresh
  also clears MAP23 UV-Max in full (3,602 compared tics and gameplay RNG).
- Player/deathmatch start markers having no map-mobj blockmap links, like
  P_SpawnMapThing. A duplicate player-start link shared the live player's order
  and could suppress its moving-floor height clip. A regression reproduces
  E3M9's tic-11 floor step. The v65 recording now matches all 2,411 compared
  tics and gameplay RNG, restoring all 68 Nightmare sample recordings across
  the latest complete sweep and follow-ups.
- BSP node tests preserving signed 32-bit subtraction before fixed-point
  multiplication. E4M8's escaped Imp fireball reaches Y=2,147,126,461;
  subtracting the root node's negative origin wraps in original R_PointOnSide.
  A regression uses the original root node, and E4M8 Nightmare matches all
  3,333 compared tics and gameplay RNG after the correction.
- Immediate damage wake-up checking whether a projectile's shooter is still
  shootable before continuing A_Chase. A dead Imp's projectile wakes an idle
  Baron in E2M4; original Doom clears threshold and returns to A_Look when no
  replacement is found. Regressions cover both idle fallback and direct player
  reacquisition without movement or extra RNG. E2M4 Nightmare matches all 1,819
  compared tics and gameplay RNG after the correction.
- Player crossings preserving monster-only teleport triggers, including W1.
  Original MAP08 leaves line 380 active when the player crosses it at tic 225;
  consuming it changed a Baron's blocked-move recovery at tic 256. Regressions
  cover W1/WR player rejection and normal monster consumption. MAP08 Nightmare
  matches all 1,103 compared tics and gameplay RNG after the correction.
- Teleport kills following original column traversal and mutable blocklinks,
  assigning randomized death timers to the correct overlapping victims. A
  regression includes shotgun drops changing the list during the kills. Both
  E4M1 Nightmare follow-ups match all 1,459 and 1,352 compared tics and gameplay RNG.
- Teleported corpses retaining the local split XY remainder after their stored
  momentum is cleared. A regression captures the original destination plus
  remaining displacement. MAP16 UV-Max's first divergence advances from tic
  1,552 to 2,568. Following original teleport blockmap traversal clears the
  remaining divergence: the full v71 recording matches all 5,970 compared tics
  and gameplay RNG. The same v71 teleport traversal also clears MAP24 UV-Max
  in full, with 9,312 compared tics and matching gameplay RNG.
- Lost Soul split XY movement dividing its first step toward zero while
  shifting the remainder, preserving original negative-odd fixed-point
  rounding. Both mixed-sign regressions pass, and E2M8 UV-Fast matches all
  1,554 compared tics and gameplay RNG.
- Chaingun second firing frames skipping the shot after the first frame used
  the last bullet. Doom still plays the sound without changing ammo, the player
  animation, muzzle flash, or gameplay RNG. MAP19 UV-Max matches all 10,750
  compared tics and gameplay RNG after this correction.
- Explosion callbacks visiting the player among monsters in original mutable
  blocklink order. Regressions cover both orders and their different randomized
  Lost Soul death durations. E2M8 UV-Max matches all 1,149 compared tics and
  gameplay RNG after this correction.
- Revenant pain recovery returning immediately after directly reacquiring
  the player, without an extra chase turn, move, or RNG calls. MAP06 UV-Max
  now matches all 5,067 compared tics and gameplay RNG.
- Skull collision cleanup re-reading the shared current mover after damage
  wakes a victim into a nested chase move. Original E2M6 tic 1,995 resets
  the Demon victim while preserving the initiating skull's charge and
  vertical momentum. Direct collision and XY movement regressions reproduce
  that result. E2M6 Nightmare now matches all 2,659 compared tics and gameplay RNG.
- Charging Lost Souls colliding with dead barrels throughout their solid
  explosion animation, consuming the slam damage roll even though the barrel
  takes no further damage. E3M4 Nightmare matches all 1,922 compared tics and
  independently matches gameplay RNG.
- Runtime-spawned monsters initializing crusher bookkeeping, so crushed
  corpses lose their physical bounds like map-spawned corpses. E1M7 Nightmare,
  E1M5 speed and E4M8 pacifist now match their full comparisons and gameplay RNG.
- Hitscan block-boundary nudges applying to the intercept ray and impact
  positions as well as cell traversal. MAP25 Nightmare matches all 1,511
  compared tics and gameplay RNG; regression uses original captured ray values.
- Floating monsters retaining height tracking toward already-targeted monster
  corpses, as original P_ZMovement does. MAP28 pacifist and E2M5 Nightmare
  pass all 1,825 and 1,161 compared tics and gameplay RNG respectively.
- Lost Souls processing charge slams during moving-sector height clips as
  original Doom does, including collisions with non-shootable objects above
  or below the skull. The MAP11 Nightmare and new MAP06 speedrun follow-ups
  match all 1,292 and 2,070 compared tics and gameplay RNG respectively.
- Pickups and blood respecting ML_BLOCKMONSTERS like other non-player objects.
  E4M6's first mismatch advances from tic 661 to 1,970; E2M2 advances from
  5,648 to 6,655, with gameplay RNG matching throughout the latter replay.
- Repeated crusher-stop crossings preserving the saved movement direction,
  so restarting a stopped crusher resumes the original thinker instead of
  leaving it frozen with a zero saved direction.
- Newly crushed corpses receiving a second height clip in the same tic when
  another moving plane reaches them, using their new zero-sized bounds.
- Fast mode retaining each monster's original walking step, speeding only
  demon/spectre run, attack, and pain frames alongside the original projectile
  rules. Spawned effects and missile traces also preserve Nightmare's zero
  initial reaction delay, rather than hardcoding the normal eight tics.
  Only Imp, Cacodemon and Baron/Hell Knight shots accelerate; Revenant,
  Mancubus, Cyberdemon and Arachnotron projectiles retain their original speeds.
  Both new UV-Fast demos now match completely (1,303 and 1,114 tics and RNG).
  The generic post-attack A_Chase gate skips new direction selection in fast
  modes, preserving movement counters, positions and gameplay RNG.
- Ammo supplies doubling on the easiest difficulty and Nightmare, including
  dropped clips, placed and duplicate weapons, and backpacks. Fast demon
  pain traces use their halved state durations when selecting the pain frame.
- Commander Keen's shootability, hanging/pain/death states, original damage
  mass and sounds, and tag-666 opening only after the last Keen dies.
- Tiny corpse momentum still performing vertical movement when the floor
  has risen above the actor's current Z.

Comparator normalization has not been loosened. The comparator reads large
traces one tic at a time; gameplay `prndindex` is audited independently over
the complete comparison window. Matches may discard traces after that audit.
Compressed failures preserve complete original evidence, rather than truncating
at the first mismatch.

Blood effects preserve movement, support, lookup state, visibility, and thinker
order through binary snapshots. Monsters also persist their original spawn
points for respawning. Save format is now **25** and netplay keyframe format
**12**; earlier formats 21/8, 22/9, 23/10, and 24/11 are
incompatible. Binary round-trip regressions pass.

## Remaining quick-set frontiers

Exact recording names, hashes, first state differences, and first RNG
differences are retained with their source phase in the machine-readable report.
MAP23 now matches all 1,329 compared tics after the environmental damage,
melee-door, and punch-effect corrections. MAP31 also matches all 1,322 tics after the SS damage-wake and JUSTATTACKED
corrections.

All recordings in this selection match state and gameplay RNG.

MAP17's tic-2,554 autoaim difference came from treating the map thing number
30 (tall green pillar) as the internal `MT_BARREL` enum number. The original
third fallback ray correctly aimed at a chaingunner; the port aimed at the
pillar on the second ray. The corrected type identity passes all 4,475 tics.

MAP29's tic-2,494 difference came from the approximate arch-vile resurrection.
Original `A_VileChase` scans around the prospective next step, tests terminal
corpses in blockmap order, then enters the vile's heal frames and corpse's raise
frames. The correction passes all 3,578 tics. Focused regressions also cover
rejected corpse thrust, delayed chasing, crushed ghosts, and snapshot restoration.

E4M6's tic-1,898 difference came from a damaged Cyberdemon remaining in its
first standing frame. Running the original immediate chase entry action now
passes all 3,310 tics. Each of these replays independently matches gameplay RNG.
A failing normalized list index need not identify the same actor on both sides
after sorting diverging positions.

## Additional corpus batches

The first new batch contains 66 normal UV recordings and two UV-Fast recordings,
with one previously unattempted recording per starting map. Its original ignored
manifest's selection note mistakenly says Nightmare; the actual headers are
one-based skill 4. The second batch explicitly selects skill 5 and contains
68 genuine Nightmare recordings. Per-phase results and live completion counts
are retained in `breadth_batch_progress` in [COMPET-N-results.json](COMPET-N-results.json).
The remaining-corpus MAP02 recording `DOOM2-MAP02-LV02-___-2e0d04c2fe92`
crashes the original Linux Doom reference at tic 2,251. GDB reproduces SIGSEGV
in `PIT_AddThingIntercepts` (`p_maputl.c:686`) during `A_FireShotgun2`.
It remains an unresolved eligible recording; no complete comparison or
full gameplay RNG audit is available. The backtrace is retained in
`tmp/compet-n-continue-map02-reference-crash/reference-gdb.log`.

The phase-v42 Nightmare sweep predates the effect reaction metadata correction;
its later reruns record effect metadata, difficulty ammo, projectile speeds and
post-attack chase corrections separately. The third batch adds 272 previously
unattempted recordings, up to four shortest for every starting map, with all
skills and categories eligible. These samples extend coverage of the complete
4,634-recording manifest; they do not replace it.

| Selection / binary | Completed | State and RNG match | Mismatch | Status |
| --- | ---: | ---: | ---: | --- |
| uv / v40 | 68/68 | 66 | 2 | complete |
| nightmare_baseline / v42 | 68/68 | 1 | 67 | complete |
| nightmare_effects / v43 | 68/68 | 18 | 50 | complete |
| nightmare_ammo_projectiles / v46 | 68/68 | 41 | 27 | complete |
| nightmare_current_v63 / v63 | 68/68 | 67 | 1 | complete |
| nightmare_current / v60 | 68/68 | 66 | 2 | complete |
| nightmare_split_death / v53 | 68/68 | 62 | 6 | complete |
| nightmare_latest / v47 | 68/68 | 56 | 12 | complete |
| remaining / v58 | 406/3817 | 337 | 66 | running |
| breadth_four / v50 | 272/272 | 241 | 31 | complete |
| breadth_three / v46 | 272/272 | 238 | 34 | complete |

The third breadth selection, with its recorded newer follow-ups, has
**272/272 strict matches**.
The fourth breadth selection, with completed targeted follow-ups, has
**272/272 strict matches**.
The full remaining-corpus job continues independently.

## UV-Max sweep and reference limitations

The latest completed 68-map combat phase, with newer targeted results, is `tmp/compet-n-continue-combat-v34/summary.json`.
Its remaining failure matches gameplay RNG. Later fixes can improve
these results, so reruns must use their recorded binary hash.

| Game | Combat match | State mismatch |
| --- | ---: | ---: |
| Ultimate Doom | 35 | 1 |
| Doom II | 32 | 0 |
| Total | **67** | **1** |

`DOOMU-E4M8-U48GLICH-bdc56641f6c7.lmp` crashes the original reference binary
before GD-DOOM is invoked. It remains eligible in the full manifest and is
classified as a reference error. The active 68-map combat selection uses the
next-shortest E4M8 UV-Max recording,
`DOOMU-E4M8-U4M8-219-be97b0d74754.lmp`. This replacement is stored in
`tmp/compet-n/combat-active.json`; the imported `combat.json` stays unchanged.

The E3M5 combat recording matches gameplay RNG throughout, while the default
comparator flags `ceiling.topheight`: `EV_DoCeiling` neither initializes nor
uses that field for lowerAndCrush. A diagnostic replacing only that unused
reference field with zero matches all **7,004** tics. The original traces and
default mismatch status are preserved. The comparator's normalization is
unchanged. The diagnostic is retained at
`tmp/compet-n-lowercrush-followup/DOOMU-E3M5-e3m5-308-d38186ff56de/unused-field-diagnostic.json`.

| Map | First state tic | Field | First RNG tic |
| --- | ---: | --- | ---: |
| E3M5 | 2343 | `root.specials[5].topheight` | matches |
