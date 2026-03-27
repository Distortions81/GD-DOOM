package main

import (
	"flag"
	"fmt"
	"gddoom/internal/mapdata"
	"gddoom/internal/wad"
	"log"
)

func secForSide(m *mapdata.Map, side int16) int {
	if side < 0 || int(side) >= len(m.Sidedefs) {
		return -1
	}
	return int(m.Sidedefs[int(side)].Sector)
}

func main() {
	wadPath := flag.String("wad", "", "path to IWAD/PWAD")
	mapName := flag.String("map", "", "map name, e.g. E1M5")
	sector := flag.Int("sector", -1, "sector index to inspect")
	flag.Parse()

	if *wadPath == "" || *mapName == "" || *sector < 0 {
		log.Fatal("usage: tmpinspect -wad <path> -map <name> -sector <idx>")
	}

	wf, err := wad.Open(*wadPath)
	if err != nil {
		log.Fatal(err)
	}
	m, err := mapdata.LoadMap(wf, mapdata.MapName(*mapName))
	if err != nil {
		log.Fatal(err)
	}
	if *sector >= len(m.Sectors) {
		log.Fatalf("sector %d out of range (len=%d)", *sector, len(m.Sectors))
	}

	s := m.Sectors[*sector]
	fmt.Printf("map=%s sector=%d floor=%d ceil=%d light=%d special=%d tag=%d\n",
		m.Name, *sector, s.FloorHeight, s.CeilingHeight, s.Light, s.Special, s.Tag)
	if s.Tag != 0 {
		fmt.Printf("sectors with tag %d:\n", s.Tag)
		for i, sec := range m.Sectors {
			if sec.Tag == s.Tag {
				fmt.Printf("  sector=%d floor=%d ceil=%d special=%d\n", i, sec.FloorHeight, sec.CeilingHeight, sec.Special)
			}
		}
	}

	for i, ld := range m.Linedefs {
		info := mapdata.LookupLineSpecial(ld.Special)
		if info.Special == 0 || info.Name == "" {
			continue
		}
		fs := secForSide(m, ld.SideNum[0])
		bs := secForSide(m, ld.SideNum[1])
		targets := false
		if int(ld.Tag) == int(s.Tag) && s.Tag != 0 {
			targets = true
		}
		if fs == *sector || bs == *sector || targets {
			v1 := m.Vertexes[ld.V1]
			v2 := m.Vertexes[ld.V2]
			fmt.Printf("line=%d special=%d name=%s trigger=%s tag=%d front=%d back=%d a=(%d,%d) b=(%d,%d)\n",
				i, ld.Special, info.Name, info.Trigger, ld.Tag, fs, bs, v1.X, v1.Y, v2.X, v2.Y)
		}
	}
}
