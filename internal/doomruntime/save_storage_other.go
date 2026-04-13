//go:build !js || !wasm

package doomruntime

import (
	"fmt"
	"os"
	"sort"
	"time"
)

func readSavedSlotData(slot int) ([]byte, error) {
	return os.ReadFile(saveGamePath(slot))
}

func readSavedSlotDataWithModTime(slot int) ([]byte, time.Time, error) {
	path := saveGamePath(slot)
	stat, err := os.Stat(path)
	if err != nil {
		return nil, time.Time{}, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, time.Time{}, err
	}
	return data, stat.ModTime(), nil
}

func writeSavedSlotData(slot int, data []byte) error {
	if err := os.MkdirAll(saveGameDirName, 0o755); err != nil {
		return fmt.Errorf("create save dir: %w", err)
	}
	return os.WriteFile(saveGamePath(slot), data, 0o644)
}

func listSavedSlots() ([]int, error) {
	entries, err := os.ReadDir(saveGameDirName)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	seen := map[int]struct{}{}
	out := make([]int, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		slot, ok := saveGameSlotFromFileName(entry.Name())
		if !ok {
			continue
		}
		if _, exists := seen[slot]; exists {
			continue
		}
		seen[slot] = struct{}{}
		out = append(out, slot)
	}
	sort.Ints(out)
	return out, nil
}
