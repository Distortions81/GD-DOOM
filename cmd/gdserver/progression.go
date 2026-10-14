package main

import (
	"gddoom/internal/mapdata"
	"gddoom/internal/sessionflow"
	"gddoom/internal/wad"
)

// A rotation is an explicit operator policy, including a one-map arena. In its
// absence deathmatch exits follow the same normal/secret routes as campaign
// play. An empty next map finishes the session without rebuilding this level.
func nextServerMap(file *wad.File, current mapdata.MapName, secret, deathmatch bool, rotation []mapdata.MapName, rotationIndex int) (mapdata.MapName, int, error) {
	if len(rotation) != 0 {
		rotationIndex = (rotationIndex + 1) % len(rotation)
		return rotation[rotationIndex], rotationIndex, nil
	}
	if _, finale := sessionflow.StartFinale(current, secret); finale || current == "MAP30" {
		return "", rotationIndex, nil
	}
	next, err := mapdata.NextMapName(file, current, secret)
	if err != nil && deathmatch {
		// A custom WAD can contain just one playable map. Exhausting that
		// content is an ordinary match completion, not a crashed lobby worker.
		for _, name := range mapdata.AvailableMapNames(file) {
			if name == current {
				return "", rotationIndex, nil
			}
		}
	}
	return next, rotationIndex, err
}
