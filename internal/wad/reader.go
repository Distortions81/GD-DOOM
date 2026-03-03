package wad

import (
	"encoding/binary"
	"fmt"
	"os"
	"strings"
)

const (
	headerSize    = 12
	directorySize = 16
)

func Open(path string) (*File, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read wad: %w", err)
	}
	if len(data) < headerSize {
		return nil, ErrTruncatedHeader
	}

	hdr := Header{
		Identification: string(data[0:4]),
		NumLumps:       int32(binary.LittleEndian.Uint32(data[4:8])),
		InfoTableOfs:   int32(binary.LittleEndian.Uint32(data[8:12])),
	}
	if hdr.Identification != "IWAD" {
		return nil, fmt.Errorf("%w: got %q", ErrInvalidIdentification, hdr.Identification)
	}
	if hdr.NumLumps < 0 || hdr.InfoTableOfs < 0 {
		return nil, fmt.Errorf("%w: negative directory metadata", ErrInvalidDirectory)
	}

	dirStart := int(hdr.InfoTableOfs)
	dirBytes := int(hdr.NumLumps) * directorySize
	if dirStart > len(data) || dirBytes < 0 || dirStart+dirBytes > len(data) {
		return nil, ErrTruncatedDirectory
	}

	lumps := make([]Lump, 0, hdr.NumLumps)
	for i := 0; i < int(hdr.NumLumps); i++ {
		ofs := dirStart + i*directorySize
		entry := data[ofs : ofs+directorySize]
		filePos := int32(binary.LittleEndian.Uint32(entry[0:4]))
		size := int32(binary.LittleEndian.Uint32(entry[4:8]))
		name := normalizeName(entry[8:16])

		if size < 0 || filePos < 0 {
			return nil, fmt.Errorf("%w: lump[%d] negative position/size", ErrInvalidLumpBounds, i)
		}
		end := int(filePos) + int(size)
		if int(filePos) > len(data) || end < int(filePos) || end > len(data) {
			return nil, fmt.Errorf("%w: lump[%d] out of file bounds", ErrInvalidLumpBounds, i)
		}

		lumps = append(lumps, Lump{
			Name:    name,
			FilePos: filePos,
			Size:    size,
			Index:   i,
		})
	}

	return &File{Path: path, Header: hdr, Lumps: lumps, data: data}, nil
}

func (f *File) LumpByName(name string) (Lump, bool) {
	needle := strings.ToUpper(strings.TrimSpace(name))
	for _, l := range f.Lumps {
		if l.Name == needle {
			return l, true
		}
	}
	return Lump{}, false
}

func (f *File) LumpData(l Lump) ([]byte, error) {
	start := int(l.FilePos)
	end := start + int(l.Size)
	if start < 0 || start > len(f.data) || end < start || end > len(f.data) {
		return nil, fmt.Errorf("%w: %s", ErrInvalidLumpBounds, l.Name)
	}
	out := make([]byte, l.Size)
	copy(out, f.data[start:end])
	return out, nil
}

func normalizeName(b []byte) string {
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
	if n == 0 {
		return ""
	}
	return strings.ToUpper(string(b[:n]))
}
