#!/usr/bin/env python3
"""Original sprites/colormaps for height-sector render comparisons, no IWAD assets."""
from pathlib import Path
import struct
from make_boom_smoke_wad import name
from make_boom_water_wad import build as water


def patch(width, height, index):
    # One uniform opaque post per column isolates clipping and light selection.
    columns = [bytes([0,height,0])+bytes([index for y in range(height)])+bytes([0,255]) for _ in range(width)]
    offsets=[]
    start=8+width*4
    for c in columns:
        offsets.append(start)
        start+=len(c)
    return struct.pack('<4h',width,height,width//2,height)+struct.pack('<'+'I'*width,*offsets)+b''.join(columns)


def build():
    raw=water()
    n,off=struct.unpack_from('<ii',raw,4)
    lumps=[]
    mapno=0
    for i in range(n):
        start,size,label=struct.unpack_from('<ii8s',raw,off+i*16)
        label=label.rstrip(b'\0').decode()
        data=raw[start:start+size]
        if label.startswith('MAP'): mapno=int(label[3:])
        if label=='THINGS':
            data+=struct.pack('<5h',128,0,0,48,7) # Tall, crosses both model planes.
            data+=struct.pack('<5h',128,64,0,2035,7) # Wholly submerged barrel.
        if label=='SECTORS':
            sectors=[list(s) for s in struct.iter_unpack('<hh8s8shhh',data)]
            sectors[1][0]=-32
            sectors[2][2],sectors[2][3]=name('BWDEEP'),name('F_SKY1' if mapno==3 else 'BWCEIL')
            if mapno==3: sectors[1][3]=name('F_SKY1')
            data=b''.join(struct.pack('<hh8s8shhh',*s) for s in sectors)
        if label=='SIDEDEFS' and mapno in (2,3):
            sides=[list(s) for s in struct.iter_unpack('<hh8s8s8sH',data)]
            sides[-1][2:5]=[name('BWHIGH'),name('BWLOW'),name('BWMID')]
            data=b''.join(struct.pack('<hh8s8s8sH',*s) for s in sides)
        lumps.append((label,data))
    lumps += [('S_START',b''),('ELECA0',patch(32,224,160)),('BAR1A0',patch(24,24,112)),('S_END',b'')]
    lumps.append(('C_START',b''))
    for label,base in [('BWLOW',192),('BWMID',160),('BWHIGH',112)]:
        lumps.append((label,bytes(base+max(0,15-(row//2))+((i%4)//2) if row<32 else i for row in range(34) for i in range(256))))
    lumps.append(('C_END',b''))
    out,directory=bytearray(bytes(12)),bytearray()
    for label,data in lumps:
        directory.extend(struct.pack('<ii8s',len(out),len(data),name(label)))
        out.extend(data)
    struct.pack_into('<4sii',out,0,b'PWAD',len(lumps),len(out))
    return out+directory


if __name__=='__main__':
    output=Path(__file__).resolve().parents[1]/'testdata/boom/water-actors.wad'
    output.write_bytes(build())
    print(output)
