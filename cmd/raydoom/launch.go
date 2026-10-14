//go:build raylib && cgo && !js

package main

import (
	"fmt"
	"gddoom/internal/mapdata"
	"strings"
)

type nativeLaunch struct {
	mapName            mapdata.MapName
	showMenu, frontend bool
}

// Explicit maps retain the experiment's direct-launch workflow. Ordinary
// launches enter the frontend and select the WAD's first campaign level.
func chooseNativeLaunch(requested string, showMenu, menuSpecified bool, maps []mapdata.MapName) (nativeLaunch, error) {
	if len(maps) == 0 {
		return nativeLaunch{}, fmt.Errorf("WAD has no playable maps")
	}
	requested = strings.ToUpper(strings.TrimSpace(requested))
	name := mapdata.MapName(requested)
	if requested == "" {
		name = maps[0]
		for _, preferred := range []mapdata.MapName{"E1M1", "MAP01"} {
			found := false
			for _, available := range maps {
				if available == preferred {
					name = available
					found = true
					break
				}
			}
			if found {
				break
			}
		}
	} else if !menuSpecified {
		showMenu = false
	}
	frontend := showMenu && requested == ""
	if frontend && !menuSpecified {
		showMenu = false
	}
	return nativeLaunch{mapName: name, showMenu: showMenu, frontend: frontend}, nil
}
