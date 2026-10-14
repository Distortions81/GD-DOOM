package launchcatalog

import (
	"bytes"
	"crypto/sha1"
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"gddoom/internal/wad"
	"github.com/zeebo/blake3"
)

func writeCatalogTestWAD(t *testing.T, path, marker, tag string) []byte {
	t.Helper()
	names := []string{marker, "THINGS", "LINEDEFS", "SIDEDEFS", "VERTEXES", "SEGS", "SSECTORS", "NODES", "SECTORS", "REJECT", "BLOCKMAP", "TEST"}
	if marker == "" {
		names = []string{"TEST"}
	}
	data := make([]byte, 12+len(tag)+16*len(names))
	copy(data, "PWAD")
	binary.LittleEndian.PutUint32(data[4:8], uint32(len(names)))
	binary.LittleEndian.PutUint32(data[8:12], uint32(12+len(tag)))
	copy(data[12:], tag)
	for i, name := range names {
		entry := data[12+len(tag)+16*i:]
		binary.LittleEndian.PutUint32(entry[0:4], 12)
		if name == "TEST" {
			binary.LittleEndian.PutUint32(entry[4:8], uint32(len(tag)))
		}
		copy(entry[8:16], name)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
	}
	return data
}

func TestSharedWADStackOrderingMapSelectionAndFingerprints(t *testing.T) {
	dir := t.TempDir()
	base, first, last, art := filepath.Join(dir, "base.wad"), filepath.Join(dir, "first.wad"), filepath.Join(dir, "last.wad"), filepath.Join(dir, "art.wad")
	baseBytes := writeCatalogTestWAD(t, base, "E1M1", "base")
	firstBytes := writeCatalogTestWAD(t, first, "E1M3", "first")
	lastBytes := writeCatalogTestWAD(t, last, "E1M4", "last")
	artBytes := writeCatalogTestWAD(t, art, "", "art")
	overlays := ResolveWADOverlayPaths("  " + first + ", ," + last + "," + art + " ")
	file, paths, err := OpenWADStack(base, overlays)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(paths, []string{base, first, last, art}) {
		t.Fatalf("stack=%v", paths)
	}
	lump, ok := file.LumpByName("TEST")
	if !ok {
		t.Fatal("missing override lump")
	}
	data, err := file.LumpData(lump)
	if err != nil || string(data) != "art" {
		t.Fatal("last file did not override artwork", err)
	}
	name, err := DefaultStartMap(file, overlays)
	if err != nil || name != "E1M4" {
		t.Fatal("non-map overlay masked the last supplied map", name, err)
	}
	wantHash := fmt.Sprintf("%x", sha1.Sum(bytes.Join([][]byte{baseBytes, firstBytes, lastBytes, artBytes}, nil)))
	if got := HashWADStackSHA1(paths); got != wantHash {
		t.Fatalf("relay fingerprint=%q want %q", got, wantHash)
	}
	if HashWADStackSHA1([]string{base, last, first, art}) == wantHash {
		t.Fatal("relay fingerprint ignores load order")
	}
	sources := BuildWADSources(paths)
	for i, b := range [][]byte{baseBytes, firstBytes, lastBytes, artBytes} {
		if sources[i].Name != filepath.Base(paths[i]) || sources[i].Hash != fmt.Sprintf("%x", blake3.Sum256(b)) {
			t.Fatal("save sources differ from ordered WAD stack")
		}
	}
	if _, _, err := OpenWADStack("", nil); err == nil {
		t.Fatal("accepted missing base path")
	}
	if _, _, err := OpenWADStack(base, []string{filepath.Join(dir, "missing.wad")}); err == nil {
		t.Fatal("ignored missing overlay")
	}
	baseOnly, err := wad.Open(base)
	if err != nil {
		t.Fatal(err)
	}
	if name, err := DefaultStartMap(baseOnly, nil); err != nil || name != "E1M1" {
		t.Fatal("base-only launch lost first map")
	}
}
