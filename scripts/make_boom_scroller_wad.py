#!/usr/bin/env python3
"""Original Boom scroller fixtures with an autonomous timed-door controller."""
from pathlib import Path
import json
import math
import struct
from make_boom_smoke_wad import name
from make_boom_transfer_wad import map_lumps

SPECIALS=[245,246,247,248,249,214,215,216,217,218,254,255]


def patch():
    width=height=128
    columns=[bytes([0,height,0])+bytes([128+32*((x//64)+2*(y//64)) for y in range(height)])+bytes([0,255]) for x in range(width)]
    return struct.pack('<4h',width,height,0,0)+struct.pack('<128I',*(8+4*width+x*len(columns[0]) for x in range(width)))+b''.join(columns)


def texture(label,width,height,patch_index):
    return struct.pack('<8sIhhIh',name(label),0,width,height,0,1)+struct.pack('<hhhhh',0,0,patch_index,0,0)


def build():
    lumps=[]
    for number,special in enumerate(SPECIALS,1):
        room=map_lumps(number)
        data=dict(room)
        sides=[list(v) for v in struct.iter_unpack('<hh8s8s8sH',data['SIDEDEFS'])]
        for s in sides:s[4]=name('BSCROLL')
        lines=[list(v) for v in struct.iter_unpack('<7H',data['LINEDEFS'])]
        for l in lines:l[4]=17
        vertices=list(struct.iter_unpack('<hh',data['VERTEXES']))
        if special==254:
            vertices[3]=(192,-128) # Slanted target wall exercises vector rotation.
            segs=[list(v) for v in struct.iter_unpack('<6H',data['SEGS'])]
            for seg in segs:
                a,b=vertices[seg[0]],vertices[seg[1]]
                seg[2]=int(math.atan2(b[1]-a[1],b[0]-a[0])*65536/(2*math.pi))&65535
            data['SEGS']=b''.join(struct.pack('<6H',*v) for v in segs)
        first=len(vertices);vertices.extend([(-64,0),(0,32)])
        side=len(sides);sides.append([0,0,name('-'),name('-'),name('-'),1])
        if special==255:
            lines[0][3]=255;sides[0][0],sides[0][1]=3,-2
        else:
            lines.append([first,first+1,0,special,17 if special in (249,218,254) else 7,side,65535])
        sectors=[(0,128,'BSCFLR','BSCCEI',160,0,7),(0,32,'BSCFLR','BSCCEI',160,10,0)]
        data['VERTEXES']=b''.join(struct.pack('<hh',*v) for v in vertices)
        data['LINEDEFS']=b''.join(struct.pack('<7H',*v) for v in lines)
        data['SIDEDEFS']=b''.join(struct.pack('<hh8s8s8sH',*v) for v in sides)
        data['SECTORS']=b''.join(struct.pack('<hh8s8shhh',f,c,name(fp),name(cp),light,sp,tag) for f,c,fp,cp,light,sp,tag in sectors)
        data['REJECT']=bytes(1)
        words=[-256,-128,5,3]+[19]*15+[0]+list(range(len(lines)))+[-1]
        data['BLOCKMAP']=struct.pack('<'+'h'*len(words),*words)
        lumps.extend((label,data[label]) for label,_ in room)
    lumps.extend([('P_START',b''),('BSCROLL',patch()),('P_END',b'')])
    labels=['BSCROLL','RSKY1','RSKY2','RSKY3']
    lumps.append(('PNAMES',struct.pack('<I',len(labels))+b''.join(name(x) for x in labels)))
    # Doom reserves wall texture index zero as the missing-texture sentinel.
    textures=[texture('BSNULL',128,128,0),texture('BSCROLL',128,128,0)]+[texture(f'SKY{i}',256,128,i) for i in range(1,4)]
    offsets=[];offset=4+4*len(textures)
    for t in textures:offsets.append(offset);offset+=len(t)
    lumps.append(('TEXTURE1',struct.pack('<I',len(textures))+struct.pack('<'+'I'*len(offsets),*offsets)+b''.join(textures)))
    lumps.append(('F_START',b''))
    for label,shift in [('BSCFLR',0),('BSCCEI',1)]:
        lumps.append((label,bytes(128+32*((x//32+shift)%2+2*(y//32)) for y in range(64) for x in range(64))))
    lumps.append(('F_END',b''))
    out,directory=bytearray(bytes(12)),bytearray()
    for label,data in lumps:
        directory.extend(struct.pack('<ii8s',len(out),len(data),name(label)));out.extend(data)
    struct.pack_into('<4sii',out,0,b'PWAD',len(lumps),len(out))
    return out+directory


def scenes():
    views=[]
    for number,special in enumerate(SPECIALS,1):
        for tic in ([0,7,17] if special>=254 else [0,1050,1055,1067,1075]):
            # Checker flat boundaries allow 2% raster differences; offsets are
            # independently compared bit-for-bit before any image checks.
            # Camera is independent of the simulated player's conveyor travel.
            views.append(dict(name=f'scroll-{special}-{tic}',map=f'MAP{number:02}',X=128 if special==255 else -128,Y=0,Z=41,angle=2147483648 if special==255 else 0,tic=tic,category='boom-scroller',checks=[dict(name='floor-phase',box=[144,180,176,196],max_different_percent=2),dict(name='ceiling-phase',box=[144,4,176,20],max_different_percent=2),dict(name='wall-phase',box=[150,80,170,96])]))
    return views


if __name__=='__main__':
    root=Path(__file__).resolve().parents[1]/'testdata/boom'
    (root/'scrollers.wad').write_bytes(build())
    (root/'scroller-scenes.json').write_text(json.dumps(scenes(),indent=2)+'\n')
    print(root/'scrollers.wad')
