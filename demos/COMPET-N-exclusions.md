# Demo compatibility exclusions

Updated 2026-10-01. The user excluded NoClip emulation and other extreme original-engine glitches or crashes from the desync fix scope.

**18 recordings remain excluded from fixes**: seventeen original-reference
crashes and one timed door that uses uninitialized memory. The v102 follow-up
strictly fixes all thirteen door/platform recordings. The thirteen unused
ceiling-field recordings now have an opt-in semantic comparison; their raw
strict results remain separate and semantic matches never count as strict
passes. All 26 affected recordings and all 101 normal validation fixtures have
completed state and independently audited gameplay RNG checks on v102.

The complete v101 baseline remains **4,602 strict passes plus 44 exclusions**.
A full v102 recheck is in progress; historical passes are not reused as current
runtime evidence. The authoritative current policy and per-input results are
in [COMPET-N-results.json](COMPET-N-results.json). Original policy and results
are preserved in `historical_evaluation_policies.v101` and
`v101_baseline_verification`.

Local evidence links point to ignored captures under `tmp/`. Historical tables
below retain the original trigger tics and raw evidence. New follow-up evidence
is [the 26-demo v102 replay](../tmp/compet-n-exclusion-followups-v102/summary.json)
and [the preserved nonzero ceiling captures](../tmp/compet-n-retained-ceiling-semantic-v102/summary.json).

## Original reference crashes — 17 recordings

These inputs crash the original Linux Doom reference during trace capture, before GD-DOOM playback. Their result stays **error**, and no complete state comparison or gameplay RNG match is claimed. The MAP02 NoClip recording has a GDB backtrace at tic 2251 in `PIT_AddThingIntercepts` during `A_FireShotgun2`; DOS overflow emulation is outside scope. E4M8 is another bonus glitch recording. The long E4M1 Tyson recording passes its startup allocation limit with a 16 MiB zone, then crashes at tic 91,667 in `PIT_CheckLine` during a Demon chase. GDB in the correct retail E4M1 mode shows an eight-entry `spechit` array indexed by corrupted value -150406339. The exact causes of the other thirteen E4M1 and one E1M5 crashes remain unclassified.

| Recording | Local evidence |
| --- | --- |
| `DOOM2-MAP02-LV02-___-2e0d04c2fe92.lmp` | [Evidence](../tmp/compet-n-original-overflow-diagnostics/diagnostic-result.json) |
| `DOOMU-E4M1-t4m1long-0b8ede500426.lmp` | [GDB evidence](../tmp/compet-n-continue-t4m1long-reference-retail-crash-debug-v101/diagnostic-result.json) |
| `DOOMU-E4M1-E4M1-216-b60348996d54.lmp` | [Evidence](../tmp/compet-n-continue-current-remainder-v95/DOOMU-E4M1-E4M1-216-b60348996d54/harness.log) |
| `DOOMU-E4M1-F4M1-243-2b049b0988fa.lmp` | [Evidence](../tmp/compet-n-continue-current-remainder-v95/DOOMU-E4M1-F4M1-243-2b049b0988fa/harness.log) |
| `DOOMU-E4M1-F4M1-256-43eeb8830de3.lmp` | [Evidence](../tmp/compet-n-continue-current-remainder-v95/DOOMU-E4M1-F4M1-256-43eeb8830de3/harness.log) |
| `DOOMU-E4M1-N4S1-058-e28ef8f766e5.lmp` | [Evidence](../tmp/compet-n-continue-current-remainder-v84/DOOMU-E4M1-N4S1-058-e28ef8f766e5/harness.log) |
| `DOOMU-E4M1-R4M1-152-bb8176145739.lmp` | [Evidence](../tmp/compet-n-continue-current-remainder-v93/DOOMU-E4M1-R4M1-152-bb8176145739/harness.log) |
| `DOOMU-E4M1-U4M1-228-164073eb719a.lmp` | [Evidence](../tmp/compet-n-continue-current-remainder-v95/DOOMU-E4M1-U4M1-228-164073eb719a/harness.log) |
| `DOOMU-E4M1-U4M1-232-3cd3abf3a131.lmp` | [Evidence](../tmp/compet-n-continue-current-remainder-v95/DOOMU-E4M1-U4M1-232-3cd3abf3a131/harness.log) |
| `DOOMU-E4M1-f4m1-223-43133ef22cd1.lmp` | [Evidence](../tmp/compet-n-continue-current-remainder-v93/DOOMU-E4M1-f4m1-223-43133ef22cd1/harness.log) |
| `DOOMU-E4M1-n4s1-037-2e9eeef6819b.lmp` | [Evidence](../tmp/compet-n-continue-current-remainder-v84/DOOMU-E4M1-n4s1-037-2e9eeef6819b/harness.log) |
| `DOOMU-E4M1-r4m1-137-56623bb117d6.lmp` | [Evidence](../tmp/compet-n-continue-current-remainder-v89/DOOMU-E4M1-r4m1-137-56623bb117d6/harness.log) |
| `DOOMU-E4M8-U48GLICH-bdc56641f6c7.lmp` | [Evidence](../tmp/compet-n-original-overflow-diagnostics/diagnostic-result.json) |
| `DOOMU-E4M1-R4M1-344-f647cedb3719.lmp` | [Evidence](../tmp/compet-n-continue-current-comedy-eight-remainder-v96/DOOMU-E4M1-R4M1-344-f647cedb3719/harness.log) |
| `DOOMU-E4M1-F4M1-426-569d92cffbed.lmp` | [Evidence](../tmp/compet-n-continue-current-comedy-eight-remainder-v96/DOOMU-E4M1-F4M1-426-569d92cffbed/harness.log) |
| `DOOMU-E1M5-P1M5-422-663073c68528.lmp` | [Evidence](../tmp/compet-n-continue-current-comedy-eight-remainder-v96/DOOMU-E1M5-P1M5-422-663073c68528/harness.log) |
| `DOOMU-E4M1-R4M1-525-a6d57dc4666a.lmp` | [Evidence](../tmp/compet-n-continue-current-comedy-eight-remainder-v98/DOOMU-E4M1-R4M1-525-a6d57dc4666a/harness.log) |

## Fixed: door/platform structure corruption — 13 recordings

Original `EV_VerticalDoor` treats an active `T_PlatRaise` as `vldoor_t`;
`door.direction` aliases `plat.wait`. v102 safely applies the same -1/1 field
transition for manual raise doors, without creating a door thinker or touching
invalid memory. It also decrements the platform countdown with 32-bit wrapping
and resumes only at exactly zero, as `!--plat->count` does in the original.
All thirteen recordings now **strictly match the complete state/RNG window**.
They are removed from the current exclusion list. Historical mismatches and
trigger diagnostics remain preserved below.

| Recording | Overwrite tic | Trigger | Local evidence |
| --- | ---: | --- | --- |
| `DOOMU-E1M1-EP1-2220-73499049af7e.lmp` | 20013 | E1M4 line 564, special 27, sector 41 | [Evidence](../tmp/compet-n-continue-ep1-platform-cast-debug-v100/diagnostic-result.json) |
| `DOOMU-E1M4-E1M4-024-c54d049c73b9.lmp` | 689 | Active platform handled as a door | [Evidence](../tmp/compet-n-continue-current-remainder-v84/DOOMU-E1M4-E1M4-024-c54d049c73b9/result.json) |
| `DOOMU-E1M4-E1M4-111-af18a5d8e72a.lmp` | 2157 | Line 564, special 27, sector 41 | [Evidence](../tmp/compet-n-continue-e1m4-platform-cast-debug-v89/diagnostic-result.json) |
| `DOOMU-E1M4-R1M4-314-966f0636af13.lmp` | 6304 | Line 564, special 27, sector 41 | [Evidence](../tmp/compet-n-continue-e1m4-platform-cast-r314-debug-v96/diagnostic-result.json) |
| `DOOMU-E3M7-E3M7-223-38fa61282ccf.lmp` | 3528 | Line 352, special 1, sector 23 | [Evidence](../tmp/compet-n-continue-e3m7-platform-cast-223-debug-v93/diagnostic-result.json) |
| `DOOMU-E3M7-F3M7-231-ea9f2bda9899.lmp` | 3318 | Line 352, special 1, sector 23 | [Evidence](../tmp/compet-n-continue-e3m7-platform-cast-fast231-debug-v95/diagnostic-result.json) |
| `DOOMU-E3M7-F3M7-243-6127dad8cc65.lmp` | 4538 | Line 352, special 1, sector 23 | [Evidence](../tmp/compet-n-continue-e3m7-platform-cast-fast243-debug-v95/diagnostic-result.json) |
| `DOOMU-E3M7-R3M7-210-18da98ebe969.lmp` | 2906 | Line 352, special 1, sector 23 | [Evidence](../tmp/compet-n-continue-e3m7-platform-cast-debug-v93/diagnostic-result.json) |
| `DOOMU-E3M7-e3m7-209-64e5951c47a3.lmp` | 3470 | Line 352, special 1, sector 23 | [Evidence](../tmp/compet-n-continue-e3m7-platform-cast-209-debug-v93/diagnostic-result.json) |
| `DOOMU-E3M7-e3m7-219-66c4dc4fe127.lmp` | 3651 | Line 352, special 1, sector 23 | [Evidence](../tmp/compet-n-continue-e3m7-platform-cast-219-debug-v93/diagnostic-result.json) |
| `DOOMU-E3M7-e3m7-221-9a58cc073080.lmp` | 3704 | Line 352, special 1, sector 23 | [Evidence](../tmp/compet-n-continue-e3m7-platform-cast-221-debug-v95/diagnostic-result.json) |
| `DOOMU-E3M7-f3m7-225-69af3a73a317.lmp` | 3191 | Line 352, special 1, sector 23 | [Evidence](../tmp/compet-n-continue-e3m7-platform-cast-fast225-debug-v95/diagnostic-result.json) |
| `DOOMU-E3M7-T3M7-800-18e3e502d35c.lmp` | 1580 | Line 352, special 1, sector 23 | [Evidence](../tmp/compet-n-continue-e3m7-platform-cast-t800-debug-v100/diagnostic-result.json) |

## Semantic comparison: unused ceiling field — 13 recordings

Original `EV_DoCeiling` leaves `ceiling.topheight` uninitialized for
`lowerAndCrush` (type 2); its downward action never reads that field.
`-ignore-unused-ceiling-topheight` enables a separate comparison that ignores
only this key on paired `kind: ceiling`, `type: 2`, `direction: -1` records.
All other fields and complete-window gameplay RNG still have to match. The
corpus runner exposes this as `--semantic-unused-ceiling-fields` and preserves
the raw strict status, report, and exit code.

The fresh v102 replay has **six strict matches and seven semantic-only
state/RNG matches** across these thirteen recordings. Allocator residue can
happen to be zero, so strict counts can vary across reference runs. All thirteen
preserved captures containing nonzero residue also pass the new semantic
comparator and remain historical strict mismatches. These cases are tracked as
semantic comparison cases, separate from the eighteen remaining exclusions.

| Recording | Diagnostic compared tics | Local evidence |
| --- | ---: | --- |
| `DOOMU-E3M5-E3M5-316-7cde201b0e6e.lmp` | 7,056 | [Diagnostic](../tmp/compet-n-continue-current-remainder-v95/DOOMU-E3M5-E3M5-316-7cde201b0e6e/unused-field-diagnostic.json) |
| `DOOMU-E3M5-e3m5-308-d38186ff56de.lmp` | 7,004 | [Diagnostic](../tmp/compet-n-lowercrush-followup/DOOMU-E3M5-e3m5-308-d38186ff56de/unused-field-diagnostic.json) |
| `DOOMU-E3M5-r3m5-244-7ac2652e0f78.lmp` | 6,035 | [Diagnostic](../tmp/compet-n-continue-current-remainder-v95/DOOMU-E3M5-r3m5-244-7ac2652e0f78/unused-field-diagnostic.json) |
| `DOOMU-E3M5-t3m5-352-8d74e86f0b0e.lmp` | 8,283 | [Diagnostic](../tmp/compet-n-continue-current-comedy-eight-remainder-v96/DOOMU-E3M5-t3m5-352-8d74e86f0b0e/unused-field-diagnostic.json) |
| `DOOMU-E3M5-R3M5-417-a3b5b72e5e82.lmp` | 9,757 | [Diagnostic](../tmp/compet-n-continue-current-comedy-eight-remainder-v96/DOOMU-E3M5-R3M5-417-a3b5b72e5e82/unused-field-diagnostic.json) |
| `DOOMU-E3M5-t3m5-427-dd086626a5ca.lmp` | 9,932 | [Diagnostic](../tmp/compet-n-continue-current-comedy-eight-remainder-v96/DOOMU-E3M5-t3m5-427-dd086626a5ca/unused-field-diagnostic.json) |
| `DOOMU-E3M5-E3M5-423-32b6c2d9672e.lmp` | 10,322 | [Diagnostic](../tmp/compet-n-continue-current-comedy-eight-remainder-v96/DOOMU-E3M5-E3M5-423-32b6c2d9672e/unused-field-diagnostic.json) |
| `DOOMU-E3M5-E3m5-520-0f3d02dc5b80.lmp` | 11,762 | [Diagnostic](../tmp/compet-n-continue-current-comedy-eight-remainder-v98/DOOMU-E3M5-E3m5-520-0f3d02dc5b80/unused-field-diagnostic.json) |
| `DOOMU-E3M5-E3M5-528-25f3fea7e995.lmp` | 13,616 | [Diagnostic](../tmp/compet-n-continue-current-comedy-eight-remainder-v98/DOOMU-E3M5-E3M5-528-25f3fea7e995/unused-field-diagnostic.json) |
| `DOOMU-E3M5-F3m5-617-a6011667ab09.lmp` | 14,693 | [Diagnostic](../tmp/compet-n-continue-current-comedy-eight-remainder-v98/DOOMU-E3M5-F3m5-617-a6011667ab09/unused-field-diagnostic.json) |
| `DOOMU-E3M5-T3M5-736-f6996424dd26.lmp` | 17,113 | [Diagnostic](../tmp/compet-n-continue-current-comedy-eight-remainder-v100/DOOMU-E3M5-T3M5-736-f6996424dd26/unused-field-diagnostic.json) |
| `DOOMU-E3M5-t3m5-827-3b8ef8f20705.lmp` | 18,715 | [Diagnostic](../tmp/compet-n-continue-current-comedy-eight-remainder-v100/DOOMU-E3M5-t3m5-827-3b8ef8f20705/unused-field-diagnostic.json) |
| `DOOMU-E3M5-t3m51230-5f13e6939c65.lmp` | 27,215 | [Diagnostic](../tmp/compet-n-continue-current-comedy-eight-remainder-v100/DOOMU-E3M5-t3m51230-5f13e6939c65/unused-field-diagnostic.json) |

## Timed door uses uninitialized memory — 1 recording

`DOOM2-MAP27-LV27-045-39985710cb55.lmp` manually reopens a timed door at tic 1082. Original `P_SpawnDoorCloseIn30` leaves `topheight` and `topwait` uninitialized; manual reopening then uses allocator residue as the live destination and wait (captured values 1330266433 and 16908801). Emulating those allocator contents is excluded. The raw **strict mismatch** and matching gameplay RNG remain recorded. [Local diagnostic](../tmp/compet-n-original-overflow-diagnostics/map27-timed-door-uninitialized.json).

## Accounting and future exclusions

Execution manifests are immutable: a failure classified after a queue was frozen remains in that historical queue. The original queues retain their 4,634-input accounting. Expanded coverage adds a separate twelve-recording follow-up for all 4,646 eligible inputs. Current verified passes, explicit exclusions, and remaining work stay separate. An accepted limitation never increases the strict match count.

Add a recording only after evidence identifies an original reference crash or an excluded original-engine glitch. Record its exact input, reason, raw outcome, and evidence in the machine-readable policy and this ledger. A similar map name or mismatch field alone is insufficient. Retain traces and logs for unexplained cases until classification or a verified ordinary fix is available.
