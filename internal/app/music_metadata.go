package app

import (
	"gddoom/internal/launchcatalog"
	"gddoom/internal/mapdata"
)

func mapDisplayLabel(name mapdata.MapName) string { return launchcatalog.MapDisplayLabel(name) }
func musicTitleForLump(name string) string        { return launchcatalog.MusicTitleForLump(name) }
func mapMusicInfo(name string) (string, string)   { return launchcatalog.MapMusicInfo(name) }
