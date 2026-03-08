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
  for _, idx := range []int{35,20,39,31,133,136} {
    s := m.Sectors[idx]
    fmt.Printf("sector[%d] floor=%d ceil=%d special=%d tag=%d\n", idx, s.FloorHeight, s.CeilingHeight, s.Special, s.Tag)
  }
}
