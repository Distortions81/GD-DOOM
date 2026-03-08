package main

import (
  "fmt"
  "gddoom/internal/mapdata"
  "gddoom/internal/wad"
)

func secForSide(m *mapdata.Map, side int16) int {
  if side < 0 || int(side) >= len(m.Sidedefs) { return -1 }
  return int(m.Sidedefs[int(side)].Sector)
}

func main(){
  wf,_:=wad.Open("/home/dist/github/GD-DOOM/doom.wad")
  m,_:=mapdata.LoadMap(wf,"E1M5")
  mx, my := int16(-93), int16(227)
  fmt.Printf("near move target approx=(%d,%d)\n", mx, my)
  for i, th := range m.Things {
    dx := int(th.X - mx)
    if dx < 0 { dx = -dx }
    dy := int(th.Y - my)
    if dy < 0 { dy = -dy }
    if dx <= 128 && dy <= 128 {
      fmt.Printf("near thing idx=%d type=%d pos=(%d,%d) angle=%d flags=0x%04x\n", i, th.Type, th.X, th.Y, th.Angle, uint16(th.Flags))
    }
  }
  for i, th := range m.Things {
    x:=int64(th.X)<<16
    y:=int64(th.Y)<<16
    if x == -4194304 && y == 16777216 {
      fmt.Printf("thing idx=%d type=%d angle=%d flags=0x%04x\n", i, th.Type, th.Angle, uint16(th.Flags))
    }
  }
  idx:=692
  ld:=m.Linedefs[idx]
  a:=m.Vertexes[ld.V1]
  b:=m.Vertexes[ld.V2]
  fmt.Printf("line idx=%d a=(%d,%d) b=(%d,%d) flags=0x%04x special=%d tag=%d front=%d back=%d\n",
    idx, a.X, a.Y, b.X, b.Y, uint16(ld.Flags), ld.Special, ld.Tag, secForSide(m, ld.SideNum[0]), secForSide(m, ld.SideNum[1]))
  px, py := int16(-22), int16(-586)
  fmt.Printf("near player approx=(%d,%d)\n", px, py)
  for i, ld := range m.Linedefs {
    a:=m.Vertexes[ld.V1]
    b:=m.Vertexes[ld.V2]
    minx,maxx:=a.X,a.X
    miny,maxy:=a.Y,a.Y
    if b.X<minx { minx=b.X }; if b.X>maxx { maxx=b.X }
    if b.Y<miny { miny=b.Y }; if b.Y>maxy { maxy=b.Y }
    if px >= minx-64 && px <= maxx+64 && py >= miny-64 && py <= maxy+64 {
      fmt.Printf("near line=%d a=(%d,%d) b=(%d,%d) flags=0x%04x special=%d front=%d back=%d\n",
        i, a.X, a.Y, b.X, b.Y, uint16(ld.Flags), ld.Special, secForSide(m, ld.SideNum[0]), secForSide(m, ld.SideNum[1]))
    }
  }
}
