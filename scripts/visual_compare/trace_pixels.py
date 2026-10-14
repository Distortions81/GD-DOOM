#!/usr/bin/env python3
"""Check final RGB parity, separating classic's traced out-of-buffer reads."""
import argparse
import csv
import json
from pathlib import Path

from PIL import Image


def summarize(directory):
    results = []
    for scene in json.loads((directory / "scenes.json").read_text()):
        name = scene["name"]
        with Image.open(directory / f"{name}-classic.png") as im:
            classic = im.convert("RGB")
        with Image.open(directory / f"{name}-cpu.png") as im:
            cpu = im.convert("RGB")
        counts = dict(bounded_differences=0, undefined_differences=0, classic_out_of_bounds_reads=0)
        examples = []
        with (directory / f"{name}-classic-pixels.csv").open() as stream:
            for raw in csv.DictReader(stream):
                r = {k: int(v) for k, v in raw.items()}
                # A negative available count means unknown ownership, not
                # permission to discard a difference from the comparison.
                outside = r["kind"] != 0 and 0 <= r["available"] <= r["texel"]
                counts["classic_out_of_bounds_reads"] += outside
                if classic.getpixel((r["x"], r["y"])) != cpu.getpixel((r["x"], r["y"])):
                    counts["undefined_differences" if outside else "bounded_differences"] += 1
                    if len(examples) < 8:
                        examples.append(r)
        results.append(dict(scene=name, counts=counts, examples=examples))
    return results


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("directory", type=Path)
    parser.add_argument("--require-bounded-parity", action="store_true")
    args = parser.parse_args()
    results = summarize(args.directory)
    (args.directory / "pixel-differences.json").write_text(json.dumps(results, indent=2))
    for row in results:
        print(row["scene"], row["counts"])
    if args.require_bounded_parity and any(r["counts"]["bounded_differences"] for r in results):
        raise SystemExit("Defined classic pixels differ (trace retained)")
