"""Corpus safety and routing checks; run with python3 -B scripts/test_demo_corpus.py."""

import hashlib
import gzip
import io
import json
from pathlib import Path
import shutil
import subprocess
import tempfile
import unittest
from unittest import mock
import zipfile

import demo_trace_compare_all as runner
import demo_trace_trim as trimmer
import fetch_compet_n as corpus


def demo_bytes(episode=1, map_number=1):
    return bytes([109, 3, episode, map_number, 0, 9, 7, 0, 0, 1, 0, 0, 0,
                  50, 0, 0, 0, 128])


def demo_zip(member, data):
    buffer = io.BytesIO()
    with zipfile.ZipFile(buffer, "w") as archive:
        archive.writestr(member, data)
    return buffer.getvalue()


class CorpusTests(unittest.TestCase):
    @unittest.skipUnless(shutil.which("7z") or shutil.which("7zz"), "7-Zip unavailable")
    def test_import_legacy_implode_and_arj_snapshot_entries(self):
        fixtures = Path(__file__).parent / "testdata/compet-n-legacy"
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            archive = root / "snapshot.zip"
            with zipfile.ZipFile(archive, "w") as outer:
                outer.writestr("compet-n/doom2/respawn/200-re08.zip",
                               (fixtures / "zip-implode.zip").read_bytes())
                outer.writestr("compet-n/doom2/nmare/nm23-058.zip",
                               (fixtures / "arj-disguised-as-zip.zip").read_bytes())
            stats = corpus.prepare(archive, root / "out")
            self.assertEqual((stats["unique_demos"], stats["skipped"]), (2, 0))
            demos = json.loads((root / "out/manifest.json").read_text())["demos"]
            self.assertEqual({d["sha256"] for d in demos}, {
                "25a57397f5a94d5f5e7cb1fac8821a5672ba259931ce39c5ac034d5be6a6137c",
                "4bcfb817e536fea455f8fb838dc6649ef09cab83ab044b6c3e71459d51b4da51"})
            self.assertEqual({d["map"] for d in demos}, {"MAP08", "MAP23"})
            self.assertTrue(next(d for d in demos if d["map"] == "MAP08")["respawn"])
            for demo in demos:
                self.assertEqual(hashlib.sha256((root / "out" / demo["path"]).read_bytes()).hexdigest(),
                                 demo["sha256"])

    def test_legacy_decode_uses_literal_members_and_stdout(self):
        member = "-o/tmp/unwanted*[member].lmp"
        result = subprocess.CompletedProcess([], 0, stdout=b"demo", stderr=b"")
        with mock.patch.object(corpus.shutil, "which", return_value="/usr/bin/7z"), \
                mock.patch.object(corpus.subprocess, "run", return_value=result) as run:
            self.assertEqual(corpus.legacy_decode(b"archive", "zip", "e", member), b"demo")
        args, kwargs = run.call_args
        command = args[0]
        self.assertIn("-so", command)
        self.assertIn("-spd", command)
        self.assertEqual(command[-3], "--")
        self.assertEqual(command[-1], member)
        self.assertEqual(kwargs["stdin"], subprocess.DEVNULL)

    def test_legacy_decode_rejects_crc_or_size_mismatch(self):
        data = (Path(__file__).parent / "testdata/compet-n-legacy/zip-implode.zip").read_bytes()
        member = corpus.archive_members(data)[0]
        with mock.patch.object(corpus, "legacy_decode", return_value=b"incorrect"):
            with self.assertRaisesRegex(zipfile.BadZipFile, "size or CRC differs"):
                corpus.archive_read(data, member)

    def test_legacy_decode_reports_missing_optional_decoder(self):
        with mock.patch.object(corpus.shutil, "which", return_value=None):
            with self.assertRaisesRegex(NotImplementedError, "requires 7z or 7zz"):
                corpus.legacy_decode(b"archive", "zip", "e", "demo.lmp")

    def test_batch_schedules_manifest_priority_and_filters_without_resorting(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            wad = root / "DOOM2.WAD"
            wad.write_bytes(b"test wad")
            tools = [root / name for name in ["harness", "comparator", "trimmer"]]
            for tool in tools:
                tool.write_bytes(b"test tool")
            entries = []
            for name in ["z-priority", "a-later", "m-last"]:
                demo = root / (name + ".lmp")
                demo.write_bytes(demo_bytes())
                entries.append({"path": demo.name, "wad": wad.name,
                                "sha256": runner.sha256(demo)})
            manifest = root / "manifest.json"
            manifest.write_text(json.dumps({"demos": entries}))
            for filters, expected in [([], ["z-priority", "a-later", "m-last"]),
                                      (["--demo", "m-last", "--demo", "z-priority"],
                                       ["z-priority", "m-last"])]:
                scheduled = []
                with self.subTest(filters=filters), \
                        mock.patch.object(runner, "ROOT", root), \
                        mock.patch.object(runner.subprocess, "run"), \
                        mock.patch.object(runner, "pinned_replay_tools", return_value=tools), \
                        mock.patch.object(runner, "ThreadPoolExecutor") as executor, \
                        mock.patch.object(runner.sys, "argv", ["runner", "--manifest", str(manifest),
                            "--gd-bin", str(tools[0]), "--ref-bin", str(tools[0]),
                            "--out-root", str(root / "out"), *filters]):
                    def capture_queue(compare, demos):
                        scheduled.extend(demo.stem for demo in demos)
                        return []
                    executor.return_value.__enter__.return_value.map.side_effect = capture_queue
                    self.assertEqual(runner.main(), 0)
                self.assertEqual(scheduled, expected)

    def test_in_place_trim_preserves_legacy_comparison_prefix(self):
        meta = b'{"kind":"meta"}\n'
        alive = b'{"kind":"tic","player":{"playerstate":0}}\n'
        other = b'{"kind":"tic","player":{"playerstate":10}}\n'
        dead = b'{"kind":"tic","player":{"playerstate":1,"health":0}}\n'
        for contents in [meta + alive * 4, meta + alive + dead + alive * 10000 + dead,
                         meta + other + dead, meta + b'{"kind":"meta","player":{"playerstate":1}}\n' + alive]:
            for max_tics in [0, 1, 3, 10001]:
                with self.subTest(size=len(contents), max_tics=max_tics), tempfile.TemporaryDirectory() as directory:
                    trace = Path(directory) / "trace.jsonl"
                    trace.write_bytes(contents)
                    inode = trace.stat().st_ino
                    if max_tics:
                        program = f'{{ if ($0 ~ /"kind":"tic"/ && ++n > {max_tics}) exit; print }}'
                    else:
                        program = r'{ print; if ($0 ~ /"kind":"tic"/ && $0 ~ /"player":\{"playerstate":1([,}])/) exit }'
                    expected = subprocess.run(["awk", program], input=contents,
                                              stdout=subprocess.PIPE, check=True).stdout
                    trimmer.trim(trace, max_tics)
                    self.assertEqual(trace.read_bytes(), expected)
                    self.assertEqual(trace.stat().st_ino, inode)
                    trimmer.trim(trace, max_tics)
                    self.assertEqual(trace.read_bytes(), expected)

    def test_batch_pins_in_place_trimmer_and_valid_shell(self):
        with tempfile.TemporaryDirectory() as directory:
            tools = Path(directory) / ".tmp"
            tools.mkdir()
            (tools / "demotracecmp").write_bytes(b"test comparator")
            harness, comparator, trim = runner.pinned_replay_tools(tools)
            self.assertIn(trim.name, harness.read_text())
            self.assertIn(comparator.name, harness.read_text())
            self.assertEqual(trim.read_bytes(), (runner.ROOT / "scripts/demo_trace_trim.py").read_bytes())
            subprocess.run(["bash", "-n", str(harness)], check=True)

    def test_batch_tool_snapshot_survives_source_edits(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            source = root / "live.sh"
            source.write_bytes(b"#!/bin/sh\nprintf original\\n\n")
            first = runner.immutable_snapshot(source.read_bytes(), root, "harness")
            original = first.read_bytes()
            source.write_bytes(b"#!/bin/sh\nprintf changed\\n\n")
            second = runner.immutable_snapshot(source.read_bytes(), root, "harness")
            self.assertNotEqual(first, second)
            self.assertEqual(first.read_bytes(), original)
            self.assertEqual(second.read_bytes(), source.read_bytes())
            self.assertEqual(runner.immutable_snapshot(source.read_bytes(), root, "harness"), second)
            self.assertTrue(first.stat().st_mode & 0o111)

    def test_compress_retained_traces_preserves_complete_evidence(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            contents = b'{"kind":"meta"}\n' + b'{"kind":"tic","gametic":3}\n' * 1000
            trace = root / "reference-check.jsonl"
            trace.write_bytes(contents)
            log = root / "compare.log"
            log.write_text("mismatch line=4 path=root.health\n")
            self.assertEqual(runner.compress_retained_traces(root), ["reference-check.jsonl.gz"])
            self.assertFalse(trace.exists())
            with gzip.open(root / "reference-check.jsonl.gz", "rb") as file:
                self.assertEqual(file.read(), contents)
            self.assertEqual(log.read_text(), "mismatch line=4 path=root.health\n")
            self.assertEqual(runner.compress_retained_traces(root), [])

    def test_filter_deduplicate_and_route_untrusted_archive_members(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            archive = root / "snapshot.zip"
            single = demo_bytes(4, 9)
            multiplayer = bytearray(single)
            multiplayer[10] = 1
            modern = bytearray(single)
            modern[0] = 110
            with zipfile.ZipFile(archive, "w") as outer:
                outer.writestr("compet-n/doom/speed/a.zip", demo_zip("../../escape.LMP", single))
                outer.writestr("compet-n/doom/max/b.zip", demo_zip("alias.lmp", single))
                outer.writestr("compet-n/doom2/max/c.zip", demo_zip("map32.lmp", demo_bytes(1, 32)))
                outer.writestr("compet-n/doom/max/coop.zip", demo_zip("coop.lmp", multiplayer))
                outer.writestr("compet-n/doom/max/modern.zip", demo_zip("modern.lmp", modern))
                outer.writestr("compet-n/doom/max/broken.zip", b"broken ZIP")
                outer.writestr("compet-n/pwads/av/max/d.zip", demo_zip("custom.lmp", single))
                outer.writestr("compet-n/doom/movie/movie.zip", demo_zip("movie.lmp", single))
            stats = corpus.prepare(archive, root / "out")
            self.assertEqual(stats["unique_demos"], 2)
            self.assertEqual(stats["skipped"], 3)
            manifest_path = root / "out/manifest.json"
            manifest = json.loads(manifest_path.read_text())
            ultimate = next(d for d in manifest["demos"] if d["game"] == "doom")
            self.assertEqual(ultimate["map"], "E4M9")
            self.assertTrue(ultimate["respawn"] and ultimate["fast"])
            self.assertEqual(len(ultimate["sources"]), 2)
            self.assertFalse((root / "escape.LMP").exists())
            routed = runner.manifest_inputs(manifest_path)
            self.assertEqual(set(routed.values()), {corpus.ROOT / wad for wad in corpus.WADS.values()})
            path = root / "out" / ultimate["path"]
            self.assertEqual(hashlib.sha256(path.read_bytes()).hexdigest(), ultimate["sha256"])
            path.write_bytes(b"changed")
            with self.assertRaisesRegex(ValueError, "hash differs"):
                runner.manifest_inputs(manifest_path)

    def test_reject_invalid_maps_and_incomplete_command_streams(self):
        with self.assertRaisesRegex(ValueError, "invalid-skill-or-map"):
            corpus.header(demo_bytes(5, 1), "doom")
        with self.assertRaisesRegex(ValueError, "invalid-skill-or-map"):
            corpus.header(demo_bytes(1, 33), "doom2")
        with self.assertRaisesRegex(ValueError, "missing-marker"):
            corpus.header(demo_bytes()[:-1] + b"\x00", "doom")

    def test_detect_finale_replay_replacing_requested_map(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            demo = root / "boss.lmp"
            demo.write_bytes(demo_bytes(1, 8))
            trace = root / "trace.jsonl"
            trace.write_text('{"kind":"meta","map":"E1M5"}\n')
            self.assertIn("expected E1M8", runner.replay_map_error(demo, Path("DOOMU.WAD"), trace))
            trace.write_text('{"kind":"meta","map":"E1M8"}\n')
            self.assertIsNone(runner.replay_map_error(demo, Path("DOOMU.WAD"), trace))


if __name__ == "__main__":
    unittest.main()
