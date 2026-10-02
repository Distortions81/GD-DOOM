#!/usr/bin/env python3
"""Prepare an opt-in vanilla Doom I/II replay corpus from the COMPET-N snapshot."""

import argparse
from collections import Counter
import hashlib
import io
import json
from pathlib import Path, PurePosixPath
import re
import shutil
import subprocess
import tempfile
import urllib.request
import zipfile
import zlib

ROOT = Path(__file__).resolve().parents[1]
URL = "https://compet-n.gamers.org/public/compet-n/compet-n_2019-01-21.zip"
ARCHIVE_SHA256 = "cb79b0a1bdc9233c2351d89fa229e00a5e461ae7d791ad535aea715673470fb1"
CATEGORIES = {"speed", "max", "nmare", "nm100s", "tyson", "pacifist", "fast", "respawn", "nomo"}
WADS = {"doom": "wads/DOOMU.WAD", "doom2": "wads/DOOM2.WAD"}


def legacy_decode(data, kind, command, member=None):
    decoder = shutil.which("7z") or shutil.which("7zz")
    if not decoder:
        raise NotImplementedError("Legacy ZIP/ARJ decoding requires 7z or 7zz")
    with tempfile.TemporaryDirectory(prefix="compet-n-archive-") as directory:
        archive = Path(directory) / "input.archive"
        archive.write_bytes(data)
        args = [decoder, command, f"-t{kind}", "-spd"]
        args += ["-slt"] if command == "l" else ["-so"]
        args += ["--", str(archive)]
        if member is not None:
            args.append(member)
        try:
            result = subprocess.run(args, stdin=subprocess.DEVNULL, stdout=subprocess.PIPE,
                                    stderr=subprocess.PIPE, timeout=30)
        except subprocess.TimeoutExpired as error:
            raise zipfile.BadZipFile(f"Legacy {kind} decoder timed out") from error
        if result.returncode:
            raise zipfile.BadZipFile(f"Legacy {kind} decoder failed: {result.stderr.decode(errors='replace').strip()}")
        return result.stdout


def archive_members(data):
    if data.startswith(b"\x60\xea"):
        # Two snapshot files named .zip are actually ARJ archives.
        listing = legacy_decode(data, "Arj", "l").decode("utf-8", errors="replace")
        members = []
        for block in listing.split("\n\n"):
            fields = dict(line.split(" = ", 1) for line in block.splitlines() if " = " in line)
            name = fields.get("Path", "")
            if not name.lower().endswith(".lmp") or fields.get("Folder") != "-":
                continue
            try:
                members.append({"name": name, "size": int(fields["Size"]),
                                "crc": int(fields["CRC"], 16), "format": "Arj"})
            except (KeyError, ValueError) as error:
                raise zipfile.BadZipFile("ARJ member lacks a valid size or CRC") from error
        return members
    with zipfile.ZipFile(io.BytesIO(data)) as archive:
        return [{"name": member.filename, "size": member.file_size, "crc": member.CRC,
                 "format": "zip", "zip_info": member} for member in archive.infolist()
                if member.filename.lower().endswith(".lmp")]


def archive_read(data, member):
    if member["format"] == "zip":
        try:
            with zipfile.ZipFile(io.BytesIO(data)) as archive:
                return archive.read(member["zip_info"])
        except NotImplementedError:
            pass
    decoded = legacy_decode(data, member["format"], "e", member["name"])
    if len(decoded) != member["size"] or zlib.crc32(decoded) != member["crc"]:
        raise zipfile.BadZipFile("Legacy decoded member size or CRC differs")
    return decoded


def header(data, game):
    # The reference executable requires version 109. Multiplayer command
    # streams and multi-level movies need separate harness support.
    if len(data) < 18 or data[0] != 109:
        raise ValueError("unsupported-version-or-short-demo")
    if data[4] or data[8] or data[9:13] != bytes([1, 0, 0, 0]):
        raise ValueError("multiplayer-or-deathmatch")
    episode, slot = data[2:4]
    if data[1] > 4 or (game == "doom" and not (1 <= episode <= 4 and 1 <= slot <= 9)) or (
            game == "doom2" and not (episode == 1 and 1 <= slot <= 32)):
        raise ValueError("invalid-skill-or-map")
    marker = next((i for i in range(13, len(data), 4) if data[i] == 128), None)
    if marker is None or marker == 13:
        raise ValueError("missing-marker-or-empty-demo")
    return {"map": f"E{episode}M{slot}" if game == "doom" else f"MAP{slot:02}",
            "tics": (marker - 13) // 4, "skill": data[1] + 1,
            "respawn": bool(data[5]), "fast": bool(data[6]), "nomonsters": bool(data[7])}


def prepare(archive, out):
    demos, skipped = {}, []
    out.mkdir(parents=True, exist_ok=True)
    (out / "demos").mkdir(exist_ok=True)
    with zipfile.ZipFile(archive) as outer:
        for item in sorted(outer.infolist(), key=lambda item: item.filename):
            parts = PurePosixPath(item.filename).parts
            if len(parts) != 4 or parts[0] != "compet-n" or parts[1] not in WADS or parts[2] not in CATEGORIES:
                continue
            game, category = parts[1:3]
            if not item.filename.lower().endswith(".zip"):
                continue
            try:
                archive_data = outer.read(item)
                for member in archive_members(archive_data):
                    source = {"archive_member": item.filename, "demo_member": member["name"],
                              "category": category}
                    try:
                        data = archive_read(archive_data, member)
                        info = header(data, game)
                    except (ValueError, NotImplementedError, zipfile.BadZipFile) as error:
                        skipped.append({**source, "reason": str(error)})
                        continue
                    digest = hashlib.sha256(data).hexdigest()
                    key = (game, digest)
                    if key in demos:
                        demos[key]["sources"].append(source)
                        continue
                    # Archive names are metadata, never extraction paths.
                    name = re.sub(r"[^A-Za-z0-9_-]", "_", PurePosixPath(member["name"]).stem)
                    prefix = "DOOMU" if game == "doom" else "DOOM2"
                    path = f"demos/{prefix}-{info['map']}-{name}-{digest[:12]}.lmp"
                    (out / path).write_bytes(data)
                    demos[key] = {"path": path, "wad": WADS[game], "game": game,
                                  "sha256": digest, **info, "sources": [source]}
            except (zipfile.BadZipFile, NotImplementedError) as error:
                skipped.append({"archive_member": item.filename, "reason": str(error)})
    entries = sorted(demos.values(), key=lambda demo: demo["path"])
    maps = sorted({(demo["game"], demo["map"]) for demo in entries})
    smoke, combat = [], []
    for game, map_name in maps:
        candidates = [demo for demo in entries if (demo["game"], demo["map"]) == (game, map_name)]
        speed = [demo for demo in candidates if any(s["category"] == "speed" for s in demo["sources"])]
        maximum = [demo for demo in candidates if any(s["category"] == "max" for s in demo["sources"])]
        smoke.append(min(speed or candidates, key=lambda demo: (demo["tics"], demo["path"])))
        if maximum:
            combat.append(min(maximum, key=lambda demo: (demo["tics"], demo["path"])))
    common = {"source_url": URL, "archive_sha256": ARCHIVE_SHA256,
              "snapshot_date": "2019-01-21", "demo_paths_relative_to": ".",
              "wad_paths_relative_to": "repository", "skipped": skipped}
    for name, selection in (("manifest", entries), ("smoke", smoke), ("combat", combat)):
        manifest = {**common, "demos": selection}
        (out / f"{name}.json").write_text(json.dumps(manifest, indent=2) + "\n")
    stats = {"unique_demos": len(entries), "by_game": dict(Counter(d["game"] for d in entries)),
             "maps": len(maps), "smoke_demos": len(smoke), "combat_demos": len(combat),
             "recorded_hours": round(sum(d["tics"] for d in entries) / 35 / 3600, 2),
             "skipped": len(skipped)}
    (out / "stats.json").write_text(json.dumps(stats, indent=2) + "\n")
    return stats


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--out", type=Path, default=ROOT / "tmp/compet-n")
    parser.add_argument("--archive", type=Path, help="Use a previously downloaded snapshot ZIP")
    args = parser.parse_args()
    archive = args.archive or args.out / "compet-n_2019-01-21.zip"
    if not archive.exists():
        if args.archive:
            parser.error(f"Archive does not exist: {archive}")
        archive.parent.mkdir(parents=True, exist_ok=True)
        partial = archive.with_suffix(".part")
        print(f"Downloading {URL}", flush=True)
        with urllib.request.urlopen(URL, timeout=60) as response, partial.open("wb") as file:
            while chunk := response.read(1024 * 1024):
                file.write(chunk)
        partial.rename(archive)
    with archive.open("rb") as file:
        digest = hashlib.file_digest(file, "sha256").hexdigest()
    if digest != ARCHIVE_SHA256:
        parser.error(f"Snapshot SHA-256 differs: {digest}; expected {ARCHIVE_SHA256}")
    print(json.dumps(prepare(archive, args.out), indent=2))
    print(f"Manifests: {args.out}/{{manifest,smoke,combat}}.json")


if __name__ == "__main__":
    main()
