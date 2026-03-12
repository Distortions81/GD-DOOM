package wad

// Header is the 12-byte WAD file header.
type Header struct {
	Identification string
	NumLumps       int32
	InfoTableOfs   int32
}

// Lump is a single directory entry in a WAD file.
type Lump struct {
	Name    string
	FilePos int32
	Size    int32
	Index   int

	file *File
}

// File is an opened and indexed WAD file.
type File struct {
	Path   string
	Header Header
	Lumps  []Lump

	data []byte
}
