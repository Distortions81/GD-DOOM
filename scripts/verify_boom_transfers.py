#!/usr/bin/env python3
"""Compare GD CPU/GPU with DSDA-Doom 0.27.5 at identical fixed cameras.

Requires Go, Pillow, xvfb-run, an owned IWAD, and a DSDA capture build. Use
--reference-source to build a private source copy with dsda_probe.inc; optional
--cmake-arg flags select local dependencies. Rendering code is never patched.
Scene `checks` are explicit pixel regions asserting transfer behavior; whole
frame differences remain diagnostic and are always reported separately.
"""
import argparse
import csv
import hashlib
import html
import json
import os
from pathlib import Path
import shutil
import struct
import subprocess

from PIL import Image, ImageDraw
from visual_compare import compare, replace_once

ROOT = Path(__file__).resolve().parents[1]


def digest(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()


def run(command, log, env=None, cwd=ROOT):
    with log.open("w") as stream:
        subprocess.run(list(map(str, command)), cwd=cwd, env=env,
                       stdout=stream, stderr=subprocess.STDOUT, check=True, timeout=300)


def build_reference(source, out, cmake_args):
    source = source.resolve()
    if (source / "prboom2").is_dir():
        source = source / "prboom2"
    if 'VERSION 0.27.5' not in (source / "CMakeLists.txt").read_text():
        raise ValueError("reference source must be DSDA-Doom 0.27.5")
    target = out / "reference-source"
    shutil.copytree(source, target, dirs_exist_ok=True, ignore=shutil.ignore_patterns(".git", "build"))
    shutil.copy(ROOT / "scripts/visual_compare/dsda_probe.inc", target / "src")
    replace_once(target / "src/d_main.c", "void D_DoomMain(void)",
                 '#include "dsda_probe.inc"\n\nvoid D_DoomMain(void)')
    replace_once(target / "src/d_main.c", "  D_DoomMainSetup(); // CPhipps - setup out of main execution stack",
                 "  D_DoomMainSetup(); // CPhipps - setup out of main execution stack\n  GD_VisualProbe();")
    build = out / "reference-build"
    run(["cmake", "-S", target, "-B", build, "-DWITH_DUMB=OFF", "-DWITH_FLUIDSYNTH=OFF",
         "-DWITH_IMAGE=OFF", "-DWITH_MAD=OFF", "-DWITH_PORTMIDI=OFF", "-DWITH_VORBISFILE=OFF",
         *cmake_args], out / "reference-configure.log")
    run(["cmake", "--build", build, "-j8"], out / "reference-build.log")
    return build / "dsda-doom"


def merge_wads(paths, target):
    lumps = []
    for path in paths:
        raw = path.read_bytes()
        count, directory = struct.unpack_from("<ii", raw, 4)
        for i in range(count):
            start, size, label = struct.unpack_from("<ii8s", raw, directory+i*16)
            lumps.append((label, raw[start:start+size]))
    data, directory = bytearray(bytes(12)), bytearray()
    for label, raw in lumps:
        directory.extend(struct.pack("<ii8s", len(data), len(raw), label))
        data.extend(raw)
    struct.pack_into("<4sii", data, 0, b"PWAD", len(lumps), len(data))
    target.write_bytes(data + directory)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--iwad", required=True, type=Path)
    parser.add_argument("--pwad", required=True, type=Path)
    parser.add_argument("--scenes", required=True, type=Path)
    reference = parser.add_mutually_exclusive_group(required=True)
    reference.add_argument("--reference", type=Path, help="DSDA with the capture probe already compiled")
    reference.add_argument("--reference-source", type=Path)
    parser.add_argument("--cmake-arg", action="append", default=[])
    parser.add_argument("--out", required=True, type=Path)
    parser.add_argument("--complevel", type=int, choices=(9,11), default=9, help="DSDA compatibility: Boom 2.02 or MBF")
    parser.add_argument("--actors", action="store_true", help="render map actors in both engines")
    parser.add_argument("--width", type=int, default=320)
    parser.add_argument("--height", type=int, default=200)
    args = parser.parse_args()
    out = args.out.resolve()
    out.mkdir(parents=True, exist_ok=True)
    iwad, pwad, manifest = args.iwad.resolve(), args.pwad.resolve(), args.scenes.resolve()
    scenes = json.loads(manifest.read_text())
    if not scenes or not any(c.get("checks") for c in scenes):
        parser.error("provide scenes with explicit transfer-check regions")
    executable = args.reference.resolve() if args.reference else build_reference(args.reference_source, out, args.cmake_arg)
    if manifest != out / "input-scenes.json":
        shutil.copy(manifest, out / "input-scenes.json")
    merged = out / "merged.wad"
    merge_wads([iwad, pwad], merged)
    maps = ",".join(dict.fromkeys(c["map"] for c in scenes))
    env = dict(os.environ, SDL_AUDIODRIVER="dummy", GD_VISUAL_WAD=str(merged),
               GD_VISUAL_OUT=str(out), GD_VISUAL_WIDTH=str(args.width), GD_VISUAL_HEIGHT=str(args.height),
               GD_VISUAL_MAPS=maps, GD_VISUAL_LIGHTING="1", GD_VISUAL_ACTORS="1" if args.actors else "0", GD_VISUAL_SCENES=str(manifest))
    # Fixed scenes must never inherit a demo/death/lighting override from a shell.
    for key in ("GD_VISUAL_DEMO", "GD_VISUAL_DISABLE_LIGHTING", "GD_VISUAL_SOURCE_PORT"):
        env.pop(key, None)
    run(["xvfb-run", "-a", "go", "test", "-tags", "integration", "./internal/doomruntime",
         "-run", "^TestVisualRendererCapture$", "-count=1"], out / "gd.log", env)
    env.update(GD_DSDA_VISUAL_SCENES=str(out/"scenes.tsv"), GD_DSDA_VISUAL_OUT=str(out))
    run(["xvfb-run", "-a", executable, "-iwad", iwad, "-file", pwad, "-complevel", args.complevel,
         "-nosound", "-nomonsters", "-width", args.width, "-height", args.height, "-vidmode", "8bit",
         "-config", out/"dsda.cfg"], out / "dsda.log", env)
    with (out/"dsda-cameras.csv").open() as f:
        cameras = {r["name"]: r for r in csv.DictReader(f)}
    if len(cameras) != len(scenes):
        raise ValueError("reference did not capture every scene")
    results, cards, passed = [], [], True
    for c in scenes:
        name, camera = c["name"], cameras[c["name"]]
        expected = dict(x=round(c["X"]*65536), y=round(c["Y"]*65536), z=round(c["Z"]*65536),
                        angle=c["angle"], leveltime=c["tic"], width=args.width, height=args.height)
        if any(int(camera[k]) != v for k, v in expected.items()):
            raise ValueError(f"camera mismatch for {name}: {camera} expected {expected}")
        images = {backend: Image.open(out/(name+suffix)).convert("RGB")
                  for backend, suffix in [("dsda", "-dsda.ppm"), ("cpu", "-cpu.png"), ("gpu", "-gpu.png")]}
        if any(im.size != (args.width, args.height) for im in images.values()):
            raise ValueError("capture dimensions differ")
        images["dsda"].save(out/(name+"-dsda.png"))
        metrics, checks = {}, []
        for a, b in [("dsda", "cpu"), ("dsda", "gpu"), ("cpu", "gpu")]:
            metrics[a+"-"+b], diff = compare(images[a], images[b])
            diff.point(lambda x: min(255, x*4)).save(out/(name+"-"+a+"-"+b+"-diff.png"))
        passed &= metrics["cpu-gpu"]["different_percent"] == 0
        for check in c.get("checks", []):
            box = check["box"]
            if not (0 <= box[0] < box[2] <= args.width and 0 <= box[1] < box[3] <= args.height):
                raise ValueError("check region outside image")
            for backend in ("cpu", "gpu"):
                m, _ = compare(images["dsda"].crop(box), images[backend].crop(box))
                ok = m["different_percent"] <= check.get("max_different_percent", 0)
                passed &= ok
                checks.append(dict(name=check["name"], box=box, backend=backend, passed=ok, metrics=m))
        sheet = Image.new("RGB", (args.width*3, args.height+24), "#181818")
        draw = ImageDraw.Draw(sheet)
        for i, backend in enumerate(("dsda", "cpu", "gpu")):
            draw.text((i*args.width+8, 5), backend.upper(), fill="white")
            sheet.paste(images[backend], (i*args.width, 24))
        sheet.save(out/(name+"-comparison.png"))
        results.append(dict(scene=c, camera_verified=True, metrics=metrics, checks=checks))
        print(name, "DSDA/CPU differing pixels:", round(metrics["dsda-cpu"]["different_percent"], 4),
              "checks:", all(v["passed"] for v in checks), flush=True)
        cards.append(f'<h2>{html.escape(name)}</h2><img src="{name}-comparison.png"><p><a href="{name}-dsda-cpu-diff.png">DSDA/CPU difference ×4</a></p>')
    source_hash = hashlib.sha256()
    for path in sorted((ROOT/"internal").rglob("*")):
        if path.suffix in (".go", ".kage"):
            source_hash.update(str(path.relative_to(ROOT)).encode()+b"\0"+path.read_bytes())
    metadata = dict(complevel=args.complevel, actors=args.actors, verifier_sha256=digest(Path(__file__)), scenes_sha256=digest(manifest),
                    gd_source_sha256=source_hash.hexdigest(), reference="DSDA-Doom 0.27.5 software renderer, camera/export probe only",
                    reference_sha256=digest(executable), probe_sha256=digest(ROOT/"scripts/visual_compare/dsda_probe.inc"),
                    iwad_sha256=digest(iwad), pwad_sha256=digest(pwad), dimensions=[args.width,args.height],
                    gd_revision=subprocess.check_output(["git","rev-parse","HEAD"],cwd=ROOT,text=True).strip(),
                    gd_dirty=bool(subprocess.check_output(["git","status","--porcelain"],cwd=ROOT,text=True)),
                    gd_diff_sha256=hashlib.sha256(subprocess.check_output(["git","diff","HEAD"],cwd=ROOT)).hexdigest())
    (out/"results.json").write_text(json.dumps(dict(passed=passed, metadata=metadata, results=results), indent=2))
    (out/"index.html").write_text('<!doctype html><meta charset="utf-8"><title>Boom transfer comparison</title><style>body{background:#181818;color:#eee;font:16px system-ui;margin:24px}img{max-width:100%;image-rendering:pixelated}a{color:#9cf}</style><h1>DSDA / GD CPU / GD GPU</h1><p>Exact cameras; 320×200; raw PLAYPAL; no actors or HUD. Whole-image differences are diagnostic. Explicit transfer regions and CPU/GPU equality are checked.</p>'+"".join(cards)+'<p><a href="results.json">Metrics, checks and provenance</a></p>')
    if not passed:
        raise SystemExit("transfer region or CPU/GPU comparison failed; inspect results.json")


if __name__ == "__main__":
    main()
