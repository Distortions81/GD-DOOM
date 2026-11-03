#!/usr/bin/env python3
"""Compare every Boom lock special, six-key mask and trigger outcome with DSDA."""
import argparse
import csv
import hashlib
import json
import os
from pathlib import Path
from verify_boom_transfers import ROOT, build_reference, digest, merge_wads, run
from make_boom_locks_wad import build


def main():
    parser=argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--iwad',required=True,type=Path)
    ref=parser.add_mutually_exclusive_group(required=True)
    ref.add_argument('--reference',type=Path)
    ref.add_argument('--reference-source',type=Path)
    parser.add_argument('--cmake-arg',action='append',default=[])
    parser.add_argument('--out',default=ROOT/'tmp/boom-locks',type=Path)
    args=parser.parse_args();out=args.out.resolve();out.mkdir(parents=True,exist_ok=True)
    pwad=out/'locks.wad';build(pwad)
    executable=args.reference.resolve() if args.reference else build_reference(args.reference_source,out,args.cmake_arg)
    merged=out/'merged.wad';merge_wads([args.iwad.resolve(),pwad],merged)
    env=dict(os.environ,SDL_AUDIODRIVER='dummy',GD_LOCK_WAD=str(merged),GD_LOCK_OUT=str(out/'gd.csv'))
    run(['xvfb-run','-a','go','test','-tags','integration','./internal/doomruntime','-run','^TestBoomLocksCapture$','-count=1'],out/'gd.log',env)
    for key in ['GD_DSDA_ENV_CASES','GD_DSDA_TELEPORT_CASES','GD_DSDA_VISUAL_SCENES']:env.pop(key,None)
    env['GD_DSDA_LOCK_OUT']=str(out/'dsda.csv')
    run(['xvfb-run','-a',executable,'-iwad',args.iwad.resolve(),'-file',pwad,'-complevel','9','-nosound','-width','320','-height','200','-vidmode','8bit','-config',out/'dsda.cfg'],out/'dsda.log',env)
    failures=[];count=0;matched=0
    with (out/'gd.csv').open() as a,(out/'dsda.csv').open() as b:
        gd=csv.DictReader(a);dsda=csv.DictReader(b)
        for special in range(0x3800,0x3c00):
            for mask in range(64):
                for monster in range(2):
                    for phase in range(2):
                        actual=next(gd);expected=next(dsda)
                        identity=dict(special=str(special),mask=str(mask),monster=str(monster),phase=str(phase))
                        if any(actual[k]!=v or expected[k]!=v for k,v in identity.items()):raise ValueError('missing or reordered cases')
                        diff={k:dict(gd=actual[k],dsda=expected[k]) for k in expected if actual[k]!=expected[k]}
                        count+=1;matched+=not diff
                        if diff and len(failures)<30:failures.append(dict(**identity,differences=diff))
        if next(gd,None) is not None or next(dsda,None) is not None:raise ValueError('extra capture rows')
    metadata={name:digest(path) for name,path in dict(iwad=args.iwad,pwad=pwad,reference=executable,probe=ROOT/'scripts/visual_compare/dsda_lock_probe.inc',verifier=Path(__file__)).items()}
    source_hash=hashlib.sha256()
    for path in sorted((ROOT/'internal').rglob('*.go')):
        source_hash.update(str(path.relative_to(ROOT)).encode()+b'\0'+path.read_bytes())
    metadata['gd_source']=source_hash.hexdigest()
    metadata['message_probe']=digest(ROOT/'scripts/visual_compare/dsda_lock_messages.inc')
    metadata['fixture_generator']=digest(ROOT/'scripts/make_boom_locks_wad.py')
    result=dict(passed=matched==count,specials=1024,key_combinations=64,actors=['player','imp'],attempts=2,rows=count,matched=matched,failures=failures,metadata=metadata,scope='Real use, walk-crossing and gun dispatchers; no movement or combat simulation in this matrix. Second attempt checks busy and consumed triggers.')
    (out/'results.json').write_text(json.dumps(result,indent=2)+'\n')
    print(f'{matched}/{count} DSDA lock rows match')
    if failures:print(json.dumps(failures[:8],indent=2));raise SystemExit(1)

if __name__=='__main__':main()
