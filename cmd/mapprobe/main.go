package main

import (
	"flag"
	"fmt"
	"log"
	"strconv"
	"strings"

	"gddoom/internal/mapdata"
	"gddoom/internal/wad"
)

func main() {
	var (
		wadPath        = flag.String("wad", "DOOM2.WAD", "path to WAD file")
		mapName        = flag.String("map", "MAP01", "map name")
		sectorIndexes  = flag.String("sectors", "", "comma-separated sector indexes to print")
		lineIndexes    = flag.String("lines", "", "comma-separated linedef indexes to print")
		lineVerts      = flag.String("line-verts", "", "comma-separated linedef indexes whose vertices should be printed")
		thingIndexes   = flag.String("things", "", "comma-separated THING indexes to print")
		tagLines       = flag.Int("tag-lines", -1, "print all linedefs with this tag")
		sectorTagLines = flag.Int("sector-tag-lines", -1, "print all linedefs matching the tag of this sector")
		thingPoint     = flag.String("thing-point", "", "print THINGs at exact map coordinates x,y")
		thingFixed     = flag.String("thing-fixed-point", "", "print THINGs whose fixed-point coordinates (x<<16,y<<16) match x,y")
	)
	flag.Parse()

	wf, err := wad.Open(*wadPath)
	if err != nil {
		log.Fatalf("open wad: %v", err)
	}
	m, err := mapdata.LoadMap(wf, mapdata.MapName(*mapName))
	if err != nil {
		log.Fatalf("load map %s: %v", *mapName, err)
	}

	if idxs, err := parseCSVInts(*sectorIndexes); err != nil {
		log.Fatal(err)
	} else {
		for _, sec := range idxs {
			if sec < 0 || sec >= len(m.Sectors) {
				log.Fatalf("sector index out of range: %d", sec)
			}
			s := m.Sectors[sec]
			fmt.Printf("sector %d floor=%d ceil=%d special=%d tag=%d floorpic=%s ceilpic=%s\n",
				sec, s.FloorHeight, s.CeilingHeight, s.Special, s.Tag, s.FloorPic, s.CeilingPic)
		}
	}

	if idxs, err := parseCSVInts(*lineIndexes); err != nil {
		log.Fatal(err)
	} else {
		for _, lineIdx := range idxs {
			printLineSummary(m, lineIdx)
		}
	}

	if idxs, err := parseCSVInts(*lineVerts); err != nil {
		log.Fatal(err)
	} else {
		for _, lineIdx := range idxs {
			if lineIdx < 0 || lineIdx >= len(m.Linedefs) {
				log.Fatalf("linedef index out of range: %d", lineIdx)
			}
			ld := m.Linedefs[lineIdx]
			v1 := m.Vertexes[ld.V1]
			v2 := m.Vertexes[ld.V2]
			fmt.Printf("line %d verts=(%d,%d)->(%d,%d)\n", lineIdx, v1.X, v1.Y, v2.X, v2.Y)
		}
	}

	if idxs, err := parseCSVInts(*thingIndexes); err != nil {
		log.Fatal(err)
	} else {
		for _, thingIdx := range idxs {
			if thingIdx < 0 || thingIdx >= len(m.Things) {
				log.Fatalf("thing index out of range: %d", thingIdx)
			}
			th := m.Things[thingIdx]
			fmt.Printf("thing idx=%d type=%d x=%d y=%d flags=%d angle=%d\n",
				thingIdx, th.Type, th.X, th.Y, th.Flags, th.Angle)
		}
	}

	if *tagLines >= 0 {
		for i, ld := range m.Linedefs {
			if int(ld.Tag) == *tagLines {
				printLineSummary(m, i)
			}
		}
	}

	if *sectorTagLines >= 0 {
		if *sectorTagLines < 0 || *sectorTagLines >= len(m.Sectors) {
			log.Fatalf("sector index out of range: %d", *sectorTagLines)
		}
		tag := m.Sectors[*sectorTagLines].Tag
		for i, ld := range m.Linedefs {
			if ld.Tag == uint16(tag) {
				printLineSummary(m, i)
			}
		}
	}

	if *thingPoint != "" {
		x, y := parsePoint(*thingPoint)
		for i, th := range m.Things {
			if int(th.X) == x && int(th.Y) == y {
				fmt.Printf("thing at %d,%d idx=%d type=%d angle=%d flags=%d\n",
					x, y, i, th.Type, th.Angle, th.Flags)
			}
		}
	}

	if *thingFixed != "" {
		x, y := parsePoint64(*thingFixed)
		for i, th := range m.Things {
			tx := int64(th.X) << 16
			ty := int64(th.Y) << 16
			if tx == x && ty == y {
				fmt.Printf("thing fixed=%d,%d idx=%d type=%d angle=%d flags=%d map=(%d,%d)\n",
					x, y, i, th.Type, th.Angle, th.Flags, th.X, th.Y)
			}
		}
	}
}

func parseCSVInts(raw string) ([]int, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	parts := strings.Split(raw, ",")
	out := make([]int, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		v, err := strconv.Atoi(part)
		if err != nil {
			return nil, fmt.Errorf("parse int %q: %w", part, err)
		}
		out = append(out, v)
	}
	return out, nil
}

func parsePoint(raw string) (int, int) {
	parts := strings.Split(strings.TrimSpace(raw), ",")
	if len(parts) != 2 {
		log.Fatalf("point must be x,y: %q", raw)
	}
	x, err := strconv.Atoi(strings.TrimSpace(parts[0]))
	if err != nil {
		log.Fatalf("parse point x %q: %v", parts[0], err)
	}
	y, err := strconv.Atoi(strings.TrimSpace(parts[1]))
	if err != nil {
		log.Fatalf("parse point y %q: %v", parts[1], err)
	}
	return x, y
}

func parsePoint64(raw string) (int64, int64) {
	parts := strings.Split(strings.TrimSpace(raw), ",")
	if len(parts) != 2 {
		log.Fatalf("point must be x,y: %q", raw)
	}
	x, err := strconv.ParseInt(strings.TrimSpace(parts[0]), 10, 64)
	if err != nil {
		log.Fatalf("parse point x %q: %v", parts[0], err)
	}
	y, err := strconv.ParseInt(strings.TrimSpace(parts[1]), 10, 64)
	if err != nil {
		log.Fatalf("parse point y %q: %v", parts[1], err)
	}
	return x, y
}

func printLineSummary(m *mapdata.Map, lineIdx int) {
	if lineIdx < 0 || lineIdx >= len(m.Linedefs) {
		log.Fatalf("linedef index out of range: %d", lineIdx)
	}
	ld := m.Linedefs[lineIdx]
	s0, s1 := ld.SideNum[0], ld.SideNum[1]
	sec0, sec1 := -1, -1
	if s0 >= 0 {
		sec0 = int(m.Sidedefs[s0].Sector)
	}
	if s1 >= 0 {
		sec1 = int(m.Sidedefs[s1].Sector)
	}
	fmt.Printf("line %d flags=%d special=%d tag=%d sides=(%d,%d) sectors=(%d,%d)\n",
		lineIdx, ld.Flags, ld.Special, ld.Tag, s0, s1, sec0, sec1)
}
