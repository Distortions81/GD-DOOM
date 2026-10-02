# COMPET-N demo compatibility

GD-DOOM uses COMPET-N speedrun recordings to check its gameplay against the
original Doom engine. The completed sweep covers **4,646 unique single-player
recordings** from Ultimate Doom and Doom II. Results were verified on
**2026-10-02** against the gameplay source shipped in **v0.1.1**.

## Results

| Outcome | Recordings | Share of corpus |
| --- | ---: | ---: |
| Strict state and gameplay RNG match | 4,621 | 99.46% |
| Semantic state and gameplay RNG match only | 7 | 0.15% |
| Documented exclusions | 18 | 0.39% |
| Pending or unexplained failures | 0 | 0.00% |
| Total | 4,646 | 100.00% |

**4,628 recordings (99.61%) pass** with the limited semantic policy described
below. Exclusions count in the denominator and never count as passes. All
eligible inputs are accounted for; passing recordings were checked on the
current gameplay source rather than credited from older runtime results.

The [machine-readable results](COMPET-N-results.json) retain each recording's
SHA-256, archive provenance, compared tic count, strict outcome, independent
gameplay RNG audit, and raw-result hash. Semantic results are recorded
separately. The [exclusion guide](COMPET-N-exclusions.md) explains the eighteen
remaining cases and the compatibility fixes that resolved earlier exclusions.

These results apply to this snapshot and these replay windows. They do not
establish compatibility with every Doom demo, modified WAD, or original DOS
memory-corruption behavior.

## Corpus

The inputs come from the [COMPET-N public archive](https://compet-n.gamers.org/public/compet-n/),
using its [2019-01-21 snapshot](https://compet-n.gamers.org/public/compet-n/compet-n_2019-01-21.zip).
The importer verifies the snapshot's pinned SHA-256 before reading its demos.

| Game | Unique recordings | Starting maps | Required IWAD |
| --- | ---: | --- | --- |
| Ultimate Doom | 2,411 | E1M1–E4M9, all 36 maps | `wads/DOOMU.WAD` |
| Doom II | 2,235 | MAP01–MAP32, all 32 maps | `wads/DOOM2.WAD` |

The nine categories are UV Speed, UV Max, Nightmare Speed, Nightmare 100%
Secrets, Tyson, Pacifist, UV Fast, UV Respawn, and No Monsters. The command
streams contain **238.99 recorded hours**; this describes input duration,
not execution time or the duration successfully compared.

Byte-identical recordings within each game are deduplicated, while category
and archive-member provenance are retained. All LMP members are inspected,
including extra attempts inside an archive. Twelve recordings require legacy
ZIP Implode or ARJ decoding; the complete import includes them, verifies their
size and CRC, and has **zero skipped archive entries**.

The selected corpus contains single-player version-109 demos for the two
listed IWADs. Co-op, multiplayer, other IWADs/PWADs, built/miscellaneous
categories, and multi-level movies are outside the selection. Newer archive
submissions are outside this snapshot.

## How comparison works

The harness replays identical inputs in a trace-enabled original Linux Doom
1.10 reference and GD-DOOM, then compares their normalized state at every tic.
It separately checks the gameplay random-number index (`prndindex`) at every
compared tic. Input, runtime, tool, configuration, and result hashes identify
the evidence used for the sweep.

Each comparison ends when the original replay terminates or at the first
player death, **including the death tic**. A pass makes no claim about input
after that boundary. The reference uses an isolated configuration with a
16 MiB memory zone, so existing local reference settings do not affect a run.
See the [harness guide](../desync-harness.md) for setup and trace details.

Strict comparison remains the default, using the comparator's documented
normalization of trace-only differences. The optional semantic policy ignores
only `topheight` when **both** compared records are ceilings of type 2
(`lowerAndCrush`) moving downward (`direction: -1`). Original Doom leaves this
field uninitialized, and that downward action never reads it. Every other
field, the trace length, and gameplay RNG still have to match.

Thirteen recordings exercise this ceiling case. In this sweep six matched
strictly and seven required semantic comparison. The strict split can vary
with the reference allocator's unused contents. Raw strict mismatches are
preserved; semantic matches never become strict matches. A separate check of
thirteen retained nonzero-residue captures also passed this semantic policy
and contributed no additional corpus passes.

## Reproduce a comparison

Use the Go and native dependencies listed in the [README](../README.md), a
trace-enabled original reference executable, and legally obtained copies of
the required IWADs. Install `7z` or `7zz` for the twelve legacy archives.
Generated inputs, manifests, logs, and traces stay in ignored local output
directories; commercial IWADs and the external demo pack are not bundled here.

From the repository root:

```bash
python3 -B scripts/fetch_compet_n.py

# Start with one recording per map, using the default strict comparison.
xvfb-run -a python3 -B scripts/demo_trace_compare_all.py \
  --manifest tmp/compet-n/smoke.json --jobs 2 \
  --discard-matching-traces --compress-retained-traces \
  --out-root tmp/compet-n-smoke

# Run the full inventory with the optional unused-ceiling-field policy.
xvfb-run -a python3 -B scripts/demo_trace_compare_all.py \
  --manifest tmp/compet-n/manifest.json --jobs 2 \
  --semantic-unused-ceiling-fields \
  --discard-matching-traces --compress-retained-traces \
  --out-root tmp/compet-n-full
```

The importer also accepts `--archive PATH` to reuse a downloaded snapshot.
It produces `smoke.json` (one recording per starting map), `combat.json`
(one UV Max recording per map), and `manifest.json` (the full inventory).
Confirm the importer reports 4,646 recordings and zero skipped entries before
claiming complete coverage. Omitting `--manifest` runs the 25 repository demos.
Use `--ref-bin PATH` to override the default reference executable at
`../doom-source/linuxdoom-1.10/linux/linuxxdoom`.

The runner writes per-recording logs and results, plus `summary.json` and
`summary.tsv` when the selection finishes. Matching traces may be discarded
after the RNG audit; mismatch traces remain available for diagnosis.
`--compress-retained-traces` compresses complete retained traces without
dropping tics. Large traces need substantial disk space, so begin with a small
selection and a modest worker count.

An excluded original-reference crash still appears as an error in a new run;
the runner does not turn exclusions into successful comparisons. Classify
results against the exact inputs in the exclusion guide.

## Additional validation

All **101 ordinary validation fixtures**, including the 25 repository demos,
matched state and gameplay RNG on the same gameplay source. All thirteen
door/platform follow-ups now match strictly. The full Go suite, fourteen
Python corpus checks, and release CI/vulnerability checks also passed.
Validation fixtures can overlap the external corpus and do not increase its
4,646-recording total.

The prior completed runtime baseline had 4,602 strict matches and 44 exclusions.
The current sweep supersedes that coverage, resolves all thirteen platform
cases, and separates the thirteen unused-ceiling cases from the remaining
eighteen exclusions. Historical raw captures and operational audit receipts
are retained locally; public results contain portable technical identifiers
without personal paths, host details, or conversation excerpts.
