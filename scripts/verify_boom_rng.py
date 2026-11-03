#!/usr/bin/env python3
"""Compare seeded Boom RNG and actual monster demo playback against DSDA."""
import argparse,csv,hashlib,itertools,json,os,struct
from pathlib import Path
from verify_boom_transfers import ROOT,run,digest,merge_wads,build_reference
from make_boom_legacy_wad import build as build_rooms


def fixture(path,typ=3001):
    build_rooms(path);raw=path.read_bytes();n,offset=struct.unpack_from('<ii',raw,4);lumps=[]
    for i in range(n):
        start,size,label=struct.unpack_from('<ii8s',raw,offset+i*16);data=raw[start:start+size];name=label.rstrip(b'\0')
        if name==b'THINGS':data=struct.pack('<10h',-256,-48,0,1,7,0,-48,180,typ,7)
        if name==b'SECTORS':
            data=bytearray(data)
            for j in range(0,len(data),26):struct.pack_into('<hh',data,j,0,128)
        lumps.append((label,data))
    payload=bytearray(bytes(12));directory=bytearray()
    for label,data in lumps:directory+=struct.pack('<ii8s',len(payload),len(data),label);payload+=data
    struct.pack_into('<4sii',payload,0,b'PWAD',len(lumps),len(payload));path.write_bytes(payload+directory)


def demo(path,seed,insurance,attack=True,tics=350,recoil=False,bob=True,fast=False,respawn=False,nomonsters=False,remember=True,forward=0,attack_tics=None):
    options=bytearray(64);options[0:4]=bytes([int(remember),1,int(recoil),1]);options[5]=int(bob);options[6]=int(respawn);options[7]=int(fast);options[8]=int(nomonsters);options[9]=insurance;struct.pack_into('>I',options,10,seed)
    header=bytes([202,0x1d])+b'Boom'+bytes([0xe6,0,2,1,1,0,0])+options+bytes([1])+bytes(31)
    assert len(header)==109
    commands=b''.join(bytes([forward if i<35 else 0,0,0,1 if attack and (attack_tics is None or i<attack_tics) else 0]) for i in range(tics))
    path.write_bytes(header+commands+b'\x80');return options


def compare(a,b,expected_rows):
    rows=matched=0;failures=[]
    with a.open() as x,b.open() as y:
        for actual,expected in itertools.zip_longest(csv.DictReader(x),csv.DictReader(y)):
            if actual is None or expected is None:raise ValueError('missing reference rows')
            rows+=1;diff={k:dict(gd=actual[k],dsda=expected[k]) for k in expected if actual[k]!=expected[k]}
            matched+=not diff
            if diff and len(failures)<6:failures.append(dict(row=rows,diff=diff))
    if rows!=expected_rows:raise ValueError(f"expected {expected_rows} rows, got {rows}")
    return dict(passed=matched==rows,rows=rows,matched=matched,failures=failures,gd_hash=digest(a),dsda_hash=digest(b))


def main():
    p=argparse.ArgumentParser(description=__doc__);p.add_argument('--iwad',required=True,type=Path)
    group=p.add_mutually_exclusive_group(required=True);group.add_argument('--reference',type=Path);group.add_argument('--reference-source',type=Path)
    p.add_argument('--cmake-arg',action='append',default=[]);p.add_argument('--out',type=Path,default=ROOT/'tmp/boom-rng');p.add_argument('--case',action='append')
    args=p.parse_args();out=args.out.resolve();out.mkdir(parents=True,exist_ok=True)
    known={'vectors','lifts','diagnostic_ammo_exhaustion'}|{f'monster_{typ}_i{insurance}_s{seed}' for typ in [3001,3004,3002] for insurance in [0,1] for seed in [0,1993]}|{'option_'+name for name in ['recoil','nobob','fast','respawn','nomonsters','forget']}
    if args.case and set(args.case)-known:p.error('unknown case: '+','.join(sorted(set(args.case)-known)))
    exe=args.reference.resolve() if args.reference else build_reference(args.reference_source,out,args.cmake_arg)
    env=dict(os.environ,SDL_AUDIODRIVER='dummy');results={}
    vectors=[dict(Name=f's{seed}_i{insurance}_t{tic}_c{cls}',Seed=seed,Insurance=bool(insurance),Tic=tic,Class=cls,Count=512) for seed in [0,1,1993,0x80000000,0xffffffff] for insurance in [0,1] for tic in [0,1,255,65536] for cls in [-1,0,3,7,16,48,49,64]]
    (out/'vectors.json').write_text(json.dumps(vectors));(out/'vectors.tsv').write_text(''.join(f"{c['Name']} {c['Seed']} {int(c['Insurance'])} {c['Tic']} {c['Class']} {c['Count']}\n" for c in vectors))
    if not args.case or 'vectors' in args.case:
        run(['xvfb-run','-a','go','test','-tags','integration','./internal/doomruntime','-run','^TestBoomRNGVectorsCapture$','-count=1'],out/'gd-vectors.log',dict(env,GD_RNG_VECTORS=str(out/'vectors.json'),GD_RNG_OUT=str(out/'gd-vectors.csv')))
        run(['xvfb-run','-a',exe,'-iwad',args.iwad.resolve(),'-complevel','9','-nosound','-config',out/'dsda.cfg'],out/'dsda-vectors.log',dict(env,GD_DSDA_RNG_VECTORS=str(out/'vectors.tsv'),GD_DSDA_RNG_OUT=str(out/'dsda-vectors.csv')))
        results['vectors']=compare(out/'gd-vectors.csv',out/'dsda-vectors.csv',len(vectors)*512);print('vectors',results['vectors']['matched'],results['vectors']['rows'],flush=True)
    if not args.case or 'lifts' in args.case:
        folder=out/'lifts';folder.mkdir(exist_ok=True);pwad=folder/'lifts.wad';build_rooms(pwad);merged=folder/'merged.wad';merge_wads([args.iwad.resolve(),pwad],merged)
        cases=[dict(Name=f's{seed}_i{insurance}_sp{special}_d{draws}',Seed=seed,Insurance=bool(insurance),Special=special,Draws=draws,Tics=512,Stop=180,Resume=240) for seed in [0,1,1993,0xffffffff] for insurance in [0,1] for special in [0x3703,0x371b,87] for draws in [0,1,7]]
        (folder/'cases.json').write_text(json.dumps(cases));(folder/'cases.tsv').write_text(''.join(' '.join(map(str,[c[k] if k!='Insurance' else int(c[k]) for k in ['Name','Seed','Insurance','Special','Draws','Tics','Stop','Resume']]))+'\n' for c in cases))
        run(['xvfb-run','-a','go','test','-tags','integration','./internal/doomruntime','-run','^TestBoomRNGLiftsCapture$','-count=1'],folder/'gd.log',dict(env,GD_RNG_LIFTS=str(folder/'cases.json'),GD_RNG_WAD=str(merged),GD_RNG_OUT=str(folder/'gd.csv')))
        run(['xvfb-run','-a',exe,'-iwad',args.iwad.resolve(),'-file',pwad,'-complevel','9','-nosound','-config',out/'dsda.cfg'],folder/'dsda.log',dict(env,GD_DSDA_RNG_LIFTS=str(folder/'cases.tsv'),GD_DSDA_RNG_OUT=str(folder/'dsda.csv')))
        results['lifts']=compare(folder/'gd.csv',folder/'dsda.csv',len(cases)*513);print('lifts',results['lifts']['matched'],results['lifts']['rows'],flush=True)
        if not results['lifts']['passed']:print(json.dumps(results['lifts']['failures'][:1],indent=2),flush=True)
    scenes=[(f'monster_{typ}_i{insurance}_s{seed}',typ,seed,insurance,{}) for typ in [3001,3004,3002] for insurance in [0,1] for seed in [0,1993]]
    scenes += [('option_'+name,3001,1993,1,settings) for name,settings in [('recoil',dict(recoil=True)),('nobob',dict(bob=False,forward=25)),('fast',dict(fast=True)),('respawn',dict(respawn=True,tics=2200,attack_tics=200)),('nomonsters',dict(nomonsters=True)),('forget',dict(remember=False))]]
    if args.case and 'diagnostic_ammo_exhaustion' in args.case:scenes.append(('diagnostic_ammo_exhaustion',3001,1993,1,dict(respawn=True,tics=2200)))
    for name,typ,seed,insurance,settings in scenes:
        if args.case and name not in args.case:continue
        folder=out/name;folder.mkdir(exist_ok=True);pwad=folder/'combat.wad';fixture(pwad,typ);merged=folder/'merged.wad';merge_wads([args.iwad.resolve(),pwad],merged)
        lmp=folder/'combat.lmp';demo(lmp,seed,insurance,**settings)
        run(['xvfb-run','-a','go','test','-tags','integration','./internal/doomruntime','-run','^TestBoomRNGDemoCapture$','-count=1'],folder/'gd.log',dict(env,GD_RNG_DEMO=str(lmp),GD_RNG_WAD=str(merged),GD_RNG_OUT=str(folder/'gd.csv')))
        run(['xvfb-run','-a',exe,'-iwad',args.iwad.resolve(),'-file',pwad,'-timedemo',lmp,'-nosound','-config',out/'dsda.cfg'],folder/'dsda.log',dict(env,GD_DSDA_RNG_DEMO_OUT=str(folder/'dsda.csv')))
        results[name]=compare(folder/'gd.csv',folder/'dsda.csv',settings.get('tics',350));print(name,results[name]['matched'],results[name]['rows'],flush=True)
        if not results[name]['passed']:print(json.dumps(results[name]['failures'][:1],indent=2),flush=True)
    result=dict(passed=all(r['passed'] for r in results.values()),cases=results,metadata=dict(reference=digest(exe),verifier=digest(Path(__file__))))
    source=hashlib.sha256()
    for path in sorted((ROOT/'internal').rglob('*.go')):source.update(str(path.relative_to(ROOT)).encode()+b'\0'+path.read_bytes())
    result['metadata']['gd_source']=source.hexdigest();result['metadata']['iwad']=digest(args.iwad)
    for path in sorted((ROOT/'scripts/visual_compare').glob('dsda_rng*inc')):result['metadata'][str(path.relative_to(ROOT))]=digest(path)
    for path in [out/'vectors.json',out/'lifts/cases.json',*sorted(out.glob('*/combat.lmp')),*sorted(out.glob('*/combat.wad'))]:
        if path.exists():result['metadata'][str(path.relative_to(out))]=digest(path)
    core=exe.parent.parent/'reference-source/src'
    for name in ['m_random.c','p_enemy.c','p_mobj.c','p_inter.c','p_pspr.c','p_genlin.c','p_plats.c']:
        if (core/name).exists():result['metadata']['reference_core/'+name]=digest(core/name)
    result['scope']='Seeded Boom P_Random vectors and real dispatch/thinker lift trajectories without forced directions; generated Boom demos with full monster AI/combat. Player physics, health, ammo and view height, all non-player non-pickup mobjs and every gameplay RNG seed are compared each tic. Cosmetic misc RNG and player/pickup animation states are excluded. This does not establish arbitrary-map combat-demo parity.'
    (out/'results.json').write_text(json.dumps(result,indent=2)+'\n')
    if not result['passed']:raise SystemExit(1)
if __name__=='__main__':main()
