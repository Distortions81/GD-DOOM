#!/usr/bin/env python3
"""Manual doors sharing sectors with independent, overlapping Boom thinkers."""
import json
from pathlib import Path
import struct
from make_boom_legacy_wad import build as build_rooms

SPECIALS=[1,26,27,28,31,32,33,34,117,118,0x3c06,0x3c07,0x3c86,0x3c87,0x3c26,0x3c27]
# none, lift down/wait/up/stasis, elevator up/down, crusher fast/slow/stasis,
# stairs active/locked, floor, ceiling, door opening/closing/waiting/open, gen door.
KINDS=list(range(19))
AGES={2:20,3:128,11:20,16:40}

def build(path):
    build_rooms(path);raw=path.read_bytes();count,directory=struct.unpack_from('<ii',raw,4);lumps=[]
    for i in range(count):
        start,size,label=struct.unpack_from('<ii8s',raw,directory+i*16);data=raw[start:start+size];name=label.rstrip(b'\0').decode()
        if name=='LINEDEFS':
            assert len(data)//14==10
            line=bytearray(data[28:42]);struct.pack_into('<H',line,6,0);data+=line
        if name=='BLOCKMAP':
            assert struct.unpack_from('<h',data,len(data)-2)[0]==-1
            data=data[:-2]+struct.pack('<hh',10,-1)
        lumps.append((label,data))
    payload=bytearray(bytes(12));directory=bytearray()
    for label,data in lumps:directory.extend(struct.pack('<ii8s',len(payload),len(data),label));payload.extend(data)
    struct.pack_into('<4sii',payload,0,b'PWAD',len(lumps),len(payload));path.write_bytes(payload+directory)

def cases():
    result=[]
    def add(name,special,kind,**kw):
        c=dict(name=name,special=special,second=special,kind=kind,age=AGES.get(kind,0),actor=0,activator=0,side=0,secret=0,keys=1,tics=256,at2=20,at3=160,late=-1,floors=[0,64*65536,96*65536],ceilings=[256*65536,192*65536,256*65536])
        if kind in [10,11]:c['floors']=[64*65536]*3
        c.update(kw);result.append(c)
    for kind in KINDS:
        for special in SPECIALS:
            for activator in [0,1]:
                for actor in [0,2]:add(f'm{kind}_s{special}_u{activator}_a{actor}',special,kind,activator=activator,actor=actor)
    for kind in [1,5,6,7,10,14]:
        for special in [1,31,32,117,118,0x3c07,0x3c87,0x3c27]:
            for actor in [1,2]:
                for label,ceil in [('low',[256,192,128]),('tight',[256,120,128])]:
                    add(f'{label}_m{kind}_s{special}_a{actor}',special,kind,actor=actor,ceilings=[v*65536 for v in ceil])
    for kind in [1,5,7,10,15]:
        for special in SPECIALS:
            for label,kw in [('back',dict(side=1)),('secret_monster',dict(secret=1,activator=1)),('nokey',dict(keys=0))]:add(f'{label}_m{kind}_s{special}',special,kind,tics=48,**kw)
    # A one-shot door displaces a mover; a separate repeatable line then uses
    # the current slot, including an empty pointer with old thinkers still live.
    for kind in [1,5,6,7,8,9,10,13,14,15,16,17,18]:
        for first in [31,118]:
            for second in [1,117,31,118,0x3c07]:
                for at in [1,40,180]:
                    add(f'overlap_m{kind}_d{first}_s{second}_t{at}',first,kind,second=second,at2=at,at3=at+8,tics=320)
    for kind in [7,8,9,13]:
        add(f'replacement_ceiling_m{kind}',31,kind,second=1,at2=40,at3=200,late=64,tics=384)
    return result

if __name__=='__main__':
    root=Path(__file__).resolve().parents[1]/'testdata/boom';build(root/'manual-doors.wad')
    (root/'manual-door-cases.json').write_text(json.dumps(cases(),indent=2)+'\n')
