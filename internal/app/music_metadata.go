package app

import (
	"fmt"
	"strings"

	"gddoom/internal/mapdata"
)

var doomMapTitles = map[string]string{
	"E1M1":  "Hangar",
	"E1M2":  "Nuclear Plant",
	"E1M3":  "Toxin Refinery",
	"E1M4":  "Command Control",
	"E1M5":  "Phobos Lab",
	"E1M6":  "Central Processing",
	"E1M7":  "Computer Station",
	"E1M8":  "Phobos Anomaly",
	"E1M9":  "Military Base",
	"E2M1":  "Deimos Anomaly",
	"E2M2":  "Containment Area",
	"E2M3":  "Refinery",
	"E2M4":  "Deimos Lab",
	"E2M5":  "Command Center",
	"E2M6":  "Halls of the Damned",
	"E2M7":  "Spawning Vats",
	"E2M8":  "Tower of Babel",
	"E2M9":  "Fortress of Mystery",
	"E3M1":  "Hell Keep",
	"E3M2":  "Slough of Despair",
	"E3M3":  "Pandemonium",
	"E3M4":  "House of Pain",
	"E3M5":  "Unholy Cathedral",
	"E3M6":  "Mt. Erebus",
	"E3M7":  "Limbo",
	"E3M8":  "Dis",
	"E3M9":  "Warrens",
	"E4M1":  "Hell Beneath",
	"E4M2":  "Perfect Hatred",
	"E4M3":  "Sever The Wicked",
	"E4M4":  "Unruly Evil",
	"E4M5":  "They Will Repent",
	"E4M6":  "Against Thee Wickedly",
	"E4M7":  "And Hell Followed",
	"E4M8":  "Unto The Cruel",
	"E4M9":  "Fear",
	"MAP01": "Entryway",
	"MAP02": "Underhalls",
	"MAP03": "The Gantlet",
	"MAP04": "The Focus",
	"MAP05": "The Waste Tunnels",
	"MAP06": "The Crusher",
	"MAP07": "Dead Simple",
	"MAP08": "Tricks and Traps",
	"MAP09": "The Pit",
	"MAP10": "Refueling Base",
	"MAP11": "'O' of Destruction!",
	"MAP12": "The Factory",
	"MAP13": "Downtown",
	"MAP14": "The Inmost Dens",
	"MAP15": "Industrial Zone",
	"MAP16": "Suburbs",
	"MAP17": "Tenements",
	"MAP18": "The Courtyard",
	"MAP19": "The Citadel",
	"MAP20": "Gotcha!",
	"MAP21": "Nirvana",
	"MAP22": "The Catacombs",
	"MAP23": "Barrels o' Fun",
	"MAP24": "The Chasm",
	"MAP25": "Bloodfalls",
	"MAP26": "The Abandoned Mines",
	"MAP27": "Monster Condo",
	"MAP28": "The Spirit World",
	"MAP29": "The Living End",
	"MAP30": "Icon of Sin",
	"MAP31": "Wolfenstein",
	"MAP32": "Grosse",
}

var doomMusicTitles = map[string]string{
	"D_E1M1":    "At Doom's Gate",
	"D_E1M2":    "The Imp's Song",
	"D_E1M3":    "Dark Halls",
	"D_E1M4":    "Kitchen Ace (And Taking Names)",
	"D_E1M5":    "Suspense",
	"D_E1M6":    "On the Hunt",
	"D_E1M7":    "Demons on the Prey",
	"D_E1M8":    "Sign of Evil",
	"D_E1M9":    "Hiding the Secrets",
	"D_E2M1":    "I Sawed the Demons",
	"D_E2M2":    "The Demons from Adrian's Pen",
	"D_E2M3":    "Intermission from Doom",
	"D_E2M4":    "They're Going to Get You",
	"D_E2M5":    "Sinister",
	"D_E2M6":    "Waltz of the Demons",
	"D_E2M7":    "Nobody Told Me About id",
	"D_E2M8":    "Donna to the Rescue",
	"D_E2M9":    "Untitled",
	"D_E3M1":    "Facing the Spider",
	"D_E3M2":    "Deep into the Code",
	"D_E3M3":    "Adrian's Asleep",
	"D_E3M4":    "Waiting for Romero to Play",
	"D_E3M5":    "Message for the Arch-Vile",
	"D_E3M6":    "Between Levels",
	"D_E3M7":    "Opening to Hell",
	"D_E3M8":    "Evil, Incarnate",
	"D_E3M9":    "Untitled",
	"D_INTER":   "Intermission from Doom",
	"D_INTRO":   "Intro",
	"D_BUNNY":   "Bunny Scroll",
	"D_VICTOR":  "Victory",
	"D_INTROA":  "Title Screen",
	"D_RUNNIN":  "Running from Evil",
	"D_STALKS":  "The Healer Stalks",
	"D_COUNTD":  "Countdown to Death",
	"D_BETWEE":  "Between Levels",
	"D_DOOM":    "Doom",
	"D_THE_DA":  "In the Dark",
	"D_SHAWN":   "Shawn's Got the Shotgun",
	"D_DDTBLU":  "The Dave D. Taylor Blues",
	"D_IN_CIT":  "Into Sandy's City",
	"D_DEAD":    "The Demon's Dead",
	"D_STLKS2":  "Adrian's Asleep",
	"D_THE_DA2": "Message for the Arch-Vile",
	"D_DOOM2":   "Bye Bye American Pie",
	"D_DDTBL2":  "Untitled",
	"D_RUNNI2":  "Waiting for Romero to Play",
	"D_DEAD2":   "Opening to Hell",
	"D_STLKS3":  "Evil, Incarnate",
	"D_ROMERO":  "The Romero One Mind Any Weapon",
	"D_SHAWN2":  "Shawn's Mystery",
	"D_MESSAG":  "Message for the Arch-Vile",
	"D_COUNT2":  "Countdown to Death",
	"D_DDTBL3":  "The Dave D. Taylor Blues",
	"D_AMPIE":   "Bye Bye American Pie",
	"D_THEDA3":  "In the Dark",
	"D_ADRIAN":  "Adrian's Asleep",
	"D_MESSG2":  "Message for the Arch-Vile",
	"D_ROMER2":  "They're Going to Get You",
	"D_TENSE":   "Getting Too Tense",
	"D_SHAWN3":  "Shawn's Got the Shotgun",
	"D_OPENIN":  "Opening to Hell",
	"D_EVIL":    "Evil, Incarnate",
	"D_ULTIMA":  "Ultimate Doom",
	"D_READ_M":  "Read This",
	"D_DM2TTL":  "Doom II Title",
	"D_DM2INT":  "Doom II Intermission",
}

func mapDisplayLabel(name mapdata.MapName) string {
	mapID := strings.ToUpper(strings.TrimSpace(string(name)))
	title := strings.TrimSpace(doomMapTitles[mapID])
	if title == "" {
		return mapID
	}
	return fmt.Sprintf("%s - %s", mapID, title)
}

func musicTitleForLump(lumpName string) string {
	lump := strings.ToUpper(strings.TrimSpace(lumpName))
	if title := strings.TrimSpace(doomMusicTitles[lump]); title != "" {
		return title
	}
	lump = strings.TrimPrefix(lump, "D_")
	lump = strings.ReplaceAll(lump, "_", " ")
	lump = strings.TrimSpace(lump)
	if lump == "" {
		return strings.ToUpper(strings.TrimSpace(lumpName))
	}
	return lump
}

func mapMusicInfo(mapName string) (levelLabel string, musicName string) {
	name := mapdata.MapName(strings.ToUpper(strings.TrimSpace(mapName)))
	levelLabel = mapDisplayLabel(name)
	lump, ok := mapMusicLumpName(name)
	if !ok {
		return levelLabel, ""
	}
	return levelLabel, musicTitleForLump(lump)
}
