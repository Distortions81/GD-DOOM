package launchcatalog

import (
	"crypto/sha1"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"gddoom/internal/mapdata"
	"gddoom/internal/runtimecfg"
	"gddoom/internal/wad"
	"github.com/zeebo/blake3"
)

func ResolveWADOverlayPaths(value string) []string {
	var paths []string
	for _, part := range strings.Split(value, ",") {
		if path := strings.TrimSpace(part); path != "" {
			paths = append(paths, ResolveIWADAliasPath(path))
		}
	}
	return paths
}

// OpenWADStack retains input order; the WAD reader resolves duplicate lumps
// from the last file that supplies them, including complete replacement maps.
func OpenWADStack(basePath string, overlays []string) (*wad.File, []string, error) {
	basePath = strings.TrimSpace(ResolveIWADAliasPath(basePath))
	if basePath == "" {
		return nil, nil, fmt.Errorf("missing base wad path")
	}
	paths := []string{basePath}
	for _, overlay := range overlays {
		if path := strings.TrimSpace(ResolveIWADAliasPath(overlay)); path != "" {
			paths = append(paths, path)
		}
	}
	file, err := wad.OpenFiles(paths...)
	if err != nil {
		return nil, nil, err
	}
	return file, paths, nil
}

func DefaultStartMap(file *wad.File, overlays []string) (mapdata.MapName, error) {
	for i := len(overlays) - 1; i >= 0; i-- {
		overlay, err := wad.Open(overlays[i])
		if err != nil {
			return "", err
		}
		if name, err := mapdata.FirstMapName(overlay); err == nil {
			return name, nil
		}
	}
	return mapdata.FirstMapName(file)
}

// HashWADStackSHA1 is the existing relay fingerprint: the concatenated WAD
// bytes, in load order. Source metadata uses separate BLAKE3 hashes for saves.
func HashWADStackSHA1(paths []string) string {
	if len(paths) == 0 {
		return ""
	}
	h := sha1.New()
	for _, path := range paths {
		if data, ok := wad.EmbeddedDataForPath(path); ok {
			_, _ = h.Write(data)
			continue
		}
		f, err := os.Open(path)
		if err != nil {
			return ""
		}
		_, copyErr := io.Copy(h, f)
		closeErr := f.Close()
		if copyErr != nil || closeErr != nil {
			return ""
		}
	}
	return fmt.Sprintf("%x", h.Sum(nil))
}

func HashWADPathBlake3(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	data, ok := wad.EmbeddedDataForPath(path)
	if !ok {
		var err error
		data, err = os.ReadFile(path)
		if err != nil {
			return ""
		}
	}
	return fmt.Sprintf("%x", blake3.Sum256(data))
}

func BuildWADSources(paths []string) []runtimecfg.WADSource {
	var sources []runtimecfg.WADSource
	for _, path := range paths {
		if path = strings.TrimSpace(path); path != "" {
			sources = append(sources, runtimecfg.WADSource{Name: filepath.Base(path), Hash: HashWADPathBlake3(path)})
		}
	}
	return sources
}
