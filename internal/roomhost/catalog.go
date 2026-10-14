// Package roomhost supervises isolated authoritative game server processes.
package roomhost

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"gddoom/internal/lobby"
	"gddoom/internal/mapdata"
	"gddoom/internal/wad"
)

// ContentPack contains operator-owned paths. Only Pack is exposed to clients.
type ContentPack struct {
	Pack  lobby.Pack
	Paths []string
}

type Catalog struct {
	Packs          []CatalogPack            `json:"packs"`
	Redistribution []RedistributionApproval `json:"redistribution,omitempty"`
}
type CatalogPack struct {
	ID   string   `json:"id"`
	Name string   `json:"name"`
	WADs []string `json:"wads"`
}

// LoadCatalog resolves relative paths against the configuration file, hashes
// resources in load order, and publishes only maps the runtime can load.
func LoadCatalog(filename string) ([]ContentPack, error) {
	packs, _, err := LoadCatalogWithPolicy(filename)
	return packs, err
}

// LoadCatalogWithPolicy also returns explicit operator redistribution approvals.
// The policy is keyed by exact file hashes and is never inferred from a name.
func LoadCatalogWithPolicy(filename string) ([]ContentPack, []RedistributionApproval, error) {
	catalog, base, err := readCatalog(filename)
	if err != nil {
		return nil, nil, err
	}
	packs, err := inspectCatalog(catalog, base)
	return packs, catalog.Redistribution, err
}

func readCatalog(filename string) (Catalog, string, error) {
	f, err := os.Open(filename)
	if err != nil {
		return Catalog{}, "", err
	}
	defer f.Close()
	decoder := json.NewDecoder(io.LimitReader(f, 1<<20))
	decoder.DisallowUnknownFields()
	var catalog Catalog
	if err := decoder.Decode(&catalog); err != nil {
		return Catalog{}, "", fmt.Errorf("catalog: %w", err)
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return Catalog{}, "", fmt.Errorf("catalog must contain one JSON object")
	}
	if len(catalog.Packs) == 0 || len(catalog.Packs) > lobby.MaxPacks {
		return Catalog{}, "", fmt.Errorf("catalog requires 1..%d content packs", lobby.MaxPacks)
	}
	base, err := filepath.Abs(filepath.Dir(filename))
	if err != nil {
		return Catalog{}, "", err
	}
	return catalog, base, nil
}

func inspectCatalog(catalog Catalog, base string) ([]ContentPack, error) {
	seen := make(map[string]bool)
	packs := make([]ContentPack, 0, len(catalog.Packs))
	for _, entry := range catalog.Packs {
		if seen[entry.ID] {
			return nil, fmt.Errorf("duplicate content pack %q", entry.ID)
		}
		seen[entry.ID] = true
		if len(entry.WADs) == 0 || len(entry.WADs) > 64 {
			return nil, fmt.Errorf("pack %q requires 1..64 ordered WADs", entry.ID)
		}
		pack := ContentPack{Pack: lobby.Pack{ID: entry.ID, Name: entry.Name}}
		var files []*wad.File
		for _, filename := range entry.WADs {
			if filename == "" {
				return nil, fmt.Errorf("empty WAD path in pack %q", entry.ID)
			}
			if !filepath.IsAbs(filename) {
				filename = filepath.Join(base, filename)
			}
			filename = filepath.Clean(filename)
			data, err := os.ReadFile(filename)
			if err != nil {
				return nil, fmt.Errorf("pack %q: %w", entry.ID, err)
			}
			file, err := wad.OpenData(filename, data)
			if err != nil {
				return nil, fmt.Errorf("pack %q: %w", entry.ID, err)
			}
			digest := sha256.Sum256(data)
			pack.Paths = append(pack.Paths, filename)
			pack.Pack.WADHashes = append(pack.Pack.WADHashes, hex.EncodeToString(digest[:]))
			files = append(files, file)
		}
		merged := wad.Merge(files...)
		names, err := uniqueMapNames(merged)
		if err != nil {
			return nil, err
		}
		for _, name := range names {
			if _, err := mapdata.LoadMap(merged, name); err == nil {
				pack.Pack.Maps = append(pack.Pack.Maps, string(name))
			}
		}
		if err := lobby.ValidatePack(pack.Pack); err != nil {
			return nil, fmt.Errorf("pack %q: %w", entry.ID, err)
		}
		packs = append(packs, pack)
	}
	return packs, nil
}

func uniqueMapNames(file *wad.File) ([]mapdata.MapName, error) {
	seen := make(map[mapdata.MapName]bool)
	var names []mapdata.MapName
	for _, name := range mapdata.AvailableMapNames(file) {
		if !seen[name] {
			seen[name] = true
			names = append(names, name)
		}
		if len(names) > lobby.MaxMaps {
			return nil, fmt.Errorf("content stack contains too many maps")
		}
	}
	return names, nil
}
