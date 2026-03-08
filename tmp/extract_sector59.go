package main

import (
  "fmt"
  "gddoom/internal/mapdata"
  "gddoom/internal/wad"
)

func main() {
  w, err := wad.Open("/home/dist/github/GD-DOOM/doom.wad")
  if err != nil { panic(err) }
  m, err := mapdata.LoadMap(w, "E1M5")
  if err != nil { panic(err) }
  for _, idx := range []int{59, 60, 61, 62, 63} {
    s := m.Sectors[idx]
    fmt.Printf("sector[%d] floor=%d ceil=%d light=%d special=%d tag=%d\n", idx, s.FloorHeight, s.CeilingHeight, s.Light, s.Special, s.Tag)
  }
}
