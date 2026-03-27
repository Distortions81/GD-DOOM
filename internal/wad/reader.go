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
	if data, ok := embeddedDataForPath(path); ok {
		return openData(path, data)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read wad: %w", err)
	}
	return openData(path, data)
}

func OpenData(path string, data []byte) (*File, error) {
	return openData(path, data)
}

func openData(path string, data []byte) (*File, error) {
	if len(data) < headerSize {
		return nil, ErrTruncatedHeader
	}

	hdr := Header{
		Identification: string(data[0:4]),
		NumLumps:       int32(binary.LittleEndian.Uint32(data[4:8])),
		InfoTableOfs:   int32(binary.LittleEndian.Uint32(data[8:12])),
	}
	if hdr.Identification != "IWAD" && hdr.Identification != "PWAD" {
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

	f := &File{Path: path, Header: hdr, Lumps: lumps, data: data}
	for i := range f.Lumps {
		f.Lumps[i].file = f
	}
	return f, nil
}

func EmbeddedDataForPath(path string) ([]byte, bool) {
	return embeddedDataForPath(path)
}

func OpenFiles(paths ...string) (*File, error) {
	trimmed := make([]string, 0, len(paths))
	for _, path := range paths {
		path = strings.TrimSpace(path)
		if path == "" {
			continue
		}
		trimmed = append(trimmed, path)
	}
	if len(trimmed) == 0 {
		return nil, fmt.Errorf("open wad stack: no wad paths")
	}

	files := make([]*File, 0, len(trimmed))
	for _, path := range trimmed {
		f, err := Open(path)
		if err != nil {
			return nil, err
		}
		files = append(files, f)
	}
	return Merge(files...), nil
}

func Merge(files ...*File) *File {
	if len(files) == 0 {
		return nil
	}
	if len(files) == 1 {
		return files[0]
	}

	out := &File{
		Path:   files[0].Path,
		Header: Header{Identification: files[0].Header.Identification},
		Lumps:  make([]Lump, 0, countLumps(files)),
	}
	for _, src := range files {
		if src == nil {
			continue
		}
		if out.Path == "" {
			out.Path = src.Path
		}
		for _, lump := range src.Lumps {
			lump.Index = len(out.Lumps)
			lump.file = src
			out.Lumps = append(out.Lumps, lump)
		}
	}
	out.Header.NumLumps = int32(len(out.Lumps))
	return out
}

func countLumps(files []*File) int {
	total := 0
	for _, f := range files {
		if f != nil {
			total += len(f.Lumps)
		}
	}
	return total
}

func (f *File) LumpByName(name string) (Lump, bool) {
	needle := strings.ToUpper(strings.TrimSpace(name))
	for i := len(f.Lumps) - 1; i >= 0; i-- {
		if f.Lumps[i].Name == needle {
			return f.Lumps[i], true
		}
	}
	return Lump{}, false
}

func (f *File) LumpData(l Lump) ([]byte, error) {
	view, err := f.LumpDataView(l)
	if err != nil {
		return nil, err
	}
	out := make([]byte, len(view))
	copy(out, view)
	return out, nil
}

func (f *File) LumpDataView(l Lump) ([]byte, error) {
	src := f
	if l.file != nil {
		src = l.file
	}
	if src == nil {
		return nil, fmt.Errorf("%w: %s", ErrInvalidLumpBounds, l.Name)
	}
	start := int(l.FilePos)
	end := start + int(l.Size)
	if start < 0 || start > len(src.data) || end < start || end > len(src.data) {
		return nil, fmt.Errorf("%w: %s", ErrInvalidLumpBounds, l.Name)
	}
	return src.data[start:end], nil
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
