# Large original-game demo corpus

Current eligible corpus: **4,646 recordings**. The original **4,634-input**
inventory and frozen execution queues remain unchanged. A full re-import of the
checksum-verified snapshot recovers **12 additional valid single-player demos**:
ten ZIP Implode entries and two ARJ archives named `.zip`. All original records
are identical and the expanded import has zero skipped entries. The importer
uses optional `7z`/`7zz` only for legacy formats, streams literal members to stdout,
and checks decoded size and CRC. All twelve Python corpus tests pass.

The recovered recordings have a separate follow-up manifest, prepared for the
same v101 runtime, 16 MiB isolated reference config, and 12 uncapped nice-19
workers on Comedy-SSD/Scratch after the current base sweep terminates.
Recovered replay progress: **0/12 completed, 0 strict state/RNG matches**.
They are pending comparisons until verified, and are never preclassified as
exclusions. Current accepted exclusions remain **44**. Full expanded-corpus
verification remains unfinished.
Evidence: `tmp/compet-n/archive-recovery-v101.json`; follow-up selection:
`tmp/compet-n-archive-recovered-v101/followup-v101.json`.


See the [documented exclusions](COMPET-N-exclusions.md) for confirmed original-engine glitches and crashes outside the fix scope.

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

## Verification — 2026-10-01

The initial quick sweep found 31 matches, 35 state mismatches, and two replay
errors. The latest complete quick sweep is `tmp/compet-n-continue-heap-smoke-v101/summary.json`.
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
**4634 recordings have completed attempts**, with
**0 without completed attempts**. The most recent completed combat phase
plus targeted follow-ups has **67 strict matches and 1 state mismatch**. Across the latest
quick results and that combat phase, **135 pass and 1 fail**.
Binary hashes and per-recording follow-up provenance distinguish the phases;
the combat phase can precede later quick-set fixes.

The original repository suite passes **25/25**, totaling **69,398 compared tics**,
in `tmp/compet-n-continue-heap-repository-v101/summary.json`. This completed replay uses the updated dependencies.
The complete Go suite passes after the
latest source corrections and the dependency updates integrated from `d5ca1ac`;
all eight Python corpus checks pass. This checkpoint follows pushed base `4a8909d` and includes grounded-effect movement,
lethal-tic use ordering, commercial finales and newborn missile ordering.
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

Three new ordinary gameplay frontiers now pass on v86: MAP14 pa14-043
(1,804 tics), E2M5 N2M5-040 (1,701 tics), and MAP16 NM16-041 (1,888 tics),
each with an independent gameplay RNG match. Arch-vile self-damage now preserves
the full new pain frame instead of decrementing it again during the attack
thinker. Resetting a demon after a skull collision executes the spawn state's
nested A_Look/A_Chase immediately, then allows its later normal thinker to
advance the newly installed frame, including Nightmare's one-tic run states.
Live blood, puffs, smoke, teleport fog and projectile impacts now expire through
their state sequences; the port's artificial limit of 64 no longer evicts
older active thinkers. Reproducing regressions fail before each correction
and pass afterward; the full Go suite passes on v86.


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
27 exclusions; their raw mismatches/errors remain recorded. See
[the exclusion ledger](COMPET-N-exclusions.md) for recording names and evidence.

The superseded four-worker v96 execution manifest freezes 4,594 queued recordings, 14 explicitly
verified current-runtime passes, and 26 accepted limitations, accounting for
all 4,634 eligible inputs. Later control passes overlap this frozen queue.
There were 1,154 never-tested inputs when the queue was selected.
The four-worker run was superseded with 46/46 completed state/RNG matches.
The eight-worker continuation uses the same immutable runtime and comparison
behavior. Its comparator was rebuilt after commit d42bce7, changing embedded
VCS metadata; its separately pinned harness differs only in that comparator
filename. Comparator source and the trimmer are unchanged. Its new frozen manifest queues 4,492
recordings, lists 116 explicitly completed current-v96 passes, and retains the
26 accepted exclusions: all 4,634 eligible recordings are accounted for.
There were 1,108 never-tested inputs at selection. Later results overlap neither
the initial covered set nor the exclusions; the old manifest and results remain
retained. The original 26-exclusion ledger and runtime fixes are published in `d42bce7`.
E1M4 R1M4-314 was subsequently verified as excluded structure corruption at
tic 6304: manual line 564 (special 27) treats sector 41's active platform
as a door and overwrites platform.wait from 105 to -1. Its strict mismatch
and later RNG divergence at tic 6450 remain recorded. The current policy now
lists 36 exclusions after later E3M5 unused-field and E4M1 crash diagnostics;
the frozen execution manifest retains its original 26.
That eight-worker process stopped after 59 completed results: 58 state/RNG
matches and the E1M4 exclusion. Its immutable queue and completed results are
retained. The user requested resumption at the lowest CPU priority; the new
nice-19 manifest queues 4,433 recordings, covers 174 verified current-v96
passes, and lists 27 exclusions, totaling all 4,634 eligible recordings.
It had 1,049 never-tested recordings at selection.
The nice-19 eight-worker run retained seven further state/RNG passes but was
suspended after UI stalls caused by disk I/O, then superseded with exit 143.
The user requested fewer workers. The superseded one-worker low-I/O manifest
queues 4,426 recordings, covers 181 explicit current-v96 passes, and lists
27 exclusions, preserving the full 4,634-input scope. It had 1,042 never-tested
recordings at selection. Its transient system service runs as dist with
verified kernel read/write limits; no global sysctl changes were applied.
An eight-worker continuation used a shared write cap of 59.7 MiB/s, selected
as 80% of the slower direct-write sample (74.6 and 75.1 MiB/s). It retained
one further state/RNG match, but UI freezes returned and kernel logs showed
NVMe WRITE timeouts. The user moved the SSD to another slot and rebooted;
prior replay services are now absent. Health and bounded post-move checks
showed no media errors or new write timeouts, with 35/40 C sensor readings,
but sustained concurrent stability remains unproven.
The superseded post-slot manifest queued 4,425 recordings, covered 182
explicit current-v96 matches, and retained 27 exclusions, totaling all 4,634
eligible inputs. It had 1,041 never-tested recordings at selection. The
one-worker run completed 27 further state/RNG matches before being stopped
at the user's request to move trace writes to another SSD.
The superseded v96 Comedy-SSD manifest queued 4,398 recordings, covered 209 explicit
current-v96 matches, and retains 27 exclusions, preserving the full 4,634
input scope. It had 1,014 never-tested recordings at selection. Eight workers
at Linux nice 19 write captures, results and logs under
`/media/dist/Comedy-SSD/Scratch/GD-DOOM/` without read or write bandwidth caps.
The SSD error guard monitors both drives and suspends only this replay group
if new drive errors appear. Runtime and strict comparison behavior are unchanged.
The v97 correction fixes E3M3 E3M3-330: a Lost Soul's slam raises
its floor support, but original P_ZMovement floats before floor clipping.
The old Go path clipped first. A reproducing regression fails on the old
path; the corrected regression and full Go suite pass. The original input
matches all 8,223 tics with independent gameplay RNG on v97. Its GDB proof
is retained in `tmp/compet-n-continue-e3m3-skull-floor-debug-v96/diagnostic-result.json`.
Uncommitted v98 also handles a recorded E3M2 save command: original G_Ticker
queues ga_savegame for one tic, and the trace now exposes that pending action.
MAP09 arachnotron plasma retriggers a completed lift; its new thinker must run
behind existing missiles instead of lowering the floor immediately. A synthetic
regression reproduces the old imp-shot floor support mismatch. The three inputs
match all 8,506, 8,223, and 8,567 compared tics respectively on v98, with independent
gameplay RNG matching. Reproducing regressions and the full Go suite pass.
Proof is retained in `tmp/compet-n-continue-save-skull-lift-frontiers-v98/summary.json`
and `tmp/compet-n-continue-map09-projectile-floor-debug-v97/reference-gdb.log`.
The superseded v96 Scratch sweep retains 444 completed results: 434 matches,
seven strict mismatches and three original reference crashes, plus incomplete
captures. The superseded v98 sweep retains 254 completed results: 248 matches,
five strict mismatches and one original reference crash. Its frozen partition
remains 4,529 queued, 71 current-v98 passes and 34 exclusions; all captures survive.
A scan of all 4,634 eligible command streams found one recorded save action and
ten pause toggles across four recordings. A v98 prefix comparison exposes the
first recorded-pause error in MAP29 LV29-632 at tic 1,531: original leveltime
stays 1,531 while Go advances to 1,532. The new replay control consumes pause
commands before ticking the world, continues the status face ticker while paused,
and freezes gameplay RNG and player damage/bonus decay. Pending player rebirth
runs before the pause command, matching the original reload order.
All four affected recordings pass on v100: MAP29 LV29-632 (14,407 compared tics),
E2M5 E2M5-655 (15,027), E4M1 090-R4M1 (23,533, stopping at first death), and the
complete E3M1 EP3-5532 episode (123,548). All independently audited gameplay RNG
matches. Reproducing regressions and the full Go suite pass. All 68 quick-selection
and 25 repository recordings also pass on v100, for 97 verified validation inputs.
The superseded v100 Scratch sweep queues 4,524 recordings, covers 72 explicit
current-build COMPET-N passes, and lists 38 exclusions, accounting for all 4,634.
There were 342 recordings without completed attempts at selection. Its user
service ran eight workers at nice 19 without bandwidth caps and guards both SSDs
against new drive errors. Crash-core storage is disabled to avoid extra writes.
The exclusion ledger now includes thirteen separately proven unused ceiling-field
cases, 17 original reference crashes, 13 invalid mover casts and one timed door
using uninitialized memory. Exclusions retain strict mismatch/error evidence and
never count as passes. The current policy has 44 exclusions after GDB proves
E3M7 T3M7-800 corrupts an active platform through a door cast at tic 1,580.
EP1-2220 likewise corrupts E1M4 sector 41 at tic 20,013, independently proven
with a GDB watchpoint; its raw strict mismatch and matching RNG remain recorded.
E3M5 T3M5-736 has a separately proven unused ceiling-field mismatch;
normalizing only that field matches all 17,113 tics and gameplay RNG.
E3M5 t3m5-827 independently matches 18,715 tics and gameplay RNG after the
same unused-field diagnostic. E3M5 t3m51230 likewise matches all 27,215
compared tics and gameplay RNG after normalizing only the unused field.
All raw strict mismatches remain recorded.
The frozen v100 queue retains its original 38-exclusion partition.
Full current-runtime corpus verification remains unfinished.
The v100 eight-worker service was stopped with all completed results and partial
captures retained after finding an ordinary MAP30 mismatch at tic 10,355.
The telefragged Pain Elemental had no target; original A_PainDie spawned three
stationary, angle-zero Lost Souls, while GD-DOOM's legacy player fallback
incorrectly launched them. v101 preserves their null target and idle spawn
state. The regression fails on v100 and passes on v101, and the full Go suite
passes. The v101 101-input validation includes the new MAP30 recording, three
save/skull/lift fixtures, four pause recordings, 68 quick fixtures and 25
repository demos. Validation: **101/101 complete state and independently audited RNG matches**.

The superseded v101 sweep used **12 workers**, nice 19, idle I/O scheduling,
no bandwidth caps, and all captures on Comedy-SSD/Scratch. Its frozen partition
queues **4,515** recordings, retains **76** current-runtime state/RNG passes
and lists **43** exclusions, totaling all **4,634** eligible recordings. There
were **15** recordings without completed attempts at selection. Its exact user
service is `gddoom-demo-sweep-v101-comedy-twelve.service`; resource and drive
guard receipts are retained with the sweep. Current runtime and
full-corpus progress are recorded in [COMPET-N-results.json](COMPET-N-results.json).
The complete current-runtime corpus remains unfinished.

The 305,330-input-tic E4M1 `t4m1long` demo initially fails before gameplay
because the original reference inherits a 2 MiB zone. The harness now writes an
isolated `reference.cfg` with the supported `mb_used 16` setting, records its
hash and zone size, and avoids reading or rewriting the user's `.doomrc`.
The runtime binary, original executable, comparator and trimmer are unchanged.
The eight Python corpus checks and shell syntax check pass. Full validation
requires 101 strict state/RNG matches plus the separately accepted reference
crash in the 102-input selection; completion is recorded in the machine report.
With the startup limit corrected, GDB in retail E4M1 reproduces the original
crash at tic 91,667 in `PIT_CheckLine`: a Demon chase indexes the eight-entry
special-line list using corrupted `numspechit=-150406339`. This is an accepted
original-engine crash with a raw error, and never a passing comparison.
A read-only boundary audit of 16 retained original-crash traces found no complete
first-death line before failure. There is no available complete comparison prefix
to recover from those captures; the explicit NoClip exclusion remains separate.
Evidence: `tmp/compet-n/reference-crash-boundary-audit-v101.json`.
A fresh build of the current application source exactly matches the immutable
v101 runtime used by the sweep. The source hashes and build receipt are retained
in `tmp/compet-n/current-tree-runtime-audit-v101.json`.
The frozen-partition audit checks each completed result against the input hash,
immutable tools and runtime, and independently audited gameplay RNG. Results
from the new harness also verify their saved isolated 16 MiB reference config.
Evidence: `tmp/compet-n/comedy-twelve-heap-partition-audit-v101.json`.
The v101 sweep stopped with 653 new strict matches, one startup error and all
partial captures preserved while the new reference config is verified. Those
completed current-runtime matches remain covered by the successor queue.

The resumed 12-worker heap-config sweep queues **3,861** recordings, retains **729** explicit current-runtime passes and **44** exclusions, totaling all **4,634** eligible inputs. Its unit is `gddoom-demo-sweep-v101-comedy-twelve-heap.service`. All 101 normal validation inputs match state and independently audited gameplay RNG under the isolated 16 MiB reference config; the separately proven E4M1 crash stays a raw error.
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
can overlap this frozen queue. Eight workers run the current v96 streaming comparisons at Linux nice 19,
with captures and results on Comedy-SSD/Scratch and no I/O bandwidth caps.
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
Compressed failures preserve the complete comparison window. On 2026-10-01,
disk cleanup removed 2,219 superseded or matching captures (303.9 GiB), while
preserving completed result records, logs, inputs, active replay directories,
and the latest unresolved captures. Historical `retained_trace_files` entries
describe what existed when a run completed; consult
`tmp/cleanup-receipt-2026-10-01.json` for the exact later removals.

Blood effects preserve movement, support, lookup state, visibility, and thinker
order through binary snapshots. Monsters also persist their original spawn
points for respawning. Save format is now **26** and netplay keyframe format
**13**; earlier formats 21/8, 22/9, 23/10, 24/11, and 25/12 are
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
It remains in the eligible inventory as an accepted original-engine limitation;
no complete comparison or full gameplay RNG audit is available. The backtrace is retained in
`tmp/compet-n-continue-map02-reference-crash/reference-gdb.log`.

On 2026-10-01 the user excluded NoClip emulation and other extreme original-engine
glitches or crashes from the fix scope. Work continues on ordinary gameplay
state and RNG differences. The machine-readable `evaluation_policy` lists each
accepted limitation and its evidence: the MAP02 and E4M8 bonus glitches, the
E4M1 original-reference SIGSEGVs, E1M4/E3M7 manual doors writing into active
platform structure, and E3M5's unused uninitialized ceiling field. E4M1's exact
crash causes are still unclassified. MAP27 LV27-045 also reopens a timed door
whose original constructor left `topheight` and `topwait` uninitialized;
allocator residue becomes a live door destination. It is an accepted original
memory glitch. The second E4M1 crash was classified after the v86 manifest was
saved, so that immutable selection still queues it. These recordings remain in the inventory
with their original strict mismatch/error statuses; none is counted as a match.
The comparator remains unchanged. New unexplained gameplay failures and
GD-DOOM or harness crashes still require investigation.

The phase-v42 Nightmare sweep predates the effect reaction metadata correction;
its later reruns record effect metadata, difficulty ammo, projectile speeds and
post-attack chase corrections separately. The third batch adds 272 previously
unattempted recordings, up to four shortest for every starting map, with all
skills and categories eligible. These samples extend coverage of the complete
4,634-recording manifest; they do not replace it.

| Selection / binary | Completed | State and RNG match | Mismatch | Status |
| --- | ---: | ---: | ---: | --- |
| current_comedy_eight_remainder_v100 / v100 | 327/4524 | 321 | 6 | interrupted |
| current_comedy_eight_remainder_v98 / v98 | 254/4529 | 248 | 5 | interrupted |
| current_comedy_eight_remainder_v96 / v96 | 444/4398 | 434 | 7 | interrupted |
| current_postslot_remainder_v96 / v96 | 27/4425 | 27 | 0 | interrupted |
| current_eightio_remainder_v96 / v96 | 1/4426 | 1 | 0 | interrupted |
| current_lowio_remainder_v96 / v96 | 0/4426 | 0 | 0 | interrupted |
| current_nice_remainder_v96 / v96 | 7/4433 | 7 | 0 | superseded |
| current_parallel_remainder_v96 / v96 | 59/4492 | 58 | 1 | interrupted |
| current_remainder_v96 / v96 | 46/4594 | 46 | 0 | superseded |
| current_remainder_v95 / v95 | 552/4602 | 539 | 8 | superseded |
| current_remainder_v93 / v93 | 212/4611 | 204 | 6 | superseded |
| current_remainder_v92 / v92 | 30/4610 | 30 | 0 | superseded |
| uv / v40 | 68/68 | 66 | 2 | complete |
| nightmare_baseline / v42 | 68/68 | 1 | 67 | complete |
| nightmare_effects / v43 | 68/68 | 18 | 50 | complete |
| nightmare_ammo_projectiles / v46 | 68/68 | 41 | 27 | complete |
| nightmare_current_v63 / v63 | 68/68 | 67 | 1 | complete |
| nightmare_current / v60 | 68/68 | 66 | 2 | complete |
| nightmare_split_death / v53 | 68/68 | 62 | 6 | complete |
| nightmare_latest / v47 | 68/68 | 56 | 12 | complete |
| remaining / v58 | 452/3817 | 373 | 76 | superseded |
| current_remainder_v82 / v82 | 257/4619 | 256 | 0 | superseded |
| current_remainder_v91 / v91 | 169/4563 | 167 | 2 | superseded |
| current_remainder_v89 / v89 | 197/4585 | 194 | 2 | superseded |
| current_remainder_v86 / v86 | 509/4571 | 496 | 13 | superseded |
| current_remainder_v84 / v84 | 542/4561 | 535 | 5 | superseded |
| current_remainder_v83 / v83 | 3/4585 | 3 | 0 | superseded |
| breadth_four / v50 | 272/272 | 241 | 31 | complete |
| breadth_three / v46 | 272/272 | 238 | 34 | complete |

The third breadth selection, with its recorded newer follow-ups, has
**272/272 strict matches**.
The fourth breadth selection, with completed targeted follow-ups, has
**272/272 strict matches**.
The full remaining-corpus job continues independently.

## UV-Max sweep and reference limitations

The latest completed 68-map combat phase, with newer targeted results, is `tmp/compet-n-continue-combat-v34/summary.json`.
0 remaining failures diverge in gameplay RNG. Later fixes can improve
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
reference field with zero matches all **7,004** tics. This reference artifact
is now an accepted limitation under the user-directed scope above. The latest
captures, diagnostic report, and default mismatch status are preserved. The comparator's normalization is
unchanged. The diagnostic is retained at
`tmp/compet-n-lowercrush-followup/DOOMU-E3M5-e3m5-308-d38186ff56de/unused-field-diagnostic.json`.

| Map | First state tic | Field | First RNG tic |
| --- | ---: | --- | ---: |
| E3M5 | 2343 | `root.specials[5].topheight` | matches |
