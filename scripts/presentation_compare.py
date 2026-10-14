#!/usr/bin/env python3
"""Compare weapon patches, bob coordinates and status widgets to Linux Doom."""
import argparse
import csv
import hashlib
import json
import os
import subprocess
import tempfile
from pathlib import Path

from PIL import Image

from visual_compare import ROOT, build_reference, run, compare


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--wad", type=Path, default=ROOT / "DOOM1.WAD")
    parser.add_argument("--out", type=Path, default=ROOT / "tmp/presentation-compare")
    parser.add_argument("--reference-source", type=Path, default=ROOT.parent / "doom-source/linuxdoom-1.10")
    args = parser.parse_args()
    args.wad, args.reference_source = args.wad.resolve(), args.reference_source.resolve()
    out = args.out.resolve()
    out.mkdir(parents=True, exist_ok=True)
    # Failed or incomplete captures must never reuse images from an earlier run.
    for prefix in ("weapon-", "hud-", "sequence-", "net-hud-", "effect-", "power-", "shader-effect-"):
        for suffix in ("-classic.ppm", "-gd.png"):
            for path in out.glob(prefix + "*" + suffix):
                path.unlink()
    (out / "scenes.tsv").write_text("")
    with tempfile.TemporaryDirectory(prefix="gddoom-presentation-") as tmp:
        work = Path(tmp)
        binary = build_reference(args.reference_source, work, out)
        wads = work / "wad"
        wads.mkdir()
        (wads / args.wad.name.lower()).symlink_to(args.wad)
        config = work / "probe.cfg"
        config.write_text("mb_used 32\nscreenblocks 11\nusegamma 0\n")
        env = dict(os.environ, DOOMWADDIR=str(wads), GD_PRESENTATION="1")
        run([binary, "-config", config, "-visualprobe", out / "scenes.tsv", out],
            out / "classic.log", cwd=work, env=env)
    env = dict(os.environ, GD_PRESENTATION_OUT=str(out), GD_PRESENTATION_WAD=str(args.wad))
    run(["xvfb-run", "-a", "go", "test", "-tags", "integration", "./internal/doomruntime",
         "-run", "^TestPresentationCapture$", "-count=1", "-v"], out / "gddoom.log", env=env)
    results = []
    with (out / "weapons.csv").open() as stream:
        names = [row["name"] for row in csv.DictReader(stream)]
    names += [f"hud-{n:02}" for n in range(72)]
    names += [f"sequence-{n:03}" for n in range(100)]
    names += [f"net-hud-{n:02}" for n in range(64)]
    names += [f"effect-{n:02}" for n in range(96)]
    names += [f"power-{n:02}" for n in range(80)]
    names += [f"shader-effect-{n:02}" for n in range(96)]
    for name in names:
        with Image.open(out / f"{name.removeprefix('shader-')}-classic.ppm") as source:
            reference = source.convert("RGB")
        with Image.open(out / f"{name}-gd.png") as source:
            gd = source.convert("RGB")
        metrics, diff = compare(reference, gd)
        reference.save(out / f"{name}-classic.png")
        diff.point(lambda v: min(255, v * 4)).save(out / f"{name}-diff.png")
        results.append(dict(name=name, **metrics))
    results.sort(key=lambda result: result["different_percent"], reverse=True)
    metadata = {
        "wad_sha256": hashlib.sha256(args.wad.read_bytes()).hexdigest(),
        "reference": str(args.reference_source),
        "scope": "320x200 isolated weapon patches at 3 fixed-point positions and 5 light levels; "
                 "72 single-player and 64 multiplayer status fixtures; 520 bob states; "
                 "100 raising, moving, firing and death frames; 80 weapon power-up fixtures; "
                 "96 PLAYPAL flash fixtures on both CPU and shader paths. "
                 "Raw palette RGB, no gamma postprocess, world occlusion or face-selection history.",
        "gddoom_commit": subprocess.check_output(["git", "rev-parse", "HEAD"], cwd=ROOT, text=True).strip(),
        "gddoom_dirty": bool(subprocess.check_output(["git", "status", "--porcelain"], cwd=ROOT, text=True)),
        "reference_sha256": {
            name: hashlib.sha256((args.reference_source / name).read_bytes()).hexdigest()
            for name in ("p_pspr.c", "p_user.c", "r_things.c", "r_draw.c", "st_stuff.c", "st_lib.c", "tables.c")
        },
    }
    (out / "results.json").write_text(json.dumps(dict(metadata=metadata, results=results), indent=2))
    (out/'index.html').write_text('''<!doctype html><meta charset="utf-8"><title>Weapon and HUD parity</title><style>body{background:#181b20;color:white;font:16px system-ui;margin:24px}img{image-rendering:pixelated;width:640px;max-width:100%}.row{display:flex;flex-wrap:wrap}select{padding:8px}pre{white-space:pre-wrap}</style><h1>Weapon / HUD comparison</h1><select id="sel"></select><pre id="stats"></pre><div class="row"><div>Classic<br><img id="ref"></div><div>GD-DOOM<br><img id="gd"></div><div>Difference ×4<br><img id="diff"></div></div><script>const data='''+json.dumps(results)+''';sel.innerHTML=data.map((r,i)=>`<option value="${i}">${r.name}: ${r.different_percent.toFixed(3)}%</option>`).join('');function update(){let r=data[+sel.value];stats.textContent=JSON.stringify(r,null,2);ref.src=r.name+'-classic.png';gd.src=r.name+'-gd.png';diff.src=r.name+'-diff.png'}sel.onchange=update;update()</script>''')
    different = sum(result["different_percent"] != 0 for result in results)
    print(f"{len(results)} captures; {different} differ. Report: {out / 'index.html'}")
    if different:
        raise SystemExit(1)


if __name__ == "__main__":
    main()
