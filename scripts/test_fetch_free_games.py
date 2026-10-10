"""Installer safety regressions; run: python3 -B scripts/test_fetch_free_games.py."""

import hashlib
import io
from pathlib import Path
import tempfile
import unittest
from unittest import mock
import warnings
import zipfile

import fetch_free_games as installer


def pinned(data, **fields):
    return {"size": len(data), "sha256": hashlib.sha256(data).hexdigest(), **fields}


def zip_bytes(entries):
    buffer = io.BytesIO()
    with warnings.catch_warnings():
        warnings.simplefilter("ignore", UserWarning)  # Deliberate duplicate member.
        with zipfile.ZipFile(buffer, "w") as archive:
            for name, data in entries:
                archive.writestr(name, data)
    return buffer.getvalue()


def fixture(root, bad_member=False):
    """Small synthetic pinned release; never downloads or uses real game data."""
    common = {"license": "Test license", "license_source": "https://example.test/license"}
    base_data, game_data = b"base fixture", b"game fixture"
    base = pinned(base_data, id="base", name="Base", filename="base.wad", **common)
    game = pinned(game_data, id="game", name="Game", filename="game.wad",
                  archive="release", member="release/game.wad", **common)
    notices = []
    entries = [(game["member"], b"bad! fixture" if bad_member else game_data)]
    for name in ("COPYING.txt", "CREDITS.txt", "CREDITS-MUSIC.txt"):
        data = (name + " test notice\n").encode()
        notices.append(pinned(data, filename=name, member="release/" + name))
        entries.append(("release/" + name, data))
    data = zip_bytes(entries)
    archive = pinned(data, id="release", filename="release.zip", notices=notices,
                     url="https://github.com/freedoom/freedoom/releases/download/v0.13.0/release.zip")
    cache = root / "cache"
    cache.mkdir()
    (cache / archive["filename"]).write_bytes(data)
    shareware = root / "source.wad"
    shareware.write_bytes(base_data)
    manifest = {"schema_version": 1, "archives": [archive], "games": [game], "shareware": base}
    return manifest, shareware, cache


class FreeGameInstallerTests(unittest.TestCase):
    def test_payload_rejects_wrong_size_or_same_size_tampering(self):
        expected = pinned(b"approved")
        for data, message in ((b"too short", "size mismatch"), (b"tampered", "SHA-256 mismatch")):
            with self.subTest(data=data), self.assertRaisesRegex(ValueError, message):
                installer.verify_bytes(data, expected, "test payload")

    def test_cache_rejects_tampering_without_network_fallback(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            manifest, _, cache = fixture(root)
            archive = manifest["archives"][0]
            path = cache / archive["filename"]
            path.write_bytes(b"x" * archive["size"])
            with mock.patch.object(installer.urllib.request, "urlopen") as download:
                with self.assertRaisesRegex(ValueError, "SHA-256 mismatch"):
                    installer.fetch_archive(archive, cache)
                download.assert_not_called()

    def test_selected_zip_member_rejects_duplicates(self):
        data = b"fixture"
        expected = pinned(data, member="release/game.wad")
        with zipfile.ZipFile(io.BytesIO(zip_bytes([(expected["member"], data)] * 2))) as archive:
            with self.assertRaisesRegex(ValueError, "exactly one"):
                installer.archive_member(archive, expected)

    def test_selected_zip_member_rejects_symlinks(self):
        data = b"target"
        entry = zipfile.ZipInfo("release/game.wad")
        entry.create_system = 3
        entry.external_attr = 0o120777 << 16
        with zipfile.ZipFile(io.BytesIO(zip_bytes([(entry, data)]))) as archive:
            with self.assertRaisesRegex(ValueError, "not a regular file"):
                installer.archive_member(archive, pinned(data, member=entry.filename))

    def test_selected_zip_member_rejects_traversal_and_absolute_paths(self):
        for name in ("../escape.wad", "release/../../escape.wad", "/escape.wad", "release\\escape.wad"):
            with self.subTest(member=name):
                with zipfile.ZipFile(io.BytesIO(zip_bytes([(name, b"fixture")]))) as archive:
                    with self.assertRaisesRegex(ValueError, "unsafe archive member"):
                        installer.archive_member(archive, pinned(b"fixture", member=name))

    def test_member_validation_failure_does_not_publish_or_replace_catalog(self):
        for existing in (False, True):
            with self.subTest(existing_catalog=existing), tempfile.TemporaryDirectory() as directory:
                root = Path(directory)
                manifest, shareware, cache = fixture(root, bad_member=True)
                output = root / "output"
                if existing:
                    output.mkdir()
                    (output / "catalog.json").write_bytes(b"previous catalog")
                    (output / "game.wad").write_bytes(b"previous game")
                with mock.patch.object(installer.urllib.request, "urlopen") as download:
                    with self.assertRaisesRegex(ValueError, "SHA-256 mismatch"):
                        installer.install(manifest, output, shareware, cache)
                    download.assert_not_called()
                if existing:
                    self.assertEqual((output / "catalog.json").read_bytes(), b"previous catalog")
                    self.assertEqual((output / "game.wad").read_bytes(), b"previous game")
                    self.assertEqual({p.name for p in output.iterdir()}, {"catalog.json", "game.wad"})
                else:
                    self.assertFalse(output.exists())

    def test_success_keeps_full_notices_and_emits_hash_approvals(self):
        import json

        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            manifest, shareware, cache = fixture(root)
            output = root / "output"
            with mock.patch.object(installer.urllib.request, "urlopen") as download, mock.patch("builtins.print"):
                installer.install(manifest, output, shareware, cache)
                download.assert_not_called()
            catalog = json.loads((output / "catalog.json").read_text())
            self.assertEqual([p["id"] for p in catalog["packs"]], ["base", "game"])
            for approval, game in zip(catalog["redistribution"], [manifest["shareware"], *manifest["games"]]):
                self.assertTrue(approval["allow"])
                self.assertEqual(approval["sha256"], game["sha256"])
                installer.verify_file(output / game["filename"], game)
            combined = (output / "free-game-notices.txt").read_bytes()
            for notice in manifest["archives"][0]["notices"]:
                data = installer.verify_file(output / "licenses/freedoom-0.13.0" / notice["filename"], notice)
                self.assertIn(data, combined)


if __name__ == "__main__":
    unittest.main()
