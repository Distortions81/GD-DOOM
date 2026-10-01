# Demo compatibility exclusions

Updated 2026-10-01. The user excluded NoClip emulation and other extreme original-engine glitches or crashes from the desync fix scope.

**26 recordings are currently excluded from fixes.** They remain in the 4,634-recording inventory with their raw strict mismatch/error results. An exclusion is never a passing comparison. Ordinary unexplained state/RNG differences and failures in GD-DOOM or the replay harness remain under investigation.

The authoritative per-recording policy is `evaluation_policy.accepted_limitations` in [COMPET-N-results.json](COMPET-N-results.json). Names below include the imported input hash suffix. That report and its phase records retain full input hashes, strict results, and gameplay RNG outcomes. Evidence links refer to preserved local captures under `tmp/`; those captures are not committed to Git. The findings are summarized here so the exclusions remain understandable without the local captures.

## Original reference crashes — 12 recordings

These inputs crash the original Linux Doom reference during trace capture, before GD-DOOM playback. Their result stays **error**, and no complete state comparison or gameplay RNG match is claimed. The MAP02 NoClip recording has a GDB backtrace at tic 2251 in `PIT_AddThingIntercepts` during `A_FireShotgun2`; DOS overflow emulation is outside scope. E4M8 is another bonus glitch recording. The exact causes of the ten E4M1 crashes remain unclassified.

| Recording | Local evidence |
| --- | --- |
| `DOOM2-MAP02-LV02-___-2e0d04c2fe92.lmp` | [Evidence](../tmp/compet-n-original-overflow-diagnostics/diagnostic-result.json) |
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

## Door/platform structure corruption — 10 recordings

Original `EV_VerticalDoor` treats a sector's active `T_PlatRaise` as `vldoor_t`. Writing `direction = -1` overwrites `plat_t.wait` (105 becomes -1), changing the platform's completion. Replicating that invalid structure access is excluded. Each recording retains its **strict mismatch**, including any later RNG divergence; matching RNG alone does not make it a pass.

| Recording | Overwrite tic | Trigger | Local evidence |
| --- | ---: | --- | --- |
| `DOOMU-E1M4-E1M4-024-c54d049c73b9.lmp` | 689 | Active platform handled as a door | [Evidence](../tmp/compet-n-continue-current-remainder-v84/DOOMU-E1M4-E1M4-024-c54d049c73b9/result.json) |
| `DOOMU-E1M4-E1M4-111-af18a5d8e72a.lmp` | 2157 | Line 564, special 27, sector 41 | [Evidence](../tmp/compet-n-continue-e1m4-platform-cast-debug-v89/diagnostic-result.json) |
| `DOOMU-E3M7-E3M7-223-38fa61282ccf.lmp` | 3528 | Line 352, special 1, sector 23 | [Evidence](../tmp/compet-n-continue-e3m7-platform-cast-223-debug-v93/diagnostic-result.json) |
| `DOOMU-E3M7-F3M7-231-ea9f2bda9899.lmp` | 3318 | Line 352, special 1, sector 23 | [Evidence](../tmp/compet-n-continue-e3m7-platform-cast-fast231-debug-v95/diagnostic-result.json) |
| `DOOMU-E3M7-F3M7-243-6127dad8cc65.lmp` | 4538 | Line 352, special 1, sector 23 | [Evidence](../tmp/compet-n-continue-e3m7-platform-cast-fast243-debug-v95/diagnostic-result.json) |
| `DOOMU-E3M7-R3M7-210-18da98ebe969.lmp` | 2906 | Line 352, special 1, sector 23 | [Evidence](../tmp/compet-n-continue-e3m7-platform-cast-debug-v93/diagnostic-result.json) |
| `DOOMU-E3M7-e3m7-209-64e5951c47a3.lmp` | 3470 | Line 352, special 1, sector 23 | [Evidence](../tmp/compet-n-continue-e3m7-platform-cast-209-debug-v93/diagnostic-result.json) |
| `DOOMU-E3M7-e3m7-219-66c4dc4fe127.lmp` | 3651 | Line 352, special 1, sector 23 | [Evidence](../tmp/compet-n-continue-e3m7-platform-cast-219-debug-v93/diagnostic-result.json) |
| `DOOMU-E3M7-e3m7-221-9a58cc073080.lmp` | 3704 | Line 352, special 1, sector 23 | [Evidence](../tmp/compet-n-continue-e3m7-platform-cast-221-debug-v95/diagnostic-result.json) |
| `DOOMU-E3M7-f3m7-225-69af3a73a317.lmp` | 3191 | Line 352, special 1, sector 23 | [Evidence](../tmp/compet-n-continue-e3m7-platform-cast-fast225-debug-v95/diagnostic-result.json) |

## Unused uninitialized ceiling field — 3 recordings

The original `EV_DoCeiling` constructor leaves `ceiling.topheight` uninitialized for `lowerAndCrush` (type 2), and its downward action never uses that field. The strict comparator exposes allocator residue. A separate diagnostic replacing only this unused reference field with zero matches each complete comparison window, with independently matching gameplay RNG. The comparator itself remains unchanged, and the raw result remains **strict mismatch**.

| Recording | Diagnostic compared tics | Local evidence |
| --- | ---: | --- |
| `DOOMU-E3M5-E3M5-316-7cde201b0e6e.lmp` | 7,056 | [Diagnostic](../tmp/compet-n-continue-current-remainder-v95/DOOMU-E3M5-E3M5-316-7cde201b0e6e/unused-field-diagnostic.json) |
| `DOOMU-E3M5-e3m5-308-d38186ff56de.lmp` | 7,004 | [Diagnostic](../tmp/compet-n-lowercrush-followup/DOOMU-E3M5-e3m5-308-d38186ff56de/unused-field-diagnostic.json) |
| `DOOMU-E3M5-r3m5-244-7ac2652e0f78.lmp` | 6,035 | [Diagnostic](../tmp/compet-n-continue-current-remainder-v95/DOOMU-E3M5-r3m5-244-7ac2652e0f78/unused-field-diagnostic.json) |

## Timed door uses uninitialized memory — 1 recording

`DOOM2-MAP27-LV27-045-39985710cb55.lmp` manually reopens a timed door at tic 1082. Original `P_SpawnDoorCloseIn30` leaves `topheight` and `topwait` uninitialized; manual reopening then uses allocator residue as the live destination and wait (captured values 1330266433 and 16908801). Emulating those allocator contents is excluded. The raw **strict mismatch** and matching gameplay RNG remain recorded. [Local diagnostic](../tmp/compet-n-original-overflow-diagnostics/map27-timed-door-uninitialized.json).

## Accounting and future exclusions

Execution manifests are immutable: a failure classified after a queue was frozen remains in that historical queue. New queues list current verified passes, explicit exclusions, and remaining work as separate partitions totaling all 4,634 eligible inputs. An accepted limitation never increases the strict match count.

Add a recording only after evidence identifies an original reference crash or an excluded original-engine glitch. Record its exact input, reason, raw outcome, and evidence in the machine-readable policy and this ledger. A similar map name or mismatch field alone is insufficient. Retain traces and logs for unexplained cases until classification or a verified ordinary fix is available.
