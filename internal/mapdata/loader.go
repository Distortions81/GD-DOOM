package mapdata

import (
	"encoding/binary"
	"fmt"
	"regexp"
	"strings"

	"gddoom/internal/wad"
)

var (
	episodeMapRE = regexp.MustCompile(`^E[1-9]M[1-9]$`)
	mapXXRE      = regexp.MustCompile(`^MAP[0-9][0-9]$`)
)

var requiredLumps = []string{
	"THINGS",
	"LINEDEFS",
	"SIDEDEFS",
	"VERTEXES",
	"SEGS",
	"SSECTORS",
	"NODES",
	"SECTORS",
	"REJECT",
	"BLOCKMAP",
}

func FirstMapName(f *wad.File) (MapName, error) {
	for i, l := range f.Lumps {
		if isMapMarker(l.Name) && hasRequiredLumpsAt(f, i) {
			return MapName(l.Name), nil
		}
	}
	return "", fmt.Errorf("missing map marker with required lump set")
}

func LoadMap(f *wad.File, name MapName) (*Map, error) {
	target := strings.ToUpper(strings.TrimSpace(string(name)))
	if !isMapMarker(target) {
		return nil, fmt.Errorf("invalid map name %q", name)
	}

	idx := findMapMarkerIndex(f, target)
	if idx == -1 {
		return nil, fmt.Errorf("missing map marker %q", target)
	}

	if !hasRequiredLumpsAt(f, idx) {
		return nil, fmt.Errorf("missing required map lump set for %s", target)
	}

	resolved := make(map[string]wad.Lump, len(requiredLumps))
	for i, req := range requiredLumps {
		got := f.Lumps[idx+1+i]
		if got.Name != req {
			return nil, fmt.Errorf("missing required map lump %s after marker %s (got %s)", req, target, got.Name)
		}
		resolved[req] = got
	}

	m := &Map{Name: MapName(target)}
	var err error

	if m.Things, err = decodeThings(f, resolved["THINGS"]); err != nil {
		return nil, err
	}
	if m.Linedefs, err = decodeLinedefs(f, resolved["LINEDEFS"]); err != nil {
		return nil, err
	}
	if m.Sidedefs, err = decodeSidedefs(f, resolved["SIDEDEFS"]); err != nil {
		return nil, err
	}
	if m.Vertexes, err = decodeVertexes(f, resolved["VERTEXES"]); err != nil {
		return nil, err
	}
	if m.Segs, err = decodeSegs(f, resolved["SEGS"]); err != nil {
		return nil, err
	}
	if m.SubSectors, err = decodeSubSectors(f, resolved["SSECTORS"]); err != nil {
		return nil, err
	}
	if m.Nodes, err = decodeNodes(f, resolved["NODES"]); err != nil {
		if wadHasXNOD(f) {
			return nil, fmt.Errorf("map %s has no usable vanilla NODES lump and includes XNOD data; it may use a BSP node format we do not support", target)
		}
		return nil, err
	}
	if m.Sectors, err = decodeSectors(f, resolved["SECTORS"]); err != nil {
		return nil, err
	}
	if m.Reject, err = decodeReject(f, resolved["REJECT"]); err != nil {
		return nil, err
	}
	m.RejectMatrix, err = decodeRejectMatrix(m.Reject, len(m.Sectors))
	if err != nil {
		return nil, err
	}
	if m.Blockmap, m.BlockMap, err = decodeBlockmap(f, resolved["BLOCKMAP"]); err != nil {
		return nil, err
	}

	if err := Validate(m); err != nil {
		return nil, err
	}
	return m, nil
}

func findMapMarkerIndex(f *wad.File, target string) int {
	if f == nil {
		return -1
	}
	for i := len(f.Lumps) - 1; i >= 0; i-- {
		if f.Lumps[i].Name == target {
			return i
		}
	}
	return -1
}

func isMapMarker(name string) bool {
	return episodeMapRE.MatchString(name) || mapXXRE.MatchString(name)
}

func hasRequiredLumpsAt(f *wad.File, markerIndex int) bool {
	if markerIndex < 0 {
		return false
	}
	if markerIndex+len(requiredLumps) >= len(f.Lumps) {
		return false
	}
	for i, req := range requiredLumps {
		if f.Lumps[markerIndex+1+i].Name != req {
			return false
		}
	}
	return true
}

func wadHasXNOD(f *wad.File) bool {
	if f == nil {
		return false
	}
	for i := range f.Lumps {
		if f.Lumps[i].Name == "XNOD" {
			return true
		}
	}
	return false
}

func decodeThings(f *wad.File, l wad.Lump) ([]Thing, error) {
	data, err := f.LumpDataView(l)
	if err != nil {
		return nil, err
	}
	const size = 10
	if len(data)%size != 0 {
		return nil, fmt.Errorf("THINGS size %d is not divisible by %d", len(data), size)
	}
	out := make([]Thing, 0, len(data)/size)
	for i := 0; i < len(data); i += size {
		r := data[i : i+size]
		out = append(out, Thing{
			X:     int16(binary.LittleEndian.Uint16(r[0:2])),
			Y:     int16(binary.LittleEndian.Uint16(r[2:4])),
			Angle: int16(binary.LittleEndian.Uint16(r[4:6])),
			Type:  int16(binary.LittleEndian.Uint16(r[6:8])),
			Flags: int16(binary.LittleEndian.Uint16(r[8:10])),
		})
	}
	return out, nil
}

func decodeLinedefs(f *wad.File, l wad.Lump) ([]Linedef, error) {
	data, err := f.LumpDataView(l)
	if err != nil {
		return nil, err
	}
	const size = 14
	if len(data)%size != 0 {
		return nil, fmt.Errorf("LINEDEFS size %d is not divisible by %d", len(data), size)
	}
	out := make([]Linedef, 0, len(data)/size)
	for i := 0; i < len(data); i += size {
		r := data[i : i+size]
		out = append(out, Linedef{
			V1:      binary.LittleEndian.Uint16(r[0:2]),
			V2:      binary.LittleEndian.Uint16(r[2:4]),
			Flags:   binary.LittleEndian.Uint16(r[4:6]),
			Special: binary.LittleEndian.Uint16(r[6:8]),
			Tag:     binary.LittleEndian.Uint16(r[8:10]),
			SideNum: [2]int16{int16(binary.LittleEndian.Uint16(r[10:12])), int16(binary.LittleEndian.Uint16(r[12:14]))},
		})
	}
	return out, nil
}

func decodeSidedefs(f *wad.File, l wad.Lump) ([]Sidedef, error) {
	data, err := f.LumpDataView(l)
	if err != nil {
		return nil, err
	}
	const size = 30
	if len(data)%size != 0 {
		return nil, fmt.Errorf("SIDEDEFS size %d is not divisible by %d", len(data), size)
	}
	out := make([]Sidedef, 0, len(data)/size)
	for i := 0; i < len(data); i += size {
		r := data[i : i+size]
		out = append(out, Sidedef{
			TextureOffset: int16(binary.LittleEndian.Uint16(r[0:2])),
			RowOffset:     int16(binary.LittleEndian.Uint16(r[2:4])),
			Top:           parseName(r[4:12]),
			Bottom:        parseName(r[12:20]),
			Mid:           parseName(r[20:28]),
			Sector:        binary.LittleEndian.Uint16(r[28:30]),
		})
	}
	return out, nil
}

func decodeVertexes(f *wad.File, l wad.Lump) ([]Vertex, error) {
	data, err := f.LumpDataView(l)
	if err != nil {
		return nil, err
	}
	const size = 4
	if len(data)%size != 0 {
		return nil, fmt.Errorf("VERTEXES size %d is not divisible by %d", len(data), size)
	}
	out := make([]Vertex, 0, len(data)/size)
	for i := 0; i < len(data); i += size {
		r := data[i : i+size]
		out = append(out, Vertex{
			X: int16(binary.LittleEndian.Uint16(r[0:2])),
			Y: int16(binary.LittleEndian.Uint16(r[2:4])),
		})
	}
	return out, nil
}

func decodeSegs(f *wad.File, l wad.Lump) ([]Seg, error) {
	data, err := f.LumpDataView(l)
	if err != nil {
		return nil, err
	}
	const size = 12
	if len(data)%size != 0 {
		return nil, fmt.Errorf("SEGS size %d is not divisible by %d", len(data), size)
	}
	out := make([]Seg, 0, len(data)/size)
	for i := 0; i < len(data); i += size {
		r := data[i : i+size]
		out = append(out, Seg{
			StartVertex: binary.LittleEndian.Uint16(r[0:2]),
			EndVertex:   binary.LittleEndian.Uint16(r[2:4]),
			Angle:       binary.LittleEndian.Uint16(r[4:6]),
			Linedef:     binary.LittleEndian.Uint16(r[6:8]),
			Direction:   binary.LittleEndian.Uint16(r[8:10]),
			Offset:      binary.LittleEndian.Uint16(r[10:12]),
		})
	}
	return out, nil
}

func decodeSubSectors(f *wad.File, l wad.Lump) ([]SubSector, error) {
	data, err := f.LumpDataView(l)
	if err != nil {
		return nil, err
	}
	const size = 4
	if len(data)%size != 0 {
		return nil, fmt.Errorf("SSECTORS size %d is not divisible by %d", len(data), size)
	}
	out := make([]SubSector, 0, len(data)/size)
	for i := 0; i < len(data); i += size {
		r := data[i : i+size]
		out = append(out, SubSector{
			SegCount: binary.LittleEndian.Uint16(r[0:2]),
			FirstSeg: binary.LittleEndian.Uint16(r[2:4]),
		})
	}
	return out, nil
}

func decodeNodes(f *wad.File, l wad.Lump) ([]Node, error) {
	data, err := f.LumpDataView(l)
	if err != nil {
		return nil, err
	}
	const size = 28
	if len(data)%size != 0 {
		return nil, fmt.Errorf("NODES size %d is not divisible by %d", len(data), size)
	}
	out := make([]Node, 0, len(data)/size)
	for i := 0; i < len(data); i += size {
		r := data[i : i+size]
		n := Node{
			X:  int16(binary.LittleEndian.Uint16(r[0:2])),
			Y:  int16(binary.LittleEndian.Uint16(r[2:4])),
			DX: int16(binary.LittleEndian.Uint16(r[4:6])),
			DY: int16(binary.LittleEndian.Uint16(r[6:8])),
		}
		for bi := 0; bi < 4; bi++ {
			n.BBoxR[bi] = int16(binary.LittleEndian.Uint16(r[8+bi*2 : 10+bi*2]))
			n.BBoxL[bi] = int16(binary.LittleEndian.Uint16(r[16+bi*2 : 18+bi*2]))
		}
		n.ChildID[0] = binary.LittleEndian.Uint16(r[24:26])
		n.ChildID[1] = binary.LittleEndian.Uint16(r[26:28])
		out = append(out, n)
	}
	return out, nil
}

func decodeSectors(f *wad.File, l wad.Lump) ([]Sector, error) {
	data, err := f.LumpDataView(l)
	if err != nil {
		return nil, err
	}
	const size = 26
	if len(data)%size != 0 {
		return nil, fmt.Errorf("SECTORS size %d is not divisible by %d", len(data), size)
	}
	out := make([]Sector, 0, len(data)/size)
	for i := 0; i < len(data); i += size {
		r := data[i : i+size]
		out = append(out, Sector{
			FloorHeight:   int16(binary.LittleEndian.Uint16(r[0:2])),
			CeilingHeight: int16(binary.LittleEndian.Uint16(r[2:4])),
			FloorPic:      parseName(r[4:12]),
			CeilingPic:    parseName(r[12:20]),
			Light:         int16(binary.LittleEndian.Uint16(r[20:22])),
			Special:       int16(binary.LittleEndian.Uint16(r[22:24])),
			Tag:           int16(binary.LittleEndian.Uint16(r[24:26])),
		})
	}
	return out, nil
}

func decodeReject(f *wad.File, l wad.Lump) ([]byte, error) {
	return f.LumpDataView(l)
}

func decodeBlockmap(f *wad.File, l wad.Lump) ([]int16, *BlockMap, error) {
	data, err := f.LumpDataView(l)
	if err != nil {
		return nil, nil, err
	}
	if len(data)%2 != 0 {
		return nil, nil, fmt.Errorf("BLOCKMAP size %d is not divisible by 2", len(data))
	}
	out := make([]int16, 0, len(data)/2)
	for i := 0; i < len(data); i += 2 {
		out = append(out, int16(binary.LittleEndian.Uint16(data[i:i+2])))
	}
	bm, err := decodeBlockMapWords(out)
	if err != nil {
		return nil, nil, err
	}
	return out, bm, nil
}

func decodeRejectMatrix(reject []byte, sectorCount int) (*RejectMatrix, error) {
	if sectorCount < 0 {
		return nil, fmt.Errorf("invalid sector count %d", sectorCount)
	}
	neededBits := sectorCount * sectorCount
	neededBytes := (neededBits + 7) / 8
	if len(reject) == 0 && neededBytes > 0 {
		reject = make([]byte, neededBytes)
	}
	if len(reject) < neededBytes {
		return nil, fmt.Errorf("REJECT too small: have %d bytes need at least %d for %d sectors", len(reject), neededBytes, sectorCount)
	}
	return &RejectMatrix{
		SectorCount: sectorCount,
		Data:        reject,
	}, nil
}

func decodeBlockMapWords(words []int16) (*BlockMap, error) {
	if len(words) < 4 {
		return nil, fmt.Errorf("BLOCKMAP too small: %d words", len(words))
	}
	width := words[2]
	height := words[3]
	if width < 0 || height < 0 {
		return nil, fmt.Errorf("BLOCKMAP has negative dimensions width=%d height=%d", width, height)
	}
	cellCount := int(width) * int(height)
	if cellCount < 0 || 4+cellCount > len(words) {
		return nil, fmt.Errorf("BLOCKMAP invalid cell table size width=%d height=%d words=%d", width, height, len(words))
	}

	offsets := make([]uint16, cellCount)
	cells := make([][]int16, cellCount)
	for i := 0; i < cellCount; i++ {
		offset := uint16(words[4+i])
		offsets[i] = offset
		pos := int(offset)
		if pos < 0 || pos >= len(words) {
			return nil, fmt.Errorf("BLOCKMAP cell %d has out-of-range offset %d", i, pos)
		}
		start := pos
		for pos < len(words) && words[pos] != -1 {
			pos++
		}
		if pos >= len(words) {
			return nil, fmt.Errorf("BLOCKMAP cell %d list is missing terminator", i)
		}
		entries := make([]int16, pos-start)
		copy(entries, words[start:pos])
		cells[i] = entries
	}

	return &BlockMap{
		OriginX: words[0],
		OriginY: words[1],
		Width:   width,
		Height:  height,
		Offsets: offsets,
		Cells:   cells,
	}, nil
}

func parseName(b []byte) string {
	n := len(b)
	for i := 0; i < len(b); i++ {
		if b[i] == 0x00 {
			n = i
			break
		}
	}
	for n > 0 && b[n-1] == ' ' {
		n--
	}
	return string(b[:n])
}
