package roomhost

import (
	"encoding/binary"
	"fmt"

	"gddoom/internal/mapdata"
	"gddoom/internal/wad"
)

const (
	maxUploadSectors             = 8192
	maxUploadBlockCells          = 65536
	maxUploadBlockEntries        = 2 << 20
	maxUploadBlockBytes          = 4 << 20
	maxUploadCatalogDecodeBytes  = 256 << 20
	maxUploadCatalogBlockEntries = 8 << 20
)

var uploadMapLumps = [...]struct {
	name      string
	entrySize int
	maxCount  int
}{
	{"THINGS", 10, 16384},
	{"LINEDEFS", 14, 65536},
	{"SIDEDEFS", 30, 65536},
	{"VERTEXES", 4, 65536},
	{"SEGS", 12, 65536},
	{"SSECTORS", 4, 32768},
	{"NODES", 28, 32768},
	{"SECTORS", 26, maxUploadSectors},
	{"REJECT", 0, 0},
	{"BLOCKMAP", 0, 0},
}

// validateUploadGeometry runs before LoadMap, which can synthesize a quadratic
// REJECT matrix and duplicate BLOCKMAP lists once per cell. File-size bounds do
// not bound either expansion. These limits apply only to uploaded packs, not to
// operator-installed or locally loaded WADs.
func validateUploadGeometry(file *wad.File) error {
	if file == nil {
		return fmt.Errorf("uploaded WAD has no directory")
	}
	names := mapdata.AvailableMapNames(file)
	last := make(map[string]int, len(names))
	for _, name := range names {
		last[string(name)] = -1
	}
	for i, lump := range file.Lumps {
		if _, exists := last[lump.Name]; exists {
			last[lump.Name] = i
		}
	}
	var catalogBytes, catalogEntries uint64
	for _, name := range names {
		index, exists := last[string(name)]
		if !exists {
			continue
		}
		delete(last, string(name))
		// Match LoadMap's last-definition rule. An incomplete replacement is
		// rejected there before any geometry allocation, so it need not be read.
		if index < 0 || index+len(uploadMapLumps) >= len(file.Lumps) {
			continue
		}
		complete := true
		for i, spec := range uploadMapLumps {
			if file.Lumps[index+1+i].Name != spec.name {
				complete = false
				break
			}
		}
		if !complete {
			continue
		}
		var estimate uint64
		sectors := 0
		for i, spec := range uploadMapLumps[:8] {
			size := int(file.Lumps[index+1+i].Size)
			if size < 0 || size%spec.entrySize != 0 || size/spec.entrySize > spec.maxCount {
				return fmt.Errorf("map %s %s exceeds upload geometry limit (%d records of %d bytes)", name, spec.name, spec.maxCount, spec.entrySize)
			}
			// Four times the serialized geometry bounds decoded records, texture
			// names, and their allocation overhead without allocating any of it.
			estimate += uint64(size) * 4
			if spec.name == "SECTORS" {
				sectors = size / spec.entrySize
			}
		}
		// Charge the full matrix even when supplied by the file: repeated maps
		// sharing the same small SECTORS lump must still have bounded decode work.
		estimate += (uint64(sectors)*uint64(sectors) + 7) / 8
		block, err := file.LumpDataView(file.Lumps[index+len(uploadMapLumps)])
		if err != nil {
			return fmt.Errorf("map %s BLOCKMAP: %w", name, err)
		}
		cells, entries, err := validateUploadBlockmap(block)
		if err != nil {
			return fmt.Errorf("map %s BLOCKMAP: %w", name, err)
		}
		estimate += uint64(len(block)) + uint64(cells)*32 + entries*2
		catalogBytes += estimate
		catalogEntries += entries
		if catalogBytes > maxUploadCatalogDecodeBytes || catalogEntries > maxUploadCatalogBlockEntries {
			return fmt.Errorf("uploaded maps exceed combined geometry budget (256 MiB decoded estimate or 8 million BLOCKMAP entries)")
		}
	}
	return nil
}

// Count precisely the lists LoadMap will scan and copy. Counting each reference,
// including repeated offsets, prevents the usual shared-list expansion attack.
func validateUploadBlockmap(data []byte) (int, uint64, error) {
	if len(data) < 8 || len(data)%2 != 0 || len(data) > maxUploadBlockBytes {
		return 0, 0, fmt.Errorf("size must be 8 bytes to 4 MiB and divisible by two")
	}
	width := int(int16(binary.LittleEndian.Uint16(data[4:6])))
	height := int(int16(binary.LittleEndian.Uint16(data[6:8])))
	if width < 0 || height < 0 {
		return 0, 0, fmt.Errorf("negative dimensions")
	}
	cells := width * height
	words := len(data) / 2
	if cells > maxUploadBlockCells || cells > words-4 {
		return 0, 0, fmt.Errorf("cell table exceeds upload limit of %d cells or its lump bounds", maxUploadBlockCells)
	}
	var entries uint64
	for i := 0; i < cells; i++ {
		position := int(binary.LittleEndian.Uint16(data[8+i*2 : 10+i*2]))
		if position >= words {
			return 0, 0, fmt.Errorf("cell %d offset is outside the lump", i)
		}
		for position < words && binary.LittleEndian.Uint16(data[position*2:position*2+2]) != 0xffff {
			position++
			entries++
			if entries > maxUploadBlockEntries {
				return 0, 0, fmt.Errorf("expanded lists exceed upload limit of %d entries", maxUploadBlockEntries)
			}
		}
		if position == words {
			return 0, 0, fmt.Errorf("cell %d list has no terminator", i)
		}
	}
	return cells, entries, nil
}
