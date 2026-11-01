#!/usr/bin/env python3
"""Build original custom ANIMATED/SWITCHES differential fixtures."""
from pathlib import Path
import json
import struct
from make_boom_smoke_wad import name
from make_boom_transfer_wad import map_lumps
from make_boom_scroller_wad import texture

SPEEDS = [1, 2, 3, 4, 5, 6, 8]

def anim(kind, start, end, speed):
    return struct.pack('<B9s9si',kind,name(end),name(start),speed)

def build():
    lumps=[]
    for number in range(1, 13):
        room=map_lumps(number); data=dict(room)
        if number <= 7:
            wall, flat=f'BW{number}B',f'BF{number}B'
        elif number==8:
            wall,flat='BOV2','BF1B'
        else:
            wall,flat='BAOFF','BF1B'
        sides=[list(v) for v in struct.iter_unpack('<hh8s8s8sH',data['SIDEDEFS'])]
        for s in sides:s[4]=name(wall)
        lines=[list(v) for v in struct.iter_unpack('<7H',data['LINEDEFS'])]
        if number>=9:
            data['THINGS']=struct.pack('<5h',224,0,0,1,7)
            # 138 is SR light 255; 170 is S1 light 35.
            lines[2][3:5]=[170 if number==10 else 138,1]
            # Table-order beats slot priority. BAOFF is earlier than BBOFF.
            if number==11:sides[2][2],sides[2][4]=name('BBOFF'),name('BAOFF')
            if number==12:sides[2][4]=name('SW1BRCOM') # replaced defaults
        data['LINEDEFS']=b''.join(struct.pack('<7H',*v) for v in lines)
        data['SIDEDEFS']=b''.join(struct.pack('<hh8s8s8sH',*v) for v in sides)
        data['SECTORS']=struct.pack('<hh8s8shhh',0,128,name(flat),name(flat),160,0,1)
        lumps.extend((label,data[label]) for label,_ in room)
    wallnames=[f'BW{i}{c}' for i in range(1,8) for c in 'ABC']+[f'BOV{i}' for i in range(1,5)]+['BAOFF','BAON','BBOFF','BBON','BCON','SW1BRCOM','SW2BRCOM']
    # Every frame has a distinct uniform palette index. Interior image regions
    # compare phase without conflating the engines' wall/flat raster rounding.
    patchnames=['BPNUL']+wallnames+['RSKY1','RSKY2','RSKY3']
    lumps.append(('P_START',b''))
    for i,label in enumerate(patchnames[:-3]):
        col=bytes([0,128,0])+bytes([48+(i*7)%160])*128+bytes([0,255])
        lumps.append((label,struct.pack('<4h',128,128,0,0)+struct.pack('<128I',*(520+x*len(col) for x in range(128)))+col*128))
    lumps.append(('P_END',b''))
    lumps.append(('PNAMES',struct.pack('<I',len(patchnames))+b''.join(name(x) for x in patchnames)))
    textures=[texture('BNULL',128,128,0)]+[texture(n,128,128,i+1) for i,n in enumerate(wallnames)]+[texture(f'SKY{i}',256,128,len(patchnames)-4+i) for i in range(1,4)]
    offsets=[];offset=4+4*len(textures)
    for t in textures:offsets.append(offset);offset+=len(t)
    lumps.append(('TEXTURE1',struct.pack('<I',len(textures))+struct.pack('<'+'I'*len(offsets),*offsets)+b''.join(textures)))
    lumps.append(('F_START',b''))
    for i in range(1,8):
        for j,c in enumerate('ABC'):lumps.append((f'BF{i}{c}',bytes([64+i*12+j*4])*4096))
    lumps.append(('F_END',b''))
    animated=b''.join(anim(kind,f'B{prefix}{i}A',f'B{prefix}{i}C',speed) for i,speed in enumerate(SPEEDS,1) for kind,prefix in [(1,'W'),(0,'F')])
    animated+=anim(1,'BOV1','BOV3',2)+anim(1,'BOV2','BOV4',5)+anim(1,'ABSENT1','ABSENT3',3)+b'\xff'
    lumps.append(('ANIMATED',animated))
    switches=[('BAOFF','BAON',3),('BBOFF','BBON',2),('BAOFF','BCON',3),('MISSING','BAON',3),('SW1BRCOM','SW2BRCOM',4)]
    lumps.append(('SWITCHES',b''.join(struct.pack('<9s9sh',name(a),name(b),ep) for a,b,ep in switches)+bytes(20)))
    payload=bytearray(12);directory=bytearray()
    for label,data in lumps:
        directory.extend(struct.pack('<ii8s',len(payload),len(data),name(label)));payload.extend(data)
    struct.pack_into('<4sii',payload,0,b'PWAD',len(lumps),len(payload))
    return payload+directory

def scenes():
    probes=[f'{kind}:B{prefix}{i}{c}' for i in range(1,8) for c in 'ABC' for kind,prefix in [('W','W'),('F','F')]]+['W:BOV1','W:BOV2','W:BOV3','W:BOV4']
    views=[]
    for number in range(1,13):
        speed=SPEEDS[number-1] if number<=7 else 5
        tics=sorted(set([0,1,speed,speed+1,2*speed+1,3*speed+1])) if number<=8 else [0,1,2,11,34,35,36,37,38,42,46]
        for tic in tics:
            use=([1,37] if number==9 else [1,10,37]) if number>=9 else []
            views.append(dict(name=f'tables-{number:02}-{tic:02}',map=f'MAP{number:02}',X=-128,Y=0,Z=41,angle=0,tic=tic,use_tics=use,category='boom-tables',table_probes=probes,checks=[dict(name='wall-phase-or-button',box=[150,80,170,96]),dict(name='floor-phase',box=[144,180,176,196]),dict(name='ceiling-phase',box=[144,4,176,20])]))
    for view in views[1:]:view.pop("table_probes")
    return views

if __name__=='__main__':
    root=Path(__file__).resolve().parents[1]/'testdata/boom'
    (root/'tables.wad').write_bytes(build())
    (root/'tables-scenes.json').write_text(json.dumps(scenes(),indent=2)+'\n')
    print(root/'tables.wad')
