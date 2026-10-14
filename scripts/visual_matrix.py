#!/usr/bin/env python3
"""Compare all stock maps at 320x200 and selected maps at four additional sizes."""
import argparse
import html
import json
from pathlib import Path
import subprocess
import sys

ROOT = Path(__file__).resolve().parents[1]


def matrix(doom1, doom2):
    games = [
        ("doom1", doom1, [f"E{e}M{m}" for e in range(1, 5) for m in range(1, 10)],
         ["E1M1", "E1M5", "E2M1", "E2M8", "E3M1", "E3M3", "E4M1", "E4M6"]),
        ("doom2", doom2, [f"MAP{m:02d}" for m in range(1, 33)],
         ["MAP04", "MAP12", "MAP15", "MAP20", "MAP21", "MAP26", "MAP31", "MAP32"]),
    ]
    for resolution in ("320x200", "256x128", "400x250", "426x240", "640x400"):
        for game, wad, all_maps, selected in games:
            for mode, lighting, disabled in (("fullbright", False, False), ("lit", True, False), ("unlit", False, True)):
                yield dict(name=f"{game}-{resolution}-{mode}",
                           wad=str(wad.resolve()), maps=all_maps if resolution == "320x200" else selected,
                           resolution=resolution, lighting=lighting, disable_lighting=disabled)


def write_summary(out, runs):
    (out / "matrix.json").write_text(json.dumps(runs, indent=2))
    rows = []
    for run in runs:
        name = html.escape(run["name"])
        rows.append(f'<tr><td><a href="{name}/index.html">{name}</a></td>'
                    f'<td>{len(run["maps"])}</td><td>{run.get("scenes", "—")}</td>'
                    f'<td>{run.get("exact_classic", "—")}</td>'
                    f'<td>{run.get("max_cpu_gpu_percent", "—")}</td>'
                    f'<td>{html.escape(run["status"])}</td></tr>')
    page = '''<!doctype html><meta charset="utf-8"><title>Doom comparison matrix</title>
<style>body{font:15px system-ui;background:#16191d;color:#eee;margin:32px}a{color:#9cf}td,th{padding:9px 16px;text-align:left;border-bottom:1px solid #39404a}table{border-collapse:collapse}p{max-width:1000px}</style>
<h1>Doom I / Doom II comparison matrix</h1>
<p>Every scene compares classic, GD CPU, and GD shaders. Original lighting, identity-colormap fullbright, and the runtime lighting-disabled option are tested. All 68 maps run at 320×200; 16 selected maps run at four additional resolutions. Other resolutions adapt classic's buffers and retain its 320-pixel lighting/sky scale. The reference restores Ultimate Doom's episode sky after the Linux 1.10 enum collision.</p>
<p>Exact classic images include every pixel. A passing traced check permits only proven out-of-buffer classic reads; all image metrics retain those differences. Static geometry and lights, no actors/HUD. Each run links to its images and provenance; differing scenes are replayed with pixel/span traces in its <code>trace/</code> directory.</p>
<table><tr><th>Run</th><th>Maps</th><th>Scenes</th><th>Exact classic</th><th>Max CPU/GPU %</th><th>Validation</th></tr>ROWS</table>
<p><a href="matrix.json">Machine-readable results</a></p>'''
    (out / "index.html").write_text(page.replace("ROWS", "".join(rows)))


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--doom1", type=Path, default=ROOT / "wads/DOOMU.WAD")
    parser.add_argument("--doom2", type=Path, default=ROOT / "wads/DOOM2.WAD")
    parser.add_argument("--reference-source", type=Path, default=ROOT.parent / "doom-source/linuxdoom-1.10")
    parser.add_argument("--out", type=Path, default=ROOT / "tmp/visual-matrix")
    parser.add_argument("--only", help="comma-separated run names for focused reruns")
    args = parser.parse_args()
    out = args.out.resolve()
    out.mkdir(parents=True, exist_ok=True)
    runs = list(matrix(args.doom1, args.doom2))
    selected = set(args.only.split(",")) if args.only else {r["name"] for r in runs}
    if selected - {r["name"] for r in runs}:
        parser.error("unknown --only run name")
    # Retain completed rows when explicitly rerunning selected entries. Without
    # --only, recapture the entire matrix; never silently reuse stale images.
    old = {}
    if args.only and (out / "matrix.json").exists():
        old = {r["name"]: r for r in json.loads((out / "matrix.json").read_text())}
    for r in runs:
        r.update({k: v for k, v in old.get(r["name"], {}).items() if k not in r})
        if r["name"] in selected:
            r["status"] = "pending"
        else:
            r.setdefault("status", "not run")
    write_summary(out, runs)
    failed = False
    for r in runs:
        if r["name"] not in selected:
            continue
        dest = out / r["name"]
        base = [sys.executable, str(ROOT / "scripts/visual_compare.py"), "--wad", r["wad"],
                "--reference-source", str(args.reference_source.resolve()), "--maps", ",".join(r["maps"]),
                "--resolution", r["resolution"], "--require-gpu-parity"]
        if r["lighting"]:
            base.append("--lighting")
        if r["disable_lighting"]:
            base.append("--disable-lighting")
        r.pop("error", None)
        try:
            r["status"] = "capturing"
            write_summary(out, runs)
            subprocess.run(base + ["--out", str(dest)], cwd=ROOT, check=True)
            results = json.loads((dest / "results.json").read_text())["results"]
            differing = [a["scene"] for a in results if a["metrics"]["classic-cpu"]["different_percent"]]
            r.update(scenes=len(results), exact_classic=len(results)-len(differing),
                     max_cpu_gpu_percent=max(a["metrics"]["cpu-gpu"]["different_percent"] for a in results),
                     max_classic_percent=max(a["metrics"]["classic-cpu"]["different_percent"] for a in results))
            if differing:
                manifest = dest / "differing-scenes.json"
                manifest.write_text(json.dumps(differing, indent=2))
                subprocess.run(base + ["--out", str(dest / "trace"), "--scenes", str(manifest),
                                       "--require-bounded-parity", "--require-plane-parity"], cwd=ROOT, check=True)
                pixels = json.loads((dest / "trace/pixel-differences.json").read_text())
                r["traced_undefined_pixels"] = sum(a["counts"]["undefined_differences"] for a in pixels)
                r["status"] = "pass: bounded pixels + shared plane UVs exact"
            else:
                r["traced_undefined_pixels"] = 0
                r["status"] = "pass: all pixels exact"
        except (subprocess.CalledProcessError, OSError, ValueError) as error:
            r["status"] = "FAILED (see run logs)"
            r["error"] = str(error)
            failed = True
        write_summary(out, runs)
    print(f"Matrix: {out / 'index.html'}", flush=True)
    if failed:
        raise SystemExit("One or more matrix runs failed; reports retained")


if __name__ == "__main__":
    main()
