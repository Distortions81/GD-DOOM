package mapdata

type MapName string

type Thing struct {
	X     int16
	Y     int16
	Angle int16
	Type  int16
	Flags int16
}

type Vertex struct {
	X int16
	Y int16
}

type Linedef struct {
	V1      uint16
	V2      uint16
	Flags   uint16
	Special uint16
	Tag     uint16
	SideNum [2]int16
}

type Sidedef struct {
	TextureOffset int16
	RowOffset     int16
	Top           string
	Bottom        string
	Mid           string
	Sector        uint16
}

type Seg struct {
	StartVertex uint16
	EndVertex   uint16
	Angle       uint16
	Linedef     uint16
	Direction   uint16
	Offset      uint16
}

type SubSector struct {
	SegCount uint16
	FirstSeg uint16
}

type Node struct {
	X       int16
	Y       int16
	DX      int16
	DY      int16
	BBoxR   [4]int16
	BBoxL   [4]int16
	ChildID [2]uint16
}

type Sector struct {
	FloorHeight   int16
	CeilingHeight int16
	FloorPic      string
	CeilingPic    string
	Light         int16
	Special       int16
	Tag           int16
}

type RejectMatrix struct {
	SectorCount int
	Data        []byte
}

type BlockMap struct {
	OriginX int16
	OriginY int16
	Width   int16
	Height  int16
	Offsets []uint16
	Cells   [][]int16
}

type Map struct {
	Name         MapName
	Things       []Thing
	Vertexes     []Vertex
	Linedefs     []Linedef
	Sidedefs     []Sidedef
	Sectors      []Sector
	Segs         []Seg
	SubSectors   []SubSector
	Nodes        []Node
	Reject       []byte
	RejectMatrix *RejectMatrix
	Blockmap     []int16
	BlockMap     *BlockMap
}
