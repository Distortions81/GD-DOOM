//go:build raylib && cgo && !js

package main

import "gddoom/internal/render/raymesh"

// Input and drawing must reserve exactly the same space. Overlay/hidden HUDs
// use the whole scene; only the bottom status bar reduces its viewport.
func nativeViewHeight(width, height, hudMode int) int {
	if hudMode == 0 {
		return max(1, height-raymesh.HUDHeight(width, height))
	}
	return max(1, height)
}
