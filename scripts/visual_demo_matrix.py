#!/usr/bin/env python3
"""Replay bundled demos visually, including forced deaths and optional natural-death recordings."""
import argparse
import html
import json
from pathlib import Path
import subprocess
import sys

from demo_trace_compare_all import wad_for

ROOT = Path(__file__).resolve().parents[1]


def write_summary(out, runs):
    (out / "matrix.json").write_text(json.dumps(runs, indent=2))
    rows = ''.join(f'<tr><td><a href="{html.escape(r["name"])}/index.html">{html.escape(r["name"])}</a></td><td>{r.get("scenes", "—")}</td><td>{r.get("exact_classic", "—")}</td><td>{html.escape(r["status"])}</td></tr>' for r in runs)
    (out / "index.html").write_text('''<!doctype html><meta charset="utf-8"><title>Demo and death comparisons</title>
<style>body{background:#16191d;color:#eee;font:15px system-ui;margin:32px}a{color:#9cf}td,th{text-align:left;padding:9px;border-bottom:1px solid #39404a}table{border-collapse:collapse}p{max-width:1000px}</style>
<h1>Demo and death comparisons</h1><p>Independent classic and GD simulation, then CPU/shader world captures. Camera position, eye height, angle, level time and death state must match exactly. Actors/HUD and player palette effects are omitted at capture. Native 320×200 uses original lighting; 426×240 uses the runtime lighting-disabled option and a resolution-adapted reference.</p>
<p>Bundled demos are sampled after 35, 140 and 350 inputs, with normal playback and 35-tic forced-death continuations. Optional corpus recordings add actual deaths and respawn. Each differing image is traced: only proven out-of-buffer classic reads are exempt; RGB metrics retain every difference.</p>
<table><tr><th>Recording / mode</th><th>Scenes</th><th>Exact classic</th><th>Validation</th></tr>''' + rows + '</table><p><a href="matrix.json">Results</a></p>')


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--out', type=Path, default=ROOT / 'tmp/visual-demo-matrix')
    parser.add_argument('--corpus', type=Path, help='COMPET-N manifest for two natural-death recordings')
    parser.add_argument('--only', help='comma-separated exact run names to recapture')
    args = parser.parse_args()
    demos = [(p, wad_for(p), '35,140,350', '0,35') for p in sorted((ROOT/'demos').rglob('*.lmp'))]
    if args.corpus:
        selected = {'DOOMU-E4M7-n4m7-gg-3948d2ec6e4f': '53,54,55,72,89,110',
                    'DOOM2-MAP26-NM26-119-800f9a88b4bd': '396,397,398,415,432,450'}
        for entry in json.loads(args.corpus.read_text())['demos']:
            path = (args.corpus.parent / entry['path']).resolve()
            if path.stem in selected:
                demos.append((path, ROOT / entry['wad'], selected.pop(path.stem), '0'))
        if selected:
            parser.error(f'Natural-death inputs absent: {list(selected)}')
    runs = [dict(name=f'{p.stem}-{mode}', demo=str(p), wad=str(wad), tics=tics, death_tics=death,
                 mode=mode, resolution=size, status='pending')
            for p, wad, tics, death in demos
            for mode,size in [('lit','320x200'),('unlit','426x240')]]
    out=args.out.resolve();out.mkdir(parents=True,exist_ok=True)
    only=set(args.only.split(',')) if args.only else {r['name'] for r in runs}
    if only-{r['name'] for r in runs}: parser.error('Unknown run name')
    if args.only and (out/'matrix.json').exists():
        old={r['name']:r for r in json.loads((out/'matrix.json').read_text())}
        for r in runs:
            if r['name'] not in only: r.update(old.get(r['name'],dict(status='not run')))
    failed=False
    for r in runs:
        if r['name'] not in only: continue
        dest=out/r['name']
        base=[sys.executable,str(ROOT/'scripts/visual_compare.py'),'--wad',r['wad'],'--demo',r['demo'],
              '--demo-tics',r['tics'],'--death-tics',r['death_tics'],'--resolution',r['resolution'],
              '--require-gpu-parity', '--lighting' if r['mode']=='lit' else '--disable-lighting']
        try:
            r['status']='capturing';write_summary(out,runs)
            subprocess.run(base+['--out',str(dest)],check=True,cwd=ROOT)
            results=json.loads((dest/'results.json').read_text())['results']
            differing=[a['scene'] for a in results if a['metrics']['classic-cpu']['different_percent']]
            r.update(scenes=len(results),exact_classic=len(results)-len(differing),
                     max_cpu_gpu_percent=max(a['metrics']['cpu-gpu']['different_percent'] for a in results))
            if differing:
                manifest=dest/'differing-scenes.json';manifest.write_text(json.dumps(differing,indent=2))
                subprocess.run(base+['--out',str(dest/'trace'),'--scenes',str(manifest),
                                    '--require-bounded-parity','--require-plane-parity'],check=True,cwd=ROOT)
                r['status']='pass: bounded pixels + shared plane UVs exact'
            else: r['status']='pass: all pixels exact'
        except (subprocess.CalledProcessError, OSError, ValueError) as e:
            r['status']='FAILED';r['error']=str(e);failed=True
        write_summary(out,runs)
    print(f'Demo matrix: {out / "index.html"}',flush=True)
    if failed: raise SystemExit('One or more demo runs failed; reports retained')


if __name__=='__main__': main()
