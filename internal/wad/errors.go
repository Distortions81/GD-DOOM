package wad

import "errors"

var (
	ErrInvalidIdentification = errors.New("invalid wad identification")
	ErrTruncatedHeader       = errors.New("truncated wad header")
	ErrTruncatedDirectory    = errors.New("truncated wad directory")
	ErrInvalidDirectory      = errors.New("invalid wad directory")
	ErrInvalidLumpBounds     = errors.New("invalid lump bounds")
	ErrLumpNotFound          = errors.New("lump not found")
)
