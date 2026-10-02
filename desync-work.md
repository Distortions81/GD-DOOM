# Investigating demo desynchronization

The completed COMPET-N sweep checks **4,646 recordings** against the gameplay
source shipped in v0.1.1: **4,621 strict state/RNG matches**, **seven semantic-only
state/RNG matches**, and **eighteen documented exclusions**, with no pending
inputs or unexplained ordinary desyncs. Verification date: **2026-10-02**.
See the [public compatibility guide](demos/COMPET-N.md),
[per-recording results](demos/COMPET-N-results.json), and
[exclusion guide](demos/COMPET-N-exclusions.md) for scope and exact accounting.

## Find the first divergence

Start with the [desync harness](desync-harness.md). Replay the same recording
with the same IWAD in the trace-enabled original reference and GD-DOOM. Retain
both traces and logs, and compare up to original termination or the first
player-death tic, including that tic. Check gameplay RNG separately from
normalized state; a state match with different RNG is a failure.

```bash
scripts/demo_trace_compare.sh \
  --wad wads/DOOM2.WAD --demo demos/DOOM2-DEMO1.lmp \
  --out tmp/desync-investigation
```

The comparator identifies the first differing line, field, and tic. For object
state differences it also reports the normalized object entry; objects may be
sorted before comparison, so use that entry and the tic rather than assuming
the original trace's array index identifies the same object.

Use `--stop-after-tics N` for a short diagnostic window. A short-window match
is useful for investigation but does not count as a full-window corpus pass.
Work backward from the earliest differing state or RNG draw to the action
that produced it, then compare that action's ordering and arithmetic with the
original source. Debugger watchpoints are useful for mover fields, targets,
blockmap links, and shared collision state.

## Fixes covered by the sweep

The compatibility work corrected several recurring causes of desynchronization:

| Area | Original behavior preserved |
| --- | --- |
| Movement and collision | Fixed-point overflow, slide pickups, blockmap iteration and link order, retained subsector links after failed movement, and shared collision probes during nested actions |
| Moving sectors | Plane clipping order, crusher bookkeeping, corpse clipping, delayed lift thinkers, and crusher restart direction |
| Monsters and projectiles | Same-tic damage wake actions, missile spawn/impact timing, Lost Soul charge and floor clipping, target reacquisition, fast-mode timing, and targetless Pain Elemental death spawns |
| Gameplay RNG | Pickup amounts, damage thrust, radiation-suit leakage, invisibility targeting, and original action ordering |
| Game ticker | Recorded save actions and pause behavior, including frozen thinker/weapon updates |
| Manual doors and platforms | The original wait-field transition on active platforms, 32-bit countdown wrapping, and resumption only when the countdown reaches exactly zero |

The manual-door/platform correction safely reproduces the relevant field
transition without invalid structure access. All thirteen affected external
recordings now match strictly through their full state/RNG comparison windows.
Save and netplay keyframe formats remain **26** and **13**.

## Distinguish gameplay differences from unused memory

The optional ceiling policy ignores only `topheight` on paired downward
`lowerAndCrush` ceiling records (`kind: ceiling`, `type: 2`, `direction: -1`).
The original action leaves that field uninitialized and never reads it.
The corpus flag is `--semantic-unused-ceiling-fields`; the comparator flag is
`-ignore-unused-ceiling-topheight`. Keep the strict mismatch and separate
semantic result. All other fields, trace lengths, and independently audited
gameplay RNG must match.

This rule does not apply to MAP27's timed door: reopening it reads uninitialized
fields as a live destination and wait time. That recording remains excluded,
along with seventeen original-reference crashes. Classify exclusions by exact
input and diagnostic evidence; a matching map or field name is insufficient.

## Verify a correction

Add a regression that reproduces the original behavior and fails before the
fix. Recheck the entire affected recording and its gameplay RNG, then run
related fixtures and the appropriate Go tests. For comparator or importer
changes, also run the corpus tooling checks:

```bash
python3 -B scripts/test_demo_corpus.py
```

A gameplay change invalidates coverage from older runtimes. Pin the new runtime,
input manifests, reference, harness, comparator, and trimmer before another
sweep. Preserve input and result hashes and keep strict passes, semantic-only
passes, exclusions, and pending work separate. Raw captures and operational
receipts belong in ignored local output directories; public documentation
should contain portable technical evidence without personal machine details.

The current validation includes 101 ordinary fixtures, all 26 platform/ceiling
follow-ups, the full Go suite, and fourteen Python corpus checks. Fixtures that
overlap the COMPET-N inventory do not add to the corpus total.
