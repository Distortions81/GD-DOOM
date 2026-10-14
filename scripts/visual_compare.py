#!/usr/bin/env python3
"""Capture original Linux Doom and both GD-DOOM renderers at fixed cameras."""
import argparse
import hashlib
import html
import json
import os
from pathlib import Path
import shutil
import subprocess
import sys
import tempfile

from PIL import Image, ImageChops, ImageDraw

ROOT = Path(__file__).resolve().parents[1]


def run(command, log, *, cwd=ROOT, env=None):
    print("Running:", " ".join(map(str, command)), flush=True)
    with log.open("w") as stream:
        result = subprocess.run(list(map(str, command)), cwd=cwd, env=env,
                                stdout=stream, stderr=subprocess.STDOUT, timeout=300)
    if result.returncode:
        raise RuntimeError(f"Command failed ({result.returncode}); see {log}\n{log.read_text()[-4000:]}")


def replace_once(path, old, new):
    text = path.read_text()
    if text.count(old) != 1:
        raise RuntimeError(f"Unsupported reference source layout in {path.name}: expected one {old!r}")
    path.write_text(text.replace(old, new, 1))


def build_reference(source, work, out, width=320, height=200):
    target = work / "reference"
    shutil.copytree(source, target, ignore=shutil.ignore_patterns("linux", ".git"))
    (target / "linux").mkdir()
    shutil.copy(ROOT / "scripts/visual_compare/reference_probe.inc", target)
    shutil.copy(ROOT / "scripts/visual_compare/sampling_probe.inc", target)
    shutil.copy(ROOT / "scripts/visual_compare/camera_probe.inc", target)
    shutil.copy(ROOT / "scripts/visual_compare/span_trace.inc", target)
    for include in ("presentation_probe.inc", "status_probe.inc"):
        shutil.copy(ROOT / "scripts/visual_compare" / include, target)
    with (target / "st_stuff.c").open("a") as stream:
        stream.write('\n#include "status_probe.inc"\n')
    replace_once(target / "i_video.c", "void I_SetPalette (byte* palette)\n{", 'void I_SetPalette (byte* palette)\n{\n    if (M_CheckParm("-visualprobe")) return;')
    # Vanilla has a fixed 320x200 framebuffer. For other sizes, enlarge only
    # storage/viewport bounds, keeping its 320-pixel lighting and sky baseline.
    # This is a resolution-adapted oracle, explicitly recorded in provenance.
    if (width, height) != (320, 200):
        replace_once(target / "doomdef.h", "#define SCREENWIDTH  320", f"#define SCREENWIDTH  {width}")
        replace_once(target / "doomdef.h", "#define SCREENHEIGHT 200", f"#define SCREENHEIGHT {height}")
        rmain = target / "r_main.c"
        replace_once(rmain, "(SCREENWIDTH/2*FRACUNIT)", "(320/2*FRACUNIT)")
        replace_once(rmain, "FRACUNIT*SCREENWIDTH/viewwidth", "FRACUNIT*320/viewwidth")
        replace_once(rmain, "j*SCREENWIDTH/(viewwidth<<detailshift)", "j*320/(viewwidth<<detailshift)")
        # Visplane row indices and their padding must represent rows above 255.
        if height >= 255:
            defs = target / "r_defs.h"
            for field in ("pad1", "pad2", "pad3", "pad4", "top[SCREENWIDTH]", "bottom[SCREENWIDTH]"):
                replace_once(defs, "byte\t\t" + field, "unsigned short\t\t" + field)
            plane = target / "r_plane.c"
            for old in ("pl->top[x] != 0xff", "pl->top[pl->maxx+1] = 0xff", "pl->top[pl->minx-1] = 0xff"):
                replace_once(plane, old, old.replace("0xff", "0xffff"))
    main = target / "d_main.c"
    replace_once(main, "void D_DoomMain (void)", '#include "reference_probe.inc"\n\nvoid D_DoomMain (void)')
    replace_once(main, "    I_Init ();", '    if (!M_CheckParm("-visualprobe")) I_Init ();')
    replace_once(main, "    // start the apropriate game based on parms", "    VisualProbe();\n\n    // start the apropriate game based on parms")
    # NetUpdate polls the window during a world draw. The probe is headless;
    # suppress only this I/O, leaving all rasterization code intact.
    replace_once(target / "d_net.c", "void NetUpdate (void)\n{", 'void NetUpdate (void)\n{\n    if (M_CheckParm("-visualprobe")) return;')
    plane = target / "r_plane.c"
    replace_once(plane, '#include "r_sky.h"', '#include "r_sky.h"\nvoid VisualTracePlane(int, int, int);\nvoid VisualTraceSpan(void);')
    replace_once(plane, "    spanfunc ();", "    VisualTraceSpan();\n    spanfunc ();")
    replace_once(plane, "\t// sky flat", "\tVisualTracePlane(pl-visplanes, pl->height, pl->picnum);\n\t// sky flat")
    segs = target / "r_segs.c"
    replace_once(segs, '#include "r_sky.h"', '#include "r_sky.h"\nvoid VisualTraceWall(void);\nvoid VisualTraceMasked(void);')
    replace_once(segs, '    R_RenderSegLoop ();', '    VisualTraceWall();\n    R_RenderSegLoop ();')
    replace_once(segs, '    curline = ds->curline;', '    curline = ds->curline;\n    VisualTraceMasked();')
    draw = target / "r_draw.c"
    replace_once(draw, '#include "r_local.h"', '#include "r_local.h"\nvoid VisualTraceColumn(void);')
    replace_once(draw, '    // Zero length, column does not exceed a pixel.', '    VisualTraceColumn();\n    // Zero length, column does not exceed a pixel.')
    run(["make", "-j4"], out / "reference-build.log", cwd=target)
    return target / "linux/linuxxdoom"


def compare(a, b):
    dif = ImageChops.difference(a, b)
    red, green, blue = dif.split()
    histogram = ImageChops.lighter(ImageChops.lighter(red, green), blue).histogram()
    count = a.width * a.height
    # Pillow computes histograms in C; avoid allocating hundreds of thousands
    # of Python tuples per frame in the map/resolution matrix.
    error = sum((i % 256) * n for i, n in enumerate(dif.histogram()))
    return {"different_percent": 100 * (count - histogram[0]) / count,
            "over_2_percent": 100 * (count - sum(histogram[:3])) / count,
            "mean_absolute_error": error / (count * 3)}, dif


def report(out, scenes, metadata):
    results, cards = [], []
    width, height = metadata.get("dimensions", [320, 200])
    for scene in scenes:
        name = scene["name"]
        images = {}
        for backend in ("classic", "cpu", "gpu"):
            path = out / f"{name}-{backend}.{'ppm' if backend == 'classic' else 'png'}"
            with Image.open(path) as im:
                images[backend] = im.convert("RGB")
            if images[backend].size != (width, height):
                raise RuntimeError(f"Unexpected capture dimensions: {path}")
        images["classic"].save(out / f"{name}-classic.png")
        metrics = {}
        for left, right in (("classic", "cpu"), ("classic", "gpu"), ("cpu", "gpu")):
            key = f"{left}-{right}"
            metrics[key], diff = compare(images[left], images[right])
            diff.point(lambda v: min(255, v * 4)).save(out / f"{name}-{key}-diff.png")
        sheet = Image.new("RGB", (width * 3, height + 24), "#181818")
        draw = ImageDraw.Draw(sheet)
        for i, backend in enumerate(("classic", "cpu", "gpu")):
            draw.text((i*width+8, 5), backend.upper(), fill="white")
            sheet.paste(images[backend], (i*width, 24))
        sheet.save(out / f"{name}-comparison.png")
        results.append({"scene": scene, "metrics": metrics})
        cards.append(f'<option value="{html.escape(name)}">{html.escape(name)}</option>')
    payload = json.dumps(results).replace("<", "\\u003c")
    page = '''<!doctype html><meta charset="utf-8"><title>Doom renderer comparison</title>
<style>body{background:#16191d;color:#eee;font:15px system-ui;margin:24px}select,input,button{margin:8px;padding:6px}img,canvas{image-rendering:pixelated;max-width:100%}.row{display:flex;gap:12px;flex-wrap:wrap}.panel{flex:1;min-width:320px}.panel img{width:100%}#overlay{position:relative;width:640px;aspect-ratio:WIDTH/HEIGHT}#overlay img{position:absolute;width:100%;height:100%}pre{white-space:pre-wrap;color:#b5c5d6}label{display:inline-block}a{color:#9cf}</style>
<h1>Classic / CPU / GPU</h1><p>Fixed camera · WIDTH×HEIGHT · raw PLAYPAL · frozen geometry · no sprites or HUD. Classic differences are diagnostic, not a pass/fail claim. Difference maps amplify RGB error 4×.</p>
<select id="scene">OPTIONS</select><button id="prev">Previous</button><button id="next">Next</button><p id="summary"></p><details><summary>Camera and detailed metrics</summary><pre id="info"></pre></details>
<div class="row"><div class="panel">Classic<img id="classic"></div><div class="panel">GD CPU<img id="cpu"></div><div class="panel">GD GPU<img id="gpu"></div></div>
<div class="row"><div class="panel">Classic ↔ CPU<img id="classic-cpu-diff"></div><div class="panel">Classic ↔ GPU<img id="classic-gpu-diff"></div><div class="panel">CPU ↔ GPU<img id="cpu-gpu-diff"></div></div>
<h2>Overlay</h2><label>Compare to classic <select id="backend"><option>cpu</option><option>gpu</option></select></label><label>Opacity <input id="opacity" type="range" min="0" max="1" step="0.01" value="0.5"></label><div id="overlay"><img id="under"><img id="over"></div>
<p><a href="results.json">Metrics and provenance</a> · <a href="scenes.json">Reusable camera manifest</a></p>
<script>const results=PAYLOAD;const sel=document.querySelector('#scene');function update(){const r=results[sel.selectedIndex],n=r.scene.name;for(const id of ['classic','cpu','gpu','classic-cpu-diff','classic-gpu-diff','cpu-gpu-diff'])document.getElementById(id).src=n+'-'+id+'.png';under.src=n+'-classic.png';over.src=n+'-'+backend.value+'.png';over.style.opacity=opacity.value;summary.textContent='Pixels differing by >2 levels — Classic/CPU: '+r.metrics['classic-cpu'].over_2_percent.toFixed(3)+'% · Classic/GPU: '+r.metrics['classic-gpu'].over_2_percent.toFixed(3)+'% · CPU/GPU: '+r.metrics['cpu-gpu'].over_2_percent.toFixed(3)+'%';info.textContent=JSON.stringify(r,null,2);}sel.onchange=backend.onchange=opacity.oninput=update;prev.onclick=()=>{sel.selectedIndex=Math.max(0,sel.selectedIndex-1);update()};next.onclick=()=>{sel.selectedIndex=Math.min(results.length-1,sel.selectedIndex+1);update()};update();</script>'''
    if metadata.get("demo"):
        page = page.replace("Fixed camera · WIDTH×HEIGHT · raw PLAYPAL · frozen geometry · no sprites or HUD.", "Demo replay · WIDTH×HEIGHT · raw PLAYPAL · simulated world and camera · no sprites or HUD; player palette effects normalized.")
    (out / "index.html").write_text(page.replace("WIDTH", str(width)).replace("HEIGHT", str(height)).replace("OPTIONS", "".join(cards)).replace("PAYLOAD", payload))
    (out / "results.json").write_text(json.dumps({"metadata": metadata, "results": results}, indent=2))
    return results


def verify_demo_cameras(out, scenes):
    import csv
    with (out / "classic-cameras.csv").open() as stream:
        rows = {r["name"]: r for r in csv.DictReader(stream)}
    for scene in scenes:
        row = rows[scene["name"]]
        for key, expected in (("x", round(scene["X"]*65536)), ("y", round(scene["Y"]*65536)),
                              ("z", round(scene["Z"]*65536)), ("angle", scene["angle"]),
                              ("leveltime", scene["level_time"]), ("dead", int(scene["dead"]))):
            if int(row[key]) != expected:
                raise RuntimeError(f"Demo camera {scene['name']} {key}: classic={row[key]}, GD={expected}")


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--wad", type=Path, default=ROOT / "DOOM1.WAD")
    parser.add_argument("--maps", default="E1M1,E1M2,E1M3")
    parser.add_argument("--reference-source", type=Path, default=ROOT.parent / "doom-source/linuxdoom-1.10")
    parser.add_argument("--demo", type=Path, help="replay this version-109 single-player demo before each capture")
    parser.add_argument("--demo-tics", default="1,35,140,350", help="comma-separated consumed input counts for demo snapshots")
    parser.add_argument("--death-tics", default="0", help="additional forced-death simulation tics per demo snapshot, e.g. 0,1,18,35,48")
    parser.add_argument("--scenes", type=Path, help="reuse/edit a scenes.json camera manifest")
    parser.add_argument("--out", type=Path, default=ROOT / "tmp/visual-compare")
    parser.add_argument("--trace-spans", action="store_true", help="export per-span fixed-point plane coordinates from both renderers")
    parser.add_argument("--trace-pixels", action="store_true", help="export classic pixel ownership, source palette index and light row")
    parser.add_argument("--require-bounded-parity", action="store_true", help="trace pixels and require exact RGB parity except proven out-of-buffer classic reads")
    parser.add_argument("--require-plane-parity", action="store_true", help="trace spans and fail on coordinate differences for shared floor/ceiling pixels")
    parser.add_argument("--resolution", default="320x200", help="even capture dimensions, e.g. 256x128, 400x250, 640x400 (non-320x200 uses a resolution-adapted reference)")
    parser.add_argument("--disable-lighting", action="store_true", help="exercise the runtime DisableDoomLighting option; compare to identity classic colormaps")
    parser.add_argument("--lighting", action="store_true", help="keep original colormaps instead of isolating coordinates with fullbright")
    parser.add_argument("--require-gpu-parity", action="store_true", help="require byte-exact CPU/GPU RGB pixels, including differences of only one level")
    parser.add_argument("--max-gpu-diff", type=float, default=1.0, help="fail if CPU/GPU pixels differing by >2 RGB levels exceed this percentage")
    args = parser.parse_args()
    if args.lighting and args.disable_lighting:
        parser.error("--lighting and --disable-lighting are mutually exclusive")
    args.wad, args.reference_source, out = args.wad.resolve(), args.reference_source.resolve(), args.out.resolve()
    if not args.wad.is_file() or not (args.reference_source / "d_main.c").is_file():
        parser.error("IWAD and reference source directory must exist")
    if not 0 <= args.max_gpu_diff <= 100:
        parser.error("--max-gpu-diff must be between 0 and 100")
    try:
        width, height = map(int, args.resolution.lower().split("x"))
        if width % 2 or height % 2 or not (64 <= width <= 1280 and 64 <= height <= 800):
            raise ValueError()
    except ValueError:
        parser.error("--resolution must be even WIDTHxHEIGHT, from 64x64 to 1280x800")
    if args.demo:
        header = args.demo.read_bytes()[:13]
        if len(header) != 13 or header[0] not in (109, 110) or header[4] or header[8] or list(header[9:]) != [1, 0, 0, 0]:
            parser.error("--demo requires a single-player 109/110 recording")
        args.maps = f"MAP{header[3]:02d}" if args.wad.name.upper() == "DOOM2.WAD" else f"E{header[2]}M{header[3]}"
    out.mkdir(parents=True, exist_ok=True)
    env = dict(os.environ, GD_VISUAL_WAD=str(args.wad), GD_VISUAL_OUT=str(out),
               GD_VISUAL_MAPS=args.maps.upper(), GD_VISUAL_WIDTH=str(width), GD_VISUAL_HEIGHT=str(height), GD_VISUAL_LIGHTING="1" if args.lighting else "0",
               GD_VISUAL_DISABLE_LIGHTING="1" if args.disable_lighting else "0")
    env["GD_VISUAL_TRACE"] = "1" if args.trace_spans or args.require_plane_parity else "0"
    env.pop("GD_VISUAL_SCENES", None)
    env["GD_VISUAL_DEMO"] = str(args.demo.resolve()) if args.demo else ""
    env["GD_VISUAL_DEMO_TICS"] = args.demo_tics
    env["GD_VISUAL_DEATH_TICS"] = args.death_tics
    if args.scenes:
        # Read before captures overwrite scenes.json when reusing the output dir.
        scenes_input = args.scenes.resolve().read_text()
        (out / "input-scenes.json").write_text(scenes_input)
        env["GD_VISUAL_SCENES"] = str(out / "input-scenes.json")
    run(["xvfb-run", "-a", "go", "test", "-tags", "integration", "./internal/doomruntime",
         "-run", "^TestVisualRendererCapture$", "-count=1", "-v"], out / "gddoom.log", env=env)
    scenes = json.loads((out / "scenes.json").read_text())
    if not scenes:
        raise RuntimeError("No scenes were captured")
    for scene in scenes:
        # Never let a partial reference run reuse a prior capture as evidence.
        (out / f"{scene['name']}-classic.ppm").unlink(missing_ok=True)
    with tempfile.TemporaryDirectory(prefix="gddoom-visual-") as tmp:
        work = Path(tmp)
        binary = build_reference(args.reference_source, work, out, width, height)
        wad_dir = work / "wad"
        wad_dir.mkdir()
        # Linux Doom identifies the game by the canonical IWAD filename.
        (wad_dir / args.wad.name.lower()).symlink_to(args.wad)
        config = work / "probe.cfg"
        config.write_text("mb_used 32\nscreenblocks 11\nusegamma 0\n")
        ref_env = dict(os.environ, DOOMWADDIR=str(wad_dir), GD_VISUAL_TRACE=env["GD_VISUAL_TRACE"], GD_VISUAL_PIXELS="1" if args.trace_pixels or args.require_bounded_parity else "0")
        command = [binary, "-config", config, "-visualprobe", out / "scenes.tsv", out]
        if args.demo:
            (work / "check.lmp").symlink_to(args.demo.resolve())
            command += ["-timedemo", "check"]
            ref_env["GD_VISUAL_DEMO"] = "1"
        run(command, out / "classic.log", cwd=work, env=ref_env)
    oracle_env = dict(os.environ, GD_VISUAL_WIDTH=str(width), GD_VISUAL_HEIGHT=str(height), GD_CLASSIC_SAMPLING=str(out / "classic-sampling.csv"), GD_CLASSIC_LIGHTING=str(out / "classic-lighting.csv"), GD_CLASSIC_CAMERA=str(out / "classic-camera.csv"))
    run(["xvfb-run", "-a", "go", "test", "./internal/doomruntime", "-run", "^TestClassic(Sampling|Lighting|Camera)Oracle$", "-count=1"], out / "sampling-check.log", env=oracle_env)
    metadata = {"wad": str(args.wad), "wad_sha256": hashlib.sha256(args.wad.read_bytes()).hexdigest(),
                "reference_source": str(args.reference_source), "reference_retail_sky_setup": "restore G_InitNew episode sky after Linux gamemode/gamemission enum collision", "lighting": args.lighting, "disable_lighting": args.disable_lighting,
                "dimensions": [width, height], "reference_resolution_adapted": (width, height) != (320, 200), "gamma": 0, "palette": "raw PLAYPAL (display gamma normalized)",
                "note": "Static geometry, no actors/HUD; only texture specials advance. RGB differences include projection/lighting differences and are not proof of incorrect UVs."}
    metadata["gddoom_commit"] = subprocess.check_output(["git", "rev-parse", "HEAD"], cwd=ROOT, text=True).strip()
    metadata["gddoom_dirty"] = bool(subprocess.check_output(["git", "status", "--porcelain"], cwd=ROOT, text=True))
    reference_files = sorted(set(args.reference_source.glob("r_*.c")) | {
        args.reference_source / name for name in ("p_spec.c", "p_setup.c", "tables.c", "w_wad.c")})
    metadata["reference_sha256"] = {p.name: hashlib.sha256(p.read_bytes()).hexdigest() for p in reference_files}
    if args.demo:
        metadata["demo"] = str(args.demo.resolve())
        metadata["demo_sha256"] = hashlib.sha256(args.demo.read_bytes()).hexdigest()
        metadata["note"] = "Independently replayed demo world/camera snapshots; forced-death continuations use zero input. Actors/HUD and player powerup palette effects omitted at capture."
        verify_demo_cameras(out, scenes)
    results = report(out, scenes, metadata)
    worst = max(r["metrics"]["cpu-gpu"]["over_2_percent"] for r in results)
    print(f"Captured {len(scenes)} scenes. Worst CPU/GPU difference (>2 levels): {worst:.3f}%")
    print(f"Visual report: {out / 'index.html'}")
    if args.trace_pixels or args.require_bounded_parity:
        command = [sys.executable, ROOT / "scripts/visual_compare/trace_pixels.py", out]
        if args.require_bounded_parity:
            command.append("--require-bounded-parity")
        run(command, out / "pixel-check.log")
    if args.trace_spans or args.require_plane_parity:
        command = [sys.executable, ROOT / "scripts/visual_compare/trace_spans.py", out]
        if args.require_plane_parity:
            command.append("--require-plane-parity")
        run(command, out / "span-check.log")
    if args.require_gpu_parity and any(r["metrics"]["cpu-gpu"]["different_percent"] for r in results):
        raise SystemExit("CPU/GPU pixels are not exact (report retained)")
    if worst > args.max_gpu_diff:
        raise SystemExit(f"CPU/GPU difference exceeds {args.max_gpu_diff}% (report retained)")


if __name__ == "__main__":
    main()
