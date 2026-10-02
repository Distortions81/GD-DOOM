# Demo compatibility exclusions

Verified 2026-10-02 against the gameplay source shipped in v0.1.1.

The completed 4,646-recording sweep has **4,621 strict state/RNG matches**,
**seven semantic-only state/RNG matches**, and **eighteen exclusions**.
No recordings remain pending and no unexplained ordinary desync remains.
Exclusions retain their raw error or mismatch and never count as passes.

The remaining exclusions are seventeen crashes of the original reference and
one timed door that reads uninitialized memory. NoClip overflow emulation,
extreme original-engine crashes, and live allocator-dependent behavior are
outside the compatibility scope. Cases are classified individually, using
exact input identities and diagnostic evidence.

All thirteen door/platform cases below are now fixed and match strictly.
The thirteen unused-ceiling cases pass the opt-in semantic policy; six also
matched strictly in this sweep. They are separate from the eighteen exclusions.
See the [public compatibility guide](COMPET-N.md) for scope and reproduction,
and [machine-readable results](COMPET-N-results.json) for exact input hashes,
raw outcomes, independent RNG audits, and immutable result hashes. Raw capture
files are retained locally rather than linked as publicly available downloads.

## Original reference crashes — 17 recordings

These inputs crash the original Linux Doom reference during trace capture, before GD-DOOM playback. Their result stays **error**, and no complete state comparison or gameplay RNG match is claimed. The MAP02 NoClip recording has a GDB backtrace at tic 2251 in `PIT_AddThingIntercepts` during `A_FireShotgun2`; DOS overflow emulation is outside scope. E4M8 is another bonus glitch recording. The long E4M1 Tyson recording passes its startup allocation limit with a 16 MiB zone, then crashes at tic 91,667 in `PIT_CheckLine` during a Demon chase. GDB in the correct retail E4M1 mode shows an eight-entry `spechit` array indexed by corrupted value -150406339. The exact causes of the other thirteen E4M1 and one E1M5 crashes remain unclassified.

| Recording |
| --- |
| `DOOM2-MAP02-LV02-___-2e0d04c2fe92.lmp` |
| `DOOMU-E4M1-t4m1long-0b8ede500426.lmp` |
| `DOOMU-E4M1-E4M1-216-b60348996d54.lmp` |
| `DOOMU-E4M1-F4M1-243-2b049b0988fa.lmp` |
| `DOOMU-E4M1-F4M1-256-43eeb8830de3.lmp` |
| `DOOMU-E4M1-N4S1-058-e28ef8f766e5.lmp` |
| `DOOMU-E4M1-R4M1-152-bb8176145739.lmp` |
| `DOOMU-E4M1-U4M1-228-164073eb719a.lmp` |
| `DOOMU-E4M1-U4M1-232-3cd3abf3a131.lmp` |
| `DOOMU-E4M1-f4m1-223-43133ef22cd1.lmp` |
| `DOOMU-E4M1-n4s1-037-2e9eeef6819b.lmp` |
| `DOOMU-E4M1-r4m1-137-56623bb117d6.lmp` |
| `DOOMU-E4M8-U48GLICH-bdc56641f6c7.lmp` |
| `DOOMU-E4M1-R4M1-344-f647cedb3719.lmp` |
| `DOOMU-E4M1-F4M1-426-569d92cffbed.lmp` |
| `DOOMU-E1M5-P1M5-422-663073c68528.lmp` |
| `DOOMU-E4M1-R4M1-525-a6d57dc4666a.lmp` |

## Fixed: door/platform structure corruption — 13 recordings

Original `EV_VerticalDoor` treats an active `T_PlatRaise` as `vldoor_t`;
`door.direction` aliases `plat.wait`. GD-DOOM safely applies the same -1/1 field
transition for manual raise doors, without creating a door thinker or touching
invalid memory. It also decrements the platform countdown with 32-bit wrapping
and resumes only at exactly zero, as `!--plat->count` does in the original.
All thirteen recordings now **strictly match the complete state/RNG window**.
They are removed from the current exclusion list. Historical mismatches remain preserved in the raw evidence;
trigger details are listed below.

| Recording | Overwrite tic | Trigger |
| --- | ---: | --- |
| `DOOMU-E1M1-EP1-2220-73499049af7e.lmp` | 20013 | E1M4 line 564, special 27, sector 41 |
| `DOOMU-E1M4-E1M4-024-c54d049c73b9.lmp` | 689 | Active platform handled as a door |
| `DOOMU-E1M4-E1M4-111-af18a5d8e72a.lmp` | 2157 | Line 564, special 27, sector 41 |
| `DOOMU-E1M4-R1M4-314-966f0636af13.lmp` | 6304 | Line 564, special 27, sector 41 |
| `DOOMU-E3M7-E3M7-223-38fa61282ccf.lmp` | 3528 | Line 352, special 1, sector 23 |
| `DOOMU-E3M7-F3M7-231-ea9f2bda9899.lmp` | 3318 | Line 352, special 1, sector 23 |
| `DOOMU-E3M7-F3M7-243-6127dad8cc65.lmp` | 4538 | Line 352, special 1, sector 23 |
| `DOOMU-E3M7-R3M7-210-18da98ebe969.lmp` | 2906 | Line 352, special 1, sector 23 |
| `DOOMU-E3M7-e3m7-209-64e5951c47a3.lmp` | 3470 | Line 352, special 1, sector 23 |
| `DOOMU-E3M7-e3m7-219-66c4dc4fe127.lmp` | 3651 | Line 352, special 1, sector 23 |
| `DOOMU-E3M7-e3m7-221-9a58cc073080.lmp` | 3704 | Line 352, special 1, sector 23 |
| `DOOMU-E3M7-f3m7-225-69af3a73a317.lmp` | 3191 | Line 352, special 1, sector 23 |
| `DOOMU-E3M7-T3M7-800-18e3e502d35c.lmp` | 1580 | Line 352, special 1, sector 23 |

## Semantic comparison: unused ceiling field — 13 recordings

Original `EV_DoCeiling` leaves `ceiling.topheight` uninitialized for
`lowerAndCrush` (type 2); its downward action never reads that field.
`-ignore-unused-ceiling-topheight` enables a separate comparison that ignores
only this key on paired `kind: ceiling`, `type: 2`, `direction: -1` records.
All other fields and complete-window gameplay RNG still have to match. The
corpus runner exposes this as `--semantic-unused-ceiling-fields` and preserves
the raw strict status, report, and exit code.

The current sweep has **six strict matches and seven semantic-only
state/RNG matches** across these thirteen recordings. Allocator residue can
happen to be zero, so strict counts can vary across reference runs. All thirteen
preserved captures containing nonzero residue also pass the new semantic
comparator and remain historical strict mismatches. These cases are tracked as
semantic comparison cases, separate from the eighteen remaining exclusions.

| Recording | Diagnostic compared tics |
| --- | ---: |
| `DOOMU-E3M5-E3M5-316-7cde201b0e6e.lmp` | 7,056 |
| `DOOMU-E3M5-e3m5-308-d38186ff56de.lmp` | 7,004 |
| `DOOMU-E3M5-r3m5-244-7ac2652e0f78.lmp` | 6,035 |
| `DOOMU-E3M5-t3m5-352-8d74e86f0b0e.lmp` | 8,283 |
| `DOOMU-E3M5-R3M5-417-a3b5b72e5e82.lmp` | 9,757 |
| `DOOMU-E3M5-t3m5-427-dd086626a5ca.lmp` | 9,932 |
| `DOOMU-E3M5-E3M5-423-32b6c2d9672e.lmp` | 10,322 |
| `DOOMU-E3M5-E3m5-520-0f3d02dc5b80.lmp` | 11,762 |
| `DOOMU-E3M5-E3M5-528-25f3fea7e995.lmp` | 13,616 |
| `DOOMU-E3M5-F3m5-617-a6011667ab09.lmp` | 14,693 |
| `DOOMU-E3M5-T3M5-736-f6996424dd26.lmp` | 17,113 |
| `DOOMU-E3M5-t3m5-827-3b8ef8f20705.lmp` | 18,715 |
| `DOOMU-E3M5-t3m51230-5f13e6939c65.lmp` | 27,215 |

## Timed door uses uninitialized memory — 1 recording

`DOOM2-MAP27-LV27-045-39985710cb55.lmp` manually reopens a timed door at tic 1082. Original `P_SpawnDoorCloseIn30` leaves `topheight` and `topwait` uninitialized; manual reopening then uses allocator residue as the live destination and wait (captured values 1330266433 and 16908801). Emulating those allocator contents is excluded. The raw **strict mismatch** and matching gameplay RNG remain recorded.

## Accounting and future exclusions

Each recording is identified by its input hash and IWAD. Passing state and RNG
comparisons, semantic-only comparisons, and exclusions stay separate. An
accepted limitation never increases the strict match count. The current report
accounts for every eligible recording and distinguishes the previous runtime
baseline from the completed current sweep.

Add an exclusion only after evidence identifies an original reference crash
or unsupported original-engine behavior. Record the exact input, reason, raw
outcome, and diagnostic evidence. A similar map name or mismatch field alone
is insufficient. Preserve traces and logs for unexplained cases until an
ordinary fix is verified or the case is individually classified.
