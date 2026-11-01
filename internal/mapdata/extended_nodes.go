package mapdata

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"fmt"
	"io"
	"math"
)

// Bound decompression and validate counts against remaining bytes before allocating.
const maxExtendedNodesBytes = 256 << 20

func decodeExtendedNodes(m *Map, data []byte) error {
	format := string(data[:4])
	data = data[4:]
	if format == "ZNOD" {
		source := bytes.NewReader(data)
		z, err := zlib.NewReader(source)
		if err != nil {
			return fmt.Errorf("ZNOD: %w", err)
		}
		data, err = io.ReadAll(io.LimitReader(z, maxExtendedNodesBytes+1))
		closeErr := z.Close()
		if err != nil {
			return fmt.Errorf("ZNOD: %w", err)
		}
		if closeErr != nil {
			return closeErr
		}
		if source.Len() != 0 {
			return fmt.Errorf("ZNOD trailing compressed data")
		}
	}
	if len(data) > maxExtendedNodesBytes {
		return fmt.Errorf("extended nodes exceed size limit")
	}
	r := bytes.NewReader(data)
	read := func(v any) error { return binary.Read(r, binary.LittleEndian, v) }
	count := func(stride int) (uint32, error) {
		var n uint32
		if err := read(&n); err != nil {
			return 0, err
		}
		if uint64(n)*uint64(stride) > uint64(r.Len()) {
			return 0, fmt.Errorf("extended nodes count %d exceeds remaining data", n)
		}
		return n, nil
	}
	var original uint32
	if err := read(&original); err != nil {
		return err
	}
	if uint64(original) > uint64(len(m.Vertexes)) {
		return fmt.Errorf("extended nodes original vertex count %d exceeds VERTEXES", original)
	}
	extra, err := count(8)
	if err != nil {
		return err
	}
	verts := make([]Vertex, int(original)+int(extra))
	copy(verts, m.Vertexes[:original])
	for i := int(original); i < len(verts); i++ {
		var xy [2]int32
		if err := read(&xy); err != nil {
			return err
		}
		verts[i] = Vertex{float64(xy[0]) / 65536, float64(xy[1]) / 65536}
	}
	m.Vertexes = verts
	subs, err := count(4)
	if err != nil {
		return err
	}
	if subs == 0 {
		return fmt.Errorf("extended nodes have no subsectors")
	}
	m.SubSectors = make([]SubSector, subs)
	var total uint64
	for i := range m.SubSectors {
		var n uint32
		if err := read(&n); err != nil {
			return err
		}
		if n == 0 {
			return fmt.Errorf("extended subsector has no segs")
		}
		m.SubSectors[i] = SubSector{n, uint32(total)}
		total += uint64(n)
		if total > math.MaxUint32 {
			return fmt.Errorf("extended seg count overflow")
		}
	}
	segs, err := count(11)
	if err != nil {
		return err
	}
	if uint64(segs) != total {
		return fmt.Errorf("extended seg count disagrees with subsectors")
	}
	m.Segs = make([]Seg, segs)
	for i := range m.Segs {
		s := &m.Segs[i]
		var side uint8
		for _, v := range []any{&s.StartVertex, &s.EndVertex, &s.Linedef, &side} {
			if err := read(v); err != nil {
				return err
			}
		}
		s.Direction = uint16(side)
		if uint64(s.StartVertex) >= uint64(len(verts)) || uint64(s.EndVertex) >= uint64(len(verts)) || int(s.Linedef) >= len(m.Linedefs) || side > 1 {
			return fmt.Errorf("extended seg[%d] has invalid references", i)
		}
		line := m.Linedefs[s.Linedef]
		anchor := line.V1
		if side == 1 {
			anchor = line.V2
		}
		if int(anchor) >= len(verts) {
			return fmt.Errorf("extended seg[%d] linedef vertex out of range", i)
		}
		a, b := verts[s.StartVertex], verts[anchor]
		// DSDA GetOffset uses float intermediates, then truncates to 16.16.
		dx, dy := float32(a.X-b.X), float32(a.Y-b.Y)
		s.Offset = math.Trunc(math.Sqrt(float64(dx*dx+dy*dy))*65536) / 65536
	}
	nodes, err := count(32)
	if err != nil {
		return err
	}
	m.Nodes = make([]Node, nodes)
	for i := range m.Nodes {
		if err := read(&m.Nodes[i]); err != nil {
			return err
		}
	}
	if r.Len() != 0 {
		return fmt.Errorf("extended nodes have %d trailing bytes", r.Len())
	}
	if nodes == 0 && subs != 1 {
		return fmt.Errorf("multiple subsectors require BSP nodes")
	}
	m.ExtendedNodes = true
	return nil
}

// Validate every component, including unreachable nodes, without recursive stack growth.
func validateExtendedTree(m *Map) error {
	state := make([]uint8, len(m.Nodes))
	type visit struct {
		index uint32
		exit  bool
	}
	for root := range m.Nodes {
		if state[root] != 0 {
			continue
		}
		stack := []visit{{index: uint32(root)}}
		for len(stack) > 0 {
			v := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			if v.exit {
				state[v.index] = 2
				continue
			}
			if state[v.index] == 1 {
				return fmt.Errorf("extended BSP contains a cycle at node %d", v.index)
			}
			if state[v.index] == 2 {
				continue
			}
			state[v.index] = 1
			stack = append(stack, visit{v.index, true})
			for _, c := range m.Nodes[v.index].ChildID {
				if c&SubsectorFlag == 0 {
					stack = append(stack, visit{index: c})
				}
			}
		}
	}
	return nil
}
