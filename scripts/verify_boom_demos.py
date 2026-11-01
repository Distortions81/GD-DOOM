#!/usr/bin/env python3
"""Compare recorded paths with DSDA-Doom ghost exports (single-player NoMo).

Downloads are pinned by SHA-256. WADs/demos stay in the output directory.
Requires Go, xvfb-run, an owned Doom II IWAD, and DSDA-Doom 0.27.x.
"""

import argparse
import hashlib
import io
import json
import os
from pathlib import Path
import shutil
import struct
import subprocess
import urllib.request
import zipfile

ROOT = Path(__file__).resolve().parents[1]
BASE = "https://dsdarchive.com"
WADS = {
    "aaliens": ("/files/wads/doom2/4503/aaliens.zip", "1c9d1e256a44bab531c8017e5a590cdcc51ac4411f154446c694589054ec2884", "aaliens_v1_2.wad"),
    "sunlust": ("/files/wads/doom2/5206/sunlust.zip", "10deec60d92a05892d36122b5d40778ba1210988b0accacae7ff10433fa8a046", "sunlust.wad"),
    "sunder": ("/files/wads/doom2/2726/sunder.zip", "eab6ebbeafb0afe4a2ccd93c1685484f3a517ab0aec200a9c693e29d4ca87839", "sunder.wad"),
}
CASES = [
    ("aa02o026", "aaliens", 2, "/files/demos/aaliens/70095/aa02o026.zip", "800cbadcb90341e62b424950959c1a7e1770f391b368faa51be1e560cb6b4bc0", "aa02o026.lmp"),
    ("aa01o023", "aaliens", 1, "/files/demos/aaliens/70094/aa01o023.zip", "ab4f4b0e6bcf04da26d2312c49156d252f672cf091e1d02c34389db4c3901131", "aa01o023.lmp"),
    ("sl01o016", "sunlust", 1, "/files/demos/sunlust/23036/sl01o016.zip", "37e1e8779377e35520506a9f9e791ed2316c15951d676e6f664720ab35d50169", "sl01o016.lmp"),
    ("sl02o048", "sunlust", 2, "/files/demos/sunlust/23051/sl02o048.zip", "6ad00f99ce58869fed0eb90a0e06fe16752f4bca5c9978a5bf734302a64bb3a5", "sunlust02nomo48.lmp"),
    ("su01o048", "sunder", 1, "/files/demos/sunder/22893/su01o048.zip", "f7840130a8cffed99a5bb5c2310ba52a78a17be86b38c3329fd40ac4d3e33215", "su01o048.lmp"),
]


def digest(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()


def fetch(out, archive_name, path, sha, entry):
    archive = out / archive_name
    if not archive.exists():
        archive.write_bytes(urllib.request.urlopen(BASE + path, timeout=60).read())
    if digest(archive) != sha:
        raise ValueError(f"archive SHA-256 mismatch: {archive}")
    with zipfile.ZipFile(io.BytesIO(archive.read_bytes())) as z:
        target = out / Path(entry).name
        target.write_bytes(z.read(entry))
    return target


def run(command, out, env, log):
    with log.open("w") as f:
        subprocess.run(command, cwd=out, env=env, stdout=f, stderr=subprocess.STDOUT, check=True, timeout=180)


def compare(trace, ghost, map_number):
    raw = ghost.read_bytes()
    if len(raw) < 44 or struct.unpack_from("<ii", raw) != (2, 1) or (len(raw)-8) % 36:
        raise ValueError("expected a version-2, single-player DSDA ghost")
    frames = [struct.unpack_from("<iiiIiiiii", raw, i) for i in range(8, len(raw), 36)]
    players, exits = {}, []
    with trace.open() as f:
        for line in f:
            row = json.loads(line)
            if row.get("kind") != "tic":
                continue
            p = row["player"]
            players[row["gametic"]] = tuple(p[k] for k in ("x", "y", "z", "angle"))
            if row["gamestate_name"] == "GS_INTERMISSION":
                exits.append(row["leveltime"])
    for frame in frames:
        if frame[6:8] != (map_number, 1):
            raise ValueError("reference ghost changed map or episode")
        # DSDA's exporter skips gametic zero and writes the completed tic at
        # gametic one. GD traces label that completed command gametic zero.
        tic = frame[8]-1
        if players.get(tic) != frame[:4]:
            raise ValueError(f"first divergence at tic {tic}: DSDA={frame[:4]}, GD={players.get(tic)}")
    exit_tic = frames[-1][8]+1
    if not exits or exits[0] != exit_tic:
        raise ValueError(f"exit mismatch: DSDA tic {exit_tic}, GD {exits[:1]}")
    return {"matched_frames": len(frames), "exit_leveltime": exit_tic, "exit_seconds": exit_tic/35}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--iwad", required=True, type=Path)
    parser.add_argument("--reference", default=shutil.which("dsda-doom"), type=Path)
    parser.add_argument("--engine", type=Path, help="reuse a built GD-DOOM binary")
    parser.add_argument("--out", default=ROOT / "tmp/boom-demo-verification", type=Path)
    parser.add_argument("--case", action="append", choices=[c[0] for c in CASES])
    args = parser.parse_args()
    if args.reference is None or not args.reference.is_file():
        parser.error("provide --reference pointing to a DSDA-Doom executable")
    if not args.iwad.is_file():
        parser.error("IWAD does not exist")
    out = args.out.resolve()
    out.mkdir(parents=True, exist_ok=True)
    env = dict(os.environ, SDL_AUDIODRIVER="dummy")
    reference = args.reference.resolve()
    prefix = reference.parent.parent
    if (prefix / "share/dsda-doom/dsda-doom.wad").is_file():
        env["DOOMWADDIR"] = str(prefix / "share/dsda-doom")
    libs = prefix / "lib/x86_64-linux-gnu"
    if libs.is_dir():
        env["LD_LIBRARY_PATH"] = str(libs) + ":" + env.get("LD_LIBRARY_PATH", "")
    engine = args.engine.resolve() if args.engine else out / "gddoom"
    if args.engine is None:
        subprocess.run(["go", "build", "-o", str(engine), "."], cwd=ROOT, check=True, env=env)
    results = {"iwad_sha256": digest(args.iwad), "engine_sha256": digest(engine), "reference_sha256": digest(reference), "cases": []}
    for case, wad_name, map_number, url, sha, entry in CASES:
        if args.case and case not in args.case:
            continue
        record = {"case": case, "wad": wad_name, "map": f"MAP{map_number:02}", "demo_url": BASE + url, "passed": False}
        try:
            wad_path = fetch(out, wad_name+".zip", *WADS[wad_name])
            demo_path = fetch(out, case+".zip", url, sha, entry)
            record.update(wad_sha256=digest(wad_path), demo_sha256=digest(demo_path), wad_url=BASE+WADS[wad_name][0])
            trace = out / (case+"-gd.jsonl")
            run(["xvfb-run", "-a", str(engine), "-render=false", "-wad", str(args.iwad.resolve()), "-file", str(wad_path), "-demo", str(demo_path), "-trace-demo-state", str(trace)], out, env, out/(case+"-gd.log"))
            ghost_base = out / (case+"-reference")
            ghost = ghost_base.with_suffix(".gst")
            ghost.unlink(missing_ok=True)
            data_dir = out / "reference-data"
            data_dir.mkdir(exist_ok=True)
            run(["xvfb-run", "-a", str(reference), "-iwad", str(args.iwad.resolve()), "-file", str(wad_path), "-timedemo", str(demo_path), "-export_ghost", str(ghost_base), "-levelstat", "-nosound", "-nodraw", "-config", str(out/"reference.cfg"), "-data", str(data_dir)], out, env, out/(case+"-reference.log"))
            record.update(compare(trace, ghost, map_number), passed=True)
        except (ValueError, OSError, subprocess.SubprocessError) as error:
            record["error"] = str(error)
        results["cases"].append(record)
        print(case, "PASS" if record["passed"] else "FAIL", record.get("matched_frames", record.get("error")), flush=True)
    (out/"results.json").write_text(json.dumps(results, indent=2)+"\n")
    return 0 if all(c["passed"] for c in results["cases"]) else 1


if __name__ == "__main__":
    raise SystemExit(main())
