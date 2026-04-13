package doomruntime

import (
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gddoom/internal/doomrand"
	"gddoom/internal/mapdata"
	"gddoom/internal/render/mapview"
	"gddoom/internal/runtimecfg"
	"gddoom/internal/runtimehost"

	"github.com/hajimehoshi/ebiten/v2"
)

const (
	saveGameVersion     = 19
	saveGamePrefix      = "dsg"
	saveGameQuickPrefix = "quicksave"
	keyframeVersion     = 6
	saveGameDirName     = "saves"
)

var (
	errSaveGameUnavailable = errors.New("save game unavailable")
	errNoSavedGame         = errors.New("no saved game")
	errSaveGameWebOnly     = errors.New("not available")
	errBadSaveMagic        = errors.New("invalid save format")
	errBadSaveChecksum     = errors.New("invalid save checksum")
)

var saveGameMagic = []byte("GDDOOMSAVE\x00")
var keyframeMagic = []byte("GDDOOMKEY\x00")

const saveMandatoryApplyKeyframeFlag byte = 1
const (
	saveThumbnailMaxWidth  = 320
	saveThumbnailMaxHeight = 320
	saveThumbnailFallbackW = 320
	saveThumbnailFallbackH = 200
)

type saveKeyframeSink interface {
	BroadcastKeyframe(tic uint32, blob []byte) error
}

type saveMandatoryKeyframeSink interface {
	BroadcastKeyframeWithFlags(tic uint32, blob []byte, flags byte) error
}

type saveFile struct {
	Version     int
	Description string
	Current     mapdata.MapName
	WADSources  []saveWADSource
	RNG         saveRNGState
	Game        gameSaveState
}

type saveRNGState struct {
	MenuIndex int
	PlayIndex int
}

type saveWADSource struct {
	Name string
	Hash string
}

type saveThumbnailCacheEntry struct {
	ModTime time.Time
	Image   *ebiten.Image
}

type saveSlotInfo struct {
	Slot        int
	Description string
	Current     mapdata.MapName
	WADSources  []saveWADSource
	Health      int
	WorldTic    int
	ModTime     time.Time
	Present     bool
}

type gameSaveState struct {
	Session              sessionSaveState
	Player               playerSaveState
	View                 mapview.ViewState
	Mode                 int
	RotateView           bool
	ParityReveal         int
	ParityIDDT           int
	ShowGrid             bool
	ShowLegend           bool
	PaletteLUTEnabled    bool
	GammaLevel           int
	CRTEnabled           bool
	UseFlash             int
	UseText              string
	HUDMessagesEnabled   bool
	PrevPX               int64
	PrevPY               int64
	PrevAngle            uint32
	PlayerViewZ          int64
	ThingCollected       []bool
	ThingDropped         []bool
	ThingThinkerOrder    []int64
	ThingX               []int64
	ThingY               []int64
	ThingMomX            []int64
	ThingMomY            []int64
	ThingMomZ            []int64
	ThingAngleState      []uint32
	ThingZState          []int64
	ThingFloorState      []int64
	ThingCeilState       []int64
	ThingSupportValid    []bool
	ThingHP              []int
	ThingAggro           []bool
	ThingTargetPlayer    []bool
	ThingTargetIdx       []int
	ThingThreshold       []int
	ThingCooldown        []int
	ThingMoveDir         []uint8
	ThingMoveCount       []int
	ThingJustAtk         []bool
	ThingJustHit         []bool
	ThingSkullFly        []bool
	ThingResumeChaseNow  []bool
	ThingReactionTics    []int
	ThingWakeTics        []int
	ThingLastLook        []int
	ThingDead            []bool
	ThingAmbush          []bool
	ThingInFloat         []bool
	ThingGibbed          []bool
	ThingGibTick         []int
	ThingXDeath          []bool
	ThingDeathTics       []int
	ThingAttackTics      []int
	ThingAttackPhase     []int
	ThingAttackFireTics  []int
	ThingPainTics        []int
	ThingThinkWait       []int
	ThingDoomState       []int
	ThingState           []uint8
	ThingStateTics       []int
	ThingStatePhase      []int
	BossSpawnCubes       []bossSpawnCubeSaveState
	BossSpawnFires       []bossSpawnFireSaveState
	BossBrainTargetOrder int
	BossBrainEasyToggle  bool
	Projectiles          []projectileSaveState
	ProjectileImpacts    []projectileImpactSaveState
	HitscanPuffs         []hitscanPuffSaveState
	CheatLevel           int
	Invulnerable         bool
	NoClip               bool
	Inventory            playerInventorySaveState
	AlwaysRun            bool
	AutoWeaponSwitch     bool
	WeaponRefire         bool
	WeaponAttackDown     bool
	WeaponState          int
	WeaponStateTics      int
	WeaponFlashState     int
	WeaponFlashTics      int
	WeaponPSpriteY       int
	Stats                playerStats
	WorldTic             int
	PlayerBlockOrder     int64
	NextThinkerOrder     int64
	NextBlockmapOrder    int64
	SecretFound          []bool
	SecretsFound         int
	SecretsTotal         int
	SectorSoundTarget    []bool
	IsDead               bool
	PlayerMobjHealth     int
	DamageFlashTic       int
	BonusFlashTic        int
	SectorLightFx        []sectorLightEffectSaveState
	Things               []mapdata.Thing
	Sidedefs             []mapdata.Sidedef
	Sectors              []mapdata.Sector
	SectorFloor          []int64
	SectorCeil           []int64
	LineSpecial          []uint16
	Doors                map[int]doorThinkerSaveState
	Floors               map[int]floorThinkerSaveState
	Plats                map[int]platThinkerSaveState
	Ceilings             map[int]ceilingThinkerSaveState
	DelayedSwitchReverts []delayedSwitchTextureSaveState
}

type sessionSaveState struct {
	PlayerSlot       int
	SkillLevel       int
	GameMode         string
	ShowNoSkillItems bool
	ShowAllItems     bool
	FastMonsters     bool
	RespawnMonsters  bool
	NoMonsters       bool
}

type playerSaveState struct {
	X               int64
	Y               int64
	Z               int64
	FloorZ          int64
	CeilZ           int64
	Subsector       int
	Sector          int
	Angle           uint32
	MomX            int64
	MomY            int64
	MomZ            int64
	ReactionTime    int
	ViewHeight      int64
	DeltaViewHeight int64
}

type playerInventorySaveState struct {
	BlueKey       bool
	RedKey        bool
	YellowKey     bool
	Backpack      bool
	Strength      bool
	StrengthCount int
	InvulnTics    int
	InvisTics     int
	RadSuitTics   int
	AllMap        bool
	LightAmpTics  int
	ReadyWeapon   int
	PendingWeapon int
	Weapons       map[int16]bool
}

type doorThinkerSaveState struct {
	Order        int64
	Sector       int
	Type         int
	Direction    int
	TopHeight    int64
	TopWait      int
	TopCountdown int
	Speed        int64
}

type floorThinkerSaveState struct {
	Order         int64
	Sector        int
	Direction     int
	Speed         int64
	DestHeight    int64
	Finish        uint8
	FinishFlat    string
	FinishSpecial int16
}

type platThinkerSaveState struct {
	Order         int64
	Sector        int
	Type          uint8
	Status        uint8
	OldStatus     uint8
	Speed         int64
	Low           int64
	High          int64
	Wait          int
	Count         int
	FinishFlat    string
	FinishSpecial int16
}

type ceilingThinkerSaveState struct {
	Order        int64
	Sector       int
	Action       mapdata.CeilingAction
	Direction    int
	OldDirection int
	Speed        int64
	TopHeight    int64
	BottomHeight int64
	Crush        bool
}

type bossSpawnCubeSaveState struct {
	X         int64
	Y         int64
	Z         int64
	VX        int64
	VY        int64
	VZ        int64
	TargetIdx int
	StateTics int
	StateStep int
	Reaction  int
}

type bossSpawnFireSaveState struct {
	X    int64
	Y    int64
	Z    int64
	Tics int
}

type projectileSaveState struct {
	X            int64
	Y            int64
	Z            int64
	VX           int64
	VY           int64
	VZ           int64
	FloorZ       int64
	CeilZ        int64
	Radius       int64
	Height       int64
	TTL          int
	SourceX      int64
	SourceY      int64
	SourceThing  int
	SourceType   int16
	SourcePlayer bool
	TracerPlayer bool
	Angle        uint32
	Kind         int
}

type projectileImpactSaveState struct {
	X            int64
	Y            int64
	Z            int64
	FloorZ       int64
	CeilZ        int64
	Kind         int
	SourceThing  int
	SourceType   int16
	SourcePlayer bool
	LastLook     int
	Tics         int
	TotalTics    int
	Angle        uint32
	SprayDone    bool
}

type hitscanPuffSaveState struct {
	X        int64
	Y        int64
	Z        int64
	MomZ     int64
	Tics     int
	State    int
	TotalTic int
	Kind     uint8
}

type sectorLightEffectSaveState struct {
	Kind       uint8
	MinLight   int16
	MaxLight   int16
	Count      int
	MinTime    int
	MaxTime    int
	DarkTime   int
	BrightTime int
	Direction  int
}

type delayedSwitchTextureSaveState struct {
	Line    int
	Sidedef int
	Top     string
	Bottom  string
	Mid     string
	Tics    int
}

func saveGamePath(slot int) string {
	name := saveGameBaseName(slot) + ".dsg"
	return filepath.Join(saveGameDirName, name)
}

func saveGameThumbnailPath(slot int) string {
	name := saveGameBaseName(slot) + ".png"
	return filepath.Join(saveGameDirName, name)
}

func saveGameBaseName(slot int) string {
	if slot <= 0 {
		return saveGameQuickPrefix
	}
	return fmt.Sprintf("%s%d", saveGamePrefix, slot)
}

func (sg *sessionGame) readSaveGameDescription(slot int) (string, bool) {
	if isWASMBuild() || sg == nil {
		return "", false
	}
	data, err := os.ReadFile(saveGamePath(slot))
	if err != nil {
		return "", false
	}
	file, err := decodeSnapshot(data, saveGameMagic)
	if err != nil {
		return "", false
	}
	desc := strings.TrimSpace(file.Description)
	if desc == "" {
		return "", false
	}
	return desc, true
}

func (sg *sessionGame) saveSlotDescriptions(slotCount int) []string {
	if slotCount <= 0 {
		return nil
	}
	out := make([]string, slotCount)
	for i := 0; i < slotCount; i++ {
		slot := i
		if info, ok := sg.readSaveSlotInfo(slot); ok {
			out[i] = formatSaveSummaryLabel(info)
		} else {
			out[i] = "EMPTY SLOT"
		}
	}
	return out
}

func (sg *sessionGame) saveSlotThumbnailImage(slot int) (*ebiten.Image, bool) {
	if isWASMBuild() || sg == nil {
		return nil, false
	}
	if sg.saveThumbnailCache == nil {
		sg.saveThumbnailCache = make(map[int]saveThumbnailCacheEntry)
	}
	path := saveGameThumbnailPath(slot)
	stat, err := os.Stat(path)
	if err != nil {
		delete(sg.saveThumbnailCache, slot)
		return nil, false
	}
	if entry, ok := sg.saveThumbnailCache[slot]; ok && entry.Image != nil && entry.ModTime.Equal(stat.ModTime()) {
		return entry.Image, true
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, false
	}
	defer f.Close()
	img, err := png.Decode(f)
	if err != nil {
		return nil, false
	}
	eimg := ebiten.NewImageFromImage(img)
	sg.saveThumbnailCache[slot] = saveThumbnailCacheEntry{
		ModTime: stat.ModTime(),
		Image:   eimg,
	}
	return eimg, true
}

func (sg *sessionGame) readSaveSlotInfo(slot int) (saveSlotInfo, bool) {
	if isWASMBuild() || sg == nil {
		return saveSlotInfo{}, false
	}
	path := saveGamePath(slot)
	stat, err := os.Stat(path)
	if err != nil {
		return saveSlotInfo{}, false
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return saveSlotInfo{}, false
	}
	file, err := decodeSnapshot(data, saveGameMagic)
	if err != nil {
		return saveSlotInfo{}, false
	}
	return saveSlotInfo{
		Slot:        slot,
		Description: strings.TrimSpace(file.Description),
		Current:     file.Current,
		WADSources:  append([]saveWADSource(nil), file.WADSources...),
		Health:      file.Game.Stats.Health,
		WorldTic:    file.Game.WorldTic,
		ModTime:     stat.ModTime(),
		Present:     true,
	}, true
}

func (sg *sessionGame) saveSlotDetailLines(slot int) []string {
	info, ok := sg.readSaveSlotInfo(slot)
	if !ok {
		return []string{"EMPTY SLOT"}
	}
	return []string{"WADS: " + formatSaveWADNames(info.WADSources)}
}

func (sg *sessionGame) renderSaveThumbnailSourceImage(g *game) *ebiten.Image {
	if sg == nil || g == nil {
		return nil
	}
	if !sg.opts.SourcePortMode {
		vw := max(g.viewW, 1)
		vh := max(g.viewH, 1)
		if sg.faithfulSurface == nil || sg.faithfulSurface.Bounds().Dx() != vw || sg.faithfulSurface.Bounds().Dy() != vh {
			sg.faithfulSurface = newUnmanagedImage(vw, vh)
		}
		sg.faithfulSurface.Fill(color.Black)
		if g.mode != viewMap {
			g.drawWalk3D(sg.faithfulSurface)
			g.drawWalkOverlays(sg.faithfulSurface)
		} else {
			g.Draw(sg.faithfulSurface)
		}
		src := sg.faithfulSurface
		if sg.palettePostEnabled() {
			src = sg.applyFaithfulPalettePost(sg.faithfulSurface)
		}
		return src
	}
	w := max(g.viewW, 1)
	h := max(g.viewH, 1)
	if sg.gameplaySurface == nil || sg.gameplaySurface.Bounds().Dx() != w || sg.gameplaySurface.Bounds().Dy() != h {
		sg.gameplaySurface = newUnmanagedImage(w, h)
	}
	src := sg.gameplaySurface
	src.Fill(color.Black)
	if g.mode != viewMap {
		g.drawWalk3D(src)
		g.drawWalkOverlays(src)
	} else {
		g.Draw(src)
	}
	return src
}

func (sg *sessionGame) saveThumbnailImage() (*image.RGBA, error) {
	if isWASMBuild() {
		return nil, errSaveGameWebOnly
	}
	if sg == nil || sg.g == nil {
		return nil, errSaveGameUnavailable
	}
	src := sg.renderSaveThumbnailSourceImage(sg.g)
	if src == nil {
		return nil, errSaveGameUnavailable
	}
	thumbW, thumbH := saveThumbnailDimensions(src.Bounds().Dx(), src.Bounds().Dy())
	thumb := newUnmanagedImage(thumbW, thumbH)
	thumb.Fill(color.Black)
	sw := max(src.Bounds().Dx(), 1)
	sh := max(src.Bounds().Dy(), 1)
	rw, rh, ox, oy := fitRect(thumbW, thumbH, sw, sh)
	op := &ebiten.DrawImageOptions{}
	op.Filter = ebiten.FilterNearest
	op.GeoM.Scale(float64(rw)/float64(sw), float64(rh)/float64(sh))
	op.GeoM.Translate(float64(ox), float64(oy))
	thumb.DrawImage(src, op)
	pix := make([]byte, thumbW*thumbH*4)
	thumb.ReadPixels(pix)
	out := image.NewRGBA(image.Rect(0, 0, thumbW, thumbH))
	copy(out.Pix, pix)
	return out, nil
}

func (sg *sessionGame) writeSaveThumbnail(slot int) error {
	if isWASMBuild() {
		return errSaveGameWebOnly
	}
	if sg == nil || sg.g == nil {
		return errSaveGameUnavailable
	}
	img, err := func() (*image.RGBA, error) {
		defer func() {
			if recover() != nil {
				// Ebiten pixel reads can panic before the game loop is running.
				// In that case, skip the thumbnail instead of writing a blank PNG.
			}
		}()
		return sg.saveThumbnailImage()
	}()
	if err != nil || img == nil {
		_ = os.Remove(saveGameThumbnailPath(slot))
		if sg.saveThumbnailCache != nil {
			delete(sg.saveThumbnailCache, slot)
		}
		return nil
	}
	if err := os.MkdirAll(saveGameDirName, 0o755); err != nil {
		return fmt.Errorf("create save dir: %w", err)
	}
	f, err := os.Create(saveGameThumbnailPath(slot))
	if err != nil {
		return err
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		return err
	}
	if sg.saveThumbnailCache != nil {
		delete(sg.saveThumbnailCache, slot)
	}
	return nil
}

func saveThumbnailDimensions(srcW, srcH int) (int, int) {
	srcW = max(srcW, 1)
	srcH = max(srcH, 1)
	if srcW <= saveThumbnailMaxWidth && srcH <= saveThumbnailMaxHeight {
		return srcW, srcH
	}
	rw, rh, _, _ := fitRect(saveThumbnailMaxWidth, saveThumbnailMaxHeight, srcW, srcH)
	return rw, rh
}

func formatSaveLevelLabel(name mapdata.MapName) string {
	s := strings.ToUpper(strings.TrimSpace(string(name)))
	if s == "" {
		return "UNKNOWN"
	}
	return s
}

func formatSaveLevelLegacyLabel(name mapdata.MapName) string {
	if episode, slot, ok := episodeMapSlot(name); ok {
		return fmt.Sprintf("%d (%s)", slot, fmt.Sprintf("E%dM%d", episode, slot))
	}
	s := strings.ToUpper(strings.TrimSpace(string(name)))
	if strings.HasPrefix(s, "MAP") && len(s) == 5 {
		if s[3] >= '0' && s[3] <= '9' && s[4] >= '0' && s[4] <= '9' {
			return fmt.Sprintf("%d (%s)", (int(s[3]-'0')*10)+(int(s[4]-'0')), s)
		}
	}
	if s == "" {
		return "UNKNOWN"
	}
	return s
}

func formatSavePlaytime(worldTic int) string {
	if worldTic < 0 {
		worldTic = 0
	}
	totalSec := worldTic / doomTicsPerSecond
	h := totalSec / 3600
	m := (totalSec % 3600) / 60
	s := totalSec % 60
	if h > 0 {
		return fmt.Sprintf("%d:%02d:%02d", h, m, s)
	}
	return fmt.Sprintf("%d:%02d", m, s)
}

func formatSaveModTime(t time.Time) string {
	if t.IsZero() {
		return "UNKNOWN"
	}
	return t.Local().Format("1-2-06")
}

func formatSaveWADNames(src []saveWADSource) string {
	if len(src) == 0 {
		return "NONE"
	}
	names := make([]string, 0, len(src))
	for _, wadSource := range src {
		name := strings.TrimSpace(wadSource.Name)
		if name == "" {
			name = "UNKNOWN"
		} else {
			name = strings.TrimSuffix(name, filepath.Ext(name))
			name = strings.ToLower(strings.TrimSpace(name))
			if name == "" {
				name = "unknown"
			}
		}
		names = append(names, name)
	}
	return strings.Join(names, ", ")
}

func formatSaveHealthLabel(health int) string {
	if health <= 0 {
		return "0"
	}
	return fmt.Sprintf("%d", health)
}

func formatSaveSummaryLabel(info saveSlotInfo) string {
	label := fmt.Sprintf(
		"%s HP %s %s %s",
		formatSaveLevelLabel(info.Current),
		formatSaveHealthLabel(info.Health),
		formatSavePlaytime(info.WorldTic),
		formatSaveModTime(info.ModTime),
	)
	if info.Slot <= 0 {
		return "Q: " + label
	}
	return label
}

func (sg *sessionGame) ellipsizeIntermissionText(text string, maxWidth int) string {
	text = strings.TrimSpace(text)
	if text == "" || maxWidth <= 0 {
		return text
	}
	if sg.intermissionTextWidth(text) <= maxWidth {
		return text
	}
	const ellipsis = "..."
	if sg.intermissionTextWidth(ellipsis) > maxWidth {
		return ""
	}
	runes := []rune(text)
	for len(runes) > 0 {
		runes = runes[:len(runes)-1]
		candidate := strings.TrimSpace(string(runes)) + ellipsis
		if sg.intermissionTextWidth(candidate) <= maxWidth {
			return candidate
		}
	}
	return ellipsis
}

func captureSaveWADSources(src []runtimecfg.WADSource) []saveWADSource {
	if len(src) == 0 {
		return nil
	}
	out := make([]saveWADSource, 0, len(src))
	for _, wadSource := range src {
		name := strings.TrimSpace(wadSource.Name)
		hash := strings.TrimSpace(wadSource.Hash)
		if name == "" && hash == "" {
			continue
		}
		out = append(out, saveWADSource{Name: name, Hash: hash})
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func compareSaveWADSources(expected, actual []saveWADSource) string {
	if len(expected) == 0 {
		return ""
	}
	issues := make([]string, 0, 4)
	max := len(expected)
	if len(actual) > max {
		max = len(actual)
	}
	for i := 0; i < max; i++ {
		switch {
		case i >= len(expected):
			name := strings.TrimSpace(actual[i].Name)
			if name == "" {
				name = "UNKNOWN"
			}
			issues = append(issues, fmt.Sprintf("extra WAD %d: %s", i+1, name))
		case i >= len(actual):
			name := strings.TrimSpace(expected[i].Name)
			if name == "" {
				name = "UNKNOWN"
			}
			issues = append(issues, fmt.Sprintf("missing WAD %d: %s", i+1, name))
		case strings.TrimSpace(actual[i].Hash) == "":
			name := strings.TrimSpace(expected[i].Name)
			if name == "" {
				name = strings.TrimSpace(actual[i].Name)
			}
			if name == "" {
				name = "UNKNOWN"
			}
			issues = append(issues, fmt.Sprintf("missing WAD %d: %s", i+1, name))
		case !strings.EqualFold(strings.TrimSpace(expected[i].Hash), strings.TrimSpace(actual[i].Hash)):
			name := strings.TrimSpace(expected[i].Name)
			if name == "" {
				name = strings.TrimSpace(actual[i].Name)
			}
			if name == "" {
				name = "UNKNOWN"
			}
			issues = append(issues, fmt.Sprintf("checksum mismatch WAD %d: %s", i+1, name))
		}
		if len(issues) >= 3 {
			break
		}
	}
	return strings.Join(issues, "; ")
}

func (sg *sessionGame) SaveGameToSlot(slot int) error {
	if isWASMBuild() {
		return errSaveGameWebOnly
	}
	if sg == nil || sg.g == nil {
		return errSaveGameUnavailable
	}
	if sg.frontend.Active && !sg.frontend.InGame {
		return errSaveGameUnavailable
	}
	description := fmt.Sprintf("Slot %d", slot)
	if slot <= 0 {
		description = "Quicksave"
	}
	data, err := sg.marshalSaveGame(description)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(saveGameDirName, 0o755); err != nil {
		return fmt.Errorf("create save dir: %w", err)
	}
	if err := os.WriteFile(saveGamePath(slot), data, 0o644); err != nil {
		return err
	}
	return sg.writeSaveThumbnail(slot)
}

func (sg *sessionGame) LoadGameFromSlot(slot int) error {
	if isWASMBuild() {
		return errSaveGameWebOnly
	}
	if sg == nil {
		return errNoSavedGame
	}
	data, err := os.ReadFile(saveGamePath(slot))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return errNoSavedGame
		}
		return err
	}
	file, err := sg.loadSnapshot(data, saveGameMagic, saveGameVersion)
	if err != nil {
		return err
	}
	warning := compareSaveWADSources(file.WADSources, captureSaveWADSources(sg.opts.WADSources))
	if err := sg.applyLoadedSnapshot(file, warning); err != nil {
		return err
	}
	if err := sg.broadcastMandatoryRuntimeKeyframe(); err != nil {
		return err
	}
	return nil
}

func (sg *sessionGame) marshalSaveGame(description string) ([]byte, error) {
	if sg == nil || sg.g == nil || sg.g.m == nil {
		return nil, errSaveGameUnavailable
	}
	description = strings.TrimSpace(description)
	if description == "" {
		description = "Save Game"
	}
	rndIndex, prndIndex := doomrand.State()
	file := saveFile{
		Version:     saveGameVersion,
		Description: description,
		Current:     sg.current,
		WADSources:  captureSaveWADSources(sg.opts.WADSources),
		RNG: saveRNGState{
			MenuIndex: rndIndex,
			PlayIndex: prndIndex,
		},
		Game: captureGameSaveState(sg.g),
	}
	return encodeSnapshot(saveGameMagic, file)
}

func (sg *sessionGame) unmarshalSaveGame(data []byte) error {
	return sg.unmarshalSnapshot(data, saveGameMagic, saveGameVersion)
}

func (sg *sessionGame) marshalNetplayKeyframe() ([]byte, error) {
	if sg == nil || sg.g == nil || sg.g.m == nil {
		return nil, errSaveGameUnavailable
	}
	rndIndex, prndIndex := doomrand.State()
	file := saveFile{
		Version:     keyframeVersion,
		Description: "Netplay Keyframe",
		Current:     sg.current,
		WADSources:  captureSaveWADSources(sg.opts.WADSources),
		RNG: saveRNGState{
			MenuIndex: rndIndex,
			PlayIndex: prndIndex,
		},
		Game: captureGameSaveState(sg.g),
	}
	return encodeSnapshot(keyframeMagic, file)
}

func (sg *sessionGame) unmarshalNetplayKeyframe(data []byte) error {
	return sg.unmarshalSnapshot(data, keyframeMagic, keyframeVersion)
}

func (sg *sessionGame) broadcastMandatoryRuntimeKeyframe() error {
	if sg == nil || sg.g == nil {
		return nil
	}
	var (
		mandatorySink saveMandatoryKeyframeSink
		regularSink   saveKeyframeSink
	)
	if sg.opts.LiveTicSink != nil {
		mandatorySink, _ = sg.opts.LiveTicSink.(saveMandatoryKeyframeSink)
		regularSink, _ = sg.opts.LiveTicSink.(saveKeyframeSink)
	}
	if mandatorySink == nil && regularSink == nil {
		return nil
	}
	blob, err := sg.marshalNetplayKeyframe()
	if err != nil {
		return fmt.Errorf("capture loaded keyframe: %w", err)
	}
	if len(blob) == 0 {
		return nil
	}
	if mandatorySink != nil {
		if err := mandatorySink.BroadcastKeyframeWithFlags(uint32(sg.g.worldTic), blob, saveMandatoryApplyKeyframeFlag); err != nil {
			return fmt.Errorf("broadcast mandatory keyframe: %w", err)
		}
		return nil
	}
	if err := regularSink.BroadcastKeyframe(uint32(sg.g.worldTic), blob); err != nil {
		return fmt.Errorf("broadcast mandatory keyframe: %w", err)
	}
	return nil
}

func (sg *sessionGame) loadSnapshot(data []byte, magic []byte, version int) (saveFile, error) {
	if sg == nil {
		return saveFile{}, errNoSavedGame
	}
	file, err := decodeSnapshot(data, magic)
	if err != nil {
		return saveFile{}, err
	}
	if file.Version != version {
		return saveFile{}, fmt.Errorf("incompatible save version: %d", file.Version)
	}
	return file, nil
}

func (sg *sessionGame) applyLoadedSnapshot(file saveFile, warning string) error {
	if sg == nil {
		return errNoSavedGame
	}
	if sg.opts.NewGameLoader == nil {
		return fmt.Errorf("save load requires NewGameLoader")
	}
	if strings.TrimSpace(string(file.Current)) == "" {
		return fmt.Errorf("save missing current map")
	}
	applySavedSessionOptions(&sg.opts, file.Game.Session)

	sg.stopAndClearMusic()
	if sg.g != nil {
		sg.g.clearPendingSoundState()
		sg.g.clearSpritePatchCache()
	}

	loadedMap, err := sg.opts.NewGameLoader(string(file.Current))
	if err != nil {
		return fmt.Errorf("load saved map %s: %w", file.Current, err)
	}
	if loadedMap == nil {
		return fmt.Errorf("load saved map %s: nil map", file.Current)
	}
	g := sg.buildGame(loadedMap, sg.opts)
	restoreGameSaveState(g, file.Game)
	doomrand.SetState(file.RNG.MenuIndex, file.RNG.PlayIndex)

	sg.transition.Clear()
	sg.clearQuitPrompt()
	sg.frontend = frontendState{}
	sg.frontendMenuPending = false
	sg.g = g
	sg.rt = g
	sg.current = file.Current
	if sg.current == "" && g.m != nil {
		sg.current = g.m.Name
	}
	sg.currentTemplate = cloneMapForRestart(g.m)
	sg.levelCarryover = nil
	sg.secretVisited = false
	sg.playMusicForMap(sg.current)
	sg.announceMapMusic(sg.current)
	ebiten.SetWindowTitle(runtimehost.WindowTitle(sg.current))
	msg := "GAME LOADED"
	if strings.TrimSpace(warning) != "" {
		msg = "WAD WARNING: " + strings.TrimSpace(warning)
		fmt.Println(msg)
	}
	g.setHUDMessage(msg, 70)
	return nil
}

func (sg *sessionGame) unmarshalSnapshot(data []byte, magic []byte, version int) error {
	file, err := sg.loadSnapshot(data, magic, version)
	if err != nil {
		return err
	}
	return sg.applyLoadedSnapshot(file, "")
}

func applySavedSessionOptions(dst *Options, s sessionSaveState) {
	if dst == nil {
		return
	}
	if s.PlayerSlot != 0 {
		dst.PlayerSlot = s.PlayerSlot
	}
	if s.SkillLevel != 0 {
		dst.SkillLevel = s.SkillLevel
	}
	if strings.TrimSpace(s.GameMode) != "" {
		dst.GameMode = s.GameMode
	}
	dst.ShowNoSkillItems = s.ShowNoSkillItems
	dst.ShowAllItems = s.ShowAllItems
	dst.FastMonsters = s.FastMonsters
	dst.RespawnMonsters = s.RespawnMonsters
	dst.NoMonsters = s.NoMonsters
}

func captureGameSaveState(g *game) gameSaveState {
	if g == nil {
		return gameSaveState{}
	}
	return gameSaveState{
		Session: sessionSaveState{
			PlayerSlot:       g.opts.PlayerSlot,
			SkillLevel:       g.opts.SkillLevel,
			GameMode:         g.opts.GameMode,
			ShowNoSkillItems: g.opts.ShowNoSkillItems,
			ShowAllItems:     g.opts.ShowAllItems,
			FastMonsters:     g.opts.FastMonsters,
			RespawnMonsters:  g.opts.RespawnMonsters,
			NoMonsters:       g.opts.NoMonsters,
		},
		Player:               capturePlayerSaveState(g.p),
		View:                 g.State,
		Mode:                 int(g.mode),
		RotateView:           g.rotateView,
		ParityReveal:         int(g.parity.reveal),
		ParityIDDT:           g.parity.iddt,
		ShowGrid:             g.showGrid,
		ShowLegend:           g.showLegend,
		PaletteLUTEnabled:    g.paletteLUTEnabled,
		GammaLevel:           g.gammaLevel,
		CRTEnabled:           g.crtEnabled,
		UseFlash:             g.useFlash,
		UseText:              g.useText,
		HUDMessagesEnabled:   g.hudMessagesEnabled,
		PrevPX:               g.prevPX,
		PrevPY:               g.prevPY,
		PrevAngle:            g.prevAngle,
		PlayerViewZ:          g.playerViewZ,
		ThingCollected:       append([]bool(nil), g.thingCollected...),
		ThingDropped:         append([]bool(nil), g.thingDropped...),
		ThingThinkerOrder:    append([]int64(nil), g.thingThinkerOrder...),
		ThingX:               append([]int64(nil), g.thingX...),
		ThingY:               append([]int64(nil), g.thingY...),
		ThingMomX:            append([]int64(nil), g.thingMomX...),
		ThingMomY:            append([]int64(nil), g.thingMomY...),
		ThingMomZ:            append([]int64(nil), g.thingMomZ...),
		ThingAngleState:      append([]uint32(nil), g.thingAngleState...),
		ThingZState:          append([]int64(nil), g.thingZState...),
		ThingFloorState:      append([]int64(nil), g.thingFloorState...),
		ThingCeilState:       append([]int64(nil), g.thingCeilState...),
		ThingSupportValid:    append([]bool(nil), g.thingSupportValid...),
		ThingHP:              append([]int(nil), g.thingHP...),
		ThingAggro:           append([]bool(nil), g.thingAggro...),
		ThingTargetPlayer:    append([]bool(nil), g.thingTargetPlayer...),
		ThingTargetIdx:       append([]int(nil), g.thingTargetIdx...),
		ThingThreshold:       append([]int(nil), g.thingThreshold...),
		ThingCooldown:        append([]int(nil), g.thingCooldown...),
		ThingMoveDir:         cloneMonsterMoveDirSlice(g.thingMoveDir),
		ThingMoveCount:       append([]int(nil), g.thingMoveCount...),
		ThingJustAtk:         append([]bool(nil), g.thingJustAtk...),
		ThingJustHit:         append([]bool(nil), g.thingJustHit...),
		ThingSkullFly:        append([]bool(nil), g.thingSkullFly...),
		ThingResumeChaseNow:  append([]bool(nil), g.thingResumeChaseNow...),
		ThingReactionTics:    append([]int(nil), g.thingReactionTics...),
		ThingWakeTics:        append([]int(nil), g.thingWakeTics...),
		ThingLastLook:        append([]int(nil), g.thingLastLook...),
		ThingDead:            append([]bool(nil), g.thingDead...),
		ThingAmbush:          append([]bool(nil), g.thingAmbush...),
		ThingInFloat:         append([]bool(nil), g.thingInFloat...),
		ThingGibbed:          append([]bool(nil), g.thingGibbed...),
		ThingGibTick:         append([]int(nil), g.thingGibTick...),
		ThingXDeath:          append([]bool(nil), g.thingXDeath...),
		ThingDeathTics:       append([]int(nil), g.thingDeathTics...),
		ThingAttackTics:      append([]int(nil), g.thingAttackTics...),
		ThingAttackPhase:     append([]int(nil), g.thingAttackPhase...),
		ThingAttackFireTics:  append([]int(nil), g.thingAttackFireTics...),
		ThingPainTics:        append([]int(nil), g.thingPainTics...),
		ThingThinkWait:       append([]int(nil), g.thingThinkWait...),
		ThingDoomState:       append([]int(nil), g.thingDoomState...),
		ThingState:           cloneMonsterThinkStateSlice(g.thingState),
		ThingStateTics:       append([]int(nil), g.thingStateTics...),
		ThingStatePhase:      append([]int(nil), g.thingStatePhase...),
		BossSpawnCubes:       captureBossSpawnCubes(g.bossSpawnCubes),
		BossSpawnFires:       captureBossSpawnFires(g.bossSpawnFires),
		BossBrainTargetOrder: g.bossBrainTargetOrder,
		BossBrainEasyToggle:  g.bossBrainEasyToggle,
		Projectiles:          captureProjectiles(g.projectiles),
		ProjectileImpacts:    captureProjectileImpacts(g.projectileImpacts),
		HitscanPuffs:         captureHitscanPuffs(g.hitscanPuffs),
		CheatLevel:           g.cheatLevel,
		Invulnerable:         g.invulnerable,
		NoClip:               g.noClip,
		Inventory:            capturePlayerInventorySaveState(g.inventory),
		AlwaysRun:            g.alwaysRun,
		AutoWeaponSwitch:     g.autoWeaponSwitch,
		WeaponRefire:         g.weaponRefire,
		WeaponAttackDown:     g.weaponAttackDown,
		WeaponState:          int(g.weaponState),
		WeaponStateTics:      g.weaponStateTics,
		WeaponFlashState:     int(g.weaponFlashState),
		WeaponFlashTics:      g.weaponFlashTics,
		WeaponPSpriteY:       g.weaponPSpriteY,
		Stats:                g.stats,
		WorldTic:             g.worldTic,
		PlayerBlockOrder:     g.playerBlockOrder,
		NextThinkerOrder:     g.nextThinkerOrder,
		NextBlockmapOrder:    g.nextBlockmapOrder,
		SecretFound:          append([]bool(nil), g.secretFound...),
		SecretsFound:         g.secretsFound,
		SecretsTotal:         g.secretsTotal,
		SectorSoundTarget:    append([]bool(nil), g.sectorSoundTarget...),
		IsDead:               g.isDead,
		PlayerMobjHealth:     g.playerMobjHealth,
		DamageFlashTic:       g.damageFlashTic,
		BonusFlashTic:        g.bonusFlashTic,
		SectorLightFx:        captureSectorLightEffects(g.sectorLightFx),
		Things:               append([]mapdata.Thing(nil), g.m.Things...),
		Sidedefs:             append([]mapdata.Sidedef(nil), g.m.Sidedefs...),
		Sectors:              append([]mapdata.Sector(nil), g.m.Sectors...),
		SectorFloor:          append([]int64(nil), g.sectorFloor...),
		SectorCeil:           append([]int64(nil), g.sectorCeil...),
		LineSpecial:          append([]uint16(nil), g.lineSpecial...),
		Doors:                captureDoorThinkers(g.doors),
		Floors:               captureFloorThinkers(g.floors),
		Plats:                capturePlatThinkers(g.plats),
		Ceilings:             captureCeilingThinkers(g.ceilings),
		DelayedSwitchReverts: captureDelayedSwitchTextures(g.delayedSwitchReverts),
	}
}

func restoreGameSaveState(g *game, s gameSaveState) {
	if g == nil {
		return
	}
	applySavedSessionOptions(&g.opts, s.Session)
	g.p = restorePlayerSaveState(s.Player)
	g.refreshPlayerSubsectorCache(g.p.x, g.p.y)
	g.State = s.View
	g.mode = viewMode(s.Mode)
	g.rotateView = s.RotateView
	g.parity.reveal = normalizeRevealForMode(revealMode(s.ParityReveal), g.opts.SourcePortMode)
	g.parity.iddt = clampIDDT(s.ParityIDDT)
	g.showGrid = s.ShowGrid
	g.showLegend = s.ShowLegend
	g.paletteLUTEnabled = s.PaletteLUTEnabled
	g.gammaLevel = clampGamma(s.GammaLevel)
	g.crtEnabled = s.CRTEnabled
	g.useFlash = s.UseFlash
	g.useText = s.UseText
	g.hudMessagesEnabled = s.HUDMessagesEnabled
	g.prevPX = s.PrevPX
	g.prevPY = s.PrevPY
	g.prevPrevAngle = s.PrevAngle
	g.prevAngle = s.PrevAngle
	g.playerViewZ = s.PlayerViewZ
	g.thingCollected = append([]bool(nil), s.ThingCollected...)
	g.thingDropped = append([]bool(nil), s.ThingDropped...)
	g.thingThinkerOrder = append([]int64(nil), s.ThingThinkerOrder...)
	g.thingX = append([]int64(nil), s.ThingX...)
	g.thingY = append([]int64(nil), s.ThingY...)
	g.thingMomX = append([]int64(nil), s.ThingMomX...)
	g.thingMomY = append([]int64(nil), s.ThingMomY...)
	g.thingMomZ = append([]int64(nil), s.ThingMomZ...)
	g.thingAngleState = append([]uint32(nil), s.ThingAngleState...)
	g.thingZState = append([]int64(nil), s.ThingZState...)
	g.thingFloorState = append([]int64(nil), s.ThingFloorState...)
	g.thingCeilState = append([]int64(nil), s.ThingCeilState...)
	g.thingSupportValid = append([]bool(nil), s.ThingSupportValid...)
	g.thingHP = append([]int(nil), s.ThingHP...)
	g.thingAggro = append([]bool(nil), s.ThingAggro...)
	g.thingTargetPlayer = append([]bool(nil), s.ThingTargetPlayer...)
	g.thingTargetIdx = append([]int(nil), s.ThingTargetIdx...)
	g.thingThreshold = append([]int(nil), s.ThingThreshold...)
	g.thingCooldown = append([]int(nil), s.ThingCooldown...)
	g.thingMoveDir = restoreMonsterMoveDirSlice(s.ThingMoveDir)
	g.thingMoveCount = append([]int(nil), s.ThingMoveCount...)
	g.thingJustAtk = append([]bool(nil), s.ThingJustAtk...)
	g.thingJustHit = append([]bool(nil), s.ThingJustHit...)
	g.thingSkullFly = append([]bool(nil), s.ThingSkullFly...)
	g.thingResumeChaseNow = append([]bool(nil), s.ThingResumeChaseNow...)
	g.thingReactionTics = append([]int(nil), s.ThingReactionTics...)
	g.thingWakeTics = append([]int(nil), s.ThingWakeTics...)
	g.thingLastLook = append([]int(nil), s.ThingLastLook...)
	g.thingDead = append([]bool(nil), s.ThingDead...)
	g.thingAmbush = append([]bool(nil), s.ThingAmbush...)
	g.thingInFloat = append([]bool(nil), s.ThingInFloat...)
	g.thingGibbed = append([]bool(nil), s.ThingGibbed...)
	g.thingGibTick = append([]int(nil), s.ThingGibTick...)
	g.thingXDeath = append([]bool(nil), s.ThingXDeath...)
	g.thingDeathTics = append([]int(nil), s.ThingDeathTics...)
	g.thingAttackTics = append([]int(nil), s.ThingAttackTics...)
	g.thingAttackPhase = append([]int(nil), s.ThingAttackPhase...)
	g.thingAttackFireTics = append([]int(nil), s.ThingAttackFireTics...)
	g.thingPainTics = append([]int(nil), s.ThingPainTics...)
	g.thingThinkWait = append([]int(nil), s.ThingThinkWait...)
	g.thingDoomState = append([]int(nil), s.ThingDoomState...)
	g.thingState = restoreMonsterThinkStateSlice(s.ThingState)
	g.thingStateTics = append([]int(nil), s.ThingStateTics...)
	g.thingStatePhase = append([]int(nil), s.ThingStatePhase...)
	g.bossSpawnCubes = restoreBossSpawnCubes(s.BossSpawnCubes)
	g.bossSpawnFires = restoreBossSpawnFires(s.BossSpawnFires)
	g.bossBrainTargetOrder = s.BossBrainTargetOrder
	g.bossBrainEasyToggle = s.BossBrainEasyToggle
	g.projectiles = restoreProjectiles(s.Projectiles)
	g.projectileImpacts = restoreProjectileImpacts(s.ProjectileImpacts)
	g.hitscanPuffs = restoreHitscanPuffs(s.HitscanPuffs)
	g.cheatLevel = normalizeCheatLevel(s.CheatLevel)
	g.invulnerable = s.Invulnerable
	g.noClip = s.NoClip
	g.inventory = restorePlayerInventorySaveState(s.Inventory)
	g.alwaysRun = s.AlwaysRun
	g.autoWeaponSwitch = s.AutoWeaponSwitch
	g.weaponRefire = s.WeaponRefire
	g.weaponAttackDown = s.WeaponAttackDown
	g.weaponState = weaponPspriteState(s.WeaponState)
	g.weaponStateTics = s.WeaponStateTics
	g.weaponFlashState = weaponPspriteState(s.WeaponFlashState)
	g.weaponFlashTics = s.WeaponFlashTics
	g.weaponPSpriteY = s.WeaponPSpriteY
	g.stats = s.Stats
	g.worldTic = s.WorldTic
	g.playerBlockOrder = s.PlayerBlockOrder
	g.nextThinkerOrder = s.NextThinkerOrder
	g.nextBlockmapOrder = s.NextBlockmapOrder
	g.secretFound = append([]bool(nil), s.SecretFound...)
	g.secretsFound = s.SecretsFound
	g.secretsTotal = s.SecretsTotal
	g.sectorSoundTarget = append([]bool(nil), s.SectorSoundTarget...)
	g.isDead = s.IsDead
	g.playerMobjHealth = s.PlayerMobjHealth
	if g.playerMobjHealth == 0 && g.stats.Health != 0 {
		g.playerMobjHealth = g.stats.Health
	}
	g.damageFlashTic = s.DamageFlashTic
	g.bonusFlashTic = s.BonusFlashTic
	g.sectorLightFx = restoreSectorLightEffects(s.SectorLightFx)
	if len(s.Things) > 0 {
		g.m.Things = append([]mapdata.Thing(nil), s.Things...)
	}
	if len(s.Sidedefs) > 0 {
		g.m.Sidedefs = append([]mapdata.Sidedef(nil), s.Sidedefs...)
	}
	if len(s.Sectors) > 0 {
		g.m.Sectors = append([]mapdata.Sector(nil), s.Sectors...)
	}
	g.sectorFloor = append([]int64(nil), s.SectorFloor...)
	g.sectorCeil = append([]int64(nil), s.SectorCeil...)
	for sec := range g.m.Sectors {
		if sec < len(g.sectorFloor) {
			g.m.Sectors[sec].FloorHeight = int16(g.sectorFloor[sec] >> fracBits)
		}
		if sec < len(g.sectorCeil) {
			g.m.Sectors[sec].CeilingHeight = int16(g.sectorCeil[sec] >> fracBits)
		}
	}
	g.lineSpecial = append([]uint16(nil), s.LineSpecial...)
	g.doors = restoreDoorThinkers(s.Doors)
	g.floors = restoreFloorThinkers(s.Floors)
	g.plats = restorePlatThinkers(s.Plats)
	g.ceilings = restoreCeilingThinkers(s.Ceilings)
	g.delayedSwitchReverts = restoreDelayedSwitchTextures(s.DelayedSwitchReverts)
	g.thingSectorCache = make([]int, len(g.m.Things))
	for i, th := range g.m.Things {
		x, y := g.thingPosFixed(i, th)
		g.thingSectorCache[i] = g.sectorAt(x, y)
	}
	g.State.SyncRender()
	g.rebuildThingBlockmap()
	g.ensureWeaponDefaults()
	g.runtimeSettingsSeen = true
	g.runtimeSettingsLast = g.runtimeSettingsSnapshot()
	g.syncRenderState()
}

func capturePlayerSaveState(p player) playerSaveState {
	return playerSaveState{
		X:               p.x,
		Y:               p.y,
		Z:               p.z,
		FloorZ:          p.floorz,
		CeilZ:           p.ceilz,
		Subsector:       p.subsector,
		Sector:          p.sector,
		Angle:           p.angle,
		MomX:            p.momx,
		MomY:            p.momy,
		MomZ:            p.momz,
		ReactionTime:    p.reactionTime,
		ViewHeight:      p.viewHeight,
		DeltaViewHeight: p.deltaViewHeight,
	}
}

func restorePlayerSaveState(s playerSaveState) player {
	return player{
		x:               s.X,
		y:               s.Y,
		z:               s.Z,
		floorz:          s.FloorZ,
		ceilz:           s.CeilZ,
		subsector:       s.Subsector,
		sector:          s.Sector,
		angle:           s.Angle,
		momx:            s.MomX,
		momy:            s.MomY,
		momz:            s.MomZ,
		reactionTime:    s.ReactionTime,
		viewHeight:      s.ViewHeight,
		deltaViewHeight: s.DeltaViewHeight,
	}
}

func capturePlayerInventorySaveState(inv playerInventory) playerInventorySaveState {
	return playerInventorySaveState{
		BlueKey:       inv.BlueKey,
		RedKey:        inv.RedKey,
		YellowKey:     inv.YellowKey,
		Backpack:      inv.Backpack,
		Strength:      inv.Strength,
		StrengthCount: inv.StrengthCount,
		InvulnTics:    inv.InvulnTics,
		InvisTics:     inv.InvisTics,
		RadSuitTics:   inv.RadSuitTics,
		AllMap:        inv.AllMap,
		LightAmpTics:  inv.LightAmpTics,
		ReadyWeapon:   int(inv.ReadyWeapon),
		PendingWeapon: int(inv.PendingWeapon),
		Weapons:       cloneWeaponInventory(inv.Weapons),
	}
}

func restorePlayerInventorySaveState(s playerInventorySaveState) playerInventory {
	return playerInventory{
		BlueKey:       s.BlueKey,
		RedKey:        s.RedKey,
		YellowKey:     s.YellowKey,
		Backpack:      s.Backpack,
		Strength:      s.Strength,
		StrengthCount: s.StrengthCount,
		InvulnTics:    s.InvulnTics,
		InvisTics:     s.InvisTics,
		RadSuitTics:   s.RadSuitTics,
		AllMap:        s.AllMap,
		LightAmpTics:  s.LightAmpTics,
		ReadyWeapon:   weaponID(s.ReadyWeapon),
		PendingWeapon: weaponID(s.PendingWeapon),
		Weapons:       cloneWeaponInventory(s.Weapons),
	}
}

func cloneMonsterMoveDirSlice(src []monsterMoveDir) []uint8 {
	if len(src) == 0 {
		return nil
	}
	dst := make([]uint8, len(src))
	for i, v := range src {
		dst[i] = uint8(v)
	}
	return dst
}

func restoreMonsterMoveDirSlice(src []uint8) []monsterMoveDir {
	if len(src) == 0 {
		return nil
	}
	dst := make([]monsterMoveDir, len(src))
	for i, v := range src {
		dst[i] = monsterMoveDir(v)
	}
	return dst
}

func cloneMonsterThinkStateSlice(src []monsterThinkState) []uint8 {
	if len(src) == 0 {
		return nil
	}
	dst := make([]uint8, len(src))
	for i, v := range src {
		dst[i] = uint8(v)
	}
	return dst
}

func restoreMonsterThinkStateSlice(src []uint8) []monsterThinkState {
	if len(src) == 0 {
		return nil
	}
	dst := make([]monsterThinkState, len(src))
	for i, v := range src {
		dst[i] = monsterThinkState(v)
	}
	return dst
}

func captureDoorThinkers(src map[int]*doorThinker) map[int]doorThinkerSaveState {
	if len(src) == 0 {
		return nil
	}
	dst := make(map[int]doorThinkerSaveState, len(src))
	for key, thinker := range src {
		if thinker == nil {
			continue
		}
		dst[key] = doorThinkerSaveState{
			Order:        thinker.order,
			Sector:       thinker.sector,
			Type:         int(thinker.typ),
			Direction:    thinker.direction,
			TopHeight:    thinker.topHeight,
			TopWait:      thinker.topWait,
			TopCountdown: thinker.topCountdown,
			Speed:        thinker.speed,
		}
	}
	return dst
}

func restoreDoorThinkers(src map[int]doorThinkerSaveState) map[int]*doorThinker {
	if len(src) == 0 {
		return map[int]*doorThinker{}
	}
	dst := make(map[int]*doorThinker, len(src))
	for key, thinker := range src {
		dst[key] = &doorThinker{
			order:        thinker.Order,
			sector:       thinker.Sector,
			typ:          doorType(thinker.Type),
			direction:    thinker.Direction,
			topHeight:    thinker.TopHeight,
			topWait:      thinker.TopWait,
			topCountdown: thinker.TopCountdown,
			speed:        thinker.Speed,
		}
	}
	return dst
}

func captureFloorThinkers(src map[int]*floorThinker) map[int]floorThinkerSaveState {
	if len(src) == 0 {
		return nil
	}
	dst := make(map[int]floorThinkerSaveState, len(src))
	for key, thinker := range src {
		if thinker == nil {
			continue
		}
		dst[key] = floorThinkerSaveState{
			Order:         thinker.order,
			Sector:        thinker.sector,
			Direction:     thinker.direction,
			Speed:         thinker.speed,
			DestHeight:    thinker.destHeight,
			Finish:        uint8(thinker.finish),
			FinishFlat:    thinker.finishFlat,
			FinishSpecial: thinker.finishSpecial,
		}
	}
	return dst
}

func restoreFloorThinkers(src map[int]floorThinkerSaveState) map[int]*floorThinker {
	if len(src) == 0 {
		return map[int]*floorThinker{}
	}
	dst := make(map[int]*floorThinker, len(src))
	for key, thinker := range src {
		dst[key] = &floorThinker{
			order:         thinker.Order,
			sector:        thinker.Sector,
			direction:     thinker.Direction,
			speed:         thinker.Speed,
			destHeight:    thinker.DestHeight,
			finish:        floorFinishAction(thinker.Finish),
			finishFlat:    thinker.FinishFlat,
			finishSpecial: thinker.FinishSpecial,
		}
	}
	return dst
}

func capturePlatThinkers(src map[int]*platThinker) map[int]platThinkerSaveState {
	if len(src) == 0 {
		return nil
	}
	dst := make(map[int]platThinkerSaveState, len(src))
	for key, thinker := range src {
		if thinker == nil {
			continue
		}
		dst[key] = platThinkerSaveState{
			Order:         thinker.order,
			Sector:        thinker.sector,
			Type:          uint8(thinker.typ),
			Status:        uint8(thinker.status),
			OldStatus:     uint8(thinker.oldStatus),
			Speed:         thinker.speed,
			Low:           thinker.low,
			High:          thinker.high,
			Wait:          thinker.wait,
			Count:         thinker.count,
			FinishFlat:    thinker.finishFlat,
			FinishSpecial: thinker.finishSpecial,
		}
	}
	return dst
}

func restorePlatThinkers(src map[int]platThinkerSaveState) map[int]*platThinker {
	if len(src) == 0 {
		return map[int]*platThinker{}
	}
	dst := make(map[int]*platThinker, len(src))
	for key, thinker := range src {
		dst[key] = &platThinker{
			order:         thinker.Order,
			sector:        thinker.Sector,
			typ:           platType(thinker.Type),
			status:        platStatus(thinker.Status),
			oldStatus:     platStatus(thinker.OldStatus),
			speed:         thinker.Speed,
			low:           thinker.Low,
			high:          thinker.High,
			wait:          thinker.Wait,
			count:         thinker.Count,
			finishFlat:    thinker.FinishFlat,
			finishSpecial: thinker.FinishSpecial,
		}
	}
	return dst
}

func captureCeilingThinkers(src map[int]*ceilingThinker) map[int]ceilingThinkerSaveState {
	if len(src) == 0 {
		return nil
	}
	dst := make(map[int]ceilingThinkerSaveState, len(src))
	for key, thinker := range src {
		if thinker == nil {
			continue
		}
		dst[key] = ceilingThinkerSaveState{
			Order:        thinker.order,
			Sector:       thinker.sector,
			Action:       thinker.action,
			Direction:    thinker.direction,
			OldDirection: thinker.oldDirection,
			Speed:        thinker.speed,
			TopHeight:    thinker.topHeight,
			BottomHeight: thinker.bottomHeight,
			Crush:        thinker.crush,
		}
	}
	return dst
}

func restoreCeilingThinkers(src map[int]ceilingThinkerSaveState) map[int]*ceilingThinker {
	if len(src) == 0 {
		return map[int]*ceilingThinker{}
	}
	dst := make(map[int]*ceilingThinker, len(src))
	for key, thinker := range src {
		dst[key] = &ceilingThinker{
			order:        thinker.Order,
			sector:       thinker.Sector,
			action:       thinker.Action,
			direction:    thinker.Direction,
			oldDirection: thinker.OldDirection,
			speed:        thinker.Speed,
			topHeight:    thinker.TopHeight,
			bottomHeight: thinker.BottomHeight,
			crush:        thinker.Crush,
		}
	}
	return dst
}

func captureBossSpawnCubes(src []bossSpawnCube) []bossSpawnCubeSaveState {
	if len(src) == 0 {
		return nil
	}
	dst := make([]bossSpawnCubeSaveState, len(src))
	for i, cube := range src {
		dst[i] = bossSpawnCubeSaveState{
			X:         cube.x,
			Y:         cube.y,
			Z:         cube.z,
			VX:        cube.vx,
			VY:        cube.vy,
			VZ:        cube.vz,
			TargetIdx: cube.targetIdx,
			StateTics: cube.stateTics,
			StateStep: cube.stateStep,
			Reaction:  cube.reaction,
		}
	}
	return dst
}

func restoreBossSpawnCubes(src []bossSpawnCubeSaveState) []bossSpawnCube {
	if len(src) == 0 {
		return nil
	}
	dst := make([]bossSpawnCube, len(src))
	for i, cube := range src {
		dst[i] = bossSpawnCube{
			x:         cube.X,
			y:         cube.Y,
			z:         cube.Z,
			vx:        cube.VX,
			vy:        cube.VY,
			vz:        cube.VZ,
			targetIdx: cube.TargetIdx,
			stateTics: cube.StateTics,
			stateStep: cube.StateStep,
			reaction:  cube.Reaction,
		}
	}
	return dst
}

func captureBossSpawnFires(src []bossSpawnFire) []bossSpawnFireSaveState {
	if len(src) == 0 {
		return nil
	}
	dst := make([]bossSpawnFireSaveState, len(src))
	for i, fire := range src {
		dst[i] = bossSpawnFireSaveState{
			X:    fire.x,
			Y:    fire.y,
			Z:    fire.z,
			Tics: fire.tics,
		}
	}
	return dst
}

func restoreBossSpawnFires(src []bossSpawnFireSaveState) []bossSpawnFire {
	if len(src) == 0 {
		return nil
	}
	dst := make([]bossSpawnFire, len(src))
	for i, fire := range src {
		dst[i] = bossSpawnFire{
			x:    fire.X,
			y:    fire.Y,
			z:    fire.Z,
			tics: fire.Tics,
		}
	}
	return dst
}

func captureProjectiles(src []projectile) []projectileSaveState {
	if len(src) == 0 {
		return nil
	}
	dst := make([]projectileSaveState, len(src))
	for i, p := range src {
		dst[i] = projectileSaveState{
			X:            p.x,
			Y:            p.y,
			Z:            p.z,
			VX:           p.vx,
			VY:           p.vy,
			VZ:           p.vz,
			FloorZ:       p.floorz,
			CeilZ:        p.ceilz,
			Radius:       p.radius,
			Height:       p.height,
			TTL:          p.ttl,
			SourceX:      p.sourceX,
			SourceY:      p.sourceY,
			SourceThing:  p.sourceThing,
			SourceType:   p.sourceType,
			SourcePlayer: p.sourcePlayer,
			TracerPlayer: p.tracerPlayer,
			Angle:        p.angle,
			Kind:         int(p.kind),
		}
	}
	return dst
}

func restoreProjectiles(src []projectileSaveState) []projectile {
	if len(src) == 0 {
		return nil
	}
	dst := make([]projectile, len(src))
	for i, p := range src {
		dst[i] = projectile{
			x:            p.X,
			y:            p.Y,
			z:            p.Z,
			vx:           p.VX,
			vy:           p.VY,
			vz:           p.VZ,
			floorz:       p.FloorZ,
			ceilz:        p.CeilZ,
			radius:       p.Radius,
			height:       p.Height,
			ttl:          p.TTL,
			sourceX:      p.SourceX,
			sourceY:      p.SourceY,
			sourceThing:  p.SourceThing,
			sourceType:   p.SourceType,
			sourcePlayer: p.SourcePlayer,
			tracerPlayer: p.TracerPlayer,
			angle:        p.Angle,
			kind:         projectileKind(p.Kind),
		}
	}
	return dst
}

func captureProjectileImpacts(src []projectileImpact) []projectileImpactSaveState {
	if len(src) == 0 {
		return nil
	}
	dst := make([]projectileImpactSaveState, len(src))
	for i, p := range src {
		dst[i] = projectileImpactSaveState{
			X:            p.x,
			Y:            p.y,
			Z:            p.z,
			FloorZ:       p.floorz,
			CeilZ:        p.ceilz,
			Kind:         int(p.kind),
			SourceThing:  p.sourceThing,
			SourceType:   p.sourceType,
			SourcePlayer: p.sourcePlayer,
			LastLook:     p.lastLook,
			Tics:         p.tics,
			TotalTics:    p.totalTics,
			Angle:        p.angle,
			SprayDone:    p.sprayDone,
		}
	}
	return dst
}

func restoreProjectileImpacts(src []projectileImpactSaveState) []projectileImpact {
	if len(src) == 0 {
		return nil
	}
	dst := make([]projectileImpact, len(src))
	for i, p := range src {
		dst[i] = projectileImpact{
			x:            p.X,
			y:            p.Y,
			z:            p.Z,
			floorz:       p.FloorZ,
			ceilz:        p.CeilZ,
			kind:         projectileKind(p.Kind),
			sourceThing:  p.SourceThing,
			sourceType:   p.SourceType,
			sourcePlayer: p.SourcePlayer,
			lastLook:     p.LastLook,
			tics:         p.Tics,
			totalTics:    p.TotalTics,
			angle:        p.Angle,
			sprayDone:    p.SprayDone,
		}
	}
	return dst
}

func captureHitscanPuffs(src []hitscanPuff) []hitscanPuffSaveState {
	if len(src) == 0 {
		return nil
	}
	dst := make([]hitscanPuffSaveState, len(src))
	for i, puff := range src {
		dst[i] = hitscanPuffSaveState{
			X:        puff.x,
			Y:        puff.y,
			Z:        puff.z,
			MomZ:     puff.momz,
			Tics:     puff.tics,
			State:    puff.state,
			TotalTic: puff.totalTic,
			Kind:     puff.kind,
		}
	}
	return dst
}

func restoreHitscanPuffs(src []hitscanPuffSaveState) []hitscanPuff {
	if len(src) == 0 {
		return nil
	}
	dst := make([]hitscanPuff, len(src))
	for i, puff := range src {
		dst[i] = hitscanPuff{
			x:        puff.X,
			y:        puff.Y,
			z:        puff.Z,
			momz:     puff.MomZ,
			tics:     puff.Tics,
			state:    puff.State,
			totalTic: puff.TotalTic,
			kind:     puff.Kind,
		}
	}
	return dst
}

func captureSectorLightEffects(src []sectorLightEffect) []sectorLightEffectSaveState {
	if len(src) == 0 {
		return nil
	}
	dst := make([]sectorLightEffectSaveState, len(src))
	for i, fx := range src {
		dst[i] = sectorLightEffectSaveState{
			Kind:       uint8(fx.kind),
			MinLight:   fx.minLight,
			MaxLight:   fx.maxLight,
			Count:      fx.count,
			MinTime:    fx.minTime,
			MaxTime:    fx.maxTime,
			DarkTime:   fx.darkTime,
			BrightTime: fx.brightTime,
			Direction:  fx.direction,
		}
	}
	return dst
}

func restoreSectorLightEffects(src []sectorLightEffectSaveState) []sectorLightEffect {
	if len(src) == 0 {
		return nil
	}
	dst := make([]sectorLightEffect, len(src))
	for i, fx := range src {
		dst[i] = sectorLightEffect{
			kind:       sectorLightEffectKind(fx.Kind),
			minLight:   fx.MinLight,
			maxLight:   fx.MaxLight,
			count:      fx.Count,
			minTime:    fx.MinTime,
			maxTime:    fx.MaxTime,
			darkTime:   fx.DarkTime,
			brightTime: fx.BrightTime,
			direction:  fx.Direction,
		}
	}
	return dst
}

func captureDelayedSwitchTextures(src []delayedSwitchTexture) []delayedSwitchTextureSaveState {
	if len(src) == 0 {
		return nil
	}
	dst := make([]delayedSwitchTextureSaveState, len(src))
	for i, sw := range src {
		dst[i] = delayedSwitchTextureSaveState{
			Line:    sw.line,
			Sidedef: sw.sidedef,
			Top:     sw.top,
			Bottom:  sw.bottom,
			Mid:     sw.mid,
			Tics:    sw.tics,
		}
	}
	return dst
}

func restoreDelayedSwitchTextures(src []delayedSwitchTextureSaveState) []delayedSwitchTexture {
	if len(src) == 0 {
		return nil
	}
	dst := make([]delayedSwitchTexture, len(src))
	for i, sw := range src {
		dst[i] = delayedSwitchTexture{
			line:    sw.Line,
			sidedef: sw.Sidedef,
			top:     sw.Top,
			bottom:  sw.Bottom,
			mid:     sw.Mid,
			tics:    sw.Tics,
		}
	}
	return dst
}
