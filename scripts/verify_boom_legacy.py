#!/usr/bin/env python3
"""Compare Boom legacy movers and elevators with DSDA-Doom's real dispatchers."""
import argparse
from collections import Counter
import csv
import hashlib
import json
import os
from pathlib import Path
from make_boom_legacy_wad import build, cases, SPECIALS
from verify_boom_transfers import ROOT, build_reference, digest, merge_wads, run


def main():
    p=argparse.ArgumentParser(description=__doc__);p.add_argument('--iwad',required=True,type=Path)
    ref=p.add_mutually_exclusive_group(required=True);ref.add_argument('--reference',type=Path);ref.add_argument('--reference-source',type=Path)
    p.add_argument('--cmake-arg',action='append',default=[]);p.add_argument('--out',type=Path,default=ROOT/'tmp/boom-legacy')
    p.add_argument('--sector-sound-trace',action='store_true',help='write ordered per-frame sector sound requests')
    args=p.parse_args();out=args.out.resolve();out.mkdir(parents=True,exist_ok=True)
    pwad=out/'legacy.wad';build(pwad);manifest=out/'cases.json';inputs=cases();manifest.write_text(json.dumps(inputs,indent=2)+'\n')
    (out/'cases.tsv').write_text(''.join(' '.join(map(str,[c['name']]+[c[k] for k in ['map','special','use','actor','activator','side','position','tag','busy','target','tics','retry']]+c['floors']+c['ceilings']+c['types']))+'\n' for c in inputs))
    executable=args.reference.resolve() if args.reference else build_reference(args.reference_source,out,args.cmake_arg)
    merged=out/'merged.wad';merge_wads([args.iwad.resolve(),pwad],merged)
    env=dict(os.environ,SDL_AUDIODRIVER='dummy',GD_LEGACY_WAD=str(merged),GD_LEGACY_CASES=str(manifest),GD_LEGACY_OUT=str(out/'gd.csv'))
    if args.sector_sound_trace:env['GD_SECTOR_SOUND_OUT']=str(out/'gd-sounds.csv')
    run(['xvfb-run','-a','go','test','-tags','integration','./internal/doomruntime','-run','^TestBoomLegacyMoverCapture$','-count=1'],out/'gd.log',env)
    for key in ['GD_DSDA_ENV_CASES','GD_DSDA_TELEPORT_CASES','GD_DSDA_VISUAL_SCENES','GD_DSDA_LOCK_OUT']:env.pop(key,None)
    env.update(GD_DSDA_LEGACY_CASES=str(out/'cases.tsv'),GD_DSDA_LEGACY_OUT=str(out/'dsda.csv'))
    if args.sector_sound_trace:env['GD_SECTOR_SOUND_OUT']=str(out/'dsda-sounds.csv')
    run(['xvfb-run','-a',executable,'-iwad',args.iwad.resolve(),'-file',pwad,'-complevel','9','-nosound','-width','320','-height','200','-vidmode','8bit','-config',out/'dsda.cfg'],out/'dsda.log',env)
    failures=[];count=matched=0;fields=Counter()
    with (out/'gd.csv').open() as a,(out/'dsda.csv').open() as b:
        gd=csv.DictReader(a);dsda=csv.DictReader(b)
        for c in inputs:
            for tic in range(c['tics']+1):
                actual=next(gd);expected=next(dsda)
                if actual['name']!=c['name'] or expected['name']!=c['name'] or int(actual['tic'])!=tic or int(expected['tic'])!=tic:raise ValueError('missing or reordered rows')
                diff={k:dict(gd=actual[k],dsda=expected[k]) for k in expected if actual[k]!=expected[k]}
                count+=1;matched+=not diff;fields.update(diff.keys())
                if diff and len(failures)<50:failures.append(dict(name=c['name'],tic=tic,differences=diff))
        if next(gd,None) is not None or next(dsda,None) is not None:raise ValueError('extra rows')
    source_hash=hashlib.sha256()
    for path in sorted((ROOT/'internal').rglob('*.go')):source_hash.update(str(path.relative_to(ROOT)).encode()+b'\0'+path.read_bytes())
    metadata={label:digest(path) for label,path in dict(iwad=args.iwad,pwad=pwad,cases=manifest,reference=executable,probe=ROOT/'scripts/visual_compare/dsda_legacy_probe.inc',verifier=Path(__file__)).items()}
    metadata['gd_source']=source_hash.hexdigest()
    result=dict(passed=matched==count,specials=SPECIALS,cases=len(inputs),rows=count,matched=matched,differing_fields=dict(fields),failures=failures,metadata=metadata,scope='Real legacy dispatchers and ordered mover ticks, stationary players and actual imps; actor AI and horizontal movement are not stepped.')
    (out/'results.json').write_text(json.dumps(result,indent=2)+'\n');print(f'{matched}/{count} rows match across {len(inputs)} cases; differing fields {dict(fields)}')
    if failures:print(json.dumps(failures[:8],indent=2));raise SystemExit(1)

if __name__=='__main__':main()
