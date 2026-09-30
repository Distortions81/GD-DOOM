#!/usr/bin/env python3
"""Compare every repository demo and audit gameplay RNG independently."""

import argparse
from concurrent.futures import ThreadPoolExecutor
import hashlib
import json
from pathlib import Path
import re
import subprocess
import sys

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


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--out-root", type=Path, default=ROOT / "tmp/demo-trace-all")
    parser.add_argument("--ref-bin", type=Path,
                        default=ROOT.parent / "doom-source/linuxdoom-1.10/linux/linuxxdoom")
    parser.add_argument("--gd-bin", type=Path, help="Reuse this binary instead of rebuilding GD-DOOM")
    parser.add_argument("--jobs", type=int, default=1, help="Concurrent comparisons (large traces use substantial RAM)")
    parser.add_argument("--demo", action="append", help="Select demo stems; repeat to select several")
    args = parser.parse_args()
    if args.jobs < 1:
        parser.error("--jobs must be positive")
    demos = sorted((ROOT / "demos").rglob("*.lmp"))
    if args.demo:
        unknown = set(args.demo) - {demo.stem for demo in demos}
        if unknown:
            parser.error(f"Unknown demo stems: {sorted(unknown)}")
        demos = [demo for demo in demos if demo.stem in args.demo]
    wads = {demo: wad_for(demo) for demo in demos}
    args.out_root = args.out_root.resolve()
    args.ref_bin = args.ref_bin.resolve()
    gd_bin = args.gd_bin.resolve() if args.gd_bin else ROOT / ".tmp/gddoom-demotrace"
    (ROOT / ".tmp").mkdir(exist_ok=True)
    if not args.gd_bin:
        subprocess.run(["go", "build", "-o", str(gd_bin), "."], cwd=ROOT, check=True)
    subprocess.run(["go", "build", "-o", str(ROOT / ".tmp/demotracecmp"), "./cmd/demotracecmp"],
                   cwd=ROOT, check=True)
    hashes = {str(path): sha256(path) for path in {args.ref_bin, gd_bin, *wads.values()}}
    args.out_root.mkdir(parents=True, exist_ok=True)

    def compare(demo):
        out = args.out_root / demo.stem
        out.mkdir(parents=True, exist_ok=True)
        command = [str(ROOT / "scripts/demo_trace_compare.sh"), "--gd-bin", str(gd_bin),
                   "--ref-bin", str(args.ref_bin), "--wad", str(wads[demo]),
                   "--demo-lump", "check", "--demo", str(demo), "--out", str(out)]
        with (out / "harness.log").open("w") as log:
            proc = subprocess.run(command, cwd=ROOT, stdout=log, stderr=subprocess.STDOUT)
        report_file = out / "compare.log"
        report = report_file.read_text() if report_file.exists() else ""
        status = "match" if proc.returncode == 0 and report.startswith("traces match ") else (
            "mismatch" if report.startswith(("mismatch ", "length mismatch ")) else "error")
        rng_diff = None
        if status != "error":
            rng_diff = audit_rng(out / "reference-check.jsonl", out / f"gddoom-{demo.name}.jsonl")
            if rng_diff and status == "match":
                status = "rng-mismatch"
        result = {"demo": str(demo.relative_to(ROOT)), "wad": str(wads[demo].relative_to(ROOT)),
                  "status": status, "exit_code": proc.returncode, "report": report,
                  "gameplay_rng_mismatch": rng_diff, "demo_sha256": sha256(demo)}
        (out / "result.json").write_text(json.dumps(result, indent=2) + "\n")
        first = report.splitlines()[0] if report else "see harness.log"
        print(f"{demo.stem}: {status}: {first}; RNG={rng_diff or 'match'}", flush=True)
        return result

    with ThreadPoolExecutor(max_workers=args.jobs) as pool:
        results = list(pool.map(compare, demos))
    summary = {"input_sha256": hashes, "results": results}
    (args.out_root / "summary.json").write_text(json.dumps(summary, indent=2) + "\n")
    with (args.out_root / "summary.tsv").open("w") as file:
        file.write("demo\tstatus\tfirst_report\tfirst_rng_mismatch\n")
        for item in results:
            first = item["report"].splitlines()[0] if item["report"] else ""
            file.write(f"{item['demo']}\t{item['status']}\t{first}\t{item['gameplay_rng_mismatch']}\n")
    return 0 if all(item["status"] == "match" for item in results) else 1


if __name__ == "__main__":
    sys.exit(main())
