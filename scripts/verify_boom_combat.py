#!/usr/bin/env python3
"""Reference comparisons for Boom revenants, arch-viles and Ancient Aliens combat."""
import argparse,hashlib,json,os,struct
from pathlib import Path
from verify_boom_rng import fixture,demo,compare
from verify_boom_transfers import ROOT,run,digest,merge_wads,build_reference


def arrange(path, things, blocked=False, raised=False):
    raw=path.read_bytes(); n,offset=struct.unpack_from('<ii',raw,4); lumps=[]
    for i in range(n):
        start,size,label=struct.unpack_from('<ii8s',raw,offset+i*16); data=raw[start:start+size]
        if label.rstrip(b'\0')==b'THINGS':data=b''.join(struct.pack('<5h',*thing) for thing in things)
        if label.rstrip(b'\0')==b'SECTORS':
            data=bytearray(data)
            if blocked:struct.pack_into('<h',data,26+2,0)
            if raised:struct.pack_into('<h',data,52,32)
        lumps.append((label,data))
    payload=bytearray(12);directory=bytearray()
    for label,data in lumps:directory+=struct.pack('<ii8s',len(payload),len(data),label);payload+=data
    struct.pack_into('<4sii',payload,0,b'PWAD',len(lumps),len(payload));path.write_bytes(payload+directory)


def coverage(path, name):
    """Reject comparisons that never reach the behavior named by the fixture."""
    import csv
    rows=list(csv.DictReader(path.open()))
    actors=[[list(map(int,m.split(':'))) for m in row['mobjs'].strip(';').split(';') if m] for row in rows]
    flat=[m for frame in actors for m in frame]
    has_state=lambda typ,lo,hi:any(m[0]==typ and lo<=m[10]<=hi for m in flat)
    targets={(m[0],m[12]) for m in flat if m[11]}
    health=[int(row['health']) for row in rows]
    checks={}
    if name=='revenant_melee':
        checks.update(melee=has_state(5,336,338),player_damaged=min(health)<100)
    if name in ['revenant_homing','revenant_raised','revenant_dodge']:
        missiles=[m for m in flat if m[0]==6 and m[10] in [316,317]]
        checks.update(missile=bool(missiles),homing=len({m[4] for m in missiles})>1,
                      vertical_tracking=len({m[7] for m in missiles})>1,
                      smoke=any(m[0]==7 for m in flat))
    if name=='vile_resurrection':
        checks.update(heal=has_state(3,266,268),raise_zombie=has_state(1,203,206),
                      corpse=any(m[0]==1 and m[8]<=0 for m in flat))
    if name=='revenant_infighting':
        checks.update(revenant_retaliates=(5,15) in targets,baron_retaliates=(15,5) in targets,
                      revenant_dies=any(m[0]==5 and m[8]<=0 for m in flat))
    if name=='vile_infighting':
        checks.update(vile_retaliates=(3,5) in targets,missile_hits_vile=any(m[0]==3 and m[8]<700 for m in flat))
    if name=='vile_no_retaliation':
        checks.update(blast=has_state(3,264,264),revenant_hurt=any(m[0]==5 and m[8]<300 for m in flat),
                      no_retaliation=(5,3) not in targets)
    if name.startswith('vile_crushed'):
        checks['crushed']=any(m[0]==2 and m[9]==-1 and m[20]==0 and m[21]==0 for m in flat)
        raised=has_state(2,236,240)
        if name=='vile_crushed':
            checks.update(raised=raised,restored_bounds=any(m[0]==2 and m[8]>0 and m[20]==20*65536 and m[21]==56*65536 for m in flat))
        else:
            checks.update(rejected=not raised,stays_dead=all(m[8]<=0 for m in flat if m[0]==2))
    if name=='vile_occluded':
        checks.update(no_target=(3,0) not in targets,no_fire=not any(m[0]==4 for m in flat))
    if name=='aaliens_map01':
        checks.update(monsters_awake=any(m[0] in [1,2,5,11,12,21] and m[11] for m in flat),
                      monster_damaged=any(m[0] in [1,2,5,11,12,21] and m[8]<=0 for m in flat),
                      player_dies=min(health)==0,cyber_rockets=any(m[0]==33 for m in flat),
                      gameplay_rng_advanced=rows[0]['seeds']!=rows[-1]['seeds'])
    return dict(passed=all(checks.values()),checks=checks)


def main():
    p=argparse.ArgumentParser(description=__doc__);p.add_argument('--iwad',required=True,type=Path)
    ref=p.add_mutually_exclusive_group(required=True);ref.add_argument('--reference',type=Path);ref.add_argument('--reference-source',type=Path)
    p.add_argument('--cmake-arg',action='append',default=[]);p.add_argument('--out',type=Path,default=ROOT/'tmp/boom-combat');p.add_argument('--case',action='append');p.add_argument('--aaliens',type=Path)
    args=p.parse_args();out=args.out.resolve();out.mkdir(parents=True,exist_ok=True)
    exe=args.reference.resolve() if args.reference else build_reference(args.reference_source,out,args.cmake_arg)
    env=dict(os.environ,SDL_AUDIODRIVER='dummy');results={}
    scenes=[(f'm{typ}_i{insurance}_s{seed}_attack{attack}',typ,seed,insurance,bool(attack)) for typ in [66,64] for insurance in [0,1] for seed in [0,1993] for attack in [0,1]]
    extra={
        'revenant_melee':(66,[(-256,-48,0,1,7),(-208,-48,180,66,7)],dict(attack=False)),
        'revenant_homing':(66,[(-320,-48,0,1,7),(256,-48,180,66,7)],dict(attack=False)),
        'revenant_dodge':(66,[(-320,-48,0,1,7),(256,-48,180,66,7)],dict(attack=False)),
        'revenant_raised':(66,[(-320,-48,0,1,7),(256,-48,180,66,7)],dict(attack=False)),
        'vile_resurrection':(64,[(-320,-48,0,1,7),(-64,-48,180,3004,7),(256,-48,180,64,7)],dict(attack_tics=100)),
        'revenant_infighting':(66,[(-320,-48,0,1,7),(-64,-48,180,3003,7),(256,-48,180,66,7)],dict(attack=False)),
        'vile_infighting':(64,[(-320,-48,0,1,7),(-64,-48,0,64,15),(256,-48,180,66,7)],dict(attack=False)),
        'vile_crushed':(64,[(-320,-48,0,1,7),(-64,-48,180,64,7),(16,-48,0,9,7),(-320,-48,0,2022,7)],dict(attack=False,forward=1)),
        'vile_crushed_blocked':(64,[(-320,-48,0,1,7),(-64,-48,180,64,7),(16,-48,0,9,7),(48,-48,0,30,7),(-320,-48,0,2022,7)],dict(attack=False,forward=1)),
        'vile_no_retaliation':(64,[(-320,-48,0,1,7),(-272,-48,0,66,15),(0,-48,180,64,7),(-320,-48,0,2022,7)],dict(attack=False,forward=1)),
        'vile_occluded':(64,[(-320,-48,0,1,7),(256,-48,180,64,7)],dict(attack=False)),
    }
    scenes += [(name,typ,1993,1,settings.get('attack',True)) for name,(typ,things,settings) in extra.items()]
    known={c[0] for c in scenes}|{'aaliens_map01'}
    scenes.append(('aaliens_map01',0,1993,1,True))
    if args.case and set(args.case)-known:p.error('unknown case')
    for name,typ,seed,insurance,attack in scenes:
        if args.case and name not in args.case:continue
        folder=out/name;folder.mkdir(exist_ok=True);pwad=folder/'combat.wad'
        if name=='aaliens_map01':
            if args.aaliens:pwad=args.aaliens.resolve()
            else:
                from verify_boom_demos import fetch,WADS
                path,sha,entry=WADS['aaliens'];pwad=fetch(out,'aaliens.zip',path,sha,entry)
        else:
            fixture(pwad,typ)
            if name in extra:arrange(pwad,extra[name][1],blocked=name=='vile_occluded',raised=name=='revenant_raised')
        merged=folder/'merged.wad';merge_wads([args.iwad.resolve(),pwad],merged)
        env.pop('GD_RNG_CRUSHED_CORPSE',None)
        if name.startswith('vile_crushed'):env['GD_RNG_CRUSHED_CORPSE']='1'
        tics=1000 if name in extra else 350
        settings=dict(extra[name][2]) if name in extra else dict(attack=attack)
        lmp=folder/'combat.lmp';demo(lmp,seed,insurance,tics=tics,**settings)
        if name=='revenant_dodge':
            data=bytearray(lmp.read_bytes())
            for tic in range(tics):
                data[109+tic*4+1]=(25 if 50<=tic<70 else -25 if 120<=tic<140 else 0)&255
            lmp.write_bytes(data)
        if name=='aaliens_map01':
            from verify_boom_demos import fetch,CASES
            item=next(c for c in CASES if c[0]=='aa01o023')
            route=fetch(out,'aa01o023.zip',item[3],item[4],item[5]).read_bytes()
            commands=bytearray()
            for offset in range(109,len(route),4):
                if route[offset]==128:break
                cmd=bytearray(route[offset:offset+4]);cmd[3]|=1;commands+=cmd
            # Bound this opening combat encounter before the route's death/restart use.
            commands=commands[:500*4]
            tics=len(commands)//4
            lmp.write_bytes(lmp.read_bytes()[:109]+commands+b'\x80')
        run(['xvfb-run','-a','go','test','-tags','integration','./internal/doomruntime','-run','^TestBoomRNGDemoCapture$','-count=1'],folder/'gd.log',dict(env,GD_RNG_COMBAT_ONLY="1",GD_RNG_DEMO=str(lmp),GD_RNG_WAD=str(merged),GD_RNG_OUT=str(folder/'gd.csv')))
        run(['xvfb-run','-a',exe,'-iwad',args.iwad.resolve(),'-file',pwad,'-timedemo',lmp,'-nosound','-config',out/'dsda.cfg'],folder/'dsda.log',dict(env,GD_RNG_COMBAT_ONLY="1",GD_DSDA_RNG_DEMO_OUT=str(folder/'dsda.csv')))
        results[name]=compare(folder/'gd.csv',folder/'dsda.csv',tics)
        results[name]['coverage']=coverage(folder/'dsda.csv',name)
        results[name]['passed'] &= results[name]['coverage']['passed']
        results[name]['inputs']={str(path):digest(path) for path in [pwad,lmp]}
        print(name,results[name]['matched'],results[name]['rows'],'coverage',results[name]['coverage']['passed'],flush=True)
        if results[name]['failures']:print('first mismatch:',results[name]['failures'][0]['row'], list(results[name]['failures'][0]['diff']),flush=True)
        if not results[name]['coverage']['passed']:print('missing coverage:',results[name]['coverage'],flush=True)
    result=dict(passed=all(c['passed'] for c in results.values()),cases=results,metadata=dict(reference=digest(exe),verifier=digest(Path(__file__))))
    source=hashlib.sha256()
    for path in sorted((ROOT/'internal').rglob('*.go')):source.update(str(path.relative_to(ROOT)).encode()+b'\0'+path.read_bytes())
    result['metadata']['gd_source']=source.hexdigest();result['metadata']['iwad']=digest(args.iwad)
    result['metadata']['observer']=digest(ROOT/'scripts/visual_compare/dsda_rng_demo_probe.inc')
    core=exe.parent.parent/'reference-source/src'
    for name in ['p_enemy.c','p_mobj.c','p_inter.c','p_map.c','p_user.c','m_random.c']:
        if (core/name).exists():result['metadata']['reference_core/'+name]=digest(core/name)
    result['scope']='Real Boom demos with full monster thinkers. Every tic compares player physics, health, ammo, eye height, ordered monsters/missiles/effects, targeting presence/type, chase memory, body bounds/support and all gameplay RNG seeds. Static decoration, player/pickup animation, cosmetic misc RNG and target identity among same-type actors are excluded. Crushed-corpse cases use an explicitly prepared initial corpse and normal live resurrection. Ancient Aliens uses the first 500 tics of the pinned aa01o023 movement route with monsters enabled and attack held; it covers the opening encounter through player death, not map completion, rebirth or arbitrary combat demos.'
    (out/'results.json').write_text(json.dumps(result,indent=2)+'\n')
    if not result['passed']:raise SystemExit(1)
if __name__=='__main__':main()
