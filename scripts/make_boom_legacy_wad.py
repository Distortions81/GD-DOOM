#!/usr/bin/env python3
"""Original three-strip mover fixture and deterministic Boom action cases."""
import json
import math
from pathlib import Path
import struct
from make_boom_smoke_wad import name

SPECIALS=[78,146,150,151,153,154,155,165,166,185,186,189,190,191,*range(199,207),211,212,*range(219,223),*range(227,242),256,257,258,259]
USE={78,165,166,185,186,189,190,191,203,204,205,206,211,221,222,229,230,233,234,237,238,241,258,259}

def build(path):
    vertices=[];lines=[];sides=[];segs=[];subsectors=[];edges={}
    for sector,(left,right) in enumerate([(-384,-128),(-128,128),(128,384)]):
        poly=[(left,-128),(left,128),(right,128),(right,-128)];first=len(segs)
        for a,b in zip(poly,poly[1:]+poly[:1]):
            for v in [a,b]:
                if v not in vertices:vertices.append(v)
            va,vb=vertices.index(a),vertices.index(b);key=tuple(sorted((va,vb)))
            side=len(sides);sides.append([0,0,name('-'),name('-'),name('STARTAN3'),sector])
            if key in edges:
                li=edges[key];lines[li][2]=4;lines[li][6]=side;sides[lines[li][5]][4]=name('-');sides[side][4]=name('-');direction=1
            else:
                li=len(lines);edges[key]=li;lines.append([va,vb,1,0,0,side,65535]);direction=0
            angle=int(math.atan2(b[1]-a[1],b[0]-a[0])*65536/(2*math.pi))&65535
            segs.append([va,vb,angle,li,direction,0])
        subsectors.append([4,first])
    assert lines[2][:2]==[2,3]
    lines[2][3:5]=[227,7];sides[lines[2][5]][2]=name('SW1BRCOM')
    def box(left,right):return [128,-128,left,right]
    nodes=struct.pack('<12h2H',-128,0,0,1,*box(-128,128),*box(-384,-128),0x8001,0x8000)
    nodes+=struct.pack('<12h2H',128,0,0,1,*box(128,384),*box(-384,128),0x8002,0)
    sectors=[(0,256,'FLOOR0_1',0,0),(64,192,'FLOOR0_3',0,7),(96,128,'FLOOR0_6',0,0)]
    words=[-384,-128,7,3]+[25]*21+[0]+list(range(len(lines)))+[-1]
    lumps=[('MAP01',b''),('THINGS',struct.pack('<10h',-320,-64,0,1,7,-320,64,0,3001,7)),('LINEDEFS',b''.join(struct.pack('<7H',*v) for v in lines)),('SIDEDEFS',b''.join(struct.pack('<hh8s8s8sH',*v) for v in sides)),('VERTEXES',b''.join(struct.pack('<hh',*v) for v in vertices)),('SEGS',b''.join(struct.pack('<6H',*v) for v in segs)),('SSECTORS',b''.join(struct.pack('<2H',*v) for v in subsectors)),('NODES',nodes),('SECTORS',b''.join(struct.pack('<hh8s8shhh',f,c,name(flat),name('CEIL1_1'),160,special,tag) for f,c,flat,special,tag in sectors)),('REJECT',bytes(2)),('BLOCKMAP',struct.pack('<'+'h'*len(words),*words))]
    # MAP02 has a correctly oriented, two-sided first pillar line for donuts.
    donut=[]
    for label,data in lumps:
        if label=='MAP01':label='MAP02'
        if label=='LINEDEFS':
            rows=[list(v) for v in struct.iter_unpack('<7H',data)]
            rows[0],rows[2]=rows[2],rows[0]
            rows[0][0],rows[0][1]=rows[0][1],rows[0][0]
            rows[0][5],rows[0][6]=rows[0][6],rows[0][5]
            rows[0][3:5]=[0,0];rows[2][3:5]=[155,7]
            data=b''.join(struct.pack('<7H',*v) for v in rows)
        if label=='SEGS':
            rows=[list(v) for v in struct.iter_unpack('<6H',data)]
            for row in rows:
                if row[3]==2:row[3]=0;row[4]^=1
                elif row[3]==0:row[3]=2
            data=b''.join(struct.pack('<6H',*v) for v in rows)
        if label=='SIDEDEFS':
            data=bytearray(data);data[4:12]=name('SW1BRCOM');data=bytes(data)
        donut.append((label,data))
    lumps+=donut
    payload=bytearray(bytes(12));directory=bytearray()
    for label,data in lumps:
        directory.extend(struct.pack('<ii8s',len(payload),len(data),name(label)));payload.extend(data)
    struct.pack_into('<4sii',payload,0,b'PWAD',len(lumps),len(payload));path.write_bytes(payload+directory)


def cases():
    configurations=[('steps',[0,64,96],[256,192,128]),('tight_up',[96,64,128],[256,120,256]),('tight_down',[0,64,32],[256,120,256]),('numeric',[64,64,64],[256,192,256]),('negative',[-96,-32,0],[128,96,128]),('fractional',[0,64.5,96.25],[256,192.5,128])]
    result=[]
    for special in SPECIALS:
        for label,floors,ceilings in configurations:
            for actor in range(3):
                target=0 if special in [146,155,191] else 1
                if target==0:floors,ceilings=[64,0,32],[192,128,128]
                result.append(dict(name=f's{special}_{label}_a{actor}',map=2 if target==0 else 1,special=special,use=int(special in USE),actor=actor,activator=0,side=0,position=0,tag=7,busy=0,target=target,tics=192,retry=96,floors=[int(v*65536) for v in floors],ceilings=[int(v*65536) for v in ceilings],types=[928,0,32]))
        base=result[-3].copy()
        if special not in [146,155,191]:
            for position in [1,2]:
                for actor in [1,2]:
                    result.append(dict(base,name=f's{special}_edge{position}_a{actor}',position=position,actor=actor,floors=[0,64*65536,96*65536],ceilings=[256*65536,192*65536,224*65536]))
        if special in [146,155,191]:result.append(dict(base,name=f's{special}_malformed',map=1,tics=32,retry=-1))
        for label,updates in [('monster',dict(activator=1)),('back',dict(side=1)),('untagged',dict(tag=0)),('missing',dict(tag=99)),('floorbusy',dict(busy=1)),('ceilingbusy',dict(busy=2)),('elevatorbusy',dict(busy=3))]:
            result.append(dict(base,**dict(updates,name=f's{special}_{label}',tics=32,retry=-1)))
    return result

if __name__=='__main__':
    root=Path(__file__).resolve().parents[1]/'testdata/boom'
    build(root/'legacy-movers.wad');(root/'legacy-mover-cases.json').write_text(json.dumps(cases(),indent=2)+'\n')
