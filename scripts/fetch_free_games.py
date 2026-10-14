#!/usr/bin/env python3
"""Install the pinned free-game catalog and its complete redistribution notices.

Only the release archives and exact members listed in internal/freegames/catalog.json
are accepted. The generated catalog.json can be passed directly to gdlobby -catalog.
"""

import argparse
import hashlib
import json
import os
from pathlib import Path, PurePosixPath
import stat
import tempfile
import urllib.request
import zipfile


MANIFEST = Path(__file__).resolve().parents[1] / "internal/freegames/catalog.json"


def verify_bytes(data, expected, label):
    if len(data) != expected["size"]:
        raise ValueError(f"{label}: size mismatch (expected {expected['size']}, got {len(data)})")
    if hashlib.sha256(data).hexdigest() != expected["sha256"]:
        raise ValueError(f"{label}: SHA-256 mismatch")
    return data


def verify_file(path, expected):
    # Bound even local/cache reads: an incorrect file must not consume arbitrary RAM.
    with path.open("rb") as source:
        data = source.read(expected["size"] + 1)
    return verify_bytes(data, expected, str(path))


def filename(value):
    if not value or value in {".", ".."} or "/" in value or "\\" in value:
        raise ValueError(f"unsafe destination filename: {value!r}")
    return value


def archive_member(archive, expected):
    member = expected["member"]
    parts = PurePosixPath(member).parts
    if member.startswith("/") or "\\" in member or ".." in parts or not parts:
        raise ValueError(f"unsafe archive member: {member!r}")
    matches = [entry for entry in archive.infolist() if entry.filename == member]
    if len(matches) != 1:
        raise ValueError(f"archive must contain exactly one {member!r}")
    entry = matches[0]
    mode = entry.external_attr >> 16
    if entry.is_dir() or (stat.S_IFMT(mode) and not stat.S_ISREG(mode)):
        raise ValueError(f"archive member is not a regular file: {member}")
    if entry.file_size != expected["size"]:
        raise ValueError(f"{member}: archive size mismatch")
    with archive.open(entry) as source:
        data = source.read(expected["size"] + 1)
    return verify_bytes(data, expected, member)


def fetch_archive(expected, cache):
    path = cache / filename(expected["filename"])
    if path.exists():
        verify_file(path, expected)
        return path
    url = expected["url"]
    if not url.startswith("https://github.com/freedoom/freedoom/releases/download/v0.13.0/"):
        raise ValueError("archive URL is not a pinned official Freedoom 0.13.0 release")
    print(f"Downloading {expected['filename']}...", flush=True)
    request = urllib.request.Request(url, headers={"User-Agent": "GD-DOOM-free-games/1"})
    with urllib.request.urlopen(request, timeout=120) as response:
        if not response.url.startswith("https://"):
            raise ValueError("archive download redirected away from HTTPS")
        data = response.read(expected["size"] + 1)
    verify_bytes(data, expected, url)
    with tempfile.NamedTemporaryFile(dir=cache, prefix=".download-", delete=False) as target:
        temporary = Path(target.name)
        try:
            target.write(data)
            target.flush()
            os.fsync(target.fileno())
        except BaseException:
            temporary.unlink(missing_ok=True)
            raise
    try:
        os.replace(temporary, path)
    finally:
        temporary.unlink(missing_ok=True)
    return path


def notices_text(documents):
    text = "Freedoom: Phase 1, Freedoom: Phase 2, and FreeDM 0.13.0\n"
    text += "Project: https://freedoom.github.io/\n"
    text += "Release: https://github.com/freedoom/freedoom/releases/tag/v0.13.0\n"
    for name in ("COPYING.txt", "CREDITS.txt", "CREDITS-MUSIC.txt"):
        text += f"\n===== {name} =====\n\n" + documents[name].decode("utf-8") + "\n"
    return text.encode("utf-8")


def install(manifest, output, shareware, cache):
    if manifest["schema_version"] != 1:
        raise ValueError("unsupported free-game catalog schema")
    shareware_data = verify_file(shareware, manifest["shareware"])
    archives = {entry["id"]: entry for entry in manifest["archives"]}
    paths = {key: fetch_archive(entry, cache) for key, entry in archives.items()}
    documents = {}
    files = {filename(manifest["shareware"]["filename"]): shareware_data}
    for key, path in paths.items():
        with zipfile.ZipFile(path) as archive:
            for notice in archives[key]["notices"]:
                name = filename(notice["filename"])
                data = archive_member(archive, notice)
                if name in documents and documents[name] != data:
                    raise ValueError(f"release archives disagree on {name}")
                documents[name] = data
            for game in manifest["games"]:
                if game["archive"] == key:
                    files[filename(game["filename"])] = archive_member(archive, game)
    if len(files) != 1 + len(manifest["games"]):
        raise ValueError("catalog contains missing or duplicate game files")

    # Files are never extracted using ZIP paths. Only these explicitly selected,
    # verified payloads are written, with the room catalog published last.
    for name, data in documents.items():
        files["licenses/freedoom-0.13.0/" + name] = data
    files["free-game-notices.txt"] = notices_text(documents)
    entries = [manifest["shareware"], *manifest["games"]]
    room_catalog = {
        "packs": [
            {"id": game["id"], "name": game["name"], "wads": [game["filename"]]}
            for game in entries
        ],
        "redistribution": [
            {
                "sha256": game["sha256"],
                "name": game["filename"],
                "allow": True,
                "license": game["license"],
                "source": game["license_source"],
            }
            for game in entries
        ],
    }
    files["catalog.json"] = (json.dumps(room_catalog, indent=2) + "\n").encode("utf-8")
    output.mkdir(parents=True, exist_ok=True)
    root = output.resolve()
    for name in files:
        if not (output / name).parent.resolve().is_relative_to(root):
            raise ValueError(f"output directory contains an escaping directory symlink: {name}")
    with tempfile.TemporaryDirectory(prefix=".free-games-", dir=output) as temporary:
        staging = Path(temporary)
        for name, data in files.items():
            path = staging / name
            path.parent.mkdir(parents=True, exist_ok=True)
            path.write_bytes(data)
            path.chmod(0o644)
        for name in files:
            destination = output / name
            destination.parent.mkdir(parents=True, exist_ok=True)
            # Do not follow an existing file symlink while replacing a payload.
            os.replace(staging / name, destination)
    print(f"Installed Doom Shareware and {len(manifest['games'])} standalone free games in {output}")
    print(f"Room catalog: {output / 'catalog.json'}")
    print(f"Retain and publish: {output / 'free-game-notices.txt'}")


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("output_dir", type=Path, help="directory for WADs, notices, and gdlobby catalog.json")
    parser.add_argument("--shareware", required=True, type=Path, help="existing unmodified Doom 1.9 DOOM1.WAD")
    parser.add_argument("--cache-dir", type=Path, help="reuse checksum-verified official release ZIPs")
    args = parser.parse_args()
    manifest = json.loads(MANIFEST.read_text(encoding="utf-8"))
    try:
        if args.cache_dir:
            args.cache_dir.mkdir(parents=True, exist_ok=True)
            install(manifest, args.output_dir, args.shareware, args.cache_dir)
        else:
            with tempfile.TemporaryDirectory(prefix="gd-doom-free-games-") as cache:
                install(manifest, args.output_dir, args.shareware, Path(cache))
    except (OSError, ValueError, zipfile.BadZipFile) as error:
        parser.exit(1, f"Free-game installation failed: {error}\n")


if __name__ == "__main__":
    main()
