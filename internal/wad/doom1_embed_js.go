//go:build js && wasm

package wad

import (
	_ "embed"
	"path/filepath"
	"strings"
)

//go:embed DOOM1.WAD
var embeddedDOOM1WAD []byte

func embeddedDataForPath(path string) ([]byte, bool) {
	base := strings.ToUpper(filepath.Base(strings.TrimSpace(path)))
	if base == "DOOM1.WAD" && len(embeddedDOOM1WAD) > 0 {
		return embeddedDOOM1WAD, true
	}
	return nil, false
}
