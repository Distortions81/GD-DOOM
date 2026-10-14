package roomhost

import (
	"encoding/binary"
	"fmt"
	"strings"
	"testing"

	"gddoom/internal/mapdata"
	"gddoom/internal/wad"
)

// Map directory entries intentionally reuse the same serialized payloads. This
// models WAD aliasing: a file's byte size does not bound repeated decode work.
func uploadGeometryFixture(t *testing.T, names []string, lumps map[string][]byte) *wad.File {
	t.Helper()
	data := make([]byte, 12)
	copy(data, "PWAD")
	type region struct{ start, length uint32 }
	regions := make(map[string]region)
	for _, spec := range uploadMapLumps {
		payload := lumps[spec.name]
		if spec.name == "BLOCKMAP" && payload == nil {
			payload = make([]byte, 8) // empty 0×0 blockmap
		}
		regions[spec.name] = region{uint32(len(data)), uint32(len(payload))}
		data = append(data, payload...)
	}
	directory := make([]byte, len(names)*11*16)
	entry := 0
	for _, name := range names {
		copy(directory[entry*16+8:entry*16+16], name)
		entry++
		for _, spec := range uploadMapLumps {
			r := regions[spec.name]
			binary.LittleEndian.PutUint32(directory[entry*16:], r.start)
			binary.LittleEndian.PutUint32(directory[entry*16+4:], r.length)
			copy(directory[entry*16+8:entry*16+16], spec.name)
			entry++
		}
	}
	binary.LittleEndian.PutUint32(data[4:], uint32(entry))
	binary.LittleEndian.PutUint32(data[8:], uint32(len(data)))
	data = append(data, directory...)
	file, err := wad.OpenData("upload.wad", data)
	if err != nil {
		t.Fatal(err)
	}
	return file
}

func sharedUploadBlockmap(width, height, listSize int) []byte {
	cells := width * height
	listOffset := 4 + cells
	data := make([]byte, (listOffset+listSize+1)*2)
	binary.LittleEndian.PutUint16(data[4:], uint16(width))
	binary.LittleEndian.PutUint16(data[6:], uint16(height))
	for i := 0; i < cells; i++ {
		binary.LittleEndian.PutUint16(data[8+i*2:], uint16(listOffset))
	}
	binary.LittleEndian.PutUint16(data[(listOffset+listSize)*2:], 0xffff)
	return data
}

func TestUploadGeometrySharewareAndMapReplacementRemainSupported(t *testing.T) {
	base := uploadBase(t)
	file, err := wad.OpenData("DOOM1.WAD", base)
	if err != nil {
		t.Fatal(err)
	}
	if err := validateUploadGeometry(file); err != nil {
		t.Fatalf("normal shareware maps rejected: %v", err)
	}
	addon, err := wad.OpenData("replacement.wad", replacementMap(t, base))
	if err != nil {
		t.Fatal(err)
	}
	if err := validateUploadGeometry(wad.Merge(file, addon)); err != nil {
		t.Fatalf("normal map overlay rejected: %v", err)
	}
	// Only the effective last definition is decoded by LoadMap.
	oversized := uploadGeometryFixture(t, []string{"E1M1"}, map[string][]byte{"SECTORS": make([]byte, (maxUploadSectors+1)*26)})
	if err := validateUploadGeometry(wad.Merge(oversized, addon)); err != nil {
		t.Fatalf("overridden geometry affected the effective map: %v", err)
	}
	if err := validateUploadGeometry(wad.Merge(addon, oversized)); err == nil {
		t.Fatal("last-definition oversized geometry bypassed the cap")
	}
}

func TestUploadGeometryRejectSynthesisIsBoundedBeforeDecode(t *testing.T) {
	for _, sectors := range []int{maxUploadSectors, maxUploadSectors + 1} {
		file := uploadGeometryFixture(t, []string{"MAP01"}, map[string][]byte{"SECTORS": make([]byte, sectors*26)})
		err := validateUploadGeometry(file)
		if sectors == maxUploadSectors {
			if err != nil {
				t.Fatalf("boundary sector count rejected: %v", err)
			}
			// The accepted boundary actually decodes a bounded 8 MiB matrix.
			m, err := mapdata.LoadMap(file, "MAP01")
			if err != nil || len(m.RejectMatrix.Data) != 8<<20 {
				t.Fatalf("bounded REJECT synthesis: map=%v err=%v", m != nil, err)
			}
		} else if err == nil || !strings.Contains(err.Error(), "SECTORS") {
			t.Fatalf("oversized REJECT expansion not rejected before LoadMap: %v", err)
		}
	}
}

func TestUploadGeometrySharedBlockmapListsCountEveryExpansion(t *testing.T) {
	// Both files are about 65 KiB. Increasing one shared list by one entry
	// changes the decoded expansion from exactly 2 Mi entries to over the cap.
	for _, listSize := range []int{64, 65} {
		data := sharedUploadBlockmap(256, 128, listSize)
		cells, entries, err := validateUploadBlockmap(data)
		if listSize == 64 {
			if err != nil || cells != 32768 || entries != maxUploadBlockEntries {
				t.Fatalf("boundary blockmap rejected: cells=%d entries=%d err=%v", cells, entries, err)
			}
		} else if err == nil || !strings.Contains(err.Error(), "expanded lists") {
			t.Fatalf("shared lists bypassed expansion cap: %v", err)
		}
	}
}

func TestUploadGeometryBlockmapStructureAndCellLimits(t *testing.T) {
	oversizedCells := make([]byte, (4+257*256)*2)
	binary.LittleEndian.PutUint16(oversizedCells[4:], 257)
	binary.LittleEndian.PutUint16(oversizedCells[6:], 256)
	badOffset := sharedUploadBlockmap(1, 1, 1)
	binary.LittleEndian.PutUint16(badOffset[8:], 0xffff)
	unterminated := sharedUploadBlockmap(1, 1, 1)
	binary.LittleEndian.PutUint16(unterminated[len(unterminated)-2:], 0)
	for name, data := range map[string][]byte{
		"cell limit": oversizedCells, "outside offset": badOffset, "unterminated": unterminated,
		"truncated header": make([]byte, 6), "odd length": make([]byte, 9), "byte limit": make([]byte, maxUploadBlockBytes+2),
	} {
		t.Run(name, func(t *testing.T) {
			if _, _, err := validateUploadBlockmap(data); err == nil {
				t.Fatal("malformed or oversized blockmap accepted")
			}
		})
	}
}

func TestUploadGeometryCombinedBudgetStopsAliasedMapAmplification(t *testing.T) {
	names := make([]string, 40)
	for i := range names {
		names[i] = fmt.Sprintf("MAP%02d", i)
	}
	file := uploadGeometryFixture(t, names, map[string][]byte{"SECTORS": make([]byte, maxUploadSectors*26)})
	if err := validateUploadGeometry(file); err == nil || !strings.Contains(err.Error(), "combined geometry budget") {
		t.Fatalf("many maps referencing one sector payload bypassed aggregate budget: %v", err)
	}
	file = uploadGeometryFixture(t, names[:5], map[string][]byte{"BLOCKMAP": sharedUploadBlockmap(256, 128, 64)})
	if err := validateUploadGeometry(file); err == nil || !strings.Contains(err.Error(), "combined geometry budget") {
		t.Fatalf("aggregate blockmap scan work was unbounded: %v", err)
	}
}

func TestUploadGeometryOtherRecordCountsAndAlignment(t *testing.T) {
	for _, name := range []string{"THINGS", "LINEDEFS", "SIDEDEFS", "VERTEXES", "SEGS", "SSECTORS", "NODES"} {
		t.Run(name, func(t *testing.T) {
			for _, spec := range uploadMapLumps {
				if spec.name != name {
					continue
				}
				file := uploadGeometryFixture(t, []string{"MAP01"}, map[string][]byte{name: make([]byte, (spec.maxCount+1)*spec.entrySize)})
				if err := validateUploadGeometry(file); err == nil || !strings.Contains(err.Error(), name) {
					t.Fatalf("record count not bounded: %v", err)
				}
				file = uploadGeometryFixture(t, []string{"MAP01"}, map[string][]byte{name: {0}})
				if err := validateUploadGeometry(file); err == nil {
					t.Fatal("partial record accepted")
				}
			}
		})
	}
}
