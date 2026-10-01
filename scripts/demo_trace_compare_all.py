#!/usr/bin/env python3
"""Compare every repository demo and audit gameplay RNG independently."""

import argparse
from concurrent.futures import ThreadPoolExecutor
import gzip
import hashlib
import json
from pathlib import Path
import re
import shutil
import subprocess
import sys
import tempfile
import zipfile

ROOT = Path(__file__).resolve().parents[1]
RNG = re.compile(rb'"prndindex":(\d+)')
TIC = re.compile(rb'"gametic":(\d+)')


def wad_for(demo):
    if demo.name.startswith("DOOM1-") or demo.name == "masked.lmp":
        return ROOT / "DOOM1.WAD"
    if demo.name.startswith("DOOMU-"):
        return ROOT / "wads/DOOMU.WAD"
    if demo.name.startswith("DOOM2-") or demo.name == "grate.lmp":
        return ROOT / "wads/DOOM2.WAD"
    raise ValueError(f"No WAD mapping for {demo}; add one before running the suite")


def tic_lines(file):
    for line in file:
        if b'"kind":"tic"' in line:
            yield line


def audit_rng(left, right):
    with left.open("rb") as lf, right.open("rb") as rf:
        for a, b in zip(tic_lines(lf), tic_lines(rf)):
            lm, rm = RNG.search(a), RNG.search(b)
            if lm is None or rm is None:
                raise ValueError("Trace lacks gameplay RNG index")
            if lm.group(1) != rm.group(1):
                return {"gametic": int(TIC.search(a).group(1)),
                        "reference": int(lm.group(1)), "gddoom": int(rm.group(1))}
    return None


def sha256(path):
    with path.open("rb") as file:
        return hashlib.file_digest(file, "sha256").hexdigest()


def immutable_snapshot(contents, directory, prefix, mode=0o755):
    """Pin a batch's tools so source edits cannot alter queued comparisons."""
    digest = hashlib.sha256(contents).hexdigest()
    destination = directory / f"{prefix}-{digest}"
    if destination.exists():
        if sha256(destination) != digest:
            raise ValueError(f"Pinned tool hash differs: {destination}")
        return destination
    with tempfile.NamedTemporaryFile(dir=directory, delete=False) as file:
        temporary = Path(file.name)
        file.write(contents)
    try:
        temporary.chmod(mode)
        temporary.replace(destination)
    finally:
        temporary.unlink(missing_ok=True)
    return destination


def pinned_replay_tools(directory):
    comparator = immutable_snapshot((directory / "demotracecmp").read_bytes(),
                                    directory, "demotracecmp")
    trimmer = immutable_snapshot((ROOT / "scripts/demo_trace_trim.py").read_bytes(),
                                 directory, "demo-trace-trim")
    harness = (ROOT / "scripts/demo_trace_compare.sh").read_text()
    # Leave the legacy harness untouched while older jobs still use it. New
    # immutable copies trim in place, retaining exactly the same trace prefix.
    trim_start = harness.index('trim_trace_on_player_death() {')
    trim_end = harness.index('usage() {', trim_start)
    harness = harness[:trim_start] + f'''TRACE_TRIM_BIN="${{ROOT_DIR}}/.tmp/{trimmer.name}"

trim_trace_on_player_death() {{
  python3 "${{TRACE_TRIM_BIN}}" "$1"
}}

trim_trace_after_tics() {{
  if [[ "$2" -gt 0 ]]; then
    python3 "${{TRACE_TRIM_BIN}}" "$1" --max-tics "$2"
  fi
}}

''' + harness[trim_end:]
    assignment = 'TRACECMP_BIN="${ROOT_DIR}/.tmp/demotracecmp"'
    rebuild_start = harness.index('if go_sources_newer_than_bin "${TRACECMP_BIN}"')
    rebuild_end = harness.index('\nREF_TRACE=', rebuild_start)
    harness = harness[:rebuild_start] + 'test -x "${TRACECMP_BIN}"\n' + harness[rebuild_end:]
    if harness.count(assignment) != 1:
        raise ValueError("Replay harness comparator assignment changed")
    harness = harness.replace(assignment, f'TRACECMP_BIN="${{ROOT_DIR}}/.tmp/{comparator.name}"')
    replay = immutable_snapshot(harness.encode(), directory, "demo-trace-compare")
    return replay, comparator, trimmer


def display_path(path):
    return str(path.relative_to(ROOT)) if path.is_relative_to(ROOT) else str(path)


def manifest_inputs(path):
    manifest = json.loads(path.read_text())
    wads = {}
    for item in manifest["demos"]:
        demo = (path.parent / item["path"]).resolve()
        if sha256(demo) != item["sha256"]:
            raise ValueError(f"Demo hash differs from manifest: {demo}")
        if demo in wads:
            raise ValueError(f"Duplicate demo in manifest: {demo}")
        wads[demo] = (ROOT / item["wad"]).resolve()
    if len({demo.stem for demo in wads}) != len(wads):
        raise ValueError("Manifest demo stems must be unique for output directories")
    return wads


def replay_map_error(demo, wad, trace):
    data = demo.read_bytes()
    expected = f"MAP{data[3]:02}" if wad.name.upper() == "DOOM2.WAD" else f"E{data[2]}M{data[3]}"
    with trace.open() as file:
        meta = json.loads(next(file))
    actual = meta.get("map")
    if actual != expected:
        return f"GD-DOOM trace starts on {actual}, expected {expected}; replay may have restarted after finale"
    return None


def uncovered_archive_demos(demo_dir):
    extracted = {sha256(path) for path in demo_dir.rglob("*.lmp")}
    missing = []
    for path in sorted(demo_dir.rglob("*.zip")):
        with zipfile.ZipFile(path) as archive:
            for member in archive.namelist():
                if member.lower().endswith(".lmp") and hashlib.sha256(archive.read(member)).hexdigest() not in extracted:
                    missing.append(f"{path.relative_to(demo_dir)}:{member}")
    return missing


def compress_retained_traces(directory):
    """Keep complete failure evidence without storing repeated JSON uncompressed."""
    files = []
    for path in sorted(directory.glob("*.jsonl")):
        destination = path.with_suffix(path.suffix + ".gz")
        temporary = destination.with_suffix(destination.suffix + ".tmp")
        try:
            with path.open("rb") as source, gzip.open(temporary, "wb", compresslevel=1) as output:
                shutil.copyfileobj(source, output, length=1024 * 1024)
            temporary.replace(destination)
            path.unlink()
        finally:
            temporary.unlink(missing_ok=True)
        files.append(destination.name)
    return files


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--out-root", type=Path, default=ROOT / "tmp/demo-trace-all")
    parser.add_argument("--ref-bin", type=Path,
                        default=ROOT.parent / "doom-source/linuxdoom-1.10/linux/linuxxdoom")
    parser.add_argument("--gd-bin", type=Path, help="Reuse this binary instead of rebuilding GD-DOOM")
    parser.add_argument("--jobs", type=int, default=1, help="Concurrent comparisons (large traces use substantial RAM)")
    parser.add_argument("--demo", action="append", help="Select demo stems; repeat to select several")
    parser.add_argument("--manifest", type=Path, help="Use an external corpus manifest instead of repository demos")
    parser.add_argument("--discard-matching-traces", action="store_true",
                        help="Delete large matching traces after RNG audit; keep logs and mismatches")
    parser.add_argument("--compress-retained-traces", action="store_true",
                        help="Gzip complete retained traces after comparison and RNG audit")
    args = parser.parse_args()
    if args.jobs < 1:
        parser.error("--jobs must be positive")
    if args.manifest:
        wads = manifest_inputs(args.manifest.resolve())
    else:
        missing = uncovered_archive_demos(ROOT / "demos")
        if missing:
            parser.error("Extract these archived demos into demos/ before running the suite: " + ", ".join(missing))
        wads = {demo: wad_for(demo) for demo in sorted((ROOT / "demos").rglob("*.lmp"))}
    # External manifests can prioritize new or unresolved recordings. Preserve
    # that order when submitting jobs; repository discovery remains sorted.
    demos = list(wads) if args.manifest else sorted(wads)
    if args.demo:
        unknown = set(args.demo) - {demo.stem for demo in demos}
        if unknown:
            parser.error(f"Unknown demo stems: {sorted(unknown)}")
        demos = [demo for demo in demos if demo.stem in args.demo]
    if not demos:
        parser.error("No demos selected")
    wads = {demo: wads[demo] for demo in demos}
    args.out_root = args.out_root.resolve()
    args.ref_bin = args.ref_bin.resolve()
    gd_bin = args.gd_bin.resolve() if args.gd_bin else ROOT / ".tmp/gddoom-demotrace"
    (ROOT / ".tmp").mkdir(exist_ok=True)
    if not args.gd_bin:
        subprocess.run(["go", "build", "-o", str(gd_bin), "."], cwd=ROOT, check=True)
    subprocess.run(["go", "build", "-o", str(ROOT / ".tmp/demotracecmp"), "./cmd/demotracecmp"],
                   cwd=ROOT, check=True)
    harness, comparator, trimmer = pinned_replay_tools(ROOT / ".tmp")
    hashes = {str(path): sha256(path) for path in {args.ref_bin, gd_bin, harness, comparator, trimmer, *wads.values()}}
    args.out_root.mkdir(parents=True, exist_ok=True)

    def compare(demo):
        out = args.out_root / demo.stem
        out.mkdir(parents=True, exist_ok=True)
        command = [str(harness), "--gd-bin", str(gd_bin),
                   "--ref-bin", str(args.ref_bin), "--wad", str(wads[demo]),
                   "--demo-lump", "check", "--demo", str(demo), "--out", str(out),
                   "--demo-exit-on-death"]
        with (out / "harness.log").open("w") as log:
            proc = subprocess.run(command, cwd=ROOT, stdout=log, stderr=subprocess.STDOUT)
        report_file = out / "compare.log"
        report = report_file.read_text() if report_file.exists() else ""
        status = "match" if proc.returncode == 0 and report.startswith("traces match ") else (
            "mismatch" if report.startswith(("mismatch ", "length mismatch ")) else "error")
        rng_diff = None
        map_error = None
        if status != "error":
            map_error = replay_map_error(demo, wads[demo], out / f"gddoom-{demo.name}.jsonl")
            if map_error:
                status = "replay-error"
            else:
                rng_diff = audit_rng(out / "reference-check.jsonl", out / f"gddoom-{demo.name}.jsonl")
                if rng_diff and status == "match":
                    status = "rng-mismatch"
        result = {"demo": display_path(demo), "wad": display_path(wads[demo]),
                  "status": status, "exit_code": proc.returncode, "report": report,
                  "gameplay_rng_mismatch": rng_diff, "gameplay_rng_audited": status not in ("error", "replay-error"),
                  "replay_map_error": map_error, "demo_sha256": sha256(demo),
                  "replay_harness": display_path(harness), "replay_harness_sha256": hashes[str(harness)],
                  "trace_comparator": display_path(comparator), "trace_comparator_sha256": hashes[str(comparator)],
                  "trace_trimmer": display_path(trimmer), "trace_trimmer_sha256": hashes[str(trimmer)]}
        if args.discard_matching_traces and status == "match":
            (out / "reference-check.jsonl").unlink()
            (out / f"gddoom-{demo.name}.jsonl").unlink()
            result["matching_traces_discarded"] = True
        if args.compress_retained_traces:
            result["retained_trace_files"] = compress_retained_traces(out)
        (out / "result.json").write_text(json.dumps(result, indent=2) + "\n")
        first = map_error or (report.splitlines()[0] if report else "see harness.log")
        rng_report = (rng_diff or "match") if result["gameplay_rng_audited"] else "not audited"
        print(f"{demo.stem}: {status}: {first}; RNG={rng_report}", flush=True)
        return result

    with ThreadPoolExecutor(max_workers=args.jobs) as pool:
        results = list(pool.map(compare, demos))
    summary = {"input_sha256": hashes, "results": results}
    if args.manifest:
        summary["manifest"] = display_path(args.manifest.resolve())
        summary["manifest_sha256"] = sha256(args.manifest)
    (args.out_root / "summary.json").write_text(json.dumps(summary, indent=2) + "\n")
    with (args.out_root / "summary.tsv").open("w") as file:
        file.write("demo\tstatus\tfirst_report\tfirst_rng_mismatch\n")
        for item in results:
            first = item["replay_map_error"] or (item["report"].splitlines()[0] if item["report"] else "")
            file.write(f"{item['demo']}\t{item['status']}\t{first}\t{item['gameplay_rng_mismatch']}\n")
    return 0 if all(item["status"] == "match" for item in results) else 1


if __name__ == "__main__":
    sys.exit(main())
