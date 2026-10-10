// Package launchcatalog shares content discovery and music labels between hosts.
package launchcatalog

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gddoom/internal/demo"
	"gddoom/internal/mapdata"
	"gddoom/internal/music"
	"gddoom/internal/runtimecfg"
	"gddoom/internal/wad"
)

type IWADChoice struct{ Path, Label string }
type KnownIWADChoice struct {
	Label string
	Paths []string
}

func BuildMusicPlayerCatalog(currentWADPath string) ([]runtimecfg.MusicPlayerWAD, func(string, string) ([]byte, error)) {
	currentWADPath = strings.TrimSpace(ResolveIWADAliasPath(currentWADPath))
	if currentWADPath == "" {
		return nil, nil
	}
	seen := make(map[string]struct{}, 8)
	choices := make([]IWADChoice, 0, 8)
	appendChoice := func(path, label string) {
		path = strings.TrimSpace(ResolveIWADAliasPath(path))
		if path == "" {
			return
		}
		if _, ok := seen[path]; ok {
			return
		}
		seen[path] = struct{}{}
		choices = append(choices, IWADChoice{Path: path, Label: label})
	}
	appendChoice(currentWADPath, strings.TrimSpace(filepath.Base(currentWADPath)))
	for _, choice := range DetectAvailableIWADChoices(filepath.Dir(currentWADPath)) {
		appendChoice(choice.Path, choice.Label)
	}
	catalog := make([]runtimecfg.MusicPlayerWAD, 0, len(choices))
	for _, choice := range choices {
		wf, err := wad.Open(choice.Path)
		if err != nil {
			continue
		}
		episodes := MusicPlayerEpisodesForWAD(wf)
		if len(episodes) == 0 {
			continue
		}
		label := strings.TrimSpace(choice.Label)
		if label == "" {
			label = filepath.Base(choice.Path)
		}
		catalog = append(catalog, runtimecfg.MusicPlayerWAD{
			Key:      choice.Path,
			Label:    label,
			Episodes: episodes,
		})
	}
	if len(catalog) == 0 {
		return nil, nil
	}
	loader := func(wadKey string, lumpName string) ([]byte, error) {
		wadKey = strings.TrimSpace(wadKey)
		if wadKey == "" {
			return nil, nil
		}
		lump := strings.ToUpper(strings.TrimSpace(lumpName))
		if lump == "" {
			return nil, nil
		}
		wf, err := wad.Open(wadKey)
		if err != nil {
			return nil, err
		}
		l, ok := wf.LumpByName(lump)
		if !ok {
			return nil, nil
		}
		data, err := wf.LumpDataView(l)
		if err != nil {
			return nil, err
		}
		if _, err := music.ParseMUS(data); err != nil {
			return nil, err
		}
		return data, nil
	}
	return catalog, loader
}

func MusicPlayerEpisodesForWAD(wf *wad.File) []runtimecfg.MusicPlayerEpisode {
	if wf == nil {
		return nil
	}
	names := mapdata.AvailableMapNames(wf)
	if len(names) == 0 {
		return nil
	}
	type group struct {
		label  string
		tracks []runtimecfg.MusicPlayerTrack
	}
	order := make([]string, 0, 8)
	groups := make(map[string]*group, 8)
	seenLumps := make(map[string]struct{}, 64)
	groupFor := func(label string) *group {
		if g, ok := groups[label]; ok {
			return g
		}
		g := &group{label: label}
		groups[label] = g
		order = append(order, label)
		return g
	}
	for _, name := range names {
		lump, ok := music.MapLumpName(string(name))
		if !ok {
			continue
		}
		if _, ok := wf.LumpByName(lump); !ok {
			continue
		}
		mapLabel := strings.ToUpper(strings.TrimSpace(string(name)))
		episodeLabel := "MAPS"
		if len(mapLabel) == 4 && mapLabel[0] == 'E' && mapLabel[2] == 'M' && mapLabel[1] >= '1' && mapLabel[1] <= '9' {
			episodeLabel = fmt.Sprintf("EPISODE %c", mapLabel[1])
		}
		g := groupFor(episodeLabel)
		g.tracks = append(g.tracks, runtimecfg.MusicPlayerTrack{
			MapName:   name,
			Label:     MapDisplayLabel(name),
			LumpName:  lump,
			MusicName: MusicTitleForLump(lump),
		})
		seenLumps[lump] = struct{}{}
	}
	const otherMusicLabel = "OTHER MUSIC"
	other := groupFor(otherMusicLabel)
	seenOther := make(map[string]struct{}, 32)
	for _, lump := range wf.Lumps {
		name := strings.ToUpper(strings.TrimSpace(lump.Name))
		if !strings.HasPrefix(name, "D_") {
			continue
		}
		if _, ok := seenLumps[name]; ok {
			continue
		}
		if _, ok := seenOther[name]; ok {
			continue
		}
		other.tracks = append(other.tracks, runtimecfg.MusicPlayerTrack{
			Label:     MusicTitleForLump(name),
			LumpName:  name,
			MusicName: MusicTitleForLump(name),
		})
		seenOther[name] = struct{}{}
	}
	episodes := make([]runtimecfg.MusicPlayerEpisode, 0, len(order))
	for _, label := range order {
		g := groups[label]
		if g == nil || len(g.tracks) == 0 {
			continue
		}
		episodes = append(episodes, runtimecfg.MusicPlayerEpisode{
			Label:  g.label,
			Tracks: g.tracks,
		})
	}
	return episodes
}

func ResolveIWADAliasPath(path string) string {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return path
	}
	if resolved, ok := ResolvePathCaseInsensitive(trimmed); ok {
		return resolved
	}
	base := strings.ToUpper(filepath.Base(trimmed))
	var aliases []string
	switch base {
	case "DOOM1.WAD":
		aliases = []string{"DOOMU.WAD", "DOOM.WAD", "DOOM2.WAD"}
	case "DOOMU.WAD", "DOOM.WAD":
		aliases = []string{"DOOMU.WAD", "DOOM.WAD"}
	default:
		return path
	}
	dir := filepath.Dir(trimmed)
	for _, candidate := range aliases {
		if alias, ok := ResolvePathCaseInsensitive(filepath.Join(dir, candidate)); ok {
			return alias
		}
	}
	return path
}

func ResolvePathCaseInsensitive(path string) (string, bool) {
	if _, err := os.Stat(path); err == nil {
		return path, true
	}
	dir := filepath.Dir(path)
	name := filepath.Base(path)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", false
	}
	for _, entry := range entries {
		if strings.EqualFold(entry.Name(), name) {
			return filepath.Join(dir, entry.Name()), true
		}
	}
	return "", false
}

func DetectAvailableSoundFonts(dir string) []string {
	out := append([]string(nil), music.EmbeddedSoundFontChoices()...)
	out = append(out, music.BrowserSoundFontChoices()...)
	entries, err := os.ReadDir(dir)
	if err == nil {
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			name := strings.TrimSpace(entry.Name())
			if !strings.HasSuffix(strings.ToLower(name), ".sf2") {
				continue
			}
			out = append(out, filepath.Join(dir, name))
		}
	}
	sort.Slice(out, func(i, j int) bool {
		pi := SoundFontDefaultRank(out[i])
		pj := SoundFontDefaultRank(out[j])
		if pi != pj {
			return pi < pj
		}
		return strings.ToUpper(out[i]) < strings.ToUpper(out[j])
	})
	if len(out) == 0 {
		return nil
	}
	dedup := out[:0]
	var prev string
	for _, path := range out {
		if prev != "" && strings.EqualFold(prev, path) {
			continue
		}
		dedup = append(dedup, path)
		prev = path
	}
	return dedup
}

func SoundFontDefaultRank(path string) int {
	base := strings.ToLower(strings.TrimSpace(filepath.Base(path)))
	switch base {
	case "sc55.sf2":
		return 0
	case "sgm-hq.sf2":
		return 1
	case "general-midi.sf2":
		return 2
	default:
		return 3
	}
}

func DetectAvailableIWADChoices(dir string) []IWADChoice {
	known := KnownIWADChoices()
	browserPaths := wad.BrowserLocalWADPaths()
	out := make([]IWADChoice, 0, len(known)+len(browserPaths))
	usedBrowser := make(map[string]struct{}, len(browserPaths))
	for _, k := range known {
		for _, candidate := range k.Paths {
			if p, ok := ResolvePathCaseInsensitive(filepath.Join(dir, candidate)); ok {
				out = append(out, IWADChoice{
					Path:  p,
					Label: k.Label,
				})
				goto nextKnownIWAD
			}
		}
		for _, path := range browserPaths {
			if !BrowserWADMatchesKnownChoice(path, k) {
				continue
			}
			out = append(out, IWADChoice{
				Path:  path,
				Label: k.Label,
			})
			usedBrowser[strings.ToUpper(strings.TrimSpace(path))] = struct{}{}
			goto nextKnownIWAD
		}
		for _, candidate := range k.Paths {
			if _, ok := wad.EmbeddedDataForPath(candidate); ok {
				out = append(out, IWADChoice{
					Path:  candidate,
					Label: k.Label,
				})
				goto nextKnownIWAD
			}
		}
	nextKnownIWAD:
	}
	for _, path := range browserPaths {
		key := strings.ToUpper(strings.TrimSpace(path))
		if _, ok := usedBrowser[key]; ok {
			continue
		}
		label := "LOCAL WAD"
		if wf, err := wad.Open(path); err == nil && strings.EqualFold(wf.Header.Identification, "PWAD") {
			label = "LOCAL PWAD"
		}
		out = append(out, IWADChoice{
			Path:  path,
			Label: label,
		})
	}
	return out
}

func BrowserWADMatchesKnownChoice(path string, choice KnownIWADChoice) bool {
	base := strings.ToUpper(filepath.Base(strings.TrimSpace(path)))
	for _, candidate := range choice.Paths {
		if strings.EqualFold(candidate, base) {
			return true
		}
	}
	return false
}

func KnownIWADChoices() []KnownIWADChoice {
	return []KnownIWADChoice{
		{Label: "The Ultimate DOOM", Paths: []string{"DOOMU.WAD", "DOOM.WAD"}},
		{Label: "DOOM II: Hell on Earth", Paths: []string{"DOOM2.WAD"}},
		{Label: "Final DOOM: TNT", Paths: []string{"TNT.WAD"}},
		{Label: "Final DOOM: Plutonia", Paths: []string{"PLUTONIA.WAD"}},
		{Label: "DOOM Shareware", Paths: []string{"DOOM1.WAD"}},
		{Label: "Freedoom Phase 1", Paths: []string{"freedoom1.wad"}},
		{Label: "Freedoom Phase 2", Paths: []string{"freedoom2.wad"}},
		{Label: "FreeDM Deathmatch", Paths: []string{"freedm.wad"}},
	}
}

func KnownIWADChoiceForPath(path string) (IWADChoice, bool) {
	base := strings.ToUpper(filepath.Base(strings.TrimSpace(path)))
	for _, choice := range KnownIWADChoices() {
		for _, candidate := range choice.Paths {
			if strings.EqualFold(candidate, base) {
				return IWADChoice{Path: candidate, Label: choice.Label}, true
			}
		}
	}
	return IWADChoice{}, false
}

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

func MapDisplayLabel(name mapdata.MapName) string {
	mapID := strings.ToUpper(strings.TrimSpace(string(name)))
	title := strings.TrimSpace(doomMapTitles[mapID])
	if title == "" {
		return mapID
	}
	return fmt.Sprintf("%s - %s", mapID, title)
}

func MusicTitleForLump(lumpName string) string {
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

func MapMusicInfo(mapName string) (levelLabel string, musicName string) {
	name := mapdata.MapName(strings.ToUpper(strings.TrimSpace(mapName)))
	levelLabel = MapDisplayLabel(name)
	lump, ok := music.MapLumpName(string(name))
	if !ok {
		return levelLabel, ""
	}
	return levelLabel, MusicTitleForLump(lump)
}

func ResolveDemoStartMap(wf *wad.File, script *demo.Script, fallback mapdata.MapName) (mapdata.MapName, error) {
	if wf == nil || script == nil {
		return "", fmt.Errorf("missing demo")
	}
	candidates := make([]mapdata.MapName, 0, 2)
	if script.Header.Map > 0 {
		candidates = append(candidates, mapdata.MapName(fmt.Sprintf("MAP%02d", script.Header.Map)))
	}
	if script.Header.Episode > 0 && script.Header.Map > 0 && script.Header.Map <= 9 {
		candidates = append(candidates, mapdata.MapName(fmt.Sprintf("E%dM%d", script.Header.Episode, script.Header.Map)))
	}
	for _, candidate := range candidates {
		if _, err := mapdata.LoadMap(wf, candidate); err == nil {
			return candidate, nil
		}
	}
	if fallback != "" {
		return "", fmt.Errorf("demo map episode=%d map=%d not present in wad (requested map %s ignored)", script.Header.Episode, script.Header.Map, fallback)
	}
	return "", fmt.Errorf("demo map episode=%d map=%d not present in wad", script.Header.Episode, script.Header.Map)
}

func LoadBuiltInDemos(wf *wad.File) []*demo.Script {
	if wf == nil {
		return nil
	}
	out := make([]*demo.Script, 0, 4)
	for _, name := range []string{"DEMO1", "DEMO2", "DEMO3", "DEMO4"} {
		lump, ok := wf.LumpByName(name)
		if !ok {
			continue
		}
		data, err := wf.LumpDataView(lump)
		if err != nil {
			continue
		}
		demo, err := demo.Parse(data)
		if err != nil {
			continue
		}
		demo.Path = name
		out = append(out, demo)
	}
	return out
}
