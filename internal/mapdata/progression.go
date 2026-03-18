package mapdata

import (
	"fmt"
	"strconv"
	"strings"

	"gddoom/internal/wad"
)

func AvailableMapNames(f *wad.File) []MapName {
	names := make([]MapName, 0, 32)
	for i, l := range f.Lumps {
		if isMapMarker(l.Name) && hasRequiredLumpsAt(f, i) {
			names = append(names, MapName(l.Name))
		}
	}
	return names
}

func NextMapName(f *wad.File, current MapName, secret bool) (MapName, error) {
	cur := strings.ToUpper(strings.TrimSpace(string(current)))
	if !isMapMarker(cur) {
		return "", fmt.Errorf("invalid current map name %q", current)
	}
	if !mapExists(f, cur) {
		return "", fmt.Errorf("current map %s is not present in wad", cur)
	}

	if secret {
		if target, ok := secretExitTarget(cur); ok && mapExists(f, target) {
			return MapName(target), nil
		}
	}
	if target, ok := normalExitTarget(cur); ok && mapExists(f, target) {
		return MapName(target), nil
	}
	if target, ok := nextMapInLumpOrder(f, cur); ok {
		return MapName(target), nil
	}
	return "", fmt.Errorf("no next map after %s", cur)
}

func mapExists(f *wad.File, name string) bool {
	for i, l := range f.Lumps {
		if l.Name == name && hasRequiredLumpsAt(f, i) {
			return true
		}
	}
	return false
}

func nextMapInLumpOrder(f *wad.File, current string) (string, bool) {
	names := AvailableMapNames(f)
	for i, name := range names {
		if string(name) != current {
			continue
		}
		if i+1 < len(names) {
			return string(names[i+1]), true
		}
		return "", false
	}
	return "", false
}

func secretExitTarget(current string) (string, bool) {
	switch current {
	case "E1M3":
		return "E1M9", true
	case "E2M5":
		return "E2M9", true
	case "E3M6":
		return "E3M9", true
	case "E4M2":
		return "E4M9", true
	case "MAP15":
		return "MAP31", true
	case "MAP31":
		return "MAP32", true
	}
	return "", false
}

func normalExitTarget(current string) (string, bool) {
	switch current {
	case "E1M9":
		return "E1M4", true
	case "E2M9":
		return "E2M6", true
	case "E3M9":
		return "E3M7", true
	case "E4M9":
		return "E4M3", true
	case "MAP31", "MAP32":
		return "MAP16", true
	}

	if len(current) == 4 && current[0] == 'E' && current[2] == 'M' {
		ep := current[1]
		mn := int(current[3] - '0')
		if ep < '1' || ep > '9' || mn < 1 || mn > 8 {
			return "", false
		}
		return fmt.Sprintf("E%cM%d", ep, mn+1), true
	}

	if strings.HasPrefix(current, "MAP") && len(current) == 5 {
		n, err := strconv.Atoi(current[3:])
		if err != nil || n < 1 || n >= 99 {
			return "", false
		}
		return fmt.Sprintf("MAP%02d", n+1), true
	}
	return "", false
}
