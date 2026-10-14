#!/usr/bin/env python3
"""Audit Modern artwork/lighting against classic, allowing one logical pixel of sampling drift."""
import argparse
import csv
import hashlib
import json
import os
from pathlib import Path

from PIL import Image, ImageChops
from visual_compare import ROOT, run


def unmatched(a, b):
    # A color may move by one logical texel due to fractional placement and
    # different pixel-center rasterization. No color/brightness tolerance.
    w, h = a.size
    padded = Image.new('RGB', (w+2,h+2)); padded.paste(b,(1,1))
    bad = Image.new('L',(w,h),255)
    for y in range(3):
        for x in range(3):
            channels = ImageChops.difference(a,padded.crop((x,y,x+w,y+h))).split()
            delta = ImageChops.lighter(ImageChops.lighter(channels[0],channels[1]),channels[2])
            bad = ImageChops.darker(bad,delta.point(lambda v:255 if v else 0))
    return bad


def silhouette(img):
    r, g, b = img.split()
    return ImageChops.lighter(ImageChops.lighter(r,g),b).point(lambda v:255 if v else 0).convert('RGB')


def accuracy_mask(classic, modern, modern_coverage=None):
    # Downsampling a modern capture can skip a classic texel. Require every
    # observed Modern color to be locally valid, and require classic's shape
    # to remain present; do not require every original color to survive that
    # sampling step. The reverse color discrepancy stays visible in the report.
    colors = unmatched(modern, classic)
    shape = silhouette(classic)
    coverage = unmatched(shape, modern_coverage if modern_coverage is not None else silhouette(modern))
    coverage = ImageChops.multiply(coverage, shape.getchannel("R"))
    return ImageChops.lighter(colors,coverage)


def normalize(img, width, height, resample=Image.Resampling.NEAREST):
    scale = min(width/320, height/240)
    # Undo Modern's aspect correction, horizontal centering, bottom anchor and
    # intentional eight-unit fullscreen lift and classic's 100.5 vs 100
    # vertical sampling anchor. No best-fit/image registration.
    left, top = (width-320*scale)/2, height-240*scale
    return img.transform((320, 200), Image.Transform.AFFINE,
                         (scale, 0, left, 0, scale*1.2, top-7.5*scale*1.2),
                         resample=resample)


def main():
    ap = argparse.ArgumentParser(description=__doc__)
    ap.add_argument('--wad', type=Path, required=True)
    ap.add_argument('--reference', type=Path, required=True, help='matching presentation_compare.py report directory')
    ap.add_argument('--out', type=Path, required=True)
    ap.add_argument('--native', action='store_true')
    ap.add_argument('--analyze-only', action='store_true', help='reanalyze existing captures')
    args = ap.parse_args()
    wad, ref, out = args.wad.resolve(), args.reference.resolve(), args.out.resolve()
    metadata = json.loads((ref/'results.json').read_text())['metadata']
    if metadata['wad_sha256'] != hashlib.sha256(wad.read_bytes()).hexdigest():
        ap.error('classic reference uses a different WAD')
    out.mkdir(parents=True, exist_ok=True)
    rows = list(csv.DictReader((ref/'weapons.csv').open()))
    selected = {'PUNGA0','PISGA0','SHTGA0','SHT2A0','CHGGA0','MISGA0','PLSGA0','BFGGA0','SAWGC0',
                'PISGC0','SHTGD0','SHT2E0','PISFA0','SHTFA0','CHGFA0','MISFC0','PLSFA0','BFGFB0'}
    cases, seen = [], set()
    for width, height in ((640,480),(960,540),(1280,720)):
        for row in rows:
            if row['patch'] not in selected or '-p1-' not in row['name'] or int(row['light']) not in (64,192):
                continue
            key = (width, row['patch'], row['light'])
            if key in seen:
                continue
            seen.add(key)
            for lighting in (True, False):
                reference = row['name'] if lighting else row['name'][:-1]+'4'
                cases.append(dict(Name=f"case-{len(cases):03}", Reference=reference, Patch=row['patch'],
                                  Width=width, Height=height, Light=int(row['light']),
                                  X=int(row['x']), Y=int(row['y']), Lighting=lighting))
    (out/'cases.json').write_text(json.dumps(cases, indent=2))
    backends = ['cpu','shader'] + (['native'] if args.native else [])
    for c in ([] if args.analyze_only else cases):
        for backend in backends:
            (out/f"{c['Name']}-{backend}.png").unlink(missing_ok=True)
    env = dict(os.environ, GD_MODERN_OUT=str(out), GD_MODERN_WAD=str(wad))
    if not args.analyze_only:
        run(['xvfb-run','-a','go','test','-tags','integration','./internal/doomruntime',
             '-run','^TestModernPresentationCapture$','-count=1','-v'], out/'capture.log', env=env)
        if args.native:
            run(['xvfb-run','-a','go','test','-tags','raylib integration','./internal/doomruntime',
                 '-run','^TestModernNativePresentationCapture$','-count=1','-v'], out/'native.log', env=env)
    results = []
    for c in cases:
        classic = Image.open(ref/f"{c['Reference']}-classic.png").convert('RGB')
        classic.save(out/f"{c['Name']}-classic.png")
        a = classic
        # Modern's raised weapon reveals more bottom artwork. Comparing after
        # undoing that lift retains all of classic's visible rows.
        for backend in backends:
            physical = Image.open(out/f"{c['Name']}-{backend}.png").convert('RGB')
            actual = normalize(physical,c['Width'],c['Height'])
            # Coverage includes pixels touched by the normalization footprint;
            # nearest color sampling can skip isolated muzzle-flash flecks.
            coverage = silhouette(normalize(silhouette(physical),c['Width'],c['Height'],Image.Resampling.BILINEAR))
            actual.save(out/f"{c['Name']}-{backend}-normalized.png")
            b = actual
            bad = accuracy_mask(a,b,coverage)
            count = bad.histogram()[255]
            diff = Image.new("RGB",a.size);diff.paste((255,60,60),(0,0),bad)
            diff.save(out/f"{c['Name']}-{backend}-diff.png")
            results.append(dict(**c,Backend=backend,Unmatched=count,ClassicColorsNotResampled=unmatched(a,b).histogram()[255],Passed=count==0))
    failed = sum(not r['Passed'] for r in results)
    report = dict(scope='Static weapon artwork/lighting; 4:3 and widescreen; one logical pixel position tolerance; Modern colors must occur in that classic neighborhood and classic silhouette must remain. Reverse color loss from normalization is diagnostic. Raw palette, no gamma/CRT.',reference=metadata,results=results)
    (out/'results.json').write_text(json.dumps(report, indent=2))
    results.sort(key=lambda r:r['Unmatched'],reverse=True)
    (out/'index.html').write_text('''<!doctype html><meta charset="utf-8"><title>Modern presentation accuracy</title><style>body{font:16px system-ui;background:#171b21;color:#eee;margin:24px}.row{display:flex;flex-wrap:wrap;gap:16px}img{width:480px;image-rendering:pixelated}select{padding:8px}pre{white-space:pre-wrap}</style><h1>Modern presentation accuracy</h1><p>Aspect and fullscreen placement normalized. Modern colors and the classic silhouette may move by one logical pixel. Red marks violations; classic texels lost when downsampling Modern are reported separately.</p><select id="sel"></select><pre id="stats"></pre><div class="row"><div>Classic<img id="classic"></div><div>Modern, normalized<img id="modern"></div><div>Outside tolerance<img id="diff"></div></div><script>const data='''+json.dumps(results)+''';sel.innerHTML=data.map((r,i)=>`<option value="${i}">${r.Patch} ${r.Width}×${r.Height} ${r.Backend} ${r.Lighting?'lit':'fullbright'}: ${r.Unmatched}</option>`).join('');function update(){const r=data[+sel.value];stats.textContent=JSON.stringify(r,null,2);classic.src=r.Name+'-classic.png';modern.src=r.Name+'-'+r.Backend+'-normalized.png';diff.src=r.Name+'-'+r.Backend+'-diff.png'}sel.onchange=update;update()</script>''')
    print(f'{len(results)} Modern comparisons, {failed} outside tolerance. Report: {out/"index.html"}')
    if failed:
        raise SystemExit(1)


if __name__=='__main__':
    main()
