#!/usr/bin/env python3
"""Small deterministic checks for the visual report's difference metrics."""
import importlib.util
import json
import random
from pathlib import Path
import tempfile
import unittest

from PIL import Image

spec = importlib.util.spec_from_file_location("visual_compare", Path(__file__).with_name("visual_compare.py"))
visual_compare = importlib.util.module_from_spec(spec)
spec.loader.exec_module(visual_compare)
trace_spec = importlib.util.spec_from_file_location("trace_spans", Path(__file__).parent / "visual_compare/trace_spans.py")
trace_spans = importlib.util.module_from_spec(trace_spec)
trace_spec.loader.exec_module(trace_spans)
pixel_spec = importlib.util.spec_from_file_location("trace_pixels", Path(__file__).parent / "visual_compare/trace_pixels.py")
trace_pixels = importlib.util.module_from_spec(pixel_spec)
pixel_spec.loader.exec_module(trace_pixels)


class VisualCompareTests(unittest.TestCase):
    def test_metrics_histogram_matches_direct_rgb_comparison(self):
        rng = random.Random(42)
        a = Image.frombytes("RGB", (17, 9), bytes(rng.randrange(256) for _ in range(17*9*3)))
        b = a.copy()
        for x in range(17):
            b.putpixel((x, x % 9), tuple(min(255, v + x % 5) for v in a.getpixel((x, x % 9))))
        expected = [tuple(abs(x-y) for x, y in zip(p, q)) for p, q in zip([a.getpixel((x, y)) for y in range(9) for x in range(17)], [b.getpixel((x, y)) for y in range(9) for x in range(17)])]
        metrics, _ = visual_compare.compare(a, b)
        self.assertEqual(metrics["different_percent"], 100 * sum(max(p) > 0 for p in expected) / len(expected))
        self.assertEqual(metrics["over_2_percent"], 100 * sum(max(p) > 2 for p in expected) / len(expected))
        self.assertEqual(metrics["mean_absolute_error"], sum(map(sum, expected)) / (3*len(expected)))

    def test_report_preserves_nonstandard_dimensions(self):
        with tempfile.TemporaryDirectory() as tmp:
            out = Path(tmp)
            for backend in ("classic", "cpu", "gpu"):
                ext = "ppm" if backend == "classic" else "png"
                Image.new("RGB", (426, 240), "gray").save(out / f"scene-{backend}.{ext}")
            rows = visual_compare.report(out, [{"name": "scene"}], {"dimensions": [426, 240]})
            self.assertEqual(rows[0]["metrics"]["classic-cpu"]["different_percent"], 0)
            with Image.open(out / "scene-comparison.png") as im:
                self.assertEqual(im.size, (1278, 264))
            self.assertIn("426×240", (out / "index.html").read_text())
            self.assertIn("aspect-ratio:426/240", (out / "index.html").read_text())

    def test_matrix_covers_every_map_and_each_backend_lighting_size_combination(self):
        spec = importlib.util.spec_from_file_location("visual_matrix", Path(__file__).with_name("visual_matrix.py"))
        module = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(module)
        runs = list(module.matrix(Path("DOOMU.WAD"), Path("DOOM2.WAD")))
        self.assertEqual(len(runs), 30)
        self.assertEqual(len({r["name"] for r in runs}), 30)
        for lighting, disabled in ((False, False), (True, False), (False, True)):
            native = [r for r in runs if r["resolution"] == "320x200" and r["lighting"] == lighting and r["disable_lighting"] == disabled]
            self.assertEqual([len(r["maps"]) for r in native], [36, 32])
            for size in ("256x128", "400x250", "426x240", "640x400"):
                selected = [r for r in runs if r["resolution"] == size and r["lighting"] == lighting and r["disable_lighting"] == disabled]
                self.assertEqual([len(r["maps"]) for r in selected], [8, 8])

    def test_pixel_trace_exempts_only_proven_out_of_buffer_reads(self):
        with tempfile.TemporaryDirectory() as tmp:
            out = Path(tmp)
            (out / "scenes.json").write_text(json.dumps([{"name": "scene"}]))
            Image.new("RGB", (2, 1), "white").save(out / "scene-classic.png")
            Image.new("RGB", (2, 1), "black").save(out / "scene-cpu.png")
            (out / "scene-classic-pixels.csv").write_text(
                "x,y,kind,owner,source,row,result,texel,available\n"
                "0,0,1,0,100,0,100,127,96\n"
                "1,0,1,0,100,0,100,127,-1\n")
            self.assertEqual(trace_pixels.summarize(out)[0]["counts"],
                             dict(bounded_differences=1, undefined_differences=1,
                                  classic_out_of_bounds_reads=1))

    def test_span_trace_wraps_fixed_coordinates_and_texture_indices(self):
        with tempfile.TemporaryDirectory() as tmp:
            path = Path(tmp) / "spans.csv"
            path.write_text("plane,height,flat,y,x1,x2,u,v,du,dv\n0,0,FLAT,150,12,13,-1,4294967295,65536,1\n")
            pixels = trace_spans.expand(path)
            self.assertEqual(pixels[12, 150][1:], (0xffffffff, 0xffffffff, 4095))
            self.assertEqual(pixels[13, 150][1:], (65535, 0, 0))

    def test_span_trace_detects_subtexel_errors_even_when_images_match(self):
        with tempfile.TemporaryDirectory() as tmp:
            out = Path(tmp)
            (out / "scenes.json").write_text(json.dumps([{"name": "scene", "Z": 41}]))
            for backend, u in (("classic", 100), ("cpu", 101)):
                Image.new("RGB", (320, 200)).save(out / f"scene-{backend}.png")
                (out / f"scene-{backend}-spans.csv").write_text(
                    f"plane,height,flat,y,x1,x2,u,v,du,dv\n0,0,FLAT,150,0,1,{u},0,65536,0\n")
            counts = trace_spans.summarize(out)[0]["counts"]
            self.assertEqual(counts["shared_plane_pixels"], 2)
            self.assertEqual(counts["uv_differences"], 2)
            self.assertEqual(counts["texel_differences"], 0)
            self.assertEqual(counts["visible_texel_differences"], 0)

    def test_metrics_distinguish_exact_and_tolerated_differences(self):
        a = Image.new("RGB", (2, 1))
        b = Image.new("RGB", (2, 1))
        b.putpixel((0, 0), (2, 0, 0))
        b.putpixel((1, 0), (0, 3, 0))
        metrics, _ = visual_compare.compare(a, b)
        self.assertEqual(metrics["different_percent"], 100)
        self.assertEqual(metrics["over_2_percent"], 50)
        self.assertAlmostEqual(metrics["mean_absolute_error"], 5 / 6)

    def test_report_keeps_classic_and_backend_errors_separate(self):
        with tempfile.TemporaryDirectory() as tmp:
            out = Path(tmp)
            Image.new("RGB", (320, 200), (10, 20, 30)).save(out / "scene-classic.ppm")
            for backend in ("cpu", "gpu"):
                Image.new("RGB", (320, 200), (40, 50, 60)).save(out / f"scene-{backend}.png")
            results = visual_compare.report(out, [{"name": "scene"}], {"wad_sha256": "fixture"})
            self.assertEqual(results[0]["metrics"]["classic-cpu"]["over_2_percent"], 100)
            self.assertEqual(results[0]["metrics"]["cpu-gpu"]["over_2_percent"], 0)
            self.assertTrue((out / "scene-comparison.png").is_file())
            self.assertIn('id="opacity"', (out / "index.html").read_text())
            self.assertEqual(json.loads((out / "results.json").read_text())["metadata"]["wad_sha256"], "fixture")

    def test_demo_camera_check_rejects_even_one_fixed_point_unit(self):
        with tempfile.TemporaryDirectory() as tmp:
            out = Path(tmp)
            path = out / "classic-cameras.csv"
            path.write_text("name,x,y,z,angle,leveltime,dead\nscene,65536,0,0,123,50,1\n")
            scene = dict(name="scene", X=1, Y=0, Z=0, angle=123, level_time=50, dead=True)
            visual_compare.verify_demo_cameras(out, [scene])
            scene["Z"] = 1/65536
            with self.assertRaisesRegex(RuntimeError, "Demo camera scene z"):
                visual_compare.verify_demo_cameras(out, [scene])

    def test_reference_patch_rejects_ambiguous_source(self):
        with tempfile.TemporaryDirectory() as tmp:
            path = Path(tmp) / "probe.c"
            path.write_text("hook hook")
            with self.assertRaises(RuntimeError):
                visual_compare.replace_once(path, "hook", "changed")
            self.assertEqual(path.read_text(), "hook hook")


if __name__ == "__main__":
    unittest.main()
