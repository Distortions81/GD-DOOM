package doomruntime

import (
	"fmt"
	"image"
	"image/color"
	"math"
	"os"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"
	"unsafe"

	"gddoom/internal/doomrand"
	"gddoom/internal/mapdata"
	"gddoom/internal/media"
	"gddoom/internal/render/hud"
	"gddoom/internal/render/mapview"
	"gddoom/internal/render/mapview/linepolicy"
	"gddoom/internal/render/mapview/presenter"
	"gddoom/internal/render/scene"
	"gddoom/internal/runtimecfg"

	"github.com/dustin/go-humanize"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

const (
	doomLogicalW            = 320
	doomLogicalH            = 200
	doomScreenBlocksMin     = 0
	doomScreenBlocksOverlay = 1
	doomScreenBlocksFull    = 2
	doomScreenBlocksDefault = doomScreenBlocksMin

	lineOneSidedWidth  = 1.8
	lineTwoSidedWidth  = 1.2
	doomInitialZoomMul = 1.0 / 0.7
	// Give cursor capture/resizing a couple of frames to settle after detail changes.
	detailMouseSuppressTicks         = 3
	mlDontPegTop                     = 1 << 3
	mlDontPegBottom                  = 1 << 4
	statusNumPainFaces               = 5
	statusNumStraightFaces           = 3
	statusNumTurnFaces               = 2
	statusFaceStride                 = statusNumStraightFaces + statusNumTurnFaces + 3
	statusTurnOffset                 = statusNumStraightFaces
	statusOuchOffset                 = statusTurnOffset + statusNumTurnFaces
	statusEvilGrinOffset             = statusOuchOffset + 1
	statusRampageOffset              = statusEvilGrinOffset + 1
	statusGodFace                    = statusNumPainFaces * statusFaceStride
	statusDeadFace                   = statusGodFace + 1
	statusEvilGrinCount              = 2 * doomTicsPerSecond
	statusStraightFaceCount          = doomTicsPerSecond / 2
	statusTurnCount                  = doomTicsPerSecond
	statusRampageDelay               = 2 * doomTicsPerSecond
	statusMuchPain                   = 20
	huMsgTimeout                     = 4 * doomTicsPerSecond
	statusAng45               uint32 = 0x20000000
	statusAng180              uint32 = 0x80000000
	depthQuantScale                  = 16.0
	spriteRowOcclusionMinSpan        = 24
)

var (
	bgColor              = color.RGBA{R: 5, G: 7, B: 9, A: 255}
	wallOneSided         = color.RGBA{R: 220, G: 58, B: 48, A: 255}
	wallSecret           = color.RGBA{R: 160, G: 100, B: 220, A: 255}
	wallTeleporter       = color.RGBA{R: 40, G: 165, B: 220, A: 255}
	wallFloorChange      = color.RGBA{R: 170, G: 120, B: 60, A: 255}
	wallCeilChange       = color.RGBA{R: 220, G: 200, B: 70, A: 255}
	wallNoHeightDiff     = color.RGBA{R: 86, G: 86, B: 86, A: 255}
	wallUnrevealed       = color.RGBA{R: 100, G: 100, B: 100, A: 255}
	wallUseSpecial       = color.RGBA{R: 255, G: 80, B: 170, A: 255}
	playerColor          = color.RGBA{R: 120, G: 240, B: 130, A: 255}
	otherPlayerColor     = color.RGBA{R: 90, G: 170, B: 255, A: 255}
	useTargetColor       = color.RGBA{R: 255, G: 210, B: 70, A: 255}
	wallShadeLUTOnce     sync.Once
	wallShadeLUT         [257][256]uint8
	wallShadePackedBanks [doomGammaLevels][257][256]uint32
	wallShadePackedLUT   [257][256]uint32
	wallShadePackedOK    bool
	paletteIndexByPacked map[uint32]uint8
	sectorLightLUTOnce   sync.Once
	sectorLightMulLUT    [256]uint8
	fullbrightNoLighting bool
	doomLightingEnabled  bool
	doomSectorLighting   = true
	doomColormapEnabled  bool
	doomColormapRows     int
	doomRowShadeMulLUT   []uint16
	doomColormapBanks    [doomGammaLevels][]uint32
	doomColormapRGBA     []uint32
	doomPalIndexLUT32    []uint8
	activeGammaLevel     int
	mapLinePalette       = linepolicy.Palette{
		OneSided:     wallOneSided,
		Secret:       wallSecret,
		Teleporter:   wallTeleporter,
		FloorChange:  wallFloorChange,
		CeilChange:   wallCeilChange,
		NoHeightDiff: wallNoHeightDiff,
		Unrevealed:   wallUnrevealed,
	}
)

var (
	pixelRShift, pixelGShift, pixelBShift, pixelAShift = packedPixelShifts()
	pixelOpaqueA                                       = uint32(0xFF) << pixelAShift
	pixelLittleEndian                                  = pixelRShift == 0
)

var doomPlayerArrow = [][4]float64{
	// Rough port of Doom's AM player_arrow (points right in local space).
	{-16, 0, 18.2857, 0},
	{18.2857, 0, 9.14285, 4.5714},
	{18.2857, 0, 9.14285, -4.5714},
	{-16, 0, -20.5714, 4.5714},
	{-16, 0, -20.5714, -4.5714},
	{-10.2857, 0, -16, 4.5714},
	{-10.2857, 0, -16, -4.5714},
}

var detailPresets = [][2]int{
	{doomLogicalW, doomLogicalH},         // high detail
	{doomLogicalW, doomLogicalH},         // low detail (column-doubled)
	{doomLogicalW * 2, doomLogicalH * 2}, // extra high
}

var sourcePortDetailDivisors = []int{1, 2, 3, 4}
var sourcePortHUDScaleSteps = []float64{1.0, 2.0, 3.0, 4.0, 5.0, 6.0, 7.0, 8.0}

var doomFuzzOffsets = [...]int{
	1, -1, 1, -1, 1, 1, -1,
	1, 1, -1, 1, 1, 1, -1,
	1, 1, 1, -1, -1, -1, -1,
	1, -1, -1, 1, 1, 1, 1, -1,
	1, -1, 1, 1, -1, -1, 1,
	1, -1, -1, -1, -1, 1, 1,
	1, 1, -1, 1, 1, -1, 1,
}

type projectedPuffItem struct {
	dist       float64
	sx         float64
	sy         float64
	clipSpans  []solidSpan
	clipTop    int
	clipBottom int
	spriteTex  *WallTexture
	hasSprite  bool
}

type spriteOpaqueBounds struct {
	minX int
	maxX int
	minY int
	maxY int
}

type spriteOpaqueRect uint32

func packSpriteOpaqueRect(minX, maxX, minY, maxY int) spriteOpaqueRect {
	return spriteOpaqueRect(
		uint32(uint8(minX)) |
			(uint32(uint8(maxX)) << 8) |
			(uint32(uint8(minY)) << 16) |
			(uint32(uint8(maxY)) << 24),
	)
}

func (r spriteOpaqueRect) minX() int { return int(uint8(uint32(r))) }
func (r spriteOpaqueRect) maxX() int { return int(uint8(uint32(r) >> 8)) }
func (r spriteOpaqueRect) minY() int { return int(uint8(uint32(r) >> 16)) }
func (r spriteOpaqueRect) maxY() int { return int(uint8(uint32(r) >> 24)) }

type projectedOpaqueRect uint64

func packProjectedOpaqueRect(x0, x1, y0, y1 int) projectedOpaqueRect {
	return projectedOpaqueRect(
		uint64(uint16(x0)) |
			(uint64(uint16(x1)) << 16) |
			(uint64(uint16(y0)) << 32) |
			(uint64(uint16(y1)) << 48),
	)
}

func (r projectedOpaqueRect) x0() int { return int(uint16(uint64(r))) }
func (r projectedOpaqueRect) x1() int { return int(uint16(uint64(r) >> 16)) }
func (r projectedOpaqueRect) y0() int { return int(uint16(uint64(r) >> 32)) }
func (r projectedOpaqueRect) y1() int { return int(uint16(uint64(r) >> 48)) }

type spriteOpaqueShape struct {
	bounds spriteOpaqueBounds
	rowMin []int16
	rowMax []int16
	rects  []spriteOpaqueRect
}

type spriteRenderRef struct {
	key        string
	tex        *WallTexture
	opaque     spriteOpaqueShape
	hasOpaque  bool
	fullBright bool
}

type thingAnimRefState struct {
	refs       []*spriteRenderRef
	tics       []int
	frameUnits int
	staticRef  *spriteRenderRef
}

type monsterFrameRenderEntry struct {
	ref  *spriteRenderRef
	flip bool
}

type monsterFrameRenderKey struct {
	prefix string
	frame  byte
	rot    uint8
}

type billboardQueueKind uint8

const (
	billboardQueueProjectiles billboardQueueKind = iota
	billboardQueueMonsters
	billboardQueueWorldThings
	billboardQueuePuffs
	billboardQueueMaskedMids
)

type cutoutItem struct {
	dist            float64
	depthQ          uint16
	kind            billboardQueueKind
	idx             int
	x0              int
	x1              int
	y0              int
	y1              int
	shadeMul        uint32
	doomRow         int
	tex             *WallTexture
	flip            bool
	shadow          bool
	clipSpans       []solidSpan
	clipTop         int
	clipBottom      int
	dstX            float64
	dstY            float64
	scale           float64
	opaque          spriteOpaqueShape
	hasOpaque       bool
	opaqueRectStart int
	opaqueRectCount int
	boundsOK        bool
	debugOverlay    bool
}

type billboardQueueItem = cutoutItem

type billboardPlaneOccluderSpan struct {
	L      int
	R      int
	DepthQ uint16
}

type hitscanPuff struct {
	x        int64
	y        int64
	z        int64
	momz     int64
	floorz   int64
	ceilz    int64
	lastLook int
	tics     int
	state    int
	totalTic int
	kind     uint8
	hidden   bool
	order    int64
}

const (
	hitscanFxPuff uint8 = iota
	hitscanFxBlood
	hitscanFxSmoke
	hitscanFxTeleport
)

const maskedMidSortBucket = 8.0

const spriteMagnifyMinScale = 2.0

const (
	// 360 means the blend spans the full normalized interval for that category.
	wallFloorAnimShutterAngle = 360.0
	switchTextureShutterAngle = 180.0
	weaponSpriteShutterAngle  = 360.0
)

type maskedMidSeg struct {
	scene.MaskedMidSeg
	tex              wallTextureBlendSample
	light            int16
	occlusionY0      int16
	occlusionY1      int16
	occlusionDepthQ  uint16
	hasOcclusionBBox bool
}

func wallSegPrepassProjection(pp wallSegPrepass) scene.WallProjection {
	return pp.prepass.Projection
}

func buildTexturePointerCache(bank map[string]WallTexture) ([]WallTexture, map[string]*WallTexture) {
	if len(bank) == 0 {
		return nil, nil
	}
	store := make([]WallTexture, 0, len(bank))
	ptrs := make(map[string]*WallTexture, len(bank))
	for key, tex := range bank {
		if tex.Width > 0 && tex.Height > 0 && len(tex.RGBA) == tex.Width*tex.Height*4 {
			tex.EnsureOpaqueColumnBounds()
		}
		store = append(store, tex)
		ptrs[key] = &store[len(store)-1]
	}
	return store, ptrs
}

func (g *game) allocThinkerOrder() int64 {
	if g == nil {
		return 0
	}
	if g.nextThinkerOrder <= 0 {
		g.nextThinkerOrder = 1
	}
	order := g.nextThinkerOrder
	g.nextThinkerOrder++
	return order
}

func (g *game) allocBlockmapOrder() int64 {
	if g == nil {
		return 0
	}
	if g.nextBlockmapOrder <= 0 {
		g.nextBlockmapOrder = 1
		for _, order := range g.thingBlockOrder {
			if order >= g.nextBlockmapOrder {
				g.nextBlockmapOrder = order + 1
			}
		}
	}
	order := g.nextBlockmapOrder
	g.nextBlockmapOrder++
	return order
}

func (g *game) ensureTexturePointerCaches() {
	if g == nil {
		return
	}
	if len(g.spritePatchPtrs) == 0 && len(g.opts.SpritePatchBank) != 0 {
		g.spritePatchStore, g.spritePatchPtrs = buildTexturePointerCache(g.opts.SpritePatchBank)
	}
	if len(g.wallTexPtrs) == 0 && len(g.opts.WallTexBank) != 0 {
		g.wallTexStore, g.wallTexPtrs = buildTexturePointerCache(g.opts.WallTexBank)
	}
}

func (g *game) wallDepthColumnAt(x int) scene.WallDepthColumn {
	if g == nil || x < 0 || x >= len(g.wallDepthQCol) {
		return scene.WallDepthColumn{DepthQ: 0xFFFF, Top: 1, Bottom: 0}
	}
	col := scene.WallDepthColumn{DepthQ: g.wallDepthQCol[x], Top: g.viewH, Bottom: -1}
	if x < len(g.wallDepthTopCol) {
		col.Top = g.wallDepthTopCol[x]
	}
	if x < len(g.wallDepthBottomCol) {
		col.Bottom = g.wallDepthBottomCol[x]
	}
	if x < len(g.wallDepthClosedCol) {
		col.Closed = g.wallDepthClosedCol[x]
	}
	return col
}

type game struct {
	m                 *mapdata.Map
	opts              Options
	bounds            bounds
	paletteLUTEnabled bool
	gammaLevel        int
	crtEnabled        bool
	viewW             int
	viewH             int
	screenBlocks      int
	hudScaleStep      int
	hudLogicalLayout  bool
	State             mapview.ViewState

	mode                      viewMode
	rotateView                bool
	parity                    automapParityState
	showGrid                  bool
	showLegend                bool
	bigMap                    mapview.BigMapState
	paused                    bool
	pauseMenuActive           bool
	pauseMenuMode             int
	pauseMenuItemOn           int
	pauseMenuOptionsOn        int
	pauseMenuSoundOn          int
	pauseMenuVoiceOn          int
	pauseMenuEpisodeOn        int
	pauseMenuSelectedEpisode  int
	pauseMenuSkillOn          int
	pauseMenuKeybindRow       int
	pauseMenuKeybindSlot      int
	pauseMenuKeybindCapture   bool
	pauseMenuSkullAnimCounter int
	pauseMenuWhichSkull       int
	pauseMenuStatus           string
	pauseMenuStatusTics       int
	musicPlayerRequested      bool
	frontendMenuRequested     bool
	soundMenuRequested        bool
	frontendActive            bool
	saveGameRequested         bool
	loadGameRequested         bool
	quitPromptRequested       bool
	readThisRequested         bool
	quitPromptActive          bool
	newGameRequestedMap       *mapdata.Map
	newGameRequestedSkill     int
	marks                     mapview.MarksState
	p                         player
	currentMoveCmd            moveCmd
	localSlot                 int
	localPlayerThingIndex     int
	playerBlockOrder          int64
	peerStarts                []playerStart

	lines                   []physLine
	mapVisibleLines         []mapview.Line
	lineValid               []int
	validCount              int
	playerProbeSpecialLines []int
	thingProbeSpecialLines  [][]int
	bmapOriginX             int64
	bmapOriginY             int64
	bmapWidth               int
	bmapHeight              int
	physForLine             []int
	renderSeen              []int
	renderEpoch             int
	visibleBuf              []int
	mapLineVisibility       mapview.VisibilityState
	bspOccBuf               []solidSpan
	visibleSectorSeen       []int
	visibleSubSectorSeen    []int
	visibleEpoch            int
	nodeChildRangeEpoch     []int
	nodeChildRangeL         []int
	nodeChildRangeR         []int
	nodeChildRangeOK        []uint8
	thingSectorCache        []int
	sectorLineAdj           [][]automapSectorLine
	mapLines                mapview.LineCacheState
	useSpecialSegScratch    []mapview.Segment
	sectorFloor             []int64
	sectorCeil              []int64
	lineSpecial             []uint16
	doors                   map[int]*doorThinker
	recycledDoors           map[int]*doorThinker
	floors                  map[int]*floorThinker
	plats                   map[int]*platThinker
	ceilings                map[int]*ceilingThinker
	useFlash                int
	useText                 string
	hudMessagesEnabled      bool
	chatComposeOpen         bool
	chatCompose             []rune
	chatHistory             []chatHistoryEntry
	chatSentTimes           []time.Time
	chatRecentSent          []string
	turnHeld                int
	snd                     *soundSystem
	soundQueue              []soundEvent
	soundQueueOrigin        []queuedSoundOrigin
	delayedSfx              []delayedSoundEvent
	delayedSwitchReverts    []delayedSwitchTexture
	switchTextureBlends     []switchTextureBlend

	prevPX        int64
	prevPY        int64
	prevPrevAngle uint32
	prevAngle     uint32

	renderPX    float64
	renderPY    float64
	renderAngle uint32
	renderAlpha float64
	renderStamp time.Time
	debugAimSS  int

	lastUpdate       time.Time
	fpsFrames        int
	fpsStamp         time.Time
	fpsDisplay       float64
	fpsDisplayText   string
	renderAccum      time.Duration
	renderMSAvg      float64
	renderStageAccum [renderStageCount]time.Duration
	renderStageMS    [renderStageCount]float64
	renderStageText  string
	ticRateDisplay   float64
	ticDisplayText   string
	benchLow1MS      float64
	benchLow01MS     float64
	frameUpload      time.Duration
	perfInDraw       bool
	simTickScale     float64
	simTickAccum     float64
	watchTickStamp   time.Time
	watchTickAccum   time.Duration
	edgeInputPass    bool
	pendingUse       bool
	input            gameInputSnapshot

	lastMouseX             int
	mouseInputScaleX       float64
	mouseLookSet           bool
	mouseLookSuppressTicks int

	levelExitRequested    bool
	secretLevelExit       bool
	levelRestartRequested bool

	thingCollected        []bool
	thingDropped          []bool
	thingThinkerOrder     []int64
	prevThingX            []int64
	prevThingY            []int64
	prevThingZ            []int64
	thingRenderBlendFromX []int64
	thingRenderBlendFromY []int64
	thingRenderBlendTics  []int
	thingX                []int64
	thingY                []int64
	thingMomX             []int64
	thingMomY             []int64
	thingMomZ             []int64
	thingAngleState       []uint32
	thingZState           []int64
	thingFloorState       []int64
	thingCeilState        []int64
	thingSupportValid     []bool
	thingSkullFly         []bool
	thingResumeChaseNow   []bool
	thingBlockOrder       []int64
	thingBlockCell        []int
	thingBlockCells       [][]int
	thingHP               []int
	thingAggro            []bool
	thingAmbush           []bool
	thingTargetPlayer     []bool
	thingTargetIdx        []int
	thingThreshold        []int
	thingCooldown         []int
	thingMoveDir          []monsterMoveDir
	thingMoveCount        []int
	thingJustAtk          []bool
	thingInFloat          []bool
	thingJustHit          []bool
	thingReactionTics     []int
	thingWakeTics         []int
	thingLastLook         []int
	thingDead             []bool
	thingGibbed           []bool
	thingGibTick          []int
	thingXDeath           []bool
	thingDeathTics        []int
	thingAttackTics       []int
	thingAttackPhase      []int
	thingAttackFireTics   []int
	thingPainTics         []int
	thingThinkWait        []int
	thingDoomState        []int
	thingState            []monsterThinkState
	thingStateTics        []int
	thingStatePhase       []int
	thingWorldAnimRef     []thingAnimRefState
	thingShadeTick        []int
	thingShadeMul         []uint32
	bossSpawnCubes        []bossSpawnCube
	bossSpawnFires        []bossSpawnFire
	bossBrainTargetOrder  int
	bossBrainEasyToggle   bool
	projectiles           []projectile
	projectileImpacts     []projectileImpact
	projectileShadeTick   []int
	projectileShadeMul    []uint32
	impactShadeTick       []int
	impactShadeMul        []uint32
	hitscanPuffs          []hitscanPuff
	nextThinkerOrder      int64
	nextBlockmapOrder     int64
	platFree              []*platThinker
	cheatLevel            int
	invulnerable          bool
	noClip                bool
	inventory             playerInventory
	alwaysRun             bool
	autoWeaponSwitch      bool
	weaponRefire          bool
	weaponAttackDown      bool
	useButtonDown         bool
	prevWeaponState       weaponPspriteState
	prevWeaponFlashState  weaponPspriteState
	prevWeaponPSpriteY    int
	weaponState           weaponPspriteState
	weaponStateTics       int
	weaponFlashState      weaponPspriteState
	weaponFlashTics       int
	weaponPSpriteY        int
	stats                 playerStats
	worldTic              int
	worldTicSample        int
	spectreFuzzPos        int
	spectreFuzzCoarseX    int
	spectreFuzzCoarseY    int
	spectreFuzzCoarseSet  bool
	spectreFuzzSamplePix  []uint32
	spectreFuzzSampleTic  int
	spectreFuzzSampleInit bool
	playerViewZ           int64
	secretFound           []bool
	secretsFound          int
	secretsTotal          int
	sectorSoundTarget     []bool
	isDead                bool
	playerMobjHealth      int
	damageFlashTic        int
	bonusFlashTic         int
	sectorLightFx         []sectorLightEffect
	subSectorSec          []int
	sectorBBox            []worldBBox
	subSectorLoopVerts    [][]uint16
	subSectorLoopDiag     []loopBuildDiag
	subSectorPoly         [][]worldPt
	subSectorTris         [][][3]int
	subSectorBBox         []worldBBox
	dynamicSectorMask     []bool
	staticSubSectorMask   []bool
	subSectorPlaneID      []int
	sectorSubSectors      [][]int
	holeFillPolys         []holeFillPoly
	sectorPlaneTris       [][]worldTri
	sectorPlaneCache      []sectorPlaneCacheEntry
	sectorLightCacheTick  int
	sectorLightCacheValid bool
	orphanSubSector       []bool
	orphanRepairQueue     []orphanRepairCandidate

	mapFloorLayer                *ebiten.Image
	mapFloorPix                  []byte
	mapFloorW                    int
	mapFloorH                    int
	skyLayerShader               *ebiten.Shader
	skyLayerTex                  *ebiten.Image
	skyLayerTexKey               string
	skyLayerTexW                 int
	skyLayerTexH                 int
	skyLayerFrameActive          bool
	skyLayerFrameCamAng          float64
	skyLayerFrameFocal           float64
	skyLayerFrameTexH            float64
	skyLayerVerts                [4]ebiten.Vertex
	skyLayerIdx                  [6]uint16
	skyLayerShaderOp             ebiten.DrawTrianglesShaderOptions
	skyLayerUniforms             map[string]any
	skyLayerProjDrawW            int
	skyLayerProjDrawH            int
	skyLayerProjSampleW          int
	skyLayerProjSampleH          int
	skyOutputW                   int
	skyOutputH                   int
	mapFloorWorldLayer           *ebiten.Image
	mapFloorWorldInit            bool
	mapFloorWorldMinX            float64
	mapFloorWorldMaxY            float64
	mapFloorWorldStep            float64
	mapFloorWorldStats           floorFrameStats
	mapFloorWorldState           string
	mapFloorWorldAnim            int
	mapFloorLoopSets             []sectorLoopSet
	mapFloorLoopInit             bool
	spriteOpaqueShapeCache       map[string]spriteOpaqueShape
	spriteRenderRefCache         map[string]*spriteRenderRef
	monsterFrameRenderCache      map[monsterFrameRenderKey]monsterFrameRenderEntry
	thingSpriteExpandCache       map[string][]string
	worldThingAnimRefCache       map[int16]thingAnimRefState
	spritePatchStore             []WallTexture
	spritePatchPtrs              map[string]*WallTexture
	spritePatchResolvedCache     map[string]WallTexture
	wallTexStore                 []WallTexture
	wallTexPtrs                  map[string]*WallTexture
	wallTextureAnimRefs          map[string]textureAnimRef
	flatTextureAnimRefs          map[string]textureAnimRef
	flatNameToID                 map[string]uint16
	flatIDToName                 []string
	planeFlatCache32Scratch      [][]uint32
	planeFlatCacheIndexedScratch [][]byte
	planeFBPackedScratch         []uint32
	planeFlatTex32Scratch        [][]uint32
	planeFlatTexIndexedScratch   [][]byte
	planeFlatReadyScratch        []bool
	puffItemsScratch             []projectedPuffItem
	billboardQueueScratch        []cutoutItem
	projectedOpaqueRectScratch   []projectedOpaqueRect
	billboardPlaneOccluderRows   [][]billboardPlaneOccluderSpan
	billboardQueueCollect        bool
	maskedMidSegsScratch         []maskedMidSeg
	spriteTXScratch              []int
	spriteTXRunEndScratch        []int
	spriteTYScratch              []int
	wallLayer                    *ebiten.Image
	wallPix                      []byte
	wallPix32                    []uint32
	frameSkyLayerEnabled         bool
	frameSkyTex32                []uint32
	frameSkyTexW                 int
	frameSkyColU                 []int
	frameSkyRowV                 []int
	cutoutCoverageBits           []uint64
	wallW                        int
	wallH                        int
	wallDepthQCol                []uint16
	wallDepthTopCol              []int
	wallDepthBottomCol           []int
	wallDepthClosedCol           []bool
	maskedClipCols               [][]scene.MaskedClipSpan
	maskedClipFirstDepthQ        []uint16
	maskedClipLastDepthQ         []uint16
	maskedSpanScratchA           []solidSpan
	maskedSpanScratchB           []solidSpan
	maskedSpanScratchC           []solidSpan
	cutoutSpanScratch            []solidSpan
	maskedMidShadeTick           [256]int
	maskedMidShadeKey            [256]uint16
	maskedMidShadeMulCache       [256]uint32
	maskedMidDoomRowCache        [256]int
	wallTop3D                    []int
	wallBottom3D                 []int
	ceilingClip3D                []int
	floorClip3D                  []int
	buffers3DW                   int
	buffers3DH                   int
	flatImgCache                 map[string]*ebiten.Image
	statusBarCacheImg            *ebiten.Image
	statusBarCacheState          statusBarCacheState
	statusBarCacheValid          bool
	statusPatchImg               map[string]*ebiten.Image
	menuPatchImg                 map[string]*ebiten.Image
	pauseEpisodeNamesScratch     []string
	spritePatchImg               map[string]*ebiten.Image
	messageFontImg               map[rune]*ebiten.Image
	whitePixel                   *ebiten.Image
	cullLogBudget                int
	floorDbgMode                 floorDebugMode
	floorVisDiag                 floorVisDiagMode
	floorFrame                   floorFrameStats
	floorClip                    []int16
	ceilingClip                  []int16
	floorPlanes                  map[floorPlaneKey][]*floorVisplane
	floorPlaneOrd                []*floorVisplane
	floorSpans                   []floorSpan
	detailLevel                  int
	autoDetailEnabled            bool
	autoDetailCooldown           int
	autoDetailLowSamples         int
	autoDetailHighSamples        int
	runtimeSettingsSeen          bool
	runtimeSettingsLast          RuntimeSettings
	subSectorPolySrc             []uint8
	subSectorDiagCode            []uint8
	mapTexDiagStats              mapTexDiagStats
	skyAngleOff                  []float64
	skyAngleViewW                int
	skyAngleFocal                float64
	skyColUCache                 []int
	skyColViewW                  int
	skyRowVCache                 []int
	skyRowViewH                  int
	skyRowTexH                   int
	skyRowIScale                 float64
	skyRowDrawCache              []int
	skyRowDrawH                  int
	plane3DVisBuckets            map[plane3DKey]plane3DVisBucket
	plane3DVisGen                uint64
	plane3DOrder                 []*plane3DVisplane
	plane3DSpanScratch           [][]plane3DSpan
	plane3DSpanStartScratch      [][]int
	plane3DSpanWorkScratch       [][]planeSpanWorkItem
	plane3DPool                  []*plane3DVisplane
	plane3DPoolUsed              int
	plane3DPoolViewW             int
	wallSegStaticCache           []wallSegStatic
	wallPrepassBuf               []wallSegPrepass
	solid3DBuf                   []solidSpan
	solidClipScratch             []solidSpan
	losInterceptScratch          []intercept
	automapMappedScratch         []bool
	automapVisitedScratch        []bool
	automapQueueScratch          []automapQueueNode
	debugPlayerProbeEnabled      bool
	debugPlayerProbeTic          int
	platTickedThisTic            bool
	demoTick                     int
	demoDoneReported             bool
	demoBenchStarted             bool
	demoTraceInitialWritten      bool
	statusFaceIndex              int
	statusFaceCount              int
	statusFacePriority           int
	statusOldHealth              int
	statusRandom                 int
	statusLastAttack             int
	statusAttackDown             bool
	statusAttackerX              int64
	statusAttackerY              int64
	statusAttackerThing          int
	statusHasAttacker            bool
	statusOldWeapons             [9]bool
	statusDamageCount            int
	statusBonusCount             int
	playerMobjState              int
	playerMobjTics               int
	demoBenchStart               time.Time
	demoBenchDraws               int
	demoBenchFrameNS             []int64
	demoStartRnd                 int
	demoStartPRnd                int
	demoRNGCaptured              bool
	demoTrace                    *demoTraceWriter
	demoRecord                   []DemoTic
	demoWeaponSlot               int // weapon slot key pressed this tic (1-based, 0 = none); consumed by recordDemoTic
	typedCheatBuffer             string
}

type gameInputSnapshot struct {
	pressedKeys             map[ebiten.Key]struct{}
	justPressedKeys         map[ebiten.Key]struct{}
	pressedMouseButtons     map[ebiten.MouseButton]struct{}
	justPressedMouseButtons map[ebiten.MouseButton]struct{}
	inputChars              []rune
	wheelY                  float64
	cursorX                 int
	mouseTurnRawAccum       int64
}

type savedMapView struct {
	camX   float64
	camY   float64
	zoom   float64
	follow bool
	valid  bool
}

type delayedSoundEvent struct {
	ev         soundEvent
	tics       int
	x          int64
	y          int64
	positioned bool
	monsterTyp int16
	deathSound bool
}

type queuedSoundOrigin struct {
	x          int64
	y          int64
	positioned bool
}

type delayedSwitchTexture struct {
	line    int
	sidedef int
	top     string
	bottom  string
	mid     string
	tics    int
}

type switchTextureSlot uint8

const (
	switchTextureSlotTop switchTextureSlot = iota
	switchTextureSlotBottom
	switchTextureSlotMid
)

type switchTextureBlend struct {
	sidedef  int
	slot     switchTextureSlot
	from     string
	to       string
	startTic int
}

type textureBlendSample struct {
	fromKey string
	toKey   string
	alpha   uint8
}

type wallTextureBlendSample struct {
	from  *WallTexture
	to    *WallTexture
	alpha uint8
}

type flatTextureBlendSample struct {
	fromRGBA    []byte
	toRGBA      []byte
	fromIndexed []byte
	toIndexed   []byte
	alpha       uint8
}

type automapQueueNode struct {
	sec   int
	depth int
}

type automapSectorLine struct {
	line    int
	front   int
	back    int
	frontOK bool
	backOK  bool
}

type revealMode int

const (
	revealNormal revealMode = iota
	revealAllMap
)

const debugFixedSubsector = 28

type automapParityState struct {
	reveal revealMode
	iddt   int
}

type sourcePortThingRenderMode string

const (
	sourcePortThingRenderGlyphs  sourcePortThingRenderMode = "glyphs"
	sourcePortThingRenderItems   sourcePortThingRenderMode = "items"
	sourcePortThingRenderSprites sourcePortThingRenderMode = "sprites"
)

func normalizeSourcePortThingRenderMode(v string, sourcePort bool) string {
	mode := sourcePortThingRenderMode(strings.ToLower(strings.TrimSpace(v)))
	switch mode {
	case sourcePortThingRenderGlyphs, sourcePortThingRenderItems, sourcePortThingRenderSprites:
		return string(mode)
	default:
		if sourcePort {
			return string(sourcePortThingRenderItems)
		}
		return string(sourcePortThingRenderGlyphs)
	}
}

func normalizeSkyUpscaleMode(v string, sourcePort bool) string {
	if !sourcePort {
		return "nearest"
	}
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "":
		return "sharp"
	case "nearest":
		return "nearest"
	case "sharp", "bicubic":
		return "sharp"
	default:
		return "sharp"
	}
}

func cycleSourcePortThingRenderMode(v string) string {
	switch sourcePortThingRenderMode(normalizeSourcePortThingRenderMode(v, true)) {
	case sourcePortThingRenderGlyphs:
		return string(sourcePortThingRenderItems)
	case sourcePortThingRenderItems:
		return string(sourcePortThingRenderSprites)
	default:
		return string(sourcePortThingRenderGlyphs)
	}
}

func (g *game) playerInvulnerable() bool {
	return g != nil && (g.invulnerable || g.inventory.InvulnTics > 0)
}

func (g *game) playerPowerupInvulnerabilityVisualActive() bool {
	if g == nil {
		return false
	}
	tics := g.inventory.InvulnTics
	return tics > 4*32 || (tics&8) != 0
}

func (g *game) playerInvisible() bool {
	return g != nil && g.inventory.InvisTics > 0
}

func (g *game) automapRevealAll() bool {
	return g != nil && (g.parity.reveal == revealAllMap || g.inventory.AllMap)
}

func (g *game) playerInfraredBright() bool {
	if g == nil {
		return false
	}
	tics := g.inventory.LightAmpTics
	return tics > 4*32 || (tics&8) != 0
}

func (g *game) playerFixedColormapRow() (int, bool) {
	if g == nil {
		return 0, false
	}
	active := g.playerPowerupInvulnerabilityVisualActive()
	if !active || doomColormapRowCount() <= doomNumColorMaps {
		return 0, false
	}
	return doomNumColorMaps, true
}

func sourcePortThingRenderModeLabel(v string) string {
	switch sourcePortThingRenderMode(normalizeSourcePortThingRenderMode(v, true)) {
	case sourcePortThingRenderItems:
		return "ITEM SPRITES"
	case sourcePortThingRenderSprites:
		return "ALL SPRITES"
	default:
		return "GLYPHS"
	}
}

type floorDebugMode int

const (
	floorDebugTextured floorDebugMode = iota
	floorDebugSolid
	floorDebugUV
)

type floorVisDiagMode int

const (
	floorVisDiagOff floorVisDiagMode = iota
	floorVisDiagClip
	floorVisDiagSpan
	floorVisDiagBoth
)

type floorFrameStats struct {
	markedCols       int
	emittedSpans     int
	rejectedSpan     int
	rejectNoSector   int
	rejectNoPoly     int
	rejectDegenerate int
	rejectSpanClip   int
}

type mapTexDiagStats struct {
	ok              int
	segShort        int
	noPoly          int
	nonSimple       int
	triFail         int
	orphan          int
	loopMultiNext   int
	loopDeadEnd     int
	loopEarlyClose  int
	loopNoClose     int
	nonConvex       int
	degenerateArea  int
	triAreaMismatch int
}

type wallSegPrepass struct {
	segIdx          int
	ld              mapdata.Linedef
	frontSideDefIdx int
	prepass         scene.WallPrepass
}

type wallSegStatic struct {
	valid             bool
	ld                mapdata.Linedef
	frontSide         int
	frontSideDefIdx   int
	frontSectorIdx    int
	backSectorIdx     int
	input             scene.WallPrepassWorldInput
	hasTwoSidedMidTex bool
	portalSplitStatic bool
	portalSplit       bool
	lightTickValid    bool
	lightTick         int
	lightTickSplit    bool
}

type worldTri struct {
	a worldPt
	b worldPt
	c worldPt
}

type sectorPlaneCacheEntry struct {
	tris         []worldTri
	dynamic      bool
	lastFloor    int64
	lastCeil     int64
	dirty        bool
	lightKind    sectorLightEffectKind
	prevLight    int16
	light        int16
	prevLightMul uint8
	lightMul     uint8
	texID        string
	tex          *ebiten.Image
	flatRGBA     []byte
	texTick      int
}

type renderStage int

const (
	renderStageWallPrepass renderStage = iota
	renderStageWallTraverse
	renderStagePlaneSpans
	renderStagePlaneRaster
	renderStageBillboards
	renderStageCount
)

var renderStageLabels = [...]string{"wp", "wt", "ps", "pr", "bb"}

type orphanRepairCandidate struct {
	ss    int
	sec   int
	votes int
}

type plane3DVisBucket struct {
	gen  uint64
	list []*plane3DVisplane
}

const (
	subPolySrcNone uint8 = iota
	subPolySrcPrebuiltLoop
	subPolySrcEdgeChain
	subPolySrcWorld
	subPolySrcConvex
	subPolySrcSegList
	subPolySrcNodes
)

const sourcePortThingAnimSubsteps = 5

const (
	subDiagOK uint8 = iota
	subDiagSegShort
	subDiagNoPoly
	subDiagNonSimple
	subDiagTriFail
	subDiagLoopMultiNext
	subDiagLoopDeadEnd
	subDiagLoopEarlyClose
	subDiagLoopNoClose
	subDiagNonConvex
	subDiagDegenerateArea
	subDiagTriAreaMismatch
)

type loopBuildDiag uint8

const (
	loopDiagOK loopBuildDiag = iota
	loopDiagMultipleCandidates
	loopDiagDeadEnd
	loopDiagEarlyClose
	loopDiagNoClose
)

func newGame(m *mapdata.Map, opts Options) *game {
	// Doom clears both random streams in G_InitNew before setting up a fresh
	// level, including demo playback. Without this, hidden bootstrap/frontend
	// builds leak prior RNG state into attract demos and other new sessions.
	loadRuntimeDebugEnvFromOS()
	doomrand.LoadDebugEnvFromOS()
	doomrand.Clear()
	if opts.DemoScript != nil {
		opts = runtimecfg.PrepareDemoPlaybackOptions(opts, opts.DemoScript)
	}

	ensurePositiveRenderSize(&opts)
	opts.SkillLevel = normalizeSkillLevel(opts.SkillLevel)
	opts.GameMode = normalizeGameMode(opts.GameMode)
	opts.MouseLookSpeed = normalizeMouseLookSpeed(opts.MouseLookSpeed)
	opts.KeyboardTurnSpeed = normalizeKeyboardTurnSpeed(opts.KeyboardTurnSpeed)
	opts.MusicVolume = clampVolume(opts.MusicVolume)
	opts.SFXVolume = clampVolume(opts.SFXVolume)
	opts.OPLVolume = clampOPLVolume(opts.OPLVolume)
	opts.InputBindings = runtimecfg.NormalizeInputBindings(opts.InputBindings)
	opts.SourcePortThingRenderMode = normalizeSourcePortThingRenderMode(opts.SourcePortThingRenderMode, opts.SourcePortMode)
	opts.SkyUpscaleMode = normalizeSkyUpscaleMode(opts.SkyUpscaleMode, opts.SourcePortMode)
	if opts.PlayerSlot < 1 || opts.PlayerSlot > 4 {
		opts.PlayerSlot = 1
	}
	p, localSlot, starts, localPlayerThingIndex := spawnPlayer(m, opts.PlayerSlot)
	g := &game{
		m:                 m,
		opts:              opts,
		bounds:            mapBounds(m),
		paletteLUTEnabled: !opts.SourcePortMode,
		gammaLevel:        defaultGammaLevel,
		crtEnabled:        opts.CRTEffect,
		viewW:             opts.Width,
		viewH:             opts.Height,
		screenBlocks:      defaultScreenBlocks(opts),
		hudScaleStep:      defaultHUDScaleStep(opts),
		skyOutputW:        max(opts.Width, 1),
		skyOutputH:        max(opts.Height, 1),
		mode:              viewMap,
		State:             mapview.ViewState{FollowMode: true},
		rotateView:        opts.SourcePortMode,
		parity: automapParityState{
			reveal: revealNormal,
			iddt:   0,
		},
		showGrid:              false,
		showLegend:            opts.SourcePortMode,
		hudMessagesEnabled:    true,
		marks:                 mapview.NewMarksState(10),
		p:                     p,
		localSlot:             localSlot,
		localPlayerThingIndex: localPlayerThingIndex,
		playerBlockOrder:      int64(localPlayerThingIndex + 1),
		peerStarts:            nonLocalStarts(starts, localSlot),
		cullLogBudget:         0,
		floorDbgMode:          floorDebugTextured,
		floorVisDiag:          floorVisDiagOff,
		alwaysRun:             opts.AlwaysRun,
		autoWeaponSwitch:      opts.AutoWeaponSwitch,
		simTickScale:          1.0,
	}
	// Sourceport mode keeps Doom distance-light math without colormap remap.
	// Sector-light contribution can be toggled separately for sourceport mode.
	initDoomColormapShading(opts.DoomPaletteRGBA, opts.DoomColorMap, opts.DoomColorMapRows, !opts.SourcePortMode)
	initWallShadePackedLUT(opts.DoomPaletteRGBA)
	doomSectorLighting = !opts.SourcePortMode || opts.SourcePortSectorLighting
	if opts.DisableDoomLighting {
		disableDoomLighting()
	}
	g.plane3DVisBuckets = make(map[plane3DKey]plane3DVisBucket, 64)
	g.plane3DOrder = make([]*plane3DVisplane, 0, 64)
	g.thingSpriteExpandCache = make(map[string][]string, 256)
	g.spriteRenderRefCache = make(map[string]*spriteRenderRef, 256)
	g.monsterFrameRenderCache = make(map[monsterFrameRenderKey]monsterFrameRenderEntry, 512)
	g.worldThingAnimRefCache = make(map[int16]thingAnimRefState, 128)
	g.spritePatchStore, g.spritePatchPtrs = buildTexturePointerCache(opts.SpritePatchBank)
	g.spritePatchResolvedCache = make(map[string]WallTexture, 256)
	g.wallTexStore, g.wallTexPtrs = buildTexturePointerCache(opts.WallTexBank)
	g.wallTextureAnimRefs = buildTextureAnimRefsFromSequences(opts.WallTextureAnimSequences)
	if len(g.wallTextureAnimRefs) == 0 {
		g.wallTextureAnimRefs = defaultWallTextureAnimRefs
	}
	g.flatTextureAnimRefs = buildTextureAnimRefsFromSequences(opts.FlatTextureAnimSequences)
	if len(g.flatTextureAnimRefs) == 0 {
		g.flatTextureAnimRefs = defaultFlatTextureAnimRefs
	}
	g.detailLevel = defaultDetailLevelForMode(g.viewW, g.viewH, opts.SourcePortMode)
	if opts.InitialDetailLevel >= 0 {
		g.detailLevel = clampDetailLevelForMode(opts.InitialDetailLevel, opts.SourcePortMode)
	}
	g.autoDetailEnabled = opts.AutoDetail
	if opts.InitialGammaLevel > 0 {
		g.gammaLevel = clampGamma(opts.InitialGammaLevel)
	}
	g.setGammaLevel(g.gammaLevel)
	g.initPlayerState()
	g.bringUpWeapon()
	g.initStatusFaceState()
	g.thingCollected = make([]bool, len(m.Things))
	g.thingDropped = make([]bool, len(m.Things))
	g.thingThinkerOrder = make([]int64, len(m.Things))
	g.thingX = make([]int64, len(m.Things))
	g.thingY = make([]int64, len(m.Things))
	g.thingMomX = make([]int64, len(m.Things))
	g.thingMomY = make([]int64, len(m.Things))
	g.thingMomZ = make([]int64, len(m.Things))
	g.thingAngleState = make([]uint32, len(m.Things))
	g.thingZState = make([]int64, len(m.Things))
	g.thingFloorState = make([]int64, len(m.Things))
	g.thingCeilState = make([]int64, len(m.Things))
	g.thingSupportValid = make([]bool, len(m.Things))
	g.thingSkullFly = make([]bool, len(m.Things))
	g.thingResumeChaseNow = make([]bool, len(m.Things))
	g.thingBlockOrder = make([]int64, len(m.Things))
	g.thingBlockCell = make([]int, len(m.Things))
	g.thingHP = make([]int, len(m.Things))
	g.thingAggro = make([]bool, len(m.Things))
	g.thingAmbush = make([]bool, len(m.Things))
	g.thingTargetPlayer = make([]bool, len(m.Things))
	g.thingTargetIdx = make([]int, len(m.Things))
	for i := range g.thingTargetIdx {
		g.thingTargetIdx[i] = -1
	}
	g.thingCooldown = make([]int, len(m.Things))
	g.thingMoveDir = make([]monsterMoveDir, len(m.Things))
	g.thingMoveCount = make([]int, len(m.Things))
	g.thingJustAtk = make([]bool, len(m.Things))
	g.thingInFloat = make([]bool, len(m.Things))
	g.thingJustHit = make([]bool, len(m.Things))
	g.thingReactionTics = make([]int, len(m.Things))
	g.thingWakeTics = make([]int, len(m.Things))
	g.thingLastLook = make([]int, len(m.Things))
	g.thingDead = make([]bool, len(m.Things))
	g.thingGibbed = make([]bool, len(m.Things))
	g.thingGibTick = make([]int, len(m.Things))
	for i := range g.thingGibTick {
		g.thingGibTick[i] = -1
	}
	g.thingXDeath = make([]bool, len(m.Things))
	g.thingDeathTics = make([]int, len(m.Things))
	g.thingAttackTics = make([]int, len(m.Things))
	g.thingAttackPhase = make([]int, len(m.Things))
	g.thingAttackFireTics = make([]int, len(m.Things))
	for i := range g.thingAttackFireTics {
		g.thingAttackFireTics[i] = -1
	}
	g.thingPainTics = make([]int, len(m.Things))
	g.thingThinkWait = make([]int, len(m.Things))
	g.thingDoomState = make([]int, len(m.Things))
	for i := range g.thingDoomState {
		g.thingDoomState[i] = -1
	}
	g.thingState = make([]monsterThinkState, len(m.Things))
	g.thingStateTics = make([]int, len(m.Things))
	g.thingStatePhase = make([]int, len(m.Things))
	g.thingWorldAnimRef = make([]thingAnimRefState, len(m.Things))
	for i := range g.thingThinkerOrder {
		g.thingThinkerOrder[i] = int64(i + 1)
		g.thingBlockOrder[i] = int64(i + 1)
	}
	g.nextThinkerOrder = int64(len(m.Things) + 1)
	g.nextBlockmapOrder = int64(len(m.Things) + 1)
	g.secretFound = make([]bool, len(m.Sectors))
	g.sectorSoundTarget = make([]bool, len(m.Sectors))
	for _, sec := range m.Sectors {
		if sec.Special == 9 {
			g.secretsTotal++
		}
	}
	g.applyThingSpawnFiltering()
	g.initThingCombatState()
	g.initThingRenderState()
	g.initSectorLightEffects()
	g.cheatLevel = normalizeCheatLevel(opts.CheatLevel)
	g.invulnerable = opts.Invulnerable
	g.mode = viewWalk
	if g.opts.DemoScript != nil {
		// Demo benchmark mode is intentionally isolated from interactive controls.
		g.mode = viewWalk
		g.State.SetFollowMode(true)
		g.rotateView = false
	}
	if strings.TrimSpace(g.opts.RecordDemoPath) != "" {
		g.demoRecord = make([]DemoTic, 0, 4096)
	}
	g.initPhysics()
	// Initialize eye height after physics snaps player Z/floor/ceiling.
	// This avoids one-frame low-camera artifacts (e.g. during level melt)
	// before the first tickWorldLogic() view-height update runs.
	g.playerViewZ = g.p.z + g.p.viewHeight
	g.demoTrace = newDemoTraceWriter(opts, string(m.Name))
	g.initSubSectorSectorCache()
	g.snd = newSoundSystem(opts.SoundBank, opts.SFXVolume, sourcePortAudioEnabled(opts), opts.SFXPitchShift)
	g.soundQueue = make([]soundEvent, 0, 8)
	g.soundQueueOrigin = make([]queuedSoundOrigin, 0, 8)
	g.delayedSfx = make([]delayedSoundEvent, 0, 8)
	g.delayedSwitchReverts = make([]delayedSwitchTexture, 0, 4)
	g.switchTextureBlends = make([]switchTextureBlend, 0, 4)
	if g.opts.AllCheats {
		// Backward compatible legacy switch.
		if g.cheatLevel < 3 {
			g.cheatLevel = 3
		}
		g.invulnerable = true
	}
	g.applyCheatLevel(g.cheatLevel, false)
	if g.invulnerable {
		g.setHUDMessage("IDDQD ON", 70)
	}
	if want := strings.TrimSpace(runtimeDebugEnv("GD_DEBUG_PLAYER_PROBE_TIC")); want != "" {
		if tic, err := strconv.Atoi(want); err == nil {
			g.debugPlayerProbeEnabled = true
			g.debugPlayerProbeTic = tic
		}
	}
	g.physForLine = make([]int, len(g.m.Linedefs))
	for i := range g.physForLine {
		g.physForLine[i] = -1
	}
	for i, pl := range g.lines {
		if pl.idx >= 0 && pl.idx < len(g.physForLine) {
			g.physForLine[pl.idx] = i
		}
	}
	g.sectorLineAdj = buildAutomapSectorLineAdj(g.m)
	g.renderSeen = make([]int, len(g.m.Linedefs))
	g.visibleSectorSeen = make([]int, len(g.m.Sectors))
	g.visibleSubSectorSeen = make([]int, len(g.m.SubSectors))
	g.nodeChildRangeEpoch = make([]int, len(g.m.Nodes)*2)
	g.nodeChildRangeL = make([]int, len(g.m.Nodes)*2)
	g.nodeChildRangeR = make([]int, len(g.m.Nodes)*2)
	g.nodeChildRangeOK = make([]uint8, len(g.m.Nodes)*2)
	g.thingSectorCache = make([]int, len(g.m.Things))
	for i := range g.thingSectorCache {
		th := g.m.Things[i]
		g.thingAmbush[i] = int(th.Flags)&thingFlagAmbush != 0
		g.thingX[i] = int64(th.X) << fracBits
		g.thingY[i] = int64(th.Y) << fracBits
		g.thingAngleState[i] = thingSpawnAngle(th.Angle)
		g.thingSectorCache[i] = g.sectorAt(g.thingX[i], g.thingY[i])
		sec := g.thingSectorCache[i]
		if sec >= 0 && sec < len(g.sectorFloor) {
			g.thingFloorState[i] = g.sectorFloor[sec]
		}
		if sec >= 0 && sec < len(g.sectorCeil) {
			g.thingCeilState[i] = g.sectorCeil[sec]
		}
		g.thingZState[i] = g.thingFloorState[i]
		if thingSpawnsOnCeiling(th.Type) && sec >= 0 && sec < len(g.sectorCeil) {
			if info, ok := demoTraceThingInfoForType(th.Type); ok {
				g.thingZState[i] = g.thingCeilState[i] - info.height
			}
		}
		if sec >= 0 {
			g.thingSupportValid[i] = true
		}
	}
	if g.bmapWidth > 0 && g.bmapHeight > 0 {
		g.thingBlockCells = make([][]int, g.bmapWidth*g.bmapHeight)
		g.rebuildThingBlockmap()
	}
	g.discoverLinesAroundPlayer()
	g.resetView()
	if opts.StartZoom > 0 {
		g.State.SetZoom(opts.StartZoom)
	}
	g.reserveRenderScratch()
	if !(opts.DemoScript != nil && opts.DemoQuitOnComplete) {
		g.precacheRenderAssets()
	}
	g.syncRenderState()
	if g.mode == viewWalk {
		// Avoid startup cursor-capture deltas rotating the initial spawn heading.
		g.mouseLookSet = false
		g.mouseLookSuppressTicks = detailMouseSuppressTicks
	}
	g.fpsDisplayText = formatFPSDisplay(g.fpsDisplay, g.renderMSAvg)
	g.ticDisplayText = formatTicDisplay(g.worldTic, g.ticRateDisplay)
	g.runtimeSettingsSeen = true
	g.runtimeSettingsLast = g.runtimeSettingsSnapshot()
	return g
}

func (g *game) debugImageAlloc(tag string, w, h int) {
	if g == nil {
		return
	}
	want := strings.TrimSpace(runtimeDebugEnv("GD_DEBUG_IMAGE_ALLOC_RANGE"))
	if want == "" {
		return
	}
	start := 0
	end := 0
	if _, err := fmt.Sscanf(want, "%d:%d", &start, &end); err != nil {
		if _, err := fmt.Sscanf(want, "%d", &start); err != nil {
			return
		}
		end = start
	}
	if end < start {
		start, end = end, start
	}
	tic := g.worldTic
	if g.demoTick > tic {
		tic = g.demoTick
	}
	if tic < start || tic > end {
		return
	}
	fmt.Printf("image-alloc tag=%s world_tic=%d demo_tic=%d size=%dx%d\n", tag, g.worldTic, g.demoTick, w, h)
}

func (g *game) clearSpritePatchCache() {
	if g == nil {
		return
	}
	g.spritePatchResolvedCache = nil
	g.spritePatchImg = nil
	g.statusPatchImg = nil
	g.messageFontImg = nil
}

func reserveSliceCap[T any](buf []T, n int) []T {
	if n <= 0 {
		return buf[:0]
	}
	if cap(buf) < n {
		return make([]T, 0, n)
	}
	return buf[:0]
}

func resizeSliceLen[T any](buf []T, n int) []T {
	if n <= 0 {
		return buf[:0]
	}
	if cap(buf) < n {
		return make([]T, n)
	}
	buf = buf[:n]
	clear(buf)
	return buf
}

func resizeNestedSliceCap[T any](buf [][]T, n, innerCap int) [][]T {
	buf = resizeSliceLen(buf, n)
	if innerCap <= 0 {
		return buf
	}
	for i := range buf {
		buf[i] = reserveSliceCap(buf[i], innerCap)
	}
	return buf
}

func (g *game) reserveRenderScratch() {
	if g == nil || g.m == nil {
		return
	}
	monsterCount := 0
	otherThingCount := 0
	for _, th := range g.m.Things {
		switch {
		case isMonster(th.Type):
			monsterCount++
		case th.Type == 1 || th.Type == 2 || th.Type == 3 || th.Type == 4:
			// Player starts do not participate in billboard rendering.
		default:
			otherThingCount++
		}
	}
	wallSegCap := max(len(g.m.Segs), 64)
	billboardCap := max(monsterCount+otherThingCount+32, 64)
	rowSpanCap := max(g.viewW/2, 64)
	rowOccluderCap := max(min(billboardCap/8, 16), 4)
	planeSpanCap := max(min(g.viewW/8, 128), 32)
	sectorCap := max(len(g.m.Sectors), 64)

	g.spriteTXScratch = reserveSliceCap(g.spriteTXScratch, max(g.viewW, 64))
	g.spriteTYScratch = reserveSliceCap(g.spriteTYScratch, max(g.viewH, 64))
	g.planeFBPackedScratch = reserveSliceCap(g.planeFBPackedScratch, sectorCap)
	g.planeFlatTex32Scratch = reserveSliceCap(g.planeFlatTex32Scratch, sectorCap)
	g.planeFlatTexIndexedScratch = reserveSliceCap(g.planeFlatTexIndexedScratch, sectorCap)
	g.planeFlatReadyScratch = reserveSliceCap(g.planeFlatReadyScratch, sectorCap)
	g.plane3DSpanScratch = resizeNestedSliceCap(g.plane3DSpanScratch, sectorCap, planeSpanCap)
	g.plane3DSpanStartScratch = resizeNestedSliceCap(g.plane3DSpanStartScratch, sectorCap, max(g.viewH, 1))
	g.plane3DSpanScratch = g.plane3DSpanScratch[:0]
	g.plane3DSpanStartScratch = g.plane3DSpanStartScratch[:0]
	g.puffItemsScratch = reserveSliceCap(g.puffItemsScratch, 64)
	g.billboardQueueScratch = reserveSliceCap(g.billboardQueueScratch, billboardCap)
	g.maskedMidSegsScratch = reserveSliceCap(g.maskedMidSegsScratch, wallSegCap/2)
	g.wallPrepassBuf = reserveSliceCap(g.wallPrepassBuf, wallSegCap)
	g.solid3DBuf = reserveSliceCap(g.solid3DBuf, rowSpanCap)
	g.solidClipScratch = reserveSliceCap(g.solidClipScratch, rowSpanCap)
	g.losInterceptScratch = reserveSliceCap(g.losInterceptScratch, 16)
	g.automapMappedScratch = reserveSliceCap(g.automapMappedScratch, max(len(g.m.Linedefs), 1))
	g.automapVisitedScratch = reserveSliceCap(g.automapVisitedScratch, max(len(g.m.Sectors), 1))
	g.automapQueueScratch = reserveSliceCap(g.automapQueueScratch, max(len(g.m.Sectors), 1))

	if g.viewH > 0 {
		if len(g.billboardPlaneOccluderRows) != g.viewH {
			g.billboardPlaneOccluderRows = make([][]billboardPlaneOccluderSpan, g.viewH)
		}
		for y := range g.billboardPlaneOccluderRows {
			g.billboardPlaneOccluderRows[y] = reserveSliceCap(g.billboardPlaneOccluderRows[y], rowOccluderCap)
		}
	}
}

func (g *game) precacheRenderAssets() {
	if g == nil {
		return
	}
	g.precacheMapTextureAssets()
	g.precacheSpritePatchRenderData()
	g.precacheThingSpriteExpansion()
	g.precacheWeaponSpritePatches()
	g.precacheMonsterSpriteRefs()
	g.precacheWorldThingSpriteRefs()
	g.precacheProjectileSpriteRefs()
}

func (g *game) precacheRenderAssetsWASM() {
	if g == nil {
		return
	}
	flatKeys, _ := g.collectMapTextureUsage()
	g.initFlatIDTable(flatKeys)
	if len(g.sectorPlaneCache) == len(g.m.Sectors) && len(g.m.Sectors) != 0 {
		g.refreshSectorPlaneCacheTextureRefs()
	}
}

func collectAnimatedTextureFrames(name string, refs map[string]textureAnimRef) []string {
	key := normalizeFlatName(name)
	if key == "" {
		return nil
	}
	ref, ok := refs[key]
	if !ok || len(ref.frames) < 2 {
		return []string{key}
	}
	return ref.frames
}

func appendUniqueTextureKeys(dst []string, seen map[string]struct{}, refs map[string]textureAnimRef, name string) []string {
	for _, key := range collectAnimatedTextureFrames(name, refs) {
		if key == "" {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		dst = append(dst, key)
	}
	return dst
}

func (g *game) collectMapTextureUsage() ([]string, []string) {
	if g == nil || g.m == nil {
		return nil, nil
	}
	flatSeen := make(map[string]struct{}, len(g.m.Sectors)*2)
	wallSeen := make(map[string]struct{}, len(g.m.Sidedefs)*3)
	flatKeys := make([]string, 0, len(flatSeen))
	wallKeys := make([]string, 0, len(wallSeen))
	for _, sec := range g.m.Sectors {
		flatKeys = appendUniqueTextureKeys(flatKeys, flatSeen, g.flatTextureAnimRefs, sec.FloorPic)
		flatKeys = appendUniqueTextureKeys(flatKeys, flatSeen, g.flatTextureAnimRefs, sec.CeilingPic)
	}
	for _, side := range g.m.Sidedefs {
		wallKeys = appendUniqueTextureKeys(wallKeys, wallSeen, g.wallTextureAnimRefs, side.Mid)
		wallKeys = appendUniqueTextureKeys(wallKeys, wallSeen, g.wallTextureAnimRefs, side.Top)
		wallKeys = appendUniqueTextureKeys(wallKeys, wallSeen, g.wallTextureAnimRefs, side.Bottom)
	}
	return flatKeys, wallKeys
}

func (g *game) precacheMapTextureAssets() {
	if g == nil || g.m == nil {
		return
	}
	flatKeys, _ := g.collectMapTextureUsage()
	g.initFlatIDTable(flatKeys)
	if len(flatKeys) != 0 {
		if g.flatImgCache == nil {
			g.flatImgCache = make(map[string]*ebiten.Image, len(flatKeys))
		}
		for _, key := range flatKeys {
			g.flatRGBAResolvedKey(key)
			g.flatIndexedResolvedKey(key)
			g.flatImageResolvedKey(key)
		}
	}
	if len(g.sectorPlaneCache) == len(g.m.Sectors) && len(g.m.Sectors) != 0 {
		g.refreshSectorPlaneCacheTextureRefs()
	}
}

func (g *game) initFlatIDTable(flatKeys []string) {
	if g == nil {
		return
	}
	if g.flatNameToID == nil {
		g.flatNameToID = make(map[string]uint16, max(len(flatKeys), 16))
	} else {
		clear(g.flatNameToID)
	}
	g.flatIDToName = g.flatIDToName[:0]
	for _, key := range flatKeys {
		if key == "" {
			continue
		}
		if _, ok := g.flatNameToID[key]; ok {
			continue
		}
		if len(g.flatIDToName) >= 1<<16 {
			break
		}
		id := uint16(len(g.flatIDToName))
		g.flatNameToID[key] = id
		g.flatIDToName = append(g.flatIDToName, key)
	}
	g.planeFlatCache32Scratch = resizeSliceLen(g.planeFlatCache32Scratch, len(g.flatIDToName))
	g.planeFlatCacheIndexedScratch = resizeSliceLen(g.planeFlatCacheIndexedScratch, len(g.flatIDToName))
}

func (g *game) flatIDForResolvedName(name string) uint16 {
	if g == nil || name == "" {
		return 0
	}
	if id, ok := g.flatNameToID[name]; ok {
		return id
	}
	if g.flatNameToID == nil {
		g.flatNameToID = make(map[string]uint16, 16)
	}
	if len(g.flatIDToName) >= 1<<16 {
		return 0
	}
	id := uint16(len(g.flatIDToName))
	g.flatNameToID[name] = id
	g.flatIDToName = append(g.flatIDToName, name)
	if len(g.planeFlatCache32Scratch) < len(g.flatIDToName) {
		g.planeFlatCache32Scratch = append(g.planeFlatCache32Scratch, nil)
		g.planeFlatCacheIndexedScratch = append(g.planeFlatCacheIndexedScratch, nil)
	}
	return id
}

func (g *game) flatNameByID(id uint16) string {
	if g == nil {
		return ""
	}
	i := int(id)
	if i < 0 || i >= len(g.flatIDToName) {
		return ""
	}
	return g.flatIDToName[i]
}

func (g *game) flatIDForName(name string) uint16 {
	return g.flatIDForResolvedName(normalizeFlatName(name))
}

func (g *game) flatRGBAResolvedID(id uint16) ([]byte, bool) {
	return g.flatRGBAResolvedKey(g.flatNameByID(id))
}

func (g *game) flatIndexedResolvedID(id uint16) ([]byte, bool) {
	return g.flatIndexedResolvedKey(g.flatNameByID(id))
}

func (g *game) beginSwitchTextureBlend(sidedef int, slot switchTextureSlot, from, to string) {
	if g == nil || !g.opts.SourcePortMode || g.opts.TextureAnimCrossfadeFrames <= 0 {
		return
	}
	from = normalizeFlatName(from)
	to = normalizeFlatName(to)
	if from == "" || to == "" || from == to {
		return
	}
	for i := range g.switchTextureBlends {
		if g.switchTextureBlends[i].sidedef != sidedef || g.switchTextureBlends[i].slot != slot {
			continue
		}
		g.switchTextureBlends[i].from = from
		g.switchTextureBlends[i].to = to
		g.switchTextureBlends[i].startTic = g.worldTic
		return
	}
	g.switchTextureBlends = append(g.switchTextureBlends, switchTextureBlend{
		sidedef:  sidedef,
		slot:     slot,
		from:     from,
		to:       to,
		startTic: g.worldTic,
	})
}

func (g *game) switchTextureBlendFor(sidedef int, slot switchTextureSlot, current string) textureBlendSample {
	if g == nil || !g.opts.SourcePortMode || g.opts.TextureAnimCrossfadeFrames <= 0 || sidedef < 0 {
		return textureBlendSample{}
	}
	current = normalizeFlatName(current)
	if current == "" {
		return textureBlendSample{}
	}
	frames := g.opts.TextureAnimCrossfadeFrames
	for i := range g.switchTextureBlends {
		blend := g.switchTextureBlends[i]
		if blend.sidedef != sidedef || blend.slot != slot || blend.to != current {
			continue
		}
		elapsed := float64(g.worldTic-blend.startTic) + g.renderAlpha
		if elapsed <= 0 {
			return textureBlendSample{fromKey: blend.from, toKey: blend.to}
		}
		if elapsed >= float64(frames) {
			return textureBlendSample{fromKey: blend.to}
		}
		alpha := applyBlendShutter(elapsed/float64(frames), switchTextureShutterAngle)
		alphaU8 := uint8(math.Round(alpha * 255))
		if alphaU8 == 0 {
			return textureBlendSample{fromKey: blend.from, toKey: blend.to}
		}
		return textureBlendSample{fromKey: blend.from, toKey: blend.to, alpha: alphaU8}
	}
	return textureBlendSample{}
}

func (g *game) switchTextureBlendSample(sidedef int, slot switchTextureSlot, current string) (wallTextureBlendSample, bool) {
	sample := g.switchTextureBlendFor(sidedef, slot, current)
	if sample.fromKey == "" {
		return wallTextureBlendSample{}, false
	}
	from, ok := g.wallTexturePtr(sample.fromKey)
	if !ok || from == nil || from.Width <= 0 || from.Height <= 0 || len(from.RGBA) != from.Width*from.Height*4 {
		return wallTextureBlendSample{}, false
	}
	out := wallTextureBlendSample{from: from}
	if sample.alpha == 0 || sample.toKey == "" {
		return out, true
	}
	to, ok := g.wallTexturePtr(sample.toKey)
	if !ok || to == nil || to.Width != from.Width || to.Height != from.Height || len(to.RGBA) != len(from.RGBA) {
		return out, true
	}
	out.to = to
	out.alpha = sample.alpha
	return out, true
}

func (g *game) precacheSpritePatchRenderData() {
	if g == nil || len(g.spritePatchPtrs) == 0 {
		return
	}
	if g.spriteOpaqueShapeCache == nil {
		g.spriteOpaqueShapeCache = make(map[string]spriteOpaqueShape, len(g.spritePatchPtrs))
	}
	for key, tex := range g.spritePatchPtrs {
		if tex == nil || tex.Width <= 0 || tex.Height <= 0 {
			continue
		}
		if built, ok := synthesizeIndexedSpriteTexture(*tex); ok {
			*tex = built
			g.opts.SpritePatchBank[key] = built
		}
		g.spriteOpaqueShapeForKey(key, tex)
	}
}

func (g *game) precacheThingSpriteExpansion() {
	if g == nil || g.m == nil || len(g.m.Things) == 0 {
		return
	}
	if g.thingSpriteExpandCache == nil {
		g.thingSpriteExpandCache = make(map[string][]string, 256)
	}
	for i, th := range g.m.Things {
		name := g.mapThingSpriteName(i, th)
		if name != "" {
			g.expandThingSpriteFrames(name)
		}
	}
}

func (g *game) precacheMonsterSpriteRefs() {
	if g == nil || g.m == nil || len(g.m.Things) == 0 {
		return
	}
	seenTypes := make(map[int16]struct{}, len(g.m.Things))
	for _, th := range g.m.Things {
		if !isMonster(th.Type) {
			continue
		}
		if _, seen := seenTypes[th.Type]; seen {
			continue
		}
		seenTypes[th.Type] = struct{}{}
		g.precacheMonsterSpriteRefsForType(th.Type)
	}
}

func (g *game) precacheMonsterSpriteRefsForType(typ int16) {
	if g == nil {
		return
	}
	prefix, ok := monsterSpritePrefix(typ)
	if !ok || prefix == "" {
		if name := g.monsterSpriteName(typ, 0); name != "" {
			g.spriteRenderRef(name)
		}
		return
	}
	frames := monsterFrameLettersForPrecache(typ)
	for _, frame := range frames {
		if frame < 'A' || frame > 'Z' {
			continue
		}
		g.monsterFrameRenderRef(prefix, frame, 0)
		for rot := 1; rot <= 8; rot++ {
			g.monsterFrameRenderRefRot(prefix, frame, rot)
		}
	}
	if name := g.monsterSpriteName(typ, 0); name != "" {
		g.spriteRenderRef(name)
	}
}

func monsterFrameLettersForPrecache(typ int16) []byte {
	seen := make(map[byte]struct{}, 32)
	out := make([]byte, 0, 32)
	appendFrames := func(seq []byte) {
		for _, frame := range seq {
			if frame < 'A' || frame > 'Z' {
				continue
			}
			if _, ok := seen[frame]; ok {
				continue
			}
			seen[frame] = struct{}{}
			out = append(out, frame)
		}
	}
	appendFrames(monsterSpawnFrameSeq(typ))
	appendFrames(monsterSeeFrameSeq(typ))
	appendFrames(monsterAttackFrameSeq(typ))
	appendFrames(monsterPainFrameSeq(typ))
	appendFrames(monsterDeathFrameSeq(typ))
	appendFrames(monsterXDeathFrameSeq(typ))
	return out
}

func (g *game) precacheWorldThingSpriteRefs() {
	if g == nil || g.m == nil || len(g.m.Things) == 0 {
		return
	}
	for i, th := range g.m.Things {
		if isMonster(th.Type) || isPlayerStart(th.Type) {
			continue
		}
		if isBarrelThingType(th.Type) {
			for _, name := range barrelSpawnSprites {
				g.spriteRenderRef(name)
			}
			for _, name := range barrelDeathSprites {
				g.spriteRenderRef(name)
			}
			continue
		}
		anim := thingAnimRefState{}
		if i >= 0 && i < len(g.thingWorldAnimRef) {
			anim = g.thingWorldAnimRef[i]
		} else {
			anim = g.buildThingWorldAnimRef(th)
		}
		if anim.staticRef != nil {
			g.spriteRenderRef(anim.staticRef.key)
		}
		for _, ref := range anim.refs {
			if ref == nil || ref.key == "" {
				continue
			}
			g.spriteRenderRef(ref.key)
		}
		if name := g.worldThingSpriteNameScaled(th.Type, 0, 1); name != "" {
			g.spriteRenderRef(name)
		}
	}
}

func (g *game) precacheProjectileSpriteRefs() {
	if g == nil {
		return
	}
	for kind := projectileFireball; kind <= projectileBFGBall; kind++ {
		for frame := 0; frame < 2; frame++ {
			g.projectileSpriteRef(kind, frame)
		}
		for phase := 0; ; phase++ {
			if projectileImpactPhaseTics(kind, phase) <= 0 {
				break
			}
			g.projectileImpactSpriteRef(kind, phase)
		}
	}
	for _, p := range []hitscanPuff{
		{kind: hitscanFxPuff, tics: 6},
		{kind: hitscanFxPuff, tics: 4},
		{kind: hitscanFxPuff, tics: 2},
		{kind: hitscanFxPuff, tics: 1},
		{kind: hitscanFxBlood, tics: 7},
		{kind: hitscanFxBlood, tics: 5},
		{kind: hitscanFxBlood, tics: 1},
		{kind: hitscanFxSmoke, state: 0},
		{kind: hitscanFxSmoke, state: 1},
		{kind: hitscanFxSmoke, state: 2},
		{kind: hitscanFxSmoke, state: 3},
		{kind: hitscanFxSmoke, state: 4},
	} {
		g.hitscanEffectSpriteRef(p)
	}
	for state := 130; state <= 141; state++ {
		g.hitscanEffectSpriteRef(hitscanPuff{kind: hitscanFxTeleport, state: state})
	}
	for tic := 0; tic < 12; tic++ {
		g.bossCubeSpriteRef(tic)
	}
	for elapsed := 0; elapsed < 32; elapsed += 4 {
		g.bossSpawnFireSpriteRef(elapsed)
	}
}

func normalizeSpritePatchKey(name string) string {
	return strings.ToUpper(strings.TrimSpace(name))
}

func (g *game) spritePatchTexture(name string) (*WallTexture, bool) {
	if g == nil {
		return nil, false
	}
	g.ensureTexturePointerCaches()
	if len(g.spritePatchPtrs) == 0 {
		return nil, false
	}
	tex, ok := g.spritePatchPtrs[normalizeSpritePatchKey(name)]
	if !ok || tex == nil {
		return nil, false
	}
	return tex, true
}

func (g *game) spritePatchExists(name string) bool {
	_, ok := g.spritePatchTexture(name)
	return ok
}

func (g *game) wallTexturePtr(name string) (*WallTexture, bool) {
	if g == nil {
		return nil, false
	}
	g.ensureTexturePointerCaches()
	if len(g.wallTexPtrs) == 0 {
		return nil, false
	}
	key, _ := g.resolveAnimatedWallSample(name)
	return g.wallTexturePtrResolvedKey(key)
}

func (g *game) wallTexturePtrResolvedKey(key string) (*WallTexture, bool) {
	if g == nil {
		return nil, false
	}
	g.ensureTexturePointerCaches()
	if len(g.wallTexPtrs) == 0 {
		return nil, false
	}
	if key == "" || key == "-" {
		return nil, false
	}
	tex, ok := g.wallTexPtrs[key]
	if !ok || tex == nil {
		return nil, false
	}
	return tex, true
}

func (g *game) initSkyLayerShader() {
	if g == nil || g.skyLayerShader != nil {
		return
	}
	if g.opts.GPUSky && g.opts.SourcePortMode {
		if sh, err := ebiten.NewShader(skyBackdropShaderSrc); err == nil {
			g.skyLayerShader = sh
		}
	}
}

func defaultDetailLevelForMode(viewW, viewH int, sourcePort bool) int {
	if sourcePort {
		if isWASMBuild() && len(sourcePortDetailDivisors) > 2 {
			return 2
		}
		if len(sourcePortDetailDivisors) > 1 {
			return 1
		}
		return 0
	}
	return 0
}

func (g *game) resetView() {
	centerX, centerY, worldW, worldH := boundsViewMetrics(g.bounds)
	g.State.Reset(centerX, centerY, worldW, worldH, g.viewW, g.viewH, doomInitialZoomMul)
}

func detailPresetIndex(w, h int) int {
	for i, p := range detailPresets {
		if p[0] == w && p[1] == h {
			return i
		}
	}
	return 0
}

func faithfulDetailPresetSize(level int) (int, int) {
	if len(detailPresets) == 0 {
		return doomLogicalW, doomLogicalH
	}
	if level < 0 || level >= len(detailPresets) {
		level = 0
	}
	return detailPresets[level][0], detailPresets[level][1]
}

func (g *game) setDetailLevel(level int) bool {
	if g == nil {
		return false
	}
	level = clampDetailLevelForMode(level, g.opts.SourcePortMode)
	if level == g.detailLevel {
		return false
	}
	g.detailLevel = level
	if g.opts.SourcePortMode {
		// Detail ratio changes rewire sourceport internal resolution, so force a
		// clean sky projection/image state before the next frame.
		g.resetSkyLayerPipeline(false)
	} else {
		w, h := faithfulDetailPresetSize(g.detailLevel)
		if g.viewW != w || g.viewH != h {
			g.viewW = w
			g.viewH = h
			_, _, worldW, worldH := boundsViewMetrics(g.bounds)
			g.State.Refit(worldW, worldH, g.viewW, g.viewH, doomInitialZoomMul)
		}
	}
	g.mouseLookSet = false
	g.mouseLookSuppressTicks = detailMouseSuppressTicks
	g.syncRenderState()
	return true
}

func (g *game) detailHUDLabel() string {
	if g == nil {
		return ""
	}
	if g.autoDetailEnabled {
		return "AUTO"
	}
	if g.opts.SourcePortMode {
		div := g.sourcePortDetailDivisor()
		if div <= 1 {
			return "1x"
		}
		return fmt.Sprintf("1/%dx", div)
	}
	switch {
	case g.lowDetailMode():
		return "LOW"
	case g.detailLevel == len(detailPresets)-1:
		return "EXTRA HIGH"
	default:
		return "HIGH"
	}
}

func (g *game) detailLevelLabelFor(level int) string {
	if g == nil {
		return ""
	}
	level = clampDetailLevelForMode(level, g.opts.SourcePortMode)
	if g.opts.SourcePortMode {
		div := 1
		if level >= 0 && level < len(sourcePortDetailDivisors) {
			div = sourcePortDetailDivisors[level]
		}
		if div <= 1 {
			return "1x"
		}
		return fmt.Sprintf("1/%dx", div)
	}
	switch {
	case level == 1:
		return "LOW"
	case level == len(detailPresets)-1:
		return "EXTRA HIGH"
	default:
		return "HIGH"
	}
}

func (g *game) estimatedRenderMSForDetailLevel(level int, renderMS float64) float64 {
	if g == nil || renderMS <= 0 {
		return renderMS
	}
	level = clampDetailLevelForMode(level, g.opts.SourcePortMode)
	cur := clampDetailLevelForMode(g.detailLevel, g.opts.SourcePortMode)
	if level == cur {
		return renderMS
	}
	scale := 1.0
	if g.opts.SourcePortMode {
		curDiv := max(g.sourcePortDetailDivisor(), 1)
		nextDiv := 1
		if level >= 0 && level < len(sourcePortDetailDivisors) {
			nextDiv = max(sourcePortDetailDivisors[level], 1)
		}
		scale = math.Pow(float64(curDiv)/float64(nextDiv), 2)
	} else {
		curW, curH := faithfulDetailPresetSize(cur)
		nextW, nextH := faithfulDetailPresetSize(level)
		curPixels := max(curW*curH, 1)
		nextPixels := max(nextW*nextH, 1)
		scale = float64(nextPixels) / float64(curPixels)
		if cur == 1 && level == 0 {
			// Low detail reuses the same buffer size but doubles columns.
			scale = 2.0
		}
	}
	return renderMS * scale
}

func (g *game) applyAutoDetailSample(fps, renderMS float64) {
	if g == nil || !g.autoDetailEnabled {
		return
	}
	if g.mode != viewWalk {
		return
	}
	if g.autoDetailCooldown > 0 {
		g.autoDetailCooldown--
		return
	}
	const (
		targetFPS            = 60.0
		lowFPS               = targetFPS - 3.0
		veryLowFPS           = targetFPS - 10.0
		highRenderMS         = 1000.0 / targetFPS
		raiseRenderTargetMS  = 8.0
		lowSamplesToDrop     = 2
		highSamplesToRecover = 4
		cooldownSamples      = 3
	)
	nextHigherDetail := clampDetailLevelForMode(g.detailLevel-1, g.opts.SourcePortMode)
	projectedHigherRenderMS := g.estimatedRenderMSForDetailLevel(nextHigherDetail, renderMS)
	if fps < veryLowFPS || renderMS > highRenderMS {
		g.autoDetailLowSamples++
		g.autoDetailHighSamples = 0
	} else if nextHigherDetail != g.detailLevel && fps >= lowFPS && renderMS < raiseRenderTargetMS && projectedHigherRenderMS < raiseRenderTargetMS {
		g.autoDetailHighSamples++
		g.autoDetailLowSamples = 0
	} else if fps < lowFPS {
		g.autoDetailLowSamples++
		g.autoDetailHighSamples = 0
	} else {
		g.autoDetailLowSamples = 0
		g.autoDetailHighSamples = 0
	}
	if g.autoDetailLowSamples >= lowSamplesToDrop {
		next := clampDetailLevelForMode(g.detailLevel+1, g.opts.SourcePortMode)
		if g.setDetailLevel(next) {
			g.setHUDMessage(fmt.Sprintf("Detail: AUTO DOWN -> %s", g.detailLevelLabelFor(next)), 70)
			g.autoDetailCooldown = cooldownSamples
		}
		g.autoDetailLowSamples = 0
		g.autoDetailHighSamples = 0
		return
	}
	if g.autoDetailHighSamples >= highSamplesToRecover {
		next := clampDetailLevelForMode(g.detailLevel-1, g.opts.SourcePortMode)
		if g.setDetailLevel(next) {
			g.setHUDMessage(fmt.Sprintf("Detail: AUTO UP -> %s", g.detailLevelLabelFor(next)), 70)
			g.autoDetailCooldown = cooldownSamples
		}
		g.autoDetailLowSamples = 0
		g.autoDetailHighSamples = 0
	}
}

func (g *game) cycleDetailLevel() {
	if len(detailPresets) == 0 {
		return
	}
	g.autoDetailCooldown = 0
	g.autoDetailLowSamples = 0
	g.autoDetailHighSamples = 0
	if g.autoDetailEnabled {
		g.autoDetailEnabled = false
	}
	next := 0
	if len(detailPresets) > 1 {
		if g.lowDetailMode() {
			next = 0
		} else {
			next = 1
		}
	}
	_ = g.setDetailLevel(next)
	g.setHUDMessage(fmt.Sprintf("Detail: %s", g.detailHUDLabel()), 70)
}

func (g *game) sourcePortDetailDivisor() int {
	if len(sourcePortDetailDivisors) == 0 {
		return 1
	}
	i := g.detailLevel
	if i < 0 || i >= len(sourcePortDetailDivisors) {
		i = 0
	}
	d := sourcePortDetailDivisors[i]
	if d < 1 {
		return 1
	}
	return d
}

func (g *game) cycleSourcePortDetailLevel() {
	if len(sourcePortDetailDivisors) == 0 {
		return
	}
	g.autoDetailCooldown = 0
	g.autoDetailLowSamples = 0
	g.autoDetailHighSamples = 0
	if g.autoDetailEnabled {
		g.autoDetailEnabled = false
		g.setHUDMessage(fmt.Sprintf("Detail: %s", g.detailLevelLabelFor(g.detailLevel)), 70)
		return
	}
	next := (g.detailLevel + 1) % len(sourcePortDetailDivisors)
	if next == 0 {
		g.autoDetailEnabled = true
		g.setHUDMessage("Detail: AUTO", 70)
		return
	}
	_ = g.setDetailLevel(next)
	g.setHUDMessage(fmt.Sprintf("Detail: %s", g.detailHUDLabel()), 70)
}

func (g *game) mouseLookTurnRaw(dx int) int64 {
	scaleX := g.mouseInputScaleX
	if scaleX <= 0 {
		scaleX = 1
	}
	return mouseLookTurnRawScaled(dx, g.opts.MouseLookSpeed, scaleX, g.opts.MouseInvert)
}

func mouseLookTurnRawWithWidth(dx int, speed float64, renderW int, invertHorizontal bool) int64 {
	_ = renderW
	return mouseLookTurnRawScaled(dx, speed, 1, invertHorizontal)
}

func mouseLookTurnRawScaled(dx int, speed float64, scaleX float64, invertHorizontal bool) int64 {
	if dx == 0 {
		return 0
	}
	base := float64(40 << 16)
	if scaleX <= 0 {
		scaleX = 1
	}
	raw := int64(math.Round(float64(dx) * scaleX * base * speed))
	if raw == 0 {
		if dx > 0 {
			raw = 1
		} else {
			raw = -1
		}
	}
	if invertHorizontal {
		return raw
	}
	// Positive mouse delta should turn right (decrease world angle).
	return -raw
}

func (g *game) runtimeSettingsSnapshot() RuntimeSettings {
	return RuntimeSettings{
		DetailLevel:        g.detailLevel,
		AutoDetail:         g.autoDetailEnabled,
		GammaLevel:         g.gammaLevel,
		MusicVolume:        g.opts.MusicVolume,
		MUSPanMax:          g.opts.MUSPanMax,
		OPLVolume:          g.opts.OPLVolume,
		MusicBackend:       string(g.opts.MusicBackend),
		MusicSoundFontPath: g.opts.MusicSoundFontPath,
		SFXVolume:          g.opts.SFXVolume,
		HUDMessages:        g.hudMessagesEnabled,
		MouseLook:          g.opts.MouseLook,
		MouseInvert:        g.opts.MouseInvert,
		AlwaysRun:          g.alwaysRun,
		AutoWeaponSwitch:   g.autoWeaponSwitch,
		ThingRenderMode:    g.opts.SourcePortThingRenderMode,
		CRTEffect:          g.crtEnabled,
	}
}

func (g *game) publishRuntimeSettingsIfChanged() {
	if g == nil || g.opts.OnRuntimeSettingsChanged == nil {
		return
	}
	cur := g.runtimeSettingsSnapshot()
	if g.runtimeSettingsSeen && cur == g.runtimeSettingsLast {
		return
	}
	g.runtimeSettingsSeen = true
	g.runtimeSettingsLast = cur
	g.opts.OnRuntimeSettingsChanged(cur)
}

func (g *game) publishInputBindingsChanged() {
	if g == nil {
		return
	}
	g.opts.InputBindings = runtimecfg.NormalizeInputBindings(g.opts.InputBindings)
	if g.opts.OnInputBindingsChanged != nil {
		g.opts.OnInputBindingsChanged(g.opts.InputBindings)
	}
}

func wasmPointerLockGestureActive() bool {
	return ebiten.IsKeyPressed(ebiten.KeyW) ||
		ebiten.IsKeyPressed(ebiten.KeyA) ||
		ebiten.IsKeyPressed(ebiten.KeyS) ||
		ebiten.IsKeyPressed(ebiten.KeyD) ||
		ebiten.IsKeyPressed(ebiten.KeyArrowUp) ||
		ebiten.IsKeyPressed(ebiten.KeyArrowDown) ||
		ebiten.IsKeyPressed(ebiten.KeyArrowLeft) ||
		ebiten.IsKeyPressed(ebiten.KeyArrowRight) ||
		ebiten.IsKeyPressed(ebiten.KeySpace) ||
		ebiten.IsKeyPressed(ebiten.KeyEnter) ||
		ebiten.IsKeyPressed(ebiten.KeyKPEnter) ||
		ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) ||
		ebiten.IsMouseButtonPressed(ebiten.MouseButtonRight) ||
		ebiten.IsMouseButtonPressed(ebiten.MouseButtonMiddle)
}

func applyRuntimeCursorMode(captured bool) {
	current := ebiten.CursorMode()
	want := ebiten.CursorModeVisible
	if captured {
		want = ebiten.CursorModeCaptured
	}
	if !isWASMBuild() {
		if current != want {
			ebiten.SetCursorMode(want)
		}
		return
	}
	if !captured {
		if current != ebiten.CursorModeVisible {
			ebiten.SetCursorMode(ebiten.CursorModeVisible)
		}
		return
	}
	if current == ebiten.CursorModeCaptured {
		return
	}
	if wasmPointerLockGestureActive() {
		ebiten.SetCursorMode(ebiten.CursorModeCaptured)
	}
}

func (g *game) Update() error {
	defer g.clearSampledInput()
	if g.levelExitRequested {
		return ebiten.Termination
	}
	if g.opts.DemoScript != nil {
		return g.updateDemoMode()
	}
	if g.opts.LiveTicSource != nil {
		return g.updateWatchMode()
	}
	if err := g.pollChatMessages(); err != nil {
		return fmt.Errorf("chat stream: %w", err)
	}
	if g.handleChatInput() {
		// Chat compose owns Enter/Escape/T while it is active.
	} else {
		if g.keyJustPressed(ebiten.KeyF4) {
			g.soundMenuRequested = true
			return nil
		}
		if g.keyJustPressed(ebiten.KeyF10) {
			g.quitPromptRequested = true
			return nil
		}
		if g.keyJustPressed(ebiten.KeyEscape) {
			g.frontendMenuRequested = true
			ebiten.SetCursorMode(ebiten.CursorModeVisible)
			return nil
		}
		if g.bindingJustPressed(bindingAutomap) {
			if g.mode == viewWalk {
				g.mode = viewMap
				g.setHUDMessage("Automap Opened", 35)
			} else {
				g.mode = viewWalk
				// Reset mouse baseline when entering walk mode to avoid turn spikes.
				g.mouseLookSet = false
				g.mouseLookSuppressTicks = detailMouseSuppressTicks
				g.setHUDMessage("Automap Closed", 35)
			}
		}
		if g.keyJustPressed(ebiten.KeyTab) {
			if g.mode == viewWalk {
				g.mode = viewMap
				g.setHUDMessage("Automap Opened", 35)
			} else {
				g.mode = viewWalk
				// Reset mouse baseline when entering walk mode to avoid turn spikes.
				g.mouseLookSet = false
				g.mouseLookSuppressTicks = detailMouseSuppressTicks
				g.setHUDMessage("Automap Closed", 35)
			}
		}
		if g.opts.SourcePortMode && g.keyJustPressed(ebiten.KeyR) {
			g.rotateView = !g.rotateView
			if g.rotateView {
				g.setHUDMessage("Heading-Up ON", 70)
			} else {
				g.setHUDMessage("Heading-Up OFF", 70)
			}
		}
		if g.opts.SourcePortMode && g.keyJustPressed(ebiten.KeyBackslash) {
			g.opts.MouseLook = !g.opts.MouseLook
			if g.opts.MouseLook {
				g.setHUDMessage("Mouse Look ON", 70)
				g.mouseLookSet = false
				g.mouseLookSuppressTicks = detailMouseSuppressTicks
			} else {
				g.setHUDMessage("Mouse Look OFF", 70)
				g.mouseLookSet = false
			}
		}
		if g.keyJustPressed(ebiten.KeyF1) {
			g.readThisRequested = true
		}
		if g.keyJustPressed(ebiten.KeyComma) {
			g.setSimTickScale(g.simTickScale - 0.1)
		}
		if g.keyJustPressed(ebiten.KeyPeriod) {
			g.setSimTickScale(g.simTickScale + 0.1)
		}
		if g.keyJustPressed(ebiten.KeySlash) {
			g.setSimTickScale(1.0)
		}
		if g.bindingJustPressed(bindingUse) {
			g.pendingUse = true
		}
		if g.keyJustPressed(ebiten.KeyF5) {
			if g.opts.SourcePortMode {
				g.cycleSourcePortDetailLevel()
			} else {
				g.cycleDetailLevel()
			}
		}
		if g.isDead && (g.keyJustPressed(ebiten.KeyEnter) || g.keyJustPressed(ebiten.KeyKPEnter)) {
			g.requestLevelRestart()
		}
	}
	ticks := g.consumeSimTicks()
	for i := 0; i < ticks; i++ {
		g.edgeInputPass = i == 0
		g.capturePrevState()
		if g.mode == viewMap {
			if g.opts.SourcePortMode {
				applyRuntimeCursorMode(true)
			} else {
				applyRuntimeCursorMode(false)
			}
			g.updateMapMode()
		} else {
			applyRuntimeCursorMode(true)
			g.updateWalkMode()
		}
		g.tickStatusWidgets()
		if g.useFlash > 0 {
			g.useFlash--
		}
		g.tickChatHistory()
		if g.damageFlashTic > 0 {
			g.damageFlashTic--
		}
		if g.bonusFlashTic > 0 {
			g.bonusFlashTic--
		}
		g.tickDelayedSounds()
		g.tickDelayedSwitchReverts()
		g.flushSoundEvents()
		g.markSimUpdate(time.Now())
	}
	g.edgeInputPass = false
	g.publishRuntimeSettingsIfChanged()
	return nil
}

func (g *game) consumeSimTicks() int {
	if g.simTickScale <= 0 {
		g.simTickScale = 1
	}
	g.simTickAccum += g.simTickScale
	ticks := int(g.simTickAccum)
	if ticks <= 0 {
		return 0
	}
	const maxTicksPerFrame = 8
	if ticks > maxTicksPerFrame {
		ticks = maxTicksPerFrame
	}
	g.simTickAccum -= float64(ticks)
	if g.simTickAccum > float64(maxTicksPerFrame) {
		g.simTickAccum = float64(maxTicksPerFrame)
	}
	return ticks
}

func (g *game) setSimTickScale(v float64) {
	if v < 0.1 {
		v = 0.1
	}
	if v > 8.0 {
		v = 8.0
	}
	g.simTickScale = v
	g.setHUDMessage(fmt.Sprintf("Game Speed: %.2fx", g.simTickScale), 70)
}

func (g *game) updateDemoMode() error {
	script := g.opts.DemoScript
	if script == nil {
		return nil
	}
	if len(g.input.justPressedKeys) > 0 {
		g.frontendMenuRequested = true
		ebiten.SetCursorMode(ebiten.CursorModeVisible)
		return nil
	}
	g.capturePrevState()
	if !g.demoBenchStarted {
		g.demoBenchStarted = true
		g.demoBenchStart = time.Now()
		g.demoBenchFrameNS = g.demoBenchFrameNS[:0]
		g.benchLow1MS = 0
		g.benchLow01MS = 0
	}
	if !g.demoRNGCaptured {
		g.demoStartRnd, g.demoStartPRnd = doomrand.State()
		g.demoRNGCaptured = true
	}
	if g.demoTrace != nil && !g.demoTraceInitialWritten {
		if g.demoTick >= len(script.Tics) {
			g.demoTrace.Close()
			g.demoTrace = nil
			g.reportDemoBench(script)
			return ebiten.Termination
		}
		tc := script.Tics[g.demoTick]
		g.demoTick++
		cmd, usePressed, fireHeld := demoTicCommand(tc)
		g.runGameplayTic(cmd, usePressed, fireHeld)
		g.discoverLinesAroundPlayer()
		g.State.SetCamera(float64(g.p.x)/fracUnit, float64(g.p.y)/fracUnit)
		g.tickDelayedSounds()
		g.flushSoundEvents()
		g.tickStatusWidgets()
		g.writeDemoTraceTic(0)
		g.demoTraceInitialWritten = true
		return nil
	}
	if g.isDead && g.opts.DemoQuitOnComplete && g.opts.DemoExitOnDeath {
		g.reportDemoBench(script)
		return ebiten.Termination
	}
	if g.opts.DemoStopAfterTics > 0 && g.demoTick >= g.opts.DemoStopAfterTics {
		if g.demoTrace != nil {
			g.demoTrace.Close()
			g.demoTrace = nil
		}
		g.reportDemoBench(script)
		return ebiten.Termination
	}
	if g.demoTick >= len(script.Tics) {
		if g.demoTrace != nil {
			g.demoTrace.Close()
			g.demoTrace = nil
		}
		g.reportDemoBench(script)
		return ebiten.Termination
	}
	tc := script.Tics[g.demoTick]
	g.demoTick++
	g.stepGameplayFromDemoTic(tc)
	g.writeDemoTraceTic(g.demoTick - 1)
	if g.isDead && g.opts.DemoQuitOnComplete && g.opts.DemoExitOnDeath {
		g.reportDemoBench(script)
		return ebiten.Termination
	}
	return nil
}

func (g *game) reportDemoBench(script *DemoScript) {
	if g == nil || g.demoDoneReported || !g.opts.DemoQuitOnComplete {
		return
	}
	g.demoDoneReported = true
	elapsed := time.Since(g.demoBenchStart)
	tics := g.demoTick
	sec := elapsed.Seconds()
	tps := 0.0
	fps := 0.0
	msPerTic := 0.0
	if sec > 0 {
		tps = float64(tics) / sec
		fps = float64(g.demoBenchDraws) / sec
	}
	if tics > 0 {
		msPerTic = elapsed.Seconds() * 1000 / float64(tics)
	}
	low1NS := demoBenchLowFrameNS(g.demoBenchFrameNS, 0.99)
	low01NS := demoBenchLowFrameNS(g.demoBenchFrameNS, 0.999)
	low1MS := float64(low1NS) / float64(time.Millisecond)
	low01MS := float64(low01NS) / float64(time.Millisecond)
	low1FPS := demoBenchFPSFromFrameNS(low1NS)
	low01FPS := demoBenchFPSFromFrameNS(low01NS)
	label := "demo"
	if strings.TrimSpace(script.Path) != "" {
		label = script.Path
	}
	fmt.Printf("demo-bench path=%s wad=%s map=%s rng_start=%d/%d tics=%d draws=%d elapsed=%s tps=%.2f fps=%.2f ms_per_tic=%.3f low_1pct_ms=%.3f low_1pct_fps=%.2f low_01pct_ms=%.3f low_01pct_fps=%.2f player_dead=%t\n",
		label, g.opts.WADHash, g.m.Name, g.demoStartRnd, g.demoStartPRnd, tics, g.demoBenchDraws, elapsed.Round(time.Millisecond), tps, fps, msPerTic, low1MS, low1FPS, low01MS, low01FPS, g.isDead)
}

func (g *game) requestLevelExit(secret bool, msg string) {
	g.levelExitRequested = true
	g.secretLevelExit = secret
	g.setHUDMessage(msg, 35)
}

func (g *game) requestLevelRestart() {
	g.levelRestartRequested = true
	if g != nil && g.m != nil && g.opts.DebugEvents {
		fmt.Printf(
			"level-restart-request map=%s player=(%.2f,%.2f,%.2f) health=%d dead=%t world_tic=%d\n",
			g.m.Name,
			float64(g.p.x)/fracUnit,
			float64(g.p.y)/fracUnit,
			float64(g.p.z)/fracUnit,
			g.stats.Health,
			g.isDead,
			g.worldTic,
		)
	}
	g.setHUDMessage("Restarting level...", 20)
}

func (g *game) updateMapMode() {
	if g.chatComposeOpen {
		return
	}
	g.updateParityControls()
	g.updateWeaponHotkeys(false)
	input := g.buildMapViewInputState()
	state := g.buildMapViewUpdateState()
	result := mapview.Update(input, state)
	g.applyMapViewUpdateResult(result)
}

func (g *game) updateWalkMode() {
	cmd := moveCmd{}
	usePressed := false
	fireHeld := false
	speed := 0
	if !g.chatComposeOpen {
		g.updateParityControls()
		g.updateWeaponHotkeys(true)
		g.updateWalkScreenSize()
		ctrlHeld := g.keyHeld(ebiten.KeyControlLeft) || g.keyHeld(ebiten.KeyControlRight)
		if ctrlHeld && g.keyJustPressed(ebiten.KeyBracketRight) {
			g.adjustHUDScale(1)
		}
		if ctrlHeld && g.keyJustPressed(ebiten.KeyBracketLeft) {
			g.adjustHUDScale(-1)
		}
		speed = g.currentRunSpeed()
		strafeMod := g.bindingHeld(bindingStrafeModifier)
		if g.bindingHeld(bindingMoveForward) {
			cmd.forward += forwardMove[speed]
		}
		if g.bindingHeld(bindingMoveBackward) {
			cmd.forward -= forwardMove[speed]
		}
		if g.bindingHeld(bindingStrafeLeft) {
			cmd.side -= sideMove[speed]
		}
		if g.bindingHeld(bindingStrafeRight) {
			cmd.side += sideMove[speed]
		}
		if g.bindingHeld(bindingTurnLeft) {
			if strafeMod {
				cmd.side -= sideMove[speed]
			} else {
				cmd.turn += 1
			}
		}
		if g.bindingHeld(bindingTurnRight) {
			if strafeMod {
				cmd.side += sideMove[speed]
			} else {
				cmd.turn -= 1
			}
		}
		if g.edgeInputPass && g.pendingUse {
			usePressed = true
			g.pendingUse = false
		}
		fireHeld = g.bindingHeld(bindingFire)
		if g.opts.MouseLook {
			if g.mouseLookSuppressTicks > 0 {
				g.mouseLookSuppressTicks--
			}
			cmd.turnRaw += g.input.mouseTurnRawAccum
			g.input.mouseTurnRawAccum = 0
		} else {
			g.mouseLookSet = false
		}
	} else {
		g.pendingUse = false
		g.mouseLookSet = false
	}
	g.input.mouseTurnRawAccum = 0

	cmd.run = speed == 1
	if strings.TrimSpace(g.opts.RecordDemoPath) != "" || g.opts.LiveTicSink != nil {
		// Quantize cmd to demo format precision before running gameplay,
		// matching vanilla's G_WriteDemoTiccmd -> G_ReadDemoTiccmd round-trip.
		cmd = quantizeMoveCmdToDemo(cmd)
	}
	g.runGameplayTic(cmd, usePressed, fireHeld)
	g.recordDemoTic(cmd, usePressed, fireHeld)
	g.discoverLinesAroundPlayer()
	g.State.SetCamera(float64(g.p.x)/fracUnit, float64(g.p.y)/fracUnit)
}

func (g *game) mapRotationActive() bool {
	if g == nil || !g.rotateView {
		return false
	}
	if g.mode == viewMap && !g.State.Snapshot().FollowEnabled() {
		return false
	}
	return true
}

func (g *game) mouseLookBlocked() bool {
	return g != nil && (g.pauseMenuActive || g.quitPromptActive || g.frontendActive || g.frontendMenuRequested || g.soundMenuRequested || g.readThisRequested || g.musicPlayerRequested)
}

func (g *game) SampleInput() {
	if g == nil {
		return
	}
	g.input.pressedKeys = addPressedKeys(g.input.pressedKeys, inpututil.AppendPressedKeys(nil))
	g.input.justPressedKeys = addPressedKeys(g.input.justPressedKeys, inpututil.AppendJustPressedKeys(nil))
	for _, def := range supportedBindingMouseButtons {
		if ebiten.IsMouseButtonPressed(def.button) {
			g.input.pressedMouseButtons = addPressedMouseButtons(g.input.pressedMouseButtons, []ebiten.MouseButton{def.button})
		}
		if inpututil.IsMouseButtonJustPressed(def.button) {
			g.input.justPressedMouseButtons = addPressedMouseButtons(g.input.justPressedMouseButtons, []ebiten.MouseButton{def.button})
		}
	}
	g.input.inputChars = ebiten.AppendInputChars(g.input.inputChars[:0])
	if len(g.input.inputChars) > 0 && !g.chatComposeOpen {
		g.consumeTypedCheatInput()
	}

	_, wheelY := ebiten.Wheel()
	g.input.wheelY += wheelY
	mx, _ := ebiten.CursorPosition()
	g.input.cursorX = mx

	if !g.opts.MouseLook {
		g.mouseLookSet = false
		g.input.mouseTurnRawAccum = 0
		return
	}
	if g.mouseLookBlocked() || g.mouseLookSuppressTicks > 0 {
		g.lastMouseX = mx
		g.mouseLookSet = true
		return
	}
	if g.mouseLookSet {
		g.input.mouseTurnRawAccum += g.mouseLookTurnRaw(mx - g.lastMouseX)
	}
	g.lastMouseX = mx
	g.mouseLookSet = true
}

func (g *game) clearSampledInput() {
	if g == nil {
		return
	}
	g.input = gameInputSnapshot{}
}

func (g *game) keyHeld(key ebiten.Key) bool {
	if g == nil {
		return ebiten.IsKeyPressed(key)
	}
	_, ok := g.input.pressedKeys[key]
	return ok
}

func (g *game) keyJustPressed(key ebiten.Key) bool {
	if g == nil {
		return inpututil.IsKeyJustPressed(key)
	}
	_, ok := g.input.justPressedKeys[key]
	return ok
}

func (g *game) bindingHeld(action bindingAction) bool {
	if g == nil {
		return false
	}
	binds := bindingValue(g.opts.InputBindings, action)
	for _, name := range binds {
		if key, ok := bindingKeyFromName(name); ok && g.keyHeld(key) {
			return true
		}
		if button, ok := bindingMouseButtonFromName(name); ok && g.mouseHeld(button) {
			return true
		}
	}
	return false
}

func (g *game) bindingJustPressed(action bindingAction) bool {
	if g == nil {
		return false
	}
	binds := bindingValue(g.opts.InputBindings, action)
	return bindingPressed(g.input.justPressedKeys, binds) || bindingMousePressed(g.input.justPressedMouseButtons, binds)
}

func (g *game) mouseHeld(button ebiten.MouseButton) bool {
	if g == nil {
		return ebiten.IsMouseButtonPressed(button)
	}
	_, ok := g.input.pressedMouseButtons[button]
	return ok
}

func (g *game) mouseJustPressed(button ebiten.MouseButton) bool {
	if g == nil {
		return inpututil.IsMouseButtonJustPressed(button)
	}
	_, ok := g.input.justPressedMouseButtons[button]
	return ok
}

func addPressedKeys(dst map[ebiten.Key]struct{}, keys []ebiten.Key) map[ebiten.Key]struct{} {
	if len(keys) == 0 {
		return dst
	}
	if dst == nil {
		dst = make(map[ebiten.Key]struct{}, len(keys))
	}
	for _, key := range keys {
		dst[key] = struct{}{}
	}
	return dst
}

func addPressedMouseButtons(dst map[ebiten.MouseButton]struct{}, buttons []ebiten.MouseButton) map[ebiten.MouseButton]struct{} {
	if len(buttons) == 0 {
		return dst
	}
	if dst == nil {
		dst = make(map[ebiten.MouseButton]struct{}, len(buttons))
	}
	for _, button := range buttons {
		dst[button] = struct{}{}
	}
	return dst
}

func (g *game) currentRunSpeed() int {
	runHeld := g.bindingHeld(bindingRunModifier)
	runActive := g.alwaysRun
	if runHeld {
		runActive = !runActive
	}
	if runActive {
		return 1
	}
	return 0
}

func (g *game) recordDemoTic(cmd moveCmd, usePressed, fireHeld bool) {
	g.recordGameplayTic(cmd, usePressed, fireHeld)
}

func quantizeMoveCmdToDemo(cmd moveCmd) moveCmd {
	cmd.forward = int64(int8(clampDemoMove(cmd.forward)))
	cmd.side = int64(int8(clampDemoMove(cmd.side)))
	angleturn16 := int16(cmd.turnRaw >> 16)
	cmd.turnRaw = int64(int16(((int32(angleturn16)+128)>>8)<<8)) << 16
	return cmd
}

func (g *game) demoAngleTurn(cmd moveCmd) int16 {
	turnRaw := cmd.turnRaw
	if cmd.turn != 0 {
		held := g.turnHeld + 1
		turn := angleTurn[0]
		if held < slowTurnTics {
			turn = angleTurn[2]
		} else if cmd.run {
			turn = angleTurn[1]
		}
		turnSpeed := g.opts.KeyboardTurnSpeed
		if turnSpeed == 0 {
			turnSpeed = 1
		}
		turn = uint32(float64(turn) * turnSpeed)
		if turn == 0 {
			turn = 1
		}
		if cmd.turn < 0 {
			turnRaw -= int64(turn)
		} else {
			turnRaw += int64(turn)
		}
	}
	return int16(turnRaw / fracUnit)
}

func clampDemoMove(v int64) int8 {
	if v < -128 {
		return -128
	}
	if v > 127 {
		return 127
	}
	return int8(v)
}

func (g *game) updateWeaponHotkeys(allowCycleInput bool) {
	if !g.edgeInputPass {
		return
	}
	if g.bindingJustPressed(bindingWeapon1) {
		g.selectWeaponSlot(1)
		g.demoWeaponSlot = 1
	}
	if g.bindingJustPressed(bindingWeapon2) {
		g.selectWeaponSlot(2)
		g.demoWeaponSlot = 2
	}
	if g.bindingJustPressed(bindingWeapon3) {
		g.selectWeaponSlot(3)
		g.demoWeaponSlot = 3
	}
	if g.bindingJustPressed(bindingWeapon4) {
		g.selectWeaponSlot(4)
		g.demoWeaponSlot = 4
	}
	if g.bindingJustPressed(bindingWeapon5) {
		g.selectWeaponSlot(5)
		g.demoWeaponSlot = 5
	}
	if g.bindingJustPressed(bindingWeapon6) {
		g.selectWeaponSlot(6)
		g.demoWeaponSlot = 6
	}
	if g.bindingJustPressed(bindingWeapon7) {
		g.selectWeaponSlot(7)
		g.demoWeaponSlot = 7
	}
	if !allowCycleInput {
		return
	}
	ctrlHeld := g.keyHeld(ebiten.KeyControlLeft) || g.keyHeld(ebiten.KeyControlRight)
	if g.bindingJustPressed(bindingWeaponNext) {
		if ctrlHeld && bindingsContain(bindingValue(g.opts.InputBindings, bindingWeaponNext), ebiten.KeyBracketRight) && g.keyJustPressed(ebiten.KeyBracketRight) {
			return
		}
		g.cycleWeapon(1)
	}
	if g.bindingJustPressed(bindingWeaponPrev) {
		if ctrlHeld && bindingsContain(bindingValue(g.opts.InputBindings, bindingWeaponPrev), ebiten.KeyBracketLeft) && g.keyJustPressed(ebiten.KeyBracketLeft) {
			return
		}
		g.cycleWeapon(-1)
	}
	if g.input.wheelY < 0 {
		g.cycleWeapon(1)
	}
	if g.input.wheelY > 0 {
		g.cycleWeapon(-1)
	}
}

func (g *game) updateParityControls() {
	if !g.edgeInputPass {
		return
	}
	textInputActive := g.chatComposeOpen || len(g.input.inputChars) > 0
	if g.keyJustPressed(ebiten.KeyCapsLock) {
		g.alwaysRun = !g.alwaysRun
		if g.alwaysRun {
			g.setHUDMessage("Always Run ON", 70)
		} else {
			g.setHUDMessage("Always Run OFF", 70)
		}
	}
	if g.keyJustPressed(ebiten.KeyF12) {
		g.autoWeaponSwitch = !g.autoWeaponSwitch
		if g.autoWeaponSwitch {
			g.setHUDMessage("Auto Weapon Switch ON", 70)
		} else {
			g.setHUDMessage("Auto Weapon Switch OFF", 70)
		}
	}
	if g.keyJustPressed(ebiten.KeyG) {
		g.showGrid = !g.showGrid
		if g.showGrid {
			g.setHUDMessage("Grid ON", 70)
		} else {
			g.setHUDMessage("Grid OFF", 70)
		}
	}
	if !g.opts.SourcePortMode && g.opts.LiveTicSource == nil {
		if g.keyJustPressed(ebiten.KeyF2) {
			g.saveGameRequested = true
		}
		if g.keyJustPressed(ebiten.KeyF3) {
			g.loadGameRequested = true
		}
		if g.keyJustPressed(ebiten.KeyF8) {
			g.hudMessagesEnabled = !g.hudMessagesEnabled
			if g.hudMessagesEnabled {
				g.setHUDMessage("Messages ON", 70)
			} else {
				g.useText = "Messages OFF"
				g.useFlash = 70
			}
		}
		if g.keyJustPressed(ebiten.KeyF11) {
			g.cycleGammaLevel()
		}
	}
	if g.opts.SourcePortMode {
		if !textInputActive && g.keyJustPressed(ebiten.KeyO) {
			if g.parity.reveal == revealNormal {
				g.parity.reveal = revealAllMap
				g.setHUDMessage("Allmap ON", 70)
			} else {
				g.parity.reveal = revealNormal
				g.setHUDMessage("Allmap OFF", 70)
			}
		}
		if !textInputActive && g.keyJustPressed(ebiten.KeyI) {
			g.parity.iddt = (g.parity.iddt + 1) % 3
			g.setHUDMessage(fmt.Sprintf("IDDT %d", g.parity.iddt), 70)
		}
		if !textInputActive && g.keyJustPressed(ebiten.KeyV) {
			g.showLegend = !g.showLegend
			if g.showLegend {
				g.setHUDMessage("Thing Legend ON", 70)
			} else {
				g.setHUDMessage("Thing Legend OFF", 70)
			}
		}
		if !textInputActive && g.keyJustPressed(ebiten.KeyT) {
			g.opts.SourcePortThingRenderMode = cycleSourcePortThingRenderMode(g.opts.SourcePortThingRenderMode)
			g.setHUDMessage(fmt.Sprintf("Thing Render: %s", sourcePortThingRenderModeLabel(g.opts.SourcePortThingRenderMode)), 70)
		}
		if g.keyJustPressed(ebiten.KeyF11) {
			g.cycleGammaLevel()
		}
		if g.keyJustPressed(ebiten.KeyF8) {
			if !g.opts.KageShader {
				g.setHUDMessage("Kage shader disabled (-kage-shader)", 70)
				return
			}
			g.crtEnabled = !g.crtEnabled
			if g.crtEnabled {
				g.setHUDMessage("CRT ON", 70)
			} else {
				g.setHUDMessage("CRT OFF", 70)
			}
		}
	}
}

func (g *game) setGammaLevel(level int) {
	level = clampGamma(level)
	activeGammaLevel = level
	applyActiveGammaLUTs()
	if g != nil {
		g.gammaLevel = level
	}
}

func (g *game) cycleGammaLevel() {
	if g == nil {
		return
	}
	next := g.gammaLevel + 1
	if next >= doomGammaLevels {
		next = 0
	}
	g.setGammaLevel(next)
	g.setHUDMessage(gammaMessage(g.gammaLevel), 70)
}

func (g *game) updateZoom() {
	zoomStep := 1.03
	step := 0.0
	if g.keyHeld(ebiten.KeyEqual) || g.keyHeld(ebiten.KeyKPAdd) {
		step = zoomStep
	}
	if g.keyHeld(ebiten.KeyMinus) || g.keyHeld(ebiten.KeyKPSubtract) {
		step = -zoomStep
	}
	g.State.AdjustZoom(step, g.input.wheelY)
}

func defaultScreenBlocks(opts Options) int {
	if len(opts.StatusPatchBank) == 0 {
		return doomScreenBlocksFull
	}
	if opts.SourcePortMode {
		return doomScreenBlocksOverlay
	}
	return doomScreenBlocksDefault
}

func allowedScreenBlocksRange(opts Options) (int, int) {
	minBlocks := doomScreenBlocksMin
	if opts.SourcePortMode {
		minBlocks = doomScreenBlocksOverlay
	} else {
		minBlocks = doomScreenBlocksDefault
	}
	return minBlocks, doomScreenBlocksFull
}

func clampScreenBlocks(opts Options, blocks int) int {
	if !opts.SourcePortMode && blocks == doomScreenBlocksOverlay {
		blocks = doomScreenBlocksDefault
	}
	if opts.SourcePortMode && blocks == doomScreenBlocksDefault {
		blocks = doomScreenBlocksOverlay
	}
	minBlocks, maxBlocks := allowedScreenBlocksRange(opts)
	return min(maxBlocks, max(minBlocks, blocks))
}

func defaultHUDScaleStep(opts Options) int {
	if len(sourcePortHUDScaleSteps) == 0 {
		return 0
	}
	targetScale := 2.0
	if opts.SourcePortMode {
		targetScale = 4.0
	}
	if !opts.SourcePortMode && defaultScreenBlocks(opts) == doomScreenBlocksOverlay {
		targetScale = 1.0
	}
	for i, v := range sourcePortHUDScaleSteps {
		if v >= targetScale {
			return i
		}
	}
	return len(sourcePortHUDScaleSteps) - 1
}

func (g *game) hudScaleValue() float64 {
	if g == nil || len(sourcePortHUDScaleSteps) == 0 {
		return 1
	}
	i := g.hudScaleStep
	if i < 0 || i >= len(sourcePortHUDScaleSteps) {
		i = 0
	}
	v := sourcePortHUDScaleSteps[i]
	if v <= 0 {
		return 1
	}
	return v
}

func (g *game) hudUsesLogicalLayout() bool {
	return g != nil && (g.opts.SourcePortMode || g.hudLogicalLayout)
}

func (g *game) hudScaleDot() int {
	if g == nil || len(sourcePortHUDScaleSteps) == 0 {
		return 0
	}
	return min(len(sourcePortHUDScaleSteps)-1, max(0, g.hudScaleStep))
}

func (g *game) hudScaleLabel() string {
	return fmt.Sprintf("%d%%", int(math.Round(g.hudScaleValue()*100)))
}

type statusBarDisplayMode uint8

const (
	statusBarDisplayBottom statusBarDisplayMode = iota
	statusBarDisplayOverlay
	statusBarDisplayHidden
)

func (g *game) statusBarDisplayMode() statusBarDisplayMode {
	if g == nil || len(g.opts.StatusPatchBank) == 0 {
		return statusBarDisplayHidden
	}
	switch clampScreenBlocks(g.opts, g.screenBlocks) {
	case doomScreenBlocksFull:
		return statusBarDisplayHidden
	case doomScreenBlocksOverlay:
		return statusBarDisplayOverlay
	default:
		return statusBarDisplayBottom
	}
}

func (g *game) statusBarVisible() bool {
	return g != nil && g.statusBarDisplayMode() != statusBarDisplayHidden
}

func (g *game) walkStatusBarTop() int {
	h := max(g.viewH, 1)
	if g.statusBarDisplayMode() != statusBarDisplayBottom {
		return h
	}
	top := int(math.Floor(hud.StatusBarTop(g.viewW, g.viewH, g.hudUsesLogicalLayout(), g.hudScaleValue())))
	if top < 1 {
		return 1
	}
	if top > h {
		return h
	}
	return top
}

func (g *game) walkRenderViewportRect() image.Rectangle {
	viewW := max(g.viewW, 1)
	viewH := max(g.viewH, 1)
	if g == nil {
		return image.Rect(0, 0, viewW, viewH)
	}
	switch g.statusBarDisplayMode() {
	case statusBarDisplayOverlay, statusBarDisplayHidden:
		return image.Rect(0, 0, viewW, viewH)
	}
	availH := g.walkStatusBarTop()
	if availH <= 0 {
		return image.Rect(0, 0, viewW, 1)
	}
	return image.Rect(0, 0, viewW, availH)
}

func (g *game) walkWeaponViewportRect() image.Rectangle {
	viewW := max(g.viewW, 1)
	viewH := max(g.viewH, 1)
	if g == nil {
		return image.Rect(0, 0, viewW, viewH)
	}
	switch g.statusBarDisplayMode() {
	case statusBarDisplayOverlay, statusBarDisplayHidden:
		return image.Rect(0, 0, viewW, viewH)
	}
	availH := g.walkStatusBarTop()
	if g.hudUsesLogicalLayout() {
		top := int(math.Floor(hud.StatusBarTop(g.viewW, g.viewH, true, 1.0)))
		if top >= 1 && top <= viewH {
			availH = top
		}
	}
	if availH <= 0 {
		return image.Rect(0, 0, viewW, 1)
	}
	return image.Rect(0, 0, viewW, availH)
}

func (g *game) screenSizeDot() int {
	if g == nil {
		return doomScreenBlocksDefault
	}
	return clampScreenBlocks(g.opts, g.screenBlocks)
}

func (g *game) screenSizeLabel() string {
	switch g.statusBarDisplayMode() {
	case statusBarDisplayOverlay:
		return "OVERLAY"
	case statusBarDisplayHidden:
		return "OFF"
	default:
		return "BOTTOM"
	}
}

func (g *game) adjustScreenBlocks(dir int) bool {
	if g == nil || dir == 0 {
		return false
	}
	minBlocks, maxBlocks := allowedScreenBlocksRange(g.opts)
	next := min(maxBlocks, max(minBlocks, g.screenBlocks+dir))
	if next == g.screenBlocks {
		return false
	}
	g.screenBlocks = next
	g.setHUDMessage(fmt.Sprintf("Status bar %s", g.screenSizeLabel()), 70)
	return true
}

func (g *game) adjustHUDScale(dir int) bool {
	if g == nil || dir == 0 || len(sourcePortHUDScaleSteps) == 0 {
		return false
	}
	next := min(len(sourcePortHUDScaleSteps)-1, max(0, g.hudScaleStep+dir))
	if next == g.hudScaleStep {
		return false
	}
	g.hudScaleStep = next
	g.statusBarCacheValid = false
	g.setHUDMessage(fmt.Sprintf("HUD size %s", g.hudScaleLabel()), 70)
	return true
}

func (g *game) updateWalkScreenSize() {
	if g == nil {
		return
	}
	if g.keyJustPressed(ebiten.KeyEqual) || g.keyJustPressed(ebiten.KeyKPAdd) {
		g.adjustScreenBlocks(1)
	}
	if g.keyJustPressed(ebiten.KeyMinus) || g.keyJustPressed(ebiten.KeyKPSubtract) {
		g.adjustScreenBlocks(-1)
	}
}

func (g *game) drawWalk3D(screen *ebiten.Image) {
	now := g.renderStamp
	if now.IsZero() {
		now = time.Now()
	}
	rect := g.walkRenderViewportRect()
	if rect.Dx() >= g.viewW && rect.Dy() >= g.viewH && rect.Min.X == 0 && rect.Min.Y == 0 {
		g.prepareRenderStateAt(now)
		g.drawDoomBasic3D(screen)
		return
	}
	fullW := g.viewW
	fullH := g.viewH
	sub := screen.SubImage(rect).(*ebiten.Image)
	g.viewW = rect.Dx()
	g.viewH = rect.Dy()
	g.prepareRenderStateAt(now)
	g.drawDoomBasic3D(sub)
	g.viewW = fullW
	g.viewH = fullH
}

func (g *game) drawWalkOverlays(screen *ebiten.Image) {
	if g == nil {
		return
	}
	g.drawWeaponOverlay(screen)
	g.drawDoomStatusBar(screen)
	if g.isDead {
		g.drawDeathOverlay(screen)
	}
	g.drawFlashOverlay(screen)
	if g.useFlash > 0 {
		g.drawHUDMessage(screen, g.useText, 0, 0)
	}
	g.drawChatOverlay(screen)
	if g.paused {
		g.drawPauseOverlay(screen)
	}
	if g.opts.SourcePortMode && !g.opts.NoFPS {
		g.drawPerfOverlay(screen)
	}
	g.drawNetBandwidth(screen)
	if strings.TrimSpace(g.opts.RecordDemoPath) != "" {
		hud.DrawRecordingIndicator(screen, g.viewW, g.viewH, g.huTextWidth, g.drawHUTextAt)
	}
}

func (g *game) Draw(screen *ebiten.Image) {
	drawStart := time.Now()
	g.renderStamp = drawStart
	if g.opts.DemoScript != nil {
		g.demoBenchDraws++
	}
	g.frameUpload = 0
	g.perfInDraw = true
	defer func() { g.perfInDraw = false }()
	defer g.finishPerfCounter(drawStart)
	if g.mode != viewMap {
		debugPos := fmt.Sprintf(
			"pos=(%.2f, %.2f) ang=%.1f",
			float64(g.p.x)/fracUnit,
			float64(g.p.y)/fracUnit,
			normalizeDeg360(float64(g.p.angle)*360.0/4294967296.0),
		)
		aimSS := g.debugAimSS
		g.drawWalk3D(screen)
		if g.opts.Debug {
			ebitenutil.DebugPrintAt(screen, fmt.Sprintf("profile=%s", g.profileLabel()), 12, 28)
			if g.opts.SourcePortMode {
				ebitenutil.DebugPrintAt(screen, "renderer=doom-basic | TAB automap", 12, 12)
				ebitenutil.DebugPrintAt(screen, "TAB automap | J planes | F1 help", 12, 44)
			} else {
				ebitenutil.DebugPrintAt(screen, "renderer=doom-basic | TAB automap", 12, 12)
				ebitenutil.DebugPrintAt(screen, "TAB automap | F5 detail | F1 help", 12, 44)
			}
			planes3DOn := len(g.opts.FlatBank) > 0
			rect := g.walkRenderViewportRect()
			ebitenutil.DebugPrintAt(screen, fmt.Sprintf("planes3d=%t flats=%d detail=%dx%d render=%dx%d", planes3DOn, len(g.opts.FlatBank), g.viewW, g.viewH, rect.Dx(), rect.Dy()), 12, 60)
			ebitenutil.DebugPrintAt(screen, debugPos, 12, 76)
			ebitenutil.DebugPrintAt(screen, fmt.Sprintf("ss=%d", aimSS), 12, 92)
			if g.renderStageText != "" {
				ebitenutil.DebugPrintAt(screen, g.renderStageText, 12, 108)
			}
		}
		g.drawWalkOverlays(screen)
		return
	}
	presenter.Draw(screen, presenter.Inputs{
		DrawFloorTextures2D: g.opts.SourcePortMode && len(g.opts.FlatBank) > 0,
		DrawGrid:            g.showGrid,
		IsSourcePort:        g.opts.SourcePortMode,
		DrawThings:          presenter.ShouldDrawThings(g.parity.iddt),
		ShowLegend:          g.showLegend,
		HUDMessage:          g.useText,
		ShowHUDMessage:      g.useFlash > 0,
		IsDead:              g.isDead,
		Paused:              g.paused,
		ShowPerf:            !g.opts.NoFPS,
	}, presenter.Hooks{
		PrepareRenderState:  g.prepareRenderState,
		DrawFloorTextures2D: g.drawMapFloorTextures2D,
		DrawGrid:            g.drawGrid,
		DrawLines:           g.drawMapLines,
		DrawUseOverlays: func(screen *ebiten.Image) {
			g.drawUseSpecialLines(screen)
			g.drawUseTargetHighlight(screen)
		},
		DrawThings: g.drawThings,
		DrawActorOverlays: func(screen *ebiten.Image) {
			g.drawMarks(screen)
			g.drawPlayer(screen)
			g.drawPeerPlayers(screen)
		},
		DrawOverlays: func(screen *ebiten.Image, state mapview.RenderState) {
			if state.ShowLegend {
				presenter.DrawThingLegend(screen, presenter.LegendInputs{
					ViewWidth:            g.viewW,
					AntiAlias:            g.mapVectorAntiAlias(),
					SourcePortMode:       g.opts.SourcePortMode,
					SourcePortThingLabel: sourcePortThingRenderModeLabel(g.opts.SourcePortThingRenderMode),
				}, presenter.LegendColors{
					ThingPlayer:  presenter.ThingPlayerColor,
					ThingMonster: presenter.ThingMonsterColor,
					ThingItem:    presenter.ThingItemColor,
					ThingKey:     presenter.ThingKeyBlue,
					ThingMisc:    presenter.ThingMiscColor,
					WallOneSided: wallOneSided,
					WallFloor:    wallFloorChange,
					WallCeil:     wallCeilChange,
					WallTeleport: wallTeleporter,
					WallUse:      wallUseSpecial,
					WallHidden:   wallUnrevealed,
				})
			}
			if state.ShowHUDMessage {
				g.drawHUDMessage(screen, state.HUDMessage, 0, 0)
			}
			g.drawChatOverlay(screen)
			if state.IsDead {
				g.drawDeathOverlay(screen)
			}
			g.drawFlashOverlay(screen)
			if state.Paused {
				g.drawPauseOverlay(screen)
			}
			if state.ShowPerf {
				g.drawPerfOverlay(screen)
			}
			g.drawNetBandwidth(screen)
		},
	})
}

var inGamePauseMenuNames = [...]string{
	"M_NGAME",
	"M_OPTION",
	"M_LOADG",
	"M_SAVEG",
	"M_RDTHIS",
	"M_QUITG",
}

const (
	pauseMenuModeRoot = iota
	pauseMenuModeOptions
	pauseMenuModeSound
	pauseMenuModeVoice
	pauseMenuModeEpisode
	pauseMenuModeSkill
	pauseMenuModeKeybinds
)

var inGameEpisodeMenuNames = map[int]string{
	1: "M_EPI1",
	2: "M_EPI2",
	3: "M_EPI3",
	4: "M_EPI4",
}

func (g *game) togglePauseMenu() {
	if g == nil {
		return
	}
	g.pauseMenuActive = !g.pauseMenuActive
	g.paused = g.pauseMenuActive
	if g.pauseMenuActive {
		g.pauseMenuMode = pauseMenuModeRoot
		g.pauseMenuItemOn = 0
		g.pauseMenuOptionsOn = frontendOptionsSelectableRows[0]
		g.pauseMenuSoundOn = 0
		g.pauseMenuVoiceOn = 0
		g.pauseMenuEpisodeOn = 0
		g.pauseMenuSkillOn = max(0, normalizeSkillLevel(g.opts.SkillLevel)-1)
		g.pauseMenuKeybindRow = 0
		g.pauseMenuKeybindSlot = 0
		g.pauseMenuKeybindCapture = false
	} else {
		g.pauseMenuMode = pauseMenuModeRoot
	}
	if !g.pauseMenuActive {
		g.pauseMenuStatus = ""
		g.pauseMenuStatusTics = 0
		if g.mode == viewWalk {
			// Reset mouse baseline on resume to avoid turn spikes.
			g.mouseLookSet = false
			g.mouseLookSuppressTicks = detailMouseSuppressTicks
		}
	}
}

func (g *game) tickPauseMenu() {
	if g == nil || !g.pauseMenuActive {
		return
	}
	g.pauseMenuSkullAnimCounter++
	if g.pauseMenuSkullAnimCounter >= 8 {
		g.pauseMenuSkullAnimCounter = 0
		g.pauseMenuWhichSkull ^= 1
	}
	if g.pauseMenuStatusTics > 0 {
		g.pauseMenuStatusTics--
		if g.pauseMenuStatusTics == 0 {
			g.pauseMenuStatus = ""
		}
	}
	if g.keyJustPressed(ebiten.KeyEscape) {
		if g.pauseMenuMode != pauseMenuModeRoot {
			g.pauseMenuMode = pauseMenuModeRoot
			return
		}
		g.togglePauseMenu()
		return
	}
	switch g.pauseMenuMode {
	case pauseMenuModeOptions:
		if g.keyJustPressed(ebiten.KeyArrowUp) {
			g.pauseMenuOptionsOn = frontendNextSelectableOptionRow(g.pauseMenuOptionsOn, -1)
		}
		if g.keyJustPressed(ebiten.KeyArrowDown) {
			g.pauseMenuOptionsOn = frontendNextSelectableOptionRow(g.pauseMenuOptionsOn, 1)
		}
		if g.keyJustPressed(ebiten.KeyArrowLeft) {
			g.adjustPauseOption(-1)
		}
		if g.keyJustPressed(ebiten.KeyArrowRight) {
			g.adjustPauseOption(1)
		}
	case pauseMenuModeSound:
		if g.keyJustPressed(ebiten.KeyArrowUp) || g.keyJustPressed(ebiten.KeyArrowDown) {
			g.pauseMenuSoundOn ^= 1
		}
		if g.keyJustPressed(ebiten.KeyArrowLeft) {
			g.adjustPauseSound(-1)
		}
		if g.keyJustPressed(ebiten.KeyArrowRight) {
			g.adjustPauseSound(1)
		}
	case pauseMenuModeVoice:
		if g.keyJustPressed(ebiten.KeyArrowUp) || g.keyJustPressed(ebiten.KeyArrowDown) {
			dir := -1
			if g.keyJustPressed(ebiten.KeyArrowDown) {
				dir = 1
			}
			n := max(frontendVoiceMenuRowCount, 1)
			g.pauseMenuVoiceOn = (g.pauseMenuVoiceOn + dir + n) % n
		}
		if g.keyJustPressed(ebiten.KeyArrowLeft) {
			g.adjustPauseVoice(-1)
		}
		if g.keyJustPressed(ebiten.KeyArrowRight) {
			g.adjustPauseVoice(1)
		}
	case pauseMenuModeEpisode:
		if n := len(g.availableEpisodeChoices()); n > 0 {
			if g.keyJustPressed(ebiten.KeyArrowUp) {
				g.pauseMenuEpisodeOn = (g.pauseMenuEpisodeOn + n - 1) % n
			}
			if g.keyJustPressed(ebiten.KeyArrowDown) {
				g.pauseMenuEpisodeOn = (g.pauseMenuEpisodeOn + 1) % n
			}
		}
	case pauseMenuModeSkill:
		if g.keyJustPressed(ebiten.KeyArrowUp) {
			g.pauseMenuSkillOn = (g.pauseMenuSkillOn + len(frontendSkillMenuNames) - 1) % len(frontendSkillMenuNames)
		}
		if g.keyJustPressed(ebiten.KeyArrowDown) {
			g.pauseMenuSkillOn = (g.pauseMenuSkillOn + 1) % len(frontendSkillMenuNames)
		}
	case pauseMenuModeKeybinds:
		g.tickPauseKeybindMenu()
	default:
		if g.keyJustPressed(ebiten.KeyArrowUp) {
			g.pauseMenuItemOn = (g.pauseMenuItemOn + len(inGamePauseMenuNames) - 1) % len(inGamePauseMenuNames)
		}
		if g.keyJustPressed(ebiten.KeyArrowDown) {
			g.pauseMenuItemOn = (g.pauseMenuItemOn + 1) % len(inGamePauseMenuNames)
		}
	}
	if g.keyJustPressed(ebiten.KeyEnter) || g.keyJustPressed(ebiten.KeyKPEnter) {
		g.activatePauseMenuItem()
	}
}

func (g *game) activatePauseMenuItem() {
	if g == nil {
		return
	}
	switch g.pauseMenuMode {
	case pauseMenuModeOptions:
		g.activatePauseOptionsItem()
		return
	case pauseMenuModeSound:
		g.adjustPauseSound(1)
		return
	case pauseMenuModeVoice:
		g.adjustPauseVoice(1)
		return
	case pauseMenuModeEpisode:
		episodes := g.availableEpisodeChoices()
		if len(episodes) == 0 {
			return
		}
		if g.pauseMenuEpisodeOn < 0 || g.pauseMenuEpisodeOn >= len(episodes) {
			g.pauseMenuEpisodeOn = 0
		}
		g.pauseMenuSelectedEpisode = episodes[g.pauseMenuEpisodeOn]
		g.pauseMenuMode = pauseMenuModeSkill
		return
	case pauseMenuModeSkill:
		g.beginNewGameFromPauseMenu()
		return
	}
	switch g.pauseMenuItemOn {
	case 0:
		episodes := g.availableEpisodeChoices()
		if len(episodes) > 1 {
			g.pauseMenuEpisodeOn = 0
			g.pauseMenuSelectedEpisode = episodes[0]
			g.pauseMenuMode = pauseMenuModeEpisode
		} else {
			g.pauseMenuMode = pauseMenuModeSkill
		}
	case 1:
		g.pauseMenuMode = pauseMenuModeOptions
	case 2, 3:
		g.pauseMenuStatus = "MENU ITEM NOT WIRED YET"
		g.pauseMenuStatusTics = doomTicsPerSecond * 2
	case 4:
		g.pauseMenuActive = false
		g.paused = false
		g.pauseMenuMode = pauseMenuModeRoot
		g.pauseMenuStatus = ""
		g.pauseMenuStatusTics = 0
		g.readThisRequested = true
	case 5:
		g.quitPromptRequested = true
	}
}

func (g *game) availableEpisodeChoices() []int {
	if g == nil || len(g.opts.Episodes) == 0 {
		return nil
	}
	out := make([]int, 0, len(g.opts.Episodes))
	for _, ep := range g.opts.Episodes {
		if ep >= 1 && ep <= 4 {
			out = append(out, ep)
		}
	}
	return out
}

func (g *game) beginNewGameFromPauseMenu() {
	if g == nil || g.opts.NewGameLoader == nil {
		g.pauseMenuStatus = "NEW GAME NOT AVAILABLE"
		g.pauseMenuStatusTics = doomTicsPerSecond * 2
		g.pauseMenuMode = pauseMenuModeRoot
		return
	}
	startMap := "MAP01"
	if episodes := g.availableEpisodeChoices(); len(episodes) > 1 {
		ep := g.pauseMenuSelectedEpisode
		if ep == 0 {
			ep = episodes[0]
		}
		startMap = fmt.Sprintf("E%dM1", ep)
	}
	m, err := g.opts.NewGameLoader(startMap)
	if err != nil || m == nil {
		g.pauseMenuStatus = "NEW GAME LOAD FAILED"
		g.pauseMenuStatusTics = doomTicsPerSecond * 2
		g.pauseMenuMode = pauseMenuModeRoot
		return
	}
	g.newGameRequestedMap = m
	g.newGameRequestedSkill = g.pauseMenuSkillOn + 1
	g.pauseMenuActive = false
	g.paused = false
	g.pauseMenuMode = pauseMenuModeRoot
}

func (g *game) pauseSourcePortDetailLabel() string {
	if g == nil {
		return "FULL"
	}
	switch g.sourcePortDetailDivisor() {
	case 1:
		return "FULL"
	case 2:
		return "1/2"
	case 3:
		return "1/3"
	case 4:
		return "1/4"
	default:
		return fmt.Sprintf("1/%d", g.sourcePortDetailDivisor())
	}
}

func (g *game) adjustPauseOption(dir int) {
	if g == nil || dir == 0 {
		return
	}
	switch g.pauseMenuOptionsOn {
	case 0:
		g.hudMessagesEnabled = !g.hudMessagesEnabled
		g.publishRuntimeSettingsIfChanged()
	case 1:
		g.adjustScreenBlocks(dir)
	case 2:
		g.adjustHUDScale(dir)
	case 3:
		g.opts.NoFPS = !g.opts.NoFPS
	case 4:
		_, mouseThermoCount, _ := g.pauseMouseSensitivityLayout(36, "MOUSE SENSITIVITY")
		next := frontendNextMouseSensitivityForCount(g.opts.MouseLookSpeed, dir, mouseThermoCount)
		if next != g.opts.MouseLookSpeed {
			g.opts.MouseLookSpeed = next
			g.publishRuntimeSettingsIfChanged()
		}
	}
}

func (g *game) cyclePauseOption() {
	if g == nil {
		return
	}
	switch g.pauseMenuOptionsOn {
	case 0:
		g.hudMessagesEnabled = !g.hudMessagesEnabled
		g.publishRuntimeSettingsIfChanged()
	case 1:
		minBlocks, maxBlocks := allowedScreenBlocksRange(g.opts)
		if g.screenBlocks >= maxBlocks {
			g.screenBlocks = minBlocks
			g.setHUDMessage(fmt.Sprintf("Status bar %s", g.screenSizeLabel()), 70)
			return
		}
		g.adjustScreenBlocks(1)
	case 2:
		if len(sourcePortHUDScaleSteps) == 0 {
			return
		}
		if g.hudScaleStep >= len(sourcePortHUDScaleSteps)-1 {
			g.hudScaleStep = 0
			g.statusBarCacheValid = false
			g.setHUDMessage(fmt.Sprintf("HUD size %s", g.hudScaleLabel()), 70)
			return
		}
		g.adjustHUDScale(1)
	case 3:
		g.opts.NoFPS = !g.opts.NoFPS
	case 4:
		_, mouseThermoCount, _ := g.pauseMouseSensitivityLayout(36, "MOUSE SENSITIVITY")
		next := frontendNextMouseSensitivityForCount(g.opts.MouseLookSpeed, 1, mouseThermoCount)
		if next == g.opts.MouseLookSpeed {
			next = frontendMouseSensitivitySpeedForDotCount(0, mouseThermoCount)
		}
		g.opts.MouseLookSpeed = next
		g.publishRuntimeSettingsIfChanged()
	case frontendOptionsRowSound:
		g.pauseMenuMode = pauseMenuModeSound
	case frontendOptionsRowKeybinds:
		g.pauseMenuMode = pauseMenuModeKeybinds
	}
}

func (g *game) adjustPauseSound(dir int) {
	if g == nil || dir == 0 {
		return
	}
	switch g.pauseMenuSoundOn {
	case 0:
		next := clampVolume(g.opts.SFXVolume + float64(dir)*0.1)
		if next != g.opts.SFXVolume {
			g.opts.SFXVolume = next
			if g.snd != nil {
				g.snd.setSFXVolume(next)
			}
			g.publishRuntimeSettingsIfChanged()
		}
	default:
		next := clampVolume(g.opts.MusicVolume + float64(dir)*0.1)
		if next != g.opts.MusicVolume {
			g.opts.MusicVolume = next
			g.publishRuntimeSettingsIfChanged()
		}
	}
}

func (g *game) adjustPauseVoice(dir int) {
	if g == nil || dir == 0 {
		return
	}
	switch g.pauseMenuVoiceOn {
	case frontendVoiceMenuRowPreset:
		if err := (&sessionGame{opts: g.opts, g: g}).frontendChangeVoicePreset(dir); err != nil {
			g.pauseMenuStatus = strings.ToUpper(err.Error())
			g.pauseMenuStatusTics = doomTicsPerSecond * 2
			return
		}
	case frontendVoiceMenuRowG726Bits:
		switch normalizeVoiceCodecChoice(g.opts.VoiceCodec) {
		case "silk":
			cur := voiceBitrateChoiceIndex(g.opts.VoiceBitrate)
			n := len(frontendVoiceBitrateChoices)
			next := (cur + dir + n) % n
			if g.opts.OnVoiceSettingsChanged != nil {
				if err := g.opts.OnVoiceSettingsChanged(runtimecfg.VoiceSettings{
					Codec:         g.opts.VoiceCodec,
					G726Bits:      g.opts.VoiceG726BitsPerSample,
					Bitrate:       frontendVoiceBitrateChoices[next],
					SampleRate:    g.opts.VoiceSampleRate,
					AGCEnabled:    g.opts.VoiceAGCEnabled,
					GateEnabled:   g.opts.VoiceGateEnabled,
					GateThreshold: g.opts.VoiceGateThreshold,
				}); err != nil {
					g.pauseMenuStatus = strings.ToUpper(err.Error())
					g.pauseMenuStatusTics = doomTicsPerSecond * 2
					return
				}
			}
			g.opts.VoiceBitrate = frontendVoiceBitrateChoices[next]
		case "g726":
			cur := voiceG726BitsChoiceIndex(g.opts.VoiceG726BitsPerSample)
			n := len(frontendVoiceG726BitsChoices)
			next := (cur + dir + n) % n
			if g.opts.OnVoiceSettingsChanged != nil {
				if err := g.opts.OnVoiceSettingsChanged(runtimecfg.VoiceSettings{
					Codec:         g.opts.VoiceCodec,
					G726Bits:      frontendVoiceG726BitsChoices[next],
					Bitrate:       g.opts.VoiceBitrate,
					SampleRate:    g.opts.VoiceSampleRate,
					AGCEnabled:    g.opts.VoiceAGCEnabled,
					GateEnabled:   g.opts.VoiceGateEnabled,
					GateThreshold: g.opts.VoiceGateThreshold,
				}); err != nil {
					g.pauseMenuStatus = strings.ToUpper(err.Error())
					g.pauseMenuStatusTics = doomTicsPerSecond * 2
					return
				}
			}
			g.opts.VoiceG726BitsPerSample = frontendVoiceG726BitsChoices[next]
		default:
			return
		}
	case frontendVoiceMenuRowSampleRate:
		cur := voiceSampleRateChoiceIndex(g.opts.VoiceSampleRate)
		n := len(frontendVoiceSampleRateChoices)
		next := (cur + dir + n) % n
		if g.opts.OnVoiceSettingsChanged != nil {
			if err := g.opts.OnVoiceSettingsChanged(runtimecfg.VoiceSettings{
				Codec:         g.opts.VoiceCodec,
				G726Bits:      g.opts.VoiceG726BitsPerSample,
				Bitrate:       g.opts.VoiceBitrate,
				SampleRate:    frontendVoiceSampleRateChoices[next],
				AGCEnabled:    g.opts.VoiceAGCEnabled,
				GateEnabled:   g.opts.VoiceGateEnabled,
				GateThreshold: g.opts.VoiceGateThreshold,
			}); err != nil {
				g.pauseMenuStatus = strings.ToUpper(err.Error())
				g.pauseMenuStatusTics = doomTicsPerSecond * 2
				return
			}
		}
		g.opts.VoiceSampleRate = frontendVoiceSampleRateChoices[next]
	case frontendVoiceMenuRowAGC:
		next := !g.opts.VoiceAGCEnabled
		if g.opts.OnVoiceSettingsChanged != nil {
			if err := g.opts.OnVoiceSettingsChanged(runtimecfg.VoiceSettings{
				Codec:         g.opts.VoiceCodec,
				G726Bits:      g.opts.VoiceG726BitsPerSample,
				Bitrate:       g.opts.VoiceBitrate,
				SampleRate:    g.opts.VoiceSampleRate,
				AGCEnabled:    next,
				GateEnabled:   g.opts.VoiceGateEnabled,
				GateThreshold: g.opts.VoiceGateThreshold,
			}); err != nil {
				g.pauseMenuStatus = strings.ToUpper(err.Error())
				g.pauseMenuStatusTics = doomTicsPerSecond * 2
				return
			}
		}
		g.opts.VoiceAGCEnabled = next
	case frontendVoiceMenuRowGate:
		next := !g.opts.VoiceGateEnabled
		if g.opts.OnVoiceSettingsChanged != nil {
			if err := g.opts.OnVoiceSettingsChanged(runtimecfg.VoiceSettings{
				Codec:         g.opts.VoiceCodec,
				G726Bits:      g.opts.VoiceG726BitsPerSample,
				Bitrate:       g.opts.VoiceBitrate,
				SampleRate:    g.opts.VoiceSampleRate,
				AGCEnabled:    g.opts.VoiceAGCEnabled,
				GateEnabled:   next,
				GateThreshold: g.opts.VoiceGateThreshold,
			}); err != nil {
				g.pauseMenuStatus = strings.ToUpper(err.Error())
				g.pauseMenuStatusTics = doomTicsPerSecond * 2
				return
			}
		}
		g.opts.VoiceGateEnabled = next
	case frontendVoiceMenuRowGateThresh:
		cur := voiceGateThresholdChoiceIndex(g.opts.VoiceGateThreshold)
		n := len(frontendVoiceGateThresholdChoices)
		next := (cur + dir + n) % n
		if g.opts.OnVoiceSettingsChanged != nil {
			if err := g.opts.OnVoiceSettingsChanged(runtimecfg.VoiceSettings{
				Codec:         g.opts.VoiceCodec,
				G726Bits:      g.opts.VoiceG726BitsPerSample,
				Bitrate:       g.opts.VoiceBitrate,
				SampleRate:    g.opts.VoiceSampleRate,
				AGCEnabled:    g.opts.VoiceAGCEnabled,
				GateEnabled:   g.opts.VoiceGateEnabled,
				GateThreshold: frontendVoiceGateThresholdChoices[next],
			}); err != nil {
				g.pauseMenuStatus = strings.ToUpper(err.Error())
				g.pauseMenuStatusTics = doomTicsPerSecond * 2
				return
			}
		}
		g.opts.VoiceGateThreshold = frontendVoiceGateThresholdChoices[next]
	}
}

func (g *game) activatePauseOptionsItem() {
	if g == nil {
		return
	}
	if g.pauseMenuOptionsOn == frontendOptionsRowSound {
		g.pauseMenuMode = pauseMenuModeSound
		return
	}
	if g.pauseMenuOptionsOn == frontendOptionsRowKeybinds {
		g.pauseMenuMode = pauseMenuModeKeybinds
		return
	}
	g.cyclePauseOption()
}

func (g *game) profileLabel() string {
	if g.opts.SourcePortMode {
		return "sourceport"
	}
	return "doom"
}

func (g *game) emitSoundEvent(ev soundEvent) {
	if want := runtimeDebugEnv("GD_DEBUG_SOUND_TIC"); want != "" {
		var wantTic int
		if _, err := fmt.Sscanf(want, "%d", &wantTic); err == nil {
			if g.demoTick-1 == wantTic || g.worldTic == wantTic {
				if pc, file, line, ok := runtime.Caller(1); ok {
					name := "<unknown>"
					if fn := runtime.FuncForPC(pc); fn != nil {
						name = fn.Name()
					}
					fmt.Printf("sound-enqueue side=gd tic=%d world=%d event=%s positioned=false caller=%s file=%s:%d\n",
						g.demoTick-1, g.worldTic, soundEventDebugName(ev), name, file, line)
				}
			}
		}
	}
	g.soundQueue = append(g.soundQueue, ev)
	g.soundQueueOrigin = append(g.soundQueueOrigin, queuedSoundOrigin{})
}

func (g *game) emitSoundEventAt(ev soundEvent, x, y int64) {
	if want := runtimeDebugEnv("GD_DEBUG_SOUND_TIC"); want != "" {
		var wantTic int
		if _, err := fmt.Sscanf(want, "%d", &wantTic); err == nil {
			if g.demoTick-1 == wantTic || g.worldTic == wantTic {
				if pc, file, line, ok := runtime.Caller(1); ok {
					name := "<unknown>"
					if fn := runtime.FuncForPC(pc); fn != nil {
						name = fn.Name()
					}
					fmt.Printf("sound-enqueue side=gd tic=%d world=%d event=%s positioned=true origin=(%d,%d) caller=%s file=%s:%d\n",
						g.demoTick-1, g.worldTic, soundEventDebugName(ev), x, y, name, file, line)
				}
			}
		}
	}
	g.soundQueue = append(g.soundQueue, ev)
	g.soundQueueOrigin = append(g.soundQueueOrigin, queuedSoundOrigin{x: x, y: y, positioned: true})
}

func (g *game) emitSoundEventDelayed(ev soundEvent, tics int) {
	g.emitSoundEventDelayedAt(ev, tics, 0, 0, false)
}

func (g *game) emitSoundEventDelayedAt(ev soundEvent, tics int, x, y int64, positioned bool) {
	if tics <= 0 {
		if positioned {
			g.emitSoundEventAt(ev, x, y)
		} else {
			g.emitSoundEvent(ev)
		}
		return
	}
	g.delayedSfx = append(g.delayedSfx, delayedSoundEvent{ev: ev, tics: tics, x: x, y: y, positioned: positioned})
}

func (g *game) emitMonsterDeathSoundDelayedAt(typ int16, tics int, x, y int64) {
	if g == nil {
		return
	}
	if tics <= 0 {
		g.emitSoundEventAt(monsterDeathSoundEventVariant(typ), x, y)
		return
	}
	g.delayedSfx = append(g.delayedSfx, delayedSoundEvent{
		tics:       tics,
		x:          x,
		y:          y,
		positioned: true,
		monsterTyp: typ,
		deathSound: true,
	})
}

func (g *game) emitDoorSectorSound(sec int, ev soundEvent) {
	if x, y, ok := g.sectorSoundOrigin(sec); ok {
		g.emitSoundEventAt(ev, x, y)
		return
	}
	g.emitSoundEvent(ev)
}

func (g *game) sectorSoundOrigin(sec int) (int64, int64, bool) {
	if g == nil || sec < 0 || sec >= len(g.sectorBBox) {
		return 0, 0, false
	}
	sb := g.sectorBBox[sec]
	if sb.maxX <= sb.minX || sb.maxY <= sb.minY {
		return 0, 0, false
	}
	x := (sb.minX + sb.maxX) * 0.5
	y := (sb.minY + sb.maxY) * 0.5
	return int64(math.Round(x * fracUnit)), int64(math.Round(y * fracUnit)), true
}

func soundMapUsesFullClip(name mapdata.MapName) bool {
	s := strings.ToUpper(strings.TrimSpace(string(name)))
	if len(s) >= 4 && s[0] == 'E' && s[2] == 'M' {
		return s[3] == '8'
	}
	if len(s) == 5 && strings.HasPrefix(s, "MAP") {
		return s[3:] == "08"
	}
	return false
}

func maxMonsterVocalSoundsPerFlush() int {
	if !isWASMBuild() {
		return 0
	}
	return 6
}

func (g *game) clearPendingSoundState() {
	if g == nil {
		return
	}
	g.soundQueue = g.soundQueue[:0]
	g.soundQueueOrigin = g.soundQueueOrigin[:0]
	g.delayedSfx = g.delayedSfx[:0]
	if g.snd != nil {
		g.snd.stopAll()
	}
}

func (g *game) tickDelayedSounds() {
	if len(g.delayedSfx) == 0 {
		return
	}
	keep := g.delayedSfx[:0]
	for _, d := range g.delayedSfx {
		d.tics--
		if d.tics <= 0 {
			if d.deathSound {
				g.emitSoundEventAt(monsterDeathSoundEventVariant(d.monsterTyp), d.x, d.y)
				continue
			}
			if d.positioned {
				g.emitSoundEventAt(d.ev, d.x, d.y)
			} else {
				g.emitSoundEvent(d.ev)
			}
			continue
		}
		keep = append(keep, d)
	}
	g.delayedSfx = keep
}

func (g *game) tickDelayedSwitchReverts() {
	if len(g.delayedSwitchReverts) == 0 {
		return
	}
	keep := g.delayedSwitchReverts[:0]
	for _, s := range g.delayedSwitchReverts {
		s.tics--
		if s.tics <= 0 {
			if s.sidedef >= 0 && s.sidedef < len(g.m.Sidedefs) {
				sd := &g.m.Sidedefs[s.sidedef]
				if sd.Top != s.top {
					g.beginSwitchTextureBlend(s.sidedef, switchTextureSlotTop, sd.Top, s.top)
					sd.Top = s.top
				}
				if sd.Bottom != s.bottom {
					g.beginSwitchTextureBlend(s.sidedef, switchTextureSlotBottom, sd.Bottom, s.bottom)
					sd.Bottom = s.bottom
				}
				if sd.Mid != s.mid {
					g.beginSwitchTextureBlend(s.sidedef, switchTextureSlotMid, sd.Mid, s.mid)
					sd.Mid = s.mid
				}
			}
			continue
		}
		keep = append(keep, s)
	}
	g.delayedSwitchReverts = keep
}

func (g *game) setHUDMessage(msg string, tics int) {
	if !g.hudMessagesEnabled {
		return
	}
	g.useText = msg
	if !g.opts.SourcePortMode {
		// Doom HU messages use a fixed timeout (HU_MSGTIMEOUT).
		tics = huMsgTimeout
	}
	g.useFlash = tics
}

func (g *game) applyThingSpawnFiltering() {
	for i, th := range g.m.Things {
		if !thingSpawnsInSession(th, g.opts.SkillLevel, g.opts.GameMode, g.opts.ShowNoSkillItems, g.opts.ShowAllItems, g.opts.NoMonsters) {
			g.thingCollected[i] = true
		}
	}
}

func (g *game) flushSoundEvents() {
	monsterVocalBudget := maxMonsterVocalSoundsPerFlush()
	monsterVocalCount := 0
	for idx, ev := range g.soundQueue {
		if monsterVocalBudget > 0 && isMonsterVocalSound(ev) {
			if monsterVocalCount >= monsterVocalBudget {
				continue
			}
			monsterVocalCount++
		}
		origin := queuedSoundOrigin{}
		if idx >= 0 && idx < len(g.soundQueueOrigin) {
			origin = g.soundQueueOrigin[idx]
		}
		if want := runtimeDebugEnv("GD_DEBUG_SOUND_TIC"); want != "" {
			var wantTic int
			if _, err := fmt.Sscanf(want, "%d", &wantTic); err == nil {
				if g.demoTick-1 == wantTic || g.worldTic == wantTic {
					fmt.Printf("sound-debug side=gd tic=%d world=%d event=%s positioned=%t origin=(%d,%d)\n",
						g.demoTick-1, g.worldTic, soundEventDebugName(ev), origin.positioned, origin.x, origin.y)
				}
			}
		}
		if g.snd != nil {
			g.snd.playEventSpatial(ev, origin, g.p.x, g.p.y, g.p.angle, soundMapUsesFullClip(g.m.Name))
		} else {
			((*soundSystem)(nil)).playEventSpatial(ev, origin, g.p.x, g.p.y, g.p.angle, soundMapUsesFullClip(g.m.Name))
		}
	}
	if g.snd != nil {
		g.snd.tick()
	}
	g.soundQueue = g.soundQueue[:0]
	g.soundQueueOrigin = g.soundQueueOrigin[:0]
}

func (g *game) mapVectorAntiAlias() bool {
	// Faithful mode targets Doom-like crisp map vectors.
	return g.opts.SourcePortMode
}

func (g *game) addMark() {
	x, y := g.State.Camera()
	id, ok := g.marks.Add(x, y)
	if !ok {
		g.setHUDMessage("Marks Full", 70)
		return
	}
	g.setHUDMessage(fmt.Sprintf("Marked Spot %d", id), 70)
}

func (g *game) clearMarks() {
	g.marks.Clear()
	g.setHUDMessage("Marks Cleared", 70)
}

func (g *game) toggleBigMap() {
	centerX, centerY, _, _ := boundsViewMetrics(g.bounds)
	if g.bigMap.Toggle(&g.State, centerX, centerY) {
		g.setHUDMessage("Big Map ON", 70)
		return
	}
	g.setHUDMessage("Big Map OFF", 70)
}

func (g *game) drawGrid(screen *ebiten.Image) {
	mapview.DrawGrid(screen, g.State.Snapshot(), g.viewport(), g.worldToScreen, g.mapVectorAntiAlias())
}

func (g *game) drawThings(screen *ebiten.Image) {
	aa := g.mapVectorAntiAlias()
	view := g.State.Snapshot()
	items := make([]mapview.ThingDrawItem, 0, len(g.m.Things))
	for i, th := range g.m.Things {
		if i >= 0 && i < len(g.thingCollected) && g.thingCollected[i] {
			continue
		}
		if !g.automapThingRevealed(i, th) {
			continue
		}
		fx, fy := g.thingPosFixed(i, th)
		x := float64(fx) / fracUnit
		y := float64(fy) / fracUnit
		sx, sy := g.worldToScreen(x, y)
		item := mapview.ThingDrawItem{ScreenX: sx, ScreenY: sy}
		if img, w, h, ok := g.mapThingSprite(i, th); ok {
			item.Sprite = img
			item.SpriteW = w
			item.SpriteH = h
			item.SpriteTarget = presenter.ThingGlyphSize(view.ZoomLevel()) * 2.4
			items = append(items, item)
			continue
		}
		angle := presenter.WorldAngleToGlyphAngle(g.thingWorldAngle(i, th))
		if g.mapRotationActive() {
			angle = presenter.RelativeWorldAngle(g.thingWorldAngle(i, th), g.renderAngle)
		}
		style := presenter.StyleForThingType(th.Type, isPlayerStart(th.Type), isMonster(th.Type))
		item.DrawGlyph = func(screen *ebiten.Image, x, y float64, zoom float64, antiAlias bool) {
			presenter.DrawThingGlyph(screen, style, x, y, angle, presenter.ThingGlyphSize(zoom), antiAlias)
		}
		items = append(items, item)
	}
	mapview.DrawThings2D(screen, items, view.ZoomLevel(), aa)
}

func (g *game) shouldDrawMapThingSprite(th mapdata.Thing) bool {
	if g == nil || !g.opts.SourcePortMode {
		return false
	}
	switch sourcePortThingRenderMode(normalizeSourcePortThingRenderMode(g.opts.SourcePortThingRenderMode, g.opts.SourcePortMode)) {
	case sourcePortThingRenderItems:
		return presenter.IsItemOrPickupType(th.Type)
	case sourcePortThingRenderSprites:
		return true
	default:
		return false
	}
}

func (g *game) mapThingSprite(thingIdx int, th mapdata.Thing) (*ebiten.Image, int, int, bool) {
	if !g.shouldDrawMapThingSprite(th) {
		return nil, 0, 0, false
	}
	name := g.mapThingSpriteName(thingIdx, th)
	if name == "" {
		return nil, 0, 0, false
	}
	img, w, h, _, _, ok := g.spritePatch(name)
	if !ok || w <= 0 || h <= 0 {
		return nil, 0, 0, false
	}
	return img, w, h, true
}

func (g *game) mapThingSpriteName(thingIdx int, th mapdata.Thing) string {
	if g == nil {
		return ""
	}
	if isPlayerStart(th.Type) {
		return "PLAYN0"
	}
	if isMonster(th.Type) {
		if !g.monsterVisibleAfterDeath(thingIdx, th.Type) {
			return ""
		}
		name, _ := g.monsterSpriteNameForView(
			thingIdx,
			th,
			g.worldTic,
			float64(g.p.x)/fracUnit,
			float64(g.p.y)/fracUnit,
		)
		return name
	}
	if !g.opts.SourcePortThingBlendFrames {
		return g.runtimeWorldThingSpriteNameScaled(thingIdx, th, g.worldTic, 1)
	}
	animTickUnits, animUnitsPerTic := g.worldThingAnimTickUnits()
	return g.runtimeWorldThingSpriteNameScaled(thingIdx, th, animTickUnits, animUnitsPerTic)
}

func (g *game) worldThingDrawnInView(thingIdx int, th mapdata.Thing) bool {
	if g == nil {
		return false
	}
	if !g.thingActiveInSession(thingIdx) {
		return false
	}
	if isMonster(th.Type) {
		if !g.monsterVisibleAfterDeath(thingIdx, th.Type) {
			return false
		}
		name, _ := g.monsterSpriteNameForView(
			thingIdx,
			th,
			g.worldTic,
			float64(g.p.x)/fracUnit,
			float64(g.p.y)/fracUnit,
		)
		tex, ok := g.monsterSpriteTexture(name)
		return ok && tex.Width > 0 && tex.Height > 0
	}
	animTickUnits, animUnitsPerTic := g.worldThingAnimTickUnits()
	name := g.runtimeWorldThingSpriteNameScaled(thingIdx, th, animTickUnits, animUnitsPerTic)
	if name == "" {
		return false
	}
	tex, ok := g.monsterSpriteTexture(name)
	return ok && tex.Width > 0 && tex.Height > 0
}

func (g *game) drawMarks(screen *ebiten.Image) {
	mapview.DrawMarks(screen, g.marks.Items(), g.worldToScreen, g.mapVectorAntiAlias())
}

func (g *game) drawPlayer(screen *ebiten.Image) {
	px := g.renderPX
	py := g.renderPY
	sx, sy := g.worldToScreen(px, py)
	if g.mapRotationActive() {
		// Heading-follow: keep icon fixed-up in screen-space.
		g.drawPlayerArrowScreen(screen, sx, sy, math.Pi/2)
		return
	}
	ang := angleToRadians(g.renderAngle)
	g.drawPlayerArrowWorld(screen, px, py, ang, playerColor)
}

func (g *game) drawPlayerArrowWorld(screen *ebiten.Image, px, py, ang float64, clr color.Color) {
	ca := math.Cos(ang)
	sa := math.Sin(ang)
	for _, seg := range doomPlayerArrow {
		ax := seg[0]*ca - seg[1]*sa
		ay := seg[0]*sa + seg[1]*ca
		bx := seg[2]*ca - seg[3]*sa
		by := seg[2]*sa + seg[3]*ca
		x1, y1 := g.worldToScreen(px+ax, py+ay)
		x2, y2 := g.worldToScreen(px+bx, py+by)
		vector.StrokeLine(screen, float32(x1), float32(y1), float32(x2), float32(y2), 2, clr, g.mapVectorAntiAlias())
	}
}

func (g *game) drawPlayerArrowScreen(screen *ebiten.Image, sx, sy, ang float64) {
	ca := math.Cos(ang)
	sa := math.Sin(ang)
	scale := g.State.Snapshot().ZoomLevel()
	for _, seg := range doomPlayerArrow {
		ax := seg[0]*ca - seg[1]*sa
		ay := seg[0]*sa + seg[1]*ca
		bx := seg[2]*ca - seg[3]*sa
		by := seg[2]*sa + seg[3]*ca
		x1 := sx + ax*scale
		y1 := sy - ay*scale
		x2 := sx + bx*scale
		y2 := sy - by*scale
		vector.StrokeLine(screen, float32(x1), float32(y1), float32(x2), float32(y2), 2, playerColor, g.mapVectorAntiAlias())
	}
}

func (g *game) drawPeerPlayers(screen *ebiten.Image) {
	if len(g.peerStarts) == 0 {
		return
	}
	for _, ps := range g.peerStarts {
		px := float64(ps.x) / fracUnit
		py := float64(ps.y) / fracUnit
		ang := angleToRadians(ps.angle)
		g.drawPlayerArrowWorld(screen, px, py, ang, otherPlayerColor)
	}
}

func (g *game) drawUseTargetHighlight(screen *ebiten.Image) {
	lineIdx, tr := g.peekUseTargetLine()
	if tr != useTraceSpecial || lineIdx < 0 || lineIdx >= len(g.physForLine) {
		return
	}
	pi := g.physForLine[lineIdx]
	if pi < 0 || pi >= len(g.lines) {
		return
	}
	pl := g.lines[pi]
	x1, y1 := g.worldToScreen(float64(pl.x1)/fracUnit, float64(pl.y1)/fracUnit)
	x2, y2 := g.worldToScreen(float64(pl.x2)/fracUnit, float64(pl.y2)/fracUnit)
	mapview.DrawSegments(screen, []mapview.Segment{{
		X1: x1, Y1: y1, X2: x2, Y2: y2, Width: 3.0, Color: useTargetColor,
	}}, g.mapVectorAntiAlias())
}

func (g *game) drawDoomBasic3D(screen *ebiten.Image) {
	camX := g.renderPX
	camY := g.renderPY
	camAng := angleToRadians(g.renderAngle)
	ca := math.Cos(camAng)
	sa := math.Sin(camAng)
	eyeZ := g.playerEyeZ()
	focal := doomFocalLength(g.viewW)
	near := 2.0
	g.beginSkyLayerFrame()

	ceilClr, floorClr := g.basicPlaneColors()
	g.ensureWallLayer()
	g.prepareFrameSkyState(camAng, focal)

	wallTop, wallBottom, ceilingClip, floorClip := g.ensure3DFrameBuffers()
	planesEnabled := len(g.opts.FlatBank) > 0
	planeOrder := g.beginPlane3DFrame(g.viewW)
	solid := g.beginSolid3DFrame()
	stageStart := time.Now()
	prepass := g.buildWallSegPrepassParallel(g.visibleSegIndicesPseudo3D(), camX, camY, ca, sa, focal, near)
	g.addRenderStageDur(renderStageWallPrepass, time.Since(stageStart))
	maskedMids := g.ensureMaskedMidSegScratch(len(prepass))
	stageStart = time.Now()
	for _, pp := range prepass {
		si := pp.segIdx
		if si < 0 || si >= len(g.m.Segs) {
			continue
		}
		if !pp.prepass.OK {
			if pp.prepass.LogReason != "" {
				g.logWallCull(si, pp.prepass.LogReason, pp.prepass.LogZ1, pp.prepass.LogZ2, pp.prepass.LogX1, pp.prepass.LogX2)
			}
			continue
		}
		ld := pp.ld
		wallLightBias := doomWallLightBias(&ld, g.m.Vertexes)
		var frontSideDef *mapdata.Sidedef
		if pp.frontSideDefIdx >= 0 && pp.frontSideDefIdx < len(g.m.Sidedefs) {
			frontSideDef = &g.m.Sidedefs[pp.frontSideDefIdx]
		}
		front, back := g.segSectors(si)
		if front == nil {
			continue
		}
		frontIdx, backIdx := g.segSectorIndices(si)
		frontLight := g.sectorLightForRender(frontIdx, front)
		frontFloor := float64(front.FloorHeight)
		frontCeil := float64(front.CeilingHeight)
		if fz, cz, ok := g.sectorHeightRenderSnapshot(frontIdx); ok {
			frontFloor = float64(fz) / fracUnit
			frontCeil = float64(cz) / fracUnit
		}
		backFloor := 0.0
		backCeil := 0.0
		if back != nil {
			backFloor = float64(back.FloorHeight)
			backCeil = float64(back.CeilingHeight)
			if fz, cz, ok := g.sectorHeightRenderSnapshot(backIdx); ok {
				backFloor = float64(fz) / fracUnit
				backCeil = float64(cz) / fracUnit
			}
		}
		ws := g.classifyWallPortalCached(frontIdx, front, back, backIdx, eyeZ, frontFloor, frontCeil, backFloor, backCeil)
		worldTop := ws.WorldTop
		worldBottom := ws.WorldBottom
		worldHigh := ws.WorldHigh
		worldLow := ws.WorldLow
		maskedWorldHigh := math.Min(worldTop, worldHigh)
		maskedWorldLow := math.Max(worldBottom, worldLow)
		topWall := ws.TopWall
		bottomWall := ws.BottomWall
		markCeiling := ws.MarkCeiling
		markFloor := ws.MarkFloor
		solidWall := ws.SolidWall
		if solidWall && g.wallSpanRejectEnabled() && solidFullyCoveredFast(solid, pp.prepass.Projection.MinX, pp.prepass.Projection.MaxX) {
			g.logWallCull(si, "OCCLUDED", pp.prepass.LogZ1, pp.prepass.LogZ2, pp.prepass.LogX1, pp.prepass.LogX2)
			continue
		}
		var midTex wallTextureBlendSample
		var topTex wallTextureBlendSample
		var botTex wallTextureBlendSample
		hasMidTex := false
		hasTopTex := false
		hasBotTex := false
		midTexMid := 0.0
		topTexMid := 0.0
		botTexMid := 0.0
		texUOffset := wallSpecialScrollXOffset(ld.Special, g.worldTic)
		if frontSideDef != nil {
			texUOffset += float64(frontSideDef.TextureOffset)
			rowOffset := float64(frontSideDef.RowOffset)
			midTex, hasMidTex = g.wallTextureBlend(frontSideDef.Mid, pp.frontSideDefIdx, switchTextureSlotMid)
			if hasMidTex && midTex.from != nil {
				if back != nil {
					if (ld.Flags & mlDontPegBottom) != 0 {
						midTexMid = math.Max(frontFloor, backFloor) + float64(midTex.from.Height) - eyeZ
					} else {
						midTexMid = math.Min(frontCeil, backCeil) - eyeZ
					}
				} else if (ld.Flags & mlDontPegBottom) != 0 {
					midTexMid = frontFloor + float64(midTex.from.Height) - eyeZ
				} else {
					midTexMid = frontCeil - eyeZ
				}
				midTexMid += rowOffset
			}
			if topWall {
				topTex, hasTopTex = g.wallTextureBlend(frontSideDef.Top, pp.frontSideDefIdx, switchTextureSlotTop)
				if hasTopTex && topTex.from != nil {
					if (ld.Flags & mlDontPegTop) != 0 {
						topTexMid = frontCeil - eyeZ
					} else if back != nil {
						topTexMid = backCeil + float64(topTex.from.Height) - eyeZ
					} else {
						topTexMid = frontCeil - eyeZ
					}
					topTexMid += rowOffset
				}
			}
			if bottomWall {
				botTex, hasBotTex = g.wallTextureBlend(frontSideDef.Bottom, pp.frontSideDefIdx, switchTextureSlotBottom)
				if hasBotTex && botTex.from != nil {
					if (ld.Flags & mlDontPegBottom) != 0 {
						botTexMid = frontCeil - eyeZ
					} else if back != nil {
						botTexMid = backFloor - eyeZ
					} else {
						botTexMid = frontFloor - eyeZ
					}
					botTexMid += rowOffset
				}
			}
		}

		var floorPlane *plane3DVisplane
		var ceilPlane *plane3DVisplane
		if planesEnabled {
			var created bool
			floorPlane, created = g.ensurePlane3DForRangeCached(g.plane3DKeyForSectorCached(frontIdx, front, true), pp.prepass.Projection.MinX, pp.prepass.Projection.MaxX, g.viewW)
			if created && floorPlane != nil {
				planeOrder = append(planeOrder, floorPlane)
			}
			ceilPlane, created = g.ensurePlane3DForRangeCached(g.plane3DKeyForSectorCached(frontIdx, front, false), pp.prepass.Projection.MinX, pp.prepass.Projection.MaxX, g.viewW)
			if created && ceilPlane != nil {
				planeOrder = append(planeOrder, ceilPlane)
			}
		}

		visibleRanges := g.solidClipScratch[:0]
		if solidWall && g.wallSpanClipEnabled() {
			visibleRanges = clipRangeAgainstSolidSpans(pp.prepass.Projection.MinX, pp.prepass.Projection.MaxX, solid, visibleRanges)
		} else {
			visibleRanges = append(visibleRanges, solidSpan{L: pp.prepass.Projection.MinX, R: pp.prepass.Projection.MaxX})
		}
		g.solidClipScratch = visibleRanges
		if len(visibleRanges) == 0 {
			g.logWallCull(si, "OCCLUDED", pp.prepass.LogZ1, pp.prepass.LogZ2, pp.prepass.LogX1, pp.prepass.LogX2)
			continue
		}
		if solidWall && g.wallSliceOcclusionEnabled() {
			allOcc := true
			for _, vis := range visibleRanges {
				visOcc := g.wallSliceRangeTriFullyOccludedByWallsOnly(pp, vis.L, vis.R, worldTop, worldBottom, focal)
				if !visOcc {
					allOcc = false
					break
				}
			}
			if allOcc {
				g.logWallCull(si, "OCCLUDED", pp.prepass.LogZ1, pp.prepass.LogZ2, pp.prepass.LogX1, pp.prepass.LogX2)
				continue
			}
		}
		for _, vis := range visibleRanges {
			proj := wallSegPrepassProjection(pp)
			for x := vis.L; x <= vis.R; x++ {
				f, texU, ok := scene.ProjectedWallSampleAtX(proj, x)
				if !ok {
					continue
				}
				texU += texUOffset

				yl := int(math.Ceil(float64(g.viewH)/2 - (worldTop/f)*focal))
				if yl < ceilingClip[x]+1 {
					yl = ceilingClip[x] + 1
				}
				if markCeiling && planesEnabled && ceilPlane != nil {
					top := ceilingClip[x] + 1
					bottom := yl - 1
					if bottom >= floorClip[x] {
						bottom = floorClip[x] - 1
					}
					markPlane3DColumnRange(ceilPlane, x, top, bottom, ceilingClip, floorClip)
				}

				yh := int(math.Floor(float64(g.viewH)/2 - (worldBottom/f)*focal))
				if yh >= floorClip[x] {
					yh = floorClip[x] - 1
				}
				if markFloor && planesEnabled && floorPlane != nil {
					top := yh + 1
					bottom := floorClip[x] - 1
					if top <= ceilingClip[x] {
						top = ceilingClip[x] + 1
					}
					markPlane3DColumnRange(floorPlane, x, top, bottom, ceilingClip, floorClip)
				}
				if !solidWall {
					openTop := int(math.Ceil(float64(g.viewH)/2 - (worldHigh/f)*focal))
					openBottom := int(math.Floor(float64(g.viewH)/2 - (worldLow/f)*focal))
					if openTop < yl {
						openTop = yl
					}
					if openBottom > yh {
						openBottom = yh
					}
					g.appendSpritePortalColumnGap(x, openTop, openBottom, encodeDepthQ(f))
				}

				if solidWall {
					tex := midTex
					texMid := midTexMid
					texName := ""
					if frontSideDef != nil {
						texName = frontSideDef.Mid
					}
					// Closed two-sided doors often have upper/lower textures but no middle texture.
					if back != nil && tex.from == nil {
						if topWall && hasTopTex {
							tex = topTex
							texMid = topTexMid
							if frontSideDef != nil {
								texName = frontSideDef.Top
							}
						} else if bottomWall && hasBotTex {
							tex = botTex
							texMid = botTexMid
							if frontSideDef != nil {
								texName = frontSideDef.Bottom
							}
						}
					}
					if tex.from == nil {
						if texName != "" && texName != "-" {
							panic("missing required solid wall texture: " + texName)
						}
					} else {
						g.drawBasicWallColumn(wallTop, wallBottom, x, yl, yh, f, frontLight, wallLightBias, texU, texMid, focal, tex)
					}
					g.setWallDepthColumnClosedQ(x, encodeDepthQ(f))
					g.markSpriteClipColumnClosed(x, encodeDepthQ(f))
					ceilingClip[x] = g.viewH
					floorClip[x] = -1
					continue
				}
				if topWall {
					mid := int(math.Floor(float64(g.viewH)/2 - (worldHigh/f)*focal))
					if mid >= floorClip[x] {
						mid = floorClip[x] - 1
					}
					if mid >= yl {
						if topTex.from == nil {
							name := ""
							if frontSideDef != nil {
								name = frontSideDef.Top
							}
							if name != "" && name != "-" {
								panic("missing required top wall texture: " + name)
							}
						} else {
							g.drawBasicWallColumn(wallTop, wallBottom, x, yl, mid, f, frontLight, wallLightBias, texU, topTexMid, focal, topTex)
						}
						ceilingClip[x] = mid
					} else {
						ceilingClip[x] = yl - 1
					}
				} else if markCeiling {
					ceilingClip[x] = yl - 1
				}

				if bottomWall {
					mid := int(math.Ceil(float64(g.viewH)/2 - (worldLow/f)*focal))
					if mid <= ceilingClip[x] {
						mid = ceilingClip[x] + 1
					}
					if mid <= yh {
						if botTex.from == nil {
							name := ""
							if frontSideDef != nil {
								name = frontSideDef.Bottom
							}
							if name != "" && name != "-" {
								panic("missing required bottom wall texture: " + name)
							}
						} else {
							g.drawBasicWallColumn(wallTop, wallBottom, x, mid, yh, f, frontLight, wallLightBias, texU, botTexMid, focal, botTex)
						}
						floorClip[x] = mid
					} else {
						floorClip[x] = yh + 1
					}
				} else if markFloor {
					floorClip[x] = yh + 1
				}
			}
		}
		if back != nil && hasMidTex {
			maskedVisibleRanges := g.solidClipScratch[:0]
			if g.wallSpanClipEnabled() {
				maskedVisibleRanges = clipRangeAgainstSolidSpans(pp.prepass.Projection.MinX, pp.prepass.Projection.MaxX, solid, maskedVisibleRanges)
			} else {
				maskedVisibleRanges = append(maskedVisibleRanges, solidSpan{L: pp.prepass.Projection.MinX, R: pp.prepass.Projection.MaxX})
			}
			g.solidClipScratch = maskedVisibleRanges
			for _, vis := range maskedVisibleRanges {
				if vis.L > vis.R {
					continue
				}
				centerDepth, dist, ok := maskedMidDepthSamples(pp.prepass.Projection, vis.L, vis.R)
				if !ok {
					continue
				}
				occY0 := int(math.Ceil(float64(g.viewH)*0.5 - (maskedWorldHigh/centerDepth)*focal))
				occY1 := int(math.Floor(float64(g.viewH)*0.5 - (maskedWorldLow/centerDepth)*focal))
				if occY0 < 0 {
					occY0 = 0
				}
				if occY1 >= g.viewH {
					occY1 = g.viewH - 1
				}
				occOK := occY0 <= occY1
				if occOK && g.wallClipBBoxFullyOccludedByWallsOnly(vis.L, vis.R, occY0, occY1, encodeDepthQ(centerDepth)) {
					continue
				}
				maskedMids = append(maskedMids, maskedMidSeg{
					MaskedMidSeg: scene.MaskedMidSeg{
						Dist:       dist,
						X0:         vis.L,
						X1:         vis.R,
						Projection: pp.prepass.Projection,
						WorldHigh:  maskedWorldHigh,
						WorldLow:   maskedWorldLow,
						TexUOff:    texUOffset,
						TexMid:     midTexMid,
					},
					tex:              midTex,
					light:            frontLight,
					occlusionY0:      int16(occY0),
					occlusionY1:      int16(occY1),
					occlusionDepthQ:  encodeDepthQ(centerDepth),
					hasOcclusionBBox: occOK,
				})
			}
		}

		if solidWall {
			solid = addSolidSpan(solid, pp.prepass.Projection.MinX, pp.prepass.Projection.MaxX)
		}
	}
	g.addRenderStageDur(renderStageWallTraverse, time.Since(stageStart))
	g.maskedMidSegsScratch = maskedMids
	g.solid3DBuf = solid
	// Masked midtextures render in their own pass and should not occlude sprites.
	g.finalizeMaskedClipColumns()
	planePassReady := planesEnabled && hasMarkedPlane3DData(planeOrder)
	if planePassReady {
		g.collectCutoutItems(camX, camY, camAng, focal, near)
		g.buildBillboardPlaneOccludersFromQueue()
	} else {
		g.clearBillboardPlaneOccluderRows()
	}
	usedSkyLayer := false
	if planePassReady {
		usedSkyLayer = g.drawDoomBasicTexturedPlanesVisplanePass(g.wallPix, camX, camY, ca, sa, eyeZ, focal, ceilClr, floorClr, planeOrder)
	}
	if !planePassReady {
		stageStart = time.Now()
		g.collectCutoutItems(camX, camY, camAng, focal, near)
		g.addRenderStageDur(renderStageBillboards, time.Since(stageStart))
	}
	g.appendMaskedMidSegsToCutoutItems()
	g.sortCutoutItemsFrontToBack()
	g.clearCutoutCoverage()
	stageStart = time.Now()
	for _, it := range g.billboardQueueScratch {
		if it.shadow {
			continue
		}
		if it.debugOverlay {
			continue
		}
		g.drawCutoutItem(it, focal)
	}
	for _, it := range g.billboardQueueScratch {
		if !it.shadow || it.debugOverlay {
			continue
		}
		g.drawCutoutItem(it, focal)
	}
	for _, it := range g.billboardQueueScratch {
		if !it.debugOverlay {
			continue
		}
		g.drawCutoutItem(it, focal)
	}
	g.drawHitscanPuffsToBuffer(camX, camY, camAng, focal, near)
	g.addRenderStageDur(renderStageBillboards, time.Since(stageStart))
	g.billboardQueueScratch = g.billboardQueueScratch[:0]
	if g.lowDetailMode() {
		g.duplicateLowDetailColumns()
	}
	if usedSkyLayer {
		g.drawSkyLayerFrame(screen)
	}
	g.writePixelsTimed(g.wallLayer, g.wallPix)
	screen.DrawImage(g.wallLayer, nil)
}

func (g *game) sectorLightForRender(secIdx int, sec *mapdata.Sector) int16 {
	if g != nil && secIdx >= 0 {
		return g.sectorLightLevelCached(secIdx)
	}
	if sec != nil {
		return sec.Light
	}
	return 160
}

func (g *game) sectorsLightDifferForRender(frontSectorIdx, backSectorIdx int, front, back *mapdata.Sector) bool {
	if !doomSectorLighting {
		return false
	}
	return g.sectorLightForRender(frontSectorIdx, front) != g.sectorLightForRender(backSectorIdx, back)
}

func classifyWallPortal(front, back *mapdata.Sector, eyeZ, frontFloor, frontCeil, backFloor, backCeil float64) scene.WallPortalState {
	return (*game)(nil).classifyWallPortalCached(-1, front, back, -1, eyeZ, frontFloor, frontCeil, backFloor, backCeil)
}

func (g *game) classifyWallPortalCached(frontIdx int, front, back *mapdata.Sector, backIdx int, eyeZ, frontFloor, frontCeil, backFloor, backCeil float64) scene.WallPortalState {
	if front == nil {
		return scene.WallPortalState{}
	}
	in := scene.WallPortalInput{
		FrontFloor: frontFloor, FrontCeil: frontCeil, BackFloor: backFloor, BackCeil: backCeil, EyeZ: eyeZ,
		FrontFloorFlat:     normalizeFlatName(front.FloorPic),
		FrontCeilingFlat:   normalizeFlatName(front.CeilingPic),
		FrontLight:         g.sectorLightForRender(frontIdx, front),
		BackExists:         back != nil,
		DoomSectorLighting: doomSectorLighting,
		IsFrontCeilingSky:  isSkyFlatName(front.CeilingPic),
	}
	if back != nil {
		in.BackFloorFlat = normalizeFlatName(back.FloorPic)
		in.BackCeilingFlat = normalizeFlatName(back.CeilingPic)
		in.BackLight = g.sectorLightForRender(backIdx, back)
		in.IsBackCeilingSky = isSkyFlatName(back.CeilingPic)
	}
	return scene.ClassifyWallPortal(in)
}

func (g *game) lowDetailMode() bool {
	return !g.opts.SourcePortMode && g.detailLevel == 1
}

func (g *game) duplicateLowDetailColumns() {
	if g.viewW <= 1 || g.viewH <= 0 || len(g.wallPix32) != g.viewW*g.viewH {
		return
	}
	for y := 0; y < g.viewH; y++ {
		row := y * g.viewW
		for x := 1; x < g.viewW; x += 2 {
			g.wallPix32[row+x] = g.wallPix32[row+x-1]
		}
	}
}

func hasMarkedPlane3DData(planes []*plane3DVisplane) bool {
	for _, pl := range planes {
		if pl == nil || pl.minX > pl.maxX {
			continue
		}
		start := pl.minX
		if start < -1 {
			start = -1
		}
		stop := pl.maxX
		if stop > len(pl.top)-2 {
			stop = len(pl.top) - 2
		}
		for x := start; x <= stop; x++ {
			ix := x + 1
			if ix >= 0 && ix < len(pl.top) && pl.top[ix] != plane3DUnset {
				return true
			}
		}
	}
	return false
}

func (g *game) plane3DKeyForSector(sec *mapdata.Sector, floor bool) plane3DKey {
	return g.plane3DKeyForSectorCached(-1, sec, floor)
}

func (g *game) plane3DKeyForSectorCached(secIdx int, sec *mapdata.Sector, floor bool) plane3DKey {
	key := plane3DKey{
		light: 160,
		floor: floor,
	}
	if sec == nil {
		return key
	}
	key.light = g.sectorLightForRender(secIdx, sec)
	pic := sec.CeilingPic
	key.height = sec.CeilingHeight
	if floor {
		pic = sec.FloorPic
		key.height = sec.FloorHeight
	}
	if !floor && isSkyFlatName(pic) {
		key.sky = true
		key.height = 0
		key.light = 0
		key.flatID = g.flatIDForResolvedName("SKY")
		return key
	}
	key.flatID = g.flatIDForName(pic)
	return key
}

func (g *game) drawBasicWallColumn(wallTop, wallBottom []int, x, y0, y1 int, depth float64, sectorLight int16, lightBias int, texU, texMid, focal float64, tex wallTextureBlendSample) {
	if x < 0 || x >= g.viewW || y0 > y1 {
		return
	}
	if y0 < 0 {
		y0 = 0
	}
	if y1 >= g.viewH {
		y1 = g.viewH - 1
	}
	if y0 > y1 {
		return
	}
	if y0 < wallTop[x] {
		wallTop[x] = y0
	}
	if y1 > wallBottom[x] {
		wallBottom[x] = y1
	}
	depthQ := encodeDepthQ(depth)
	g.setWallDepthColumnMinQ(x, y0, y1, depthQ)
	g.appendSpriteClipColumnSpan(x, y0, y1, depthQ)
	shadeMul := sectorDistanceShadeMul(sectorLight, depth, doomLightingEnabled)
	doomRow := 0
	if doomLightingEnabled {
		doomRow = doomWallLightRow(sectorLight, lightBias, depth, focal)
		if !doomColormapEnabled {
			if g != nil && !g.opts.SourcePortMode {
				shadeMul = doomShadeMulFromRow(doomRow)
			} else {
				shadeMul = doomShadeMulFromRowF(doomWallLightRowF(sectorLight, lightBias, depth, focal))
			}
		}
	}
	if row, ok := g.playerFixedColormapRow(); ok {
		doomRow = row
	}
	g.drawBasicWallColumnTextured(x, y0, y1, depth, texU, texMid, focal, tex, shadeMul, doomRow)
}

func (g *game) setWallDepthColumnMinQ(x, y0, y1 int, depthQ uint16) {
	if g == nil || x < 0 || x >= len(g.wallDepthQCol) {
		return
	}
	if depthQ < g.wallDepthQCol[x] {
		g.wallDepthQCol[x] = depthQ
		if x < len(g.wallDepthTopCol) && x < len(g.wallDepthBottomCol) {
			g.wallDepthTopCol[x] = y0
			g.wallDepthBottomCol[x] = y1
		}
		if x < len(g.wallDepthClosedCol) {
			g.wallDepthClosedCol[x] = false
		}
	}
}

func (g *game) setWallDepthColumnClosedQ(x int, depthQ uint16) {
	if g == nil || x < 0 || x >= len(g.wallDepthQCol) || x >= len(g.wallDepthClosedCol) {
		return
	}
	// Mark full-column closure only when this wall is the nearest occluder.
	if depthQ < g.wallDepthQCol[x] {
		g.wallDepthQCol[x] = depthQ
		g.wallDepthClosedCol[x] = true
		return
	}
	if depthQ == g.wallDepthQCol[x] {
		g.wallDepthClosedCol[x] = true
	}
}

func (g *game) appendSpriteClipColumnSpan(x, y0, y1 int, depthQ uint16) {
	if g == nil || x < 0 || x >= len(g.maskedClipCols) || y0 > y1 {
		return
	}
	if y0 < 0 {
		y0 = 0
	}
	if y1 >= g.viewH {
		y1 = g.viewH - 1
	}
	if y0 > y1 {
		return
	}
	g.appendMaskedClipSpan(x, scene.MaskedClipSpan{
		Y0:      int16(y0),
		Y1:      int16(y1),
		DepthQ:  depthQ,
		Closed:  false,
		HasOpen: false,
	})
}

func (g *game) markSpriteClipColumnClosed(x int, depthQ uint16) {
	if g == nil || x < 0 || x >= len(g.maskedClipCols) {
		return
	}
	g.appendMaskedClipSpan(x, scene.MaskedClipSpan{
		DepthQ:  depthQ,
		Closed:  true,
		HasOpen: false,
	})
}

func (g *game) appendSpritePortalColumnGap(x, openY0, openY1 int, depthQ uint16) {
	if g == nil || x < 0 || x >= len(g.maskedClipCols) {
		return
	}
	if openY0 < 0 {
		openY0 = 0
	}
	if openY1 >= g.viewH {
		openY1 = g.viewH - 1
	}
	g.appendMaskedClipSpan(x, scene.MaskedClipSpan{
		OpenY0:  int16(openY0),
		OpenY1:  int16(openY1),
		DepthQ:  depthQ,
		Closed:  false,
		HasOpen: true,
	})
}

func (g *game) appendMaskedClipSpan(x int, sp scene.MaskedClipSpan) {
	if g == nil || x < 0 || x >= len(g.maskedClipCols) {
		return
	}
	col := g.maskedClipCols[x]
	if len(col) == 0 {
		if cap(col) == 0 {
			col = make([]scene.MaskedClipSpan, 0, 4)
		}
		if x >= 0 && x < len(g.maskedClipFirstDepthQ) {
			g.maskedClipFirstDepthQ[x] = sp.DepthQ
		}
		if x >= 0 && x < len(g.maskedClipLastDepthQ) {
			g.maskedClipLastDepthQ[x] = sp.DepthQ
		}
		g.maskedClipCols[x] = append(col, sp)
		return
	}
	if x >= 0 && x < len(g.maskedClipLastDepthQ) {
		lastDepthQ := g.maskedClipLastDepthQ[x]
		g.maskedClipLastDepthQ[x] = sp.DepthQ
		if sp.DepthQ >= lastDepthQ {
			if len(col) == cap(col) {
				col = slices.Grow(col, 1)
			}
			g.maskedClipCols[x] = append(col, sp)
			return
		}
	}
	insertAt := len(col)
	for lo, hi := 0, len(col); lo < hi; {
		mid := lo + (hi-lo)/2
		if col[mid].DepthQ <= sp.DepthQ {
			lo = mid + 1
		} else {
			hi = mid
		}
		insertAt = lo
	}
	if len(col) == cap(col) {
		col = slices.Grow(col, 1)
	}
	col = append(col, scene.MaskedClipSpan{})
	copy(col[insertAt+1:], col[insertAt:])
	col[insertAt] = sp
	if insertAt == 0 && x >= 0 && x < len(g.maskedClipFirstDepthQ) {
		g.maskedClipFirstDepthQ[x] = sp.DepthQ
	}
	g.maskedClipCols[x] = col
}

func (g *game) wallOcclusionEnabled() bool {
	return g != nil && !g.opts.DisableWallOcclusion
}

func (g *game) wallSpanRejectEnabled() bool {
	return g.wallOcclusionEnabled() && !g.opts.DisableWallSpanReject
}

func (g *game) wallSpanClipEnabled() bool {
	return g.wallOcclusionEnabled() && !g.opts.DisableWallSpanClip
}

func (g *game) wallSliceOcclusionEnabled() bool {
	return g.wallOcclusionEnabled() && !g.opts.DisableWallSliceOcclusion
}

func (g *game) billboardClippingEnabled() bool {
	return g != nil && !g.opts.DisableBillboardClipping
}

func (g *game) writeWallPixel(i int, p uint32) {
	g.wallPix32[i] = p
}

func (g *game) writeWallPixelPair(i int, p0, p1 uint32) {
	g.wallPix32[i] = p0
	g.wallPix32[i+1] = p1
}

func (g *game) clearCutoutCoverage() {
	if g == nil || len(g.cutoutCoverageBits) == 0 {
		return
	}
	clear(g.cutoutCoverageBits)
}

func (g *game) prepareFrameSkyState(camAng, focal float64) {
	if g == nil {
		return
	}
	g.frameSkyLayerEnabled = false
	g.frameSkyTex32 = nil
	g.frameSkyTexW = 0
	g.frameSkyColU = nil
	g.frameSkyRowV = nil
	if g.viewW <= 0 || g.viewH <= 0 {
		return
	}
	skyTexKey, skyTex, skyOK := g.runtimeSkyTextureEntryForMap(g.m.Name)
	if !skyOK {
		return
	}
	skyTexH := effectiveSkyTexHeight(skyTex)
	skyColU, skyRowV := g.buildSkyLookupParallel(g.viewW, g.viewH, focal, camAng, skyTex.Width, skyTexH)
	if len(skyColU) != g.viewW || len(skyRowV) != g.viewH {
		return
	}
	skyTex32 := skyTex.RGBA32
	if len(skyTex32) != skyTex.Width*skyTex.Height {
		if len(skyTex.RGBA) != skyTex.Width*skyTex.Height*4 || len(skyTex.RGBA) < 4 {
			return
		}
		skyTex32 = unsafe.Slice((*uint32)(unsafe.Pointer(unsafe.SliceData(skyTex.RGBA))), len(skyTex.RGBA)/4)
	}
	g.frameSkyTex32 = skyTex32
	g.frameSkyTexW = skyTex.Width
	g.frameSkyColU = skyColU
	g.frameSkyRowV = skyRowV
	g.frameSkyLayerEnabled = g.enableSkyLayerFrame(camAng, focal, skyTexKey, skyTex, skyTexH)
}

func (g *game) cutoutCoveredAtIndex(i int) bool {
	if g == nil || i < 0 {
		return false
	}
	word := i >> 6
	if word < 0 || word >= len(g.cutoutCoverageBits) {
		return false
	}
	return (g.cutoutCoverageBits[word] & (uint64(1) << uint(i&63))) != 0
}

func (g *game) markCutoutCoveredAtIndex(i int) {
	if g == nil || i < 0 {
		return
	}
	word := i >> 6
	if word < 0 || word >= len(g.cutoutCoverageBits) {
		return
	}
	g.cutoutCoverageBits[word] |= uint64(1) << uint(i&63)
}

func (g *game) appendUncoveredCutoutSpans(row, x0, x1 int, out []solidSpan) []solidSpan {
	if g == nil || g.viewW <= 0 || row < 0 || x0 > x1 {
		return out
	}
	if x0 < 0 {
		x0 = 0
	}
	if x1 >= g.viewW {
		x1 = g.viewW - 1
	}
	if x0 > x1 {
		return out
	}
	rowBase := row
	i0 := rowBase + x0
	i1 := rowBase + x1
	word0 := i0 >> 6
	word1 := i1 >> 6
	start := x0
	for word := word0; word <= word1; word++ {
		wordStartX := x0
		if word != word0 {
			wordStartX = ((word << 6) - rowBase)
		}
		wordEndX := x1
		if word != word1 {
			wordEndX = (((word + 1) << 6) - rowBase) - 1
		}
		bits := g.cutoutCoverageBits[word]
		startBit := 0
		if word == word0 {
			startBit = i0 & 63
			bits |= (uint64(1) << uint(startBit)) - 1
		}
		endBit := 63
		if word == word1 {
			endBit = i1 & 63
			if endBit < 63 {
				bits |= ^uint64(0) << uint(endBit+1)
			}
		}
		if bits == 0 {
			continue
		}
		if bits == ^uint64(0) {
			if start <= wordStartX-1 {
				out = append(out, solidSpan{L: start, R: wordStartX - 1})
			}
			start = wordEndX + 1
			continue
		}
		x := wordStartX
		for x <= wordEndX {
			bit := x + rowBase - (word << 6)
			if ((bits >> uint(bit)) & 1) == 0 {
				x++
				continue
			}
			if start <= x-1 {
				out = append(out, solidSpan{L: start, R: x - 1})
			}
			for x <= wordEndX {
				bit = x + rowBase - (word << 6)
				if ((bits >> uint(bit)) & 1) == 0 {
					break
				}
				x++
			}
			start = x
		}
	}
	if start <= x1 {
		out = append(out, solidSpan{L: start, R: x1})
	}
	return out
}

func (g *game) markCutoutRowSpanCovered(row, x0, x1 int) {
	if g == nil || x0 > x1 || row < 0 || len(g.cutoutCoverageBits) == 0 {
		return
	}
	i0 := row + x0
	i1 := row + x1
	word0 := i0 >> 6
	word1 := i1 >> 6
	if word0 == word1 {
		width := x1 - x0 + 1
		mask := ^uint64(0)
		if width < 64 {
			mask = (uint64(1) << uint(width)) - 1
		}
		g.cutoutCoverageBits[word0] |= mask << uint(i0&63)
		return
	}
	g.cutoutCoverageBits[word0] |= ^uint64(0) << uint(i0&63)
	for word := word0 + 1; word < word1; word++ {
		g.cutoutCoverageBits[word] = ^uint64(0)
	}
	lastBits := (i1 & 63) + 1
	lastMask := ^uint64(0)
	if lastBits < 64 {
		lastMask = (uint64(1) << uint(lastBits)) - 1
	}
	g.cutoutCoverageBits[word1] |= lastMask
}

func (g *game) cutoutVisibleSpansForRow(row int, spans []solidSpan) []solidSpan {
	if len(spans) == 0 {
		return nil
	}
	if g == nil || len(g.cutoutCoverageBits) == 0 || g.viewW <= 0 {
		return spans
	}
	out := g.cutoutSpanScratch[:0]
	for _, sp := range spans {
		if sp.L > sp.R {
			continue
		}
		out = g.appendUncoveredCutoutSpans(row, sp.L, sp.R, out)
	}
	g.cutoutSpanScratch = out
	return out
}

func (g *game) cutoutMaskRectFullyCovered(x0, x1, y0, y1 int) bool {
	if g == nil || g.viewW <= 0 || len(g.cutoutCoverageBits) == 0 || x0 > x1 || y0 > y1 {
		return false
	}
	if x0 < 0 {
		x0 = 0
	}
	if y0 < 0 {
		y0 = 0
	}
	if x1 >= g.viewW {
		x1 = g.viewW - 1
	}
	if y1 >= g.viewH {
		y1 = g.viewH - 1
	}
	if x0 > x1 || y0 > y1 {
		return false
	}
	for y := y0; y <= y1; y++ {
		rowBase := y * g.viewW
		i0 := rowBase + x0
		i1 := rowBase + x1
		word0 := i0 >> 6
		word1 := i1 >> 6
		if word0 == word1 {
			bits := g.cutoutCoverageBits[word0]
			width := x1 - x0 + 1
			mask := ^uint64(0)
			if width < 64 {
				mask = (uint64(1) << uint(width)) - 1
			}
			mask <<= uint(i0 & 63)
			if bits&mask != mask {
				return false
			}
			continue
		}
		firstMask := ^uint64(0) << uint(i0&63)
		if g.cutoutCoverageBits[word0]&firstMask != firstMask {
			return false
		}
		for word := word0 + 1; word < word1; word++ {
			if g.cutoutCoverageBits[word] != ^uint64(0) {
				return false
			}
		}
		lastBits := (i1 & 63) + 1
		lastMask := ^uint64(0)
		if lastBits < 64 {
			lastMask = (uint64(1) << uint(lastBits)) - 1
		}
		if g.cutoutCoverageBits[word1]&lastMask != lastMask {
			return false
		}
	}
	return true
}

func (g *game) cutoutColumnSpanFullyCovered(pixI, rowStridePix, count int) bool {
	if g == nil || count <= 0 {
		return false
	}
	for i := 0; i < count; i++ {
		if !g.cutoutCoveredAtIndex(pixI) {
			return false
		}
		pixI += rowStridePix
	}
	return true
}

func (g *game) cutoutColumnVisibleSpansFullyCovered(x, rowStridePix int, spans []solidSpan) bool {
	if g == nil || x < 0 || x >= g.viewW || rowStridePix <= 0 || len(spans) == 0 {
		return false
	}
	coveredAny := false
	for _, sp := range spans {
		if sp.L > sp.R {
			continue
		}
		coveredAny = true
		if !g.cutoutColumnSpanFullyCovered(sp.L*rowStridePix+x, rowStridePix, sp.R-sp.L+1) {
			return false
		}
	}
	return coveredAny
}

func (g *game) spriteOpaqueRectsFullyCoveredByCutout(rects []spriteOpaqueRect, texW int, flip bool, dstX, dstY, scale float64, clipTop, clipBottom, viewW, viewH int) bool {
	if g == nil || len(rects) == 0 || texW <= 0 || scale <= 0 {
		return false
	}
	coveredAny := false
	for _, rect := range rects {
		if flip {
			rect = flipSpriteOpaqueRectX(rect, texW)
		}
		x0, x1, y0, y1, ok := spriteRectScreenBounds(rect, dstX, dstY, scale, clipTop, clipBottom, viewW, viewH)
		if !ok {
			continue
		}
		coveredAny = true
		if !g.cutoutMaskRectFullyCovered(x0, x1, y0, y1) {
			return false
		}
	}
	return coveredAny
}

func (g *game) projectedOpaqueRectsFullyCoveredByCutout(rects []projectedOpaqueRect) bool {
	if len(rects) == 0 {
		return false
	}
	for _, rect := range rects {
		if !g.cutoutMaskRectFullyCovered(rect.x0(), rect.x1(), rect.y0(), rect.y1()) {
			return false
		}
	}
	return true
}

func (g *game) rowFullyOccludedDepthQ(depthQ uint16, rowBase, x0, x1 int) bool {
	if !g.billboardClippingEnabled() {
		return false
	}
	if g == nil || g.viewW <= 0 || x1 < x0 || rowBase < 0 {
		return false
	}
	y := rowBase / g.viewW
	for x := x0; x <= x1; x++ {
		if !g.spriteWallClipOccludedAtXYDepth(x, y, depthQ) {
			return false
		}
	}
	return true
}

func solidSpanContainsX(spans []solidSpan, x int) bool {
	for _, s := range spans {
		if x < s.L {
			return false
		}
		if x <= s.R {
			return true
		}
	}
	return false
}

func solidFullyCoveredFast(spans []solidSpan, l, r int) bool {
	if l > r {
		return true
	}
	// Two-triangle style fast reject on the 1D projection: left, right, center.
	mid := (l + r) >> 1
	if !solidSpanContainsX(spans, l) || !solidSpanContainsX(spans, r) || !solidSpanContainsX(spans, mid) {
		return false
	}
	return solidFullyCovered(spans, l, r)
}

func (g *game) rowFullyOccludedByWallsFastDepthQ(depthQ uint16, rowBase, x0, x1 int) bool {
	if g == nil || g.viewW <= 0 || x1 < x0 || rowBase < 0 {
		return false
	}
	mid := (x0 + x1) >> 1
	y := rowBase / g.viewW
	if !g.wallClipPointOccludedByWallsOnly(x0, y, depthQ) ||
		!g.wallClipPointOccludedByWallsOnly(x1, y, depthQ) ||
		!g.wallClipPointOccludedByWallsOnly(mid, y, depthQ) {
		return false
	}
	for x := x0; x <= x1; x++ {
		if !g.wallClipPointOccludedByWallsOnly(x, y, depthQ) {
			return false
		}
	}
	return true
}

func (g *game) spriteOccludedAt(depth float64, idx int, planeBias float64) bool {
	if !g.billboardClippingEnabled() {
		return false
	}
	_ = planeBias
	return g.spriteWallClipOccludedAtIndexDepth(idx, encodeDepthQ(depth))
}

func (g *game) spriteWallClipOccludedAtIndexDepth(idx int, depthQ uint16) bool {
	if g == nil || g.viewW <= 0 || idx < 0 {
		return true
	}
	x := idx % g.viewW
	y := idx / g.viewW
	return g.spriteWallClipOccludedAtXYDepth(x, y, depthQ)
}

func (g *game) spriteWallClipOccludedAtXYDepth(x, y int, depthQ uint16) bool {
	if !g.billboardClippingEnabled() {
		return false
	}
	if g == nil || x < 0 || y < 0 || y >= g.viewH {
		return true
	}
	if x >= len(g.wallDepthQCol) {
		return false
	}
	var masked []scene.MaskedClipSpan
	if x >= 0 && x < len(g.maskedClipCols) {
		masked = g.maskedClipCols[x]
	}
	if scene.WallDepthColumnOccludesPoint(g.wallDepthColumnAt(x), y, depthQ) {
		return true
	}
	return maskedClipColumnOccludesPointSorted(masked, y, depthQ)
}

func (g *game) spriteWallClipColumnOccludedBBox(x, y0, y1 int, depthQ uint16) bool {
	if g == nil || x < 0 || x >= g.viewW || y0 > y1 {
		return true
	}
	if y0 < 0 {
		y0 = 0
	}
	if y1 >= g.viewH {
		y1 = g.viewH - 1
	}
	if y0 > y1 {
		return true
	}
	if x >= len(g.wallDepthQCol) {
		return false
	}
	return scene.SpriteColumnOccludesBBox(g.wallDepthColumnAt(x), y0, y1, depthQ)
}

func (g *game) wallClipColumnOccludedBBoxByWallsOnly(x, y0, y1 int, depthQ uint16) bool {
	if g == nil || x < 0 || x >= g.viewW || y0 > y1 {
		return true
	}
	if y0 < 0 {
		y0 = 0
	}
	if y1 >= g.viewH {
		y1 = g.viewH - 1
	}
	if y0 > y1 {
		return true
	}
	if x >= len(g.wallDepthQCol) {
		return false
	}
	return scene.SpriteColumnOccludesBBox(g.wallDepthColumnAt(x), y0, y1, depthQ)
}

func (g *game) wallClipPointOccludedByWallsOnly(x, y int, depthQ uint16) bool {
	if g == nil || g.viewW <= 0 || g.viewH <= 0 {
		return true
	}
	if x < 0 || x >= g.viewW || y < 0 || y >= g.viewH {
		return true
	}
	return g.wallClipColumnOccludedBBoxByWallsOnly(x, y, y, depthQ)
}

func (g *game) wallClipBBoxFullyOccludedByWallsOnly(x0, x1, y0, y1 int, depthQ uint16) bool {
	if g == nil {
		return true
	}
	return scene.BBoxFullyOccluded(x0, x1, y0, y1, g.viewW, g.viewH, func(x, y0, y1 int) bool {
		return g.wallClipColumnOccludedBBoxByWallsOnly(x, y0, y1, depthQ)
	})
}

func (g *game) spriteWallClipBBoxFullyOccluded(x0, x1, y0, y1 int, depthQ uint16) bool {
	if g == nil {
		return true
	}
	return scene.BBoxFullyOccluded(x0, x1, y0, y1, g.viewW, g.viewH, func(x, y0, y1 int) bool {
		return g.spriteWallClipColumnOccludedBBox(x, y0, y1, depthQ)
	})
}

func (g *game) spriteOpaqueRectsFullyOccluded(rects []spriteOpaqueRect, dstX, dstY, scale float64, clipTop, clipBottom, viewW, viewH int, depthQ uint16) bool {
	if len(rects) == 0 {
		return false
	}
	anyVisibleRect := false
	for _, rect := range rects {
		x0, x1, y0, y1, ok := spriteRectScreenBounds(rect, dstX, dstY, scale, clipTop, clipBottom, viewW, viewH)
		if !ok {
			continue
		}
		anyVisibleRect = true
		if !g.spriteWallClipQuadFullyOccluded(x0, x1, y0, y1, depthQ) {
			return false
		}
	}
	return anyVisibleRect
}

func (g *game) projectedOpaqueRectsFullyOccluded(rects []projectedOpaqueRect, depthQ uint16) bool {
	if len(rects) == 0 {
		return false
	}
	for _, rect := range rects {
		if !g.spriteWallClipQuadFullyOccluded(rect.x0(), rect.x1(), rect.y0(), rect.y1(), depthQ) {
			return false
		}
	}
	return true
}

func (g *game) appendProjectedOpaqueRects(rects []spriteOpaqueRect, texW int, flip bool, dstX, dstY, scale float64, clipTop, clipBottom, viewW, viewH int) (int, int) {
	if g == nil || len(rects) == 0 || texW <= 0 || scale <= 0 {
		return 0, 0
	}
	start := len(g.projectedOpaqueRectScratch)
	for _, rect := range rects {
		if flip {
			rect = flipSpriteOpaqueRectX(rect, texW)
		}
		x0, x1, y0, y1, ok := spriteRectScreenBounds(rect, dstX, dstY, scale, clipTop, clipBottom, viewW, viewH)
		if !ok {
			continue
		}
		g.projectedOpaqueRectScratch = append(g.projectedOpaqueRectScratch, packProjectedOpaqueRect(x0, x1, y0, y1))
	}
	return start, len(g.projectedOpaqueRectScratch) - start
}

func (g *game) spriteWallClipPointOccluded(x, y int, depthQ uint16) bool {
	if g == nil || g.viewW <= 0 || g.viewH <= 0 {
		return true
	}
	if x < 0 || x >= g.viewW || y < 0 || y >= g.viewH {
		return true
	}
	return g.spriteWallClipColumnOccludedBBox(x, y, y, depthQ)
}

func (g *game) spriteWallClipQuadTriMaybeVisible(x0, x1, y0, y1 int, depthQ uint16) bool {
	return scene.QuadTriMaybeVisible(x0, x1, y0, y1, func(x, y int) bool {
		return g.spriteWallClipPointOccluded(x, y, depthQ)
	})
}

func (g *game) spriteWallClipTriangleFullyOccludedFast(ax, ay, bx, by, cx, cy int, depthQ uint16) bool {
	return g.spriteWallClipTriangleOcclusionState(ax, ay, bx, by, cx, cy, depthQ) == 2
}

// Returns:
// 0 = visible
// 1 = maybe occluded (fast tests say maybe, exact confirms not fully occluded)
// 2 = fully occluded
func (g *game) spriteWallClipTriangleOcclusionState(ax, ay, bx, by, cx, cy int, depthQ uint16) int {
	return scene.TriangleOcclusionState(ax, ay, bx, by, cx, cy, g.viewW, g.viewH, func(x, y int) bool {
		return g.spriteWallClipPointOccluded(x, y, depthQ)
	}, func(x0, x1, y0, y1 int) bool {
		return g.spriteWallClipBBoxFullyOccluded(x0, x1, y0, y1, depthQ)
	})
}

func (g *game) wallSliceYDepthAtX(pp wallSegPrepass, x int, z, focal float64) (float64, float64, bool) {
	return scene.ProjectedWallYDepthAtX(wallSegPrepassProjection(pp), x, g.viewH, z, focal)
}

func (g *game) wallSliceRangeTriFullyOccludedByWallsOnly(pp wallSegPrepass, l, r int, zTop, zBottom, focal float64) bool {
	if g == nil || l > r || g.viewW <= 0 || g.viewH <= 0 {
		return true
	}
	if l < 0 {
		l = 0
	}
	if r >= g.viewW {
		r = g.viewW - 1
	}
	if l > r {
		return true
	}
	ytL, fL, okL := g.wallSliceYDepthAtX(pp, l, zTop, focal)
	ytR, fR, okR := g.wallSliceYDepthAtX(pp, r, zTop, focal)
	ybL, _, okBL := g.wallSliceYDepthAtX(pp, l, zBottom, focal)
	ybR, _, okBR := g.wallSliceYDepthAtX(pp, r, zBottom, focal)
	if !(okL && okR && okBL && okBR) {
		return false
	}
	ax, ay := l, int(math.Floor(ytL))
	bx, by := r, int(math.Floor(ytR))
	cx, cy := r, int(math.Floor(ybR))
	dx, dy := l, int(math.Floor(ybL))
	proj := wallSegPrepassProjection(pp)
	depthQAtX := func(x int) uint16 {
		if proj.SX2 == proj.SX1 {
			return encodeDepthQ((fL + fR) * 0.5)
		}
		depth, ok := scene.ProjectedWallDepthAtX(proj, x)
		if !ok {
			return encodeDepthQ((fL + fR) * 0.5)
		}
		return encodeDepthQ(depth)
	}
	triOccState := func(ax, ay, bx, by, cx, cy int) int {
		return scene.TriangleOcclusionStateInView(ax, ay, bx, by, cx, cy, g.viewW, g.viewH, func(x, y int) bool {
			return g.wallClipPointOccludedByWallsOnly(x, y, depthQAtX(x))
		}, func(x0, x1, y0, y1 int) bool {
			for x := x0; x <= x1; x++ {
				if !g.wallClipColumnOccludedBBoxByWallsOnly(x, y0, y1, depthQAtX(x)) {
					return false
				}
			}
			return true
		})
	}
	triAOcc := triOccState(ax, ay, bx, by, cx, cy) == 2
	if !triAOcc {
		return false
	}
	triBOcc := triOccState(ax, ay, cx, cy, dx, dy) == 2
	return triBOcc
}

func (g *game) spriteWallClipQuadFullyOccluded(x0, x1, y0, y1 int, depthQ uint16) bool {
	if !g.billboardClippingEnabled() {
		return false
	}
	if g == nil || g.viewW <= 0 || x0 > x1 || y0 > y1 {
		return true
	}
	if x0 < 0 {
		x0 = 0
	}
	if x1 >= g.viewW {
		x1 = g.viewW - 1
	}
	if y0 < 0 {
		y0 = 0
	}
	if y1 >= g.viewH {
		y1 = g.viewH - 1
	}
	if x0 > x1 || y0 > y1 {
		return true
	}
	if g.wallClipBBoxFullyOccludedByWallsOnly(x0, x1, y0, y1, depthQ) {
		return true
	}
	if g.spriteWallClipQuadTriMaybeVisible(x0, x1, y0, y1, depthQ) {
		return false
	}
	return g.spriteWallClipBBoxFullyOccluded(x0, x1, y0, y1, depthQ)
}

func cacheFloorSpriteItemGeometry(sx, yb, h float64, tex *WallTexture, clipTop, clipBottom, viewW, viewH int) (scale, dstX, dstY float64, x0, x1, y0, y1 int, ok bool) {
	if tex == nil {
		return 0, 0, 0, 0, 0, 0, 0, false
	}
	th := tex.Height
	tw := tex.Width
	if th <= 0 || tw <= 0 {
		return 0, 0, 0, 0, 0, 0, 0, false
	}
	scale = h / float64(th)
	if scale <= 0 {
		return 0, 0, 0, 0, 0, 0, 0, false
	}
	dstW := float64(tw) * scale
	dstH := float64(th) * scale
	dstX = sx - float64(tex.OffsetX)*scale
	dstY = floorSpriteTop(dstH, yb)
	x0, x1, y0, y1, ok = scene.ClampedSpriteBounds(dstX, dstY, dstW, dstH, clipTop, clipBottom, viewW, viewH)
	return scale, dstX, dstY, x0, x1, y0, y1, ok
}

func cacheOriginSpriteItemGeometry(sx, sy, scale float64, tex *WallTexture, clipTop, clipBottom, viewW, viewH int) (dstW, dstH, dstX, dstY float64, x0, x1, y0, y1 int, ok bool) {
	if tex == nil {
		return 0, 0, 0, 0, 0, 0, 0, 0, false
	}
	th := tex.Height
	tw := tex.Width
	if th <= 0 || tw <= 0 || scale <= 0 {
		return 0, 0, 0, 0, 0, 0, 0, 0, false
	}
	dstW = float64(tw) * scale
	dstH = float64(th) * scale
	dstX = sx - float64(tex.OffsetX)*scale
	dstY = sy - float64(tex.OffsetY)*scale
	x0, x1, y0, y1, ok = scene.ClampedSpriteBounds(dstX, dstY, dstW, dstH, clipTop, clipBottom, viewW, viewH)
	return dstW, dstH, dstX, dstY, x0, x1, y0, y1, ok
}

func spriteClipBottomWithPatchOverhang(clipBottom int, tex *WallTexture, scale float64, viewH int) int {
	if tex == nil || scale <= 0 || viewH <= 0 {
		return clipBottom
	}
	overhang := tex.Height - tex.OffsetY
	if overhang <= 0 {
		return clipBottom
	}
	clipBottom += int(math.Ceil(float64(overhang) * scale))
	if clipBottom >= viewH {
		clipBottom = viewH - 1
	}
	return clipBottom
}

func floorSpriteTop(dstH, yb float64) float64 {
	return scene.FloorSpriteTop(dstH, yb)
}

func spriteRectScreenBounds(rect spriteOpaqueRect, dstX, dstY, scale float64, clipTop, clipBottom, viewW, viewH int) (int, int, int, int, bool) {
	return scene.OpaqueRectScreenBounds(rect.minX(), rect.minY(), rect.maxX(), rect.maxY(), dstX, dstY, scale, clipTop, clipBottom, viewW, viewH)
}

func puffItemScreenBounds(it projectedPuffItem, focal float64, viewW, viewH int) (int, int, int, int, bool) {
	if !it.hasSprite || it.spriteTex == nil || it.spriteTex.Width <= 0 || it.spriteTex.Height <= 0 {
		return 0, -1, 0, -1, false
	}
	scale := focal / it.dist
	if scale <= 0 {
		return 0, -1, 0, -1, false
	}
	return scene.SpritePatchBoundsFromScale(it.sx, it.sy, scale, it.spriteTex.Width, it.spriteTex.Height, it.spriteTex.OffsetX, it.spriteTex.OffsetY, it.clipTop, it.clipBottom, viewW, viewH)
}

func (g *game) drawBasicWallColumnTextured(x, y0, y1 int, depth, texU, texMid, focal float64, tex wallTextureBlendSample, shadeMul, doomRow int) {
	if tex.from == nil {
		return
	}
	base := tex.from
	rowStridePix := g.viewW
	pixI := y0*rowStridePix + x
	pix32 := g.wallPix32
	if !doomColormapEnabled && shadeMul <= 0 {
		for y := y0; y <= y1; y++ {
			pix32[pixI] = pixelOpaqueA
			pixI += rowStridePix
		}
		return
	}
	texIndexed := base.Indexed
	texIndexedCol := base.IndexedColMajor
	indexedReady := len(texIndexed) == base.Width*base.Height
	if !indexedReady {
		return
	}
	txi := int(floorFixed(texU) >> fracBits)
	tx := 0
	if base.Width > 0 && (base.Width&(base.Width-1)) == 0 {
		tx = txi & (base.Width - 1)
	} else {
		tx = wrapIndex(txi, base.Width)
	}
	rowScale := depth / focal
	cy := float64(g.viewH) * 0.5
	texV := texMid - ((cy - (float64(y0) + 0.5)) * rowScale)
	texVFixed := floorFixed(texV)
	texVStepFixed := floorFixed(rowScale)
	pow2H := base.Height > 0 && (base.Height&(base.Height-1)) == 0
	hmask := base.Height - 1
	colBase := tx * base.Height
	if tex.alpha != 0 && tex.to != nil && tex.to.Width == base.Width && tex.to.Height == base.Height && len(tex.to.Indexed) == len(texIndexed) {
		g.drawBasicWallColumnTexturedBlended(x, y0, y1, texU, texMid, depth, focal, base, tex.to, tex.alpha, shadeMul, doomRow)
		return
	}
	// Dominant hot path: little-endian packed output + pretransposed column-major texture + pow2 height.
	if pixelLittleEndian && pow2H && len(texIndexedCol) == base.Width*base.Height {
		col := texIndexedCol[colBase : colBase+base.Height]
		if doomRow >= doomNumColorMaps || doomColormapEnabled {
			rows := doomColormapRowCount()
			if rows <= 0 || len(doomColormapRGBA) < rows*256 {
				return
			}
			if doomRow < 0 {
				doomRow = 0
			} else if doomRow >= rows {
				doomRow = rows - 1
			}
			drawWallColumnTexturedIndexedLEColPow2Row(pix32, pixI, rowStridePix, col, texVFixed, texVStepFixed, hmask, y1-y0+1, doomColormapRGBA[doomRow*256:])
			return
		}
		if shadeMul < 0 {
			shadeMul = 0
		} else if shadeMul > 256 {
			shadeMul = 256
		}
		drawWallColumnTexturedIndexedLEColPow2Row(pix32, pixI, rowStridePix, col, texVFixed, texVStepFixed, hmask, y1-y0+1, wallShadePackedLUT[shadeMul][:])
		return
	}
	if pow2H {
		for y := y0; y <= y1; y++ {
			ty := int((texVFixed >> fracBits) & int64(hmask))
			ti := ty*base.Width + tx
			if doomRow >= doomNumColorMaps || doomColormapEnabled {
				pix32[pixI] = shadePaletteIndexDOOMRow(texIndexed[ti], doomRow)
			} else {
				pix32[pixI] = shadePaletteIndexPacked(texIndexed[ti], uint32(shadeMul))
			}
			pixI += rowStridePix
			texVFixed += texVStepFixed
		}
		return
	}
	for y := y0; y <= y1; y++ {
		ty := wrapIndex(int(texVFixed>>fracBits), base.Height)
		ti := ty*base.Width + tx
		if doomRow >= doomNumColorMaps || doomColormapEnabled {
			pix32[pixI] = shadePaletteIndexDOOMRow(texIndexed[ti], doomRow)
		} else {
			pix32[pixI] = shadePaletteIndexPacked(texIndexed[ti], uint32(shadeMul))
		}
		pixI += rowStridePix
		texVFixed += texVStepFixed
	}
}

func (g *game) drawBasicWallColumnTexturedBlended(x, y0, y1 int, texU, texMid, depth, focal float64, from, to *WallTexture, alpha uint8, shadeMul, doomRow int) {
	if from == nil || to == nil || from.Width <= 0 || from.Height <= 0 || to.Width != from.Width || to.Height != from.Height {
		return
	}
	rowStridePix := g.viewW
	pixI := y0*rowStridePix + x
	txi := int(floorFixed(texU) >> fracBits)
	tx := wrapIndex(txi, from.Width)
	rowScale := depth / focal
	cy := float64(g.viewH) * 0.5
	texVFixed := floorFixed(texMid - ((cy - (float64(y0) + 0.5)) * rowScale))
	texVStepFixed := floorFixed(rowScale)
	for y := y0; y <= y1; y++ {
		ty := wrapIndex(int(texVFixed>>fracBits), from.Height)
		ti := ty*from.Width + tx
		var p0, p1 uint32
		if doomRow >= doomNumColorMaps || doomColormapEnabled {
			p0 = shadePaletteIndexDOOMRow(from.Indexed[ti], doomRow)
			p1 = shadePaletteIndexDOOMRow(to.Indexed[ti], doomRow)
		} else {
			p0 = shadePaletteIndexPacked(from.Indexed[ti], uint32(shadeMul))
			p1 = shadePaletteIndexPacked(to.Indexed[ti], uint32(shadeMul))
		}
		g.wallPix32[pixI] = blendPackedRGBA(p0, p1, alpha)
		pixI += rowStridePix
		texVFixed += texVStepFixed
	}
}

func (g *game) drawBasicWallColumnTexturedMasked(x, y0, y1 int, depth, texU, texMid, focal float64, tex wallTextureBlendSample, shadeMul, doomRow int) {
	if tex.from == nil {
		return
	}
	base := tex.from
	if x < 0 || x >= g.viewW || y0 > y1 {
		return
	}
	if y0 < 0 {
		y0 = 0
	}
	if y1 >= g.viewH {
		y1 = g.viewH - 1
	}
	if y0 > y1 || base.Width <= 0 || base.Height <= 0 {
		return
	}
	rowStridePix := g.viewW
	pixI := y0*rowStridePix + x
	pix32 := g.wallPix32
	base.EnsureOpaqueMask()
	maskReady := len(base.OpaqueMask) == base.Width*base.Height
	if tex.to != nil {
		tex.to.EnsureOpaqueMask()
	}
	texIndexed := base.Indexed
	texIndexedCol := base.IndexedColMajor
	packedRow := maskedWallShadePackedRow(shadeMul, doomRow)
	indexedReady := len(packedRow) == 256 && len(texIndexed) == base.Width*base.Height
	if !indexedReady {
		return
	}
	txi := int(floorFixed(texU) >> fracBits)
	tx := 0
	if base.Width > 0 && (base.Width&(base.Width-1)) == 0 {
		tx = txi & (base.Width - 1)
	} else {
		tx = wrapIndex(txi, base.Width)
	}
	rowScale := depth / focal
	cy := float64(g.viewH) * 0.5
	texV := texMid - ((cy - (float64(y0) + 0.5)) * rowScale)
	texVFixed := floorFixed(texV)
	texVStepFixed := floorFixed(rowScale)
	depthQ := encodeDepthQ(depth)
	if len(base.OpaqueColumnTop) == base.Width && len(base.OpaqueColumnBot) == base.Width {
		var ok bool
		y0, y1, texVFixed, ok = trimMaskedColumnToOpaqueBounds(y0, y1, texVFixed, texVStepFixed, base.Height, int(base.OpaqueColumnTop[tx]), int(base.OpaqueColumnBot[tx]))
		if !ok {
			return
		}
	}
	visible := g.maskedColumnVisibleSpans(x, y0, y1, depthQ)
	if len(visible) == 0 {
		return
	}
	if g.cutoutColumnVisibleSpansFullyCovered(x, rowStridePix, visible) {
		return
	}
	if len(base.OpaqueRunOffs) == base.Width+1 && tx >= 0 && tx < base.Width && len(base.OpaqueRuns) >= int(base.OpaqueRunOffs[tx+1]) && tex.alpha == 0 {
		if drawMaskedColumnOpaqueRuns(g, x, y0, y1, texVFixed, texVStepFixed, base, tx, nil, depthQ, shadeMul, doomRow, visible) {
			return
		}
	}
	colBase := tx * base.Height
	useIndexedCol := len(texIndexedCol) == base.Width*base.Height
	repeatTexelRows := texVStepFixed*2 <= fracUnit
	for _, vis := range visible {
		if vis.L > vis.R {
			continue
		}
		pixI = vis.L*rowStridePix + x
		runTexVFixed := texVFixed + int64(vis.L-y0)*texVStepFixed
		for y := vis.L; y <= vis.R; y++ {
			ty := wrapIndex(int(runTexVFixed>>fracBits), base.Height)
			ti := ty*base.Width + tx
			opaque := maskReady && base.OpaqueMask[ti] != 0
			if tex.alpha != 0 && tex.to != nil && len(tex.to.OpaqueMask) == len(base.OpaqueMask) && tex.to.OpaqueMask[ti] != 0 {
				opaque = true
			}
			if !opaque {
				pixI += rowStridePix
				runTexVFixed += texVStepFixed
				continue
			}
			if g.cutoutCoveredAtIndex(pixI) {
				pixI += rowStridePix
				runTexVFixed += texVStepFixed
				continue
			}
			dst := uint32(0)
			if tex.alpha != 0 && tex.to != nil && tex.to.Width == base.Width && tex.to.Height == base.Height && len(tex.to.Indexed) == len(texIndexed) {
				p0 := packedRow[texIndexed[ti]]
				p1 := packedRow[tex.to.Indexed[ti]]
				dst = blendPackedRGBA(p0, p1, tex.alpha)
			} else if useIndexedCol {
				dst = packedRow[texIndexedCol[colBase+ty]]
			} else {
				dst = packedRow[texIndexed[ti]]
			}
			if repeatTexelRows {
				runLen := 1
				nextTexVFixed := runTexVFixed + texVStepFixed
				for y+runLen <= vis.R && wrapIndex(int(nextTexVFixed>>fracBits), base.Height) == ty {
					nextPixI := pixI + runLen*rowStridePix
					if g.cutoutCoveredAtIndex(nextPixI) {
						break
					}
					runLen++
					nextTexVFixed += texVStepFixed
				}
				if runLen > 1 {
					g.fillCutoutColumnSpan(pixI, rowStridePix, runLen, dst)
					pixI += runLen * rowStridePix
					runTexVFixed += int64(runLen) * texVStepFixed
					y += runLen - 1
					continue
				}
			}
			pix32[pixI] = dst
			g.markCutoutCoveredAtIndex(pixI)
			pixI += rowStridePix
			runTexVFixed += texVStepFixed
		}
	}
}

func drawMaskedColumnOpaqueRuns(g *game, x, y0, y1 int, texVFixed, texVStepFixed int64, tex *WallTexture, tx int, tex32 []uint32, depthQ uint16, shadeMul, doomRow int, visible []solidSpan) bool {
	if g == nil || tex == nil || texVStepFixed <= 0 || tex.Height <= 0 || tx < 0 || tx >= tex.Width {
		return false
	}
	startTy := int(texVFixed >> fracBits)
	endTy := int((texVFixed + int64(y1-y0)*texVStepFixed) >> fracBits)
	if floorDiv(startTy, tex.Height) != floorDiv(endTy, tex.Height) {
		return false
	}
	runStart := int(tex.OpaqueRunOffs[tx])
	runEnd := int(tex.OpaqueRunOffs[tx+1])
	if runStart >= runEnd {
		return true
	}
	if len(visible) == 0 {
		return true
	}
	cycleBase := floorDiv(startTy, tex.Height) * tex.Height
	rowStridePix := g.viewW
	packedRow := maskedWallShadePackedRow(shadeMul, doomRow)
	texIndexed := tex.Indexed
	texIndexedCol := tex.IndexedColMajor
	indexedReady := len(packedRow) == 256 && len(texIndexed) == tex.Width*tex.Height
	if !indexedReady {
		return false
	}
	colBase := tx * tex.Height
	useIndexedCol := len(texIndexedCol) == tex.Width*tex.Height
	repeatTexelRows := texVStepFixed*2 <= fracUnit
	for i := runStart; i < runEnd; i++ {
		runTop, runBot := media.UnpackOpaqueRun(tex.OpaqueRuns[i])
		srcTop := cycleBase + runTop
		srcBot := cycleBase + runBot
		first := ceilDiv(max64(0, int64(srcTop<<fracBits)-texVFixed), texVStepFixed)
		lastPlusOne := ceilDiv(max64(0, int64((srcBot+1)<<fracBits)-texVFixed), texVStepFixed)
		last := lastPlusOne - 1
		if first > int64(y1-y0) || last < first {
			continue
		}
		if last > int64(y1-y0) {
			last = int64(y1 - y0)
		}
		runY0 := y0 + int(first)
		runY1 := y0 + int(last)
		if runY0 > runY1 {
			continue
		}
		for _, vis := range visible {
			if vis.L > vis.R || vis.R < runY0 || vis.L > runY1 {
				continue
			}
			drawY0 := vis.L
			if drawY0 < runY0 {
				drawY0 = runY0
			}
			drawY1 := vis.R
			if drawY1 > runY1 {
				drawY1 = runY1
			}
			if repeatTexelRows {
				srcDrawTop := wrapIndex(int((texVFixed+int64(drawY0-y0)*texVStepFixed)>>fracBits), tex.Height)
				srcDrawBot := wrapIndex(int((texVFixed+int64(drawY1-y0)*texVStepFixed)>>fracBits), tex.Height)
				if srcDrawTop >= runTop && srcDrawBot <= runBot && drawMaskedColumnProjectedTexelSpans(g, x, y0, drawY0, drawY1, texVFixed, texVStepFixed, tex.Width, tex.Height, rowStridePix, packedRow, texIndexed, texIndexedCol, useIndexedCol, colBase, tx, depthQ) {
					continue
				}
			}
			if g.cutoutColumnSpanFullyCovered(drawY0*rowStridePix+x, rowStridePix, drawY1-drawY0+1) {
				continue
			}
			pixI := drawY0*rowStridePix + x
			runTexVFixed := texVFixed + int64(drawY0-y0)*texVStepFixed
			for y := drawY0; y <= drawY1; y++ {
				if g.cutoutCoveredAtIndex(pixI) {
					pixI += rowStridePix
					runTexVFixed += texVStepFixed
					continue
				}
				ty := wrapIndex(int(runTexVFixed>>fracBits), tex.Height)
				if useIndexedCol {
					dst := packedRow[texIndexedCol[colBase+ty]]
					if repeatTexelRows {
						runLen := 1
						nextTexVFixed := runTexVFixed + texVStepFixed
						for y+runLen <= drawY1 && wrapIndex(int(nextTexVFixed>>fracBits), tex.Height) == ty {
							nextPixI := pixI + runLen*rowStridePix
							if g.cutoutCoveredAtIndex(nextPixI) {
								break
							}
							runLen++
							nextTexVFixed += texVStepFixed
						}
						if runLen > 1 {
							g.fillCutoutColumnSpan(pixI, rowStridePix, runLen, dst)
							pixI += runLen * rowStridePix
							runTexVFixed += int64(runLen) * texVStepFixed
							y += runLen - 1
							continue
						}
					}
					g.wallPix32[pixI] = dst
				} else {
					dst := packedRow[texIndexed[ty*tex.Width+tx]]
					if repeatTexelRows {
						runLen := 1
						nextTexVFixed := runTexVFixed + texVStepFixed
						for y+runLen <= drawY1 && wrapIndex(int(nextTexVFixed>>fracBits), tex.Height) == ty {
							nextPixI := pixI + runLen*rowStridePix
							if g.cutoutCoveredAtIndex(nextPixI) {
								break
							}
							runLen++
							nextTexVFixed += texVStepFixed
						}
						if runLen > 1 {
							g.fillCutoutColumnSpan(pixI, rowStridePix, runLen, dst)
							pixI += runLen * rowStridePix
							runTexVFixed += int64(runLen) * texVStepFixed
							y += runLen - 1
							continue
						}
					}
					g.wallPix32[pixI] = dst
				}
				g.markCutoutCoveredAtIndex(pixI)
				pixI += rowStridePix
				runTexVFixed += texVStepFixed
			}
		}
	}
	return true
}

func drawMaskedColumnProjectedTexelSpans(g *game, x, baseY, drawY0, drawY1 int, texVFixed, texVStepFixed int64, texW, texH, rowStridePix int, packedRow []uint32, texIndexed, texIndexedCol []byte, useIndexedCol bool, colBase, tx int, depthQ uint16) bool {
	if g == nil || drawY0 > drawY1 || texVStepFixed <= 0 || texH <= 0 {
		return false
	}
	if g.cutoutColumnSpanFullyCovered(drawY0*rowStridePix+x, rowStridePix, drawY1-drawY0+1) {
		return true
	}
	srcTop := wrapIndex(int((texVFixed+int64(drawY0-baseY)*texVStepFixed)>>fracBits), texH)
	srcBot := wrapIndex(int((texVFixed+int64(drawY1-baseY)*texVStepFixed)>>fracBits), texH)
	if srcTop > srcBot {
		return false
	}
	for ty := srcTop; ty <= srcBot; ty++ {
		first := ceilDiv(max64(0, int64(ty<<fracBits)-texVFixed), texVStepFixed)
		lastPlusOne := ceilDiv(max64(0, int64((ty+1)<<fracBits)-texVFixed), texVStepFixed)
		last := lastPlusOne - 1
		spanY0 := baseY + int(first)
		spanY1 := baseY + int(last)
		if spanY0 < drawY0 {
			spanY0 = drawY0
		}
		if spanY1 > drawY1 {
			spanY1 = drawY1
		}
		if spanY0 > spanY1 {
			continue
		}
		dst := uint32(0)
		if useIndexedCol {
			dst = packedRow[texIndexedCol[colBase+ty]]
		} else {
			dst = packedRow[texIndexed[ty*texW+tx]]
		}
		pixI := spanY0*rowStridePix + x
		endPixI := spanY1*rowStridePix + x
		for pixI <= endPixI {
			if g.cutoutCoveredAtIndex(pixI) || g.spriteWallClipOccludedAtIndexDepth(pixI, depthQ) {
				pixI += rowStridePix
				continue
			}
			runLen := 1
			nextPixI := pixI + rowStridePix
			for nextPixI <= endPixI && !g.cutoutCoveredAtIndex(nextPixI) && !g.spriteWallClipOccludedAtIndexDepth(nextPixI, depthQ) {
				runLen++
				nextPixI += rowStridePix
			}
			if runLen > 1 {
				g.fillCutoutColumnSpan(pixI, rowStridePix, runLen, dst)
			} else {
				g.wallPix32[pixI] = dst
				g.markCutoutCoveredAtIndex(pixI)
			}
			pixI = nextPixI
		}
	}
	return true
}

func (g *game) maskedColumnVisibleSpans(x, y0, y1 int, depthQ uint16) []solidSpan {
	if g == nil || x < 0 || x >= g.viewW || y0 > y1 {
		return nil
	}
	if !g.billboardClippingEnabled() {
		out := g.maskedSpanScratchA[:0]
		return append(out, solidSpan{L: y0, R: y1})
	}
	if g.wallClipColumnOccludedBBoxByWallsOnly(x, y0, y1, depthQ) && (x >= len(g.maskedClipCols) || !scene.MaskedClipColumnHasAnyOccluder(g.maskedClipCols[x], y0, y1, depthQ)) {
		return nil
	}
	cur := append(g.maskedSpanScratchA[:0], solidSpan{L: y0, R: y1})
	next := g.maskedSpanScratchB[:0]
	if x < len(g.wallDepthQCol) {
		col := g.wallDepthColumnAt(x)
		next = clipVerticalSpansByWallDepth(cur, col, depthQ, next[:0])
		cur, next = next, cur[:0]
		if len(cur) == 0 {
			return nil
		}
	}
	if x < len(g.maskedClipCols) {
		tmp := g.maskedSpanScratchC[:0]
		if cap(tmp) < len(cur)+2 {
			g.maskedSpanScratchC = make([]solidSpan, 0, len(cur)+2)
			tmp = g.maskedSpanScratchC[:0]
		}
		next = clipVerticalSpansByMasked(cur, g.maskedClipCols[x], depthQ, next[:0], tmp)
		cur, next = next, cur[:0]
		if len(cur) == 0 {
			return nil
		}
	}
	g.maskedSpanScratchA = next[:0]
	g.maskedSpanScratchB = cur
	g.maskedSpanScratchC = g.maskedSpanScratchC[:0]
	return cur
}

func clipVerticalSpansByWallDepth(in []solidSpan, col scene.WallDepthColumn, depthQ uint16, out []solidSpan) []solidSpan {
	out = out[:0]
	if col.DepthQ == 0xFFFF || depthQ <= col.DepthQ {
		return append(out, in...)
	}
	if col.Closed {
		return out
	}
	if col.Top > col.Bottom {
		return append(out, in...)
	}
	for _, sp := range in {
		if sp.R < col.Top || sp.L > col.Bottom {
			out = append(out, sp)
			continue
		}
		if sp.L < col.Top {
			out = append(out, solidSpan{L: sp.L, R: col.Top - 1})
		}
		if sp.R > col.Bottom {
			out = append(out, solidSpan{L: col.Bottom + 1, R: sp.R})
		}
	}
	return out
}

func clipVerticalSpansByMasked(in []solidSpan, masked []scene.MaskedClipSpan, depthQ uint16, out, tmp []solidSpan) []solidSpan {
	out = append(out[:0], in...)
	if len(out) == 0 || len(masked) == 0 {
		return out
	}
	if cap(tmp) < len(out)+2 {
		tmp = make([]solidSpan, 0, len(out)+2)
	} else {
		tmp = tmp[:0]
	}
	for _, sp := range masked {
		if depthQ <= sp.DepthQ {
			break
		}
		tmp = tmp[:0]
		if sp.Closed {
			return tmp
		}
		if sp.HasOpen {
			openL := int(sp.OpenY0)
			openR := int(sp.OpenY1)
			for _, cur := range out {
				l := max(cur.L, openL)
				r := min(cur.R, openR)
				if l <= r {
					tmp = append(tmp, solidSpan{L: l, R: r})
				}
			}
			out = append(out[:0], tmp...)
			if len(out) == 0 {
				return out
			}
			continue
		}
		blockL := int(sp.Y0)
		blockR := int(sp.Y1)
		for _, cur := range out {
			if cur.R < blockL || cur.L > blockR {
				tmp = append(tmp, cur)
				continue
			}
			if cur.L < blockL {
				tmp = append(tmp, solidSpan{L: cur.L, R: blockL - 1})
			}
			if cur.R > blockR {
				tmp = append(tmp, solidSpan{L: blockR + 1, R: cur.R})
			}
		}
		out = append(out[:0], tmp...)
		if len(out) == 0 {
			return out
		}
	}
	return out
}

func trimMaskedColumnToOpaqueBounds(y0, y1 int, texVFixed, texVStepFixed int64, texHeight, top, bottom int) (int, int, int64, bool) {
	if y0 > y1 || texHeight <= 0 || top < 0 || bottom < top {
		return 0, -1, texVFixed, false
	}
	if texVStepFixed < 0 {
		return y0, y1, texVFixed, true
	}
	startTy := int(texVFixed >> fracBits)
	if texVStepFixed == 0 {
		ty := wrapIndex(startTy, texHeight)
		if ty < top || ty > bottom {
			return 0, -1, texVFixed, false
		}
		return y0, y1, texVFixed, true
	}
	endTy := int((texVFixed + int64(y1-y0)*texVStepFixed) >> fracBits)
	if floorDiv(startTy, texHeight) != floorDiv(endTy, texHeight) {
		return y0, y1, texVFixed, true
	}
	cycleBase := floorDiv(startTy, texHeight) * texHeight
	targetTop := cycleBase + top
	targetBottom := cycleBase + bottom
	first := ceilDiv(max64(0, int64(targetTop<<fracBits)-texVFixed), texVStepFixed)
	lastPlusOne := ceilDiv(max64(0, int64((targetBottom+1)<<fracBits)-texVFixed), texVStepFixed)
	last := lastPlusOne - 1
	if first > int64(y1-y0) || last < first {
		return 0, -1, texVFixed, false
	}
	if last > int64(y1-y0) {
		last = int64(y1 - y0)
	}
	y0 += int(first)
	y1 = y0 + int(last-first)
	texVFixed += first * texVStepFixed
	return y0, y1, texVFixed, true
}

func packedPixelShifts() (r, g, b, a uint) {
	var probe uint16 = 1
	if *(*byte)(unsafe.Pointer(&probe)) == 1 {
		// Little-endian: bytes in memory are [R G B A] for value 0xAABBGGRR.
		return 0, 8, 16, 24
	}
	// Big-endian fallback: bytes in memory are [R G B A] for value 0xRRGGBBAA.
	return 24, 16, 8, 0
}

func packRGBA(r, g, b uint8) uint32 {
	return pixelOpaqueA |
		(uint32(r) << pixelRShift) |
		(uint32(g) << pixelGShift) |
		(uint32(b) << pixelBShift)
}

func blendPackedRGBA(a, b uint32, alpha uint8) uint32 {
	if alpha == 0 {
		return a | pixelOpaqueA
	}
	if alpha == 255 {
		return b | pixelOpaqueA
	}
	inv := uint32(255 - alpha)
	r0 := (a >> pixelRShift) & 0xFF
	g0 := (a >> pixelGShift) & 0xFF
	b0 := (a >> pixelBShift) & 0xFF
	r1 := (b >> pixelRShift) & 0xFF
	g1 := (b >> pixelGShift) & 0xFF
	b1 := (b >> pixelBShift) & 0xFF
	r := (r0*inv + r1*uint32(alpha) + 127) / 255
	g := (g0*inv + g1*uint32(alpha) + 127) / 255
	bl := (b0*inv + b1*uint32(alpha) + 127) / 255
	return pixelOpaqueA | (r << pixelRShift) | (g << pixelGShift) | (bl << pixelBShift)
}

func encodeDepthQ(depth float64) uint16 {
	if depth <= 0 {
		return 0
	}
	q := int(depth*depthQuantScale + 0.5)
	if q <= 0 {
		return 0
	}
	if q >= 0xFFFF {
		return 0xFFFF
	}
	return uint16(q)
}

func addDepthQ(a, b uint16) uint16 {
	return scene.AddDepthQ(a, b)
}

func shadePackedRGBABig(src, mul uint32) uint32 {
	if mul >= 256 {
		return src | pixelOpaqueA
	}
	if doomColormapEnabled {
		return shadePackedDOOMColormap(src, mul)
	}
	if idx, ok := packedColorPaletteIndex(src); ok {
		return shadePaletteIndexPacked(idx, mul)
	}
	wallShadeLUTOnce.Do(initWallShadeLUT)
	shade := &wallShadeLUT[mul]
	r := uint32(shade[(src>>pixelRShift)&0xFF])
	g := uint32(shade[(src>>pixelGShift)&0xFF])
	b := uint32(shade[(src>>pixelBShift)&0xFF])
	return pixelOpaqueA | (r << pixelRShift) | (g << pixelGShift) | (b << pixelBShift)
}

func shadePackedRGBA(src, mul uint32) uint32 {
	if mul >= 256 {
		return src | pixelOpaqueA
	}
	if doomColormapEnabled {
		return shadePackedDOOMColormap(src, mul)
	}
	if idx, ok := packedColorPaletteIndex(src); ok {
		return shadePaletteIndexPacked(idx, mul)
	}
	wallShadeLUTOnce.Do(initWallShadeLUT)
	shade := &wallShadeLUT[mul]
	r := uint32(shade[(src>>pixelRShift)&0xFF])
	g := uint32(shade[(src>>pixelGShift)&0xFF])
	b := uint32(shade[(src>>pixelBShift)&0xFF])
	return pixelOpaqueA | (r << pixelRShift) | (g << pixelGShift) | (b << pixelBShift)
}

func shadePaletteIndexPacked(idx byte, mul uint32) uint32 {
	if mul > 256 {
		mul = 256
	}
	if !wallShadePackedOK {
		return 0
	}
	return wallShadePackedLUT[mul][idx]
}

func shadePaletteIndexDOOMRow(idx byte, row int) uint32 {
	rows := doomColormapRowCount()
	if rows <= 0 || len(doomColormapRGBA) < rows*256 {
		return 0
	}
	if row < 0 {
		row = 0
	}
	if row >= rows {
		row = rows - 1
	}
	return doomColormapRGBA[row*256+int(idx)]
}

func doomColormapRowCount() int {
	rows := doomColormapRows
	if rows <= 0 {
		return 0
	}
	maxRows := len(doomColormapRGBA) / 256
	if maxRows <= 0 {
		return 0
	}
	if rows > maxRows {
		rows = maxRows
	}
	return rows
}

func packedColorPaletteIndex(src uint32) (byte, bool) {
	src = (src &^ pixelOpaqueA) | pixelOpaqueA
	if idx, ok := paletteIndexByPacked[src]; ok {
		return idx, true
	}
	if len(doomPalIndexLUT32) != 32*32*32 {
		return 0, false
	}
	r := uint8((src >> pixelRShift) & 0xFF)
	g := uint8((src >> pixelGShift) & 0xFF)
	b := uint8((src >> pixelBShift) & 0xFF)
	qi := (int(r>>3) << 10) | (int(g>>3) << 5) | int(b>>3)
	return doomPalIndexLUT32[qi], true
}

func doomShadeRows() int {
	if doomColormapRows <= 0 {
		return 0
	}
	rows := doomColormapRows
	if rows > doomNumColorMaps {
		rows = doomNumColorMaps
	}
	return rows
}

func shadePackedDOOMColormapRow(src uint32, row int) uint32 {
	rows := doomColormapRowCount()
	if rows <= 0 || len(doomColormapRGBA) < rows*256 || len(doomPalIndexLUT32) != 32*32*32 {
		return src | pixelOpaqueA
	}
	if row < 0 {
		row = 0
	}
	if row >= rows {
		row = rows - 1
	}
	palIdx, ok := packedColorPaletteIndex(src)
	if !ok {
		return src | pixelOpaqueA
	}
	return doomColormapRGBA[row*256+int(palIdx)]
}

func shadePackedDOOMRowOrLUT(src uint32, row int) uint32 {
	if doomColormapEnabled {
		return shadePackedDOOMColormapRow(src, row)
	}
	if !doomLightingEnabled {
		return src | pixelOpaqueA
	}
	return shadePackedRGBA(src, uint32(doomShadeMulFromRow(row)))
}

func (g *game) shadePackedSpectreFuzz(src uint32) uint32 {
	if row, ok := g.playerFixedColormapRow(); ok {
		return shadePackedDOOMColormapRow(src, row)
	}
	if g == nil || !g.opts.SourcePortMode {
		return shadePackedDOOMRowOrLUT(src, 6)
	}
	if doomColormapEnabled {
		return shadePackedDOOMColormapRow(src, 6)
	}
	if !doomLightingEnabled {
		return src | pixelOpaqueA
	}
	mul := doomShadeMulFromRow(6)
	if mul > 256 {
		mul = 256
	}
	return shadePackedRGBA(src, uint32(mul))
}

func (g *game) writeFuzzPixel(x, y, i int) {
	if g == nil || i < 0 || i >= len(g.wallPix32) {
		return
	}
	if !g.opts.SourcePortMode {
		if y <= 0 || y >= g.viewH-1 {
			return
		}
		delta := doomFuzzOffsets[g.spectreFuzzPos%len(doomFuzzOffsets)]
		g.spectreFuzzPos++
		srcY := y + delta
		if srcY < 1 {
			srcY = 1
		}
		if srcY >= g.viewH-1 {
			srcY = g.viewH - 2
		}
		srcI := srcY*g.viewW + x
		if srcI < 0 || srcI >= len(g.wallPix32) {
			srcI = i
		}
		src := g.wallPix32[srcI]
		if src == 0 {
			src = packRGBA(0, 0, 0)
		}
		g.writeWallPixel(i, g.shadePackedSpectreFuzz(src))
		return
	}
	coarseW := max(1, doomLogicalW)
	coarseH := max(1, doomLogicalH)
	cx := x * coarseW / max(1, g.viewW)
	cy := y * coarseH / max(1, g.viewH)
	if !g.spectreFuzzCoarseSet || g.spectreFuzzCoarseX != cx || g.spectreFuzzCoarseY != cy {
		g.spectreFuzzCoarseSet = true
		g.spectreFuzzCoarseX = cx
		g.spectreFuzzCoarseY = cy
	}
	delta := g.nextSourcePortFuzzOffset()
	srcCY := cy + delta
	if srcCY < 1 {
		srcCY = 1
	}
	if srcCY >= coarseH-1 {
		srcCY = coarseH - 2
	}
	srcX := (cx*g.viewW + g.viewW/2) / coarseW
	srcY := (srcCY*g.viewH + g.viewH/2) / coarseH
	if srcX < 0 {
		srcX = 0
	}
	if srcX >= g.viewW {
		srcX = g.viewW - 1
	}
	if srcY < 0 {
		srcY = 0
	}
	if srcY >= g.viewH {
		srcY = g.viewH - 1
	}
	srcI := srcY*g.viewW + srcX
	if srcI < 0 || srcI >= len(g.wallPix32) {
		srcI = i
	}
	srcPix := g.updateSpectreFuzzSource()
	src := g.wallPix32[srcI]
	if srcPix != nil && srcI >= 0 && srcI < len(srcPix) {
		src = srcPix[srcI]
	}
	if src == 0 {
		src = packRGBA(0, 0, 0)
	}
	g.writeWallPixel(i, g.shadePackedSpectreFuzz(src))
}

func (g *game) updateSpectreFuzzSource() []uint32 {
	if g == nil || len(g.wallPix32) == 0 {
		return nil
	}
	if !g.opts.SourcePortMode {
		return g.wallPix32
	}
	if g.spectreFuzzSampleInit && g.spectreFuzzSampleTic == g.worldTic && len(g.spectreFuzzSamplePix) == len(g.wallPix32) {
		return g.spectreFuzzSamplePix
	}
	if len(g.spectreFuzzSamplePix) != len(g.wallPix32) {
		g.spectreFuzzSamplePix = make([]uint32, len(g.wallPix32))
	}
	copy(g.spectreFuzzSamplePix, g.wallPix32)
	g.spectreFuzzSampleTic = g.worldTic
	g.spectreFuzzSampleInit = true
	return g.spectreFuzzSamplePix
}

func (g *game) beginSourcePortSpectreFuzzFrame(alpha float64) {
	if g == nil || !g.opts.SourcePortMode || len(doomFuzzOffsets) == 0 {
		return
	}
	if alpha < 0 {
		alpha = 0
	}
	if alpha > 1 {
		alpha = 1
	}
	phase := g.worldTic
	if alpha >= 1 {
		phase++
	}
	g.spectreFuzzPos = phase % len(doomFuzzOffsets)
	g.spectreFuzzCoarseSet = false
}

func (g *game) nextSourcePortFuzzOffset() int {
	if len(doomFuzzOffsets) == 0 {
		return 0
	}
	delta := doomFuzzOffsets[g.spectreFuzzPos%len(doomFuzzOffsets)]
	g.spectreFuzzPos++
	if g.spectreFuzzPos >= len(doomFuzzOffsets) {
		g.spectreFuzzPos = 0
	}
	return delta
}

func shadePackedDOOMColormap(src, mul uint32) uint32 {
	rows := doomShadeRows()
	if rows <= 0 || len(doomColormapRGBA) < rows*256 || len(doomPalIndexLUT32) != 32*32*32 {
		return src | pixelOpaqueA
	}
	m := int(mul)
	if m < 0 {
		m = 0
	}
	if m > 256 {
		m = 256
	}
	row := ((256 - m) * (rows - 1)) / 256
	return shadePackedDOOMColormapRow(src, row)
}

func doomShadeMulFromRow(row int) int {
	rows := doomShadeRows()
	if rows <= 1 {
		return 256
	}
	if row < 0 {
		row = 0
	}
	if row >= rows {
		row = rows - 1
	}
	if len(doomRowShadeMulLUT) == rows {
		return int(doomRowShadeMulLUT[row])
	}
	m := 256 - ((row * 256) / (rows - 1))
	if m < 0 {
		return 0
	}
	if m > 256 {
		return 256
	}
	return m
}

func doomShadeMulFromRowF(row float64) int {
	rows := doomShadeRows()
	if rows <= 1 {
		return 256
	}
	if row <= 0 {
		return doomShadeMulFromRow(0)
	}
	maxRow := float64(rows - 1)
	if row >= maxRow {
		return doomShadeMulFromRow(rows - 1)
	}
	r0 := int(row)
	r1 := r0 + 1
	f := row - float64(r0)
	m0 := doomShadeMulFromRow(r0)
	m1 := doomShadeMulFromRow(r1)
	m := int(float64(m0)*(1.0-f) + float64(m1)*f + 0.5)
	if m < 0 {
		return 0
	}
	if m > 256 {
		return 256
	}
	return m
}

func spritePixels32(tex *WallTexture) ([]uint32, bool) {
	if tex == nil {
		return nil, false
	}
	if tex.Width <= 0 || tex.Height <= 0 {
		return nil, false
	}
	n := tex.Width * tex.Height
	if len(tex.RGBA32) == n {
		return tex.RGBA32, true
	}
	if len(tex.RGBA) != n*4 || len(tex.RGBA) < 4 {
		return nil, false
	}
	return unsafe.Slice((*uint32)(unsafe.Pointer(unsafe.SliceData(tex.RGBA))), len(tex.RGBA)/4), true
}

func spriteIndexedPixels(tex *WallTexture) ([]byte, []byte, bool) {
	if tex == nil {
		return nil, nil, false
	}
	if tex.Width <= 0 || tex.Height <= 0 {
		return nil, nil, false
	}
	n := tex.Width * tex.Height
	if len(tex.Indexed) != n || len(tex.OpaqueMask) != n {
		return nil, nil, false
	}
	return tex.Indexed, tex.OpaqueMask, true
}

func synthesizeIndexedSpriteTexture(tex WallTexture) (WallTexture, bool) {
	if tex.Width <= 0 || tex.Height <= 0 {
		return tex, false
	}
	n := tex.Width * tex.Height
	tex.EnsureOpaqueMask()
	if len(tex.Indexed) == n && len(tex.OpaqueMask) == n {
		return tex, true
	}
	src32, ok := spritePixels32(&tex)
	if !ok || len(src32) != n {
		return tex, false
	}
	indexed := make([]byte, n)
	mask := tex.OpaqueMask
	if len(mask) != n {
		return tex, false
	}
	for i, p := range src32 {
		if mask[i] == 0 {
			continue
		}
		idx, ok := packedColorPaletteIndex(p)
		if !ok {
			idx = 0
		}
		indexed[i] = idx
	}
	tex.Indexed = indexed
	tex.OpaqueMask = mask
	return tex, true
}

func trimScreenRangeToOpaqueLUT(x0, x1 int, lut []int, minTex, maxTex int) (int, int, []int) {
	for x0 <= x1 && len(lut) > 0 {
		tx := lut[0]
		if tx >= minTex && tx <= maxTex {
			break
		}
		x0++
		lut = lut[1:]
	}
	for x0 <= x1 && len(lut) > 0 {
		tx := lut[len(lut)-1]
		if tx >= minTex && tx <= maxTex {
			break
		}
		x1--
		lut = lut[:len(lut)-1]
	}
	return x0, x1, lut
}

func trimSpanToOpaqueLUTRange(l, r, baseX int, lut []int, minTex, maxTex int) (int, int, bool) {
	if l > r || len(lut) == 0 {
		return 0, -1, false
	}
	start := l - baseX
	end := r - baseX
	if start < 0 {
		start = 0
	}
	if end >= len(lut) {
		end = len(lut) - 1
	}
	for start <= end {
		tx := lut[start]
		if tx >= minTex && tx <= maxTex {
			break
		}
		start++
	}
	for start <= end {
		tx := lut[end]
		if tx >= minTex && tx <= maxTex {
			break
		}
		end--
	}
	if start > end {
		return 0, -1, false
	}
	return baseX + start, baseX + end, true
}

const (
	spriteOpaqueRectMaxCount       = 12
	spriteOpaqueRectMinExtraPixels = 16
	spriteOpaqueRectExtraDivisor   = 48
	spriteOpaqueRectExtraCoverage  = 5
	spriteOpaqueRectMinWidth       = 4
	spriteOpaqueRectMinHeight      = 4
	spriteOpaqueRectExpectedScale  = 6
	spriteOpaqueRectMinScreenGain  = 384
)

func spriteOpaqueRectArea(rect spriteOpaqueRect) int {
	return (rect.maxX() - rect.minX() + 1) * (rect.maxY() - rect.minY() + 1)
}

func spriteOpaqueRectWidth(rect spriteOpaqueRect) int {
	return rect.maxX() - rect.minX() + 1
}

func spriteOpaqueRectHeight(rect spriteOpaqueRect) int {
	return rect.maxY() - rect.minY() + 1
}

func keepSpriteOpaqueRectShape(rect spriteOpaqueRect, opaquePixels, coveredPixels, rectIndex int) bool {
	area := spriteOpaqueRectArea(rect)
	if area <= 0 || opaquePixels <= 0 {
		return false
	}
	if spriteOpaqueRectWidth(rect) < spriteOpaqueRectMinWidth || spriteOpaqueRectHeight(rect) < spriteOpaqueRectMinHeight {
		return false
	}
	if area*spriteOpaqueRectExpectedScale*spriteOpaqueRectExpectedScale < spriteOpaqueRectMinScreenGain {
		return false
	}
	if rectIndex == 0 {
		return true
	}
	minArea := max(spriteOpaqueRectMinExtraPixels, opaquePixels/spriteOpaqueRectExtraDivisor)
	if area < minArea {
		return false
	}
	if area*spriteOpaqueRectExtraCoverage < opaquePixels {
		return false
	}
	return true
}

func largestOpaqueRect(mask []bool, w, h int) (spriteOpaqueRect, int, bool) {
	return largestOpaqueRectWithScratch(mask, w, h, nil, nil)
}

func largestOpaqueRectWithScratch(mask []bool, w, h int, heights, stack []int) (spriteOpaqueRect, int, bool) {
	if w <= 0 || h <= 0 || len(mask) != w*h {
		return 0, 0, false
	}
	heights = ensureIntScratch(heights, w)
	stack = ensureIntScratch(stack, w+1)
	var best spriteOpaqueRect
	bestArea := 0
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if mask[y*w+x] {
				heights[x]++
			} else {
				heights[x] = 0
			}
		}
		stack = stack[:0]
		for i := 0; i <= w; i++ {
			curH := 0
			if i < w {
				curH = heights[i]
			}
			for len(stack) > 0 && curH < heights[stack[len(stack)-1]] {
				top := stack[len(stack)-1]
				stack = stack[:len(stack)-1]
				height := heights[top]
				if height <= 0 {
					continue
				}
				left := 0
				if len(stack) > 0 {
					left = stack[len(stack)-1] + 1
				}
				right := i - 1
				area := height * (right - left + 1)
				if area > bestArea {
					bestArea = area
					best = packSpriteOpaqueRect(left, right, y-height+1, y)
				}
			}
			stack = append(stack, i)
		}
	}
	if bestArea <= 0 {
		return 0, 0, false
	}
	return best, bestArea, true
}

func buildSpriteOpaqueRects(mask8 []byte, w, h int) []spriteOpaqueRect {
	if w <= 0 || h <= 0 || len(mask8) != w*h {
		return nil
	}
	mask := make([]bool, w*h)
	heights := make([]int, w)
	stack := make([]int, 0, w+1)
	opaquePixels := 0
	for i, opaque := range mask8 {
		if opaque == 0 {
			continue
		}
		mask[i] = true
		opaquePixels++
	}
	if opaquePixels == 0 {
		return nil
	}
	rects := make([]spriteOpaqueRect, 0, spriteOpaqueRectMaxCount)
	coveredPixels := 0
	for len(rects) < spriteOpaqueRectMaxCount {
		rect, area, ok := largestOpaqueRectWithScratch(mask, w, h, heights, stack)
		if !ok {
			break
		}
		if !keepSpriteOpaqueRectShape(rect, opaquePixels, coveredPixels, len(rects)) {
			break
		}
		rects = append(rects, rect)
		coveredPixels += area
		for y := rect.minY(); y <= rect.maxY(); y++ {
			row := y * w
			for x := rect.minX(); x <= rect.maxX(); x++ {
				mask[row+x] = false
			}
		}
	}
	return rects
}

func ensureIntScratch(buf []int, n int) []int {
	if cap(buf) < n {
		return make([]int, n)
	}
	buf = buf[:n]
	for i := range buf {
		buf[i] = 0
	}
	return buf
}

func (g *game) spriteOpaqueShapeForKey(key string, tex *WallTexture) (spriteOpaqueShape, bool) {
	if key == "" {
		return spriteOpaqueShape{}, false
	}
	if tex == nil {
		return spriteOpaqueShape{}, false
	}
	if g.spriteOpaqueShapeCache == nil {
		g.spriteOpaqueShapeCache = make(map[string]spriteOpaqueShape, 128)
	}
	if shape, ok := g.spriteOpaqueShapeCache[key]; ok {
		return shape, true
	}
	tex.EnsureOpaqueMask()
	tex.EnsureOpaqueColumnBounds()
	if len(tex.OpaqueMask) != tex.Width*tex.Height {
		return spriteOpaqueShape{}, false
	}
	shape := spriteOpaqueShape{
		bounds: spriteOpaqueBounds{minX: tex.Width, minY: tex.Height, maxX: -1, maxY: -1},
		rowMin: make([]int16, tex.Height),
		rowMax: make([]int16, tex.Height),
		rects:  buildSpriteOpaqueRects(tex.OpaqueMask, tex.Width, tex.Height),
	}
	for y := 0; y < tex.Height; y++ {
		shape.rowMin[y] = int16(tex.Width)
		shape.rowMax[y] = -1
	}
	if len(tex.OpaqueRowOffs) == tex.Height+1 {
		for y := 0; y < tex.Height; y++ {
			runStart := int(tex.OpaqueRowOffs[y])
			runEnd := int(tex.OpaqueRowOffs[y+1])
			if runStart >= runEnd {
				continue
			}
			minX, _ := media.UnpackOpaqueRun(tex.OpaqueRowRuns[runStart])
			_, maxX := media.UnpackOpaqueRun(tex.OpaqueRowRuns[runEnd-1])
			if minX < shape.bounds.minX {
				shape.bounds.minX = minX
			}
			if maxX > shape.bounds.maxX {
				shape.bounds.maxX = maxX
			}
			if y < shape.bounds.minY {
				shape.bounds.minY = y
			}
			if y > shape.bounds.maxY {
				shape.bounds.maxY = y
			}
			shape.rowMin[y] = int16(minX)
			shape.rowMax[y] = int16(maxX)
		}
	} else {
		for y := 0; y < tex.Height; y++ {
			row := y * tex.Width
			for x := 0; x < tex.Width; x++ {
				if tex.OpaqueMask[row+x] == 0 {
					continue
				}
				if x < shape.bounds.minX {
					shape.bounds.minX = x
				}
				if x > shape.bounds.maxX {
					shape.bounds.maxX = x
				}
				if y < shape.bounds.minY {
					shape.bounds.minY = y
				}
				if y > shape.bounds.maxY {
					shape.bounds.maxY = y
				}
				if x < int(shape.rowMin[y]) {
					shape.rowMin[y] = int16(x)
				}
				if x > int(shape.rowMax[y]) {
					shape.rowMax[y] = int16(x)
				}
			}
		}
	}
	if shape.bounds.maxX < shape.bounds.minX || shape.bounds.maxY < shape.bounds.minY {
		return spriteOpaqueShape{}, false
	}
	g.spriteOpaqueShapeCache[key] = shape
	return shape, true
}

func (g *game) ensureSpriteTXScratch(n int) []int {
	if n <= 0 {
		return nil
	}
	if cap(g.spriteTXScratch) < n {
		g.spriteTXScratch = make([]int, n)
	} else {
		g.spriteTXScratch = g.spriteTXScratch[:n]
	}
	return g.spriteTXScratch
}

func (g *game) ensureSpriteTXRunEndScratch(n int) []int {
	if n <= 0 {
		return nil
	}
	if cap(g.spriteTXRunEndScratch) < n {
		g.spriteTXRunEndScratch = make([]int, n)
	} else {
		g.spriteTXRunEndScratch = g.spriteTXRunEndScratch[:n]
	}
	return g.spriteTXRunEndScratch
}

func (g *game) buildSpriteTXRunEnds(txLUT []int) []int {
	if len(txLUT) == 0 {
		return nil
	}
	runEnds := g.ensureSpriteTXRunEndScratch(len(txLUT))
	last := len(txLUT) - 1
	runEnd := last
	runEnds[last] = last
	for i := last - 1; i >= 0; i-- {
		if txLUT[i] == txLUT[i+1] {
			runEnds[i] = runEnd
			continue
		}
		runEnd = i
		runEnds[i] = i
	}
	return runEnds
}

func (g *game) ensureSpriteTYScratch(n int) []int {
	if n <= 0 {
		return nil
	}
	if cap(g.spriteTYScratch) < n {
		g.spriteTYScratch = make([]int, n)
	} else {
		g.spriteTYScratch = g.spriteTYScratch[:n]
	}
	return g.spriteTYScratch
}

func (g *game) ensurePlaneRenderScratch(n int) ([]uint32, [][]uint32, [][]byte, []bool) {
	if n <= 0 {
		return nil, nil, nil, nil
	}
	if cap(g.planeFBPackedScratch) < n {
		g.planeFBPackedScratch = make([]uint32, n)
	} else {
		g.planeFBPackedScratch = g.planeFBPackedScratch[:n]
	}
	if cap(g.planeFlatTex32Scratch) < n {
		g.planeFlatTex32Scratch = make([][]uint32, n)
	} else {
		g.planeFlatTex32Scratch = g.planeFlatTex32Scratch[:n]
	}
	if cap(g.planeFlatTexIndexedScratch) < n {
		g.planeFlatTexIndexedScratch = make([][]byte, n)
	} else {
		g.planeFlatTexIndexedScratch = g.planeFlatTexIndexedScratch[:n]
	}
	if cap(g.planeFlatReadyScratch) < n {
		g.planeFlatReadyScratch = make([]bool, n)
	} else {
		g.planeFlatReadyScratch = g.planeFlatReadyScratch[:n]
		clear(g.planeFlatReadyScratch)
	}
	return g.planeFBPackedScratch, g.planeFlatTex32Scratch, g.planeFlatTexIndexedScratch, g.planeFlatReadyScratch
}

func (g *game) ensurePuffItemsScratch(n int) []projectedPuffItem {
	if n <= 0 {
		return nil
	}
	if cap(g.puffItemsScratch) < n {
		g.puffItemsScratch = make([]projectedPuffItem, 0, n)
	}
	g.puffItemsScratch = g.puffItemsScratch[:0]
	return g.puffItemsScratch
}

func (g *game) cutoutShadeStamp() int {
	if g == nil {
		return 0
	}
	return g.worldTic + 1
}

func computeCutoutShadeMul(fullBright bool, lightMul uint32, dist, near float64) uint32 {
	if fullBright {
		return 256
	}
	return uint32(combineShadeMul(int(monsterShadeFactor(dist, near)*256.0), int(lightMul)))
}

func (g *game) ensureThingShadeCache() {
	if g == nil || g.m == nil {
		return
	}
	if len(g.thingShadeTick) != len(g.m.Things) {
		g.thingShadeTick = make([]int, len(g.m.Things))
		g.thingShadeMul = make([]uint32, len(g.m.Things))
	}
}

func (g *game) ensureProjectileShadeCache() {
	if g == nil {
		return
	}
	if len(g.projectileShadeTick) != len(g.projectiles) {
		g.projectileShadeTick = make([]int, len(g.projectiles))
		g.projectileShadeMul = make([]uint32, len(g.projectiles))
	}
	if len(g.impactShadeTick) != len(g.projectileImpacts) {
		g.impactShadeTick = make([]int, len(g.projectileImpacts))
		g.impactShadeMul = make([]uint32, len(g.projectileImpacts))
	}
}

func (g *game) cachedThingShadeMul(i int, fullBright bool, lightMul uint32, dist, near float64) uint32 {
	if g == nil || i < 0 {
		return computeCutoutShadeMul(fullBright, lightMul, dist, near)
	}
	g.ensureThingShadeCache()
	if i >= len(g.thingShadeTick) {
		return computeCutoutShadeMul(fullBright, lightMul, dist, near)
	}
	stamp := g.cutoutShadeStamp()
	if g.thingShadeTick[i] == stamp {
		return g.thingShadeMul[i]
	}
	shade := computeCutoutShadeMul(fullBright, lightMul, dist, near)
	g.thingShadeTick[i] = stamp
	g.thingShadeMul[i] = shade
	return shade
}

func (g *game) cachedProjectileShadeMul(i int, fullBright bool, lightMul uint32, dist, near float64) uint32 {
	if g == nil || i < 0 {
		return computeCutoutShadeMul(fullBright, lightMul, dist, near)
	}
	g.ensureProjectileShadeCache()
	if i >= len(g.projectileShadeTick) {
		return computeCutoutShadeMul(fullBright, lightMul, dist, near)
	}
	stamp := g.cutoutShadeStamp()
	if g.projectileShadeTick[i] == stamp {
		return g.projectileShadeMul[i]
	}
	shade := computeCutoutShadeMul(fullBright, lightMul, dist, near)
	g.projectileShadeTick[i] = stamp
	g.projectileShadeMul[i] = shade
	return shade
}

func (g *game) cachedImpactShadeMul(i int, fullBright bool, lightMul uint32, dist, near float64) uint32 {
	if g == nil || i < 0 {
		return computeCutoutShadeMul(fullBright, lightMul, dist, near)
	}
	g.ensureProjectileShadeCache()
	if i >= len(g.impactShadeTick) {
		return computeCutoutShadeMul(fullBright, lightMul, dist, near)
	}
	stamp := g.cutoutShadeStamp()
	if g.impactShadeTick[i] == stamp {
		return g.impactShadeMul[i]
	}
	shade := computeCutoutShadeMul(fullBright, lightMul, dist, near)
	g.impactShadeTick[i] = stamp
	g.impactShadeMul[i] = shade
	return shade
}

func (g *game) drawBillboardRowSpans(row, ty, tw, x0 int, txLUT, txRunEndLUT []int, spans []solidSpan, tex *WallTexture, src32 []uint32, srcIndexed []byte, shadeMul uint32, shadeRow []uint32, fixedDOOMRow int) {
	if tex == nil {
		return
	}
	spans = g.cutoutVisibleSpansForRow(row, spans)
	if len(spans) == 0 {
		return
	}
	useIndexed := ty >= 0 && ty < tex.Height && len(srcIndexed) == tw*tex.Height
	if g.drawBillboardRowOpaqueRuns(row, ty, tw, x0, txLUT, txRunEndLUT, spans, tex, src32, srcIndexed, shadeMul, shadeRow, fixedDOOMRow) {
		return
	}
	srcMask := tex.OpaqueMask
	useShadeRow := len(shadeRow) == 256
	fullbright := shadeMul >= 256
	base := ty * tw
	for _, sp := range spans {
		if useIndexed {
			for x := sp.L; x <= sp.R; {
				lutIdx := x - x0
				s0 := base + txLUT[lutIdx]
				runEnd := txRunEndLUT[lutIdx] + x0
				if runEnd > sp.R {
					runEnd = sp.R
				}
				if srcMask[s0] != 0 {
					dst := uint32(0)
					if useShadeRow {
						dst = shadeRow[srcIndexed[s0]]
					} else if fixedDOOMRow >= 0 {
						dst = shadePaletteIndexDOOMRow(srcIndexed[s0], fixedDOOMRow)
					} else {
						dst = shadePaletteIndexPacked(srcIndexed[s0], shadeMul)
					}
					if runEnd > x {
						g.fillCutoutRowSpan(row, x, runEnd, dst)
					} else {
						i := row + x
						g.writeWallPixel(i, dst)
						g.markCutoutCoveredAtIndex(i)
					}
				}
				x = runEnd + 1
			}
			continue
		}
		for x := sp.L; x <= sp.R; {
			lutIdx := x - x0
			p0 := src32[base+txLUT[lutIdx]]
			runEnd := txRunEndLUT[lutIdx] + x0
			if runEnd > sp.R {
				runEnd = sp.R
			}
			if ((p0 >> pixelAShift) & 0xFF) != 0 {
				dst := uint32(0)
				if fixedDOOMRow >= 0 {
					dst = shadePackedDOOMColormapRow(p0, fixedDOOMRow)
				} else if fullbright {
					dst = p0 | pixelOpaqueA
				} else {
					dst = shadePackedRGBA(p0, shadeMul)
				}
				if runEnd > x {
					g.fillCutoutRowSpan(row, x, runEnd, dst)
				} else {
					i := row + x
					g.writeWallPixel(i, dst)
					g.markCutoutCoveredAtIndex(i)
				}
			}
			x = runEnd + 1
		}
	}
}

func (g *game) drawBillboardRowOpaqueRuns(row, ty, tw, x0 int, txLUT, txRunEndLUT []int, spans []solidSpan, tex *WallTexture, src32 []uint32, srcIndexed []byte, shadeMul uint32, shadeRow []uint32, fixedDOOMRow int) bool {
	if g == nil || tex == nil || ty < 0 || ty >= tex.Height || len(tex.OpaqueRowOffs) != tex.Height+1 || len(txLUT) == 0 {
		return false
	}
	runStart := int(tex.OpaqueRowOffs[ty])
	runEnd := int(tex.OpaqueRowOffs[ty+1])
	if runStart >= runEnd {
		return true
	}
	if len(tex.OpaqueRowRuns) < runEnd {
		return false
	}
	useIndexed := len(srcIndexed) == tw*tex.Height
	useShadeRow := len(shadeRow) == 256
	fullbright := shadeMul >= 256
	base := ty * tw
	for _, sp := range spans {
		for i := runStart; i < runEnd; i++ {
			minTex, maxTex := media.UnpackOpaqueRun(tex.OpaqueRowRuns[i])
			l, r, ok := trimSpanToOpaqueLUTRange(sp.L, sp.R, x0, txLUT, minTex, maxTex)
			if !ok {
				continue
			}
			if useIndexed {
				for x := l; x <= r; {
					lutIdx := x - x0
					s0 := base + txLUT[lutIdx]
					runEnd := txRunEndLUT[lutIdx] + x0
					if runEnd > r {
						runEnd = r
					}
					dst := uint32(0)
					if useShadeRow {
						dst = shadeRow[srcIndexed[s0]]
					} else if fixedDOOMRow >= 0 {
						dst = shadePaletteIndexDOOMRow(srcIndexed[s0], fixedDOOMRow)
					} else {
						dst = shadePaletteIndexPacked(srcIndexed[s0], shadeMul)
					}
					if runEnd > x {
						g.fillCutoutRowSpan(row, x, runEnd, dst)
					} else {
						i0 := row + x
						g.writeWallPixel(i0, dst)
						g.markCutoutCoveredAtIndex(i0)
					}
					x = runEnd + 1
				}
				continue
			}
			for x := l; x <= r; {
				lutIdx := x - x0
				p0 := src32[base+txLUT[lutIdx]]
				runEnd := txRunEndLUT[lutIdx] + x0
				if runEnd > r {
					runEnd = r
				}
				dst := uint32(0)
				if fixedDOOMRow >= 0 {
					dst = shadePackedDOOMColormapRow(p0, fixedDOOMRow)
				} else if fullbright {
					dst = p0 | pixelOpaqueA
				} else {
					dst = shadePackedRGBA(p0, shadeMul)
				}
				if runEnd > x {
					g.fillCutoutRowSpan(row, x, runEnd, dst)
				} else {
					i0 := row + x
					g.writeWallPixel(i0, dst)
					g.markCutoutCoveredAtIndex(i0)
				}
				x = runEnd + 1
			}
		}
	}
	return true
}

func (g *game) ensureMaskedMidSegScratch(n int) []maskedMidSeg {
	if n <= 0 {
		return nil
	}
	if cap(g.maskedMidSegsScratch) < n {
		g.maskedMidSegsScratch = make([]maskedMidSeg, 0, n)
	}
	g.maskedMidSegsScratch = g.maskedMidSegsScratch[:0]
	return g.maskedMidSegsScratch
}

func (g *game) drawMaskedMidSeg(ms maskedMidSeg, focal float64, shadeMul uint32, doomRow int) {
	g.drawMaskedMidSegRange(ms, ms.X0, ms.X1, focal, shadeMul, doomRow)
}

func (g *game) drawMaskedMidSegRange(ms maskedMidSeg, x0, x1 int, focal float64, shadeMul uint32, doomRow int) {
	if ms.tex.from == nil || ms.tex.from.Width <= 0 || ms.tex.from.Height <= 0 {
		return
	}
	if x0 < ms.X0 {
		x0 = ms.X0
	}
	if x1 > ms.X1 {
		x1 = ms.X1
	}
	if x0 > x1 {
		return
	}
	ms.X0 = x0
	ms.X1 = x1
	halfH := float64(g.viewH) * 0.5
	if g.maskedMidSegFullyOccluded(ms, focal, halfH) {
		return
	}
	g.drawMaskedMidSegColumns(ms, focal, halfH, int(shadeMul), doomRow)
}

func (g *game) maskedMidSegFullyOccluded(ms maskedMidSeg, focal, halfH float64) bool {
	if g == nil || !g.billboardClippingEnabled() || ms.X0 > ms.X1 {
		return false
	}
	if ms.hasOcclusionBBox {
		return g.spriteWallClipQuadFullyOccluded(ms.X0, ms.X1, int(ms.occlusionY0), int(ms.occlusionY1), ms.occlusionDepthQ)
	}
	y0, y1, depthQ, ok := g.maskedMidRangeOcclusionBounds(ms.Projection, ms.X0, ms.X1, ms.WorldHigh, ms.WorldLow, focal, halfH)
	if !ok {
		return false
	}
	return g.spriteWallClipQuadFullyOccluded(ms.X0, ms.X1, y0, y1, depthQ)
}

func maskedMidDepthSamples(proj scene.WallProjection, x0, x1 int) (float64, float64, bool) {
	if x0 > x1 {
		return 0, 0, false
	}
	sampleX := (x0 + x1) >> 1
	centerDepth := 0.0
	if depth, _, ok := scene.ProjectedWallSampleAtX(proj, sampleX); ok && depth > 0 && isFinite(depth) {
		centerDepth = depth
	}

	sortDepth := 0.0
	for _, edgeX := range [2]int{x0, x1} {
		depth, _, ok := scene.ProjectedWallSampleAtX(proj, edgeX)
		if !ok || depth <= 0 || !isFinite(depth) {
			continue
		}
		if depth > sortDepth {
			sortDepth = depth
		}
	}
	if centerDepth > 0 {
		if sortDepth <= 0 {
			sortDepth = centerDepth
		}
		return centerDepth, sortDepth, true
	}
	for _, edgeX := range [2]int{x0, x1} {
		depth, _, ok := scene.ProjectedWallSampleAtX(proj, edgeX)
		if ok && depth > 0 && isFinite(depth) {
			return depth, sortDepth, true
		}
	}
	return 0, 0, false
}

func maskedMidCenterDepth(proj scene.WallProjection, x0, x1 int) (float64, bool) {
	centerDepth, _, ok := maskedMidDepthSamples(proj, x0, x1)
	return centerDepth, ok
}

func maskedMidBillboardDepthGuess(proj scene.WallProjection, x0, x1 int) (float64, bool) {
	_, sortDepth, ok := maskedMidDepthSamples(proj, x0, x1)
	return sortDepth, ok
}

func (g *game) maskedMidRangeOcclusionBounds(proj scene.WallProjection, x0, x1 int, worldHigh, worldLow, focal, halfH float64) (int, int, uint16, bool) {
	if g == nil || x0 > x1 {
		return 0, 0, 0, false
	}
	bestDepth, ok := maskedMidCenterDepth(proj, x0, x1)
	if !ok {
		return 0, 0, 0, false
	}
	y0 := int(math.Ceil(halfH - (worldHigh/bestDepth)*focal))
	y1 := int(math.Floor(halfH - (worldLow/bestDepth)*focal))
	if y0 < 0 {
		y0 = 0
	}
	if y1 >= g.viewH {
		y1 = g.viewH - 1
	}
	if y0 > y1 {
		return 0, 0, 0, false
	}
	return y0, y1, encodeDepthQ(bestDepth), true
}

type maskedMidEnvelopeStepper struct {
	proj        scene.WallProjection
	t           float64
	tStep       float64
	topScale    float64
	bottomScale float64
	halfH       float64
	ok          bool
}

func newMaskedMidEnvelopeStepper(proj scene.WallProjection, x int, worldHigh, worldLow, focal, halfH float64) maskedMidEnvelopeStepper {
	if proj.SX2 == proj.SX1 {
		return maskedMidEnvelopeStepper{}
	}
	return maskedMidEnvelopeStepper{
		proj:        proj,
		t:           (float64(x) - proj.SX1) / (proj.SX2 - proj.SX1),
		tStep:       1.0 / (proj.SX2 - proj.SX1),
		topScale:    -worldHigh * focal,
		bottomScale: -worldLow * focal,
		halfH:       halfH,
		ok:          true,
	}
}

func (s maskedMidEnvelopeStepper) Sample() (int, int, bool) {
	if !s.ok {
		return 0, 0, false
	}
	t := s.t
	if t < 0 {
		t = 0
	}
	if t > 1 {
		t = 1
	}
	invDepth := s.proj.InvDepth1 + (s.proj.InvDepth2-s.proj.InvDepth1)*t
	if invDepth <= 0 {
		return 0, 0, false
	}
	y0 := int(math.Ceil(s.halfH + s.topScale*invDepth))
	y1 := int(math.Floor(s.halfH + s.bottomScale*invDepth))
	return y0, y1, true
}

func (s *maskedMidEnvelopeStepper) Next() {
	if s == nil || !s.ok {
		return
	}
	s.t += s.tStep
}

func (g *game) drawMaskedMidSegColumns(ms maskedMidSeg, focal, halfH float64, shadeMul, doomRow int) {
	stepper := scene.NewWallProjectionStepper(ms.Projection, ms.X0)
	envelope := newMaskedMidEnvelopeStepper(ms.Projection, ms.X0, ms.WorldHigh, ms.WorldLow, focal, halfH)
	for x := ms.X0; x <= ms.X1; x++ {
		f, texU, ok := stepper.Sample()
		y0, y1, envOK := envelope.Sample()
		stepper.Next()
		envelope.Next()
		if !ok {
			continue
		}
		if f <= 0 || !isFinite(f) {
			continue
		}
		if !envOK {
			continue
		}
		texU += ms.TexUOff
		if y0 > y1 {
			continue
		}
		g.drawBasicWallColumnTexturedMasked(x, y0, y1, f, texU, ms.TexMid, focal, ms.tex, shadeMul, doomRow)
	}
}

func fillUint32Span(dst []uint32, value uint32) {
	if len(dst) == 0 {
		return
	}
	dst[0] = value
	for filled := 1; filled < len(dst); filled *= 2 {
		copyLen := filled
		if copyLen > len(dst)-filled {
			copyLen = len(dst) - filled
		}
		copy(dst[filled:filled+copyLen], dst[:copyLen])
	}
}

func (g *game) fillCutoutRowSpan(row, x0, x1 int, value uint32) {
	if g == nil || x0 > x1 {
		return
	}
	fillUint32Span(g.wallPix32[row+x0:row+x1+1], value)
	g.markCutoutRowSpanCovered(row, x0, x1)
}

func (g *game) fillCutoutColumnSpan(pixI, rowStridePix, count int, value uint32) {
	if g == nil || count <= 0 {
		return
	}
	for i := 0; i < count; i++ {
		g.wallPix32[pixI] = value
		g.markCutoutCoveredAtIndex(pixI)
		pixI += rowStridePix
	}
}

func maskedWallShadePackedRow(shadeMul, doomRow int) []uint32 {
	if doomRow >= doomNumColorMaps || doomColormapEnabled {
		return doomColormapPackedRow(doomRow)
	}
	if !wallShadePackedOK {
		return nil
	}
	if shadeMul < 0 {
		shadeMul = 0
	} else if shadeMul > 256 {
		shadeMul = 256
	}
	return wallShadePackedLUT[shadeMul][:]
}

func (g *game) maskedMidShade(light int16) (int, int) {
	if light < 0 {
		light = 0
	} else if light > 255 {
		light = 255
	}
	li := int(light)
	stamp := g.cutoutShadeStamp()
	cacheKey := uint16(0)
	if doomLightingEnabled {
		cacheKey |= 1 << 0
	}
	if doomColormapEnabled {
		cacheKey |= 1 << 1
	}
	if g != nil && g.opts.SourcePortMode {
		cacheKey |= 1 << 2
	}
	if row, ok := g.playerFixedColormapRow(); ok {
		cacheKey |= 1 << 3
		cacheKey |= uint16(row&0xFF) << 8
	}
	if g.maskedMidShadeTick[li] == stamp && g.maskedMidShadeKey[li] == cacheKey {
		return int(g.maskedMidShadeMulCache[li]), g.maskedMidDoomRowCache[li]
	}
	shadeMul := sectorLightMul(light)
	doomRow := 0
	if !doomLightingEnabled {
		g.maskedMidShadeTick[li] = stamp
		g.maskedMidShadeKey[li] = cacheKey
		g.maskedMidShadeMulCache[li] = uint32(shadeMul)
		g.maskedMidDoomRowCache[li] = doomRow
		return shadeMul, doomRow
	}
	lightNum := doomClampLightNum(int(light) >> doomLightSegShift)
	doomRow = doomClampColorMapRow(doomStartMap(lightNum))
	if row, ok := g.playerFixedColormapRow(); ok {
		doomRow = row
	}
	if doomColormapEnabled {
		return shadeMul, doomRow
	}
	if g != nil && !g.opts.SourcePortMode {
		shadeMul = doomShadeMulFromRow(doomRow)
	} else {
		rowF := doomStartMapF(doomClampLightNumF(float64(light) / float64(1<<doomLightSegShift)))
		shadeMul = doomShadeMulFromRowF(rowF)
	}
	g.maskedMidShadeTick[li] = stamp
	g.maskedMidShadeKey[li] = cacheKey
	g.maskedMidShadeMulCache[li] = uint32(shadeMul)
	g.maskedMidDoomRowCache[li] = doomRow
	return shadeMul, doomRow
}

func maskedMidTextureColumn(tex WallTexture, texU float64) int {
	txi := int(floorFixed(texU) >> fracBits)
	if tex.Width > 0 && (tex.Width&(tex.Width-1)) == 0 {
		return txi & (tex.Width - 1)
	}
	return wrapIndex(txi, tex.Width)
}

func (g *game) collectCutoutItems(camX, camY, camAng, focal, near float64) {
	g.billboardQueueCollect = true
	g.billboardQueueScratch = g.billboardQueueScratch[:0]
	g.projectedOpaqueRectScratch = g.projectedOpaqueRectScratch[:0]
	g.appendProjectileCutoutItems(camX, camY, camAng, focal, near)
	g.appendHitscanPuffCutoutItems(camX, camY, camAng, focal, near)
	g.appendMonsterCutoutItems(camX, camY, camAng, focal, near)
	g.appendThingCutoutItems(camX, camY, camAng, focal, near)
	g.billboardQueueCollect = false
}

func (g *game) sortCutoutItemsFrontToBack() {
	slices.SortFunc(g.billboardQueueScratch, func(a, b cutoutItem) int {
		if a.dist < b.dist {
			return -1
		}
		if a.dist > b.dist {
			return 1
		}
		if a.depthQ < b.depthQ {
			return -1
		}
		if a.depthQ > b.depthQ {
			return 1
		}
		if a.x0 < b.x0 {
			return -1
		}
		if a.x0 > b.x0 {
			return 1
		}
		if a.x1 < b.x1 {
			return -1
		}
		if a.x1 > b.x1 {
			return 1
		}
		if a.y0 < b.y0 {
			return -1
		}
		if a.y0 > b.y0 {
			return 1
		}
		if a.y1 < b.y1 {
			return -1
		}
		if a.y1 > b.y1 {
			return 1
		}
		if a.kind < b.kind {
			return -1
		}
		if a.kind > b.kind {
			return 1
		}
		if a.idx < b.idx {
			return -1
		}
		if a.idx > b.idx {
			return 1
		}
		return 0
	})
}

func (g *game) drawCutoutItem(it cutoutItem, focal float64) {
	switch it.kind {
	case billboardQueueProjectiles:
		g.drawSpriteCutoutItem(it)
	case billboardQueuePuffs:
		g.drawSpriteCutoutItem(it)
	case billboardQueueMonsters:
		g.drawSpriteCutoutItem(it)
	case billboardQueueWorldThings:
		g.drawSpriteCutoutItem(it)
	case billboardQueueMaskedMids:
		if it.idx >= 0 && it.idx < len(g.maskedMidSegsScratch) {
			g.drawMaskedMidSegRange(g.maskedMidSegsScratch[it.idx], it.x0, it.x1, focal, it.shadeMul, it.doomRow)
		}
	}
}

func (g *game) drawSpriteCutoutItem(it cutoutItem) {
	if g == nil || !it.boundsOK || it.tex == nil {
		return
	}
	viewW := g.viewW
	th := it.tex.Height
	tw := it.tex.Width
	if th <= 0 || tw <= 0 {
		return
	}
	src32, ok32 := spritePixels32(it.tex)
	srcIndexed, _, _ := spriteIndexedPixels(it.tex)
	useIndexed := len(srcIndexed) == tw*th
	if !ok32 && !useIndexed {
		return
	}
	if it.shadow && !ok32 {
		return
	}
	scale := it.scale
	if scale <= 0 {
		return
	}
	x0, x1, y0, y1 := it.x0, it.x1, it.y0, it.y1
	if x0 > x1 || y0 > y1 {
		return
	}
	projectedOpaque := g.projectedOpaqueRectScratch[it.opaqueRectStart : it.opaqueRectStart+it.opaqueRectCount]
	if !it.debugOverlay && len(projectedOpaque) > 0 && g.projectedOpaqueRectsFullyCoveredByCutout(projectedOpaque) {
		return
	}
	if (!it.debugOverlay && len(projectedOpaque) > 0 && g.projectedOpaqueRectsFullyOccluded(projectedOpaque, it.depthQ)) ||
		g.spriteWallClipQuadFullyOccluded(x0, x1, y0, y1, it.depthQ) {
		return
	}
	shadeMul := it.shadeMul
	fixedDOOMRow := -1
	var shadeRow []uint32
	if row, ok := g.playerFixedColormapRow(); ok {
		fixedDOOMRow = row
		if useIndexed {
			shadeRow = doomColormapPackedRow(row)
		}
	} else if useIndexed && wallShadePackedOK {
		if shadeMul > 256 {
			shadeMul = 256
		}
		shadeRow = wallShadePackedLUT[shadeMul][:]
	}
	txLUT := g.ensureSpriteTXScratch(x1 - x0 + 1)
	for x := x0; x <= x1; x++ {
		tx := int((float64(x) + 0.5 - it.dstX) / scale)
		if tx < 0 {
			tx = 0
		}
		if tx >= tw {
			tx = tw - 1
		}
		if it.flip {
			tx = tw - 1 - tx
		}
		txLUT[x-x0] = tx
	}
	txRunEndLUT := g.buildSpriteTXRunEnds(txLUT)
	tyLUT := g.ensureSpriteTYScratch(y1 - y0 + 1)
	for y := y0; y <= y1; y++ {
		ty := int((float64(y) + 0.5 - it.dstY) / scale)
		if ty < 0 {
			ty = 0
		}
		if ty >= th {
			ty = th - 1
		}
		tyLUT[y-y0] = ty
	}
	if it.hasOpaque {
		x0, x1, txLUT = trimScreenRangeToOpaqueLUT(x0, x1, txLUT, it.opaque.bounds.minX, it.opaque.bounds.maxX)
		y0, y1, tyLUT = trimScreenRangeToOpaqueLUT(y0, y1, tyLUT, it.opaque.bounds.minY, it.opaque.bounds.maxY)
		if x0 > x1 || y0 > y1 {
			return
		}
	}
	if !it.shadow && scale >= spriteMagnifyMinScale {
		if g.drawSpriteCutoutMagnifiedMask(it, tw, x0, x1, y0, y1, txLUT, tyLUT, src32, srcIndexed, shadeMul, shadeRow, fixedDOOMRow) {
			return
		}
	}
	if it.shadow && g.opts.SourcePortMode {
		g.drawShadowSpriteCutoutSourcePort(it, src32, tw, txLUT, tyLUT, x0, x1, y0, y1)
		return
	}
	if it.shadow {
		for y := y0; y <= y1; y++ {
			row := y * viewW
			rowSpans := g.spriteRowVisibleSpansDepthQ(y, x0, x1, it.depthQ, it.clipSpans, g.solidClipScratch[:0])
			g.solidClipScratch = rowSpans
			for _, sp := range rowSpans {
				for x := sp.L; x <= sp.R; x++ {
					i := row + x
					p := src32[tyLUT[y-y0]*tw+txLUT[x-x0]]
					if ((p >> pixelAShift) & 0xFF) == 0 {
						continue
					}
					g.writeFuzzPixel(x, y, i)
				}
			}
		}
		return
	}
	for y := y0; y <= y1; y++ {
		ty := tyLUT[y-y0]
		row := y * viewW
		if !it.debugOverlay && len(it.clipSpans) == 0 && x1-x0 >= spriteRowOcclusionMinSpan && g.rowFullyOccludedDepthQ(it.depthQ, row, x0, x1) {
			continue
		}
		rowSpans := g.spriteRowVisibleSpansDepthQ(y, x0, x1, it.depthQ, it.clipSpans, g.solidClipScratch[:0])
		g.solidClipScratch = rowSpans
		if len(rowSpans) == 0 {
			continue
		}
		if it.hasOpaque && ty >= 0 && ty < len(it.opaque.rowMin) {
			minTex := int(it.opaque.rowMin[ty])
			maxTex := int(it.opaque.rowMax[ty])
			if maxTex < minTex {
				continue
			}
			filtered := g.solidClipScratch[:0]
			for _, sp := range rowSpans {
				l, r, ok := trimSpanToOpaqueLUTRange(sp.L, sp.R, x0, txLUT, minTex, maxTex)
				if ok {
					filtered = append(filtered, solidSpan{L: l, R: r})
				}
			}
			g.solidClipScratch = filtered
			rowSpans = filtered
			if len(rowSpans) == 0 {
				continue
			}
		}
		if it.debugOverlay {
			g.drawBillboardRowSpansDebugRed(row, ty, tw, x0, txLUT, txRunEndLUT, rowSpans, it.tex, src32, srcIndexed)
			continue
		}
		g.drawBillboardRowSpans(row, ty, tw, x0, txLUT, txRunEndLUT, rowSpans, it.tex, src32, srcIndexed, shadeMul, shadeRow, fixedDOOMRow)
	}
}

func (g *game) drawBillboardRowSpansDebugRed(row, ty, tw, x0 int, txLUT, txRunEndLUT []int, spans []solidSpan, tex *WallTexture, src32 []uint32, srcIndexed []byte) {
	if g == nil || tex == nil {
		return
	}
	useIndexed := ty >= 0 && ty < tex.Height && len(srcIndexed) == tw*tex.Height
	base := ty * tw
	mask := tex.OpaqueMask
	if useIndexed && len(mask) != tw*tex.Height {
		tex.EnsureOpaqueMask()
		mask = tex.OpaqueMask
	}
	red := packRGBA(255, 0, 0)
	for _, sp := range spans {
		for x := sp.L; x <= sp.R; {
			lutIdx := x - x0
			srcIdx := base + txLUT[lutIdx]
			runEnd := txRunEndLUT[lutIdx] + x0
			if runEnd > sp.R {
				runEnd = sp.R
			}
			opaque := false
			if useIndexed {
				opaque = len(mask) == tw*tex.Height && mask[srcIdx] != 0
			} else {
				opaque = ((src32[srcIdx] >> pixelAShift) & 0xFF) != 0
			}
			if opaque {
				if runEnd > x {
					g.fillCutoutRowSpan(row, x, runEnd, red)
				} else {
					g.writeWallPixel(row+x, red)
				}
			}
			x = runEnd + 1
		}
	}
}

func (g *game) drawSpriteCutoutMagnifiedMask(it cutoutItem, tw, x0, x1, y0, y1 int, txLUT, tyLUT []int, src32 []uint32, srcIndexed []byte, shadeMul uint32, shadeRow []uint32, fixedDOOMRow int) bool {
	if g == nil || it.tex == nil || x0 > x1 || y0 > y1 {
		return false
	}
	txRunEndLUT := g.buildSpriteTXRunEnds(txLUT)
	it.tex.EnsureOpaqueMask()
	if len(it.tex.OpaqueMask) != tw*it.tex.Height {
		return false
	}
	used := false
	mask := it.tex.OpaqueMask
	viewW := g.viewW
	for ty := 0; ty < it.tex.Height; ty++ {
		rowMask := mask[ty*tw : (ty+1)*tw]
		for tx0 := 0; tx0 < tw; {
			for tx0 < tw && rowMask[tx0] == 0 {
				tx0++
			}
			if tx0 >= tw {
				break
			}
			tx1 := tx0
			for tx1+1 < tw && rowMask[tx1+1] != 0 {
				tx1++
			}
			rect := packSpriteOpaqueRect(tx0, tx1, ty, ty)
			if it.flip {
				rect = flipSpriteOpaqueRectX(rect, tw)
			}
			rx0, rx1, ry0, ry1, ok := spriteRectScreenBounds(rect, it.dstX, it.dstY, it.scale, it.clipTop, it.clipBottom, g.viewW, g.viewH)
			if !ok {
				tx0 = tx1 + 1
				continue
			}
			if rx0 < x0 {
				rx0 = x0
			}
			if rx1 > x1 {
				rx1 = x1
			}
			if ry0 < y0 {
				ry0 = y0
			}
			if ry1 > y1 {
				ry1 = y1
			}
			if rx0 > rx1 || ry0 > ry1 {
				tx0 = tx1 + 1
				continue
			}
			if g.cutoutMaskRectFullyCovered(rx0, rx1, ry0, ry1) {
				tx0 = tx1 + 1
				continue
			}
			for yy := ry0; yy <= ry1; yy++ {
				row := yy * viewW
				if len(it.clipSpans) == 0 && rx1-rx0 >= spriteRowOcclusionMinSpan && g.rowFullyOccludedDepthQ(it.depthQ, row, rx0, rx1) {
					continue
				}
				rowSpans := g.spriteRowVisibleSpansDepthQ(yy, rx0, rx1, it.depthQ, it.clipSpans, g.solidClipScratch[:0])
				g.solidClipScratch = rowSpans
				if len(rowSpans) == 0 {
					continue
				}
				filtered := g.solidClipScratch[:0]
				for _, sp := range rowSpans {
					l, r, ok := trimSpanToOpaqueLUTRange(sp.L, sp.R, x0, txLUT, tx0, tx1)
					if ok {
						filtered = append(filtered, solidSpan{L: l, R: r})
					}
				}
				g.solidClipScratch = filtered
				rowSpans = filtered
				if len(rowSpans) == 0 {
					continue
				}
				used = true
				g.drawBillboardRowSpans(row, ty, tw, x0, txLUT, txRunEndLUT, rowSpans, it.tex, src32, srcIndexed, shadeMul, shadeRow, fixedDOOMRow)
			}
			tx0 = tx1 + 1
		}
	}
	return used
}

func (g *game) appendMaskedMidSegsToCutoutItems() {
	if len(g.maskedMidSegsScratch) == 0 {
		return
	}
	for i := range g.maskedMidSegsScratch {
		g.appendMaskedMidSegToCutoutItems(i, g.maskedMidSegsScratch[i])
	}
}

func (g *game) appendMaskedMidSegsToBillboardQueue() {
	g.appendMaskedMidSegsToCutoutItems()
}

func (g *game) appendMaskedMidSegToCutoutItems(idx int, ms maskedMidSeg) {
	if g == nil || ms.tex.from == nil || ms.X0 > ms.X1 {
		return
	}
	const maskedMidSortTexelGroup = 16
	shadeMul, doomRow := g.maskedMidShade(ms.light)
	stepper := scene.NewWallProjectionStepper(ms.Projection, ms.X0)
	runX0 := -1
	runTX := 0
	flush := func(x0, x1 int) {
		if x0 > x1 {
			return
		}
		dist, ok := maskedMidBillboardDepthGuess(ms.Projection, x0, x1)
		if !ok {
			return
		}
		g.billboardQueueScratch = append(g.billboardQueueScratch, cutoutItem{
			dist:     quantizeMaskedMidSortDist(dist),
			depthQ:   encodeDepthQ(dist),
			kind:     billboardQueueMaskedMids,
			idx:      idx,
			x0:       x0,
			x1:       x1,
			shadeMul: uint32(shadeMul),
			doomRow:  doomRow,
		})
	}
	for x := ms.X0; x <= ms.X1; x++ {
		_, texU, ok := stepper.Sample()
		stepper.Next()
		if !ok {
			if runX0 >= 0 {
				flush(runX0, x-1)
				runX0 = -1
			}
			continue
		}
		tx := maskedMidTextureColumn(*ms.tex.from, texU+ms.TexUOff)
		if maskedMidSortTexelGroup > 1 {
			tx /= maskedMidSortTexelGroup
		}
		if runX0 < 0 {
			runX0 = x
			runTX = tx
			continue
		}
		if tx != runTX {
			flush(runX0, x-1)
			runX0 = x
			runTX = tx
		}
	}
	if runX0 >= 0 {
		flush(runX0, ms.X1)
	}
}

func quantizeMaskedMidSortDist(dist float64) float64 {
	if dist <= 0 || !isFinite(dist) || maskedMidSortBucket <= 0 {
		return dist
	}
	return math.Round(dist/maskedMidSortBucket) * maskedMidSortBucket
}

func (g *game) finalizeMaskedClipColumns() {
	// Masked clip columns are kept sorted during insertion.
}

func wallSpecialScrollXOffset(special uint16, worldTic int) float64 {
	// Doom linedef special 48: first-column wall scroll.
	if special == 48 {
		return float64(worldTic)
	}
	return 0
}

func drawWallColumnTexturedIndexedLEColPow2Row(pix32 []uint32, pixI, rowStridePix int, col []byte, texVFixed, texVStepFixed int64, hmask, count int, row []uint32) {
	const fracMask = fracUnit - 1
	ty := int(texVFixed >> fracBits)
	frac := int(texVFixed & fracMask)
	stepInt := int(texVStepFixed >> fracBits)
	stepFrac := int(texVStepFixed & fracMask)
	rowStridePix2 := rowStridePix * 2
	colData := col
	rowData := row
	cur := ty & hmask
	if stepFrac == 0 {
		if stepInt == 0 {
			p := rowData[colData[cur]]
			for ; count >= 4; count -= 4 {
				pix32[pixI] = p
				pix32[pixI+rowStridePix] = p
				pix32[pixI+rowStridePix2] = p
				pix32[pixI+rowStridePix2+rowStridePix] = p
				pixI += rowStridePix2 * 2
			}
			for ; count > 0; count-- {
				pix32[pixI] = p
				pixI += rowStridePix
			}
			return
		}
		for ; count >= 4; count -= 4 {
			pix32[pixI] = rowData[colData[cur]]
			cur = (cur + stepInt) & hmask
			pix32[pixI+rowStridePix] = rowData[colData[cur]]
			cur = (cur + stepInt) & hmask
			pix32[pixI+rowStridePix2] = rowData[colData[cur]]
			cur = (cur + stepInt) & hmask
			pix32[pixI+rowStridePix2+rowStridePix] = rowData[colData[cur]]
			cur = (cur + stepInt) & hmask
			pixI += rowStridePix2 * 2
		}
		for ; count >= 2; count -= 2 {
			pix32[pixI] = rowData[colData[cur]]
			cur = (cur + stepInt) & hmask
			pix32[pixI+rowStridePix] = rowData[colData[cur]]
			cur = (cur + stepInt) & hmask
			pixI += rowStridePix2
		}
		if count > 0 {
			pix32[pixI] = rowData[colData[cur]]
		}
		return
	}
	if texVStepFixed > -fracUnit && texVStepFixed < fracUnit {
		if stepInt == 0 {
			for count > 0 {
				cur = ty & hmask
				run := stepsUntilTexelChangeFrac(frac, stepFrac)
				if run > count {
					run = count
				}
				fillPackedRunStride(pix32, pixI, rowStridePix, run, rowData[colData[cur]])
				pixI += rowStridePix * run
				count -= run
				if count <= 0 {
					return
				}
				frac += run * stepFrac
				ty += frac >> fracBits
				frac &= fracMask
			}
			return
		}
		for count > 0 {
			cur = ty & hmask
			p := rowData[colData[cur]]
			for {
				pix32[pixI] = p
				pixI += rowStridePix
				count--
				if count <= 0 {
					return
				}
				frac += stepFrac
				ty += stepInt + (frac >> fracBits)
				frac &= fracMask
				if (ty & hmask) != cur {
					break
				}
			}
		}
		return
	}
	for ; count >= 2; count -= 2 {
		ty0 := ty & hmask
		frac += stepFrac
		ty += stepInt + (frac >> fracBits)
		frac &= fracMask
		ty1 := ty & hmask
		frac += stepFrac
		ty += stepInt + (frac >> fracBits)
		frac &= fracMask
		pix32[pixI] = row[col[ty0]]
		pix32[pixI+rowStridePix] = row[col[ty1]]
		pixI += rowStridePix2
	}
	if count > 0 {
		pix32[pixI] = row[col[ty&hmask]]
	}
}

func initWallShadeLUT() {
	for mul := 0; mul <= 256; mul++ {
		for c := 0; c < 256; c++ {
			wallShadeLUT[mul][c] = uint8((c * mul) >> 8)
		}
	}
}

func initWallShadePackedLUT(paletteRGBA []byte) {
	wallShadePackedOK = false
	paletteIndexByPacked = nil
	if len(paletteRGBA) < 256*4 {
		clear(wallShadePackedBanks[:])
		clear(wallShadePackedLUT[:])
		return
	}
	paletteIndexByPacked = make(map[uint32]uint8, 256)
	for idx := 0; idx < 256; idx++ {
		pi := idx * 4
		base := pixelOpaqueA |
			(uint32(paletteRGBA[pi+0]) << pixelRShift) |
			(uint32(paletteRGBA[pi+1]) << pixelGShift) |
			(uint32(paletteRGBA[pi+2]) << pixelBShift)
		paletteIndexByPacked[base] = uint8(idx)
	}
	for gamma := 0; gamma < doomGammaLevels; gamma++ {
		for mul := 0; mul <= 256; mul++ {
			row := &wallShadePackedBanks[gamma][mul]
			for idx := 0; idx < 256; idx++ {
				pi := idx * 4
				r := uint32(paletteRGBA[pi+0])
				g := uint32(paletteRGBA[pi+1])
				b := uint32(paletteRGBA[pi+2])
				if mul < 256 {
					r = (r * uint32(mul)) >> 8
					g = (g * uint32(mul)) >> 8
					b = (b * uint32(mul)) >> 8
				}
				r = uint32(doomGammaTables[gamma][uint8(r)])
				g = uint32(doomGammaTables[gamma][uint8(g)])
				b = uint32(doomGammaTables[gamma][uint8(b)])
				row[idx] = pixelOpaqueA | (r << pixelRShift) | (g << pixelGShift) | (b << pixelBShift)
			}
		}
	}
	applyActiveGammaLUTs()
	wallShadePackedOK = true
}

func initSectorLightMulLUT() {
	for i := 0; i < len(sectorLightMulLUT); i++ {
		sectorLightMulLUT[i] = uint8(i)
	}
}

func initDoomColormapShading(paletteRGBA, colorMap []byte, rows int, enableColormapRemap bool) {
	fullbrightNoLighting = false
	doomLightingEnabled = false
	doomSectorLighting = true
	doomColormapEnabled = false
	doomColormapRows = 0
	doomRowShadeMulLUT = nil
	clear(doomColormapBanks[:])
	doomColormapRGBA = nil
	doomPalIndexLUT32 = nil
	if len(colorMap) < 256 || rows <= 0 {
		return
	}
	maxRows := len(colorMap) / 256
	if rows > maxRows {
		rows = maxRows
	}
	if rows <= 0 {
		return
	}
	doomColormapRows = rows
	doomLightingEnabled = true
	doomRowShadeMulLUT = buildDoomRowShadeMulLUT(paletteRGBA, colorMap, rows)
	if len(paletteRGBA) < 256*4 {
		return
	}
	doomPalIndexLUT32 = buildPaletteIndexLUT32(paletteRGBA)
	if len(doomPalIndexLUT32) != 32*32*32 {
		return
	}
	for gamma := 0; gamma < doomGammaLevels; gamma++ {
		doomColormapBanks[gamma] = make([]uint32, rows*256)
		for r := 0; r < rows; r++ {
			rowBase := r * 256
			for i := 0; i < 256; i++ {
				pi := int(colorMap[rowBase+i]) * 4
				if pi+3 >= len(paletteRGBA) {
					doomColormapBanks[gamma][rowBase+i] = packRGBA(0, 0, 0)
					continue
				}
				doomColormapBanks[gamma][rowBase+i] = packRGBA(
					doomGammaTables[gamma][paletteRGBA[pi+0]],
					doomGammaTables[gamma][paletteRGBA[pi+1]],
					doomGammaTables[gamma][paletteRGBA[pi+2]],
				)
			}
		}
	}
	applyActiveGammaLUTs()
	doomColormapEnabled = enableColormapRemap
}

func disableDoomLighting() {
	fullbrightNoLighting = true
	doomLightingEnabled = false
	doomSectorLighting = false
	doomColormapEnabled = false
	doomColormapRows = 0
	doomRowShadeMulLUT = nil
	clear(doomColormapBanks[:])
	doomColormapRGBA = nil
	doomPalIndexLUT32 = nil
}

func applyActiveGammaLUTs() {
	level := clampGamma(activeGammaLevel)
	activeGammaLevel = level
	wallShadePackedLUT = wallShadePackedBanks[level]
	doomColormapRGBA = doomColormapBanks[level]
}

func buildDoomRowShadeMulLUT(paletteRGBA, colorMap []byte, rows int) []uint16 {
	if rows <= 0 || len(paletteRGBA) < 256*4 || len(colorMap) < rows*256 {
		return nil
	}
	rowLuma := make([]int64, rows)
	for r := 0; r < rows; r++ {
		base := r * 256
		var sum int64
		for i := 0; i < 256; i++ {
			pi := int(colorMap[base+i]) * 4
			if pi+2 >= len(paletteRGBA) {
				continue
			}
			rr := int64(paletteRGBA[pi+0])
			gg := int64(paletteRGBA[pi+1])
			bb := int64(paletteRGBA[pi+2])
			// Integer luma approximation (weights sum to 256).
			sum += rr*54 + gg*183 + bb*19
		}
		rowLuma[r] = sum
	}
	ref := rowLuma[0]
	if ref <= 0 {
		return nil
	}
	mul := make([]uint16, rows)
	for r := 0; r < rows; r++ {
		m := int((rowLuma[r]*256 + ref/2) / ref)
		if m < 0 {
			m = 0
		}
		if m > 256 {
			m = 256
		}
		mul[r] = uint16(m)
	}
	mul[0] = 256
	return mul
}

func buildPaletteIndexLUT32(paletteRGBA []byte) []uint8 {
	if len(paletteRGBA) < 256*4 {
		return nil
	}
	lut := make([]uint8, 32*32*32)
	for r5 := 0; r5 < 32; r5++ {
		rv := r5*8 + 4
		for g5 := 0; g5 < 32; g5++ {
			gv := g5*8 + 4
			for b5 := 0; b5 < 32; b5++ {
				bv := b5*8 + 4
				bestIdx := 0
				bestDist := int(^uint(0) >> 1)
				for i := 0; i < 256; i++ {
					pi := i * 4
					dr := int(paletteRGBA[pi+0]) - rv
					dg := int(paletteRGBA[pi+1]) - gv
					db := int(paletteRGBA[pi+2]) - bv
					d := dr*dr + dg*dg + db*db
					if d < bestDist {
						bestDist = d
						bestIdx = i
					}
				}
				lut[(r5<<10)|(g5<<5)|b5] = uint8(bestIdx)
			}
		}
	}
	return lut
}

func floorFixed(v float64) int64 {
	f := v * fracUnit
	i := int64(f)
	if float64(i) > f {
		i--
	}
	return i
}

func (g *game) drawDoomBasicTexturedPlanesVisplanePass(pix []byte, camX, camY, ca, sa, eyeZ, focal float64, ceilFallback, floorFallback color.RGBA, planes []*plane3DVisplane) bool {
	if len(planes) == 0 {
		return false
	}
	w := g.viewW
	h := g.viewH
	if w <= 0 || h <= 0 || len(pix) != w*h*4 {
		return false
	}
	pix32 := g.wallPix32
	if len(pix32) != w*h {
		return false
	}
	stageStart := time.Now()
	spansByPlane, _, _, hasSky := g.buildPlaneSpansParallel(planes, h)
	g.addRenderStageDur(renderStagePlaneSpans, time.Since(stageStart))
	cx := float64(w) * 0.5
	cy := float64(h) * 0.5
	planeFlatSamples := make([]flatTextureBlendSample, len(planes))
	skyTexReady := hasSky &&
		len(g.frameSkyColU) == w &&
		len(g.frameSkyRowV) == h &&
		len(g.frameSkyTex32) != 0 &&
		g.frameSkyTexW > 0
	skyLayerEnabled := skyTexReady && g.frameSkyLayerEnabled
	for planeIdx, pl := range planes {
		key := pl.key
		if key.sky {
			continue
		}
		flatID := int(key.flatID)
		if flatID < 0 || flatID >= len(g.planeFlatCache32Scratch) {
			panic("plane flat cache index out of range")
		}
		sample, ok := g.flatTextureBlend(g.flatNameByID(key.flatID))
		if !ok || len(sample.fromRGBA) != 64*64*4 || len(sample.fromIndexed) != 64*64 {
			panic("missing or invalid plane flat texture")
		}
		planeFlatSamples[planeIdx] = sample
	}

	renderPlaneSpan := func(planeIdx int, sp plane3DSpan, planeClipScratch []solidSpan) []solidSpan {
		if sp.y < 0 || sp.y >= h {
			return planeClipScratch
		}
		pl := planes[planeIdx]
		key := pl.key
		sample := planeFlatSamples[planeIdx]
		x1 := sp.x1
		x2 := sp.x2
		if x1 < 0 {
			x1 = 0
		}
		if x2 >= w {
			x2 = w - 1
		}
		if x2 < x1 {
			return planeClipScratch
		}
		rowPix := sp.y * w
		rowOccluders := []billboardPlaneOccluderSpan(nil)
		if sp.y >= 0 && sp.y < len(g.billboardPlaneOccluderRows) {
			rowOccluders = g.billboardPlaneOccluderRows[sp.y]
		}
		if key.sky {
			if len(rowOccluders) != 0 {
				planeClipScratch = clipRangeAgainstBillboardPlaneOccluders(x1, x2, 0xFFFF, rowOccluders, planeClipScratch)
				if len(planeClipScratch) == 0 {
					return planeClipScratch
				}
			} else {
				planeClipScratch = append(planeClipScratch[:0], solidSpan{L: x1, R: x2})
			}
			if skyLayerEnabled {
				for _, vis := range planeClipScratch {
					clear(pix32[rowPix+vis.L : rowPix+vis.R+1])
				}
				return planeClipScratch
			}
			if skyTexReady {
				v := g.frameSkyRowV[sp.y]
				for _, vis := range planeClipScratch {
					pixI := rowPix + vis.L
					x := vis.L
					for ; x+1 <= vis.R; x += 2 {
						u0 := g.frameSkyColU[x]
						u1 := g.frameSkyColU[x+1]
						ti0 := v*g.frameSkyTexW + u0
						ti1 := v*g.frameSkyTexW + u1
						pix32[pixI] = g.frameSkyTex32[ti0]
						pix32[pixI+1] = g.frameSkyTex32[ti1]
						pixI += 2
					}
					if x <= vis.R {
						u := g.frameSkyColU[x]
						ti := v*g.frameSkyTexW + u
						pix32[pixI] = g.frameSkyTex32[ti]
					}
				}
			}
			return planeClipScratch
		}
		rowState, ok := g.planeRowRenderState(sp.y, key, eyeZ, camX, camY, ca, sa, focal, cx, cy)
		if !ok {
			return planeClipScratch
		}
		if g.rowFullyOccludedByWallsFastDepthQ(rowState.depthQ, rowPix, x1, x2) {
			return planeClipScratch
		}
		if len(rowOccluders) != 0 {
			planeClipScratch = clipRangeAgainstBillboardPlaneOccluders(x1, x2, rowState.depthQ, rowOccluders, planeClipScratch)
			if len(planeClipScratch) == 0 {
				return planeClipScratch
			}
			for _, vis := range planeClipScratch {
				g.drawPlaneTexturedSpanAtDepth(pix32, rowPix, vis.L, vis.R, key, sample, rowState)
			}
			return planeClipScratch
		}
		g.drawPlaneTexturedSpanAtDepth(pix32, rowPix, x1, x2, key, sample, rowState)
		return planeClipScratch
	}
	stageStart = time.Now()
	if workers, chunk, parallel := g.parallelWorkChunks(h); parallel && h >= 32 {
		workByBand := g.ensurePlaneSpanWorkScratch(workers)
		for planeIdx, spans := range spansByPlane {
			for _, sp := range spans {
				if sp.y < 0 || sp.y >= h {
					continue
				}
				band := sp.y / chunk
				if band >= workers {
					band = workers - 1
				}
				workByBand[band] = append(workByBand[band], planeSpanWorkItem{planeIdx: planeIdx, span: sp})
			}
		}
		var wg sync.WaitGroup
		for band := range workByBand {
			if len(workByBand[band]) == 0 {
				continue
			}
			wg.Add(1)
			go func(items []planeSpanWorkItem) {
				defer wg.Done()
				planeClipScratch := make([]solidSpan, 0, 16)
				for _, item := range items {
					planeClipScratch = renderPlaneSpan(item.planeIdx, item.span, planeClipScratch[:0])
				}
			}(workByBand[band])
		}
		wg.Wait()
	} else {
		planeClipScratch := g.solidClipScratch[:0]
		for planeIdx, spans := range spansByPlane {
			for _, sp := range spans {
				planeClipScratch = renderPlaneSpan(planeIdx, sp, planeClipScratch[:0])
			}
		}
	}
	g.addRenderStageDur(renderStagePlaneRaster, time.Since(stageStart))
	if g.opts.Debug && g.debugAimSS >= 0 {
		g.overlayDebugAimFloorOnPlanes(pix32, spansByPlane, planes, camX, camY, ca, sa, eyeZ, focal)
	}
	return skyLayerEnabled
}

func (g *game) overlayDebugAimFloorOnPlanes(pix32 []uint32, spansByPlane [][]plane3DSpan, planes []*plane3DVisplane, camX, camY, ca, sa, eyeZ, focal float64) {
	if g == nil || len(pix32) != g.viewW*g.viewH || g.debugAimSS < 0 {
		return
	}
	w := g.viewW
	h := g.viewH
	cx := float64(w) * 0.5
	cy := float64(h) * 0.5
	red := packRGBA(255, 0, 0)
	for planeIdx, pl := range planes {
		if pl == nil || !pl.key.floor {
			continue
		}
		for _, sp := range spansByPlane[planeIdx] {
			if sp.y < 0 || sp.y >= h {
				continue
			}
			x1 := sp.x1
			x2 := sp.x2
			if x1 < 0 {
				x1 = 0
			}
			if x2 >= w {
				x2 = w - 1
			}
			if x2 < x1 {
				continue
			}
			den := cy - (float64(sp.y) + 0.5)
			if math.Abs(den) < 1e-6 {
				continue
			}
			planeZ := float64(pl.key.height)
			depth := ((planeZ - eyeZ) / den) * focal
			if depth <= 0 {
				continue
			}
			stepWX := (depth / focal) * sa
			stepWY := -(depth / focal) * ca
			rowBaseWX := camX + depth*ca - ((cx-0.5)*depth/focal)*sa
			rowBaseWY := camY + depth*sa + ((cx-0.5)*depth/focal)*ca
			rowBaseWXFixed := floorFixed(rowBaseWX)
			rowBaseWYFixed := floorFixed(rowBaseWY)
			stepWXFixed := floorFixed(stepWX)
			stepWYFixed := floorFixed(stepWY)
			xOff := int64(x1)
			wxFixed := rowBaseWXFixed + xOff*stepWXFixed
			wyFixed := rowBaseWYFixed + xOff*stepWYFixed
			pixI := sp.y*w + x1
			for x := x1; x <= x2; x++ {
				if ss := g.subSectorAtFixed(wxFixed, wyFixed); ss == g.debugAimSS {
					pix32[pixI] = red
				}
				wxFixed += stepWXFixed
				wyFixed += stepWYFixed
				pixI++
			}
		}
	}
}

func (g *game) fill3DBackground(ceiling, floor color.RGBA) {
	w := g.viewW
	h := g.viewH
	if w <= 0 || h <= 0 || len(g.wallPix) != w*h*4 {
		return
	}
	mid := h / 2
	fillRows := func(y0, y1 int) {
		for y := y0; y < y1; y++ {
			row := y * w * 4
			c := floor
			if y < mid {
				c = ceiling
			}
			for x := 0; x < w; x++ {
				i := row + x*4
				g.wallPix[i+0] = c.R
				g.wallPix[i+1] = c.G
				g.wallPix[i+2] = c.B
				g.wallPix[i+3] = 255
			}
		}
	}
	fillRows(0, h)
}

func (g *game) compositePlaneLayer3D() {
	if len(g.wallPix) == 0 || len(g.mapFloorPix) == 0 || len(g.wallPix) != len(g.mapFloorPix) {
		return
	}
	copyChunk := func(start, end int) {
		for i := start; i < end; i += 4 {
			if g.mapFloorPix[i+3] == 0 {
				continue
			}
			g.wallPix[i+0] = g.mapFloorPix[i+0]
			g.wallPix[i+1] = g.mapFloorPix[i+1]
			g.wallPix[i+2] = g.mapFloorPix[i+2]
		}
	}
	copyChunk(0, len(g.mapFloorPix))
}

func (g *game) drawDoomBasicTexturedPlanesSpanPass(screen *ebiten.Image, camX, camY, ca, sa, eyeZ, focal float64, playerSec int, ceilFallback, floorFallback color.RGBA, wallTop, wallBottom []int) {
	g.ensureMapFloorLayer()
	pix := g.mapFloorPix
	w := g.viewW
	h := g.viewH
	if w <= 0 || h <= 0 || len(pix) != w*h*4 {
		return
	}
	for i := 0; i < len(pix); i += 4 {
		pix[i+0] = 0
		pix[i+1] = 0
		pix[i+2] = 0
		pix[i+3] = 0
	}
	cx := float64(w) * 0.5
	cy := float64(h) * 0.5
	baseFloorZ := float64(g.m.Sectors[playerSec].FloorHeight)
	baseCeilZ := float64(g.m.Sectors[playerSec].CeilingHeight)
	flatCache := make(map[string][]byte, 64)
	spanBuckets := make(map[plane3DKey][]plane3DSpan, 64)
	keyOrder := make([]plane3DKey, 0, 64)

	for y := 0; y < h; y++ {
		den := cy - (float64(y) + 0.5)
		if math.Abs(den) < 1e-6 {
			continue
		}
		isFloor := float64(y) > cy
		planeZ := baseCeilZ
		if isFloor {
			planeZ = baseFloorZ
		}
		depth := ((planeZ - eyeZ) / den) * focal
		if depth <= 0 {
			continue
		}
		s := (cx - 0.5) * depth / focal
		wx := camX + depth*ca - s*sa
		wy := camY + depth*sa + s*ca
		stepWX := (depth / focal) * sa
		stepWY := -(depth / focal) * ca
		runStart := -1
		var runKey plane3DKey
		flushRun := func(x int) {
			if runStart >= 0 {
				keyOrder, spanBuckets = bucketSpanByKey(spanBuckets, keyOrder, y, runStart, x-1, runKey)
				runStart = -1
			}
		}
		for x := 0; x < w; x++ {
			if isFloor {
				if x >= 0 && x < len(wallBottom) && y <= wallBottom[x] {
					flushRun(x)
					wx += stepWX
					wy += stepWY
					continue
				}
			} else {
				if x >= 0 && x < len(wallTop) && y >= wallTop[x] {
					flushRun(x)
					wx += stepWX
					wy += stepWY
					continue
				}
			}
			pkey := plane3DKey{
				height: int16(math.Round(planeZ)),
				light:  160,
				floor:  isFloor,
			}
			sec := g.sectorAt(int64(wx*fracUnit), int64(wy*fracUnit))
			if sec >= 0 && sec < len(g.m.Sectors) {
				pic := g.m.Sectors[sec].CeilingPic
				pkey.height = g.m.Sectors[sec].CeilingHeight
				pkey.light = g.sectorLightLevelCached(sec)
				if isFloor {
					pic = g.m.Sectors[sec].FloorPic
					pkey.height = g.m.Sectors[sec].FloorHeight
				}
				pkey.flatID = g.flatIDForName(pic)
				if !isFloor && isSkyFlatName(pic) {
					pkey.sky = true
				}
			}
			if runStart < 0 {
				runStart = x
				runKey = pkey
			} else if runKey != pkey {
				flushRun(x)
				runStart = x
				runKey = pkey
			}
			wx += stepWX
			wy += stepWY
		}
		flushRun(w)
	}
	slices.SortFunc(keyOrder, func(a, b plane3DKey) int {
		if a.floor != b.floor {
			if !a.floor {
				return -1
			}
			return 1
		}
		if a.sky != b.sky {
			if !a.sky {
				return -1
			}
			return 1
		}
		if a.height != b.height {
			if a.height < b.height {
				return -1
			}
			return 1
		}
		if a.light != b.light {
			if a.light < b.light {
				return -1
			}
			return 1
		}
		if a.flatID != b.flatID {
			if a.flatID < b.flatID {
				return -1
			}
			return 1
		}
		return 0
	})
	_, skyTex, skyTexOK := g.runtimeSkyTextureEntryForMap(g.m.Name)
	var skyColU []int
	var skyRowV []int
	if skyTexOK {
		camAng := math.Atan2(sa, ca)
		skyTexH := effectiveSkyTexHeight(skyTex)
		skyColU, skyRowV = g.buildSkyLookupParallel(w, h, focal, camAng, skyTex.Width, skyTexH)
	}
	coveredByRow := make([][]spanRange, h)
	for _, key := range keyOrder {
		fb := ceilFallback
		if key.floor {
			fb = floorFallback
		}
		tex := flatCache[g.flatNameByID(key.flatID)]
		if !key.sky && tex == nil {
			tex, _ = g.flatRGBAResolvedID(key.flatID)
			if len(tex) != 64*64*4 {
				panic("missing or invalid plane flat texture")
			}
			flatCache[g.flatNameByID(key.flatID)] = tex
		}
		for _, sp := range spanBuckets[key] {
			if sp.y < 0 || sp.y >= h {
				continue
			}
			if sp.x1 < 0 {
				sp.x1 = 0
			}
			if sp.x2 >= w {
				sp.x2 = w - 1
			}
			if sp.x2 < sp.x1 {
				continue
			}
			visible := clipRangeAgainstCovered(sp.x1, sp.x2, coveredByRow[sp.y])
			if len(visible) == 0 {
				continue
			}
			den := cy - (float64(sp.y) + 0.5)
			if math.Abs(den) < 1e-6 {
				continue
			}
			planeZ := float64(key.height)
			depth := ((planeZ - eyeZ) / den) * focal
			if depth <= 0 {
				continue
			}
			row := sp.y * w * 4
			stepWX := (depth / focal) * sa
			stepWY := -(depth / focal) * ca
			for _, vr := range visible {
				wxSpan := camX + depth*ca - ((cx-(float64(vr.l)+0.5))*depth/focal)*sa
				wySpan := camY + depth*sa + ((cx-(float64(vr.l)+0.5))*depth/focal)*ca
				v := 0
				if sp.y >= 0 && sp.y < len(skyRowV) {
					v = skyRowV[sp.y]
				}
				for x := vr.l; x <= vr.r; x++ {
					i := row + x*4
					if key.sky {
						if skyTexOK && len(skyTex.RGBA) == skyTex.Width*skyTex.Height*4 {
							u := 0
							if x >= 0 && x < len(skyColU) {
								u = skyColU[x]
							}
							ti := (v*skyTex.Width + u) * 4
							pix[i+0] = skyTex.RGBA[ti+0]
							pix[i+1] = skyTex.RGBA[ti+1]
							pix[i+2] = skyTex.RGBA[ti+2]
							pix[i+3] = 255
						} else {
							pix[i+0] = fb.R
							pix[i+1] = fb.G
							pix[i+2] = fb.B
							pix[i+3] = 255
						}
					} else {
						u := int(math.Floor(wxSpan)) & 63
						v := int(math.Floor(wySpan)) & 63
						ti := (v*64 + u) * 4
						pix[i+0] = tex[ti+0]
						pix[i+1] = tex[ti+1]
						pix[i+2] = tex[ti+2]
						pix[i+3] = 255
					}
					wxSpan += stepWX
					wySpan += stepWY
				}
				coveredByRow[sp.y] = addCoveredRange(coveredByRow[sp.y], vr.l, vr.r)
			}
		}
	}
	g.writePixelsTimed(g.mapFloorLayer, pix)
	screen.DrawImage(g.mapFloorLayer, nil)
}

func (g *game) clearRGBABuffer(pix []byte) {
	if len(pix) == 0 {
		return
	}
	clearChunk := func(start, end int) {
		for i := start; i < end; i += 4 {
			pix[i+0] = 0
			pix[i+1] = 0
			pix[i+2] = 0
			pix[i+3] = 0
		}
	}
	clearChunk(0, len(pix))
}

func (g *game) drawDoomBasicTexturedCeilingClipped(screen *ebiten.Image, camX, camY, ca, sa, eyeZ, focal float64, playerSec int, ceilFallback color.RGBA, wallTop []int, depthPix []float64) {
	g.ensureMapFloorLayer()
	pix := g.mapFloorPix
	w := g.viewW
	h := g.viewH
	if w <= 0 || h <= 0 || len(pix) != w*h*4 {
		return
	}
	cx := float64(w) * 0.5
	cy := float64(h) * 0.5
	baseCeilZ := float64(g.m.Sectors[playerSec].CeilingHeight)
	for i := 0; i < len(pix); i += 4 {
		pix[i+0] = 0
		pix[i+1] = 0
		pix[i+2] = 0
		pix[i+3] = 0
	}

	for y := 0; y < h; y++ {
		if float64(y) >= cy {
			break
		}
		rowBase := y * w * 4

		for x := 0; x < w; x++ {
			i := rowBase + x*4
			stopY := int(cy)
			if x >= 0 && x < len(wallTop) && wallTop[x] < stopY {
				stopY = wallTop[x]
			}
			if y >= stopY {
				continue
			}
			wx, wy, depth, sec, ok := g.refinePlaneSampleAtPixel(x, y, cx, cy, camX, camY, ca, sa, eyeZ, focal, baseCeilZ, true)
			if !ok {
				continue
			}
			pi := y*g.viewW + x
			if pi < 0 || pi >= len(depthPix) || depth >= depthPix[pi] {
				continue
			}
			if sec >= 0 && sec < len(g.m.Sectors) {
				name := g.m.Sectors[sec].CeilingPic
				if isSkyFlatName(name) {
					pix[i+0] = ceilFallback.R
					pix[i+1] = ceilFallback.G
					pix[i+2] = ceilFallback.B
					pix[i+3] = 255
				} else if tex, ok := g.flatTextureBlend(name); ok {
					u := int(math.Floor(wx)) & 63
					v := int(math.Floor(wy)) & 63
					p := sampleFlatBlendPacked(tex, u, v)
					pix[i+0] = uint8((p >> pixelRShift) & 0xFF)
					pix[i+1] = uint8((p >> pixelGShift) & 0xFF)
					pix[i+2] = uint8((p >> pixelBShift) & 0xFF)
					pix[i+3] = 255
				} else {
					pix[i+0] = ceilFallback.R
					pix[i+1] = ceilFallback.G
					pix[i+2] = ceilFallback.B
					pix[i+3] = 255
				}
			} else {
				pix[i+0] = ceilFallback.R
				pix[i+1] = ceilFallback.G
				pix[i+2] = ceilFallback.B
				pix[i+3] = 255
			}
		}
	}
	g.writePixelsTimed(g.mapFloorLayer, pix)
	screen.DrawImage(g.mapFloorLayer, nil)
}

func (g *game) drawDoomBasicTexturedFloorClipped(screen *ebiten.Image, camX, camY, ca, sa, eyeZ, focal float64, playerSec int, floorFallback color.RGBA, wallBottom []int, depthPix []float64) {
	g.ensureMapFloorLayer()
	pix := g.mapFloorPix
	w := g.viewW
	h := g.viewH
	if w <= 0 || h <= 0 || len(pix) != w*h*4 {
		return
	}
	cx := float64(w) * 0.5
	cy := float64(h) * 0.5
	baseFloorZ := float64(g.m.Sectors[playerSec].FloorHeight)
	for i := 0; i < len(pix); i += 4 {
		pix[i+0] = 0
		pix[i+1] = 0
		pix[i+2] = 0
		pix[i+3] = 0
	}

	for y := 0; y < h; y++ {
		rowBase := y * w * 4
		if float64(y) <= cy {
			continue
		}
		for x := 0; x < w; x++ {
			i := rowBase + x*4
			startY := int(cy)
			if x >= 0 && x < len(wallBottom) && wallBottom[x]+1 > startY {
				startY = wallBottom[x] + 1
			}
			if y < startY {
				continue
			}
			wx, wy, depth, sec, ok := g.refinePlaneSampleAtPixel(x, y, cx, cy, camX, camY, ca, sa, eyeZ, focal, baseFloorZ, false)
			if !ok {
				continue
			}
			pi := y*g.viewW + x
			if pi < 0 || pi >= len(depthPix) || depth >= depthPix[pi] {
				continue
			}
			if sec >= 0 && sec < len(g.m.Sectors) {
				if tex, ok := g.flatTextureBlend(g.m.Sectors[sec].FloorPic); ok {
					u := int(math.Floor(wx)) & 63
					v := int(math.Floor(wy)) & 63
					p := sampleFlatBlendPacked(tex, u, v)
					pix[i+0] = uint8((p >> pixelRShift) & 0xFF)
					pix[i+1] = uint8((p >> pixelGShift) & 0xFF)
					pix[i+2] = uint8((p >> pixelBShift) & 0xFF)
					pix[i+3] = 255
				} else {
					pix[i+0] = floorFallback.R
					pix[i+1] = floorFallback.G
					pix[i+2] = floorFallback.B
					pix[i+3] = 255
				}
			} else {
				pix[i+0] = floorFallback.R
				pix[i+1] = floorFallback.G
				pix[i+2] = floorFallback.B
				pix[i+3] = 255
			}
		}
	}
	g.writePixelsTimed(g.mapFloorLayer, pix)
	screen.DrawImage(g.mapFloorLayer, nil)
}

func worldPointForPlaneAtPixel(x, y int, cx, cy, camX, camY, ca, sa, eyeZ, focal, planeZ float64) (wx, wy, depth float64, ok bool) {
	den := cy - (float64(y) + 0.5)
	if math.Abs(den) < 1e-6 {
		return 0, 0, 0, false
	}
	depth = ((planeZ - eyeZ) / den) * focal
	if depth <= 0 {
		return 0, 0, 0, false
	}
	s := (cx - (float64(x) + 0.5)) * depth / focal
	wx = camX + depth*ca - s*sa
	wy = camY + depth*sa + s*ca
	return wx, wy, depth, true
}

func (g *game) refinePlaneSampleAtPixel(x, y int, cx, cy, camX, camY, ca, sa, eyeZ, focal, initialZ float64, ceiling bool) (wx, wy, depth float64, sec int, ok bool) {
	planeZ := initialZ
	lastSec := -1
	for i := 0; i < 4; i++ {
		rwx, rwy, rd, rok := worldPointForPlaneAtPixel(x, y, cx, cy, camX, camY, ca, sa, eyeZ, focal, planeZ)
		if !rok {
			return 0, 0, 0, -1, false
		}
		rsec := g.sectorAt(int64(rwx*fracUnit), int64(rwy*fracUnit))
		if rsec < 0 || rsec >= len(g.m.Sectors) {
			return rwx, rwy, rd, rsec, true
		}
		nextZ := float64(g.m.Sectors[rsec].FloorHeight)
		if ceiling {
			nextZ = float64(g.m.Sectors[rsec].CeilingHeight)
		}
		wx, wy, depth, sec = rwx, rwy, rd, rsec
		if rsec == lastSec || math.Abs(nextZ-planeZ) < 0.001 {
			return wx, wy, depth, sec, true
		}
		lastSec = rsec
		planeZ = nextZ
	}
	return wx, wy, depth, sec, sec >= 0
}

func isFinite(v float64) bool {
	return !math.IsNaN(v) && !math.IsInf(v, 0)
}

func (g *game) logWallCull(segIdx int, reason string, z1, z2, x1, x2 float64) {
	if !g.opts.Debug || g.cullLogBudget <= 0 {
		return
	}
	g.cullLogBudget--
	fmt.Printf("wall-cull seg=%d reason=%s z1=%.4f z2=%.4f x1=%.2f x2=%.2f\n", segIdx, reason, z1, z2, x1, x2)
}

func clipSegmentToNear(f1, s1, f2, s2, near float64) (float64, float64, float64, float64, bool) {
	return scene.ClipSegmentToNear(f1, s1, f2, s2, near)
}

func clipSegmentToNearWithAttr(f1, s1, a1, f2, s2, a2, near float64) (float64, float64, float64, float64, float64, float64, bool) {
	return scene.ClipSegmentToNearWithAttr(f1, s1, a1, f2, s2, a2, near)
}

type solidSpan = scene.ScreenSpan

func solidFullyCovered(spans []solidSpan, l, r int) bool {
	return scene.SpanFullyCovered(spans, l, r)
}

func addSolidSpan(spans []solidSpan, l, r int) []solidSpan {
	return scene.AddSpanInPlace(spans, l, r)
}

func clipRangeAgainstSolidSpans(l, r int, covered []solidSpan, out []solidSpan) []solidSpan {
	return scene.ClipRangeAgainstSpans(l, r, covered, out[:0])
}

func (g *game) drawBasicWallColumnRange(screen *ebiten.Image, depthPix []float64, wallTop, wallBottom []int, sx1, sx2, f1, f2, zTop, zBot, eyeZ, focal float64, base color.RGBA) {
	if zTop <= zBot {
		return
	}
	if math.Abs(sx2-sx1) < 0.001 {
		return
	}
	minX := int(math.Max(0, math.Floor(math.Min(sx1, sx2))))
	maxX := int(math.Min(float64(g.viewW-1), math.Ceil(math.Max(sx1, sx2))))
	if minX > maxX {
		return
	}
	for x := minX; x <= maxX; x++ {
		t := (float64(x) - sx1) / (sx2 - sx1)
		if t < 0 {
			t = 0
		}
		if t > 1 {
			t = 1
		}
		// Perspective-correct depth interpolation across screen columns.
		// In projected space, 1/z is linear with x (not z itself).
		invF1 := 1.0 / f1
		invF2 := 1.0 / f2
		invF := invF1 + (invF2-invF1)*t
		if invF <= 0 {
			continue
		}
		f := 1.0 / invF
		if f <= 0 {
			continue
		}
		yt := float64(g.viewH)/2 - ((zTop-eyeZ)/f)*focal
		yb := float64(g.viewH)/2 - ((zBot-eyeZ)/f)*focal
		if yb <= yt {
			continue
		}
		y0 := int(math.Max(0, math.Ceil(yt)))
		y1 := int(math.Min(float64(g.viewH-1), math.Floor(yb)))
		if y0 > y1 {
			continue
		}
		clr := shadeByDistance(base, f)
		runStart := -1
		for y := y0; y <= y1; y++ {
			pi := y*g.viewW + x
			if f < depthPix[pi] {
				depthPix[pi] = f
				if x >= 0 && x < len(wallTop) {
					if y < wallTop[x] {
						wallTop[x] = y
					}
					if y > wallBottom[x] {
						wallBottom[x] = y
					}
				}
				if runStart < 0 {
					runStart = y
				}
			} else if runStart >= 0 {
				ebitenutil.DrawRect(screen, float64(x), float64(runStart), 1, float64(y-runStart), clr)
				runStart = -1
			}
		}
		if runStart >= 0 {
			ebitenutil.DrawRect(screen, float64(x), float64(runStart), 1, float64(y1-runStart+1), clr)
		}
	}
}

func (g *game) basicPlaneColors() (color.RGBA, color.RGBA) {
	sec := g.sectorAt(g.p.x, g.p.y)
	if sec < 0 || sec >= len(g.m.Sectors) {
		return color.RGBA{R: 24, G: 24, B: 30, A: 255}, color.RGBA{R: 28, G: 22, B: 18, A: 255}
	}
	s := g.m.Sectors[sec]
	ceilBase := uint8(36 + (int(s.CeilingHeight) & 31))
	floorBase := uint8(28 + (int(s.FloorHeight) & 31))
	return color.RGBA{R: ceilBase, G: ceilBase, B: ceilBase + 8, A: 255}, color.RGBA{R: floorBase + 10, G: floorBase + 4, B: floorBase, A: 255}
}

func shadeByDistance(c color.RGBA, dist float64) color.RGBA {
	n := dist / 1200.0
	if n < 0 {
		n = 0
	}
	if n > 1 {
		n = 1
	}
	f := 1.0 - 0.72*n
	return color.RGBA{
		R: uint8(float64(c.R) * f),
		G: uint8(float64(c.G) * f),
		B: uint8(float64(c.B) * f),
		A: c.A,
	}
}

func (g *game) appendProjectileCutoutItems(camX, camY, camAng, focal, near float64) {
	viewW := g.viewW
	viewH := g.viewH
	if len(g.wallPix32) != viewW*viewH {
		return
	}
	if len(g.projectiles) == 0 && len(g.projectileImpacts) == 0 {
		return
	}
	ca := math.Cos(camAng)
	sa := math.Sin(camAng)
	eyeZ := g.playerEyeZ()
	alpha := g.renderAlpha
	for pi, p := range g.projectiles {
		rx, ry, rz := g.projectileRenderPosFixed(p, alpha)
		px := float64(rx)/fracUnit - camX
		py := float64(ry)/fracUnit - camY
		f := px*ca + py*sa
		s := -px*sa + py*ca
		if f <= near {
			continue
		}
		sec := g.sectorAt(rx, ry)
		clipTop := 0
		clipBottom := viewH - 1
		clipRadius := p.radius
		if clipRadius <= 0 {
			clipRadius = 8 * fracUnit
		}
		var clipOK bool
		clipTop, clipBottom, clipOK = g.spriteFootprintClipYBounds(rx, ry, clipRadius, viewH, eyeZ, f, focal)
		if !clipOK {
			continue
		}
		scale := focal / f
		if scale <= 0 {
			continue
		}
		sx := float64(viewW)/2 - (s/f)*focal
		yb := float64(viewH)/2 - ((float64(rz)/fracUnit-eyeZ)/f)*focal
		ref, ok := g.projectileSpriteRef(p.kind, p.frame)
		if !ok || ref == nil || ref.tex == nil || ref.tex.Height <= 0 || ref.tex.Width <= 0 {
			continue
		}
		h := float64(ref.tex.Height) * scale
		w := float64(ref.tex.Width) * scale
		if h <= 0 || w <= 0 {
			continue
		}
		xPad := w*0.5 + 4
		yPad := h + 4
		if sx+xPad < 0 || sx-xPad > float64(viewW) || yb+yPad < 0 || yb-yPad > float64(viewH) {
			continue
		}
		lightMul := uint32(256)
		if sec >= 0 && sec < len(g.m.Sectors) {
			lightMul = g.sectorLightMulCached(sec)
		}
		depthQ := encodeDepthQ(f)
		scale = h / float64(ref.tex.Height)
		dstX := sx - float64(ref.tex.OffsetX)*scale
		dstY := yb - float64(ref.tex.OffsetY)*scale
		x0, x1, y0, y1, ok := scene.ClampedSpriteBounds(dstX, dstY, w, h, clipTop, clipBottom, viewW, viewH)
		if !ok {
			continue
		}
		opaqueRectStart, opaqueRectCount := g.appendProjectedOpaqueRects(ref.opaque.rects, ref.tex.Width, false, dstX, dstY, scale, clipTop, clipBottom, viewW, viewH)
		if opaqueRectCount > 0 &&
			g.projectedOpaqueRectsFullyOccluded(g.projectedOpaqueRectScratch[opaqueRectStart:opaqueRectStart+opaqueRectCount], depthQ) {
			continue
		}
		if g.spriteWallClipQuadFullyOccluded(x0, x1, y0, y1, depthQ) {
			continue
		}
		g.billboardQueueScratch = append(g.billboardQueueScratch, cutoutItem{
			dist:            f,
			depthQ:          depthQ,
			kind:            billboardQueueProjectiles,
			x0:              x0,
			x1:              x1,
			y0:              y0,
			y1:              y1,
			shadeMul:        g.cachedProjectileShadeMul(pi, ref.fullBright, lightMul, f, near),
			tex:             ref.tex,
			clipTop:         clipTop,
			clipBottom:      clipBottom,
			dstX:            dstX,
			dstY:            dstY,
			scale:           scale,
			opaque:          ref.opaque,
			hasOpaque:       ref.hasOpaque,
			opaqueRectStart: opaqueRectStart,
			opaqueRectCount: opaqueRectCount,
			boundsOK:        true,
		})
	}
	for _, cube := range g.bossSpawnCubes {
		px := float64(cube.x)/fracUnit - camX
		py := float64(cube.y)/fracUnit - camY
		f := px*ca + py*sa
		s := -px*sa + py*ca
		if f <= near {
			continue
		}
		clipTop := 0
		clipBottom := viewH - 1
		clipRadius := int64(6 * fracUnit)
		var clipOK bool
		clipTop, clipBottom, clipOK = g.spriteFootprintClipYBounds(cube.x, cube.y, clipRadius, viewH, eyeZ, f, focal)
		if !clipOK {
			continue
		}
		scale := focal / f
		if scale <= 0 {
			continue
		}
		sx := float64(viewW)/2 - (s/f)*focal
		yb := float64(viewH)/2 - ((float64(cube.z)/fracUnit-eyeZ)/f)*focal
		ref, ok := g.bossCubeSpriteRef(g.worldTic)
		if !ok || ref == nil || ref.tex == nil || ref.tex.Height <= 0 || ref.tex.Width <= 0 {
			continue
		}
		h := float64(ref.tex.Height) * scale
		w := float64(ref.tex.Width) * scale
		if h <= 0 || w <= 0 {
			continue
		}
		xPad := w*0.5 + 4
		yPad := h + 4
		if sx+xPad < 0 || sx-xPad > float64(viewW) || yb+yPad < 0 || yb-yPad > float64(viewH) {
			continue
		}
		depthQ := encodeDepthQ(f)
		scale = h / float64(ref.tex.Height)
		dstX := sx - float64(ref.tex.OffsetX)*scale
		dstY := yb - float64(ref.tex.OffsetY)*scale
		x0, x1, y0, y1, ok := scene.ClampedSpriteBounds(dstX, dstY, w, h, clipTop, clipBottom, viewW, viewH)
		if !ok {
			continue
		}
		opaqueRectStart, opaqueRectCount := g.appendProjectedOpaqueRects(ref.opaque.rects, ref.tex.Width, false, dstX, dstY, scale, clipTop, clipBottom, viewW, viewH)
		if opaqueRectCount > 0 &&
			g.projectedOpaqueRectsFullyOccluded(g.projectedOpaqueRectScratch[opaqueRectStart:opaqueRectStart+opaqueRectCount], depthQ) {
			continue
		}
		if g.spriteWallClipQuadFullyOccluded(x0, x1, y0, y1, depthQ) {
			continue
		}
		g.billboardQueueScratch = append(g.billboardQueueScratch, cutoutItem{
			dist:            f,
			depthQ:          depthQ,
			kind:            billboardQueueProjectiles,
			x0:              x0,
			x1:              x1,
			y0:              y0,
			y1:              y1,
			shadeMul:        256,
			tex:             ref.tex,
			clipTop:         clipTop,
			clipBottom:      clipBottom,
			dstX:            dstX,
			dstY:            dstY,
			scale:           scale,
			opaque:          ref.opaque,
			hasOpaque:       ref.hasOpaque,
			opaqueRectStart: opaqueRectStart,
			opaqueRectCount: opaqueRectCount,
			boundsOK:        true,
		})
	}
	for _, fire := range g.bossSpawnFires {
		px := float64(fire.x)/fracUnit - camX
		py := float64(fire.y)/fracUnit - camY
		f := px*ca + py*sa
		s := -px*sa + py*ca
		if f <= near {
			continue
		}
		clipTop := 0
		clipBottom := viewH - 1
		clipRadius := int64(20 * fracUnit)
		var clipOK bool
		clipTop, clipBottom, clipOK = g.spriteFootprintClipYBounds(fire.x, fire.y, clipRadius, viewH, eyeZ, f, focal)
		if !clipOK {
			continue
		}
		scale := focal / f
		if scale <= 0 {
			continue
		}
		sx := float64(viewW)/2 - (s/f)*focal
		yb := float64(viewH)/2 - ((float64(fire.z)/fracUnit-eyeZ)/f)*focal
		ref, ok := g.bossSpawnFireSpriteRef(32 - fire.tics)
		if !ok || ref == nil || ref.tex == nil || ref.tex.Height <= 0 || ref.tex.Width <= 0 {
			continue
		}
		h := float64(ref.tex.Height) * scale
		w := float64(ref.tex.Width) * scale
		if h <= 0 || w <= 0 {
			continue
		}
		xPad := w*0.5 + 4
		yPad := h + 4
		if sx+xPad < 0 || sx-xPad > float64(viewW) || yb+yPad < 0 || yb-yPad > float64(viewH) {
			continue
		}
		depthQ := encodeDepthQ(f)
		scale = h / float64(ref.tex.Height)
		dstX := sx - float64(ref.tex.OffsetX)*scale
		dstY := yb - float64(ref.tex.OffsetY)*scale
		x0, x1, y0, y1, ok := scene.ClampedSpriteBounds(dstX, dstY, w, h, clipTop, clipBottom, viewW, viewH)
		if !ok {
			continue
		}
		opaqueRectStart, opaqueRectCount := g.appendProjectedOpaqueRects(ref.opaque.rects, ref.tex.Width, false, dstX, dstY, scale, clipTop, clipBottom, viewW, viewH)
		if opaqueRectCount > 0 &&
			g.projectedOpaqueRectsFullyOccluded(g.projectedOpaqueRectScratch[opaqueRectStart:opaqueRectStart+opaqueRectCount], depthQ) {
			continue
		}
		if g.spriteWallClipQuadFullyOccluded(x0, x1, y0, y1, depthQ) {
			continue
		}
		g.billboardQueueScratch = append(g.billboardQueueScratch, cutoutItem{
			dist:            f,
			depthQ:          depthQ,
			kind:            billboardQueueProjectiles,
			x0:              x0,
			x1:              x1,
			y0:              y0,
			y1:              y1,
			shadeMul:        256,
			tex:             ref.tex,
			clipTop:         clipTop,
			clipBottom:      clipBottom,
			dstX:            dstX,
			dstY:            dstY,
			scale:           scale,
			opaque:          ref.opaque,
			hasOpaque:       ref.hasOpaque,
			opaqueRectStart: opaqueRectStart,
			opaqueRectCount: opaqueRectCount,
			boundsOK:        true,
		})
	}
	for fi, fx := range g.projectileImpacts {
		px := float64(fx.x)/fracUnit - camX
		py := float64(fx.y)/fracUnit - camY
		f := px*ca + py*sa
		s := -px*sa + py*ca
		if f <= near {
			continue
		}
		sec := g.sectorAt(fx.x, fx.y)
		clipTop := 0
		clipBottom := viewH - 1
		clipRadius := int64(8 * fracUnit)
		var clipOK bool
		clipTop, clipBottom, clipOK = g.spriteFootprintClipYBounds(fx.x, fx.y, clipRadius, viewH, eyeZ, f, focal)
		if !clipOK {
			continue
		}
		scale := focal / f
		if scale <= 0 {
			continue
		}
		sx := float64(viewW)/2 - (s/f)*focal
		yb := float64(viewH)/2 - ((float64(fx.z)/fracUnit-eyeZ)/f)*focal
		ref, ok := g.projectileImpactSpriteRef(fx.kind, fx.phase)
		if !ok || ref == nil || ref.tex == nil || ref.tex.Height <= 0 || ref.tex.Width <= 0 {
			continue
		}
		h := float64(ref.tex.Height) * scale
		w := float64(ref.tex.Width) * scale
		if h <= 0 || w <= 0 {
			continue
		}
		xPad := w*0.5 + 4
		yPad := h + 4
		if sx+xPad < 0 || sx-xPad > float64(viewW) || yb+yPad < 0 || yb-yPad > float64(viewH) {
			continue
		}
		lightMul := uint32(256)
		if sec >= 0 && sec < len(g.m.Sectors) {
			lightMul = g.sectorLightMulCached(sec)
		}
		depthQ := encodeDepthQ(f)
		scale = h / float64(ref.tex.Height)
		dstX := sx - float64(ref.tex.OffsetX)*scale
		dstY := yb - float64(ref.tex.OffsetY)*scale
		x0, x1, y0, y1, ok := scene.ClampedSpriteBounds(dstX, dstY, w, h, clipTop, clipBottom, viewW, viewH)
		if !ok {
			continue
		}
		opaqueRectStart, opaqueRectCount := g.appendProjectedOpaqueRects(ref.opaque.rects, ref.tex.Width, false, dstX, dstY, scale, clipTop, clipBottom, viewW, viewH)
		if opaqueRectCount > 0 &&
			g.projectedOpaqueRectsFullyOccluded(g.projectedOpaqueRectScratch[opaqueRectStart:opaqueRectStart+opaqueRectCount], depthQ) {
			continue
		}
		if g.spriteWallClipQuadFullyOccluded(x0, x1, y0, y1, depthQ) {
			continue
		}
		g.billboardQueueScratch = append(g.billboardQueueScratch, cutoutItem{
			dist:            f,
			depthQ:          depthQ,
			kind:            billboardQueueProjectiles,
			x0:              x0,
			x1:              x1,
			y0:              y0,
			y1:              y1,
			shadeMul:        g.cachedImpactShadeMul(fi, ref.fullBright, lightMul, f, near),
			tex:             ref.tex,
			clipTop:         clipTop,
			clipBottom:      clipBottom,
			dstX:            dstX,
			dstY:            dstY,
			scale:           scale,
			opaque:          ref.opaque,
			hasOpaque:       ref.hasOpaque,
			opaqueRectStart: opaqueRectStart,
			opaqueRectCount: opaqueRectCount,
			boundsOK:        true,
		})
	}
}

func (g *game) appendHitscanPuffCutoutItems(camX, camY, camAng, focal, near float64) {
	viewW := g.viewW
	viewH := g.viewH
	if len(g.wallPix32) != viewW*viewH || len(g.hitscanPuffs) == 0 {
		return
	}
	ca := math.Cos(camAng)
	sa := math.Sin(camAng)
	eyeZ := g.playerEyeZ()
	for _, p := range g.hitscanPuffs {
		if p.kind == hitscanFxTeleport || p.hidden {
			continue
		}
		px := float64(p.x)/fracUnit - camX
		py := float64(p.y)/fracUnit - camY
		f := px*ca + py*sa
		s := -px*sa + py*ca
		if f <= near {
			continue
		}
		clipTop := 0
		clipBottom := viewH - 1
		clipRadius := int64(8 * fracUnit)
		clipTop, clipBottom, ok := g.spriteFootprintClipYBounds(p.x, p.y, clipRadius, viewH, eyeZ, f, focal)
		if !ok {
			continue
		}
		scale := focal / f
		if scale <= 0 {
			continue
		}
		sx := float64(viewW)/2 - (s/f)*focal
		pz := float64(p.z) / fracUnit
		sy := float64(viewH)/2 - ((pz-eyeZ)/f)*focal
		ref, ok := g.hitscanEffectSpriteRef(p)
		if !ok || ref == nil || ref.tex == nil || ref.tex.Width <= 0 || ref.tex.Height <= 0 {
			continue
		}
		h := float64(ref.tex.Height) * scale
		w := float64(ref.tex.Width) * scale
		if h <= 0 || w <= 0 {
			continue
		}
		xPad := w*0.5 + 4
		yPad := h + 4
		if sx+xPad < 0 || sx-xPad > float64(viewW) || sy+yPad < 0 || sy-yPad > float64(viewH) {
			continue
		}
		depthQ := encodeDepthQ(f)
		dstX := sx - float64(ref.tex.OffsetX)*scale
		dstY := sy - float64(ref.tex.OffsetY)*scale
		x0, x1, y0, y1, ok := scene.ClampedSpriteBounds(dstX, dstY, w, h, clipTop, clipBottom, viewW, viewH)
		if !ok {
			continue
		}
		opaqueRectStart, opaqueRectCount := g.appendProjectedOpaqueRects(ref.opaque.rects, ref.tex.Width, false, dstX, dstY, scale, clipTop, clipBottom, viewW, viewH)
		if opaqueRectCount > 0 &&
			g.projectedOpaqueRectsFullyOccluded(g.projectedOpaqueRectScratch[opaqueRectStart:opaqueRectStart+opaqueRectCount], depthQ) {
			continue
		}
		if g.spriteWallClipQuadFullyOccluded(x0, x1, y0, y1, depthQ) {
			continue
		}
		g.billboardQueueScratch = append(g.billboardQueueScratch, cutoutItem{
			dist:            f,
			depthQ:          depthQ,
			kind:            billboardQueuePuffs,
			x0:              x0,
			x1:              x1,
			y0:              y0,
			y1:              y1,
			shadeMul:        256,
			tex:             ref.tex,
			clipTop:         clipTop,
			clipBottom:      clipBottom,
			dstX:            dstX,
			dstY:            dstY,
			scale:           scale,
			opaque:          ref.opaque,
			hasOpaque:       ref.hasOpaque,
			opaqueRectStart: opaqueRectStart,
			opaqueRectCount: opaqueRectCount,
			boundsOK:        true,
		})
	}
}

func (g *game) projectileSpriteRef(kind projectileKind, frame int) (*spriteRenderRef, bool) {
	name := g.projectileSpriteNameForFrame(kind, frame)
	if name == "" {
		return nil, false
	}
	return g.spriteRenderRef(name)
}

func (g *game) projectileImpactSpriteRef(kind projectileKind, phase int) (*spriteRenderRef, bool) {
	name := g.projectileImpactSpriteNameForPhase(kind, phase)
	if name == "" {
		return nil, false
	}
	return g.spriteRenderRef(name)
}

func (g *game) projectileImpactSpriteNameForPhase(kind projectileKind, phase int) string {
	if phase < 0 {
		phase = 0
	}
	prefix := "BAL1"
	frame := byte('C')
	switch kind {
	case projectileBFGBall:
		prefix = "BFE1"
		frame = byte('A' + min(phase, 5))
	case projectileRocket:
		prefix = "MISL"
		frame = byte('B' + min(phase, 2))
	case projectileBaronBall:
		prefix = "BAL7"
		frame = byte('C' + min(phase, 2))
	case projectilePlasmaBall:
		prefix = "BAL2"
		frame = byte('C' + min(phase, 2))
	case projectilePlayerPlasma:
		prefix = "PLSE"
		frame = byte('A' + min(phase, 4))
	case projectileTracer:
		prefix = "FBXP"
		frame = byte('A' + min(phase, 2))
	case projectileFatShot:
		prefix = "MISL"
		frame = byte('B' + min(phase, 2))
	default:
		frame = byte('C' + min(phase, 2))
	}
	name0 := spriteFrameName(prefix, byte(frame), '0')
	if g.spritePatchExists(name0) {
		return name0
	}
	if name, _, ok := g.monsterSpriteRotFrame(prefix, byte(frame), 1); ok {
		return name
	}
	// Fallback to flight sprite if impact frame is unavailable in the bank.
	return g.projectileSpriteName(kind, 0)
}

func (g *game) projectileSpriteNameForFrame(kind projectileKind, frame int) string {
	pickPrefixFrame := func(prefix string, frameLetters []byte, frame int) string {
		if len(frameLetters) == 0 {
			return ""
		}
		for i := 0; i < len(frameLetters); i++ {
			fl := frameLetters[(frame+i)%len(frameLetters)]
			// Some assets use non-rotating frame notation (e.g. BAL1A0).
			name0 := spriteFrameName(prefix, fl, '0')
			if g.spritePatchExists(name0) {
				return name0
			}
			// Standard Doom rotating/projectile frames (e.g. BAL1A1, paired lumps, etc).
			if name, _, ok := g.monsterSpriteRotFrame(prefix, fl, 1); ok {
				return name
			}
		}
		return ""
	}
	frame2 := frame & 1
	switch kind {
	case projectileBFGBall:
		return pickPrefixFrame("BFS1", []byte{'A', 'B'}, frame2)
	case projectileRocket:
		return pickPrefixFrame("MISL", []byte{'A'}, 0)
	case projectilePlayerPlasma:
		return pickPrefixFrame("PLSS", []byte{'A', 'B'}, frame2)
	case projectileBaronBall:
		return pickPrefixFrame("BAL7", []byte{'A', 'B'}, frame2)
	case projectileTracer:
		return pickPrefixFrame("FATB", []byte{'A', 'B'}, frame2)
	case projectileFatShot:
		return pickPrefixFrame("MANF", []byte{'A', 'B'}, frame2)
	case projectilePlasmaBall:
		return pickPrefixFrame("BAL2", []byte{'A', 'B'}, frame2)
	default:
		return pickPrefixFrame("BAL1", []byte{'A', 'B'}, frame2)
	}
}

func (g *game) projectileImpactSpriteName(kind projectileKind, elapsed int) string {
	if elapsed < 0 {
		elapsed = 0
	}
	phase := 0
	for {
		next := projectileImpactPhaseTics(kind, phase)
		if next <= 0 || elapsed < next {
			break
		}
		elapsed -= next
		phase++
	}
	return g.projectileImpactSpriteNameForPhase(kind, phase)
}

func (g *game) projectileSpriteName(kind projectileKind, tic int) string {
	if tic < 0 {
		tic = 0
	}
	return g.projectileSpriteNameForFrame(kind, (tic/4)&1)
}

func (g *game) spawnHitscanPuff(x, y, z int64) {
	const maxPuffs = 64
	if len(g.hitscanPuffs) >= maxPuffs {
		copy(g.hitscanPuffs, g.hitscanPuffs[1:])
		g.hitscanPuffs = g.hitscanPuffs[:maxPuffs-1]
	}
	z += int64((doomrand.PRandom() - doomrand.PRandom()) << 10)
	lastLook := doomrand.PRandom() & 3
	tics := 4 - (doomrand.PRandom() & 3)
	if want := runtimeDebugEnv("GD_DEBUG_HITSCAN_FX"); want != "" {
		var wantTic int
		if _, err := fmt.Sscanf(want, "%d", &wantTic); err == nil {
			if g.demoTick-1 == wantTic || g.worldTic == wantTic {
				rnd, prnd := doomrand.State()
				fmt.Printf("hitscan-fx-debug tic=%d world=%d kind=puff pos=(%d,%d,%d) lastlook=%d tics=%d rnd=%d prnd=%d\n",
					g.demoTick-1, g.worldTic, x, y, z, lastLook, tics, rnd, prnd)
			}
		}
	}
	if tics < 1 {
		tics = 1
	}
	floorz, ceilz, ok := g.subsectorFloorCeilAt(x, y)
	if !ok && g != nil && g.m != nil {
		floorz = g.thingFloorZ(x, y)
		if sec := g.sectorAt(x, y); sec >= 0 && sec < len(g.sectorCeil) {
			ceilz = g.sectorCeil[sec]
		}
	}
	g.hitscanPuffs = append(g.hitscanPuffs, hitscanPuff{
		x:        x,
		y:        y,
		z:        z,
		momz:     fracUnit,
		floorz:   floorz,
		ceilz:    ceilz,
		lastLook: lastLook,
		tics:     tics,
		totalTic: tics,
		state:    93,
		kind:     hitscanFxPuff,
		order:    g.allocThinkerOrder(),
	})
}

func (g *game) spawnHitscanBlood(x, y, z int64, damage int) {
	const maxPuffs = 64
	if len(g.hitscanPuffs) >= maxPuffs {
		copy(g.hitscanPuffs, g.hitscanPuffs[1:])
		g.hitscanPuffs = g.hitscanPuffs[:maxPuffs-1]
	}
	state := 90
	z += int64((doomrand.PRandom() - doomrand.PRandom()) << 10)
	lastLook := doomrand.PRandom() & 3
	tics := 8 - (doomrand.PRandom() & 3)
	if want := runtimeDebugEnv("GD_DEBUG_HITSCAN_FX"); want != "" {
		var wantTic int
		if _, err := fmt.Sscanf(want, "%d", &wantTic); err == nil {
			if g.demoTick-1 == wantTic || g.worldTic == wantTic {
				rnd, prnd := doomrand.State()
				fmt.Printf("hitscan-fx-debug tic=%d world=%d kind=blood pos=(%d,%d,%d) damage=%d lastlook=%d tics=%d rnd=%d prnd=%d\n",
					g.demoTick-1, g.worldTic, x, y, z, damage, lastLook, tics, rnd, prnd)
			}
		}
	}
	if tics < 1 {
		tics = 1
	}
	if damage <= 12 && damage >= 9 {
		state = 91
		tics = 8
	} else if damage < 9 {
		state = 92
		tics = 8
	}
	floorz, ceilz, ok := g.subsectorFloorCeilAt(x, y)
	if !ok && g != nil && g.m != nil {
		floorz = g.thingFloorZ(x, y)
		if sec := g.sectorAt(x, y); sec >= 0 && sec < len(g.sectorCeil) {
			ceilz = g.sectorCeil[sec]
		}
	}
	g.hitscanPuffs = append(g.hitscanPuffs, hitscanPuff{
		x:        x,
		y:        y,
		z:        z,
		momz:     2 * fracUnit,
		floorz:   floorz,
		ceilz:    ceilz,
		lastLook: lastLook,
		tics:     tics,
		totalTic: tics,
		state:    state,
		kind:     hitscanFxBlood,
		order:    g.allocThinkerOrder(),
	})
}

func (g *game) spawnTracerSmokeTrail(x, y, z, momx, momy int64) {
	jitterZ := z + int64((doomrand.PRandom()-doomrand.PRandom())<<10)
	g.spawnHitscanPuff(x, y, jitterZ)
	const maxPuffs = 64
	if len(g.hitscanPuffs) >= maxPuffs {
		copy(g.hitscanPuffs, g.hitscanPuffs[1:])
		g.hitscanPuffs = g.hitscanPuffs[:maxPuffs-1]
	}
	lastLook := doomrand.PRandom() & 3
	tics := 4 - (doomrand.PRandom() & 3)
	if tics < 1 {
		tics = 1
	}
	g.hitscanPuffs = append(g.hitscanPuffs, hitscanPuff{
		x:        x - momx,
		y:        y - momy,
		z:        z,
		momz:     fracUnit,
		lastLook: lastLook,
		tics:     tics,
		totalTic: tics + 16,
		state:    0,
		kind:     hitscanFxSmoke,
		order:    g.allocThinkerOrder(),
	})
}

func (g *game) spawnTeleportFog(x, y, z int64) {
	const maxPuffs = 64
	const teleportFogFrameTics = 6
	if len(g.hitscanPuffs) >= maxPuffs {
		copy(g.hitscanPuffs, g.hitscanPuffs[1:])
		g.hitscanPuffs = g.hitscanPuffs[:maxPuffs-1]
	}
	lastLook := doomrand.PRandom() & 3
	floorz, ceilz, ok := g.subsectorFloorCeilAt(x, y)
	if !ok && g != nil && g.m != nil {
		floorz = g.thingFloorZ(x, y)
		if sec := g.sectorAt(x, y); sec >= 0 && sec < len(g.sectorCeil) {
			ceilz = g.sectorCeil[sec]
		}
	}
	g.hitscanPuffs = append(g.hitscanPuffs, hitscanPuff{
		x:        x,
		y:        y,
		z:        z,
		floorz:   floorz,
		ceilz:    ceilz,
		lastLook: lastLook,
		tics:     teleportFogFrameTics,
		totalTic: teleportFogFrameTics,
		state:    130,
		kind:     hitscanFxTeleport,
		order:    g.allocThinkerOrder(),
	})
}

func (g *game) hitscanEffectSprite(p hitscanPuff) (*WallTexture, bool) {
	ref, ok := g.hitscanEffectSpriteRef(p)
	if !ok || ref == nil {
		return nil, false
	}
	return ref.tex, ref.tex != nil
}

func (g *game) hitscanEffectSpriteRef(p hitscanPuff) (*spriteRenderRef, bool) {
	find := func(names ...string) (*spriteRenderRef, bool) {
		for _, name := range names {
			if ref, ok := g.spriteRenderRef(name); ok {
				return ref, true
			}
		}
		return nil, false
	}
	if p.kind == hitscanFxBlood {
		switch {
		case p.tics >= 7:
			return find("BLUDA0", "BLUDA1")
		case p.tics >= 5:
			return find("BLUDB0", "BLUDB1")
		default:
			return find("BLUDC0", "BLUDC1")
		}
	}
	if p.kind == hitscanFxSmoke {
		switch p.state {
		case 0:
			return find("PUFFB0", "PUFFB1")
		case 1:
			return find("PUFFC0", "PUFFC1")
		case 2:
			return find("PUFFB0", "PUFFB1")
		case 3:
			return find("PUFFC0", "PUFFC1")
		default:
			return find("PUFFD0", "PUFFD1")
		}
	}
	if p.kind == hitscanFxTeleport {
		frame := 'A'
		switch p.state {
		case 130:
			frame = 'A'
		case 131:
			frame = 'B'
		case 132:
			frame = 'A'
		case 133:
			frame = 'B'
		case 134:
			frame = 'C'
		case 135:
			frame = 'D'
		case 136:
			frame = 'E'
		case 137:
			frame = 'F'
		case 138:
			frame = 'G'
		case 139:
			frame = 'H'
		case 140:
			frame = 'I'
		case 141:
			frame = 'J'
		}
		return find(spriteFrameName("TFOG", byte(frame), '0'))
	}
	switch {
	case p.tics >= 6:
		return find("PUFFA0", "PUFFA1")
	case p.tics >= 4:
		return find("PUFFB0", "PUFFB1")
	case p.tics >= 2:
		return find("PUFFC0", "PUFFC1")
	default:
		return find("PUFFD0", "PUFFD1")
	}
}

func (g *game) tickHitscanPuffs() {
	if len(g.hitscanPuffs) == 0 {
		return
	}
	keep := g.hitscanPuffs[:0]
	for _, p := range g.hitscanPuffs {
		if !g.tickHitscanPuff(&p) {
			continue
		}
		keep = append(keep, p)
	}
	g.hitscanPuffs = keep
}

func (g *game) tickHitscanPuffByOrder(order int64) {
	if g == nil || len(g.hitscanPuffs) == 0 {
		return
	}
	for i := range g.hitscanPuffs {
		if g.hitscanPuffs[i].order != order {
			continue
		}
		p := g.hitscanPuffs[i]
		if g.tickHitscanPuff(&p) {
			g.hitscanPuffs[i] = p
		} else {
			g.hitscanPuffs = append(g.hitscanPuffs[:i], g.hitscanPuffs[i+1:]...)
		}
		return
	}
}

func (g *game) tickHitscanPuff(p *hitscanPuff) bool {
	if p == nil {
		return false
	}
	p.z += p.momz
	if p.z <= p.floorz {
		if p.momz < 0 {
			p.momz = 0
		}
		p.z = p.floorz
	} else if p.kind == hitscanFxBlood {
		if p.momz == 0 {
			p.momz = -2 * fracUnit
		} else {
			p.momz -= fracUnit
		}
	}
	const hitscanEffectHeight = 16 * fracUnit
	if p.z+hitscanEffectHeight > p.ceilz {
		if p.momz > 0 {
			p.momz = 0
		}
		p.z = p.ceilz - hitscanEffectHeight
	}
	p.tics--
	if p.kind == hitscanFxPuff && p.tics <= 0 {
		switch p.state {
		case 93:
			p.state, p.tics = 94, 4
		case 94:
			p.state, p.tics = 95, 4
		case 95:
			p.state, p.tics = 96, 4
		}
	} else if p.kind == hitscanFxBlood && p.tics <= 0 {
		switch p.state {
		case 90:
			p.state, p.tics = 91, 8
		case 91:
			p.state, p.tics = 92, 8
		}
	} else if p.kind == hitscanFxSmoke && p.tics <= 0 {
		switch p.state {
		case 0:
			p.state, p.tics = 1, 4
		case 1:
			p.state, p.tics = 2, 4
		case 2:
			p.state, p.tics = 3, 4
		case 3:
			p.state, p.tics = 4, 4
		}
	} else if p.kind == hitscanFxTeleport && p.tics <= 0 {
		if p.state >= 130 && p.state < 141 {
			p.state++
			p.tics = 6
		}
	}
	return p.tics > 0
}

func (g *game) drawHitscanPuffsToBuffer(camX, camY, camAng, focal, near float64) {
	wallPix := g.wallPix32
	viewW := g.viewW
	viewH := g.viewH
	if len(wallPix) != viewW*viewH || len(g.hitscanPuffs) == 0 {
		return
	}
	ca := math.Cos(camAng)
	sa := math.Sin(camAng)
	eyeZ := g.playerEyeZ()
	for _, p := range g.hitscanPuffs {
		if p.kind != hitscanFxTeleport || p.hidden {
			continue
		}
		px := float64(p.x)/fracUnit - camX
		py := float64(p.y)/fracUnit - camY
		f := px*ca + py*sa
		s := -px*sa + py*ca
		if f <= near {
			continue
		}
		clipTop := 0
		clipBottom := viewH - 1
		clipRadius := int64(8 * fracUnit)
		var clipOK bool
		clipTop, clipBottom, clipOK = g.spriteFootprintClipYBounds(p.x, p.y, clipRadius, viewH, eyeZ, f, focal)
		if !clipOK {
			continue
		}
		sx := float64(viewW)/2 - (s/f)*focal
		pz := float64(p.z) / fracUnit
		sy := float64(viewH)/2 - ((pz-eyeZ)/f)*focal
		spriteTex, hasSprite := g.hitscanEffectSprite(p)
		if !hasSprite || spriteTex == nil || spriteTex.Width <= 0 || spriteTex.Height <= 0 {
			continue
		}
		it := projectedPuffItem{
			dist:       f,
			sx:         sx,
			sy:         sy,
			clipTop:    clipTop,
			clipBottom: clipBottom,
			spriteTex:  spriteTex,
			hasSprite:  true,
		}
		g.drawProjectedPuffItem(it, focal, viewW, viewH)
	}
}

func (g *game) drawProjectedPuffItem(it projectedPuffItem, focal float64, viewW, viewH int) {
	if !it.hasSprite || it.spriteTex == nil {
		return
	}
	depthQ := encodeDepthQ(it.dist)
	th := it.spriteTex.Height
	tw := it.spriteTex.Width
	if th <= 0 || tw <= 0 {
		return
	}
	src32, ok32 := spritePixels32(it.spriteTex)
	if !ok32 {
		return
	}
	scale := focal / it.dist
	if scale <= 0 {
		return
	}
	dstW := float64(tw) * scale
	dstH := float64(th) * scale
	dstX := it.sx - float64(it.spriteTex.OffsetX)*scale
	dstY := it.sy - float64(it.spriteTex.OffsetY)*scale
	x0 := int(math.Floor(dstX))
	y0 := int(math.Floor(dstY))
	x1 := int(math.Ceil(dstX+dstW)) - 1
	y1 := int(math.Ceil(dstY+dstH)) - 1
	if x0 < 0 {
		x0 = 0
	}
	if y0 < 0 {
		y0 = 0
	}
	if x1 >= viewW {
		x1 = viewW - 1
	}
	if y1 >= viewH {
		y1 = viewH - 1
	}
	if y0 < it.clipTop {
		y0 = it.clipTop
	}
	if y1 > it.clipBottom {
		y1 = it.clipBottom
	}
	if y0 > y1 {
		return
	}
	if g.spriteWallClipQuadFullyOccluded(x0, x1, y0, y1, depthQ) {
		return
	}
	txLUT := g.ensureSpriteTXScratch(x1 - x0 + 1)
	for x := x0; x <= x1; x++ {
		tx := int((float64(x) + 0.5 - dstX) / scale)
		if tx < 0 {
			tx = 0
		}
		if tx >= tw {
			tx = tw - 1
		}
		txLUT[x-x0] = tx
	}
	tyLUT := g.ensureSpriteTYScratch(y1 - y0 + 1)
	for y := y0; y <= y1; y++ {
		ty := int((float64(y) + 0.5 - dstY) / scale)
		if ty < 0 {
			ty = 0
		}
		if ty >= th {
			ty = th - 1
		}
		tyLUT[y-y0] = ty
	}
	for y := y0; y <= y1; y++ {
		ty := tyLUT[y-y0]
		row := y * viewW
		if x1-x0 >= spriteRowOcclusionMinSpan && g.rowFullyOccludedDepthQ(depthQ, row, x0, x1) {
			continue
		}
		for x := x0; x <= x1; x++ {
			i := row + x
			if g.spriteWallClipOccludedAtIndexDepth(i, depthQ) {
				continue
			}
			pix := src32[ty*tw+txLUT[x-x0]]
			if ((pix >> pixelAShift) & 0xFF) == 0 {
				continue
			}
			g.writeWallPixel(i, pix)
		}
	}
}

func (g *game) appendMonsterCutoutItems(camX, camY, camAng, focal, near float64) {
	viewW := g.viewW
	viewH := g.viewH
	if len(g.wallPix32) != viewW*viewH {
		return
	}
	ca := math.Cos(camAng)
	sa := math.Sin(camAng)
	eyeZ := g.playerEyeZ()
	alpha := g.renderAlpha
	for i, th := range g.m.Things {
		if i < 0 || i >= len(g.thingCollected) || g.thingCollected[i] {
			continue
		}
		if !isMonster(th.Type) {
			continue
		}
		if !g.monsterVisibleAfterDeath(i, th.Type) {
			continue
		}
		txFixed, tyFixed, baseZFixed := g.thingRenderPosFixed(i, th, alpha)
		tx := float64(txFixed)/fracUnit - camX
		ty := float64(tyFixed)/fracUnit - camY
		f := tx*ca + ty*sa
		s := -tx*sa + ty*ca
		if f <= near {
			continue
		}
		sx := float64(viewW)/2 - (s/f)*focal
		baseZ := float64(baseZFixed) / fracUnit
		ref, flip, ok := g.monsterSpriteRefForView(i, th, g.worldTic, camX, camY)
		if !ok || ref == nil || ref.tex.Height <= 0 || ref.tex.Width <= 0 {
			continue
		}
		clipRadius := monsterSpriteClipRadius(th.Type)
		clipTop, clipBottom, clipOK := g.spriteFootprintClipYBounds(txFixed, tyFixed, clipRadius, viewH, eyeZ, f, focal)
		if !clipOK {
			continue
		}
		scale := focal / f
		if scale <= 0 {
			continue
		}
		clipBottom = spriteClipBottomWithPatchOverhang(clipBottom, ref.tex, scale, viewH)
		sy := float64(viewH)/2 - ((baseZ-eyeZ)/f)*focal
		w, h, dstX, dstY, x0, x1, y0, y1, boundsOK := cacheOriginSpriteItemGeometry(sx, sy, scale, ref.tex, clipTop, clipBottom, viewW, viewH)
		if h <= 0 || w <= 0 {
			continue
		}
		xPad := w/2 + 8
		yPad := h + 4
		if sx+xPad < 0 || sx-xPad > float64(viewW) || sy+yPad < 0 || sy-yPad > float64(viewH) {
			continue
		}
		sec := g.thingSectorCached(i, th)
		lightMul := uint32(256)
		if sec >= 0 && sec < len(g.m.Sectors) {
			lightMul = g.sectorLightMulCached(sec)
		}
		depthQ := encodeDepthQ(f)
		shadeMul := g.cachedThingShadeMul(i, ref.fullBright, lightMul, f, near)
		if !boundsOK {
			continue
		}
		opaqueRectStart, opaqueRectCount := g.appendProjectedOpaqueRects(ref.opaque.rects, ref.tex.Width, flip, dstX, dstY, scale, clipTop, clipBottom, viewW, viewH)
		if opaqueRectCount > 0 &&
			g.projectedOpaqueRectsFullyOccluded(g.projectedOpaqueRectScratch[opaqueRectStart:opaqueRectStart+opaqueRectCount], depthQ) {
			continue
		}
		if g.spriteWallClipQuadFullyOccluded(x0, x1, y0, y1, depthQ) {
			continue
		}
		item := cutoutItem{
			dist:            f,
			depthQ:          depthQ,
			kind:            billboardQueueMonsters,
			x0:              x0,
			x1:              x1,
			y0:              y0,
			y1:              y1,
			shadeMul:        shadeMul,
			tex:             ref.tex,
			flip:            flip,
			shadow:          monsterUsesShadow(th.Type),
			clipTop:         clipTop,
			clipBottom:      clipBottom,
			dstX:            dstX,
			dstY:            dstY,
			scale:           scale,
			opaque:          ref.opaque,
			hasOpaque:       ref.hasOpaque,
			opaqueRectStart: opaqueRectStart,
			opaqueRectCount: opaqueRectCount,
			boundsOK:        true,
		}
		g.billboardQueueScratch = append(g.billboardQueueScratch, item)
		if g.opts.DebugMonsterThinkerBlend {
			g.appendMonsterThinkerDebugCutoutItem(i, th, ref, flip, camX, camY, ca, sa, eyeZ, focal, near)
		}
	}
}

func (g *game) appendMonsterThinkerDebugCutoutItem(i int, th mapdata.Thing, ref *spriteRenderRef, flip bool, camX, camY, ca, sa, eyeZ, focal, near float64) {
	if g == nil || ref == nil || ref.tex == nil || ref.tex.Height <= 0 || ref.tex.Width <= 0 {
		return
	}
	txFixed, tyFixed := g.thingPosFixed(i, th)
	baseZFixed := g.monsterRenderBaseZ(i, th, txFixed, tyFixed)
	tx := float64(txFixed)/fracUnit - camX
	ty := float64(tyFixed)/fracUnit - camY
	f := tx*ca + ty*sa
	s := -tx*sa + ty*ca
	if f <= near {
		return
	}
	clipRadius := monsterSpriteClipRadius(th.Type)
	clipTop, clipBottom, clipOK := g.spriteFootprintClipYBounds(txFixed, tyFixed, clipRadius, g.viewH, eyeZ, f, focal)
	if !clipOK {
		return
	}
	scale := focal / f
	if scale <= 0 {
		return
	}
	clipBottom = spriteClipBottomWithPatchOverhang(clipBottom, ref.tex, scale, g.viewH)
	sx := float64(g.viewW)/2 - (s/f)*focal
	baseZ := float64(baseZFixed) / fracUnit
	sy := float64(g.viewH)/2 - ((baseZ-eyeZ)/f)*focal
	w, h, dstX, dstY, x0, x1, y0, y1, boundsOK := cacheOriginSpriteItemGeometry(sx, sy, scale, ref.tex, clipTop, clipBottom, g.viewW, g.viewH)
	if !boundsOK || h <= 0 || w <= 0 {
		return
	}
	opaqueRectStart, opaqueRectCount := g.appendProjectedOpaqueRects(ref.opaque.rects, ref.tex.Width, flip, dstX, dstY, scale, clipTop, clipBottom, g.viewW, g.viewH)
	g.billboardQueueScratch = append(g.billboardQueueScratch, cutoutItem{
		dist:            f,
		depthQ:          encodeDepthQ(f),
		kind:            billboardQueueMonsters,
		x0:              x0,
		x1:              x1,
		y0:              y0,
		y1:              y1,
		shadeMul:        256,
		tex:             ref.tex,
		flip:            flip,
		clipTop:         clipTop,
		clipBottom:      clipBottom,
		dstX:            dstX,
		dstY:            dstY,
		scale:           scale,
		opaque:          ref.opaque,
		hasOpaque:       ref.hasOpaque,
		opaqueRectStart: opaqueRectStart,
		opaqueRectCount: opaqueRectCount,
		boundsOK:        true,
		debugOverlay:    true,
	})
}

func (g *game) monsterRenderBaseZ(i int, th mapdata.Thing, x, y int64) int64 {
	if monsterCanFloat(th.Type) && (i < 0 || i >= len(g.thingDead) || !g.thingDead[i]) {
		z, _, _ := g.thingSupportState(i, th)
		return z
	}
	return g.thingFloorZ(x, y)
}

func (g *game) drawShadowSpriteCutoutSourcePort(
	it cutoutItem,
	src32 []uint32,
	tw int,
	txLUT, tyLUT []int,
	x0, x1, y0, y1 int,
) {
	if g == nil || g.viewW <= 0 || g.viewH <= 0 || x0 > x1 || y0 > y1 {
		return
	}
	coarseW := max(1, doomLogicalW)
	coarseH := max(1, doomLogicalH)
	cx0 := x0 * coarseW / g.viewW
	cx1 := x1 * coarseW / g.viewW
	cy0 := y0 * coarseH / g.viewH
	cy1 := y1 * coarseH / g.viewH
	for cx := cx0; cx <= cx1; cx++ {
		hx0 := cx * g.viewW / coarseW
		hx1 := ((cx+1)*g.viewW + coarseW - 1) / coarseW
		hx1--
		if hx0 < x0 {
			hx0 = x0
		}
		if hx1 > x1 {
			hx1 = x1
		}
		if hx0 > hx1 {
			continue
		}
		repX := (hx0 + hx1) / 2
		if repX < x0 {
			repX = x0
		}
		if repX > x1 {
			repX = x1
		}
		tx := txLUT[repX-x0]
		for cy := cy0; cy <= cy1; cy++ {
			hy0 := cy * g.viewH / coarseH
			hy1 := ((cy+1)*g.viewH + coarseH - 1) / coarseH
			hy1--
			if hy0 < y0 {
				hy0 = y0
			}
			if hy1 > y1 {
				hy1 = y1
			}
			if hy0 > hy1 {
				continue
			}
			repY := (hy0 + hy1) / 2
			if repY < y0 {
				repY = y0
			}
			if repY > y1 {
				repY = y1
			}
			ty := tyLUT[repY-y0]
			p := src32[ty*tw+tx]
			if ((p >> pixelAShift) & 0xFF) == 0 {
				continue
			}

			delta := g.nextSourcePortFuzzOffset()
			srcCY := cy + delta
			if srcCY < 1 {
				srcCY = 1
			}
			if srcCY >= coarseH-1 {
				srcCY = coarseH - 2
			}
			srcHX := (cx*g.viewW + g.viewW/2) / coarseW
			srcHY := (srcCY*g.viewH + g.viewH/2) / coarseH
			if srcHX < 0 {
				srcHX = 0
			}
			if srcHX >= g.viewW {
				srcHX = g.viewW - 1
			}
			if srcHY < 0 {
				srcHY = 0
			}
			if srcHY >= g.viewH {
				srcHY = g.viewH - 1
			}
			srcI := srcHY*g.viewW + srcHX
			if srcI < 0 || srcI >= len(g.wallPix32) {
				srcI = repY*g.viewW + repX
			}
			fuzzPix := g.wallPix32[srcI]
			if fuzzPix == 0 {
				fuzzPix = packRGBA(0, 0, 0)
			}
			fuzzPix = g.shadePackedSpectreFuzz(fuzzPix)

			for y := hy0; y <= hy1; y++ {
				row := y * g.viewW
				for x := hx0; x <= hx1; x++ {
					if !xInSolidSpans(x, it.clipSpans) {
						continue
					}
					i := row + x
					if g.spriteWallClipOccludedAtIndexDepth(i, it.depthQ) {
						continue
					}
					g.writeWallPixel(i, fuzzPix)
				}
			}
		}
	}
}

func (g *game) appendThingCutoutItems(camX, camY, camAng, focal, near float64) {
	viewW := g.viewW
	viewH := g.viewH
	if len(g.wallPix32) != viewW*viewH {
		return
	}
	ca := math.Cos(camAng)
	sa := math.Sin(camAng)
	eyeZ := g.playerEyeZ()
	animTickUnits, animUnitsPerTic := g.worldThingAnimTickUnits()
	for i, th := range g.m.Things {
		if i < 0 || i >= len(g.thingCollected) || g.thingCollected[i] {
			continue
		}
		if isMonster(th.Type) || isPlayerStart(th.Type) {
			continue
		}
		sec := g.thingSectorCached(i, th)
		ref, ok := g.runtimeWorldThingSpriteRef(i, th, animTickUnits, animUnitsPerTic)
		if !ok || ref == nil || ref.tex.Height <= 0 || ref.tex.Width <= 0 {
			continue
		}
		txFixed, tyFixed := g.thingPosFixed(i, th)
		tx := float64(txFixed)/fracUnit - camX
		ty := float64(tyFixed)/fracUnit - camY
		f := tx*ca + ty*sa
		s := -tx*sa + ty*ca
		if f <= near {
			continue
		}
		scale := focal / f
		if scale <= 0 {
			continue
		}
		floorZFixed := g.thingFloorZ(txFixed, tyFixed)
		floorZ := float64(floorZFixed) / fracUnit
		yb := float64(viewH)/2 - ((floorZ-eyeZ)/f)*focal
		h := float64(ref.tex.Height) * scale
		if h <= 0 {
			continue
		}
		sx := float64(viewW)/2 - (s/f)*focal
		w := float64(ref.tex.Width) * scale
		xPad := w/2 + 4
		if sx+xPad < 0 || sx-xPad > float64(viewW) {
			continue
		}
		clipRadius := worldThingSpriteClipRadius(th.Type)
		clipTop, clipBottom, clipOK := g.spriteFootprintClipYBounds(txFixed, tyFixed, clipRadius, viewH, eyeZ, f, focal)
		if !clipOK {
			continue
		}
		lightMul := uint32(256)
		if sec >= 0 && sec < len(g.m.Sectors) {
			lightMul = g.sectorLightMulCached(sec)
		}
		depthQ := encodeDepthQ(f)
		scale, dstX, dstY, x0, x1, y0, y1, boundsOK := cacheFloorSpriteItemGeometry(sx, yb, h, ref.tex, clipTop, clipBottom, viewW, viewH)
		shadeMul := g.cachedThingShadeMul(i, ref.fullBright, lightMul, f, near)
		if !boundsOK {
			continue
		}
		opaqueRectStart, opaqueRectCount := g.appendProjectedOpaqueRects(ref.opaque.rects, ref.tex.Width, false, dstX, dstY, scale, clipTop, clipBottom, viewW, viewH)
		if opaqueRectCount > 0 &&
			g.projectedOpaqueRectsFullyOccluded(g.projectedOpaqueRectScratch[opaqueRectStart:opaqueRectStart+opaqueRectCount], depthQ) {
			continue
		}
		if g.spriteWallClipQuadFullyOccluded(x0, x1, y0, y1, depthQ) {
			continue
		}
		g.billboardQueueScratch = append(g.billboardQueueScratch, cutoutItem{
			dist:            f,
			depthQ:          depthQ,
			kind:            billboardQueueWorldThings,
			x0:              x0,
			x1:              x1,
			y0:              y0,
			y1:              y1,
			shadeMul:        shadeMul,
			tex:             ref.tex,
			clipTop:         clipTop,
			clipBottom:      clipBottom,
			dstX:            dstX,
			dstY:            dstY,
			scale:           scale,
			opaque:          ref.opaque,
			hasOpaque:       ref.hasOpaque,
			opaqueRectStart: opaqueRectStart,
			opaqueRectCount: opaqueRectCount,
			boundsOK:        true,
		})
	}
}

func (g *game) worldThingAnimTickUnits() (tickUnits int, unitsPerTic int) {
	unitsPerTic = 1
	tickUnits = g.worldTic
	if g == nil || !g.opts.SourcePortMode || sourcePortThingAnimSubsteps <= 1 {
		return tickUnits, unitsPerTic
	}
	unitsPerTic = sourcePortThingAnimSubsteps
	alpha := g.renderAlpha
	sub := int(alpha * float64(unitsPerTic))
	if sub < 0 {
		sub = 0
	}
	if sub >= unitsPerTic {
		sub = unitsPerTic - 1
	}
	tickUnits = g.worldTic*unitsPerTic + sub
	return tickUnits, unitsPerTic
}

func (g *game) worldThingSpriteName(typ int16, tic int) string {
	return g.worldThingSpriteNameScaled(typ, tic, 1)
}

func (g *game) worldThingSpriteNameScaled(typ int16, tickUnits, unitsPerTic int) string {
	pick := func(seq ...string) string {
		return g.pickAnimatedThingSpriteNameScaled(tickUnits, 8, unitsPerTic, seq...)
	}
	pickState := func(states ...thingAnimState) string {
		return g.pickThingStateSpriteNameScaled(tickUnits, unitsPerTic, states...)
	}
	switch typ {
	case 15:
		return pick("PLAYN0")
	case 18:
		return pick("POSSL0")
	case 19:
		return pick("SPOSL0")
	case 20:
		return pick("TROOL0")
	case 21:
		return pick("SARGN0")
	case 22:
		return pick("HEADL0")
	case 23:
		return pick("SKULF0")
	case 24:
		return pick("POL5A0")
	case 25:
		return pick("POL1A0")
	case 26:
		return pickState(
			thingAnimState{name: "POL6A0", tics: 6},
			thingAnimState{name: "POL6B0", tics: 8},
		)
	case 27:
		return pick("POL4A0")
	case 28:
		return pick("POL2A0")
	case 29:
		return pickState(
			thingAnimState{name: "POL3A0", tics: 6},
			thingAnimState{name: "POL3B0", tics: 6},
		)
	case 30:
		return pick("COL1A0")
	case 31:
		return pick("COL2A0")
	case 32:
		return pick("COL3A0")
	case 33:
		return pick("COL4A0")
	case 34:
		return pick("CANDA0")
	case 35:
		return pick("CBRAA0")
	case 36:
		return pickState(
			thingAnimState{name: "COL5A0", tics: 14},
			thingAnimState{name: "COL5B0", tics: 14},
		)
	case 37:
		return pick("COL6A0")
	case 41:
		return pickState(
			thingAnimState{name: "CEYEA0", tics: 6},
			thingAnimState{name: "CEYEB0", tics: 6},
			thingAnimState{name: "CEYEC0", tics: 6},
			thingAnimState{name: "CEYEB0", tics: 6},
		)
	case 42:
		return pickState(
			thingAnimState{name: "FSKUA0", tics: 6},
			thingAnimState{name: "FSKUB0", tics: 6},
			thingAnimState{name: "FSKUC0", tics: 6},
		)
	case 43:
		return pick("TRE1A0")
	case 47:
		return pick("SMITA0")
	case 48:
		return pick("ELECA0")
	case 2028:
		return pick("COLUA0")
	case 49:
		return pickState(
			thingAnimState{name: "GOR1A0", tics: 10},
			thingAnimState{name: "GOR1B0", tics: 15},
			thingAnimState{name: "GOR1C0", tics: 8},
			thingAnimState{name: "GOR1B0", tics: 6},
		)
	case 50:
		return pick("GOR2A0")
	case 51:
		return pick("GOR3A0")
	case 52:
		return pick("GOR4A0")
	case 53:
		return pick("GOR5A0")
	case 54:
		return pick("TRE2A0")
	case 59:
		return pick("GOR2A0")
	case 60:
		return pick("GOR4A0")
	case 61:
		return pick("GOR3A0")
	case 62:
		return pick("GOR5A0")
	case 63:
		return pickState(
			thingAnimState{name: "GOR1A0", tics: 10},
			thingAnimState{name: "GOR1B0", tics: 15},
			thingAnimState{name: "GOR1C0", tics: 8},
			thingAnimState{name: "GOR1B0", tics: 6},
		)
	case 70:
		return pickState(
			thingAnimState{name: "FCANA0", tics: 4},
			thingAnimState{name: "FCANB0", tics: 4},
			thingAnimState{name: "FCANC0", tics: 4},
		)
	case 72:
		return pickState(thingAnimState{name: "KEENA0", tics: -1})
	case 73:
		return pick("HDB1A0")
	case 74:
		return pick("HDB2A0")
	case 75:
		return pick("HDB3A0")
	case 76:
		return pick("HDB4A0")
	case 77:
		return pick("HDB5A0")
	case 78:
		return pick("HDB6A0")
	case 79:
		return pick("POB1A0")
	case 80:
		return pick("POB2A0")
	case 81:
		return pick("BRS1A0")
	case 5:
		return pickState(
			thingAnimState{name: "BKEYA0", tics: 10},
			thingAnimState{name: "BKEYB0", tics: 10},
		)
	case 6:
		return pickState(
			thingAnimState{name: "YKEYA0", tics: 10},
			thingAnimState{name: "YKEYB0", tics: 10},
		)
	case 13:
		return pickState(
			thingAnimState{name: "RKEYA0", tics: 10},
			thingAnimState{name: "RKEYB0", tics: 10},
		)
	case 38:
		return pickState(
			thingAnimState{name: "RSKUA0", tics: 10},
			thingAnimState{name: "RSKUB0", tics: 10},
		)
	case 39:
		return pickState(
			thingAnimState{name: "YSKUA0", tics: 10},
			thingAnimState{name: "YSKUB0", tics: 10},
		)
	case 40:
		return pickState(
			thingAnimState{name: "BSKUA0", tics: 10},
			thingAnimState{name: "BSKUB0", tics: 10},
		)
	case 2011:
		return pick("STIMA0")
	case 2012:
		return pick("MEDIA0")
	case 2013:
		return pickState(
			thingAnimState{name: "SOULA0", tics: 6},
			thingAnimState{name: "SOULB0", tics: 6},
			thingAnimState{name: "SOULC0", tics: 6},
			thingAnimState{name: "SOULD0", tics: 6},
			thingAnimState{name: "SOULC0", tics: 6},
			thingAnimState{name: "SOULB0", tics: 6},
		)
	case 2014:
		return pickState(
			thingAnimState{name: "BON1A0", tics: 6},
			thingAnimState{name: "BON1B0", tics: 6},
			thingAnimState{name: "BON1C0", tics: 6},
			thingAnimState{name: "BON1D0", tics: 6},
			thingAnimState{name: "BON1C0", tics: 6},
			thingAnimState{name: "BON1B0", tics: 6},
		)
	case 2015:
		return pickState(
			thingAnimState{name: "BON2A0", tics: 6},
			thingAnimState{name: "BON2B0", tics: 6},
			thingAnimState{name: "BON2C0", tics: 6},
			thingAnimState{name: "BON2D0", tics: 6},
			thingAnimState{name: "BON2C0", tics: 6},
			thingAnimState{name: "BON2B0", tics: 6},
		)
	case 2018:
		return pickState(
			thingAnimState{name: "ARM1A0", tics: 6},
			thingAnimState{name: "ARM1B0", tics: 7},
		)
	case 2019:
		return pickState(
			thingAnimState{name: "ARM2A0", tics: 6},
			thingAnimState{name: "ARM2B0", tics: 6},
		)
	case 2022:
		return pickState(
			thingAnimState{name: "PINVA0", tics: 6},
			thingAnimState{name: "PINVB0", tics: 6},
			thingAnimState{name: "PINVC0", tics: 6},
			thingAnimState{name: "PINVD0", tics: 6},
		)
	case 2023:
		return pickState(thingAnimState{name: "PSTRA0", tics: -1})
	case 2024:
		return pickState(
			thingAnimState{name: "PINSA0", tics: 6},
			thingAnimState{name: "PINSB0", tics: 6},
			thingAnimState{name: "PINSC0", tics: 6},
			thingAnimState{name: "PINSD0", tics: 6},
		)
	case 2025:
		return pickState(thingAnimState{name: "SUITA0", tics: -1})
	case 2026:
		return pickState(
			thingAnimState{name: "PMAPA0", tics: 6},
			thingAnimState{name: "PMAPB0", tics: 6},
			thingAnimState{name: "PMAPC0", tics: 6},
			thingAnimState{name: "PMAPD0", tics: 6},
			thingAnimState{name: "PMAPC0", tics: 6},
			thingAnimState{name: "PMAPB0", tics: 6},
		)
	case 2045:
		return pickState(
			thingAnimState{name: "PVISA0", tics: 6},
			thingAnimState{name: "PVISB0", tics: 6},
		)
	case 83:
		return pickState(
			thingAnimState{name: "MEGAA0", tics: 6},
			thingAnimState{name: "MEGAB0", tics: 6},
			thingAnimState{name: "MEGAC0", tics: 6},
			thingAnimState{name: "MEGAD0", tics: 6},
		)
	case 2007:
		return pick("CLIPA0")
	case 2048:
		return pick("AMMOA0")
	case 2008:
		return pick("SHELA0")
	case 2049:
		return pick("SBOXA0")
	case 2010:
		return pick("ROCKA0")
	case 2046:
		return pick("BROKA0")
	case 2047:
		return pick("CELLA0")
	case 17:
		return pick("CELPA0")
	case 8:
		return pick("BPAKA0")
	case 2001:
		return pick("SHOTA0")
	case 2002:
		return pick("MGUNA0")
	case 2003:
		return pick("LAUNA0")
	case 2004:
		return pick("PLASA0")
	case 2005:
		return pick("CSAWA0")
	case 2006:
		return pick("BFUGA0")
	case 2035:
		return pickState(
			thingAnimState{name: "BAR1A0", tics: 6},
			thingAnimState{name: "BAR1B0", tics: 6},
			thingAnimState{name: "BEXPA0", tics: 6},
		)
	case 44:
		return pickState(
			thingAnimState{name: "TBLUA0", tics: 4},
			thingAnimState{name: "TBLUB0", tics: 4},
			thingAnimState{name: "TBLUC0", tics: 4},
			thingAnimState{name: "TBLUD0", tics: 4},
		)
	case 45:
		return pickState(
			thingAnimState{name: "TGRNA0", tics: 4},
			thingAnimState{name: "TGRNB0", tics: 4},
			thingAnimState{name: "TGRNC0", tics: 4},
			thingAnimState{name: "TGRND0", tics: 4},
		)
	case 46:
		return pickState(
			thingAnimState{name: "TREDA0", tics: 4},
			thingAnimState{name: "TREDB0", tics: 4},
			thingAnimState{name: "TREDC0", tics: 4},
			thingAnimState{name: "TREDD0", tics: 4},
		)
	case 55:
		return pickState(
			thingAnimState{name: "SMRTA0", tics: 4},
			thingAnimState{name: "SMRTB0", tics: 4},
			thingAnimState{name: "SMRTC0", tics: 4},
			thingAnimState{name: "SMRTD0", tics: 4},
		)
	case 56:
		return pickState(
			thingAnimState{name: "SMGTA0", tics: 4},
			thingAnimState{name: "SMGTB0", tics: 4},
			thingAnimState{name: "SMGTC0", tics: 4},
			thingAnimState{name: "SMGTD0", tics: 4},
		)
	case 57:
		return pickState(
			thingAnimState{name: "SMBTA0", tics: 4},
			thingAnimState{name: "SMBTB0", tics: 4},
			thingAnimState{name: "SMBTC0", tics: 4},
			thingAnimState{name: "SMBTD0", tics: 4},
		)
	case 85:
		return pickState(
			thingAnimState{name: "TLMPA0", tics: 4},
			thingAnimState{name: "TLMPB0", tics: 4},
			thingAnimState{name: "TLMPC0", tics: 4},
			thingAnimState{name: "TLMPD0", tics: 4},
		)
	case 86:
		return pickState(
			thingAnimState{name: "TLP2A0", tics: 4},
			thingAnimState{name: "TLP2B0", tics: 4},
			thingAnimState{name: "TLP2C0", tics: 4},
			thingAnimState{name: "TLP2D0", tics: 4},
		)
	default:
		return ""
	}
}

func (g *game) bossCubeSpriteName(tic int) string {
	frame := (tic / 3) & 3
	frameLetters := []byte{'A', 'B', 'C', 'D'}
	for i := 0; i < len(frameLetters); i++ {
		fl := frameLetters[(frame+i)%len(frameLetters)]
		name0 := spriteFrameName("BOSF", fl, '0')
		if g.spritePatchExists(name0) {
			return name0
		}
		if name, _, ok := g.monsterSpriteRotFrame("BOSF", fl, 1); ok {
			return name
		}
	}
	return ""
}

func (g *game) bossCubeSpriteRef(tic int) (*spriteRenderRef, bool) {
	name := g.bossCubeSpriteName(tic)
	if name == "" {
		return nil, false
	}
	return g.spriteRenderRef(name)
}

func (g *game) bossSpawnFireSpriteName(elapsed int) string {
	frameLetters := []byte{'A', 'B', 'C', 'D', 'E', 'F', 'G', 'H'}
	frame := (elapsed / 4) % len(frameLetters)
	for i := 0; i < len(frameLetters); i++ {
		fl := frameLetters[(frame+i)%len(frameLetters)]
		name0 := spriteFrameName("FIRE", fl, '0')
		if g.spritePatchExists(name0) {
			return name0
		}
		if name, _, ok := g.monsterSpriteRotFrame("FIRE", fl, 1); ok {
			return name
		}
	}
	return ""
}

func (g *game) bossSpawnFireSpriteRef(elapsed int) (*spriteRenderRef, bool) {
	name := g.bossSpawnFireSpriteName(elapsed)
	if name == "" {
		return nil, false
	}
	return g.spriteRenderRef(name)
}

func (g *game) spriteRenderRef(name string) (*spriteRenderRef, bool) {
	if name == "" {
		return nil, false
	}
	if g.spriteRenderRefCache == nil {
		g.spriteRenderRefCache = make(map[string]*spriteRenderRef, 256)
	}
	if ref, ok := g.spriteRenderRefCache[name]; ok {
		return ref, ref != nil
	}
	tex, ok := g.monsterSpriteTexture(name)
	if !ok || tex == nil || tex.Width <= 0 || tex.Height <= 0 {
		g.spriteRenderRefCache[name] = nil
		return nil, false
	}
	opaque, hasOpaque := g.spriteOpaqueShapeForKey(name, tex)
	ref := &spriteRenderRef{
		key:        name,
		tex:        tex,
		opaque:     opaque,
		hasOpaque:  hasOpaque,
		fullBright: doomSourceSpriteFullBright(name),
	}
	g.spriteRenderRefCache[name] = ref
	return ref, true
}

func pickThingAnimRef(anim thingAnimRefState, tickUnits, unitsPerTic int) *spriteRenderRef {
	if len(anim.refs) == 0 {
		return nil
	}
	if len(anim.refs) == 1 {
		return anim.refs[0]
	}
	if unitsPerTic <= 0 {
		unitsPerTic = 1
	}
	if anim.frameUnits > 0 {
		return anim.refs[(tickUnits/anim.frameUnits)%len(anim.refs)]
	}
	total := 0
	for _, tics := range anim.tics {
		if tics > 0 {
			total += tics * unitsPerTic
		}
	}
	if total <= 0 {
		return anim.refs[0]
	}
	t := tickUnits % total
	if t < 0 {
		t += total
	}
	acc := 0
	for i, ref := range anim.refs {
		stepUnits := anim.tics[i] * unitsPerTic
		if stepUnits <= 0 {
			return ref
		}
		acc += stepUnits
		if t < acc {
			return ref
		}
	}
	return anim.refs[len(anim.refs)-1]
}

func (g *game) thingAnimRefsFromSeq(seq ...string) thingAnimRefState {
	if len(seq) == 0 {
		return thingAnimRefState{}
	}
	refs := make([]*spriteRenderRef, 0, len(seq))
	seenExplicit := make(map[string]struct{}, len(seq))
	for _, raw := range seq {
		name := strings.ToUpper(strings.TrimSpace(raw))
		if name == "" {
			continue
		}
		if _, dup := seenExplicit[name]; dup {
			continue
		}
		ref, ok := g.spriteRenderRef(name)
		if !ok {
			continue
		}
		seenExplicit[name] = struct{}{}
		refs = append(refs, ref)
	}
	if len(refs) <= 1 {
		expanded := make([]*spriteRenderRef, 0, len(seq)*4)
		seen := make(map[string]struct{}, len(seq)*8)
		for _, raw := range seq {
			name := strings.ToUpper(strings.TrimSpace(raw))
			if name == "" {
				continue
			}
			if ref, ok := g.spriteRenderRef(name); ok {
				if _, dup := seen[ref.key]; !dup {
					seen[ref.key] = struct{}{}
					expanded = append(expanded, ref)
				}
			}
			for _, ex := range g.expandThingSpriteFrames(name) {
				ref, ok := g.spriteRenderRef(ex)
				if !ok {
					continue
				}
				if _, dup := seen[ref.key]; dup {
					continue
				}
				seen[ref.key] = struct{}{}
				expanded = append(expanded, ref)
			}
		}
		refs = expanded
	}
	anim := thingAnimRefState{refs: refs, frameUnits: 8}
	if len(refs) == 1 {
		anim.staticRef = refs[0]
	}
	return anim
}

func (g *game) thingAnimRefsFromStates(states ...thingAnimState) thingAnimRefState {
	if len(states) == 0 {
		return thingAnimRefState{}
	}
	refs := make([]*spriteRenderRef, 0, len(states))
	tics := make([]int, 0, len(states))
	for _, st := range states {
		name := strings.ToUpper(strings.TrimSpace(st.name))
		if name == "" {
			continue
		}
		ref, ok := g.spriteRenderRef(name)
		if !ok {
			continue
		}
		refs = append(refs, ref)
		tics = append(tics, st.tics)
	}
	anim := thingAnimRefState{refs: refs, tics: tics}
	if len(refs) == 1 {
		anim.staticRef = refs[0]
	}
	return anim
}

func (g *game) worldThingAnimRefs(typ int16) thingAnimRefState {
	if g.worldThingAnimRefCache == nil {
		g.worldThingAnimRefCache = make(map[int16]thingAnimRefState, 128)
	}
	if anim, ok := g.worldThingAnimRefCache[typ]; ok {
		return anim
	}
	var anim thingAnimRefState
	switch typ {
	case 15:
		anim = g.thingAnimRefsFromSeq("PLAYN0")
	case 18:
		anim = g.thingAnimRefsFromSeq("POSSL0")
	case 19:
		anim = g.thingAnimRefsFromSeq("SPOSL0")
	case 20:
		anim = g.thingAnimRefsFromSeq("TROOL0")
	case 21:
		anim = g.thingAnimRefsFromSeq("SARGN0")
	case 22:
		anim = g.thingAnimRefsFromSeq("HEADL0")
	case 23:
		anim = g.thingAnimRefsFromSeq("SKULF0")
	case 24:
		anim = g.thingAnimRefsFromSeq("POL5A0")
	case 25:
		anim = g.thingAnimRefsFromSeq("POL1A0")
	case 26:
		anim = g.thingAnimRefsFromStates(
			thingAnimState{name: "POL6A0", tics: 6},
			thingAnimState{name: "POL6B0", tics: 8},
		)
	case 27:
		anim = g.thingAnimRefsFromSeq("POL4A0")
	case 28:
		anim = g.thingAnimRefsFromSeq("POL2A0")
	case 29:
		anim = g.thingAnimRefsFromStates(
			thingAnimState{name: "POL3A0", tics: 6},
			thingAnimState{name: "POL3B0", tics: 6},
		)
	case 30:
		anim = g.thingAnimRefsFromSeq("COL1A0")
	case 31:
		anim = g.thingAnimRefsFromSeq("COL2A0")
	case 32:
		anim = g.thingAnimRefsFromSeq("COL3A0")
	case 33:
		anim = g.thingAnimRefsFromSeq("COL4A0")
	case 34:
		anim = g.thingAnimRefsFromSeq("CANDA0")
	case 35:
		anim = g.thingAnimRefsFromSeq("CBRAA0")
	case 36:
		anim = g.thingAnimRefsFromStates(
			thingAnimState{name: "COL5A0", tics: 14},
			thingAnimState{name: "COL5B0", tics: 14},
		)
	case 37:
		anim = g.thingAnimRefsFromSeq("COL6A0")
	case 41:
		anim = g.thingAnimRefsFromStates(
			thingAnimState{name: "CEYEA0", tics: 6},
			thingAnimState{name: "CEYEB0", tics: 6},
			thingAnimState{name: "CEYEC0", tics: 6},
			thingAnimState{name: "CEYEB0", tics: 6},
		)
	case 42:
		anim = g.thingAnimRefsFromStates(
			thingAnimState{name: "FSKUA0", tics: 6},
			thingAnimState{name: "FSKUB0", tics: 6},
			thingAnimState{name: "FSKUC0", tics: 6},
		)
	case 88:
		anim = g.thingAnimRefsFromStates(
			thingAnimState{name: "BBRNA0", tics: -1},
			thingAnimState{name: "BBRNB0", tics: 36},
		)
	case 89:
		anim = g.thingAnimRefsFromStates(
			thingAnimState{name: "SSWVE0", tics: 10},
			thingAnimState{name: "SSWVE0", tics: 181},
			thingAnimState{name: "SSWVE0", tics: 150},
		)
	case 43:
		anim = g.thingAnimRefsFromSeq("TRE1A0")
	case 47:
		anim = g.thingAnimRefsFromSeq("SMITA0")
	case 48:
		anim = g.thingAnimRefsFromSeq("ELECA0")
	case 2028:
		anim = g.thingAnimRefsFromSeq("COLUA0")
	case 49:
		anim = g.thingAnimRefsFromStates(
			thingAnimState{name: "GOR1A0", tics: 10},
			thingAnimState{name: "GOR1B0", tics: 15},
			thingAnimState{name: "GOR1C0", tics: 8},
			thingAnimState{name: "GOR1B0", tics: 6},
		)
	case 50:
		anim = g.thingAnimRefsFromSeq("GOR2A0")
	case 51:
		anim = g.thingAnimRefsFromSeq("GOR3A0")
	case 52:
		anim = g.thingAnimRefsFromSeq("GOR4A0")
	case 53:
		anim = g.thingAnimRefsFromSeq("GOR5A0")
	case 54:
		anim = g.thingAnimRefsFromSeq("TRE2A0")
	case 59:
		anim = g.thingAnimRefsFromSeq("GOR2A0")
	case 60:
		anim = g.thingAnimRefsFromSeq("GOR4A0")
	case 61:
		anim = g.thingAnimRefsFromSeq("GOR3A0")
	case 62:
		anim = g.thingAnimRefsFromSeq("GOR5A0")
	case 63:
		anim = g.thingAnimRefsFromStates(
			thingAnimState{name: "GOR1A0", tics: 10},
			thingAnimState{name: "GOR1B0", tics: 15},
			thingAnimState{name: "GOR1C0", tics: 8},
			thingAnimState{name: "GOR1B0", tics: 6},
		)
	case 70:
		anim = g.thingAnimRefsFromStates(
			thingAnimState{name: "FCANA0", tics: 4},
			thingAnimState{name: "FCANB0", tics: 4},
			thingAnimState{name: "FCANC0", tics: 4},
		)
	case 72:
		anim = g.thingAnimRefsFromStates(thingAnimState{name: "KEENA0", tics: -1})
	case 73:
		anim = g.thingAnimRefsFromSeq("HDB1A0")
	case 74:
		anim = g.thingAnimRefsFromSeq("HDB2A0")
	case 75:
		anim = g.thingAnimRefsFromSeq("HDB3A0")
	case 76:
		anim = g.thingAnimRefsFromSeq("HDB4A0")
	case 77:
		anim = g.thingAnimRefsFromSeq("HDB5A0")
	case 78:
		anim = g.thingAnimRefsFromSeq("HDB6A0")
	case 79:
		anim = g.thingAnimRefsFromSeq("POB1A0")
	case 80:
		anim = g.thingAnimRefsFromSeq("POB2A0")
	case 81:
		anim = g.thingAnimRefsFromSeq("BRS1A0")
	case 5:
		anim = g.thingAnimRefsFromStates(thingAnimState{name: "BKEYA0", tics: 10}, thingAnimState{name: "BKEYB0", tics: 10})
	case 6:
		anim = g.thingAnimRefsFromStates(thingAnimState{name: "YKEYA0", tics: 10}, thingAnimState{name: "YKEYB0", tics: 10})
	case 13:
		anim = g.thingAnimRefsFromStates(thingAnimState{name: "RKEYA0", tics: 10}, thingAnimState{name: "RKEYB0", tics: 10})
	case 38:
		anim = g.thingAnimRefsFromStates(thingAnimState{name: "RSKUA0", tics: 10}, thingAnimState{name: "RSKUB0", tics: 10})
	case 39:
		anim = g.thingAnimRefsFromStates(thingAnimState{name: "YSKUA0", tics: 10}, thingAnimState{name: "YSKUB0", tics: 10})
	case 40:
		anim = g.thingAnimRefsFromStates(thingAnimState{name: "BSKUA0", tics: 10}, thingAnimState{name: "BSKUB0", tics: 10})
	case 2011:
		anim = g.thingAnimRefsFromSeq("STIMA0")
	case 2012:
		anim = g.thingAnimRefsFromSeq("MEDIA0")
	case 2013:
		anim = g.thingAnimRefsFromStates(
			thingAnimState{name: "SOULA0", tics: 6}, thingAnimState{name: "SOULB0", tics: 6},
			thingAnimState{name: "SOULC0", tics: 6}, thingAnimState{name: "SOULD0", tics: 6},
			thingAnimState{name: "SOULC0", tics: 6}, thingAnimState{name: "SOULB0", tics: 6},
		)
	case 2014:
		anim = g.thingAnimRefsFromStates(
			thingAnimState{name: "BON1A0", tics: 6}, thingAnimState{name: "BON1B0", tics: 6},
			thingAnimState{name: "BON1C0", tics: 6}, thingAnimState{name: "BON1D0", tics: 6},
			thingAnimState{name: "BON1C0", tics: 6}, thingAnimState{name: "BON1B0", tics: 6},
		)
	case 2015:
		anim = g.thingAnimRefsFromStates(
			thingAnimState{name: "BON2A0", tics: 6}, thingAnimState{name: "BON2B0", tics: 6},
			thingAnimState{name: "BON2C0", tics: 6}, thingAnimState{name: "BON2D0", tics: 6},
			thingAnimState{name: "BON2C0", tics: 6}, thingAnimState{name: "BON2B0", tics: 6},
		)
	case 2018:
		anim = g.thingAnimRefsFromStates(thingAnimState{name: "ARM1A0", tics: 6}, thingAnimState{name: "ARM1B0", tics: 7})
	case 2019:
		anim = g.thingAnimRefsFromStates(thingAnimState{name: "ARM2A0", tics: 6}, thingAnimState{name: "ARM2B0", tics: 6})
	case 2022:
		anim = g.thingAnimRefsFromStates(
			thingAnimState{name: "PINVA0", tics: 6}, thingAnimState{name: "PINVB0", tics: 6},
			thingAnimState{name: "PINVC0", tics: 6}, thingAnimState{name: "PINVD0", tics: 6},
		)
	case 2023:
		anim = g.thingAnimRefsFromStates(thingAnimState{name: "PSTRA0", tics: -1})
	case 2024:
		anim = g.thingAnimRefsFromStates(
			thingAnimState{name: "PINSA0", tics: 6}, thingAnimState{name: "PINSB0", tics: 6},
			thingAnimState{name: "PINSC0", tics: 6}, thingAnimState{name: "PINSD0", tics: 6},
		)
	case 2025:
		anim = g.thingAnimRefsFromStates(thingAnimState{name: "SUITA0", tics: -1})
	case 2026:
		anim = g.thingAnimRefsFromStates(
			thingAnimState{name: "PMAPA0", tics: 6}, thingAnimState{name: "PMAPB0", tics: 6},
			thingAnimState{name: "PMAPC0", tics: 6}, thingAnimState{name: "PMAPD0", tics: 6},
			thingAnimState{name: "PMAPC0", tics: 6}, thingAnimState{name: "PMAPB0", tics: 6},
		)
	case 2045:
		anim = g.thingAnimRefsFromStates(thingAnimState{name: "PVISA0", tics: 6}, thingAnimState{name: "PVISB0", tics: 6})
	case 83:
		anim = g.thingAnimRefsFromStates(
			thingAnimState{name: "MEGAA0", tics: 6}, thingAnimState{name: "MEGAB0", tics: 6},
			thingAnimState{name: "MEGAC0", tics: 6}, thingAnimState{name: "MEGAD0", tics: 6},
		)
	case 2007:
		anim = g.thingAnimRefsFromSeq("CLIPA0")
	case 2048:
		anim = g.thingAnimRefsFromSeq("AMMOA0")
	case 2008:
		anim = g.thingAnimRefsFromSeq("SHELA0")
	case 2049:
		anim = g.thingAnimRefsFromSeq("SBOXA0")
	case 2010:
		anim = g.thingAnimRefsFromSeq("ROCKA0")
	case 2046:
		anim = g.thingAnimRefsFromSeq("BROKA0")
	case 2047:
		anim = g.thingAnimRefsFromSeq("CELLA0")
	case 17:
		anim = g.thingAnimRefsFromSeq("CELPA0")
	case 8:
		anim = g.thingAnimRefsFromSeq("BPAKA0")
	case 2001:
		anim = g.thingAnimRefsFromSeq("SHOTA0")
	case 2002:
		anim = g.thingAnimRefsFromSeq("MGUNA0")
	case 2003:
		anim = g.thingAnimRefsFromSeq("LAUNA0")
	case 2004:
		anim = g.thingAnimRefsFromSeq("PLASA0")
	case 2005:
		anim = g.thingAnimRefsFromSeq("CSAWA0")
	case 2006:
		anim = g.thingAnimRefsFromSeq("BFUGA0")
	case 82:
		anim = g.thingAnimRefsFromSeq("SGN2A0")
	case 2035:
		anim = g.thingAnimRefsFromStates(
			thingAnimState{name: "BAR1A0", tics: 6},
			thingAnimState{name: "BAR1B0", tics: 6},
			thingAnimState{name: "BEXPA0", tics: 6},
		)
	case 44:
		anim = g.thingAnimRefsFromStates(
			thingAnimState{name: "TBLUA0", tics: 4}, thingAnimState{name: "TBLUB0", tics: 4},
			thingAnimState{name: "TBLUC0", tics: 4}, thingAnimState{name: "TBLUD0", tics: 4},
		)
	case 45:
		anim = g.thingAnimRefsFromStates(
			thingAnimState{name: "TGRNA0", tics: 4}, thingAnimState{name: "TGRNB0", tics: 4},
			thingAnimState{name: "TGRNC0", tics: 4}, thingAnimState{name: "TGRND0", tics: 4},
		)
	case 46:
		anim = g.thingAnimRefsFromStates(
			thingAnimState{name: "TREDA0", tics: 4}, thingAnimState{name: "TREDB0", tics: 4},
			thingAnimState{name: "TREDC0", tics: 4}, thingAnimState{name: "TREDD0", tics: 4},
		)
	case 55:
		anim = g.thingAnimRefsFromStates(
			thingAnimState{name: "SMRTA0", tics: 4}, thingAnimState{name: "SMRTB0", tics: 4},
			thingAnimState{name: "SMRTC0", tics: 4}, thingAnimState{name: "SMRTD0", tics: 4},
		)
	case 56:
		anim = g.thingAnimRefsFromStates(
			thingAnimState{name: "SMGTA0", tics: 4}, thingAnimState{name: "SMGTB0", tics: 4},
			thingAnimState{name: "SMGTC0", tics: 4}, thingAnimState{name: "SMGTD0", tics: 4},
		)
	case 57:
		anim = g.thingAnimRefsFromStates(
			thingAnimState{name: "SMBTA0", tics: 4}, thingAnimState{name: "SMBTB0", tics: 4},
			thingAnimState{name: "SMBTC0", tics: 4}, thingAnimState{name: "SMBTD0", tics: 4},
		)
	case 85:
		anim = g.thingAnimRefsFromStates(
			thingAnimState{name: "TLMPA0", tics: 4}, thingAnimState{name: "TLMPB0", tics: 4},
			thingAnimState{name: "TLMPC0", tics: 4}, thingAnimState{name: "TLMPD0", tics: 4},
		)
	case 86:
		anim = g.thingAnimRefsFromStates(
			thingAnimState{name: "TLP2A0", tics: 4}, thingAnimState{name: "TLP2B0", tics: 4},
			thingAnimState{name: "TLP2C0", tics: 4}, thingAnimState{name: "TLP2D0", tics: 4},
		)
	}
	g.worldThingAnimRefCache[typ] = anim
	return anim
}

func (g *game) initThingRenderState() {
	if g == nil || g.m == nil {
		return
	}
	if len(g.thingWorldAnimRef) != len(g.m.Things) {
		g.thingWorldAnimRef = make([]thingAnimRefState, len(g.m.Things))
	}
	for i, th := range g.m.Things {
		g.thingWorldAnimRef[i] = g.buildThingWorldAnimRef(th)
	}
}

func (g *game) buildThingWorldAnimRef(th mapdata.Thing) thingAnimRefState {
	if isMonster(th.Type) || isPlayerStart(th.Type) {
		return thingAnimRefState{}
	}
	if isBarrelThingType(th.Type) {
		return thingAnimRefState{}
	}
	return g.worldThingAnimRefs(th.Type)
}

type thingAnimState struct {
	name string
	tics int
}

func (g *game) pickThingStateSpriteName(tic int, states ...thingAnimState) string {
	return g.pickThingStateSpriteNameScaled(tic, 1, states...)
}

func (g *game) pickThingStateSpriteNameScaled(tickUnits, unitsPerTic int, states ...thingAnimState) string {
	if unitsPerTic <= 0 {
		unitsPerTic = 1
	}
	if len(states) == 0 {
		return ""
	}
	available := make([]thingAnimState, 0, len(states))
	for _, st := range states {
		name := strings.ToUpper(strings.TrimSpace(st.name))
		if name == "" {
			continue
		}
		if !g.spritePatchExists(name) {
			continue
		}
		available = append(available, thingAnimState{name: name, tics: st.tics})
	}
	if len(available) == 0 {
		return ""
	}
	if len(available) == 1 || available[0].tics <= 0 {
		return available[0].name
	}
	total := 0
	for _, st := range available {
		if st.tics > 0 {
			total += st.tics * unitsPerTic
		}
	}
	if total <= 0 {
		return available[0].name
	}
	t := tickUnits % total
	if t < 0 {
		t += total
	}
	acc := 0
	for _, st := range available {
		stepUnits := st.tics * unitsPerTic
		if stepUnits <= 0 {
			return st.name
		}
		acc += stepUnits
		if t < acc {
			return st.name
		}
	}
	return available[len(available)-1].name
}

func (g *game) pickAnimatedThingSpriteName(tic, frameTics int, seq ...string) string {
	return g.pickAnimatedThingSpriteNameScaled(tic, frameTics, 1, seq...)
}

func (g *game) pickAnimatedThingSpriteNameScaled(tickUnits, frameTics, unitsPerTic int, seq ...string) string {
	if unitsPerTic <= 0 {
		unitsPerTic = 1
	}
	if frameTics <= 0 || len(seq) == 0 {
		return ""
	}
	explicit := make([]string, 0, len(seq))
	seenExplicit := make(map[string]struct{}, len(seq))
	for _, raw := range seq {
		name := strings.ToUpper(strings.TrimSpace(raw))
		if name == "" {
			continue
		}
		if g.spritePatchExists(name) {
			if _, dup := seenExplicit[name]; !dup {
				seenExplicit[name] = struct{}{}
				explicit = append(explicit, name)
			}
		}
	}
	available := explicit
	if len(explicit) <= 1 {
		// For single-seed mappings (e.g. BON1A0), auto-expand to A..Z variants.
		available = make([]string, 0, len(seq)*4)
		seen := make(map[string]struct{}, len(seq)*8)
		for _, raw := range seq {
			name := strings.ToUpper(strings.TrimSpace(raw))
			if name == "" {
				continue
			}
			if g.spritePatchExists(name) {
				if _, dup := seen[name]; !dup {
					seen[name] = struct{}{}
					available = append(available, name)
				}
			}
			for _, ex := range g.expandThingSpriteFrames(name) {
				if _, dup := seen[ex]; dup {
					continue
				}
				seen[ex] = struct{}{}
				available = append(available, ex)
			}
		}
	}
	if len(available) == 0 {
		return ""
	}
	if len(available) == 1 {
		return available[0]
	}
	frameUnits := frameTics * unitsPerTic
	if frameUnits <= 0 {
		frameUnits = frameTics
	}
	return available[(tickUnits/frameUnits)%len(available)]
}

func (g *game) expandThingSpriteFrames(seed string) []string {
	name := strings.ToUpper(strings.TrimSpace(seed))
	if len(name) < 6 {
		return nil
	}
	if g.thingSpriteExpandCache == nil {
		g.thingSpriteExpandCache = make(map[string][]string, 256)
	}
	if out, ok := g.thingSpriteExpandCache[name]; ok {
		return out
	}
	// Only auto-expand conventional thing animations that start from A-frame seeds.
	// This avoids animating static corpse/decoration sprites like PLAYN0.
	if name[4] != 'A' {
		g.thingSpriteExpandCache[name] = nil
		return nil
	}
	rot := name[5]
	if rot < '0' || rot > '9' {
		g.thingSpriteExpandCache[name] = nil
		return nil
	}
	prefix := name[:4]
	if len(prefix) != 4 {
		g.thingSpriteExpandCache[name] = nil
		return nil
	}
	out := make([]string, 0, 8)
	seen := make(map[string]struct{}, 52)
	addFrames := func(r byte) {
		for frame := byte('A'); frame <= byte('Z'); frame++ {
			key := spriteFrameName(prefix, frame, r)
			if _, dup := seen[key]; dup {
				continue
			}
			if g.spritePatchExists(key) {
				seen[key] = struct{}{}
				out = append(out, key)
			}
		}
	}
	// Prefer non-rotating thing-style frames, then include rotation-1 variants.
	addFrames('0')
	addFrames('1')
	// If seed itself uses a different rotation digit, include that series too.
	if rot != '0' && rot != '1' {
		addFrames(rot)
	}
	for frame := byte('A'); frame <= byte('Z'); frame++ {
		key := spriteFrameName(prefix, frame, rot)
		if g.spritePatchExists(key) {
			if _, dup := seen[key]; !dup {
				seen[key] = struct{}{}
				out = append(out, key)
			}
		}
	}
	if len(out) <= 1 {
		g.thingSpriteExpandCache[name] = nil
		return nil
	}
	g.thingSpriteExpandCache[name] = out
	return out
}

func monsterShadeFactor(dist, near float64) float32 {
	n := (dist - near) / 1200.0
	if n < 0 {
		n = 0
	}
	if n > 1 {
		n = 1
	}
	return float32(1.0 - 0.55*n)
}

func (g *game) monsterSpriteTexture(name string) (*WallTexture, bool) {
	key := strings.ToUpper(strings.TrimSpace(name))
	if key == "" {
		return nil, false
	}
	p, ok := g.spritePatchTexture(key)
	if !ok || p.Width <= 0 || p.Height <= 0 {
		return nil, false
	}
	n := p.Width * p.Height
	if len(p.RGBA) != n*4 && (len(p.Indexed) != n || len(p.OpaqueMask) != n) {
		return nil, false
	}
	if tex, built := synthesizeIndexedSpriteTexture(*p); built {
		*p = tex
		if g.opts.SpritePatchBank != nil {
			g.opts.SpritePatchBank[key] = tex
		}
	}
	return p, true
}

func (g *game) monsterSpriteName(typ int16, tic int) string {
	frame := (tic / 8) & 3
	pick := func(a, b, c, d string) string {
		seq := [4]string{a, b, c, d}
		for i := 0; i < 4; i++ {
			name := seq[(frame+i)&3]
			if g.spritePatchExists(name) {
				return name
			}
		}
		return ""
	}
	switch typ {
	case 3004:
		return pick("POSSA1", "POSSB1", "POSSC1", "POSSD1")
	case 9:
		return pick("SPOSA1", "SPOSB1", "SPOSC1", "SPOSD1")
	case 84:
		return pick("SSWVA1", "SSWVB1", "SSWVC1", "SSWVD1")
	case 3001:
		return pick("TROOA1", "TROOB1", "TROOC1", "TROOD1")
	case 3002:
		return pick("SARGA1", "SARGB1", "SARGC1", "SARGD1")
	case 3006:
		return pick("SKULA1", "SKULB1", "SKULC1", "SKULD1")
	case 3005:
		return pick("HEADA1", "HEADB1", "HEADC1", "HEADD1")
	case 3003:
		return pick("BOSSA1", "BOSSB1", "BOSSC1", "BOSSD1")
	case 69:
		return pick("BOS2A1", "BOS2B1", "BOS2C1", "BOS2D1")
	case 64:
		return pick("VILEA1", "VILEB1", "VILEC1", "VILED1")
	case 65:
		return pick("CPOSA1", "CPOSB1", "CPOSC1", "CPOSD1")
	case 66:
		return pick("SKELA1", "SKELB1", "SKELC1", "SKELD1")
	case 67:
		return pick("FATTA1", "FATTB1", "FATTC1", "FATTD1")
	case 68:
		return pick("BSPIA1", "BSPIB1", "BSPIC1", "BSPID1")
	case 16:
		return pick("CYBRA1", "CYBRB1", "CYBRC1", "CYBRD1")
	case 7:
		return pick("SPIDA1", "SPIDB1", "SPIDC1", "SPIDD1")
	case 71:
		return pick("PAINA1", "PAINB1", "PAINC1", "PAIND1")
	default:
		return ""
	}
}

func monsterAttackFrameSeq(typ int16) []byte {
	switch typ {
	case 3004, 9:
		return monsterAttackSeqEFE
	case 65:
		return monsterAttackSeqEFEF
	case 3001:
		return monsterAttackSeqEFG
	case 3002, 58: // demon/spectre
		return monsterAttackSeqEFG
	case 3006:
		return monsterAttackSeqLostSoul
	case 3005:
		return monsterAttackSeqBCD
	case 3003, 69: // baron/knight
		return monsterAttackSeqEFG
	case 64: // arch-vile
		return monsterAttackSeqArchvile
	case 66: // revenant missile
		return monsterAttackSeqRevenant
	case 67: // mancubus
		return monsterAttackSeqMancubus
	case 16:
		return monsterAttackSeqCyber
	case 7:
		return monsterAttackSeqSpider
	case 68: // arachnotron
		return monsterAttackSeqSpider
	case 71: // pain elemental
		return monsterAttackSeqPain
	case 84: // wolf ss
		return monsterAttackSeqWolfSS
	default:
		return nil
	}
}

func monsterSpawnFrameSeq(typ int16) []byte {
	switch typ {
	case 3004, 9, 65, 3001, 3002, 58, 3003, 69, 3006:
		return monsterSpawnSeqAB
	case 3005:
		return monsterSpawnSeqA
	case 64, 66, 68, 16, 84:
		return monsterSpawnSeqAB
	case 67:
		return monsterSpawnSeqAB
	case 71:
		return monsterSpawnSeqA
	case 7:
		return monsterSpawnSeqAB
	default:
		return nil
	}
}

var (
	monsterAttackSeqEFE        = []byte{'E', 'F', 'E'}
	monsterAttackSeqEFEF       = []byte{'E', 'F', 'E', 'F'}
	monsterAttackSeqEFG        = []byte{'E', 'F', 'G'}
	monsterAttackSeqLostSoul   = []byte{'C', 'D', 'C', 'D'}
	monsterAttackSeqBCD        = []byte{'B', 'C', 'D'}
	monsterAttackSeqArchvile   = []byte{'G', 'G', 'H', 'I', 'J', 'K', 'L', 'M', 'N', 'O', 'P'}
	monsterAttackSeqRevenant   = []byte{'H', 'H', 'K', 'K'}
	monsterAttackSeqMancubus   = []byte{'G', 'H', 'I', 'G', 'H', 'I', 'G', 'H', 'I', 'G'}
	monsterAttackSeqCyber      = []byte{'E', 'F', 'E', 'F', 'E', 'F'}
	monsterAttackSeqSpider     = []byte{'A', 'G', 'H', 'H'}
	monsterAttackSeqPain       = []byte{'D', 'E', 'F', 'F'}
	monsterAttackSeqWolfSS     = []byte{'E', 'F', 'G', 'F', 'G', 'F'}
	monsterSpawnSeqAB          = []byte{'A', 'B'}
	monsterSpawnSeqA           = []byte{'A'}
	monsterSpawnTicsAB         = []int{10, 10}
	monsterSpawnTicsA          = []int{10}
	monsterSpawnTicsMancubus   = []int{15, 15}
	monsterAttackTicsZombieman = []int{10, 8, 8}
	monsterAttackTicsShotgun   = []int{10, 10, 10}
	monsterAttackTicsChaingun  = []int{10, 4, 4, 1}
	monsterAttackTicsImp       = []int{8, 8, 6}
	monsterAttackTicsDemon     = []int{8, 8, 8}
	monsterAttackTicsLostSoul  = []int{10, 4, 4, 4}
	monsterAttackTicsCaco      = []int{5, 5, 5}
	monsterAttackTicsArchvile  = []int{0, 10, 8, 8, 8, 8, 8, 8, 8, 8, 20}
	monsterAttackTicsRevenant  = []int{0, 10, 10, 10}
	monsterAttackTicsMancubus  = []int{20, 10, 5, 5, 10, 5, 5, 10, 5, 5}
	monsterAttackTicsSpider    = []int{20, 4, 4, 1}
	monsterAttackTicsArachn    = []int{20, 4, 4, 1}
	monsterAttackTicsCyber     = []int{6, 12, 12, 12, 12, 12}
	monsterAttackTicsPain      = []int{5, 5, 5, 0}
	monsterAttackTicsWolfSS    = []int{10, 10, 4, 6, 4, 1}
	monsterSeeTics4            = []int{4, 4, 4, 4, 4, 4, 4, 4}
	monsterSeeTics2            = []int{2, 2, 2, 2, 2, 2, 2, 2}
	monsterSeeTics3            = []int{3, 3, 3, 3, 3, 3, 3, 3}
	monsterSeeTics12x2         = []int{2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2}
	monsterSeeTics12x3         = []int{3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3}
	monsterSeeTics12x4         = []int{4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4}
	monsterSeeTics6x3          = []int{3, 3, 3, 3, 3, 3}
	monsterSeeTicsLostSoul     = []int{6, 6}
	monsterSeeSeqAABBCCDD      = []byte{'A', 'A', 'B', 'B', 'C', 'C', 'D', 'D'}
	monsterSeeSeqA             = []byte{'A'}
	monsterSeeSeqAB            = []byte{'A', 'B'}
	monsterSeeSeq12            = []byte{'A', 'A', 'B', 'B', 'C', 'C', 'D', 'D', 'E', 'E', 'F', 'F'}
	monsterSeeSeq6             = []byte{'A', 'A', 'B', 'B', 'C', 'C'}
	monsterPainSeqGG           = []byte{'G', 'G'}
	monsterPainSeqHH           = []byte{'H', 'H'}
	monsterPainSeqEE           = []byte{'E', 'E'}
	monsterPainSeqEEF          = []byte{'E', 'E', 'F'}
	monsterPainSeqQQ           = []byte{'Q', 'Q'}
	monsterPainSeqLL           = []byte{'L', 'L'}
	monsterPainSeqJJ           = []byte{'J', 'J'}
	monsterPainSeqG            = []byte{'G'}
	monsterPainSeqII           = []byte{'I', 'I'}
	monsterPainTics33          = []int{3, 3}
	monsterPainTics22          = []int{2, 2}
	monsterPainTics336         = []int{3, 3, 6}
	monsterPainTics10          = []int{10}
	monsterPainTics55          = []int{5, 5}
	monsterPainTics66          = []int{6, 6}
	monsterDeathTics5x5        = []int{5, 5, 5, 5, 5}
	monsterDeathTics5x6        = []int{5, 5, 5, 5, 5, -1}
	monsterDeathTics5x7        = []int{5, 5, 5, 5, 5, 5, -1}
	monsterDeathTics5x9        = []int{5, 5, 5, 5, 5, 5, 5, 5, -1}
	monsterDeathTicsImp        = []int{8, 8, 6, 6, 6}
	monsterDeathTicsDemon      = []int{8, 8, 4, 4, 4, 4}
	monsterDeathTicsLostSoul   = []int{6, 6, 6, 6, 6, 6}
	monsterDeathTics8x6        = []int{8, 8, 8, 8, 8, 8}
	monsterDeathTics8x7        = []int{8, 8, 8, 8, 8, 8, 8}
	monsterDeathTicsArch       = []int{7, 7, 7, 7, 7, 7, 7, 5, 5, -1}
	monsterDeathTics7x6        = []int{7, 7, 7, 7, 7, 7}
	monsterDeathTics6x10       = []int{6, 6, 6, 6, 6, 6, 6, 6, 6, -1}
	monsterDeathTicsCyber      = []int{10, 10, 10, 10, 10, 10, 10, 10, 30}
	monsterDeathTicsSpider     = []int{20, 10, 10, 10, 10, 10, 10, 10, 10, 30}
)

func monsterSpawnFrameTics(typ int16) []int {
	switch typ {
	case 3004, 9, 65, 3001, 3002, 58, 3003, 69, 3006:
		return monsterSpawnTicsAB
	case 3005:
		return monsterSpawnTicsA
	case 64, 66, 68, 16, 84:
		return monsterSpawnTicsAB
	case 67:
		return monsterSpawnTicsMancubus
	case 71:
		return monsterSpawnTicsA
	case 7:
		return monsterSpawnTicsAB
	default:
		return nil
	}
}

func monsterSeeFrameSeq(typ int16) []byte {
	switch typ {
	case 3004, 9, 65, 3001, 3002, 58, 3003, 69:
		return monsterSeeSeqAABBCCDD
	case 3006:
		return monsterSeeSeqAB
	case 3005:
		return monsterSeeSeqA
	case 64, 66, 68:
		return monsterSeeSeq12
	case 67:
		return monsterSeeSeq12
	case 71:
		return monsterSeeSeq6
	case 7:
		return monsterSeeSeq12
	case 84, 16:
		return monsterSeeSeqAABBCCDD
	default:
		return nil
	}
}

func monsterAttackFrameTics(typ int16) []int {
	switch typ {
	case 3004: // zombieman
		return monsterAttackTicsZombieman
	case 9: // shotgun guy
		return monsterAttackTicsShotgun
	case 65: // chaingunner
		return monsterAttackTicsChaingun
	case 3001: // imp
		return monsterAttackTicsImp
	case 3002, 58: // demon/spectre
		return monsterAttackTicsDemon
	case 3006: // lost soul
		return monsterAttackTicsLostSoul
	case 3005: // cacodemon
		return monsterAttackTicsCaco
	case 3003, 69: // baron/knight
		return monsterAttackTicsDemon
	case 64: // arch-vile
		return monsterAttackTicsArchvile
	case 66: // revenant
		return monsterAttackTicsRevenant
	case 67: // mancubus
		return monsterAttackTicsMancubus
	case 16: // cyberdemon
		return monsterAttackTicsCyber
	case 7: // spider mastermind
		return monsterAttackTicsSpider
	case 68: // arachnotron
		return monsterAttackTicsArachn
	case 71: // pain elemental
		return monsterAttackTicsPain
	case 84: // wolf ss
		return monsterAttackTicsWolfSS
	default:
		return nil
	}
}

func monsterSeeFrameTics(typ int16, fast bool) []int {
	switch typ {
	case 3004:
		tics := monsterSeeTics4
		if fast {
			tics = monsterSeeTics2
		}
		return tics
	case 9, 65:
		tics := monsterSeeTics3
		if fast {
			tics = monsterSeeTics2
		}
		return tics
	case 3001, 3003, 69:
		return monsterSeeTics3
	case 3002, 58:
		return monsterSeeTics2
	case 3006:
		return monsterSeeTicsLostSoul
	case 3005:
		return monsterPainTics33[:1]
	case 64, 66:
		return monsterSeeTics12x2
	case 67:
		return monsterSeeTics12x4
	case 68:
		return monsterSeeTics12x3
	case 71:
		return monsterSeeTics6x3
	case 7:
		return monsterSeeTics12x3
	case 84, 16:
		return monsterSeeTics3
	default:
		return nil
	}
}

func monsterAttackAnimTotalTics(typ int16) int {
	tics := monsterAttackFrameTics(typ)
	total := 0
	for _, t := range tics {
		if t > 0 {
			total += t
		}
	}
	return total
}

func monsterPainFrameSeq(typ int16) []byte {
	switch typ {
	case 3004:
		return monsterPainSeqGG
	case 9, 65:
		return monsterPainSeqGG
	case 3001:
		return monsterPainSeqHH
	case 3002, 58:
		return monsterPainSeqHH
	case 3006:
		return monsterPainSeqEE
	case 3005:
		return monsterPainSeqEEF
	case 3003, 69:
		return monsterPainSeqHH
	case 64:
		return monsterPainSeqQQ
	case 66:
		return monsterPainSeqLL
	case 67:
		return monsterPainSeqJJ
	case 16:
		return monsterPainSeqG
	case 7:
		return monsterPainSeqII
	case 68:
		return monsterPainSeqII
	case 71:
		return monsterPainSeqGG
	case 84:
		return monsterPainSeqHH
	default:
		return nil
	}
}

func monsterPainFrameTics(typ int16) []int {
	switch typ {
	case 3004:
		return monsterPainTics33
	case 9, 65:
		return monsterPainTics33
	case 3001:
		return monsterPainTics22
	case 3002, 58:
		return monsterPainTics22
	case 3006:
		return monsterPainTics33
	case 3005:
		return monsterPainTics336
	case 3003, 69:
		return monsterPainTics22
	case 64:
		return monsterPainTics55
	case 66:
		return monsterPainTics55
	case 67:
		return monsterPainTics33
	case 16:
		return monsterPainTics10
	case 7:
		return monsterPainTics33
	case 68:
		return monsterPainTics33
	case 71:
		return monsterPainTics66
	case 84:
		return monsterPainTics33
	default:
		return nil
	}
}

func monsterPainAnimTotalTics(typ int16) int {
	tics := monsterPainFrameTics(typ)
	total := 0
	for _, t := range tics {
		if t > 0 {
			total += t
		}
	}
	return total
}

var (
	monsterDeathFrames5x5    = []byte{'H', 'I', 'J', 'K', 'L'}
	monsterDeathFrames5x7    = []byte{'H', 'I', 'J', 'K', 'L', 'M', 'N'}
	monsterDeathFramesImp    = []byte{'I', 'J', 'K', 'L', 'M'}
	monsterDeathFramesDemon  = []byte{'I', 'J', 'K', 'L', 'M', 'N'}
	monsterDeathFramesLost   = []byte{'F', 'G', 'H', 'I', 'J', 'K'}
	monsterDeathFrames8x6    = []byte{'G', 'H', 'I', 'J', 'K', 'L'}
	monsterDeathFrames8x7    = []byte{'I', 'J', 'K', 'L', 'M', 'N', 'O'}
	monsterDeathFramesCyber  = []byte{'H', 'I', 'J', 'K', 'L', 'M', 'N', 'O', 'P'}
	monsterDeathFramesSpider = []byte{'J', 'K', 'L', 'M', 'N', 'O', 'P', 'Q', 'R', 'S'}
	monsterDeathFramesArch   = []byte{'Q', 'R', 'S', 'T', 'U', 'V', 'W', 'X', 'Y', 'Z'}
	monsterDeathFramesRev    = []byte{'L', 'M', 'N', 'O', 'P', 'Q'}
	monsterDeathFramesManc   = []byte{'K', 'L', 'M', 'N', 'O', 'P', 'Q', 'R', 'S', 'T'}
	monsterDeathFramesArachn = []byte{'J', 'K', 'L', 'M', 'N', 'O', 'P'}
	monsterDeathFramesPain   = []byte{'H', 'I', 'J', 'K', 'L', 'M'}
	monsterXDeathFrames5x9   = []byte{'M', 'N', 'O', 'P', 'Q', 'R', 'S', 'T', 'U'}
	monsterXDeathFrames5x6   = []byte{'O', 'P', 'Q', 'R', 'S', 'T'}
	monsterXDeathFramesWolf  = []byte{'N', 'O', 'P', 'Q', 'R', 'S', 'T', 'U', 'V'}
)

func monsterDeathFrameSeq(typ int16) []byte {
	switch typ {
	case 3004:
		return monsterDeathFrames5x5
	case 9:
		return monsterDeathFrames5x5
	case 3001:
		return monsterDeathFramesImp
	case 3002, 58:
		return monsterDeathFramesDemon
	case 65:
		return monsterDeathFrames5x7
	case 3006:
		return monsterDeathFramesLost
	case 3005:
		return monsterDeathFrames8x6
	case 3003, 69:
		return monsterDeathFrames8x7
	case 64:
		return monsterDeathFramesArch
	case 66:
		return monsterDeathFramesRev
	case 67:
		return monsterDeathFramesManc
	case 16:
		return monsterDeathFramesCyber
	case 7:
		return monsterDeathFramesSpider
	case 68:
		return monsterDeathFramesArachn
	case 71:
		return monsterDeathFramesPain
	case 84:
		return monsterDeathFramesImp
	default:
		return nil
	}
}

func monsterXDeathFrameSeq(typ int16) []byte {
	switch typ {
	case 3004, 9:
		return monsterXDeathFrames5x9
	case 65:
		return monsterXDeathFrames5x6
	case 84:
		return monsterXDeathFramesWolf
	default:
		return nil
	}
}

func monsterDeathFrameTics(typ int16) []int {
	switch typ {
	case 3004:
		return monsterDeathTics5x5
	case 9:
		return monsterDeathTics5x5
	case 3001:
		return monsterDeathTicsImp
	case 3002, 58:
		return monsterDeathTicsDemon
	case 65:
		return monsterDeathTics5x7
	case 3006:
		return monsterDeathTicsLostSoul
	case 3005:
		return monsterDeathTics8x6
	case 3003, 69:
		return monsterDeathTics8x7
	case 64:
		return monsterDeathTicsArch
	case 66:
		return monsterDeathTics7x6
	case 67:
		return monsterDeathTics6x10
	case 16:
		return monsterDeathTicsCyber
	case 7:
		return monsterDeathTicsSpider
	case 68:
		return monsterDeathTics7x6
	case 71:
		return monsterDeathTics8x6
	case 84:
		return monsterDeathTics5x5
	default:
		return nil
	}
}

func monsterXDeathFrameTics(typ int16) []int {
	switch typ {
	case 3004, 9, 84:
		return monsterDeathTics5x9
	case 65:
		return monsterDeathTics5x6
	default:
		return nil
	}
}

func monsterHasXDeath(typ int16) bool {
	return len(monsterXDeathFrameTics(typ)) > 0
}

func monsterDeathFrameSeqForMode(typ int16, xdeath bool) []byte {
	if xdeath {
		if seq := monsterXDeathFrameSeq(typ); len(seq) > 0 {
			return seq
		}
	}
	return monsterDeathFrameSeq(typ)
}

func monsterDeathFrameTicsForMode(typ int16, xdeath bool) []int {
	if xdeath {
		if tics := monsterXDeathFrameTics(typ); len(tics) > 0 {
			return tics
		}
	}
	return monsterDeathFrameTics(typ)
}

func monsterDeathSoundDelayTics(typ int16) int {
	// Doom plays death sounds on A_Scream, which is usually the 2nd death frame.
	switch typ {
	case 3004, 9, 65, 84:
		return 5
	case 3001, 3002, 58, 3003, 69, 3005:
		return 8
	case 3006:
		return 6
	case 64:
		return 7
	case 67:
		return 6
	case 71:
		return 8
	case 16:
		return 10
	case 7:
		// Spider mastermind screams on the first death frame.
		return 0
	default:
		return 0
	}
}

func monsterDeathAnimTotalTics(typ int16) int {
	return monsterDeathAnimTotalTicsForMode(typ, false)
}

func monsterDeathAnimTotalTicsForMode(typ int16, xdeath bool) int {
	tics := monsterDeathFrameTicsForMode(typ, xdeath)
	total := 0
	for _, t := range tics {
		if t > 0 {
			total += t
		}
	}
	return total
}

func (g *game) monsterFrameLetter(i int, th mapdata.Thing, tic int) byte {
	if i >= 0 && i < len(g.thingDead) && g.thingDead[i] {
		xdeath := i >= 0 && i < len(g.thingXDeath) && g.thingXDeath[i]
		seq := monsterDeathFrameSeqForMode(th.Type, xdeath)
		frameTics := monsterDeathFrameTicsForMode(th.Type, xdeath)
		if len(seq) > 0 && len(seq) == len(frameTics) {
			total := monsterDeathAnimTotalTicsForMode(th.Type, xdeath)
			elapsed := total
			if i < len(g.thingDeathTics) && g.thingDeathTics[i] > 0 {
				elapsed = total - g.thingDeathTics[i]
			}
			if elapsed < 0 {
				elapsed = 0
			}
			acc := 0
			for fi, ft := range frameTics {
				if ft <= 0 {
					continue
				}
				acc += ft
				if elapsed < acc {
					return seq[fi]
				}
			}
			return seq[len(seq)-1]
		}
	}
	if monsterUsesExactDoomStateMachine(th.Type) && i >= 0 && i < len(g.thingDoomState) {
		if frame, ok := monsterDoomStateFrameLetter(g.thingDoomState[i]); ok {
			return frame
		}
	}
	if i >= 0 && i < len(g.thingPainTics) && g.thingPainTics[i] > 0 {
		seq := monsterPainFrameSeq(th.Type)
		frameTics := monsterPainFrameTics(th.Type)
		if len(seq) > 0 && len(seq) == len(frameTics) {
			total := monsterPainAnimTotalTics(th.Type)
			elapsed := total - g.thingPainTics[i]
			if elapsed < 0 {
				elapsed = 0
			}
			acc := 0
			for fi, ft := range frameTics {
				if ft <= 0 {
					continue
				}
				acc += ft
				if elapsed < acc {
					return seq[fi]
				}
			}
			return seq[len(seq)-1]
		}
	}
	if i >= 0 && i < len(g.thingAttackTics) && g.thingAttackTics[i] > 0 {
		seq := monsterAttackFrameSeq(th.Type)
		frameTics := monsterAttackFrameTics(th.Type)
		if len(seq) > 0 && len(seq) == len(frameTics) {
			total := monsterAttackAnimTotalTics(th.Type)
			elapsed := total - g.thingAttackTics[i]
			if elapsed < 0 {
				elapsed = 0
			}
			acc := 0
			for fi, ft := range frameTics {
				if ft <= 0 {
					continue
				}
				acc += ft
				if elapsed < acc {
					return seq[fi]
				}
			}
			return seq[len(seq)-1]
		}
	}
	if i >= 0 && i < len(g.thingState) && i < len(g.thingStatePhase) {
		phase := g.thingStatePhase[i]
		switch g.thingState[i] {
		case monsterStateSpawn:
			seq := monsterSpawnFrameSeq(th.Type)
			if len(seq) > 0 {
				if phase < 0 || phase >= len(seq) {
					phase = 0
				}
				return seq[phase]
			}
		case monsterStateSee:
			seq := monsterSeeFrameSeq(th.Type)
			if len(seq) > 0 {
				if phase < 0 || phase >= len(seq) {
					phase = 0
				}
				return seq[phase]
			}
		}
	}
	return byte('A' + ((tic / 8) & 3))
}

func (g *game) monsterSpriteNameForView(i int, th mapdata.Thing, tic int, viewX, viewY float64) (string, bool) {
	if ref, flip, ok := g.monsterSpriteRefForView(i, th, tic, viewX, viewY); ok && ref != nil {
		return ref.key, flip
	}
	prefix, ok := monsterSpritePrefix(th.Type)
	if !ok {
		return g.monsterSpriteName(th.Type, tic), false
	}
	frameLetter := g.monsterFrameLetter(i, th, tic)
	if i >= 0 && i < len(g.thingDead) && g.thingDead[i] {
		name0 := spriteFrameName(prefix, frameLetter, '0')
		if g.spritePatchExists(name0) {
			return name0, false
		}
	}
	fx, fy := g.thingPosFixed(i, th)
	rot := monsterSpriteRotationIndexAt(g.thingWorldAngle(i, th), float64(fx)/fracUnit, float64(fy)/fracUnit, viewX, viewY)
	if name, flip, ok := g.monsterSpriteRotFrame(prefix, frameLetter, rot); ok {
		return name, flip
	}
	if name, flip, ok := g.monsterSpriteRotFrame(prefix, frameLetter, 1); ok {
		return name, flip
	}
	for _, fallback := range monsterFallbackFrameLetters(frameLetter) {
		if name, flip, ok := g.monsterSpriteRotFrame(prefix, fallback, rot); ok {
			return name, flip
		}
		if name, flip, ok := g.monsterSpriteRotFrame(prefix, fallback, 1); ok {
			return name, flip
		}
	}
	return g.monsterSpriteName(th.Type, tic), false
}

func (g *game) monsterSpriteRefForView(i int, th mapdata.Thing, tic int, viewX, viewY float64) (*spriteRenderRef, bool, bool) {
	prefix, ok := monsterSpritePrefix(th.Type)
	if !ok {
		ref, ok := g.spriteRenderRef(g.monsterSpriteName(th.Type, tic))
		return ref, false, ok
	}
	frameLetter := g.monsterFrameLetter(i, th, tic)
	if i >= 0 && i < len(g.thingDead) && g.thingDead[i] {
		if ref, ok := g.monsterFrameRenderRef(prefix, frameLetter, 0); ok {
			return ref, false, true
		}
	}
	fx, fy := g.thingPosFixed(i, th)
	rot := monsterSpriteRotationIndexAt(g.thingWorldAngle(i, th), float64(fx)/fracUnit, float64(fy)/fracUnit, viewX, viewY)
	if ref, flip, ok := g.monsterFrameRenderRefRot(prefix, frameLetter, rot); ok {
		return ref, flip, true
	}
	if ref, flip, ok := g.monsterFrameRenderRefRot(prefix, frameLetter, 1); ok {
		return ref, flip, true
	}
	for _, fallback := range monsterFallbackFrameLetters(frameLetter) {
		if ref, flip, ok := g.monsterFrameRenderRefRot(prefix, fallback, rot); ok {
			return ref, flip, true
		}
		if ref, flip, ok := g.monsterFrameRenderRefRot(prefix, fallback, 1); ok {
			return ref, flip, true
		}
	}
	ref, ok := g.spriteRenderRef(g.monsterSpriteName(th.Type, tic))
	return ref, false, ok
}

func monsterSpriteRotationIndex(th mapdata.Thing, viewX, viewY float64) int {
	return monsterSpriteRotationIndexAt(thingDegToWorldAngle(th.Angle), float64(th.X), float64(th.Y), viewX, viewY)
}

func monsterUsesShadow(typ int16) bool {
	switch typ {
	case 58: // spectre
		return true
	default:
		return false
	}
}

func monsterSpritePrefix(typ int16) (string, bool) {
	switch typ {
	case 3004:
		return "POSS", true
	case 9:
		return "SPOS", true
	case 3001:
		return "TROO", true
	case 3002, 58:
		return "SARG", true
	case 3006:
		return "SKUL", true
	case 3005:
		return "HEAD", true
	case 3003:
		return "BOSS", true
	case 69:
		return "BOS2", true
	case 64:
		return "VILE", true
	case 65:
		return "CPOS", true
	case 66:
		return "SKEL", true
	case 67:
		return "FATT", true
	case 68:
		return "BSPI", true
	case 16:
		return "CYBR", true
	case 7:
		return "SPID", true
	case 71:
		return "PAIN", true
	case 84:
		return "SSWV", true
	default:
		return "", false
	}
}

func monsterSpriteRotationIndexAt(worldAngle uint32, thingX, thingY, viewX, viewY float64) int {
	facing := normalizeDeg360(float64(worldAngle) * (360.0 / 4294967296.0))
	angToView := math.Atan2(viewY-thingY, viewX-thingX) * (180.0 / math.Pi)
	angToView = normalizeDeg360(angToView)
	delta := normalizeDeg360(angToView - facing)
	return int(math.Floor((delta+22.5)/45.0))%8 + 1
}

func (g *game) monsterSpriteRotFrame(prefix string, frame byte, rot int) (string, bool, bool) {
	if rot < 1 || rot > 8 {
		return "", false, false
	}
	rotDigit := byte('0' + rot)
	// Prefer exact per-rotation patch if present.
	name := spriteFrameName(prefix, frame, rotDigit)
	if g.spritePatchExists(name) {
		return name, false, true
	}
	opp := 10 - rot
	if rot == 1 {
		opp = 5
	} else if rot == 5 {
		opp = 1
	}
	oppDigit := byte('0' + opp)
	// Doom paired-rotation patch, e.g. TROOA2A8.
	pairA := spriteFramePairName(prefix, frame, rotDigit, oppDigit)
	if g.spritePatchExists(pairA) {
		return pairA, false, true
	}
	// Reverse order pair (some content uses the opposite declaration order).
	pairB := spriteFramePairName(prefix, frame, oppDigit, rotDigit)
	if g.spritePatchExists(pairB) {
		return pairB, true, true
	}
	return "", false, false
}

func (g *game) monsterFrameRenderRef(prefix string, frame byte, rot int) (*spriteRenderRef, bool) {
	key := monsterFrameRenderKey{prefix: prefix, frame: frame, rot: uint8(rot)}
	if g.monsterFrameRenderCache == nil {
		g.monsterFrameRenderCache = make(map[monsterFrameRenderKey]monsterFrameRenderEntry, 512)
	}
	if cached, ok := g.monsterFrameRenderCache[key]; ok {
		return cached.ref, cached.ref != nil
	}
	var ref *spriteRenderRef
	if rot == 0 {
		ref, _ = g.spriteRenderRef(spriteFrameName(prefix, frame, '0'))
	} else if name, flip, ok := g.monsterSpriteRotFrame(prefix, frame, rot); ok {
		var found bool
		ref, found = g.spriteRenderRef(name)
		if found {
			g.monsterFrameRenderCache[key] = monsterFrameRenderEntry{ref: ref, flip: flip}
			return ref, true
		}
	}
	g.monsterFrameRenderCache[key] = monsterFrameRenderEntry{ref: ref}
	return ref, ref != nil
}

func (g *game) monsterFrameRenderRefRot(prefix string, frame byte, rot int) (*spriteRenderRef, bool, bool) {
	if rot < 1 || rot > 8 {
		return nil, false, false
	}
	key := monsterFrameRenderKey{prefix: prefix, frame: frame, rot: uint8(rot)}
	if g.monsterFrameRenderCache == nil {
		g.monsterFrameRenderCache = make(map[monsterFrameRenderKey]monsterFrameRenderEntry, 512)
	}
	if cached, ok := g.monsterFrameRenderCache[key]; ok {
		return cached.ref, cached.flip, cached.ref != nil
	}
	name, flip, ok := g.monsterSpriteRotFrame(prefix, frame, rot)
	if !ok {
		g.monsterFrameRenderCache[key] = monsterFrameRenderEntry{}
		return nil, false, false
	}
	ref, ok := g.spriteRenderRef(name)
	if !ok {
		g.monsterFrameRenderCache[key] = monsterFrameRenderEntry{}
		return nil, false, false
	}
	g.monsterFrameRenderCache[key] = monsterFrameRenderEntry{ref: ref, flip: flip}
	return ref, flip, true
}

func spriteFrameName(prefix string, frame, rot byte) string {
	if len(prefix) != 4 {
		return ""
	}
	var name [6]byte
	name[0] = prefix[0]
	name[1] = prefix[1]
	name[2] = prefix[2]
	name[3] = prefix[3]
	name[4] = frame
	name[5] = rot
	return string(name[:])
}

func spriteFramePairName(prefix string, frame, rotA, rotB byte) string {
	if len(prefix) != 4 {
		return ""
	}
	var name [8]byte
	name[0] = prefix[0]
	name[1] = prefix[1]
	name[2] = prefix[2]
	name[3] = prefix[3]
	name[4] = frame
	name[5] = rotA
	name[6] = frame
	name[7] = rotB
	return string(name[:])
}

func monsterFallbackFrameLetters(frame byte) []byte {
	if frame <= 'A' || frame > 'Z' {
		return nil
	}
	out := make([]byte, 0, int(frame-'A'))
	for f := frame - 1; f >= 'A'; f-- {
		out = append(out, f)
		if f == 'A' {
			break
		}
	}
	return out
}

func normalizeDeg360(deg float64) float64 {
	for deg < 0 {
		deg += 360
	}
	for deg >= 360 {
		deg -= 360
	}
	return deg
}

func (g *game) playerEyeZ() float64 {
	if g.playerViewZ == 0 {
		return float64(g.p.z)/fracUnit + 41.0
	}
	return float64(g.playerViewZ) / fracUnit
}

func monsterSpriteFullBright(name string) bool {
	return doomSourceSpriteFullBright(name)
}

func worldThingSpriteFullBright(name string) bool {
	return doomSourceSpriteFullBright(name)
}

func monsterRenderHeight(typ int16) float64 {
	switch typ {
	case 3002:
		return 56
	case 3006:
		return 56
	case 3005:
		return 56
	case 3003, 69, 67, 68:
		return 64
	case 16:
		return 110
	case 7:
		return 100
	default:
		return 56
	}
}

func (g *game) drawUseSpecialLines(screen *ebiten.Image) {
	segs := g.useSpecialSegScratch[:0]
	for _, li := range g.visibleLineIndices() {
		if li < 0 || li >= len(g.lineSpecial) || !buttonHighlightEligible(g.lineSpecial[li]) {
			continue
		}
		pi := g.physForLine[li]
		if pi < 0 || pi >= len(g.lines) {
			continue
		}
		ld := g.m.Linedefs[li]
		d := g.linedefDecision(ld)
		if !d.Visible {
			continue
		}
		pl := g.lines[pi]
		x1, y1 := g.worldToScreen(float64(pl.x1)/fracUnit, float64(pl.y1)/fracUnit)
		x2, y2 := g.worldToScreen(float64(pl.x2)/fracUnit, float64(pl.y2)/fracUnit)
		segs = append(segs, mapview.Segment{
			X1: x1, Y1: y1, X2: x2, Y2: y2, Width: 2.4, Color: wallUseSpecial,
		})
	}
	g.useSpecialSegScratch = segs
	mapview.DrawSegments(screen, segs, true)
}

func buttonHighlightEligible(special uint16) bool {
	if special == 0 {
		return false
	}
	info := mapdata.LookupLineSpecial(special)
	return info.Trigger == mapdata.TriggerUse
}

func (g *game) drawDeathOverlay(screen *ebiten.Image) {
	hud.DrawDeathOverlay(screen, hud.DeathOverlayInputs{
		ViewW: g.viewW,
		ViewH: g.viewH,
	}, g.huTextWidth, g.drawHUTextAt)
}

func (g *game) drawFlashOverlay(screen *ebiten.Image) {
	hud.DrawFlashOverlay(
		screen,
		g.viewW,
		g.viewH,
		g.statusDamageCount,
		g.statusBonusCount,
		g.inventory.StrengthCount,
		g.inventory.RadSuitTics,
	)
}

func (g *game) setSkyOutputSize(w, h int) {
	if w < 1 {
		w = 1
	}
	if h < 1 {
		h = 1
	}
	if g.skyOutputW == w && g.skyOutputH == h {
		return
	}
	g.skyOutputW = w
	g.skyOutputH = h
	if g.opts.GPUSky && g.opts.SourcePortMode {
		g.resetSkyLayerPipeline(false)
	}
}

func (g *game) Layout(outsideWidth, outsideHeight int) (int, int) {
	if g.opts.SourcePortMode {
		w := max(outsideWidth, 1)
		h := max(outsideHeight, 1)
		w, h = clampSourcePortGameSizeForPlatform(w, h, isWASMBuild())
		if w != g.viewW || h != g.viewH {
			g.viewW = w
			g.viewH = h
			_, _, worldW, worldH := boundsViewMetrics(g.bounds)
			g.State.Refit(worldW, worldH, g.viewW, g.viewH, doomInitialZoomMul)
			// Resolution changes only invalidate sky projection/image caches.
			g.resetSkyLayerPipeline(false)
			g.mouseLookSet = false
			g.mouseLookSuppressTicks = detailMouseSuppressTicks
			g.syncRenderState()
		}
		return g.viewW, g.viewH
	}
	w := max(outsideWidth, 1)
	h := max(outsideHeight, 1)
	if w != g.viewW || h != g.viewH {
		g.viewW = w
		g.viewH = h
		_, _, worldW, worldH := boundsViewMetrics(g.bounds)
		g.State.Refit(worldW, worldH, g.viewW, g.viewH, doomInitialZoomMul)
		g.mouseLookSet = false
		g.mouseLookSuppressTicks = detailMouseSuppressTicks
		g.syncRenderState()
	}
	return g.viewW, g.viewH
}

func (g *game) worldToScreen(x, y float64) (float64, float64) {
	return g.State.WorldToScreenViewport(g.viewport(), x, y)
}

func (g *game) screenToWorld(sx, sy float64) (float64, float64) {
	return g.State.ScreenToWorldViewport(g.viewport(), sx, sy)
}

func (g *game) ensureMapFloorLayer() {
	need := g.viewW * g.viewH * 4
	if g.mapFloorLayer == nil || g.mapFloorW != g.viewW || g.mapFloorH != g.viewH || len(g.mapFloorPix) != need {
		g.mapFloorLayer = newDebugImageWithOptions("framebuffer:map-floor", image.Rect(0, 0, g.viewW, g.viewH), &ebiten.NewImageOptions{
			Unmanaged: true,
		})
		g.mapFloorPix = make([]byte, need)
		g.mapFloorW = g.viewW
		g.mapFloorH = g.viewH
	}
}

func (g *game) ensureWallLayer() {
	need := g.viewW * g.viewH * 4
	if g.wallLayer == nil || g.wallW != g.viewW || g.wallH != g.viewH || len(g.wallPix) != need {
		g.wallLayer = newDebugImageWithOptions("framebuffer:wall", image.Rect(0, 0, g.viewW, g.viewH), &ebiten.NewImageOptions{
			Unmanaged: true,
		})
		g.wallPix = make([]byte, need)
		g.wallW = g.viewW
		g.wallH = g.viewH
	}
	if len(g.wallPix) >= 4 {
		g.wallPix32 = unsafe.Slice((*uint32)(unsafe.Pointer(unsafe.SliceData(g.wallPix))), len(g.wallPix)/4)
	} else {
		g.wallPix32 = g.wallPix32[:0]
	}
	coverNeed := (g.viewW*g.viewH + 63) >> 6
	if len(g.cutoutCoverageBits) != coverNeed {
		g.cutoutCoverageBits = make([]uint64, coverNeed)
	}
}

func (g *game) ensure3DFrameBuffers() ([]int, []int, []int, []int) {
	w := g.viewW
	h := g.viewH
	if w <= 0 {
		w = 1
	}
	if h <= 0 {
		h = 1
	}
	if g.buffers3DW != w || g.buffers3DH != h ||
		len(g.wallTop3D) != w || len(g.wallBottom3D) != w ||
		len(g.ceilingClip3D) != w || len(g.floorClip3D) != w {
		g.wallTop3D = make([]int, w)
		g.wallBottom3D = make([]int, w)
		g.ceilingClip3D = make([]int, w)
		g.floorClip3D = make([]int, w)
		g.buffers3DW = w
		g.buffers3DH = h
	}
	if len(g.wallDepthQCol) != w {
		g.wallDepthQCol = make([]uint16, w)
	}
	if len(g.wallDepthTopCol) != w {
		g.wallDepthTopCol = make([]int, w)
	}
	if len(g.wallDepthBottomCol) != w {
		g.wallDepthBottomCol = make([]int, w)
	}
	if len(g.wallDepthClosedCol) != w {
		g.wallDepthClosedCol = make([]bool, w)
	}
	if len(g.maskedClipCols) != w {
		g.maskedClipCols = make([][]scene.MaskedClipSpan, w)
	}
	if len(g.maskedClipFirstDepthQ) != w {
		g.maskedClipFirstDepthQ = make([]uint16, w)
	}
	if len(g.maskedClipLastDepthQ) != w {
		g.maskedClipLastDepthQ = make([]uint16, w)
	}
	for i := 0; i < w; i++ {
		g.wallTop3D[i] = h
		g.wallBottom3D[i] = -1
		g.ceilingClip3D[i] = -1
		g.floorClip3D[i] = h
		g.wallDepthQCol[i] = 0xFFFF
		g.wallDepthTopCol[i] = h
		g.wallDepthBottomCol[i] = -1
		g.wallDepthClosedCol[i] = false
		if len(g.maskedClipCols[i]) != 0 {
			g.maskedClipCols[i] = g.maskedClipCols[i][:0]
		}
		g.maskedClipFirstDepthQ[i] = 0
		g.maskedClipLastDepthQ[i] = 0
	}
	return g.wallTop3D, g.wallBottom3D, g.ceilingClip3D, g.floorClip3D
}

func (g *game) beginPlane3DFrame(viewW int) []*plane3DVisplane {
	if g.plane3DPoolViewW != viewW {
		g.plane3DPool = g.plane3DPool[:0]
		g.plane3DPoolUsed = 0
		g.plane3DPoolViewW = viewW
	}
	if g.plane3DVisGen == ^uint64(0) {
		g.plane3DVisGen = 1
	} else {
		g.plane3DVisGen++
	}
	g.plane3DPoolUsed = 0
	g.plane3DOrder = g.plane3DOrder[:0]
	return g.plane3DOrder
}

func (g *game) beginSolid3DFrame() []solidSpan {
	g.solid3DBuf = g.solid3DBuf[:0]
	return g.solid3DBuf
}

func (g *game) ensurePlaneSpanScratch(n int) ([][]plane3DSpan, [][]int) {
	if n <= 0 {
		g.plane3DSpanScratch = g.plane3DSpanScratch[:0]
		g.plane3DSpanStartScratch = g.plane3DSpanStartScratch[:0]
		return g.plane3DSpanScratch, g.plane3DSpanStartScratch
	}
	if cap(g.plane3DSpanScratch) < n {
		g.plane3DSpanScratch = make([][]plane3DSpan, n)
	} else {
		g.plane3DSpanScratch = g.plane3DSpanScratch[:n]
	}
	if cap(g.plane3DSpanStartScratch) < n {
		g.plane3DSpanStartScratch = make([][]int, n)
	} else {
		g.plane3DSpanStartScratch = g.plane3DSpanStartScratch[:n]
	}
	for i := 0; i < n; i++ {
		if i < len(g.plane3DSpanScratch) {
			g.plane3DSpanScratch[i] = g.plane3DSpanScratch[i][:0]
		}
		if i < len(g.plane3DSpanStartScratch) && g.viewH > 0 {
			if cap(g.plane3DSpanStartScratch[i]) < g.viewH {
				g.plane3DSpanStartScratch[i] = make([]int, g.viewH)
			} else {
				g.plane3DSpanStartScratch[i] = g.plane3DSpanStartScratch[i][:g.viewH]
				clear(g.plane3DSpanStartScratch[i])
			}
		}
	}
	return g.plane3DSpanScratch, g.plane3DSpanStartScratch
}

func (g *game) ensurePlaneSpanWorkScratch(n int) [][]planeSpanWorkItem {
	if n <= 0 {
		g.plane3DSpanWorkScratch = g.plane3DSpanWorkScratch[:0]
		return g.plane3DSpanWorkScratch
	}
	if cap(g.plane3DSpanWorkScratch) < n {
		g.plane3DSpanWorkScratch = make([][]planeSpanWorkItem, n)
	} else {
		g.plane3DSpanWorkScratch = g.plane3DSpanWorkScratch[:n]
	}
	for i := range g.plane3DSpanWorkScratch {
		g.plane3DSpanWorkScratch[i] = g.plane3DSpanWorkScratch[i][:0]
	}
	return g.plane3DSpanWorkScratch
}

func (g *game) acquirePlane3DVisplane(key plane3DKey, start, stop, viewW int) *plane3DVisplane {
	if g.plane3DPoolViewW != viewW {
		g.plane3DPool = g.plane3DPool[:0]
		g.plane3DPoolUsed = 0
		g.plane3DPoolViewW = viewW
	}
	var pl *plane3DVisplane
	if g.plane3DPoolUsed < len(g.plane3DPool) {
		pl = g.plane3DPool[g.plane3DPoolUsed]
	} else {
		pl = newPlane3DVisplane(key, start, stop, viewW)
		g.plane3DPool = append(g.plane3DPool, pl)
	}
	g.plane3DPoolUsed++
	pl.key = key
	pl.minX = start
	pl.maxX = stop
	for i := range pl.top {
		pl.top[i] = plane3DUnset
		pl.bottom[i] = plane3DUnset
	}
	return pl
}

func (g *game) ensurePlane3DForRangeCached(key plane3DKey, start, stop, viewW int) (*plane3DVisplane, bool) {
	if start > stop {
		start, stop = stop, start
	}
	if start < 0 {
		start = 0
	}
	if stop >= viewW {
		stop = viewW - 1
	}
	if start > stop {
		return nil, false
	}
	b := g.plane3DVisBuckets[key]
	if b.gen != g.plane3DVisGen {
		b.gen = g.plane3DVisGen
		b.list = b.list[:0]
	}
	for _, pl := range b.list {
		intrl := start
		if pl.minX > intrl {
			intrl = pl.minX
		}
		intrh := stop
		if pl.maxX < intrh {
			intrh = pl.maxX
		}
		conflict := false
		if intrl <= intrh {
			for x := intrl; x <= intrh; x++ {
				ix := x + 1
				if ix >= 0 && ix < len(pl.top) && pl.top[ix] != plane3DUnset {
					conflict = true
					break
				}
			}
		}
		if conflict {
			continue
		}
		if start < pl.minX {
			pl.minX = start
		}
		if stop > pl.maxX {
			pl.maxX = stop
		}
		g.plane3DVisBuckets[key] = b
		return pl, false
	}
	pl := g.acquirePlane3DVisplane(key, start, stop, viewW)
	b.list = append(b.list, pl)
	g.plane3DVisBuckets[key] = b
	return pl, true
}

func (g *game) wallTexture(name string) (*WallTexture, bool) {
	key, _ := g.resolveAnimatedWallSample(name)
	key = g.resolveAnimatedWallFrameAvailable(name, key, 1)
	tex, ok := g.wallTexturePtrResolvedKey(key)
	if !ok || tex.Width <= 0 || tex.Height <= 0 || len(tex.RGBA) != tex.Width*tex.Height*4 {
		return nil, false
	}
	return tex, true
}

func (g *game) wallTextureBlend(name string, sidedef int, slot switchTextureSlot) (wallTextureBlendSample, bool) {
	if blend, ok := g.switchTextureBlendSample(sidedef, slot, name); ok {
		return blend, true
	}
	sample := g.textureBlendSample(name, g.wallTextureAnimRefs)
	sample.fromKey = g.resolveAnimatedWallFrameAvailable(name, sample.fromKey, 1)
	from, ok := g.wallTexturePtrResolvedKey(sample.fromKey)
	if !ok || from == nil || from.Width <= 0 || from.Height <= 0 || len(from.RGBA) != from.Width*from.Height*4 {
		return wallTextureBlendSample{}, false
	}
	out := wallTextureBlendSample{from: from}
	if sample.alpha == 0 || sample.toKey == "" {
		return out, true
	}
	sample.toKey = g.resolveAnimatedWallFrameAvailable(name, sample.toKey, 1)
	if sample.toKey == "" || sample.toKey == sample.fromKey {
		return out, true
	}
	to, ok := g.wallTexturePtrResolvedKey(sample.toKey)
	if !ok || to == nil || to.Width != from.Width || to.Height != from.Height || len(to.RGBA) != len(from.RGBA) {
		return out, true
	}
	out.to = to
	out.alpha = sample.alpha
	return out, true
}

func skyTextureForMap(mapName mapdata.MapName, wallTexBank map[string]WallTexture) (*WallTexture, bool) {
	_, tex, ok := skyTextureEntryForMap(mapName, wallTexBank)
	return tex, ok
}

func skyTextureEntryForMap(mapName mapdata.MapName, wallTexBank map[string]WallTexture) (string, *WallTexture, bool) {
	if key, ok := primarySkyTextureKey(mapName); ok {
		if tex, ok := wallTexBank[key]; ok && tex.Width > 0 && tex.Height > 0 && len(tex.RGBA) == tex.Width*tex.Height*4 {
			return key, &tex, true
		}
	}
	for _, key := range [...]string{"SKY1", "SKY2", "SKY3", "SKY4"} {
		if tex, ok := wallTexBank[key]; ok && tex.Width > 0 && tex.Height > 0 && len(tex.RGBA) == tex.Width*tex.Height*4 {
			return key, &tex, true
		}
	}
	return "", nil, false
}

func (g *game) runtimeSkyTextureEntryForMap(mapName mapdata.MapName) (string, *WallTexture, bool) {
	if g == nil {
		return "", nil, false
	}
	g.ensureTexturePointerCaches()
	if key, ok := primarySkyTextureKey(mapName); ok {
		if tex, ok := g.wallTexPtrs[key]; ok && tex != nil && tex.Width > 0 && tex.Height > 0 && len(tex.RGBA) == tex.Width*tex.Height*4 {
			return key, tex, true
		}
	}
	for _, key := range [...]string{"SKY1", "SKY2", "SKY3", "SKY4"} {
		if tex, ok := g.wallTexPtrs[key]; ok && tex != nil && tex.Width > 0 && tex.Height > 0 && len(tex.RGBA) == tex.Width*tex.Height*4 {
			return key, tex, true
		}
	}
	return "", nil, false
}

func primarySkyTextureKey(mapName mapdata.MapName) (string, bool) {
	name := string(mapName)
	if len(name) >= 4 &&
		(name[0] == 'E' || name[0] == 'e') &&
		(name[2] == 'M' || name[2] == 'm') &&
		name[1] >= '1' && name[1] <= '4' &&
		name[3] >= '0' && name[3] <= '9' {
		switch name[1] {
		case '1':
			return "SKY1", true
		case '2':
			return "SKY2", true
		case '3':
			return "SKY3", true
		case '4':
			return "SKY4", true
		}
	}
	if len(name) >= 5 &&
		(name[0] == 'M' || name[0] == 'm') &&
		(name[1] == 'A' || name[1] == 'a') &&
		(name[2] == 'P' || name[2] == 'p') {
		n := 0
		for i := 3; i < len(name); i++ {
			if name[i] < '0' || name[i] > '9' {
				n = 0
				break
			}
			n = n*10 + int(name[i]-'0')
		}
		switch {
		case n >= 1 && n <= 11:
			return "SKY1", true
		case n >= 12 && n <= 20:
			return "SKY2", true
		case n >= 21:
			return "SKY3", true
		}
	}
	return "", false
}

func effectiveSkyTexHeight(tex *WallTexture) int {
	if tex == nil {
		return 0
	}
	return scene.EffectiveTextureHeight(scene.Texture{RGBA: tex.RGBA, Width: tex.Width, Height: tex.Height})
}

func (g *game) beginSkyLayerFrame() {
	g.skyLayerFrameActive = false
}

func (g *game) resetSkyLayerPipeline(rebuildShader bool) {
	g.skyLayerFrameActive = false

	// Clear sky lookup caches so the next frame recomputes against current
	// resolution/focal/texture parameters. Keep backing storage so output-size
	// jitter does not force fresh allocations on the next frame.
	g.skyColViewW = 0
	g.skyAngleViewW = 0
	g.skyAngleFocal = 0
	g.skyRowViewH = 0
	g.skyRowTexH = 0
	g.skyRowIScale = 0
	g.skyRowDrawH = 0
	g.skyLayerProjDrawW = 0
	g.skyLayerProjDrawH = 0
	g.skyLayerProjSampleW = 0
	g.skyLayerProjSampleH = 0
	g.skyLayerUniforms = nil

	if rebuildShader && g.opts.GPUSky && g.opts.SourcePortMode && g.skyLayerShader == nil {
		if sh, err := ebiten.NewShader(skyBackdropShaderSrc); err == nil {
			g.skyLayerShader = sh
		}
	}
}

func (g *game) enableSkyLayerFrame(camAng, focal float64, texKey string, tex *WallTexture, texH int) bool {
	if g.skyLayerShader == nil || !g.opts.SourcePortMode {
		return false
	}
	if tex == nil {
		return false
	}
	if texKey == "" || tex.Width <= 0 || tex.Height <= 0 || len(tex.RGBA) != tex.Width*tex.Height*4 {
		return false
	}
	drawW, drawH, sampleW, sampleH := g.skyProjectionSize()
	if g.skyLayerProjDrawW != drawW || g.skyLayerProjDrawH != drawH || g.skyLayerProjSampleW != sampleW || g.skyLayerProjSampleH != sampleH {
		g.skyLayerProjDrawW = drawW
		g.skyLayerProjDrawH = drawH
		g.skyLayerProjSampleW = sampleW
		g.skyLayerProjSampleH = sampleH
	}
	if g.skyLayerTex == nil || g.skyLayerTexKey != texKey || g.skyLayerTexW != tex.Width || g.skyLayerTexH != tex.Height {
		img := newDebugImage("sky:"+texKey, tex.Width, tex.Height)
		img.WritePixels(tex.RGBA)
		g.skyLayerTex = img
		g.skyLayerTexKey = texKey
		g.skyLayerTexW = tex.Width
		g.skyLayerTexH = tex.Height
	}
	g.skyLayerFrameActive = true
	g.skyLayerFrameCamAng = camAng
	if g.opts.SourcePortMode {
		g.skyLayerFrameFocal = doomFocalLength(sampleW)
	} else {
		g.skyLayerFrameFocal = focal
	}
	g.skyLayerFrameTexH = float64(max(texH, 1))
	return true
}

func (g *game) drawSkyLayerFrame(screen *ebiten.Image) bool {
	if !g.skyLayerFrameActive || g.skyLayerShader == nil || g.skyLayerTex == nil || screen == nil {
		return false
	}
	w := g.viewW
	h := g.viewH
	if w <= 0 || h <= 0 {
		return false
	}
	texW := g.skyLayerTexW
	texH := g.skyLayerTexH
	if texW <= 0 || texH <= 0 {
		return false
	}
	v := &g.skyLayerVerts
	v[0] = ebiten.Vertex{DstX: 0, DstY: 0, SrcX: 0, SrcY: 0, ColorR: 1, ColorG: 1, ColorB: 1, ColorA: 1, Custom0: 0, Custom1: 0}
	v[1] = ebiten.Vertex{DstX: float32(w), DstY: 0, SrcX: float32(texW), SrcY: 0, ColorR: 1, ColorG: 1, ColorB: 1, ColorA: 1, Custom0: float32(w), Custom1: 0}
	v[2] = ebiten.Vertex{DstX: 0, DstY: float32(h), SrcX: 0, SrcY: float32(texH), ColorR: 1, ColorG: 1, ColorB: 1, ColorA: 1, Custom0: 0, Custom1: float32(h)}
	v[3] = ebiten.Vertex{DstX: float32(w), DstY: float32(h), SrcX: float32(texW), SrcY: float32(texH), ColorR: 1, ColorG: 1, ColorB: 1, ColorA: 1, Custom0: float32(w), Custom1: float32(h)}
	g.skyLayerIdx = [6]uint16{0, 1, 2, 1, 2, 3}
	op := &g.skyLayerShaderOp
	op.Images[0] = g.skyLayerTex
	_, _, sampleW, sampleH := g.skyProjectionSize()
	if g.skyLayerUniforms == nil {
		g.skyLayerUniforms = make(map[string]any, 9)
	}
	g.skyLayerUniforms["CamAngle"] = g.skyLayerFrameCamAng
	g.skyLayerUniforms["Focal"] = g.skyLayerFrameFocal
	g.skyLayerUniforms["DrawW"] = float64(w)
	g.skyLayerUniforms["DrawH"] = float64(h)
	g.skyLayerUniforms["SampleW"] = float64(sampleW)
	g.skyLayerUniforms["SampleH"] = float64(sampleH)
	g.skyLayerUniforms["SkyTexW"] = float64(texW)
	g.skyLayerUniforms["SkyTexH"] = g.skyLayerFrameTexH
	if g.opts.SkyUpscaleMode == "sharp" {
		g.skyLayerUniforms["SharpUpscale"] = float64(1)
	} else {
		g.skyLayerUniforms["SharpUpscale"] = float64(0)
	}
	op.Uniforms = g.skyLayerUniforms
	screen.DrawTrianglesShader(g.skyLayerVerts[:], g.skyLayerIdx[:], g.skyLayerShader, op)
	return true
}

func sourcePortAudioEnabled(opts Options) bool {
	return opts.SourcePortMode && !isWASMBuild()
}

func (g *game) rendererWorkerCount() int {
	maxProcs := runtime.GOMAXPROCS(0)
	if maxProcs <= 1 || isWASMBuild() {
		return 1
	}
	if g != nil && g.opts.RendererWorkers > 0 {
		if g.opts.RendererWorkers > maxProcs {
			return maxProcs
		}
		return g.opts.RendererWorkers
	}
	if runtime.NumCPU() > 1 {
		if maxProcs < 2 {
			return maxProcs
		}
		return 2
	}
	return 1
}

func (g *game) parallelWorkChunks(n int) (workers, chunk int, ok bool) {
	if n < 2 {
		return 1, n, false
	}
	workers = g.rendererWorkerCount()
	if workers <= 1 {
		return 1, n, false
	}
	if workers > n {
		workers = n
	}
	chunk = (n + workers - 1) / workers
	if chunk <= 0 {
		chunk = n
	}
	return workers, chunk, workers > 1
}

func (g *game) buildPlaneSpansParallel(planes []*plane3DVisplane, viewH int) ([][]plane3DSpan, int, int, bool) {
	spansByPlane, spanStartScratch := g.ensurePlaneSpanScratch(len(planes))
	if len(planes) == 0 {
		return spansByPlane, 0, 0, false
	}
	if _, chunk, parallel := g.parallelWorkChunks(len(planes)); parallel && len(planes) >= 32 {
		var wg sync.WaitGroup
		for start := 0; start < len(planes); start += chunk {
			end := min(start+chunk, len(planes))
			wg.Add(1)
			go func(start, end int) {
				defer wg.Done()
				for i := start; i < end; i++ {
					pl := planes[i]
					if pl == nil {
						spansByPlane[i] = spansByPlane[i][:0]
						continue
					}
					spansByPlane[i] = makePlane3DSpansWithScratch(pl, viewH, spansByPlane[i][:0], spanStartScratch[i])
				}
			}(start, end)
		}
		wg.Wait()
	} else {
		for i := range planes {
			pl := planes[i]
			if pl == nil {
				spansByPlane[i] = spansByPlane[i][:0]
				continue
			}
			spansByPlane[i] = makePlane3DSpansWithScratch(pl, viewH, spansByPlane[i][:0], spanStartScratch[i])
		}
	}
	active := 0
	input := 0
	hasSky := false
	for i, spans := range spansByPlane {
		if len(spans) == 0 {
			continue
		}
		active++
		input += len(spans)
		if planes[i].key.sky {
			hasSky = true
		}
	}
	return spansByPlane, active, input, hasSky
}

func (g *game) buildWallSegPrepassParallel(visible []int, camX, camY, ca, sa, focal, near float64) []wallSegPrepass {
	out := g.ensureWallPrepassBuffer(len(visible))
	if len(visible) == 0 {
		return out
	}
	if _, chunk, parallel := g.parallelWorkChunks(len(visible)); parallel && len(visible) >= 64 {
		var wg sync.WaitGroup
		for start := 0; start < len(visible); start += chunk {
			end := min(start+chunk, len(visible))
			wg.Add(1)
			go func(start, end int) {
				defer wg.Done()
				for i := start; i < end; i++ {
					out[i] = g.buildWallSegPrepassSingle(visible[i], camX, camY, ca, sa, focal, near)
				}
			}(start, end)
		}
		wg.Wait()
	} else {
		for i, si := range visible {
			out[i] = g.buildWallSegPrepassSingle(si, camX, camY, ca, sa, focal, near)
		}
	}
	return out
}

func (g *game) resolveAnimatedWallFrameAvailable(name, preferred string, dir int) string {
	key := normalizeFlatName(name)
	if key == "" {
		return ""
	}
	ref, ok := g.wallTextureAnimRefs[key]
	if !ok || len(ref.frames) < 2 {
		if _, ok := g.wallTexturePtrResolvedKey(preferred); ok {
			return preferred
		}
		if _, ok := g.wallTexturePtrResolvedKey(key); ok {
			return key
		}
		return preferred
	}
	start := 0
	preferred = normalizeFlatName(preferred)
	for i, frame := range ref.frames {
		if frame == preferred {
			start = i
			break
		}
	}
	if dir == 0 {
		dir = 1
	}
	for n := 0; n < len(ref.frames); n++ {
		idx := (start + dir*n) % len(ref.frames)
		if idx < 0 {
			idx += len(ref.frames)
		}
		frame := ref.frames[idx]
		if _, ok := g.wallTexturePtrResolvedKey(frame); ok {
			return frame
		}
	}
	if _, ok := g.wallTexturePtrResolvedKey(key); ok {
		return key
	}
	return preferred
}

func (g *game) buildWallSegPrepassSingle(si int, camX, camY, ca, sa, focal, near float64) wallSegPrepass {
	pp := wallSegPrepass{
		segIdx:          si,
		frontSideDefIdx: -1,
	}
	cacheOK := si >= 0 && si < len(g.wallSegStaticCache) && g.wallSegStaticCache[si].valid
	var (
		ld             mapdata.Linedef
		input          scene.WallPrepassWorldInput
		hasTwoSidedMid bool
		frontSectorIdx = -1
		backSectorIdx  = -1
	)
	if cacheOK {
		c := g.wallSegStaticCache[si]
		ld = c.ld
		input = c.input
		pp.frontSideDefIdx = c.frontSideDefIdx
		hasTwoSidedMid = c.hasTwoSidedMidTex
		frontSectorIdx = c.frontSectorIdx
		backSectorIdx = c.backSectorIdx
		animatedLight := g.sectorLightKindCached(frontSectorIdx) != sectorLightEffectNone ||
			g.sectorLightKindCached(backSectorIdx) != sectorLightEffectNone
		if !hasTwoSidedMid && c.portalSplitStatic && !animatedLight {
			d := g.linedefDecisionPseudo3D(ld)
			if !d.Visible && !c.portalSplit {
				pp.ld = ld
				return pp
			}
		}
	} else {
		if si < 0 || si >= len(g.m.Segs) {
			return pp
		}
		seg := g.m.Segs[si]
		li := int(seg.Linedef)
		if li < 0 || li >= len(g.m.Linedefs) {
			return pp
		}
		ld = g.m.Linedefs[li]
		var ok bool
		x1w, y1w, x2w, y2w, ok := g.segWorldEndpoints(si)
		if !ok {
			return pp
		}
		frontSide := int(seg.Direction)
		if frontSide < 0 || frontSide > 1 {
			frontSide = 0
		}
		backSide := frontSide ^ 1
		if sn := ld.SideNum[frontSide]; sn >= 0 && int(sn) < len(g.m.Sidedefs) {
			pp.frontSideDefIdx = int(sn)
		}
		segLen := math.Hypot(x2w-x1w, y2w-y1w)
		// Projection U tracks seg-local alignment. Sidedef texture offset is
		// applied later at draw time, matching Doom's rw_offset setup.
		uBase := float64(seg.Offset)
		input = scene.NewWallPrepassWorldInput(x1w, y1w, x2w, y2w, uBase, segLen, frontSide)
		hasTwoSidedMid = g.segHasTwoSidedMidTexture(si)
		frontSectorIdx = g.sectorIndexFromSideNum(ld.SideNum[frontSide])
		backSectorIdx = g.sectorIndexFromSideNum(ld.SideNum[backSide])
	}
	pp.ld = ld
	d := g.linedefDecisionPseudo3D(ld)
	portalSplit := false
	portalSplit = g.segPortalSplitAtTick(si, cacheOK, frontSectorIdx, backSectorIdx)
	if !d.Visible && !hasTwoSidedMid && !portalSplit {
		return pp
	}
	prepass := scene.BuildWallPrepassFromWorld(input, camX, camY, ca, sa, g.viewW, focal, near)
	pp.prepass = prepass
	if !pp.prepass.OK {
		return pp
	}
	return pp
}

func (g *game) segPortalSplitAtTick(segIdx int, cacheOK bool, frontSectorIdx, backSectorIdx int) bool {
	if g == nil || g.m == nil ||
		frontSectorIdx < 0 || backSectorIdx < 0 ||
		frontSectorIdx >= len(g.m.Sectors) || backSectorIdx >= len(g.m.Sectors) {
		return false
	}
	animatedLight := g.sectorLightKindCached(frontSectorIdx) != sectorLightEffectNone ||
		g.sectorLightKindCached(backSectorIdx) != sectorLightEffectNone
	if cacheOK && segIdx >= 0 && segIdx < len(g.wallSegStaticCache) {
		c := &g.wallSegStaticCache[segIdx]
		if c.portalSplitStatic && !animatedLight {
			return c.portalSplit
		}
		if c.lightTickValid && c.lightTick == g.worldTic {
			return c.lightTickSplit
		}
	}
	front := &g.m.Sectors[frontSectorIdx]
	back := &g.m.Sectors[backSectorIdx]
	split := front.FloorHeight != back.FloorHeight ||
		front.CeilingHeight != back.CeilingHeight ||
		normalizeFlatName(front.FloorPic) != normalizeFlatName(back.FloorPic) ||
		normalizeFlatName(front.CeilingPic) != normalizeFlatName(back.CeilingPic) ||
		g.sectorsLightDifferForRender(frontSectorIdx, backSectorIdx, front, back)
	if cacheOK && segIdx >= 0 && segIdx < len(g.wallSegStaticCache) {
		c := &g.wallSegStaticCache[segIdx]
		c.lightTickValid = true
		c.lightTick = g.worldTic
		c.lightTickSplit = split
	}
	return split
}

func (g *game) segHasTwoSidedMidTexture(segIdx int) bool {
	if segIdx < 0 || segIdx >= len(g.m.Segs) {
		return false
	}
	sg := g.m.Segs[segIdx]
	li := int(sg.Linedef)
	if li < 0 || li >= len(g.m.Linedefs) {
		return false
	}
	ld := g.m.Linedefs[li]
	frontSide := int(sg.Direction)
	if frontSide < 0 || frontSide > 1 {
		frontSide = 0
	}
	backSide := frontSide ^ 1
	if ld.SideNum[frontSide] < 0 || ld.SideNum[backSide] < 0 {
		return false
	}
	fs := int(ld.SideNum[frontSide])
	bs := int(ld.SideNum[backSide])
	if fs < 0 || fs >= len(g.m.Sidedefs) || bs < 0 || bs >= len(g.m.Sidedefs) {
		return false
	}
	frontSec := g.sectorIndexFromSideNum(ld.SideNum[frontSide])
	backSec := g.sectorIndexFromSideNum(ld.SideNum[backSide])
	if frontSec < 0 || backSec < 0 {
		return false
	}
	mid := normalizeFlatName(g.m.Sidedefs[fs].Mid)
	if mid == "" || mid == "-" {
		return false
	}
	return true
}

func (g *game) ensureWallPrepassBuffer(n int) []wallSegPrepass {
	if n <= 0 {
		g.wallPrepassBuf = g.wallPrepassBuf[:0]
		return g.wallPrepassBuf
	}
	if cap(g.wallPrepassBuf) < n {
		g.wallPrepassBuf = make([]wallSegPrepass, n)
	} else {
		g.wallPrepassBuf = g.wallPrepassBuf[:n]
	}
	return g.wallPrepassBuf
}

func (g *game) buildSkyLookupParallel(viewW, viewH int, focal, camAngle float64, texW, texH int) ([]int, []int) {
	if viewW <= 0 || viewH <= 0 || texW <= 0 || texH <= 0 {
		return nil, nil
	}
	_, _, sampleW, sampleH := g.skyProjectionSize()
	if sampleW <= 0 || sampleH <= 0 {
		return nil, nil
	}
	projFocal := focal
	if g.opts.SourcePortMode {
		projFocal = doomFocalLength(sampleW)
	}
	angleOff := g.ensureSkyAngleOffsets(sampleW, projFocal)
	sampleRow := g.ensureSkyRowLookup(sampleW, sampleH, texH)
	row := g.ensureSkyDrawRowBuffer(viewH)
	col := g.ensureSkyColBuffer(viewW)
	for y := 0; y < viewH; y++ {
		sampleY := scene.ProjectedSampleIndex(y, viewH, sampleH)
		row[y] = sampleRow[sampleY]
	}
	for x := 0; x < viewW; x++ {
		sampleX := scene.ProjectedSampleIndex(x, viewW, sampleW)
		angle := camAngle + angleOff[sampleX]
		col[x] = wrapIndex(int(math.Floor(angle*float64(texW*4)/(2*math.Pi))), texW)
	}
	return col, row
}

func (g *game) ensureSkyColBuffer(viewW int) []int {
	if viewW <= 0 {
		return nil
	}
	if len(g.skyColUCache) != viewW || g.skyColViewW != viewW {
		g.skyColUCache = resizeSliceLen(g.skyColUCache, viewW)
		g.skyColViewW = viewW
	}
	return g.skyColUCache
}

func (g *game) ensureSkyAngleOffsets(viewW int, focal float64) []float64 {
	if viewW <= 0 {
		return nil
	}
	if focal <= 1e-6 {
		focal = 1
	}
	if len(g.skyAngleOff) == viewW && g.skyAngleViewW == viewW && math.Abs(g.skyAngleFocal-focal) < 1e-9 {
		return g.skyAngleOff
	}
	off := resizeSliceLen(g.skyAngleOff, viewW)
	cx := float64(viewW) * 0.5
	for x := 0; x < viewW; x++ {
		sampleX := float64(x) + 0.5
		off[x] = math.Atan((cx - sampleX) / focal)
	}
	g.skyAngleOff = off
	g.skyAngleViewW = viewW
	g.skyAngleFocal = focal
	return g.skyAngleOff
}

func (g *game) ensureSkyRowLookup(viewW, viewH, texH int) []int {
	if viewW <= 0 || viewH <= 0 || texH <= 0 {
		return nil
	}
	iscale := doomSkyIScale(viewW)
	if len(g.skyRowVCache) == viewH && g.skyRowViewH == viewH && g.skyRowTexH == texH && math.Abs(g.skyRowIScale-iscale) < 1e-9 {
		return g.skyRowVCache
	}
	row := resizeSliceLen(g.skyRowVCache, viewH)
	cy := float64(viewH) * 0.5
	textureMid := 100.0
	for y := 0; y < viewH; y++ {
		frac := textureMid + ((float64(y) - cy) * iscale)
		row[y] = wrapIndex(int(math.Floor(frac)), texH)
	}
	g.skyRowVCache = row
	g.skyRowViewH = viewH
	g.skyRowTexH = texH
	g.skyRowIScale = iscale
	return g.skyRowVCache
}

func (g *game) ensureSkyDrawRowBuffer(drawH int) []int {
	if drawH <= 0 {
		return nil
	}
	if len(g.skyRowDrawCache) != drawH || g.skyRowDrawH != drawH {
		g.skyRowDrawCache = resizeSliceLen(g.skyRowDrawCache, drawH)
		g.skyRowDrawH = drawH
	}
	return g.skyRowDrawCache
}

func (g *game) skyProjectionSize() (drawW, drawH, sampleW, sampleH int) {
	p := scene.ProjectionSize(g.viewW, g.viewH, g.skyOutputW, g.skyOutputH, g.opts.SourcePortMode)
	return p.DrawW, p.DrawH, p.SampleW, p.SampleH
}

func doomSkyIScale(viewW int) float64 {
	return scene.DoomSkyIScale(viewW)
}

func wrapIndex(x, size int) int {
	if size <= 0 {
		return 0
	}
	m := x % size
	if m < 0 {
		m += size
	}
	return m
}

func floorDiv(x, y int) int {
	if y <= 0 {
		return 0
	}
	q := x / y
	r := x % y
	if r != 0 && x < 0 {
		q--
	}
	return q
}

func ceilDiv(x, y int64) int64 {
	if y <= 0 {
		return 0
	}
	if x <= 0 {
		return 0
	}
	return (x + y - 1) / y
}

func shadeFactorByDistance(dist float64) float64 {
	n := dist / 1200.0
	if n < 0 {
		n = 0
	}
	if n > 1 {
		n = 1
	}
	return 1.0 - 0.72*n
}

func shadeMulByDistance(dist float64) int {
	sf := shadeFactorByDistance(dist)
	m := int(sf * 256.0)
	if m < 0 {
		return 0
	}
	if m > 256 {
		return 256
	}
	return m
}

const (
	doomNumColorMaps  = 32
	doomLightLevels   = 16
	doomDistMap       = 2
	doomLightSegShift = 4
	doomMaxLightScale = 48
	doomMaxLightZ     = 128
)

func doomWallLightBias(ld *mapdata.Linedef, verts []mapdata.Vertex) int {
	if ld == nil || int(ld.V1) >= len(verts) || int(ld.V2) >= len(verts) {
		return 0
	}
	v1 := verts[int(ld.V1)]
	v2 := verts[int(ld.V2)]
	// Vanilla Doom applies directional light bias for axis-aligned walls.
	if v1.Y == v2.Y {
		return -1
	}
	if v1.X == v2.X {
		return 1
	}
	return 0
}

func doomClampLightNum(lightNum int) int {
	if lightNum < 0 {
		return 0
	}
	if lightNum >= doomLightLevels {
		return doomLightLevels - 1
	}
	return lightNum
}

func doomClampLightNumF(lightNum float64) float64 {
	if lightNum < 0 {
		return 0
	}
	maxLight := float64(doomLightLevels - 1)
	if lightNum > maxLight {
		return maxLight
	}
	return lightNum
}

func doomStartMap(lightNum int) int {
	rows := doomShadeRows()
	if rows <= 0 {
		rows = doomNumColorMaps
	}
	return ((doomLightLevels - 1 - lightNum) * 2 * rows) / doomLightLevels
}

func doomStartMapF(lightNum float64) float64 {
	rows := doomShadeRows()
	if rows <= 0 {
		rows = doomNumColorMaps
	}
	return ((float64(doomLightLevels-1) - lightNum) * 2 * float64(rows)) / float64(doomLightLevels)
}

func doomClampColorMapRow(row int) int {
	rows := doomShadeRows()
	if rows <= 0 {
		return 0
	}
	if row < 0 {
		return 0
	}
	if row >= rows {
		return rows - 1
	}
	return row
}

func doomWallLightRow(light int16, lightBias int, depth, focal float64) int {
	return doomClampColorMapRow(int(doomWallLightRowF(light, lightBias, depth, focal)))
}

func doomWallLightRowF(light int16, lightBias int, depth, focal float64) float64 {
	if !doomSectorLighting {
		light = 255
	}
	lightNum := doomClampLightNumF(float64(light)/float64(1<<doomLightSegShift) + float64(lightBias))
	startMap := doomStartMapF(lightNum)
	rows := doomShadeRows()
	if rows <= 0 {
		return 0
	}
	if depth <= 0 || focal <= 0 {
		if startMap < 0 {
			return 0
		}
		maxRow := float64(rows - 1)
		if startMap > maxRow {
			return maxRow
		}
		return startMap
	}
	// Doom wall index ~= (rw_scale >> LIGHTSCALESHIFT), with rw_scale in 16.16.
	lightScale := (focal / depth) * 16.0
	// Normalize to Doom's 320-wide light-table basis so wall shading stays
	// consistent across internal render resolutions.
	lightScale *= float64(doomLogicalW) / (2.0 * focal)
	if lightScale < 0 {
		lightScale = 0
	}
	if lightScale >= float64(doomMaxLightScale) {
		lightScale = float64(doomMaxLightScale - 1)
	}
	row := float64(startMap) - (lightScale / float64(doomDistMap))
	if row < 0 {
		return 0
	}
	maxRow := float64(rows - 1)
	if row > maxRow {
		return maxRow
	}
	return row
}

func doomPlaneLightRow(light int16, depth float64) int {
	return doomClampColorMapRow(int(doomPlaneLightRowF(light, depth)))
}

func doomPlaneLightRowF(light int16, depth float64) float64 {
	if !doomSectorLighting {
		light = 255
	}
	lightNum := doomClampLightNumF(float64(light) / float64(1<<doomLightSegShift))
	startMap := doomStartMapF(lightNum)
	rows := doomShadeRows()
	if rows <= 0 {
		return 0
	}
	if depth <= 0 {
		if startMap < 0 {
			return 0
		}
		maxRow := float64(rows - 1)
		if startMap > maxRow {
			return maxRow
		}
		return startMap
	}
	// Doom plane index ~= distance >> LIGHTZSHIFT with 16.16 fixed distance.
	lightZ := depth / 16.0
	if lightZ < 0 {
		lightZ = 0
	}
	if lightZ >= float64(doomMaxLightZ) {
		lightZ = float64(doomMaxLightZ - 1)
	}
	// Doom zlight uses an inverse-distance scale term before DISTMAP quantization.
	scale := (float64(doomLogicalW) / 2.0) / (lightZ + 1.0)
	row := float64(startMap) - (scale / float64(doomDistMap))
	if row < 0 {
		return 0
	}
	maxRow := float64(rows - 1)
	if row > maxRow {
		return maxRow
	}
	return row
}

func combineShadeMul(a, b int) int {
	// Combine distance and sector lighting multiplicatively.
	m := (a * b) >> 8
	if m < 0 {
		return 0
	}
	if m > 256 {
		return 256
	}
	return m
}

func sectorDistanceShadeMul(light int16, dist float64, distanceEnabled bool) int {
	if fullbrightNoLighting {
		return 256
	}
	mul := sectorLightMul(light)
	if !distanceEnabled {
		return mul
	}
	return combineShadeMul(mul, shadeMulByDistance(dist))
}

func sectorLightMul(light int16) int {
	if fullbrightNoLighting {
		return 256
	}
	if !doomSectorLighting {
		return 256
	}
	sectorLightLUTOnce.Do(initSectorLightMulLUT)
	i := int(light)
	if i < 0 {
		i = 0
	}
	if i > 255 {
		i = 255
	}
	return int(sectorLightMulLUT[i])
}

func doomFocalLength(viewW int) float64 {
	// Doom's classic horizontal FOV is approximately 90 degrees.
	// In a pinhole camera model this corresponds to focal = viewW / 2.
	if viewW <= 0 {
		return 1
	}
	return float64(viewW) * 0.5
}

func shadeRGBByMul(r, g, b byte, mul uint32) (byte, byte, byte) {
	if mul >= 256 {
		return r, g, b
	}
	return byte((uint32(r) * mul) >> 8), byte((uint32(g) * mul) >> 8), byte((uint32(b) * mul) >> 8)
}

func (g *game) drawMapFloorTextures2D(screen *ebiten.Image) {
	g.floorFrame = floorFrameStats{}
	g.drawMapFloorTextures2DRasterized(screen)
}

func (g *game) ensureMapFloorLoopSetsBuilt() {
	if g.mapFloorLoopInit {
		return
	}
	g.mapFloorLoopSets = g.buildSectorLoopSets()
	g.mapFloorLoopInit = true
}

func (g *game) mapFloorLoopSetsForView() []mapview.FloorLoopSet {
	out := make([]mapview.FloorLoopSet, len(g.mapFloorLoopSets))
	for sec, set := range g.mapFloorLoopSets {
		if !g.automapSectorRevealed(sec) {
			continue
		}
		rings := make([][]mapview.WorldPt, 0, len(set.rings))
		for _, ring := range set.rings {
			if len(ring) == 0 {
				continue
			}
			pts := make([]mapview.WorldPt, 0, len(ring))
			for _, p := range ring {
				pts = append(pts, mapview.WorldPt{X: p.x, Y: p.y})
			}
			rings = append(rings, pts)
		}
		out[sec] = mapview.FloorLoopSet{
			Rings: rings,
			BBox: mapview.WorldBBox{
				MinX: set.bbox.minX,
				MinY: set.bbox.minY,
				MaxX: set.bbox.maxX,
				MaxY: set.bbox.maxY,
			},
		}
	}
	return out
}

func (g *game) automapSectorRevealed(sec int) bool {
	if g == nil || g.m == nil || sec < 0 || sec >= len(g.m.Sectors) {
		return false
	}
	if g.automapRevealAll() {
		return true
	}
	if sec < 0 || sec >= len(g.sectorLineAdj) {
		return false
	}
	for _, entry := range g.sectorLineAdj[sec] {
		if entry.line < 0 || entry.line >= len(g.m.Linedefs) {
			continue
		}
		ld := g.m.Linedefs[entry.line]
		if ld.Flags&lineNeverSee != 0 {
			continue
		}
		if ld.Flags&mlMapped != 0 {
			return true
		}
	}
	return false
}

func (g *game) automapThingRevealed(i int, th mapdata.Thing) bool {
	if g == nil {
		return false
	}
	if g.automapRevealAll() {
		return true
	}
	sec := g.thingSectorCached(i, th)
	if sec < 0 {
		x, y := g.thingPosFixed(i, th)
		sec = g.sectorAt(x, y)
	}
	return g.automapSectorRevealed(sec)
}

func (g *game) mapFloorShadeMuls() []uint32 {
	out := make([]uint32, len(g.m.Sectors))
	for sec := range out {
		out[sec] = g.sectorLightMulCached(sec)
	}
	return out
}

func (g *game) mapFloorTextures() [][]byte {
	out := make([][]byte, len(g.m.Sectors))
	for sec := range out {
		if sec >= 0 && sec < len(g.sectorPlaneCache) && len(g.sectorPlaneCache[sec].flatRGBA) == 64*64*4 {
			out[sec] = g.sectorPlaneCache[sec].flatRGBA
		}
	}
	return out
}

func (g *game) drawMapFloorTextures2DRasterized(screen *ebiten.Image) {
	if g.m == nil || len(g.m.Sectors) == 0 || len(g.opts.FlatBank) == 0 {
		return
	}
	g.ensureSectorPlaneLevelCacheFresh()
	g.refreshSectorPlaneCacheTextureRefs()
	g.ensureMapFloorLoopSetsBuilt()
	if len(g.mapFloorLoopSets) == 0 {
		g.floorFrame.rejectedSpan++
		g.floorFrame.rejectNoPoly++
		return
	}
	g.ensureMapFloorLayer()
	clear(g.mapFloorPix)
	w := g.viewW
	h := g.viewH
	viewWB := g.screenWorldBBox()
	pix := g.mapFloorPix
	stats := mapview.RasterizeFloor2D(pix, mapview.FloorRasterInput{
		ViewW:         w,
		ViewH:         h,
		ViewBBox:      mapview.WorldBBox{MinX: viewWB.minX, MinY: viewWB.minY, MaxX: viewWB.maxX, MaxY: viewWB.maxY},
		LoopSets:      g.mapFloorLoopSetsForView(),
		ShadeMuls:     g.mapFloorShadeMuls(),
		Textures:      g.mapFloorTextures(),
		FallbackRGB:   [3]byte{wallFloorChange.R, wallFloorChange.G, wallFloorChange.B},
		ScreenToWorld: g.screenToWorld,
		WorldToScreen: g.worldToScreen,
	})
	g.writePixelsTimed(g.mapFloorLayer, pix)
	screen.DrawImage(g.mapFloorLayer, nil)
	g.mapFloorWorldState = "live-screen"
	g.floorFrame = floorFrameStats{
		markedCols:       stats.MarkedCols,
		emittedSpans:     stats.EmittedSpans,
		rejectedSpan:     stats.RejectedSpan,
		rejectNoSector:   stats.RejectNoSector,
		rejectNoPoly:     stats.RejectNoPoly,
		rejectDegenerate: stats.RejectDegenerate,
		rejectSpanClip:   stats.RejectSpanClip,
	}
}

func (g *game) screenWorldBBox() worldBBox {
	x0, y0 := g.screenToWorld(0, 0)
	x1, y1 := g.screenToWorld(float64(g.viewW), 0)
	x2, y2 := g.screenToWorld(float64(g.viewW), float64(g.viewH))
	x3, y3 := g.screenToWorld(0, float64(g.viewH))
	minX := math.Min(math.Min(x0, x1), math.Min(x2, x3))
	minY := math.Min(math.Min(y0, y1), math.Min(y2, y3))
	maxX := math.Max(math.Max(x0, x1), math.Max(x2, x3))
	maxY := math.Max(math.Max(y0, y1), math.Max(y2, y3))
	return worldBBox{minX: minX, minY: minY, maxX: maxX, maxY: maxY}
}

func (g *game) updateMapTextureDiagCache() {
	g.mapTexDiagStats = mapTexDiagStats{}
	if g.m == nil || len(g.m.SubSectors) == 0 {
		g.subSectorDiagCode = nil
		return
	}
	if len(g.subSectorDiagCode) != len(g.m.SubSectors) {
		g.subSectorDiagCode = make([]uint8, len(g.m.SubSectors))
	}
	for ss := range g.m.SubSectors {
		sub := g.m.SubSectors[ss]
		code := subDiagOK
		geomDiag := g.subSectorLoopGeomDiag(ss)
		switch {
		case ss < len(g.subSectorPoly) && len(g.subSectorPoly[ss]) >= 3 &&
			ss < len(g.subSectorTris) && len(g.subSectorTris[ss]) > 0:
			code = subDiagOK
		case ss < len(g.subSectorLoopDiag) && g.subSectorLoopDiag[ss] == loopDiagMultipleCandidates:
			code = subDiagLoopMultiNext
		case ss < len(g.subSectorLoopDiag) && g.subSectorLoopDiag[ss] == loopDiagDeadEnd:
			code = subDiagLoopDeadEnd
		case ss < len(g.subSectorLoopDiag) && g.subSectorLoopDiag[ss] == loopDiagEarlyClose:
			code = subDiagLoopEarlyClose
		case ss < len(g.subSectorLoopDiag) && g.subSectorLoopDiag[ss] == loopDiagNoClose:
			code = subDiagLoopNoClose
		case sub.SegCount < 3:
			code = subDiagSegShort
		case geomDiag == subDiagNonConvex:
			code = subDiagNonConvex
		case geomDiag == subDiagDegenerateArea:
			code = subDiagDegenerateArea
		case geomDiag == subDiagTriAreaMismatch:
			code = subDiagTriAreaMismatch
		case ss >= len(g.subSectorPoly) || len(g.subSectorPoly[ss]) < 3:
			code = subDiagNoPoly
		case !polygonSimple(g.subSectorPoly[ss]):
			code = subDiagNonSimple
		case ss >= len(g.subSectorTris) || len(g.subSectorTris[ss]) == 0:
			code = subDiagTriFail
		}
		g.subSectorDiagCode[ss] = code
		if ss < len(g.orphanSubSector) && g.orphanSubSector[ss] {
			g.mapTexDiagStats.orphan++
		}
		switch code {
		case subDiagOK:
			g.mapTexDiagStats.ok++
		case subDiagSegShort:
			g.mapTexDiagStats.segShort++
		case subDiagNoPoly:
			g.mapTexDiagStats.noPoly++
		case subDiagNonSimple:
			g.mapTexDiagStats.nonSimple++
		case subDiagTriFail:
			g.mapTexDiagStats.triFail++
		case subDiagLoopMultiNext:
			g.mapTexDiagStats.loopMultiNext++
		case subDiagLoopDeadEnd:
			g.mapTexDiagStats.loopDeadEnd++
		case subDiagLoopEarlyClose:
			g.mapTexDiagStats.loopEarlyClose++
		case subDiagLoopNoClose:
			g.mapTexDiagStats.loopNoClose++
		case subDiagNonConvex:
			g.mapTexDiagStats.nonConvex++
		case subDiagDegenerateArea:
			g.mapTexDiagStats.degenerateArea++
		case subDiagTriAreaMismatch:
			g.mapTexDiagStats.triAreaMismatch++
		}
	}
}

func (g *game) subSectorLoopGeomDiag(ss int) uint8 {
	if g == nil || g.m == nil || ss < 0 || ss >= len(g.m.SubSectors) || ss >= len(g.subSectorLoopVerts) {
		return subDiagOK
	}
	if ss >= len(g.subSectorPolySrc) || g.subSectorPolySrc[ss] != subPolySrcPrebuiltLoop {
		return subDiagOK
	}
	chain := g.subSectorLoopVerts[ss]
	if len(chain) < 3 {
		return subDiagOK
	}
	verts := vertexChainToWorld(g.m, chain)
	if len(verts) < 3 {
		return subDiagOK
	}
	const eps = 1e-6
	sign := 0
	for i := 0; i < len(verts); i++ {
		a := verts[i]
		b := verts[(i+1)%len(verts)]
		c := verts[(i+2)%len(verts)]
		abx := b.x - a.x
		aby := b.y - a.y
		bcx := c.x - b.x
		bcy := c.y - b.y
		cross := abx*bcy - aby*bcx
		if math.Abs(cross) <= eps {
			continue
		}
		curSign := 1
		if cross < 0 {
			curSign = -1
		}
		if sign == 0 {
			sign = curSign
		} else if sign != curSign {
			return subDiagNonConvex
		}
	}
	area2 := polygonArea2(verts)
	if math.Abs(area2) <= eps {
		return subDiagDegenerateArea
	}
	if area2 < 0 {
		area2 = -area2
		for i, j := 0, len(verts)-1; i < j; i, j = i+1, j-1 {
			verts[i], verts[j] = verts[j], verts[i]
		}
	}
	sumTriArea2 := 0.0
	a := verts[0]
	for i := 1; i+1 < len(verts); i++ {
		b := verts[i]
		c := verts[i+1]
		triArea2 := (b.x-a.x)*(c.y-a.y) - (b.y-a.y)*(c.x-a.x)
		if triArea2 <= eps {
			return subDiagTriAreaMismatch
		}
		sumTriArea2 += triArea2
	}
	tol := math.Max(1e-4, math.Abs(area2)*1e-4)
	if math.Abs(sumTriArea2-area2) > tol {
		return subDiagTriAreaMismatch
	}
	return subDiagOK
}

func (g *game) subSectorVerticesFromSegList(ss int) ([]worldPt, float64, float64, bool) {
	if ss < 0 || ss >= len(g.m.SubSectors) {
		return nil, 0, 0, false
	}
	sub := g.m.SubSectors[ss]
	if sub.SegCount < 3 {
		return nil, 0, 0, false
	}
	verts := make([]worldPt, 0, sub.SegCount)
	var firstStart uint16
	var prevEnd uint16
	haveEdge := false
	for i := 0; i < int(sub.SegCount); i++ {
		si := int(sub.FirstSeg) + i
		if si < 0 || si >= len(g.m.Segs) {
			return nil, 0, 0, false
		}
		sg := g.m.Segs[si]
		if !haveEdge {
			firstStart = sg.StartVertex
			prevEnd = sg.EndVertex
			haveEdge = true
		} else {
			if sg.StartVertex != prevEnd {
				return nil, 0, 0, false
			}
			prevEnd = sg.EndVertex
		}
		// Use subsector seg order directly (Doom BSP output).
		vi := sg.StartVertex
		if int(vi) >= len(g.m.Vertexes) {
			return nil, 0, 0, false
		}
		v := g.m.Vertexes[vi]
		p := worldPt{x: float64(v.X), y: float64(v.Y)}
		if len(verts) > 0 {
			last := verts[len(verts)-1]
			if last.x == p.x && last.y == p.y {
				continue
			}
		}
		verts = append(verts, p)
	}
	if !haveEdge || prevEnd != firstStart {
		return nil, 0, 0, false
	}
	if len(verts) >= 2 {
		a := verts[0]
		b := verts[len(verts)-1]
		if a.x == b.x && a.y == b.y {
			verts = verts[:len(verts)-1]
		}
	}
	if len(verts) < 3 {
		return nil, 0, 0, false
	}
	area2 := polygonArea2(verts)
	if math.Abs(area2) < 1e-6 {
		return nil, 0, 0, false
	}
	if area2 < 0 {
		for i, j := 0, len(verts)-1; i < j; i, j = i+1, j-1 {
			verts[i], verts[j] = verts[j], verts[i]
		}
	}
	cx, cy := 0.0, 0.0
	for _, v := range verts {
		cx += v.x
		cy += v.y
	}
	cx /= float64(len(verts))
	cy /= float64(len(verts))
	return verts, cx, cy, true
}

func (g *game) subSectorConvexVertices(ss int) ([]worldPt, float64, float64, bool) {
	if ss < 0 || ss >= len(g.m.SubSectors) {
		return nil, 0, 0, false
	}
	sub := g.m.SubSectors[ss]
	if sub.SegCount < 3 {
		return nil, 0, 0, false
	}
	chain, closed := subsectorVertexLoopFromSegOrder(g.m, sub)
	if !closed {
		// Some WAD subsectors reuse geometry/lines; fall back to unique vertices.
		verts, ok := uniqueSubsectorVertices(g.m, sub)
		if !ok || len(verts) < 3 {
			return nil, 0, 0, false
		}
		cx, cy := 0.0, 0.0
		for _, v := range verts {
			cx += v.x
			cy += v.y
		}
		cx /= float64(len(verts))
		cy /= float64(len(verts))
		slices.SortFunc(verts, func(a, b worldPt) int {
			ai := math.Atan2(a.y-cy, a.x-cx)
			aj := math.Atan2(b.y-cy, b.x-cx)
			if ai < aj {
				return -1
			}
			if ai > aj {
				return 1
			}
			return 0
		})
		if math.Abs(polygonArea2(verts)) < 1e-6 {
			return nil, 0, 0, false
		}
		return verts, cx, cy, true
	}
	verts := vertexChainToWorld(g.m, chain)
	if len(verts) < 3 {
		return nil, 0, 0, false
	}
	area2 := polygonArea2(verts)
	if math.Abs(area2) < 1e-6 {
		return nil, 0, 0, false
	}
	if area2 < 0 {
		for i, j := 0, len(verts)-1; i < j; i, j = i+1, j-1 {
			verts[i], verts[j] = verts[j], verts[i]
		}
	}
	cx, cy := 0.0, 0.0
	for _, v := range verts {
		cx += v.x
		cy += v.y
	}
	cx /= float64(len(verts))
	cy /= float64(len(verts))
	return verts, cx, cy, true
}

func frac01(x float64) float64 {
	return x - math.Floor(x)
}

type worldPt struct {
	x float64
	y float64
}

type holeFillPoly struct {
	sector int
	verts  []worldPt
	tris   [][3]int
	bbox   worldBBox
}

type holeQuantPt struct {
	x int64
	y int64
}

type holeEdgeKey struct {
	ax int64
	ay int64
	bx int64
	by int64
}

type holeBoundaryEdge struct {
	a  holeQuantPt
	b  holeQuantPt
	aw worldPt
	bw worldPt
}

type holeEdgeDirBucket struct {
	ab []holeBoundaryEdge
	ba []holeBoundaryEdge
}

type subsectorEdge struct {
	a uint16
	b uint16
}

func (g *game) subSectorWorldVertices(ss int) ([]worldPt, float64, float64, bool) {
	if ss < 0 || ss >= len(g.m.SubSectors) {
		return nil, 0, 0, false
	}
	sub := g.m.SubSectors[ss]
	if sub.SegCount < 3 {
		return nil, 0, 0, false
	}
	var chain []uint16
	if ss < len(g.subSectorLoopVerts) && len(g.subSectorLoopVerts[ss]) >= 3 {
		chain = g.subSectorLoopVerts[ss]
	} else {
		var closed bool
		chain, closed = subsectorVertexLoopFromSegOrder(g.m, sub)
		if !closed {
			return nil, 0, 0, false
		}
	}
	verts := vertexChainToWorld(g.m, chain)
	if len(verts) < 3 {
		return nil, 0, 0, false
	}
	if !normalizeAndValidateConvexFanPolygon(verts) {
		return nil, 0, 0, false
	}
	// Polygon centroid estimate (mean of vertices is enough for convex subsectors).
	cx, cy := 0.0, 0.0
	for _, v := range verts {
		cx += v.x
		cy += v.y
	}
	cx /= float64(len(verts))
	cy /= float64(len(verts))
	return verts, cx, cy, true
}

func normalizeAndValidateConvexFanPolygon(verts []worldPt) bool {
	if len(verts) < 3 {
		return false
	}
	const eps = 1e-6

	// Convexity: all non-zero cross products must share the same sign.
	sign := 0
	for i := 0; i < len(verts); i++ {
		a := verts[i]
		b := verts[(i+1)%len(verts)]
		c := verts[(i+2)%len(verts)]
		abx := b.x - a.x
		aby := b.y - a.y
		bcx := c.x - b.x
		bcy := c.y - b.y
		cross := abx*bcy - aby*bcx
		if math.Abs(cross) <= eps {
			continue
		}
		curSign := 1
		if cross < 0 {
			curSign = -1
		}
		if sign == 0 {
			sign = curSign
		} else if sign != curSign {
			return false
		}
	}

	area2 := polygonArea2(verts)
	if math.Abs(area2) <= eps {
		return false
	}
	if area2 < 0 {
		for i, j := 0, len(verts)-1; i < j; i, j = i+1, j-1 {
			verts[i], verts[j] = verts[j], verts[i]
		}
		area2 = -area2
	}

	// Fan triangulation sanity: no degenerate tri and area sum matches polygon area.
	sumTriArea2 := 0.0
	a := verts[0]
	for i := 1; i+1 < len(verts); i++ {
		b := verts[i]
		c := verts[i+1]
		triArea2 := (b.x-a.x)*(c.y-a.y) - (b.y-a.y)*(c.x-a.x)
		if triArea2 <= eps {
			return false
		}
		sumTriArea2 += triArea2
	}
	tol := math.Max(1e-4, math.Abs(area2)*1e-4)
	if math.Abs(sumTriArea2-area2) > tol {
		return false
	}
	return true
}

func subsectorVertexLoopFromSegOrder(m *mapdata.Map, sub mapdata.SubSector) ([]uint16, bool) {
	chain, _, ok := subsectorVertexLoopFromSegOrderDiag(m, sub)
	if ok {
		return chain, true
	}
	return subsectorVertexLoopNormalized(m, sub)
}

func subsectorVertexLoopFromSegOrderDiag(m *mapdata.Map, sub mapdata.SubSector) ([]uint16, loopBuildDiag, bool) {
	if sub.SegCount < 3 {
		return nil, loopDiagDeadEnd, false
	}
	edges := make([]subsectorEdge, 0, sub.SegCount)
	for i := 0; i < int(sub.SegCount); i++ {
		si := int(sub.FirstSeg) + i
		if si < 0 || si >= len(m.Segs) {
			return nil, loopDiagDeadEnd, false
		}
		sg := m.Segs[si]
		if int(sg.StartVertex) >= len(m.Vertexes) || int(sg.EndVertex) >= len(m.Vertexes) {
			return nil, loopDiagDeadEnd, false
		}
		if sg.StartVertex == sg.EndVertex {
			return nil, loopDiagDeadEnd, false
		}
		edges = append(edges, subsectorEdge{a: sg.StartVertex, b: sg.EndVertex})
	}
	if len(edges) < 3 {
		return nil, loopDiagDeadEnd, false
	}
	// Minimal ports-style builder:
	// - no geometric sorting
	// - no best-angle guessing
	// - require exact directed cycle using all segs.
	next := make(map[uint16]int, len(edges))
	for i, e := range edges {
		if _, exists := next[e.a]; exists {
			return nil, loopDiagMultipleCandidates, false
		}
		next[e.a] = i
	}

	startV := edges[0].a
	curV := edges[0].b
	loop := make([]uint16, 0, len(edges))
	loop = append(loop, startV)
	used := make([]bool, len(edges))
	used[0] = true
	usedCount := 1

	for steps := 0; steps < len(edges); steps++ {
		loop = append(loop, curV)
		if curV == startV {
			// Drop duplicate closing vertex.
			loop = loop[:len(loop)-1]
			if len(loop) != len(edges) || usedCount != len(edges) {
				return nil, loopDiagEarlyClose, false
			}
			return loop, loopDiagOK, true
		}
		j, exists := next[curV]
		if !exists {
			return nil, loopDiagDeadEnd, false
		}
		if used[j] {
			return nil, loopDiagEarlyClose, false
		}
		used[j] = true
		usedCount++
		curV = edges[j].b
	}
	return nil, loopDiagNoClose, false
}

func subsectorVertexLoopNormalized(m *mapdata.Map, sub mapdata.SubSector) ([]uint16, bool) {
	if m == nil || sub.SegCount < 3 {
		return nil, false
	}
	type dirEdge struct {
		a uint16
		b uint16
	}
	edges := make([]dirEdge, 0, sub.SegCount)
	sumX := 0.0
	sumY := 0.0
	vCount := 0.0
	for i := 0; i < int(sub.SegCount); i++ {
		si := int(sub.FirstSeg) + i
		if si < 0 || si >= len(m.Segs) {
			return nil, false
		}
		sg := m.Segs[si]
		if int(sg.StartVertex) >= len(m.Vertexes) || int(sg.EndVertex) >= len(m.Vertexes) || sg.StartVertex == sg.EndVertex {
			return nil, false
		}
		edges = append(edges, dirEdge{a: sg.StartVertex, b: sg.EndVertex})
		va := m.Vertexes[sg.StartVertex]
		vb := m.Vertexes[sg.EndVertex]
		sumX += float64(va.X) + float64(vb.X)
		sumY += float64(va.Y) + float64(vb.Y)
		vCount += 2.0
	}
	if len(edges) < 3 || vCount <= 0 {
		return nil, false
	}
	cx := sumX / vCount
	cy := sumY / vCount

	angleOfStart := func(e dirEdge) float64 {
		v := m.Vertexes[e.a]
		return math.Atan2(float64(v.Y)-cy, float64(v.X)-cx)
	}
	angleDiffCW := func(prev, cur float64) float64 {
		d := prev - cur
		for d <= 0 {
			d += 2 * math.Pi
		}
		return d
	}

	first := edges[0]
	used := make([]bool, len(edges))
	used[0] = true
	usedCount := 1
	chain := make([]uint16, 0, len(edges)*2+2)
	chain = append(chain, first.a, first.b)
	currEnd := first.b
	prevAngle := angleOfStart(first)
	startVertex := first.a

	for usedCount < len(edges) {
		best := -1
		bestDiff := math.MaxFloat64
		bestStart := uint16(0)
		bestEnd := uint16(0)
		bestContinuity := 2
		for i, e := range edges {
			if used[i] {
				continue
			}
			continuity := 1
			start := e.a
			end := e.b
			if e.a == currEnd {
				continuity = 0
			} else if e.b == currEnd {
				continuity = 0
				start, end = e.b, e.a
			}
			diff := angleDiffCW(prevAngle, math.Atan2(float64(m.Vertexes[start].Y)-cy, float64(m.Vertexes[start].X)-cx))
			if continuity < bestContinuity || (continuity == bestContinuity && diff < bestDiff) {
				bestContinuity = continuity
				bestDiff = diff
				best = i
				bestStart = start
				bestEnd = end
			}
		}
		if best < 0 {
			return nil, false
		}
		if bestContinuity > 0 {
			// Mimic nodebuilder behavior: add a connecting mini-seg.
			chain = append(chain, bestStart)
		}
		chain = append(chain, bestEnd)
		currEnd = bestEnd
		prevAngle = math.Atan2(float64(m.Vertexes[bestStart].Y)-cy, float64(m.Vertexes[bestStart].X)-cx)
		used[best] = true
		usedCount++
	}

	if currEnd != startVertex {
		chain = append(chain, startVertex)
	}
	if len(chain) < 4 || chain[len(chain)-1] != chain[0] {
		return nil, false
	}
	chain = chain[:len(chain)-1]
	if len(chain) < 3 {
		return nil, false
	}
	chain = simplifyVertexChainIndices(m, chain)
	if len(chain) < 3 {
		return nil, false
	}
	return chain, true
}

func simplifyVertexChainIndices(m *mapdata.Map, in []uint16) []uint16 {
	if len(in) < 3 || m == nil {
		return in
	}
	out := make([]uint16, 0, len(in))
	for _, vi := range in {
		if len(out) == 0 || out[len(out)-1] != vi {
			out = append(out, vi)
		}
	}
	if len(out) >= 2 && out[0] == out[len(out)-1] {
		out = out[:len(out)-1]
	}
	if len(out) < 3 {
		return out
	}
	const eps = 1e-9
	changed := true
	for changed && len(out) >= 3 {
		changed = false
		next := make([]uint16, 0, len(out))
		for i := 0; i < len(out); i++ {
			prev := out[(i-1+len(out))%len(out)]
			cur := out[i]
			nxt := out[(i+1)%len(out)]
			if int(prev) >= len(m.Vertexes) || int(cur) >= len(m.Vertexes) || int(nxt) >= len(m.Vertexes) {
				next = append(next, cur)
				continue
			}
			a := m.Vertexes[prev]
			b := m.Vertexes[cur]
			c := m.Vertexes[nxt]
			abx := float64(b.X - a.X)
			aby := float64(b.Y - a.Y)
			bcx := float64(c.X - b.X)
			bcy := float64(c.Y - b.Y)
			cross := abx*bcy - aby*bcx
			if math.Abs(cross) <= eps {
				changed = true
				continue
			}
			next = append(next, cur)
		}
		if len(next) >= 3 {
			out = next
		} else {
			break
		}
	}
	return out
}

func uniqueSubsectorVertices(m *mapdata.Map, sub mapdata.SubSector) ([]worldPt, bool) {
	seen := make(map[uint16]struct{}, int(sub.SegCount)*2)
	out := make([]worldPt, 0, int(sub.SegCount)*2)
	for i := 0; i < int(sub.SegCount); i++ {
		si := int(sub.FirstSeg) + i
		if si < 0 || si >= len(m.Segs) {
			continue
		}
		sg := m.Segs[si]
		for _, vi := range []uint16{sg.StartVertex, sg.EndVertex} {
			if _, ok := seen[vi]; ok {
				continue
			}
			if int(vi) >= len(m.Vertexes) {
				continue
			}
			v := m.Vertexes[vi]
			out = append(out, worldPt{x: float64(v.X), y: float64(v.Y)})
			seen[vi] = struct{}{}
		}
	}
	return out, len(out) >= 3
}

func chainSubsectorEdges(edges []subsectorEdge) ([]uint16, bool) {
	if len(edges) == 0 {
		return nil, false
	}
	used := make([]bool, len(edges))
	chain := make([]uint16, 0, len(edges)+1)
	chain = append(chain, edges[0].a, edges[0].b)
	used[0] = true
	for len(chain) <= len(edges)+1 {
		last := chain[len(chain)-1]
		progress := false
		for i, e := range edges {
			if used[i] {
				continue
			}
			if e.a == last {
				chain = append(chain, e.b)
				used[i] = true
				progress = true
				break
			}
			if e.b == last {
				chain = append(chain, e.a)
				used[i] = true
				progress = true
				break
			}
		}
		if !progress {
			break
		}
		if len(chain) >= 3 && chain[len(chain)-1] == chain[0] {
			allUsed := true
			for _, u := range used {
				if !u {
					allUsed = false
					break
				}
			}
			if allUsed {
				chain = chain[:len(chain)-1]
				return chain, true
			}
			break
		}
	}
	return nil, false
}

func subsectorVertexLoopByEdgeChain(m *mapdata.Map, sub mapdata.SubSector) ([]uint16, bool) {
	if m == nil || sub.SegCount < 3 {
		return nil, false
	}
	edges := make([]subsectorEdge, 0, sub.SegCount)
	for i := 0; i < int(sub.SegCount); i++ {
		si := int(sub.FirstSeg) + i
		if si < 0 || si >= len(m.Segs) {
			return nil, false
		}
		sg := m.Segs[si]
		if int(sg.StartVertex) >= len(m.Vertexes) || int(sg.EndVertex) >= len(m.Vertexes) {
			return nil, false
		}
		if sg.StartVertex == sg.EndVertex {
			continue
		}
		edges = append(edges, subsectorEdge{a: sg.StartVertex, b: sg.EndVertex})
	}
	if len(edges) < 3 {
		return nil, false
	}

	// First try a bounded mutation search over edge order/orientation to find a
	// valid simple closed loop that uses all edges exactly once.
	if chain, ok := func() ([]uint16, bool) {
		type stepChoice struct {
			edge int
			next uint16
			area float64
			dist float64
		}
		const maxExplore = 50000
		explored := 0
		bestArea := math.Inf(1)
		var bestChain []uint16

		var dfs func(start uint16, prev uint16, cur uint16, used []bool, usedCount int, chain []uint16, inPath map[uint16]bool)
		dfs = func(start uint16, prev uint16, cur uint16, used []bool, usedCount int, chain []uint16, inPath map[uint16]bool) {
			if explored >= maxExplore {
				return
			}
			explored++
			if usedCount == len(edges) {
				if cur != start || len(chain) < 4 {
					return
				}
				cand := append([]uint16(nil), chain[:len(chain)-1]...)
				cand = simplifyVertexChainIndices(m, cand)
				if len(cand) < 3 {
					return
				}
				verts := vertexChainToWorld(m, cand)
				if len(verts) < 3 || !polygonSimple(verts) {
					return
				}
				area := math.Abs(polygonArea2(verts))
				if area <= 1e-6 {
					return
				}
				if area < bestArea {
					bestArea = area
					bestChain = cand
				}
				return
			}

			choices := make([]stepChoice, 0, 8)
			for i, e := range edges {
				if used[i] {
					continue
				}
				var next uint16
				if e.a == cur {
					next = e.b
				} else if e.b == cur {
					next = e.a
				} else {
					continue
				}
				// Don't close loop before the last edge.
				if next == start && usedCount+1 != len(edges) {
					continue
				}
				// Avoid repeated vertices inside the path.
				if next != start && inPath[next] {
					continue
				}
				if int(prev) >= len(m.Vertexes) || int(cur) >= len(m.Vertexes) || int(next) >= len(m.Vertexes) {
					continue
				}
				pv := m.Vertexes[prev]
				cv := m.Vertexes[cur]
				nv := m.Vertexes[next]
				v1x := float64(cv.X - pv.X)
				v1y := float64(cv.Y - pv.Y)
				v2x := float64(nv.X - cv.X)
				v2y := float64(nv.Y - cv.Y)
				area2 := math.Abs(v1x*v2y - v1y*v2x)
				dx := float64(nv.X - cv.X)
				dy := float64(nv.Y - cv.Y)
				choices = append(choices, stepChoice{
					edge: i,
					next: next,
					area: area2,
					dist: dx*dx + dy*dy,
				})
			}
			slices.SortFunc(choices, func(a, b stepChoice) int {
				if math.Abs(a.area-b.area) > 1e-9 {
					if a.area < b.area {
						return -1
					}
					return 1
				}
				if a.dist < b.dist {
					return -1
				}
				if a.dist > b.dist {
					return 1
				}
				return 0
			})
			for _, ch := range choices {
				used[ch.edge] = true
				chain = append(chain, ch.next)
				added := false
				if ch.next != start {
					inPath[ch.next] = true
					added = true
				}
				dfs(start, cur, ch.next, used, usedCount+1, chain, inPath)
				if added {
					delete(inPath, ch.next)
				}
				chain = chain[:len(chain)-1]
				used[ch.edge] = false
				if explored >= maxExplore {
					return
				}
			}
		}

		for i, e := range edges {
			startPairs := [][2]uint16{{e.a, e.b}, {e.b, e.a}}
			for _, p := range startPairs {
				start := p[0]
				next := p[1]
				used := make([]bool, len(edges))
				used[i] = true
				chain := []uint16{start, next}
				inPath := map[uint16]bool{
					start: true,
					next:  true,
				}
				dfs(start, start, next, used, 1, chain, inPath)
				if explored >= maxExplore {
					break
				}
			}
			if explored >= maxExplore {
				break
			}
		}
		if len(bestChain) >= 3 {
			return bestChain, true
		}
		return nil, false
	}(); ok {
		return chain, true
	}

	used := make([]bool, len(edges))
	chain := make([]uint16, 0, len(edges)+1)
	chain = append(chain, edges[0].a, edges[0].b)
	used[0] = true
	prev := edges[0].a
	cur := edges[0].b

	for steps := 1; steps < len(edges); steps++ {
		best := -1
		bestNext := uint16(0)
		bestArea2 := math.Inf(1)
		bestDist2 := math.Inf(1)
		for i, e := range edges {
			if used[i] {
				continue
			}
			var next uint16
			if e.a == cur {
				next = e.b
			} else if e.b == cur {
				next = e.a
			} else {
				continue
			}
			// Avoid immediate backtracking unless there is no other option.
			if next == prev {
				continue
			}
			if int(cur) >= len(m.Vertexes) || int(next) >= len(m.Vertexes) {
				continue
			}
			if int(prev) >= len(m.Vertexes) {
				continue
			}
			pv := m.Vertexes[prev]
			cv := m.Vertexes[cur]
			nv := m.Vertexes[next]
			// Prefer the continuation that adds the smallest area wedge at this step.
			// area2 = |cross((cur-prev), (next-cur))|
			v1x := float64(cv.X - pv.X)
			v1y := float64(cv.Y - pv.Y)
			v2x := float64(nv.X - cv.X)
			v2y := float64(nv.Y - cv.Y)
			area2 := math.Abs(v1x*v2y - v1y*v2x)
			dx := float64(nv.X - cv.X)
			dy := float64(nv.Y - cv.Y)
			d2 := dx*dx + dy*dy
			if area2 < bestArea2 || (math.Abs(area2-bestArea2) <= 1e-9 && d2 < bestDist2) {
				bestArea2 = area2
				bestDist2 = d2
				best = i
				bestNext = next
			}
		}
		if best < 0 {
			// Last resort: allow backtracking match if nothing else exists.
			for i, e := range edges {
				if used[i] {
					continue
				}
				var next uint16
				if e.a == cur {
					next = e.b
				} else if e.b == cur {
					next = e.a
				} else {
					continue
				}
				best = i
				bestNext = next
				break
			}
		}
		if best < 0 {
			return nil, false
		}
		used[best] = true
		prev = cur
		cur = bestNext
		chain = append(chain, cur)
	}
	if len(chain) < 4 || chain[len(chain)-1] != chain[0] {
		return nil, false
	}
	chain = chain[:len(chain)-1]
	chain = simplifyVertexChainIndices(m, chain)
	if len(chain) < 3 {
		return nil, false
	}
	return chain, true
}

func rawSubsectorVertexOrder(m *mapdata.Map, sub mapdata.SubSector) []uint16 {
	chain := make([]uint16, 0, sub.SegCount)
	for i := 0; i < int(sub.SegCount); i++ {
		si := int(sub.FirstSeg) + i
		if si < 0 || si >= len(m.Segs) {
			continue
		}
		sg := m.Segs[si]
		if int(sg.StartVertex) >= len(m.Vertexes) {
			continue
		}
		chain = append(chain, sg.StartVertex)
	}
	if len(chain) >= 2 && chain[len(chain)-1] == chain[0] {
		chain = chain[:len(chain)-1]
	}
	return chain
}

func vertexChainToWorld(m *mapdata.Map, chain []uint16) []worldPt {
	if len(chain) < 3 {
		return nil
	}
	verts := make([]worldPt, 0, len(chain))
	lastX, lastY := math.Inf(1), math.Inf(1)
	for _, vi := range chain {
		if int(vi) >= len(m.Vertexes) {
			continue
		}
		v := m.Vertexes[vi]
		x, y := float64(v.X), float64(v.Y)
		if x == lastX && y == lastY {
			continue
		}
		verts = append(verts, worldPt{x: x, y: y})
		lastX, lastY = x, y
	}
	if len(verts) >= 2 {
		a := verts[0]
		b := verts[len(verts)-1]
		if a.x == b.x && a.y == b.y {
			verts = verts[:len(verts)-1]
		}
	}
	return verts
}

func polygonArea2(verts []worldPt) float64 {
	area2 := 0.0
	for i := 0; i < len(verts); i++ {
		j := (i + 1) % len(verts)
		area2 += verts[i].x*verts[j].y - verts[j].x*verts[i].y
	}
	return area2
}

func triangulateWorldPolygon(verts []worldPt) ([][3]int, bool) {
	n := len(verts)
	if n < 3 {
		return nil, false
	}
	if !polygonSimple(verts) {
		return nil, false
	}
	idx := make([]int, 0, n)
	for i := 0; i < n; i++ {
		if len(idx) > 0 && nearlyEqualWorldPt(verts[idx[len(idx)-1]], verts[i], 1e-9) {
			continue
		}
		idx = append(idx, i)
	}
	if len(idx) >= 2 && nearlyEqualWorldPt(verts[idx[0]], verts[idx[len(idx)-1]], 1e-9) {
		idx = idx[:len(idx)-1]
	}
	if len(idx) < 3 {
		return nil, false
	}
	const colEps = 1e-9
	for changed := true; changed && len(idx) > 3; {
		changed = false
		for i := 0; i < len(idx); i++ {
			pi := idx[(i-1+len(idx))%len(idx)]
			ci := idx[i]
			ni := idx[(i+1)%len(idx)]
			if pointOnSegmentWithEps(verts[ci], verts[pi], verts[ni], colEps) {
				idx = append(idx[:i], idx[i+1:]...)
				changed = true
				break
			}
		}
	}
	if len(idx) < 3 {
		return nil, false
	}
	area2 := polygonArea2Indexed(verts, idx)
	if math.Abs(area2) < 1e-9 {
		return nil, false
	}
	if area2 < 0 {
		for i, j := 0, len(idx)-1; i < j; i, j = i+1, j-1 {
			idx[i], idx[j] = idx[j], idx[i]
		}
	}
	if cdtTris, ok := triangulateWorldPolygonCDT(indexedWorldPts(verts, idx)); ok && len(cdtTris) > 0 {
		out := make([][3]int, 0, len(cdtTris))
		for _, tri := range cdtTris {
			out = append(out, [3]int{idx[tri[0]], idx[tri[1]], idx[tri[2]]})
		}
		return out, true
	}
	out := make([][3]int, 0, len(idx)-2)
	guard := 0
	for len(idx) > 3 && guard < len(idx)*len(idx)+n*n {
		guard++
		earFound := false
		for i := 0; i < len(idx); i++ {
			pi := idx[(i-1+len(idx))%len(idx)]
			ci := idx[i]
			ni := idx[(i+1)%len(idx)]
			if !isCCW(verts[pi], verts[ci], verts[ni]) {
				continue
			}
			if containsAnyPointInTri(verts, idx, pi, ci, ni) {
				continue
			}
			out = append(out, [3]int{pi, ci, ni})
			idx = append(idx[:i], idx[i+1:]...)
			earFound = true
			break
		}
		if !earFound {
			return nil, false
		}
	}
	if len(idx) == 3 {
		out = append(out, [3]int{idx[0], idx[1], idx[2]})
	}
	return out, len(out) > 0
}

func indexedWorldPts(verts []worldPt, idx []int) []worldPt {
	out := make([]worldPt, 0, len(idx))
	for _, i := range idx {
		if i >= 0 && i < len(verts) {
			out = append(out, verts[i])
		}
	}
	return out
}

func triangulateWorldPolygonQuadFirst(verts []worldPt) ([][3]int, bool) {
	// For 4-point loops, prefer a quad split (2 triangles) first.
	if len(verts) == 4 && polygonSimple(verts) {
		const eps = 1e-9
		validTri := func(i0, i1, i2 int) bool {
			a, b, c := verts[i0], verts[i1], verts[i2]
			if math.Abs(orient2D(a, b, c)) <= eps {
				return false
			}
			cent := worldPt{x: (a.x + b.x + c.x) / 3.0, y: (a.y + b.y + c.y) / 3.0}
			return pointInWorldPoly(cent, verts)
		}
		diag02 := [][3]int{{0, 1, 2}, {0, 2, 3}}
		diag13 := [][3]int{{1, 2, 3}, {1, 3, 0}}
		ok02 := validTri(diag02[0][0], diag02[0][1], diag02[0][2]) &&
			validTri(diag02[1][0], diag02[1][1], diag02[1][2])
		ok13 := validTri(diag13[0][0], diag13[0][1], diag13[0][2]) &&
			validTri(diag13[1][0], diag13[1][1], diag13[1][2])
		if ok02 || ok13 {
			d02 := (verts[0].x-verts[2].x)*(verts[0].x-verts[2].x) + (verts[0].y-verts[2].y)*(verts[0].y-verts[2].y)
			d13 := (verts[1].x-verts[3].x)*(verts[1].x-verts[3].x) + (verts[1].y-verts[3].y)*(verts[1].y-verts[3].y)
			if ok02 && (!ok13 || d02 <= d13) {
				return diag02, true
			}
			return diag13, true
		}
	}
	return triangulateWorldPolygon(verts)
}

func triangulateByAngleFan(verts []worldPt) ([][3]int, bool) {
	n := len(verts)
	if n < 3 {
		return nil, false
	}
	cx, cy := 0.0, 0.0
	for _, v := range verts {
		cx += v.x
		cy += v.y
	}
	cx /= float64(n)
	cy /= float64(n)

	order := make([]int, n)
	for i := range order {
		order[i] = i
	}
	slices.SortFunc(order, func(a, b int) int {
		ai := math.Atan2(verts[a].y-cy, verts[a].x-cx)
		aj := math.Atan2(verts[b].y-cy, verts[b].x-cx)
		if ai < aj {
			return -1
		}
		if ai > aj {
			return 1
		}
		return 0
	})

	// Reject clearly degenerate results.
	area2 := 0.0
	for i := 0; i < n; i++ {
		a := verts[order[i]]
		b := verts[order[(i+1)%n]]
		area2 += a.x*b.y - b.x*a.y
	}
	if math.Abs(area2) < 1e-6 {
		return nil, false
	}

	tris := make([][3]int, 0, n-2)
	for i := 1; i+1 < n; i++ {
		tris = append(tris, [3]int{order[0], order[i], order[i+1]})
	}
	return tris, len(tris) > 0
}

func polygonSimple(verts []worldPt) bool {
	n := len(verts)
	if n < 3 {
		return false
	}
	for i := 0; i < n; i++ {
		a1 := verts[i]
		a2 := verts[(i+1)%n]
		for j := i + 1; j < n; j++ {
			// Skip adjacent edges and the same closing edge pair.
			if j == i || (j+1)%n == i || j == (i+1)%n {
				continue
			}
			b1 := verts[j]
			b2 := verts[(j+1)%n]
			if segmentsIntersectStrict(a1, a2, b1, b2) {
				return false
			}
		}
	}
	return true
}

func polygonConvex(verts []worldPt) bool {
	n := len(verts)
	if n < 3 {
		return false
	}
	sign := 0
	const eps = 1e-9
	for i := 0; i < n; i++ {
		a := verts[i]
		b := verts[(i+1)%n]
		c := verts[(i+2)%n]
		o := orient2D(a, b, c)
		if math.Abs(o) <= eps {
			continue
		}
		s := 1
		if o < 0 {
			s = -1
		}
		if sign == 0 {
			sign = s
			continue
		}
		if s != sign {
			return false
		}
	}
	return true
}

func segmentsIntersectStrict(a1, a2, b1, b2 worldPt) bool {
	o1 := orient2D(a1, a2, b1)
	o2 := orient2D(a1, a2, b2)
	o3 := orient2D(b1, b2, a1)
	o4 := orient2D(b1, b2, a2)
	return (o1*o2 < 0) && (o3*o4 < 0)
}

func orient2D(a, b, c worldPt) float64 {
	return (b.x-a.x)*(c.y-a.y) - (b.y-a.y)*(c.x-a.x)
}

func isCCW(a, b, c worldPt) bool {
	return orient2D(a, b, c) > 1e-9
}

func containsAnyPointInTri(verts []worldPt, idx []int, ai, bi, ci int) bool {
	a, b, c := verts[ai], verts[bi], verts[ci]
	for _, vi := range idx {
		if vi == ai || vi == bi || vi == ci {
			continue
		}
		if pointInTriStrict(verts[vi], a, b, c) {
			return true
		}
	}
	return false
}

func pointInTriStrict(p, a, b, c worldPt) bool {
	ab := orient2D(a, b, p)
	bc := orient2D(b, c, p)
	ca := orient2D(c, a, p)
	const eps = 1e-8
	return ab > eps && bc > eps && ca > eps
}

func polygonArea2Indexed(verts []worldPt, idx []int) float64 {
	if len(idx) < 3 {
		return 0
	}
	area2 := 0.0
	for i := 0; i < len(idx); i++ {
		j := (i + 1) % len(idx)
		a := verts[idx[i]]
		b := verts[idx[j]]
		area2 += a.x*b.y - b.x*a.y
	}
	return area2
}

func pointOnSegmentWithEps(p, a, b worldPt, eps float64) bool {
	if math.Abs(orient2D(a, b, p)) > eps {
		return false
	}
	minX := math.Min(a.x, b.x) - eps
	maxX := math.Max(a.x, b.x) + eps
	minY := math.Min(a.y, b.y) - eps
	maxY := math.Max(a.y, b.y) + eps
	return p.x >= minX && p.x <= maxX && p.y >= minY && p.y <= maxY
}

func (g *game) subSectorScreenPolygon(ss int) ([]screenPt, []worldPt, float64, float64, polyBBox, bool) {
	verts, cx, cy, ok := g.subSectorWorldVertices(ss)
	if !ok {
		return nil, nil, 0, 0, polyBBox{}, false
	}
	poly := make([]screenPt, 0, len(verts))
	minX, minY := g.viewW-1, g.viewH-1
	maxX, maxY := 0, 0
	for _, v := range verts {
		sx, sy := g.worldToScreen(v.x, v.y)
		poly = append(poly, screenPt{x: sx, y: sy})
		ix := int(math.Round(sx))
		iy := int(math.Round(sy))
		if ix < minX {
			minX = ix
		}
		if ix > maxX {
			maxX = ix
		}
		if iy < minY {
			minY = iy
		}
		if iy > maxY {
			maxY = iy
		}
	}
	if maxX < 0 || maxY < 0 || minX >= g.viewW || minY >= g.viewH {
		return nil, nil, 0, 0, polyBBox{}, false
	}
	if minX < 0 {
		minX = 0
	}
	if minY < 0 {
		minY = 0
	}
	if maxX >= g.viewW {
		maxX = g.viewW - 1
	}
	if maxY >= g.viewH {
		maxY = g.viewH - 1
	}
	if minX > maxX || minY > maxY {
		return nil, nil, 0, 0, polyBBox{}, false
	}
	return poly, verts, cx, cy, polyBBox{minX: minX, minY: minY, maxX: maxX, maxY: maxY}, true
}

func (g *game) subSectorSectorIndex(ss int) (int, bool) {
	if ss < 0 || ss >= len(g.m.SubSectors) {
		return 0, false
	}
	sub := g.m.SubSectors[ss]
	if sub.SegCount == 0 {
		return 0, false
	}
	// Doom associates a subsector with the sector of its first seg.
	firstSeg := int(sub.FirstSeg)
	if sec, ok := g.subSectorSectorFromSeg(firstSeg); ok {
		return sec, true
	}
	return 0, false
}

func (g *game) assignSubSectorSectorsFromPolys() {
	if g == nil || g.m == nil || len(g.m.SubSectors) == 0 || len(g.subSectorSec) != len(g.m.SubSectors) {
		return
	}
	loopSets := g.buildSectorLoopSets()
	if len(loopSets) != len(g.m.Sectors) {
		return
	}
	const eps = 1e-6
	for ss := range g.m.SubSectors {
		if ss < 0 || ss >= len(g.subSectorPoly) || len(g.subSectorPoly[ss]) < 3 {
			continue
		}
		c, ok := worldPolygonCentroid(g.subSectorPoly[ss])
		if !ok {
			continue
		}
		found := -1
		for sec, set := range loopSets {
			if len(set.rings) == 0 {
				continue
			}
			if c.x < set.bbox.minX-eps || c.x > set.bbox.maxX+eps || c.y < set.bbox.minY-eps || c.y > set.bbox.maxY+eps {
				continue
			}
			if pointInRingsEvenOdd(c.x, c.y, set.rings) {
				found = sec
				break
			}
		}
		if found >= 0 && found < len(g.m.Sectors) {
			g.subSectorSec[ss] = found
		}
	}
	if len(g.subSectorPlaneID) != len(g.subSectorSec) {
		g.subSectorPlaneID = make([]int, len(g.subSectorSec))
	}
	if len(g.sectorSubSectors) != len(g.m.Sectors) {
		g.sectorSubSectors = make([][]int, len(g.m.Sectors))
	}
	for sec := range g.sectorSubSectors {
		g.sectorSubSectors[sec] = g.sectorSubSectors[sec][:0]
	}
	for ss, sec := range g.subSectorSec {
		g.subSectorPlaneID[ss] = sec
		if sec >= 0 && sec < len(g.sectorSubSectors) {
			g.sectorSubSectors[sec] = append(g.sectorSubSectors[sec], ss)
		}
	}
	if len(g.staticSubSectorMask) != len(g.m.SubSectors) {
		g.staticSubSectorMask = make([]bool, len(g.m.SubSectors))
	}
	for ss := range g.staticSubSectorMask {
		g.staticSubSectorMask[ss] = false
		sec := -1
		if ss >= 0 && ss < len(g.subSectorSec) {
			sec = g.subSectorSec[ss]
		}
		if sec >= 0 && sec < len(g.dynamicSectorMask) {
			g.staticSubSectorMask[ss] = !g.dynamicSectorMask[sec]
		}
	}
}

func worldPolygonCentroid(poly []worldPt) (worldPt, bool) {
	if len(poly) < 3 {
		return worldPt{}, false
	}
	area2 := 0.0
	cx := 0.0
	cy := 0.0
	for i := 0; i < len(poly); i++ {
		j := (i + 1) % len(poly)
		cross := poly[i].x*poly[j].y - poly[j].x*poly[i].y
		area2 += cross
		cx += (poly[i].x + poly[j].x) * cross
		cy += (poly[i].y + poly[j].y) * cross
	}
	if math.Abs(area2) > 1e-9 {
		inv := 1.0 / (3.0 * area2)
		return worldPt{x: cx * inv, y: cy * inv}, true
	}
	// Degenerate fallback: arithmetic mean.
	sx := 0.0
	sy := 0.0
	for _, p := range poly {
		sx += p.x
		sy += p.y
	}
	return worldPt{x: sx / float64(len(poly)), y: sy / float64(len(poly))}, true
}

func (g *game) buildDynamicSectorMask() []bool {
	if g == nil || g.m == nil || len(g.m.Sectors) == 0 {
		return nil
	}
	dyn := make([]bool, len(g.m.Sectors))
	byTag := make(map[uint16][]int, 64)
	for si, sec := range g.m.Sectors {
		if sec.Tag >= 0 {
			byTag[uint16(sec.Tag)] = append(byTag[uint16(sec.Tag)], si)
		}
		// Timed door sector specials mutate ceiling at runtime.
		if sec.Special == 10 || sec.Special == 14 {
			dyn[si] = true
		}
	}
	for _, ld := range g.m.Linedefs {
		info := mapdata.LookupLineSpecial(ld.Special)
		if info.Door == nil {
			continue
		}
		if info.Door.UsesTag {
			for _, sec := range byTag[ld.Tag] {
				if sec >= 0 && sec < len(dyn) {
					dyn[sec] = true
				}
			}
			continue
		}
		// Manual/un-tagged door specials target the back sidedef sector.
		if ld.SideNum[1] < 0 || int(ld.SideNum[1]) >= len(g.m.Sidedefs) {
			continue
		}
		sec := int(g.m.Sidedefs[int(ld.SideNum[1])].Sector)
		if sec < 0 || sec >= len(dyn) {
			continue
		}
		dyn[sec] = true
	}
	return dyn
}

func (g *game) buildWallSegStaticCache() {
	if g == nil || g.m == nil || len(g.m.Segs) == 0 {
		g.wallSegStaticCache = nil
		return
	}
	if len(g.wallSegStaticCache) != len(g.m.Segs) {
		g.wallSegStaticCache = make([]wallSegStatic, len(g.m.Segs))
	} else {
		for i := range g.wallSegStaticCache {
			g.wallSegStaticCache[i] = wallSegStatic{}
		}
	}
	for si, seg := range g.m.Segs {
		li := int(seg.Linedef)
		if li < 0 || li >= len(g.m.Linedefs) {
			continue
		}
		ld := g.m.Linedefs[li]
		frontSide := int(seg.Direction)
		if frontSide < 0 || frontSide > 1 {
			frontSide = 0
		}
		backSide := frontSide ^ 1
		frontSideDefIdx := -1
		if sn := ld.SideNum[frontSide]; sn >= 0 && int(sn) < len(g.m.Sidedefs) {
			frontSideDefIdx = int(sn)
		}
		if int(seg.StartVertex) >= len(g.m.Vertexes) || int(seg.EndVertex) >= len(g.m.Vertexes) {
			continue
		}
		v1 := g.m.Vertexes[seg.StartVertex]
		v2 := g.m.Vertexes[seg.EndVertex]
		x1w := float64(v1.X)
		y1w := float64(v1.Y)
		x2w := float64(v2.X)
		y2w := float64(v2.Y)
		segLen := math.Hypot(x2w-x1w, y2w-y1w)
		// Projection U tracks seg-local alignment. Sidedef texture offset is
		// applied later at draw time, matching Doom's rw_offset setup.
		uBase := float64(seg.Offset)
		frontSectorIdx := g.sectorIndexFromSideNum(ld.SideNum[frontSide])
		backSectorIdx := g.sectorIndexFromSideNum(ld.SideNum[backSide])
		hasTwoSidedMidTex := false
		if ld.SideNum[frontSide] >= 0 && ld.SideNum[backSide] >= 0 &&
			frontSectorIdx >= 0 && backSectorIdx >= 0 &&
			frontSideDefIdx >= 0 && frontSideDefIdx < len(g.m.Sidedefs) {
			mid := normalizeFlatName(g.m.Sidedefs[frontSideDefIdx].Mid)
			hasTwoSidedMidTex = mid != "" && mid != "-"
		}
		portalSplitStatic, portalSplit := g.cachedSegPortalSplit(frontSectorIdx, backSectorIdx)
		g.wallSegStaticCache[si] = wallSegStatic{
			valid:             true,
			ld:                ld,
			frontSide:         frontSide,
			frontSideDefIdx:   frontSideDefIdx,
			frontSectorIdx:    frontSectorIdx,
			backSectorIdx:     backSectorIdx,
			input:             scene.NewWallPrepassWorldInput(x1w, y1w, x2w, y2w, uBase, segLen, frontSide),
			hasTwoSidedMidTex: hasTwoSidedMidTex,
			portalSplitStatic: portalSplitStatic,
			portalSplit:       portalSplit,
		}
	}
}

func (g *game) cachedSegPortalSplit(frontSectorIdx, backSectorIdx int) (bool, bool) {
	if g == nil || g.m == nil {
		return false, false
	}
	if frontSectorIdx < 0 || backSectorIdx < 0 ||
		frontSectorIdx >= len(g.m.Sectors) || backSectorIdx >= len(g.m.Sectors) {
		return false, false
	}
	if frontSectorIdx < len(g.dynamicSectorMask) && g.dynamicSectorMask[frontSectorIdx] {
		return false, false
	}
	if backSectorIdx < len(g.dynamicSectorMask) && g.dynamicSectorMask[backSectorIdx] {
		return false, false
	}
	if g.sectorLightKindCached(frontSectorIdx) != sectorLightEffectNone ||
		g.sectorLightKindCached(backSectorIdx) != sectorLightEffectNone {
		return false, false
	}
	front := &g.m.Sectors[frontSectorIdx]
	back := &g.m.Sectors[backSectorIdx]
	portalSplit := front.FloorHeight != back.FloorHeight ||
		front.CeilingHeight != back.CeilingHeight ||
		normalizeFlatName(front.FloorPic) != normalizeFlatName(back.FloorPic) ||
		normalizeFlatName(front.CeilingPic) != normalizeFlatName(back.CeilingPic) ||
		g.sectorsLightDifferForRender(frontSectorIdx, backSectorIdx, front, back)
	return true, portalSplit
}

func (g *game) initSubSectorSectorCache() {
	if g.m == nil || len(g.m.SubSectors) == 0 {
		g.subSectorSec = nil
		g.sectorBBox = nil
		g.subSectorLoopVerts = nil
		g.subSectorLoopDiag = nil
		g.subSectorPoly = nil
		g.subSectorTris = nil
		g.subSectorBBox = nil
		g.dynamicSectorMask = nil
		g.staticSubSectorMask = nil
		g.subSectorPlaneID = nil
		g.sectorSubSectors = nil
		g.subSectorPolySrc = nil
		g.subSectorDiagCode = nil
		g.mapTexDiagStats = mapTexDiagStats{}
		g.holeFillPolys = nil
		g.sectorPlaneTris = nil
		g.sectorPlaneCache = nil
		g.orphanSubSector = nil
		g.orphanRepairQueue = nil
		g.wallSegStaticCache = nil
		return
	}
	g.subSectorSec = make([]int, len(g.m.SubSectors))
	g.sectorBBox = buildSectorBBoxCache(g.m)
	g.subSectorLoopVerts = make([][]uint16, len(g.m.SubSectors))
	g.subSectorLoopDiag = make([]loopBuildDiag, len(g.m.SubSectors))
	g.subSectorBBox = make([]worldBBox, len(g.m.SubSectors))
	g.subSectorPoly = make([][]worldPt, len(g.m.SubSectors))
	g.subSectorTris = make([][][3]int, len(g.m.SubSectors))
	g.dynamicSectorMask = g.buildDynamicSectorMask()
	g.staticSubSectorMask = make([]bool, len(g.m.SubSectors))
	g.subSectorPlaneID = make([]int, len(g.m.SubSectors))
	g.sectorSubSectors = make([][]int, len(g.m.Sectors))
	g.subSectorPolySrc = make([]uint8, len(g.m.SubSectors))
	g.subSectorDiagCode = make([]uint8, len(g.m.SubSectors))
	g.mapTexDiagStats = mapTexDiagStats{}
	g.holeFillPolys = nil
	g.orphanSubSector = make([]bool, len(g.m.SubSectors))
	g.orphanRepairQueue = nil
	for i := range g.subSectorSec {
		g.subSectorSec[i] = -1
		g.subSectorBBox[i] = worldBBox{
			minX: math.Inf(1),
			minY: math.Inf(1),
			maxX: math.Inf(-1),
			maxY: math.Inf(-1),
		}
		g.subSectorPlaneID[i] = -1
	}
	for ss := range g.m.SubSectors {
		if sec, ok := g.subSectorSectorIndex(ss); ok {
			g.subSectorSec[ss] = sec
			g.subSectorPlaneID[ss] = sec
			if sec >= 0 && sec < len(g.sectorSubSectors) {
				g.sectorSubSectors[sec] = append(g.sectorSubSectors[sec], ss)
			}
			if sec >= 0 && sec < len(g.dynamicSectorMask) {
				g.staticSubSectorMask[ss] = !g.dynamicSectorMask[sec]
			}
		}
		if b, ok := g.subSectorSegBBox(ss); ok {
			g.subSectorBBox[ss] = b
		}
	}
	g.buildSubSectorLoopCache()
	g.buildCanonicalSubSectorMeshCache()

	// Primary source: per-SSECTOR convex loops from local seg ranges.
	g.buildSubSectorPolysFromSegLoops()
	// Step-2 coverage fallback for degenerate BSP leaves (numsegs<3): synthesize
	// clipped node-leaf polygons so polygon-fill mode does not punch holes.
	g.buildSubSectorPolysFromNodes()
	g.constrainAmbiguousNodePolysToSectorBounds()

	for ss := range g.m.SubSectors {
		if len(g.subSectorPoly[ss]) >= 3 {
			// Keep BSP-derived shapes inside the subsector's supporting seg planes.
			if clipped := g.clipSubSectorPolyBySegBounds(ss, g.subSectorPoly[ss]); len(clipped) >= 3 {
				g.subSectorPoly[ss] = clipped
			}
		}
		if len(g.subSectorPoly[ss]) < 3 {
			if verts, _, _, ok := g.subSectorWorldVertices(ss); ok && len(verts) >= 3 {
				g.subSectorPoly[ss] = verts
				if len(g.subSectorLoopVerts[ss]) >= 3 {
					g.subSectorPolySrc[ss] = subPolySrcPrebuiltLoop
				} else {
					g.subSectorPolySrc[ss] = subPolySrcWorld
				}
			} else if verts, _, _, ok := g.subSectorConvexVertices(ss); ok && len(verts) >= 3 {
				g.subSectorPoly[ss] = verts
				g.subSectorPolySrc[ss] = subPolySrcConvex
			} else if verts, _, _, ok := g.subSectorVerticesFromSegList(ss); ok && len(verts) >= 3 {
				g.subSectorPoly[ss] = verts
				g.subSectorPolySrc[ss] = subPolySrcSegList
			}
		}
		if len(g.subSectorPoly[ss]) < 3 {
			continue
		}
		p := g.subSectorPoly[ss]
		if math.Abs(polygonArea2(p)) < 1e-6 || !polygonSimple(p) {
			g.subSectorPoly[ss] = nil
			g.subSectorPolySrc[ss] = subPolySrcNone
			continue
		}
		if polygonArea2(p) < 0 {
			for i, j := 0, len(p)-1; i < j; i, j = i+1, j-1 {
				p[i], p[j] = p[j], p[i]
			}
			g.subSectorPoly[ss] = p
		}
	}
	g.buildSubSectorTriCache()
	g.buildHoleFillPolys()
	g.buildSectorPlaneTriCache()
	if g.recoverOrphanSubSectorPolysByBSPCoverage() > 0 {
		g.buildSubSectorTriCache()
		g.buildHoleFillPolys()
		g.buildSectorPlaneTriCache()
	}
	if g.recoverOrphanSubSectorPolysSecondPass() > 0 {
		g.buildSubSectorTriCache()
		g.buildHoleFillPolys()
		g.buildSectorPlaneTriCache()
	}
	g.orphanSubSector = g.detectOrphanSubSectorsByBSPCoverage()
	g.initSectorPlaneLevelCache()
	g.updateMapTextureDiagCache()
	g.buildWallSegStaticCache()
}

func (g *game) buildSubSectorLoopCache() {
	if g == nil || g.m == nil || len(g.m.SubSectors) == 0 {
		g.subSectorLoopVerts = nil
		return
	}
	if len(g.subSectorLoopVerts) != len(g.m.SubSectors) {
		g.subSectorLoopVerts = make([][]uint16, len(g.m.SubSectors))
	}
	for ss, sub := range g.m.SubSectors {
		g.subSectorLoopVerts[ss] = nil
		g.subSectorLoopDiag[ss] = loopDiagOK
		if sub.SegCount < 3 {
			continue
		}
		chain, diag, ok := subsectorVertexLoopFromSegOrderDiag(g.m, sub)
		if !ok || len(chain) < 3 {
			if fb, fbOK := subsectorVertexLoopByEdgeChain(g.m, sub); fbOK && len(fb) >= 3 {
				chain = fb
			} else {
				g.subSectorLoopDiag[ss] = diag
				continue
			}
		}
		g.subSectorLoopVerts[ss] = append(g.subSectorLoopVerts[ss], simplifyVertexChainIndices(g.m, chain)...)
	}
}

func (g *game) buildCanonicalSubSectorMeshCache() {
	if g == nil || g.m == nil || len(g.m.SubSectors) == 0 {
		return
	}
	for ss, sub := range g.m.SubSectors {
		if ss >= len(g.subSectorPoly) || ss >= len(g.subSectorTris) || ss >= len(g.subSectorLoopVerts) {
			continue
		}
		if len(g.subSectorPoly[ss]) >= 3 && len(g.subSectorTris[ss]) > 0 {
			continue
		}
		loop := g.subSectorLoopVerts[ss]
		if len(loop) < 3 {
			continue
		}
		// Canonical fast-path only accepts loops that map 1:1 to subsector seg edges.
		// Loops that required inserted connector edges are left to node/other fallback paths.
		if len(loop) != int(sub.SegCount) {
			continue
		}
		verts := vertexChainToWorld(g.m, loop)
		if len(verts) < 3 || math.Abs(polygonArea2(verts)) < 1e-6 || !polygonSimple(verts) {
			continue
		}
		if !normalizeAndValidateConvexFanPolygon(verts) {
			continue
		}
		tris := make([][3]int, 0, len(verts)-2)
		for i := 1; i+1 < len(verts); i++ {
			tris = append(tris, [3]int{0, i, i + 1})
		}
		if len(tris) == 0 {
			continue
		}

		g.subSectorPoly[ss] = verts
		g.subSectorTris[ss] = tris
		if ss < len(g.subSectorPolySrc) {
			g.subSectorPolySrc[ss] = subPolySrcPrebuiltLoop
		}
	}
}

func (g *game) buildSubSectorPolysFromSegLoops() {
	if g.m == nil || len(g.m.SubSectors) == 0 {
		return
	}
	if len(g.subSectorPoly) != len(g.m.SubSectors) {
		return
	}
	loopSets := g.buildSectorLoopSets()
	for ss := range g.m.SubSectors {
		loop := []uint16(nil)
		src := subPolySrcPrebuiltLoop
		if ss < len(g.subSectorLoopVerts) && len(g.subSectorLoopVerts[ss]) >= 3 {
			loop = g.subSectorLoopVerts[ss]
		} else {
			sub := g.m.SubSectors[ss]
			if chain, _, ok := subsectorVertexLoopFromSegOrderDiag(g.m, sub); ok && len(chain) >= 3 {
				loop = chain
			} else if chain, ok := subsectorVertexLoopByEdgeChain(g.m, sub); ok && len(chain) >= 3 {
				loop = chain
				src = subPolySrcEdgeChain
			}
		}
		if len(loop) < 3 {
			continue
		}
		verts := vertexChainToWorld(g.m, loop)
		if len(verts) < 3 {
			continue
		}
		if !polygonSimple(verts) {
			continue
		}
		if !normalizeAndValidateConvexFanPolygon(verts) {
			continue
		}
		sec := -1
		if ss < len(g.subSectorSec) {
			sec = g.subSectorSec[ss]
		}
		if sec < 0 || sec >= len(g.m.Sectors) {
			if s, ok := g.subSectorSectorIndex(ss); ok {
				sec = s
			}
		}
		if !polygonInsideSectorLoops(verts, sec, loopSets) {
			continue
		}
		g.subSectorPoly[ss] = verts
		if ss < len(g.subSectorPolySrc) {
			g.subSectorPolySrc[ss] = src
		}
	}
}

func (g *game) buildSubSectorTriCache() {
	if g.m == nil || len(g.m.SubSectors) == 0 {
		g.subSectorTris = nil
		return
	}
	if len(g.subSectorTris) != len(g.m.SubSectors) {
		g.subSectorTris = make([][][3]int, len(g.m.SubSectors))
	}
	for ss := range g.m.SubSectors {
		g.subSectorTris[ss] = nil
		if ss >= len(g.subSectorPoly) || len(g.subSectorPoly[ss]) < 3 {
			continue
		}
		verts := g.subSectorPoly[ss]
		tris, ok := triangulateWorldPolygonQuadFirst(verts)
		if !ok {
			tris, ok = triangulateByAngleFan(verts)
		}
		if !ok || len(tris) == 0 {
			continue
		}
		g.subSectorTris[ss] = tris
	}
}

func (g *game) buildSectorPlaneTriCache() {
	if g == nil || g.m == nil || len(g.m.Sectors) == 0 {
		g.sectorPlaneTris = nil
		g.sectorPlaneCache = nil
		return
	}
	g.sectorPlaneTris = make([][]worldTri, len(g.m.Sectors))
	loopSets := g.buildSectorLoopSets()
	for sec := range g.m.Sectors {
		g.sectorPlaneTris[sec] = g.buildSectorPlaneTrisForSector(sec, loopSets)
	}
}

func (g *game) buildSectorPlaneTrisForSector(sec int, loopSets []sectorLoopSet) []worldTri {
	if g == nil || g.m == nil || sec < 0 || sec >= len(g.m.Sectors) {
		return nil
	}
	out := make([]worldTri, 0, 16)
	ssList := []int(nil)
	if sec >= 0 && sec < len(g.sectorSubSectors) && len(g.sectorSubSectors[sec]) > 0 {
		ssList = g.sectorSubSectors[sec]
	} else {
		ssList = make([]int, 0, len(g.m.SubSectors))
		for ss := range g.m.SubSectors {
			ssList = append(ssList, ss)
		}
	}
	for _, ss := range ssList {
		ssec := -1
		if ss < len(g.subSectorSec) {
			ssec = g.subSectorSec[ss]
		}
		if ssec < 0 || ssec >= len(g.m.Sectors) {
			if s, ok := g.subSectorSectorIndex(ss); ok {
				ssec = s
			}
		}
		if ssec != sec {
			continue
		}
		// Consume cached subsector mesh only; do not rebuild direct world fans here.
		if !g.ensureSubSectorPolyAndTris(ss) || ss >= len(g.subSectorPoly) || ss >= len(g.subSectorTris) {
			continue
		}
		verts := g.subSectorPoly[ss]
		tris := g.subSectorTris[ss]
		if len(verts) < 3 || len(tris) == 0 {
			continue
		}
		for _, tri := range tris {
			i0, i1, i2 := tri[0], tri[1], tri[2]
			if i0 < 0 || i1 < 0 || i2 < 0 || i0 >= len(verts) || i1 >= len(verts) || i2 >= len(verts) {
				continue
			}
			out = append(out, worldTri{a: verts[i0], b: verts[i1], c: verts[i2]})
		}
	}
	// Merge hole-fill patches into the main per-sector triangle cache so the
	// primary draw path includes them and we don't depend on a secondary pass.
	for _, hp := range g.holeFillPolys {
		if hp.sector != sec || len(hp.verts) < 3 || len(hp.tris) == 0 {
			continue
		}
		for _, tri := range hp.tris {
			i0, i1, i2 := tri[0], tri[1], tri[2]
			if i0 < 0 || i1 < 0 || i2 < 0 || i0 >= len(hp.verts) || i1 >= len(hp.verts) || i2 >= len(hp.verts) {
				continue
			}
			out = append(out, worldTri{
				a: hp.verts[i0],
				b: hp.verts[i1],
				c: hp.verts[i2],
			})
		}
	}
	return out
}

func (g *game) sectorHeightSnapshot(sec int) (int64, int64, bool) {
	if g == nil || g.m == nil || sec < 0 || sec >= len(g.m.Sectors) {
		return 0, 0, false
	}
	if sec < len(g.sectorFloor) && sec < len(g.sectorCeil) {
		return g.sectorFloor[sec], g.sectorCeil[sec], true
	}
	return int64(g.m.Sectors[sec].FloorHeight) << fracBits, int64(g.m.Sectors[sec].CeilingHeight) << fracBits, true
}

func (g *game) sectorHeightRenderSnapshot(sec int) (int64, int64, bool) {
	floor, ceil, ok := g.sectorHeightSnapshot(sec)
	if !ok || g == nil {
		return floor, ceil, ok
	}
	if sec < 0 || sec >= len(g.sectorCeil) {
		return floor, ceil, true
	}
	d, moving := g.doors[sec]
	if !moving || d == nil || (d.direction != -1 && d.direction != 1) {
		return floor, ceil, true
	}
	alpha := g.renderAlpha
	if alpha <= 0 {
		return floor, ceil, true
	}
	step := int64(math.Round(float64(d.speed) * alpha))
	if step <= 0 {
		return floor, ceil, true
	}
	if d.direction < 0 {
		ceil -= step
		if ceil < floor {
			ceil = floor
		}
	} else {
		ceil += step
		if ceil > d.topHeight {
			ceil = d.topHeight
		}
	}
	if ceil < floor {
		ceil = floor
	}
	return floor, ceil, true
}

func (g *game) sectorLightingSnapshot(sec int) (int16, uint8, sectorLightEffectKind, bool) {
	if g == nil || g.m == nil || sec < 0 || sec >= len(g.m.Sectors) {
		return 0, 0, sectorLightEffectNone, false
	}
	light := g.m.Sectors[sec].Light
	kind := sectorLightEffectKind(sectorLightEffectNone)
	if sec < len(g.sectorLightFx) {
		kind = g.sectorLightFx[sec].kind
	}
	return light, uint8(sectorLightMul(light)), kind, true
}

func (g *game) initSectorPlaneLevelCache() {
	if g == nil || g.m == nil || len(g.m.Sectors) == 0 {
		g.sectorPlaneCache = nil
		g.sectorLightCacheValid = false
		return
	}
	if len(g.sectorPlaneTris) != len(g.m.Sectors) {
		g.buildSectorPlaneTriCache()
	}
	g.sectorPlaneCache = make([]sectorPlaneCacheEntry, len(g.m.Sectors))
	for sec := range g.m.Sectors {
		floor, ceil, _ := g.sectorHeightSnapshot(sec)
		light, lightMul, lightKind, _ := g.sectorLightingSnapshot(sec)
		dyn := sec < len(g.dynamicSectorMask) && g.dynamicSectorMask[sec]
		g.sectorPlaneCache[sec] = sectorPlaneCacheEntry{
			tris:         append([]worldTri(nil), g.sectorPlaneTris[sec]...),
			dynamic:      dyn,
			lastFloor:    floor,
			lastCeil:     ceil,
			dirty:        false,
			prevLight:    light,
			light:        light,
			prevLightMul: lightMul,
			lightMul:     lightMul,
			lightKind:    lightKind,
			texTick:      -1,
		}
	}
	g.sectorLightCacheTick = g.worldTic
	g.sectorLightCacheValid = true
}

func (g *game) markDynamicSectorPlaneCacheDirty(sec int) {
	if g == nil || sec < 0 || sec >= len(g.sectorPlaneCache) {
		return
	}
	if !g.sectorPlaneCache[sec].dynamic {
		return
	}
	g.sectorPlaneCache[sec].dirty = true
}

func (g *game) refreshDynamicSectorPlaneCache() {
	if g == nil || g.m == nil || len(g.m.Sectors) == 0 {
		return
	}
	for sec := range g.sectorPlaneCache {
		entry := &g.sectorPlaneCache[sec]
		if !entry.dynamic {
			continue
		}
		floor, ceil, ok := g.sectorHeightSnapshot(sec)
		if !ok {
			continue
		}
		if floor != entry.lastFloor || ceil != entry.lastCeil {
			entry.lastFloor = floor
			entry.lastCeil = ceil
			entry.dirty = true
		}
		if !entry.dirty {
			continue
		}
		entry.tris = g.buildSectorPlaneTrisForSector(sec, nil)
		g.sectorPlaneTris[sec] = append(g.sectorPlaneTris[sec][:0], entry.tris...)
		entry.dirty = false
	}
}

func (g *game) refreshSectorPlaneCacheLighting() {
	if g == nil || g.m == nil || len(g.sectorPlaneCache) != len(g.m.Sectors) {
		return
	}
	for sec := range g.sectorPlaneCache {
		light, lightMul, lightKind, ok := g.sectorLightingSnapshot(sec)
		if !ok {
			continue
		}
		g.sectorPlaneCache[sec].light = light
		g.sectorPlaneCache[sec].lightMul = lightMul
		g.sectorPlaneCache[sec].lightKind = lightKind
	}
	g.sectorLightCacheTick = g.worldTic
	g.sectorLightCacheValid = true
}

func (g *game) ensureSectorPlaneCacheLightingFresh() {
	if g == nil || !g.sectorLightCacheValid || g.sectorLightCacheTick == g.worldTic {
		return
	}
	g.refreshSectorPlaneCacheLighting()
}

func (g *game) ensureSectorPlaneLevelCacheFresh() {
	if g == nil || g.m == nil || len(g.m.Sectors) == 0 {
		return
	}
	if len(g.sectorPlaneTris) != len(g.m.Sectors) {
		g.buildSectorPlaneTriCache()
	}
	if len(g.sectorPlaneCache) != len(g.m.Sectors) {
		g.initSectorPlaneLevelCache()
	}
	g.ensureSectorPlaneCacheLightingFresh()
	g.refreshDynamicSectorPlaneCache()
}

func (g *game) refreshSectorPlaneCacheTextureRefs() {
	if g == nil || g.m == nil || len(g.sectorPlaneCache) != len(g.m.Sectors) {
		return
	}
	animTick := g.textureAnimTick()
	for sec := range g.sectorPlaneCache {
		entry := &g.sectorPlaneCache[sec]
		flatRGBA, _ := g.flatRGBA(g.m.Sectors[sec].FloorPic)
		texID := normalizeFlatName(g.m.Sectors[sec].FloorPic)
		if entry.tex != nil && entry.texID == texID && entry.texTick == animTick {
			continue
		}
		entry.texID = texID
		entry.texTick = animTick
		entry.tex = nil
		entry.flatRGBA = flatRGBA
		if img, ok := g.flatImage(g.m.Sectors[sec].FloorPic); ok {
			entry.tex = img
		}
	}
}

func (g *game) sectorPlaneTrisCached(sec int) []worldTri {
	if g != nil && sec >= 0 && sec < len(g.sectorPlaneCache) {
		return g.sectorPlaneCache[sec].tris
	}
	if g != nil && sec >= 0 && sec < len(g.sectorPlaneTris) {
		return g.sectorPlaneTris[sec]
	}
	return nil
}

func (g *game) sectorLightLevelCached(sec int) int16 {
	if g != nil && g.sectorLightCacheValid {
		g.ensureSectorPlaneCacheLightingFresh()
	}
	if g != nil && sec >= 0 && sec < len(g.sectorPlaneCache) {
		entry := g.sectorPlaneCache[sec]
		if !g.sectorLightCacheValid && entry.light == 0 && entry.prevLight == 0 && g.m != nil && sec < len(g.m.Sectors) {
			return g.m.Sectors[sec].Light
		}
		if entry.lightKind == sectorLightEffectNone {
			return entry.light
		}
		return g.interpolateSectorLight(entry.prevLight, entry.light)
	}
	if g != nil && g.m != nil && sec >= 0 && sec < len(g.m.Sectors) {
		return g.m.Sectors[sec].Light
	}
	return 160
}

func (g *game) sectorLightMulCached(sec int) uint32 {
	if g.playerInfraredBright() {
		return 256
	}
	if g != nil && g.sectorLightCacheValid {
		g.ensureSectorPlaneCacheLightingFresh()
	}
	if g != nil && sec >= 0 && sec < len(g.sectorPlaneCache) {
		entry := g.sectorPlaneCache[sec]
		if !g.sectorLightCacheValid && entry.lightMul == 0 && entry.prevLightMul == 0 && g.m != nil && sec < len(g.m.Sectors) {
			return uint32(sectorLightMul(g.m.Sectors[sec].Light))
		}
		if entry.lightKind == sectorLightEffectNone {
			return uint32(entry.lightMul)
		}
		return uint32(g.interpolateSectorLightMul(entry.prevLightMul, entry.lightMul))
	}
	return uint32(sectorLightMul(g.sectorLightLevelCached(sec)))
}

func (g *game) sectorLightKindCached(sec int) sectorLightEffectKind {
	if g != nil && g.sectorLightCacheValid {
		g.ensureSectorPlaneCacheLightingFresh()
	}
	if g != nil && sec >= 0 && sec < len(g.sectorPlaneCache) {
		return g.sectorPlaneCache[sec].lightKind
	}
	return sectorLightEffectNone
}

func (g *game) interpolateSectorLight(prev, curr int16) int16 {
	if g == nil || !g.opts.SourcePortMode {
		return curr
	}
	alpha := g.renderAlpha
	if alpha <= 0 {
		return prev
	}
	if alpha >= 1 {
		return curr
	}
	light := int(math.Round(lerp(float64(prev), float64(curr), alpha)))
	if light < 0 {
		light = 0
	}
	if light > 255 {
		light = 255
	}
	return int16(light)
}

func (g *game) interpolateSectorLightMul(prev, curr uint8) uint8 {
	if g == nil || !g.opts.SourcePortMode {
		return curr
	}
	alpha := g.renderAlpha
	if alpha <= 0 {
		return prev
	}
	if alpha >= 1 {
		return curr
	}
	mul := int(math.Round(lerp(float64(prev), float64(curr), alpha)))
	if mul < 0 {
		mul = 0
	}
	if mul > 255 {
		mul = 255
	}
	return uint8(mul)
}

func triangleInsideSectorLoops(t worldTri, sec int, loopSets []sectorLoopSet) bool {
	if sec < 0 || sec >= len(loopSets) || len(loopSets[sec].rings) == 0 {
		return false
	}
	rings := loopSets[sec].rings
	const eps = 1e-6
	insideOrEdge := func(p worldPt) bool {
		if pointInRingsEvenOdd(p.x, p.y, rings) {
			return true
		}
		return pointOnAnyRingEdge(p, rings, eps)
	}
	// Use centroid containment as acceptance gate. Requiring all three vertices
	// to be inside is too strict for boundary-aligned triangles and causes
	// persistent one-triangle holes.
	c := worldPt{x: (t.a.x + t.b.x + t.c.x) / 3.0, y: (t.a.y + t.b.y + t.c.y) / 3.0}
	if !insideOrEdge(c) {
		return false
	}
	return true
}

func polygonInsideSectorLoops(poly []worldPt, sec int, loopSets []sectorLoopSet) bool {
	if len(poly) < 3 || sec < 0 || sec >= len(loopSets) || len(loopSets[sec].rings) == 0 {
		return false
	}
	rings := loopSets[sec].rings
	const eps = 1e-6
	for _, v := range poly {
		if !pointInRingsOrOnEdge(v, rings, eps) {
			return false
		}
	}
	c, ok := worldPolygonCentroid(poly)
	if !ok {
		return false
	}
	return pointInRingsOrOnEdge(c, rings, eps)
}

func pointInRingsOrOnEdge(p worldPt, rings [][]worldPt, eps float64) bool {
	if pointInRingsEvenOdd(p.x, p.y, rings) {
		return true
	}
	return pointOnAnyRingEdge(p, rings, eps)
}

func pointOnAnyRingEdge(p worldPt, rings [][]worldPt, eps float64) bool {
	e2 := eps * eps
	for _, ring := range rings {
		if len(ring) < 2 {
			continue
		}
		for i := 0; i < len(ring); i++ {
			a := ring[i]
			b := ring[(i+1)%len(ring)]
			if pointSegmentDist2(p, a, b) <= e2 {
				return true
			}
		}
	}
	return false
}

func pointSegmentDist2(p, a, b worldPt) float64 {
	vx := b.x - a.x
	vy := b.y - a.y
	wx := p.x - a.x
	wy := p.y - a.y
	den := vx*vx + vy*vy
	if den <= 1e-12 {
		dx := p.x - a.x
		dy := p.y - a.y
		return dx*dx + dy*dy
	}
	t := (wx*vx + wy*vy) / den
	if t < 0 {
		t = 0
	} else if t > 1 {
		t = 1
	}
	cx := a.x + vx*t
	cy := a.y + vy*t
	dx := p.x - cx
	dy := p.y - cy
	return dx*dx + dy*dy
}

func (g *game) ensureSubSectorPolyAndTris(ss int) bool {
	if g.m == nil || ss < 0 || ss >= len(g.m.SubSectors) {
		return false
	}
	if ss >= len(g.subSectorPoly) {
		return false
	}
	if len(g.subSectorPoly[ss]) < 3 {
		verts, ok := g.subSectorWorldPolyCached(ss)
		if !ok || len(verts) < 3 {
			return false
		}
		g.subSectorPoly[ss] = verts
		if ss < len(g.subSectorBBox) {
			if b, ok := g.subSectorSegBBox(ss); ok {
				g.subSectorBBox[ss] = b
			}
		}
	}
	if ss >= len(g.subSectorTris) {
		return false
	}
	if len(g.subSectorTris[ss]) == 0 {
		verts := g.subSectorPoly[ss]
		tris, ok := triangulateWorldPolygonQuadFirst(verts)
		if !ok {
			tris, ok = triangulateByAngleFan(verts)
		}
		if !ok || len(tris) == 0 {
			return false
		}
		g.subSectorTris[ss] = tris
	}
	return true
}

func holeQuantizeWorldPt(p worldPt) holeQuantPt {
	const q = 64.0
	return holeQuantPt{
		x: int64(math.Round(p.x * q)),
		y: int64(math.Round(p.y * q)),
	}
}

func holeQuantLess(a, b holeQuantPt) bool {
	if a.x != b.x {
		return a.x < b.x
	}
	return a.y < b.y
}

func canonicalHoleEdgeKey(a, b holeQuantPt) (holeEdgeKey, bool) {
	if holeQuantLess(a, b) {
		return holeEdgeKey{ax: a.x, ay: a.y, bx: b.x, by: b.y}, true
	}
	return holeEdgeKey{ax: b.x, ay: b.y, bx: a.x, by: a.y}, false
}

func (g *game) buildHoleFillPolys() {
	g.holeFillPolys = nil
	if g.m == nil || len(g.m.SubSectors) == 0 || len(g.subSectorPoly) == 0 {
		return
	}
	loopSets := g.buildSectorLoopSets()

	perSector := make(map[int]map[holeEdgeKey]*holeEdgeDirBucket)
	for ss := range g.m.SubSectors {
		sec := -1
		if ss < len(g.subSectorSec) {
			sec = g.subSectorSec[ss]
		}
		if sec < 0 || sec >= len(g.m.Sectors) {
			if s, ok := g.subSectorSectorIndex(ss); ok && s >= 0 && s < len(g.m.Sectors) {
				sec = s
			}
		}
		if sec < 0 || sec >= len(g.m.Sectors) {
			continue
		}
		if ss >= len(g.subSectorPoly) || len(g.subSectorPoly[ss]) < 3 {
			continue
		}
		poly := g.subSectorPoly[ss]
		area2 := polygonArea2(poly)
		if math.Abs(area2) < 1e-6 {
			continue
		}
		if area2 < 0 {
			cp := make([]worldPt, len(poly))
			copy(cp, poly)
			for i, j := 0, len(cp)-1; i < j; i, j = i+1, j-1 {
				cp[i], cp[j] = cp[j], cp[i]
			}
			poly = cp
		}

		edgeBuckets, ok := perSector[sec]
		if !ok {
			edgeBuckets = make(map[holeEdgeKey]*holeEdgeDirBucket)
			perSector[sec] = edgeBuckets
		}
		for i := 0; i < len(poly); i++ {
			a := poly[i]
			b := poly[(i+1)%len(poly)]
			qa := holeQuantizeWorldPt(a)
			qb := holeQuantizeWorldPt(b)
			if qa == qb {
				continue
			}
			key, forward := canonicalHoleEdgeKey(qa, qb)
			bucket := edgeBuckets[key]
			if bucket == nil {
				bucket = &holeEdgeDirBucket{}
				edgeBuckets[key] = bucket
			}
			edge := holeBoundaryEdge{a: qa, b: qb, aw: a, bw: b}
			if forward {
				bucket.ab = append(bucket.ab, edge)
			} else {
				bucket.ba = append(bucket.ba, edge)
			}
		}
	}

	out := make([]holeFillPoly, 0, 16)
	for sec, edgeBuckets := range perSector {
		boundary := make([]holeBoundaryEdge, 0, len(edgeBuckets))
		for _, b := range edgeBuckets {
			if len(b.ab) > len(b.ba) {
				boundary = append(boundary, b.ab[:len(b.ab)-len(b.ba)]...)
			} else if len(b.ba) > len(b.ab) {
				boundary = append(boundary, b.ba[:len(b.ba)-len(b.ab)]...)
			}
		}
		if len(boundary) < 3 {
			continue
		}

		adj := make(map[holeQuantPt][]int, len(boundary))
		for i, e := range boundary {
			adj[e.a] = append(adj[e.a], i)
		}
		used := make([]bool, len(boundary))

		for i := range boundary {
			if used[i] {
				continue
			}
			start := boundary[i]
			cur := start
			used[i] = true
			loop := make([]worldPt, 0, 12)
			loop = append(loop, cur.aw)

			closed := false
			guard := 0
			for guard < len(boundary)+4 {
				guard++
				loop = append(loop, cur.bw)
				if cur.b == start.a {
					closed = true
					break
				}
				nextIdx := -1
				for _, cand := range adj[cur.b] {
					if !used[cand] {
						nextIdx = cand
						break
					}
				}
				if nextIdx < 0 {
					break
				}
				used[nextIdx] = true
				cur = boundary[nextIdx]
			}
			if !closed {
				continue
			}
			if len(loop) >= 2 && nearlyEqualWorldPt(loop[0], loop[len(loop)-1], 1e-6) {
				loop = loop[:len(loop)-1]
			}
			if len(loop) < 3 {
				continue
			}
			area2 := polygonArea2(loop)
			if math.Abs(area2) < 1e-6 {
				continue
			}
			if !polygonInsideSectorLoops(loop, sec, loopSets) {
				continue
			}
			// Remaining boundary loops from subsector unions are CCW for outer borders
			// and CW for holes. Only fill CW loops.
			if area2 >= 0 {
				continue
			}
			tris, ok := triangulateWorldPolygonQuadFirst(loop)
			if !ok || len(tris) == 0 {
				continue
			}
			bbox := worldPolyBBox(loop)
			if !isFinite(bbox.minX) || !isFinite(bbox.minY) || !isFinite(bbox.maxX) || !isFinite(bbox.maxY) {
				continue
			}
			out = append(out, holeFillPoly{
				sector: sec,
				verts:  loop,
				tris:   tris,
				bbox:   bbox,
			})
		}

		// Final patch pass: emit tiny orphan triangle pockets directly from the
		// remaining boundary edge graph.
		if sec < 0 || sec >= len(loopSets) || len(loopSets[sec].rings) == 0 {
			continue
		}
		type triKey struct {
			ax, ay int64
			bx, by int64
			cx, cy int64
		}
		type quadKey struct {
			ax, ay int64
			bx, by int64
			cx, cy int64
			dx, dy int64
		}
		order3 := func(a, b, c holeQuantPt) (holeQuantPt, holeQuantPt, holeQuantPt) {
			if holeQuantLess(b, a) {
				a, b = b, a
			}
			if holeQuantLess(c, b) {
				b, c = c, b
			}
			if holeQuantLess(b, a) {
				a, b = b, a
			}
			return a, b, c
		}
		order4 := func(a, b, c, d holeQuantPt) [4]holeQuantPt {
			pts := [4]holeQuantPt{a, b, c, d}
			slices.SortFunc(pts[:], func(a, b holeQuantPt) int {
				if holeQuantLess(a, b) {
					return -1
				}
				if holeQuantLess(b, a) {
					return 1
				}
				return 0
			})
			return pts
		}
		worldByQuant := make(map[holeQuantPt]worldPt, len(boundary)*2)
		adjSet := make(map[holeQuantPt]map[holeQuantPt]struct{}, len(boundary))
		for _, e := range boundary {
			worldByQuant[e.a] = e.aw
			worldByQuant[e.b] = e.bw
			if adjSet[e.a] == nil {
				adjSet[e.a] = make(map[holeQuantPt]struct{}, 4)
			}
			if adjSet[e.b] == nil {
				adjSet[e.b] = make(map[holeQuantPt]struct{}, 4)
			}
			adjSet[e.a][e.b] = struct{}{}
			adjSet[e.b][e.a] = struct{}{}
		}
		seenTri := make(map[triKey]struct{}, 16)
		seenQuad := make(map[quadKey]struct{}, 16)
		for a, nbsA := range adjSet {
			if len(nbsA) < 2 {
				continue
			}
			nbList := make([]holeQuantPt, 0, len(nbsA))
			for b := range nbsA {
				nbList = append(nbList, b)
			}
			for i := 0; i < len(nbList); i++ {
				b := nbList[i]
				for j := i + 1; j < len(nbList); j++ {
					c := nbList[j]
					if _, ok := adjSet[b][c]; !ok {
						continue
					}
					oa, ob, oc := order3(a, b, c)
					key := triKey{
						ax: oa.x, ay: oa.y,
						bx: ob.x, by: ob.y,
						cx: oc.x, cy: oc.y,
					}
					if _, dup := seenTri[key]; dup {
						continue
					}
					seenTri[key] = struct{}{}

					wa, oka := worldByQuant[a]
					wb, okb := worldByQuant[b]
					wc, okc := worldByQuant[c]
					if !oka || !okb || !okc {
						continue
					}
					// Prefer quad patches when these points can be extended to a
					// 4-point pocket; only use single-triangle fill as fallback.
					hasQuadCandidate := false
					for d := range adjSet[a] {
						if d == b || d == c {
							continue
						}
						if _, ok := adjSet[b][d]; !ok {
							continue
						}
						if _, ok := adjSet[c][d]; !ok {
							continue
						}
						hasQuadCandidate = true
						break
					}
					if hasQuadCandidate {
						continue
					}
					triVerts := []worldPt{wa, wb, wc}
					area2 := polygonArea2(triVerts)
					if math.Abs(area2) < 1e-6 {
						continue
					}
					if area2 < 0 {
						triVerts[1], triVerts[2] = triVerts[2], triVerts[1]
					}
					if !polygonInsideSectorLoops(triVerts, sec, loopSets) {
						continue
					}
					centroid := worldPt{
						x: (triVerts[0].x + triVerts[1].x + triVerts[2].x) / 3.0,
						y: (triVerts[0].y + triVerts[1].y + triVerts[2].y) / 3.0,
					}
					if !pointInRingsEvenOdd(centroid.x, centroid.y, loopSets[sec].rings) {
						continue
					}
					if sec >= 0 && sec < len(g.sectorPlaneTris) && pointInAnySectorTri(centroid, g.sectorPlaneTris[sec]) {
						continue
					}
					bbox := worldPolyBBox(triVerts)
					out = append(out, holeFillPoly{
						sector: sec,
						verts:  triVerts,
						tris:   [][3]int{{0, 1, 2}},
						bbox:   bbox,
					})
				}
			}
		}
		// Quad pockets: fill as 2 triangles.
		for a, nbsA := range adjSet {
			for b := range nbsA {
				nbsB := adjSet[b]
				for c := range nbsB {
					if c == a {
						continue
					}
					nbsC := adjSet[c]
					for d := range nbsC {
						if d == a || d == b {
							continue
						}
						if _, ok := adjSet[d][a]; !ok {
							continue
						}
						if _, ok := adjSet[a][b]; !ok {
							continue
						}
						ord := order4(a, b, c, d)
						key := quadKey{
							ax: ord[0].x, ay: ord[0].y,
							bx: ord[1].x, by: ord[1].y,
							cx: ord[2].x, cy: ord[2].y,
							dx: ord[3].x, dy: ord[3].y,
						}
						if _, dup := seenQuad[key]; dup {
							continue
						}
						seenQuad[key] = struct{}{}

						wa, oka := worldByQuant[a]
						wb, okb := worldByQuant[b]
						wc, okc := worldByQuant[c]
						wd, okd := worldByQuant[d]
						if !oka || !okb || !okc || !okd {
							continue
						}
						q := []worldPt{wa, wb, wc, wd}
						if !polygonSimple(q) {
							continue
						}
						qa2 := polygonArea2(q)
						if math.Abs(qa2) < 1e-6 {
							continue
						}
						if qa2 < 0 {
							q[1], q[3] = q[3], q[1]
						}
						if !polygonInsideSectorLoops(q, sec, loopSets) {
							continue
						}
						centroid, ok := worldPolygonCentroid(q)
						if !ok {
							continue
						}
						if sec >= 0 && sec < len(g.sectorPlaneTris) && pointInAnySectorTri(centroid, g.sectorPlaneTris[sec]) {
							continue
						}
						tris, ok := triangulateWorldPolygonQuadFirst(q)
						if !ok || len(tris) == 0 {
							continue
						}
						out = append(out, holeFillPoly{
							sector: sec,
							verts:  q,
							tris:   tris,
							bbox:   worldPolyBBox(q),
						})
					}
				}
			}
		}
	}

	g.holeFillPolys = out
}

func (g *game) subSectorWorldPolyCached(ss int) ([]worldPt, bool) {
	verts, _, _, ok := g.subSectorWorldVertices(ss)
	if !ok {
		verts, _, _, ok = g.subSectorVerticesFromSegList(ss)
	}
	if !ok {
		verts, _, _, ok = g.subSectorConvexVertices(ss)
	}
	if !ok || len(verts) < 3 {
		return nil, false
	}
	return verts, true
}

func (g *game) subSectorAtFixed(x, y int64) int {
	if len(g.m.Nodes) == 0 {
		if len(g.m.SubSectors) == 0 {
			return -1
		}
		return 0
	}
	child := uint16(len(g.m.Nodes) - 1)
	for {
		if child&0x8000 != 0 {
			ss := int(child & 0x7fff)
			if ss < 0 || ss >= len(g.m.SubSectors) {
				return -1
			}
			return ss
		}
		ni := int(child)
		if ni < 0 || ni >= len(g.m.Nodes) {
			return -1
		}
		n := g.m.Nodes[ni]
		dl := divline{
			x:  int64(n.X) << fracBits,
			y:  int64(n.Y) << fracBits,
			dx: int64(n.DX) << fracBits,
			dy: int64(n.DY) << fracBits,
		}
		side := doomPointOnDivlineSide(x, y, dl)
		child = n.ChildID[side]
	}
}

func (g *game) sectorForSubSector(ss int) int {
	if ss >= 0 && ss < len(g.subSectorSec) {
		if sec := g.subSectorSec[ss]; sec >= 0 && sec < len(g.m.Sectors) {
			return sec
		}
	}
	if ss < 0 || ss >= len(g.m.SubSectors) {
		return -1
	}
	s := g.m.SubSectors[ss]
	if int(s.FirstSeg) >= len(g.m.Segs) {
		return -1
	}
	seg := g.m.Segs[s.FirstSeg]
	if int(seg.Linedef) >= len(g.m.Linedefs) {
		return -1
	}
	ld := g.m.Linedefs[seg.Linedef]
	side := int(seg.Direction)
	if side < 0 || side > 1 {
		side = 0
	}
	sideNum := ld.SideNum[side]
	if sideNum < 0 || int(sideNum) >= len(g.m.Sidedefs) {
		return -1
	}
	sec := int(g.m.Sidedefs[int(sideNum)].Sector)
	if sec < 0 || sec >= len(g.m.Sectors) {
		return -1
	}
	return sec
}

func pointOnWorldSegment(p, a, b worldPt) bool {
	const eps = 1e-6
	cross := orient2D(a, b, p)
	if math.Abs(cross) > eps {
		return false
	}
	minX := math.Min(a.x, b.x) - eps
	maxX := math.Max(a.x, b.x) + eps
	minY := math.Min(a.y, b.y) - eps
	maxY := math.Max(a.y, b.y) + eps
	return p.x >= minX && p.x <= maxX && p.y >= minY && p.y <= maxY
}

func pointInWorldPoly(p worldPt, poly []worldPt) bool {
	if len(poly) < 3 {
		return false
	}
	inside := false
	for i, j := 0, len(poly)-1; i < len(poly); j, i = i, i+1 {
		a := poly[j]
		b := poly[i]
		if pointOnWorldSegment(p, a, b) {
			return true
		}
		yiAbove := a.y > p.y
		yjAbove := b.y > p.y
		if yiAbove == yjAbove {
			continue
		}
		xInt := (b.x-a.x)*(p.y-a.y)/(b.y-a.y) + a.x
		if xInt > p.x {
			inside = !inside
		}
	}
	return inside
}

type polyBBox struct {
	minX int
	minY int
	maxX int
	maxY int
}

type worldBBox struct {
	minX float64
	minY float64
	maxX float64
	maxY float64
}

type sectorEdge struct {
	a uint16
	b uint16
}

type sectorLoopSet struct {
	rings [][]worldPt
	bbox  worldBBox
}

func expandWorldBBox(b worldBBox, pad float64) worldBBox {
	return worldBBox{
		minX: b.minX - pad,
		minY: b.minY - pad,
		maxX: b.maxX + pad,
		maxY: b.maxY + pad,
	}
}

func worldBBoxIntersection(a, b worldBBox) (worldBBox, bool) {
	out := worldBBox{
		minX: math.Max(a.minX, b.minX),
		minY: math.Max(a.minY, b.minY),
		maxX: math.Min(a.maxX, b.maxX),
		maxY: math.Min(a.maxY, b.maxY),
	}
	if out.minX >= out.maxX || out.minY >= out.maxY {
		return worldBBox{}, false
	}
	return out, true
}

func worldBBoxArea(b worldBBox) float64 {
	if !isFinite(b.minX) || !isFinite(b.minY) || !isFinite(b.maxX) || !isFinite(b.maxY) {
		return 0
	}
	if b.maxX <= b.minX || b.maxY <= b.minY {
		return 0
	}
	return (b.maxX - b.minX) * (b.maxY - b.minY)
}

func buildSectorBBoxCache(m *mapdata.Map) []worldBBox {
	if m == nil || len(m.Sectors) == 0 {
		return nil
	}
	out := make([]worldBBox, len(m.Sectors))
	for i := range out {
		out[i] = worldBBox{
			minX: math.Inf(1),
			minY: math.Inf(1),
			maxX: math.Inf(-1),
			maxY: math.Inf(-1),
		}
	}
	expand := func(sec int, x, y float64) {
		if sec < 0 || sec >= len(out) {
			return
		}
		if x < out[sec].minX {
			out[sec].minX = x
		}
		if y < out[sec].minY {
			out[sec].minY = y
		}
		if x > out[sec].maxX {
			out[sec].maxX = x
		}
		if y > out[sec].maxY {
			out[sec].maxY = y
		}
	}
	for _, ld := range m.Linedefs {
		if int(ld.V1) >= len(m.Vertexes) || int(ld.V2) >= len(m.Vertexes) {
			continue
		}
		v1 := m.Vertexes[ld.V1]
		v2 := m.Vertexes[ld.V2]
		for _, sn := range ld.SideNum {
			if sn < 0 || int(sn) >= len(m.Sidedefs) {
				continue
			}
			sec := int(m.Sidedefs[int(sn)].Sector)
			expand(sec, float64(v1.X), float64(v1.Y))
			expand(sec, float64(v2.X), float64(v2.Y))
		}
	}
	return out
}

func (g *game) buildMapFloorWorldLayer() bool {
	worldW := math.Max(g.bounds.maxX-g.bounds.minX, 1)
	worldH := math.Max(g.bounds.maxY-g.bounds.minY, 1)
	maxDim := math.Max(worldW, worldH)
	step := 1.0
	if maxDim > 2048 {
		step = math.Ceil(maxDim / 2048.0)
	}
	if step < 1 {
		step = 1
	}

	w := int(math.Ceil(worldW/step)) + 2
	h := int(math.Ceil(worldH/step)) + 2
	if w < 1 || h < 1 {
		return false
	}
	// Guard against pathological allocations on malformed bounds.
	if w > 8192 || h > 8192 {
		return false
	}

	layer := newDebugImage("map-floor-world", w, h)
	pix := make([]byte, w*h*4)

	minX := g.bounds.minX
	maxY := g.bounds.maxY
	g.ensureMapFloorLoopSetsBuilt()
	loops := g.mapFloorLoopSets

	stats := floorFrameStats{}
	for sec := range g.m.Sectors {
		if sec < 0 || sec >= len(loops) {
			continue
		}
		set := loops[sec]
		if len(set.rings) == 0 {
			stats.rejectedSpan++
			stats.rejectNoPoly++
			continue
		}

		tex, texOK := g.flatTextureBlend(g.m.Sectors[sec].FloorPic)
		minPX := int(math.Floor((set.bbox.minX - minX) / step))
		maxPX := int(math.Ceil((set.bbox.maxX - minX) / step))
		minPY := int(math.Floor((maxY - set.bbox.maxY) / step))
		maxPY := int(math.Ceil((maxY - set.bbox.minY) / step))
		if minPX < 0 {
			minPX = 0
		}
		if minPY < 0 {
			minPY = 0
		}
		if maxPX >= w {
			maxPX = w - 1
		}
		if maxPY >= h {
			maxPY = h - 1
		}
		if minPX > maxPX || minPY > maxPY {
			continue
		}

		for py := minPY; py <= maxPY; py++ {
			wy := maxY - (float64(py)+0.5)*step
			row := py * w * 4
			for px := minPX; px <= maxPX; px++ {
				wx := minX + (float64(px)+0.5)*step
				if !pointInRingsEvenOdd(wx, wy, set.rings) {
					continue
				}
				i := row + px*4
				if texOK {
					u := int(math.Floor(wx)) & 63
					v := int(math.Floor(wy)) & 63
					p := sampleFlatBlendPacked(tex, u, v)
					pix[i+0] = uint8((p >> pixelRShift) & 0xFF)
					pix[i+1] = uint8((p >> pixelGShift) & 0xFF)
					pix[i+2] = uint8((p >> pixelBShift) & 0xFF)
					pix[i+3] = 255
					stats.markedCols++
				} else {
					pix[i+0] = wallFloorChange.R
					pix[i+1] = wallFloorChange.G
					pix[i+2] = wallFloorChange.B
					pix[i+3] = 255
					stats.rejectedSpan++
					stats.rejectNoSector++
				}
			}
		}
		stats.emittedSpans++
	}

	g.writePixelsTimed(layer, pix)
	g.mapFloorWorldLayer = layer
	g.mapFloorWorldMinX = minX
	g.mapFloorWorldMaxY = maxY
	g.mapFloorWorldStep = step
	g.mapFloorWorldInit = true
	g.mapFloorWorldStats = stats
	g.mapFloorWorldAnim = g.textureAnimTick()
	g.mapFloorWorldState = fmt.Sprintf("ready %dx%d step=%.0f", w, h, step)
	return true
}

func pointInRingsEvenOdd(x, y float64, rings [][]worldPt) bool {
	p := worldPt{x: x, y: y}
	inside := false
	for _, ring := range rings {
		if len(ring) < 3 {
			continue
		}
		if pointInWorldPoly(p, ring) {
			inside = !inside
		}
	}
	return inside
}

func (g *game) buildSectorLoopSets() []sectorLoopSet {
	if g.m == nil || len(g.m.Sectors) == 0 {
		return nil
	}
	edgeBySector := make([][]sectorEdge, len(g.m.Sectors))
	for _, ld := range g.m.Linedefs {
		v1 := ld.V1
		v2 := ld.V2
		if int(v1) >= len(g.m.Vertexes) || int(v2) >= len(g.m.Vertexes) || v1 == v2 {
			continue
		}
		if ld.SideNum[0] >= 0 && int(ld.SideNum[0]) < len(g.m.Sidedefs) {
			sec := int(g.m.Sidedefs[int(ld.SideNum[0])].Sector)
			if sec >= 0 && sec < len(edgeBySector) {
				edgeBySector[sec] = append(edgeBySector[sec], sectorEdge{a: v1, b: v2})
			}
		}
		if ld.SideNum[1] >= 0 && int(ld.SideNum[1]) < len(g.m.Sidedefs) {
			sec := int(g.m.Sidedefs[int(ld.SideNum[1])].Sector)
			if sec >= 0 && sec < len(edgeBySector) {
				edgeBySector[sec] = append(edgeBySector[sec], sectorEdge{a: v2, b: v1})
			}
		}
	}

	out := make([]sectorLoopSet, len(g.m.Sectors))
	for sec := range out {
		rings := g.extractSectorRings(edgeBySector[sec])
		if len(rings) == 0 {
			continue
		}
		bbox := worldBBox{minX: math.Inf(1), minY: math.Inf(1), maxX: math.Inf(-1), maxY: math.Inf(-1)}
		valid := make([][]worldPt, 0, len(rings))
		for _, ring := range rings {
			if len(ring) < 3 || math.Abs(polygonArea2(ring)) < 1e-6 || !polygonSimple(ring) {
				continue
			}
			valid = append(valid, ring)
			rb := worldPolyBBox(ring)
			if rb.minX < bbox.minX {
				bbox.minX = rb.minX
			}
			if rb.minY < bbox.minY {
				bbox.minY = rb.minY
			}
			if rb.maxX > bbox.maxX {
				bbox.maxX = rb.maxX
			}
			if rb.maxY > bbox.maxY {
				bbox.maxY = rb.maxY
			}
		}
		if len(valid) == 0 {
			continue
		}
		out[sec] = sectorLoopSet{rings: valid, bbox: bbox}
	}
	return out
}

func (g *game) extractSectorRings(edges []sectorEdge) [][]worldPt {
	if len(edges) == 0 {
		return nil
	}
	outgoing := make(map[uint16][]int, len(edges))
	for i, e := range edges {
		outgoing[e.a] = append(outgoing[e.a], i)
	}
	used := make([]bool, len(edges))
	rings := make([][]worldPt, 0, 4)

	for i := range edges {
		if used[i] {
			continue
		}
		start := edges[i].a
		prev := edges[i].a
		curr := edges[i].b
		used[i] = true
		chain := make([]uint16, 0, 16)
		chain = append(chain, start)

		closed := false
		for guard := 0; guard < len(edges)+8; guard++ {
			if curr == start {
				closed = true
				break
			}
			chain = append(chain, curr)
			next := g.chooseNextSectorEdge(prev, curr, edges, used, outgoing)
			if next < 0 {
				break
			}
			used[next] = true
			prev = curr
			curr = edges[next].b
		}
		if !closed || len(chain) < 3 {
			continue
		}
		ring := make([]worldPt, 0, len(chain))
		for _, vi := range chain {
			if int(vi) >= len(g.m.Vertexes) {
				continue
			}
			v := g.m.Vertexes[vi]
			p := worldPt{x: float64(v.X), y: float64(v.Y)}
			if len(ring) > 0 && nearlyEqualWorldPt(ring[len(ring)-1], p, 1e-6) {
				continue
			}
			ring = append(ring, p)
		}
		if len(ring) >= 2 && nearlyEqualWorldPt(ring[0], ring[len(ring)-1], 1e-6) {
			ring = ring[:len(ring)-1]
		}
		if len(ring) >= 3 {
			rings = append(rings, ring)
		}
	}
	return rings
}

func (g *game) chooseNextSectorEdge(prev, curr uint16, edges []sectorEdge, used []bool, outgoing map[uint16][]int) int {
	cands := outgoing[curr]
	if len(cands) == 0 {
		return -1
	}
	prevPt := g.m.Vertexes[prev]
	currPt := g.m.Vertexes[curr]
	pvx := float64(currPt.X - prevPt.X)
	pvy := float64(currPt.Y - prevPt.Y)
	best := -1
	bestScore := -1e100
	for _, ci := range cands {
		if ci < 0 || ci >= len(edges) || used[ci] {
			continue
		}
		nv := edges[ci].b
		if int(nv) >= len(g.m.Vertexes) {
			continue
		}
		nextPt := g.m.Vertexes[nv]
		cvx := float64(nextPt.X - currPt.X)
		cvy := float64(nextPt.Y - currPt.Y)
		dot := pvx*cvx + pvy*cvy
		crs := pvx*cvy - pvy*cvx
		ang := math.Atan2(crs, dot)
		if ang > bestScore {
			bestScore = ang
			best = ci
		}
	}
	return best
}

func worldPolyBBox(poly []worldPt) worldBBox {
	b := worldBBox{
		minX: math.Inf(1),
		minY: math.Inf(1),
		maxX: math.Inf(-1),
		maxY: math.Inf(-1),
	}
	for _, v := range poly {
		if v.x < b.minX {
			b.minX = v.x
		}
		if v.y < b.minY {
			b.minY = v.y
		}
		if v.x > b.maxX {
			b.maxX = v.x
		}
		if v.y > b.maxY {
			b.maxY = v.y
		}
	}
	return b
}

func nodeBBoxToWorld(bb [4]int16) (worldBBox, bool) {
	// Doom node bbox order is top, bottom, left, right.
	top := float64(bb[0])
	bottom := float64(bb[1])
	left := float64(bb[2])
	right := float64(bb[3])
	b := worldBBox{
		minX: math.Min(left, right),
		minY: math.Min(bottom, top),
		maxX: math.Max(left, right),
		maxY: math.Max(bottom, top),
	}
	if !isFinite(b.minX) || !isFinite(b.minY) || !isFinite(b.maxX) || !isFinite(b.maxY) {
		return worldBBox{}, false
	}
	if b.minX > b.maxX || b.minY > b.maxY {
		return worldBBox{}, false
	}
	return b, true
}

func clipWorldPolyByBBox(poly []worldPt, b worldBBox) []worldPt {
	if len(poly) < 3 {
		return nil
	}
	const eps = 1e-6
	clip := func(in []worldPt, inside func(worldPt) bool, intersect func(worldPt, worldPt) worldPt) []worldPt {
		if len(in) < 3 {
			return nil
		}
		out := make([]worldPt, 0, len(in)+2)
		prev := in[len(in)-1]
		prevIn := inside(prev)
		for _, cur := range in {
			curIn := inside(cur)
			if prevIn && curIn {
				out = appendWorldPtUnique(out, cur, eps)
			} else if prevIn && !curIn {
				out = appendWorldPtUnique(out, intersect(prev, cur), eps)
			} else if !prevIn && curIn {
				out = appendWorldPtUnique(out, intersect(prev, cur), eps)
				out = appendWorldPtUnique(out, cur, eps)
			}
			prev = cur
			prevIn = curIn
		}
		if len(out) >= 2 && nearlyEqualWorldPt(out[0], out[len(out)-1], eps) {
			out = out[:len(out)-1]
		}
		if len(out) < 3 {
			return nil
		}
		return out
	}

	out := poly
	out = clip(out, func(p worldPt) bool { return p.x >= b.minX-eps }, func(a, c worldPt) worldPt {
		den := c.x - a.x
		if math.Abs(den) < 1e-12 {
			return worldPt{x: b.minX, y: a.y}
		}
		t := (b.minX - a.x) / den
		return worldPt{x: b.minX, y: a.y + (c.y-a.y)*t}
	})
	out = clip(out, func(p worldPt) bool { return p.x <= b.maxX+eps }, func(a, c worldPt) worldPt {
		den := c.x - a.x
		if math.Abs(den) < 1e-12 {
			return worldPt{x: b.maxX, y: a.y}
		}
		t := (b.maxX - a.x) / den
		return worldPt{x: b.maxX, y: a.y + (c.y-a.y)*t}
	})
	out = clip(out, func(p worldPt) bool { return p.y >= b.minY-eps }, func(a, c worldPt) worldPt {
		den := c.y - a.y
		if math.Abs(den) < 1e-12 {
			return worldPt{x: a.x, y: b.minY}
		}
		t := (b.minY - a.y) / den
		return worldPt{x: a.x + (c.x-a.x)*t, y: b.minY}
	})
	out = clip(out, func(p worldPt) bool { return p.y <= b.maxY+eps }, func(a, c worldPt) worldPt {
		den := c.y - a.y
		if math.Abs(den) < 1e-12 {
			return worldPt{x: a.x, y: b.maxY}
		}
		t := (b.maxY - a.y) / den
		return worldPt{x: a.x + (c.x-a.x)*t, y: b.maxY}
	})
	if len(out) < 3 || math.Abs(polygonArea2(out)) < 1e-6 {
		return nil
	}
	return out
}

func (g *game) subSectorSegBBox(ss int) (worldBBox, bool) {
	if g.m == nil || ss < 0 || ss >= len(g.m.SubSectors) {
		return worldBBox{}, false
	}
	sub := g.m.SubSectors[ss]
	if sub.SegCount == 0 {
		return worldBBox{}, false
	}
	b := worldBBox{
		minX: math.Inf(1),
		minY: math.Inf(1),
		maxX: math.Inf(-1),
		maxY: math.Inf(-1),
	}
	for i := 0; i < int(sub.SegCount); i++ {
		si := int(sub.FirstSeg) + i
		if si < 0 || si >= len(g.m.Segs) {
			continue
		}
		sg := g.m.Segs[si]
		for _, vi := range []uint16{sg.StartVertex, sg.EndVertex} {
			if int(vi) >= len(g.m.Vertexes) {
				continue
			}
			v := g.m.Vertexes[vi]
			x := float64(v.X)
			y := float64(v.Y)
			if x < b.minX {
				b.minX = x
			}
			if y < b.minY {
				b.minY = y
			}
			if x > b.maxX {
				b.maxX = x
			}
			if y > b.maxY {
				b.maxY = y
			}
		}
	}
	if !isFinite(b.minX) || !isFinite(b.minY) || !isFinite(b.maxX) || !isFinite(b.maxY) {
		return worldBBox{}, false
	}
	if b.minX > b.maxX || b.minY > b.maxY {
		return worldBBox{}, false
	}
	return b, true
}

func nearlyEqualWorldPt(a, b worldPt, eps float64) bool {
	return math.Abs(a.x-b.x) <= eps && math.Abs(a.y-b.y) <= eps
}

func appendWorldPtUnique(dst []worldPt, p worldPt, eps float64) []worldPt {
	if len(dst) > 0 && nearlyEqualWorldPt(dst[len(dst)-1], p, eps) {
		return dst
	}
	return append(dst, p)
}

func clipWorldPolyByDivline(poly []worldPt, a, b worldPt, side int) []worldPt {
	if len(poly) < 3 {
		return nil
	}
	const eps = 1e-6
	inside := func(p worldPt) bool {
		o := orient2D(a, b, p)
		if side == 0 {
			return o <= eps
		}
		return o >= -eps
	}
	intersect := func(p1, p2 worldPt) (worldPt, bool) {
		o1 := orient2D(a, b, p1)
		o2 := orient2D(a, b, p2)
		den := o1 - o2
		if math.Abs(den) < 1e-12 {
			return worldPt{}, false
		}
		t := o1 / den
		return worldPt{
			x: p1.x + (p2.x-p1.x)*t,
			y: p1.y + (p2.y-p1.y)*t,
		}, true
	}

	out := make([]worldPt, 0, len(poly)+2)
	prev := poly[len(poly)-1]
	prevIn := inside(prev)
	for _, cur := range poly {
		curIn := inside(cur)
		if prevIn && curIn {
			out = appendWorldPtUnique(out, cur, eps)
		} else if prevIn && !curIn {
			if ip, ok := intersect(prev, cur); ok {
				out = appendWorldPtUnique(out, ip, eps)
			}
		} else if !prevIn && curIn {
			if ip, ok := intersect(prev, cur); ok {
				out = appendWorldPtUnique(out, ip, eps)
			}
			out = appendWorldPtUnique(out, cur, eps)
		}
		prev = cur
		prevIn = curIn
	}
	if len(out) >= 2 && nearlyEqualWorldPt(out[0], out[len(out)-1], eps) {
		out = out[:len(out)-1]
	}
	if len(out) < 3 {
		return nil
	}
	if math.Abs(polygonArea2(out)) < 1e-6 {
		return nil
	}
	return out
}

func (g *game) subSectorSeedPoint(ss int, fallback []worldPt) (worldPt, bool) {
	if _, cx, cy, ok := g.subSectorVerticesFromSegList(ss); ok {
		return worldPt{x: cx, y: cy}, true
	}
	if _, cx, cy, ok := g.subSectorWorldVertices(ss); ok {
		return worldPt{x: cx, y: cy}, true
	}
	if len(fallback) >= 3 {
		cx, cy := 0.0, 0.0
		for _, p := range fallback {
			cx += p.x
			cy += p.y
		}
		return worldPt{x: cx / float64(len(fallback)), y: cy / float64(len(fallback))}, true
	}
	return worldPt{}, false
}

func (g *game) clipSubSectorPolyBySegBounds(ss int, poly []worldPt) []worldPt {
	if ss < 0 || ss >= len(g.m.SubSectors) || len(poly) < 3 {
		return nil
	}
	seed, ok := g.subSectorSeedPoint(ss, poly)
	if !ok {
		return poly
	}
	sub := g.m.SubSectors[ss]
	out := poly
	const sideEps = 1e-7
	for i := 0; i < int(sub.SegCount); i++ {
		si := int(sub.FirstSeg) + i
		if si < 0 || si >= len(g.m.Segs) {
			continue
		}
		sg := g.m.Segs[si]
		if int(sg.StartVertex) >= len(g.m.Vertexes) || int(sg.EndVertex) >= len(g.m.Vertexes) {
			continue
		}
		va := g.m.Vertexes[sg.StartVertex]
		vb := g.m.Vertexes[sg.EndVertex]
		a := worldPt{x: float64(va.X), y: float64(va.Y)}
		b := worldPt{x: float64(vb.X), y: float64(vb.Y)}

		seedSide := orient2D(a, b, seed)
		if math.Abs(seedSide) <= sideEps {
			// Ambiguous seed-on-edge case: choose the side that keeps the larger
			// clipped polygon to avoid precision-driven half-plane flips.
			c0 := clipWorldPolyByDivline(out, a, b, 0)
			c1 := clipWorldPolyByDivline(out, a, b, 1)
			a0 := 0.0
			if len(c0) >= 3 {
				a0 = math.Abs(polygonArea2(c0))
			}
			a1 := 0.0
			if len(c1) >= 3 {
				a1 = math.Abs(polygonArea2(c1))
			}
			switch {
			case a0 == 0 && a1 == 0:
				return nil
			case a1 > a0:
				out = c1
			default:
				out = c0
			}
			continue
		}
		side := 0
		if seedSide > 0 {
			side = 1
		}
		clipped := clipWorldPolyByDivline(out, a, b, side)
		if len(clipped) >= 3 && pointInWorldPoly(seed, clipped) {
			out = clipped
			continue
		}
		if len(clipped) < 3 || !pointInWorldPoly(seed, clipped) {
			alt := clipWorldPolyByDivline(out, a, b, side^1)
			if len(alt) >= 3 {
				clipped = alt
			} else {
				return nil
			}
		}
		out = clipped
	}
	if len(out) < 3 || math.Abs(polygonArea2(out)) < 1e-6 {
		return nil
	}
	return out
}

func (g *game) buildSubSectorPolysFromNodes() {
	if g.m == nil || len(g.m.Nodes) == 0 || len(g.m.SubSectors) == 0 {
		return
	}

	w := math.Max(g.bounds.maxX-g.bounds.minX, 1)
	h := math.Max(g.bounds.maxY-g.bounds.minY, 1)
	pad := math.Max(w, h)*2 + 1024
	root := []worldPt{
		{x: g.bounds.minX - pad, y: g.bounds.minY - pad},
		{x: g.bounds.maxX + pad, y: g.bounds.minY - pad},
		{x: g.bounds.maxX + pad, y: g.bounds.maxY + pad},
		{x: g.bounds.minX - pad, y: g.bounds.maxY + pad},
	}

	var walk func(child uint16, poly []worldPt)
	walk = func(child uint16, poly []worldPt) {
		if len(poly) < 3 {
			return
		}
		if child&0x8000 != 0 {
			ss := int(child & 0x7fff)
			if ss < 0 || ss >= len(g.m.SubSectors) {
				return
			}
			// Node-clipped fallback is only for degenerate leaves (numsegs<3).
			// Regular subsectors must come from ordered seg loops.
			if g.m.SubSectors[ss].SegCount >= 3 {
				return
			}
			if len(g.subSectorPoly[ss]) >= 3 {
				return
			}
			area2 := polygonArea2(poly)
			if len(poly) >= 3 && math.Abs(area2) > 1e-6 {
				cp := make([]worldPt, len(poly))
				copy(cp, poly)
				if area2 < 0 {
					for i, j := 0, len(cp)-1; i < j; i, j = i+1, j-1 {
						cp[i], cp[j] = cp[j], cp[i]
					}
				}
				g.subSectorPoly[ss] = cp
				if ss < len(g.subSectorPolySrc) {
					g.subSectorPolySrc[ss] = subPolySrcNodes
				}
			}
			return
		}
		ni := int(child)
		if ni < 0 || ni >= len(g.m.Nodes) {
			return
		}
		n := g.m.Nodes[ni]
		a := worldPt{x: float64(n.X), y: float64(n.Y)}
		b := worldPt{x: float64(n.X) + float64(n.DX), y: float64(n.Y) + float64(n.DY)}

		p0 := clipWorldPolyByDivline(poly, a, b, 0)
		if len(p0) >= 3 {
			walk(n.ChildID[0], p0)
		}
		p1 := clipWorldPolyByDivline(poly, a, b, 1)
		if len(p1) >= 3 {
			walk(n.ChildID[1], p1)
		}
	}

	walk(uint16(len(g.m.Nodes)-1), root)
}

func (g *game) recoverOrphanSubSectorPolysByBSPCoverage() int {
	if g == nil || g.m == nil || len(g.m.SubSectors) == 0 || len(g.m.Sectors) == 0 {
		return 0
	}
	if len(g.sectorPlaneTris) != len(g.m.Sectors) {
		return 0
	}
	loopSets := g.buildSectorLoopSets()
	if len(loopSets) != len(g.m.Sectors) {
		return 0
	}
	const sampleStep = 16.0
	recovered := 0
	visited := make(map[int]struct{}, 64)
	for sec := range g.m.Sectors {
		set := loopSets[sec]
		if len(set.rings) == 0 {
			continue
		}
		if !isFinite(set.bbox.minX) || !isFinite(set.bbox.minY) || !isFinite(set.bbox.maxX) || !isFinite(set.bbox.maxY) {
			continue
		}
		for y := set.bbox.minY + sampleStep*0.5; y <= set.bbox.maxY; y += sampleStep {
			for x := set.bbox.minX + sampleStep*0.5; x <= set.bbox.maxX; x += sampleStep {
				if !pointInRingsEvenOdd(x, y, set.rings) {
					continue
				}
				if g.pointCoveredBySectorFill(sec, worldPt{x: x, y: y}) {
					continue
				}
				ss := g.subSectorAtFixed(worldToFixed(x), worldToFixed(y))
				if ss < 0 || ss >= len(g.m.SubSectors) {
					continue
				}
				if _, ok := visited[ss]; ok {
					continue
				}
				visited[ss] = struct{}{}
				if ss < len(g.subSectorPoly) && len(g.subSectorPoly[ss]) >= 3 {
					continue
				}
				if g.sectorForSubSector(ss) != sec {
					continue
				}
				if g.recoverMissingSubsectorPolyFromBSP(ss, sec) {
					recovered++
				}
			}
		}
	}
	return recovered
}

func (g *game) recoverOrphanSubSectorPolysSecondPass() int {
	if g == nil || g.m == nil || len(g.m.SubSectors) == 0 || len(g.m.Sectors) == 0 {
		return 0
	}
	if len(g.sectorPlaneTris) != len(g.m.Sectors) {
		return 0
	}
	loopSets := g.buildSectorLoopSets()
	if len(loopSets) != len(g.m.Sectors) {
		return 0
	}

	type vote struct {
		sec   int
		count int
	}
	votes := make(map[int]vote, 64) // ss -> vote
	g.orphanRepairQueue = g.orphanRepairQueue[:0]

	const sampleStep = 8.0
	for sec := range g.m.Sectors {
		set := loopSets[sec]
		if len(set.rings) == 0 {
			continue
		}
		if !isFinite(set.bbox.minX) || !isFinite(set.bbox.minY) || !isFinite(set.bbox.maxX) || !isFinite(set.bbox.maxY) {
			continue
		}
		for y := set.bbox.minY + sampleStep*0.5; y <= set.bbox.maxY; y += sampleStep {
			for x := set.bbox.minX + sampleStep*0.5; x <= set.bbox.maxX; x += sampleStep {
				if !pointInRingsEvenOdd(x, y, set.rings) {
					continue
				}
				if g.pointCoveredBySectorFill(sec, worldPt{x: x, y: y}) {
					continue
				}
				ss, ok := g.bspConsensusLeafAt(x, y, 4.0)
				if !ok || ss < 0 || ss >= len(g.m.SubSectors) {
					continue
				}
				if g.sectorForSubSector(ss) != sec {
					continue
				}
				if ss < len(g.subSectorPoly) && len(g.subSectorPoly[ss]) >= 3 {
					continue
				}
				v := votes[ss]
				v.sec = sec
				v.count++
				votes[ss] = v
			}
		}
	}

	// Record candidates first, then repair in one final pass.
	for ss, v := range votes {
		if v.count < 2 {
			continue
		}
		g.orphanRepairQueue = append(g.orphanRepairQueue, orphanRepairCandidate{
			ss:    ss,
			sec:   v.sec,
			votes: v.count,
		})
	}
	slices.SortFunc(g.orphanRepairQueue, func(a, b orphanRepairCandidate) int {
		if a.votes > b.votes {
			return -1
		}
		if a.votes < b.votes {
			return 1
		}
		return 0
	})

	recovered := 0
	for _, c := range g.orphanRepairQueue {
		ss := c.ss
		if g.recoverMissingSubsectorPolyFromBSP(ss, c.sec) {
			recovered++
		}
	}
	return recovered
}

func (g *game) bspConsensusLeafAt(x, y, delta float64) (int, bool) {
	pts := [][2]float64{
		{x, y},
		{x - delta, y},
		{x + delta, y},
		{x, y - delta},
		{x, y + delta},
	}
	ss0 := -1
	for i, p := range pts {
		ss := g.subSectorAtFixed(worldToFixed(p[0]), worldToFixed(p[1]))
		if ss < 0 {
			return -1, false
		}
		if i == 0 {
			ss0 = ss
			continue
		}
		if ss != ss0 {
			return -1, false
		}
	}
	return ss0, true
}

func (g *game) detectOrphanSubSectorsByBSPCoverage() []bool {
	out := make([]bool, len(g.m.SubSectors))
	if g == nil || g.m == nil || len(g.m.SubSectors) == 0 || len(g.m.Sectors) == 0 {
		return out
	}
	if len(g.sectorPlaneTris) != len(g.m.Sectors) {
		return out
	}
	loopSets := g.buildSectorLoopSets()
	if len(loopSets) != len(g.m.Sectors) {
		return out
	}
	const sampleStep = 8.0
	for sec := range g.m.Sectors {
		set := loopSets[sec]
		if len(set.rings) == 0 {
			continue
		}
		if !isFinite(set.bbox.minX) || !isFinite(set.bbox.minY) || !isFinite(set.bbox.maxX) || !isFinite(set.bbox.maxY) {
			continue
		}
		for y := set.bbox.minY + sampleStep*0.5; y <= set.bbox.maxY; y += sampleStep {
			for x := set.bbox.minX + sampleStep*0.5; x <= set.bbox.maxX; x += sampleStep {
				if !pointInRingsEvenOdd(x, y, set.rings) {
					continue
				}
				if g.pointCoveredBySectorFill(sec, worldPt{x: x, y: y}) {
					continue
				}
				ss := g.subSectorAtFixed(worldToFixed(x), worldToFixed(y))
				if ss < 0 || ss >= len(out) {
					continue
				}
				if g.sectorForSubSector(ss) != sec {
					continue
				}
				out[ss] = true
			}
		}
	}
	return out
}

func pointInAnySectorTri(p worldPt, tris []worldTri) bool {
	const eps = 1e-6
	for _, t := range tris {
		area := orient2D(t.a, t.b, t.c)
		if math.Abs(area) <= eps {
			continue
		}
		o1 := orient2D(t.a, t.b, p)
		o2 := orient2D(t.b, t.c, p)
		o3 := orient2D(t.c, t.a, p)
		hasNeg := o1 < -eps || o2 < -eps || o3 < -eps
		hasPos := o1 > eps || o2 > eps || o3 > eps
		if !(hasNeg && hasPos) {
			return true
		}
	}
	return false
}

func pointInTriOrEdge(p, a, b, c worldPt, eps float64) bool {
	o1 := orient2D(a, b, p)
	o2 := orient2D(b, c, p)
	o3 := orient2D(c, a, p)
	hasNeg := o1 < -eps || o2 < -eps || o3 < -eps
	hasPos := o1 > eps || o2 > eps || o3 > eps
	return !(hasNeg && hasPos)
}

func (g *game) pointCoveredBySectorFill(sec int, p worldPt) bool {
	if sec < 0 || sec >= len(g.sectorPlaneTris) {
		return false
	}
	if pointInAnySectorTri(p, g.sectorPlaneTris[sec]) {
		return true
	}
	const eps = 1e-6
	for _, hp := range g.holeFillPolys {
		if hp.sector != sec || len(hp.verts) < 3 || len(hp.tris) == 0 {
			continue
		}
		if p.x < hp.bbox.minX-eps || p.x > hp.bbox.maxX+eps || p.y < hp.bbox.minY-eps || p.y > hp.bbox.maxY+eps {
			continue
		}
		for _, tri := range hp.tris {
			i0, i1, i2 := tri[0], tri[1], tri[2]
			if i0 < 0 || i1 < 0 || i2 < 0 || i0 >= len(hp.verts) || i1 >= len(hp.verts) || i2 >= len(hp.verts) {
				continue
			}
			a := hp.verts[i0]
			b := hp.verts[i1]
			c := hp.verts[i2]
			if pointInTriOrEdge(p, a, b, c, eps) {
				return true
			}
		}
	}
	return false
}

func worldToFixed(v float64) int64 {
	return int64(math.Round(v * float64(1<<fracBits)))
}

func (g *game) recoverMissingSubsectorPolyFromBSP(ss, sec int) bool {
	if g == nil || g.m == nil || ss < 0 || ss >= len(g.m.SubSectors) {
		return false
	}
	poly, ok := g.subSectorNodeLeafPoly(ss)
	if !ok || len(poly) < 3 {
		return false
	}
	if sec >= 0 && sec < len(g.sectorBBox) {
		sb := g.sectorBBox[sec]
		if isFinite(sb.minX) && isFinite(sb.minY) && isFinite(sb.maxX) && isFinite(sb.maxY) {
			if clipped := clipWorldPolyByBBox(poly, expandWorldBBox(sb, 2)); len(clipped) >= 3 {
				poly = clipped
			} else {
				return false
			}
		}
	}
	if ss >= 0 && ss < len(g.subSectorBBox) {
		lb := g.subSectorBBox[ss]
		if isFinite(lb.minX) && isFinite(lb.minY) && isFinite(lb.maxX) && isFinite(lb.maxY) {
			if clipped := clipWorldPolyByBBox(poly, expandWorldBBox(lb, 768)); len(clipped) >= 3 {
				poly = clipped
			} else {
				return false
			}
		}
	}
	if len(poly) < 3 || math.Abs(polygonArea2(poly)) < 1e-6 || !polygonSimple(poly) {
		return false
	}
	if !g.polyLeafConsensusAccept(ss, sec, poly) {
		return false
	}
	if polygonArea2(poly) < 0 {
		for i, j := 0, len(poly)-1; i < j; i, j = i+1, j-1 {
			poly[i], poly[j] = poly[j], poly[i]
		}
	}
	if ss >= len(g.subSectorPoly) || ss >= len(g.subSectorPolySrc) {
		return false
	}
	g.subSectorPoly[ss] = poly
	g.subSectorPolySrc[ss] = subPolySrcNodes
	return true
}

func (g *game) polyLeafConsensusAccept(ss, sec int, poly []worldPt) bool {
	if len(poly) < 3 {
		return false
	}
	if !g.patchInsideSectorRings(sec, poly) {
		return false
	}
	c, ok := worldPolygonCentroid(poly)
	if !ok {
		return false
	}
	ssCenter := g.subSectorAtFixed(worldToFixed(c.x), worldToFixed(c.y))
	if ssCenter == ss {
		return true
	}

	// Last-fix heuristic: for tiny wedge triangles, allow recovery when local BSP
	// sampling agrees this polygon belongs to the same local sector and mostly to
	// the intended subsector.
	samples := make([]worldPt, 0, 1+len(poly)*2)
	samples = append(samples, c)
	for i := 0; i < len(poly); i++ {
		v := poly[i]
		m := worldPt{x: (v.x + c.x) * 0.5, y: (v.y + c.y) * 0.5}
		samples = append(samples, m)
		n := poly[(i+1)%len(poly)]
		e := worldPt{x: (v.x + n.x) * 0.5, y: (v.y + n.y) * 0.5}
		samples = append(samples, e)
	}

	exact := 0
	sameSector := 0
	for _, p := range samples {
		sst := g.subSectorAtFixed(worldToFixed(p.x), worldToFixed(p.y))
		if sst == ss {
			exact++
		}
		if g.sectorForSubSector(sst) == sec {
			sameSector++
		}
	}

	if sameSector != len(samples) {
		return false
	}
	if exact >= (len(samples)+1)/2 {
		return true
	}
	// Triangles are common orphan shape; permit weaker exact match if all
	// samples still stay within the expected sector.
	if len(poly) == 3 && exact >= 1 {
		return true
	}
	return false
}

func (g *game) patchInsideSectorRings(sec int, poly []worldPt) bool {
	if g == nil || g.m == nil || sec < 0 || sec >= len(g.m.Sectors) || len(poly) < 3 {
		return false
	}
	sets := g.buildSectorLoopSets()
	if sec >= len(sets) || len(sets[sec].rings) == 0 {
		return false
	}
	rings := sets[sec].rings
	const eps = 1e-6

	insideOrEdge := func(p worldPt) bool {
		if pointInRingsEvenOdd(p.x, p.y, rings) {
			return true
		}
		return pointOnAnyRingEdge(p, rings, eps)
	}

	c, ok := worldPolygonCentroid(poly)
	if !ok || !insideOrEdge(c) {
		return false
	}
	for _, v := range poly {
		if !insideOrEdge(v) {
			return false
		}
	}
	return true
}

func (g *game) subSectorNodeLeafPoly(targetSS int) ([]worldPt, bool) {
	if g == nil || g.m == nil || len(g.m.Nodes) == 0 || targetSS < 0 || targetSS >= len(g.m.SubSectors) {
		return nil, false
	}
	w := math.Max(g.bounds.maxX-g.bounds.minX, 1)
	h := math.Max(g.bounds.maxY-g.bounds.minY, 1)
	pad := math.Max(w, h)*2 + 1024
	root := []worldPt{
		{x: g.bounds.minX - pad, y: g.bounds.minY - pad},
		{x: g.bounds.maxX + pad, y: g.bounds.minY - pad},
		{x: g.bounds.maxX + pad, y: g.bounds.maxY + pad},
		{x: g.bounds.minX - pad, y: g.bounds.maxY + pad},
	}

	var walk func(child uint16, poly []worldPt) ([]worldPt, bool)
	walk = func(child uint16, poly []worldPt) ([]worldPt, bool) {
		if len(poly) < 3 {
			return nil, false
		}
		if child&0x8000 != 0 {
			ss := int(child & 0x7fff)
			if ss != targetSS {
				return nil, false
			}
			if math.Abs(polygonArea2(poly)) < 1e-6 {
				return nil, false
			}
			cp := make([]worldPt, len(poly))
			copy(cp, poly)
			return cp, true
		}
		ni := int(child)
		if ni < 0 || ni >= len(g.m.Nodes) {
			return nil, false
		}
		n := g.m.Nodes[ni]
		a := worldPt{x: float64(n.X), y: float64(n.Y)}
		b := worldPt{x: float64(n.X) + float64(n.DX), y: float64(n.Y) + float64(n.DY)}

		if p0 := clipWorldPolyByDivline(poly, a, b, 0); len(p0) >= 3 {
			if out, ok := walk(n.ChildID[0], p0); ok {
				return out, true
			}
		}
		if p1 := clipWorldPolyByDivline(poly, a, b, 1); len(p1) >= 3 {
			if out, ok := walk(n.ChildID[1], p1); ok {
				return out, true
			}
		}
		return nil, false
	}

	return walk(uint16(len(g.m.Nodes)-1), root)
}

func (g *game) constrainAmbiguousNodePolysToSectorBounds() {
	if g.m == nil || len(g.m.SubSectors) == 0 || len(g.sectorBBox) != len(g.m.Sectors) {
		return
	}
	const shortBBoxPad = 8.0
	const normalBBoxPad = 2.0
	const shortMinOverlapRatio = 0.15
	const normalMinOverlapRatio = 0.02
	const shortLocalPad = 640.0
	for ss, sub := range g.m.SubSectors {
		if ss >= len(g.subSectorPoly) || ss >= len(g.subSectorPolySrc) {
			continue
		}
		if g.subSectorPolySrc[ss] != subPolySrcNodes {
			continue
		}
		poly := g.subSectorPoly[ss]
		if len(poly) < 3 {
			continue
		}
		sec := -1
		if ss < len(g.subSectorSec) {
			sec = g.subSectorSec[ss]
		}
		if sec < 0 || sec >= len(g.sectorBBox) {
			continue
		}
		sb := g.sectorBBox[sec]
		if !isFinite(sb.minX) || !isFinite(sb.minY) || !isFinite(sb.maxX) || !isFinite(sb.maxY) {
			continue
		}
		pad := normalBBoxPad
		minOverlapRatio := normalMinOverlapRatio
		if sub.SegCount < 3 {
			pad = shortBBoxPad
			minOverlapRatio = shortMinOverlapRatio
		}
		sb = expandWorldBBox(sb, pad)
		pb := worldPolyBBox(poly)
		if ib, ok := worldBBoxIntersection(pb, sb); !ok || worldBBoxArea(pb) <= 0 || worldBBoxArea(ib)/worldBBoxArea(pb) < minOverlapRatio {
			g.subSectorPoly[ss] = nil
			g.subSectorPolySrc[ss] = subPolySrcNone
			continue
		}
		if clipped := clipWorldPolyByBBox(poly, sb); len(clipped) >= 3 {
			poly = clipped
			if sub.SegCount < 3 && ss < len(g.subSectorBBox) {
				if bb := g.subSectorBBox[ss]; isFinite(bb.minX) && isFinite(bb.minY) && isFinite(bb.maxX) && isFinite(bb.maxY) {
					local := expandWorldBBox(bb, shortLocalPad)
					if localClip := clipWorldPolyByBBox(poly, local); len(localClip) >= 3 {
						poly = localClip
					} else {
						g.subSectorPoly[ss] = nil
						g.subSectorPolySrc[ss] = subPolySrcNone
						continue
					}
				}
			}
			g.subSectorPoly[ss] = poly
			continue
		}
		g.subSectorPoly[ss] = nil
		g.subSectorPolySrc[ss] = subPolySrcNone
	}
}

type screenPt struct {
	x float64
	y float64
}

func (g *game) flatImage(name string) (*ebiten.Image, bool) {
	if g.flatImgCache == nil {
		g.flatImgCache = make(map[string]*ebiten.Image)
	}
	key, _ := g.resolveAnimatedFlatSample(name)
	return g.flatImageResolvedKey(key)
}

func (g *game) flatImageResolvedKey(key string) (*ebiten.Image, bool) {
	if key == "" {
		return nil, false
	}
	if g.flatImgCache == nil {
		g.flatImgCache = make(map[string]*ebiten.Image)
	}
	if img, ok := g.flatImgCache[key]; ok {
		return img, true
	}
	rgba, ok := g.flatRGBAResolvedKey(key)
	if !ok || len(rgba) != 64*64*4 {
		return nil, false
	}
	g.debugImageAlloc("flat:"+key, 64, 64)
	img := newDebugImage("flat:"+key, 64, 64)
	g.writePixelsTimed(img, rgba)
	g.flatImgCache[key] = img
	return img, true
}

func (g *game) flatRGBA(name string) ([]byte, bool) {
	key, _ := g.resolveAnimatedFlatSample(name)
	return g.flatRGBAResolvedKey(key)
}

func (g *game) flatTextureBlend(name string) (flatTextureBlendSample, bool) {
	sample := g.textureBlendSample(name, g.flatTextureAnimRefs)
	fromRGBA, ok := g.flatRGBAResolvedKey(sample.fromKey)
	if !ok || len(fromRGBA) != 64*64*4 {
		return flatTextureBlendSample{}, false
	}
	out := flatTextureBlendSample{fromRGBA: fromRGBA}
	if fromIndexed, ok := g.flatIndexedResolvedKey(sample.fromKey); ok && len(fromIndexed) == 64*64 {
		out.fromIndexed = fromIndexed
	}
	if sample.alpha == 0 || sample.toKey == "" {
		return out, true
	}
	toRGBA, ok := g.flatRGBAResolvedKey(sample.toKey)
	if !ok || len(toRGBA) != len(fromRGBA) {
		return out, true
	}
	out.toRGBA = toRGBA
	if toIndexed, ok := g.flatIndexedResolvedKey(sample.toKey); ok && len(toIndexed) == 64*64 {
		out.toIndexed = toIndexed
	} else {
		out.toRGBA = nil
	}
	out.alpha = sample.alpha
	return out, true
}

func sampleFlatBlendPacked(sample flatTextureBlendSample, u, v int) uint32 {
	ti := ((v & 63) * 64 * 4) + ((u & 63) * 4)
	if ti < 0 || ti+3 >= len(sample.fromRGBA) {
		return pixelOpaqueA
	}
	p0 := packRGBA(sample.fromRGBA[ti+0], sample.fromRGBA[ti+1], sample.fromRGBA[ti+2])
	if sample.alpha == 0 || len(sample.toRGBA) != len(sample.fromRGBA) {
		return p0
	}
	p1 := packRGBA(sample.toRGBA[ti+0], sample.toRGBA[ti+1], sample.toRGBA[ti+2])
	return blendPackedRGBA(p0, p1, sample.alpha)
}

func (g *game) flatRGBAResolvedKey(key string) ([]byte, bool) {
	rgba, ok := g.opts.FlatBank[key]
	if !ok || len(rgba) != 64*64*4 {
		return nil, false
	}
	return rgba, true
}

func (g *game) flatIndexedResolvedKey(key string) ([]byte, bool) {
	indexed, ok := g.opts.FlatBankIndexed[key]
	if !ok || len(indexed) != 64*64 {
		return nil, false
	}
	return indexed, true
}

const textureAnimTics = 8

type textureAnimRef struct {
	frames []string
	index  int
}

var defaultWallTextureAnimRefs = buildTextureAnimRefs([][]string{
	{"BLODGR1", "BLODGR2", "BLODGR3", "BLODGR4"},
	{"SLADRIP1", "SLADRIP2", "SLADRIP3"},
	{"BLODRIP1", "BLODRIP2", "BLODRIP3", "BLODRIP4"},
	{"FIREWALA", "FIREWALB", "FIREWALC", "FIREWALD", "FIREWALE", "FIREWALF", "FIREWALG", "FIREWALH", "FIREWALI", "FIREWALJ", "FIREWALK", "FIREWALL"},
	{"GSTFONT1", "GSTFONT2", "GSTFONT3"},
	{"FIRELAV3", "FIRELAVA"},
	{"FIREMAG1", "FIREMAG2", "FIREMAG3"},
	{"FIREBLU1", "FIREBLU2"},
	{"ROCKRED1", "ROCKRED2", "ROCKRED3"},
	{"BFALL1", "BFALL2", "BFALL3", "BFALL4"},
	{"SFALL1", "SFALL2", "SFALL3", "SFALL4"},
	{"WFALL1", "WFALL2", "WFALL3", "WFALL4"},
	{"DBRAIN1", "DBRAIN2", "DBRAIN3", "DBRAIN4"},
})

var defaultFlatTextureAnimRefs = buildTextureAnimRefs([][]string{
	{"NUKAGE1", "NUKAGE2", "NUKAGE3"},
	{"FWATER1", "FWATER2", "FWATER3", "FWATER4"},
	{"SWATER1", "SWATER2", "SWATER3", "SWATER4"},
	{"LAVA1", "LAVA2", "LAVA3", "LAVA4"},
	{"BLOOD1", "BLOOD2", "BLOOD3"},
	{"RROCK05", "RROCK06", "RROCK07", "RROCK08"},
	{"SLIME01", "SLIME02", "SLIME03", "SLIME04"},
	{"SLIME05", "SLIME06", "SLIME07", "SLIME08"},
	{"SLIME09", "SLIME10", "SLIME11", "SLIME12"},
})

func buildTextureAnimRefs(seqs [][]string) map[string]textureAnimRef {
	refs := make(map[string]textureAnimRef, 64)
	for _, seq := range seqs {
		frames := make([]string, 0, len(seq))
		for _, raw := range seq {
			name := normalizeFlatName(raw)
			if name == "" {
				continue
			}
			frames = append(frames, name)
		}
		if len(frames) < 2 {
			continue
		}
		for i, frame := range frames {
			refs[frame] = textureAnimRef{
				frames: frames,
				index:  0,
			}
			_ = i
		}
	}
	return refs
}

func buildTextureAnimRefsFromSequences(seqs map[string][]string) map[string]textureAnimRef {
	if len(seqs) == 0 {
		return nil
	}
	refs := make(map[string]textureAnimRef, len(seqs))
	for key, frames := range seqs {
		normKey := normalizeFlatName(key)
		if normKey == "" || len(frames) < 2 {
			continue
		}
		normFrames := make([]string, 0, len(frames))
		for _, frame := range frames {
			frame = normalizeFlatName(frame)
			if frame == "" {
				continue
			}
			normFrames = append(normFrames, frame)
		}
		if len(normFrames) < 2 {
			continue
		}
		refs[normKey] = textureAnimRef{frames: normFrames}
	}
	if len(refs) == 0 {
		return nil
	}
	return refs
}

func (g *game) resolveAnimatedWallName(name string) string {
	key, _ := g.resolveAnimatedWallSample(name)
	return key
}

func (g *game) resolveAnimatedFlatName(name string) string {
	key, _ := g.resolveAnimatedFlatSample(name)
	return key
}

func (g *game) resolveAnimatedWallSample(name string) (string, bool) {
	sample := g.textureBlendSample(name, g.wallTextureAnimRefs)
	return sample.fromKey, sample.alpha != 0 && sample.toKey != ""
}

func (g *game) resolveAnimatedFlatSample(name string) (string, bool) {
	sample := g.textureBlendSample(name, g.flatTextureAnimRefs)
	return sample.fromKey, sample.alpha != 0 && sample.toKey != ""
}

func resolveAnimatedTextureName(name string, worldTic int, refs map[string]textureAnimRef) string {
	key := normalizeFlatName(name)
	if key == "" {
		return ""
	}
	ref, ok := refs[key]
	if !ok || len(ref.frames) < 2 {
		return key
	}
	ticks := worldTic / textureAnimTics
	idx := (ref.index + ticks) % len(ref.frames)
	if idx < 0 {
		idx += len(ref.frames)
	}
	return ref.frames[idx]
}

func (g *game) resolveAnimatedTextureSample(name string, worldTic int, refs map[string]textureAnimRef) (string, bool) {
	key := normalizeFlatName(name)
	if key == "" {
		return "", false
	}
	ref, ok := refs[key]
	if !ok || len(ref.frames) < 2 {
		return key, false
	}
	ticks := worldTic / textureAnimTics
	idx := (ref.index + ticks) % len(ref.frames)
	if idx < 0 {
		idx += len(ref.frames)
	}
	return ref.frames[idx], false
}

func (g *game) textureBlendSample(name string, refs map[string]textureAnimRef) textureBlendSample {
	key := normalizeFlatName(name)
	if key == "" {
		return textureBlendSample{}
	}
	ref, ok := refs[key]
	if !ok || len(ref.frames) < 2 {
		return textureBlendSample{fromKey: key}
	}
	pos := float64(g.worldTic)
	if g != nil && g.opts.SourcePortMode {
		pos += g.renderAlpha
	}
	if pos < 0 {
		pos = 0
	}
	frameSpan := float64(textureAnimTics)
	if frameSpan < 1 {
		frameSpan = 1
	}
	frameTick := int(math.Floor(pos / frameSpan))
	idx := (ref.index + frameTick) % len(ref.frames)
	if idx < 0 {
		idx += len(ref.frames)
	}
	out := textureBlendSample{fromKey: ref.frames[idx]}
	if g == nil || !g.opts.SourcePortMode {
		return out
	}
	if g.opts.TextureAnimCrossfadeFrames <= 0 {
		return out
	}
	framePhase := math.Mod(pos, frameSpan)
	if framePhase < 0 {
		framePhase += frameSpan
	}
	alpha := framePhase / frameSpan
	if alpha <= 0 {
		return out
	}
	alpha = applyBlendShutter(alpha, wallFloorAnimShutterAngle)
	if alpha <= 0 {
		return out
	}
	nextIdx := (idx + 1) % len(ref.frames)
	out.toKey = ref.frames[nextIdx]
	out.alpha = uint8(math.Round(alpha * 255))
	return out
}

func (g *game) textureAnimTick() int {
	return g.worldTic / textureAnimTics
}

func normalizeFlatName(name string) string {
	if len(name) == 0 {
		return ""
	}
	n := 0
	canonical := true
	for ; n < len(name) && n < 8; n++ {
		c := name[n]
		if c == 0 {
			break
		}
		if c >= 'a' && c <= 'z' {
			canonical = false
		}
	}
	if canonical {
		return name[:n]
	}
	var out [8]byte
	n = 0
	for i := 0; i < len(name) && n < len(out); i++ {
		c := name[i]
		if c == 0 {
			break
		}
		if c >= 'a' && c <= 'z' {
			c -= 'a' - 'A'
		}
		out[n] = c
		n++
	}
	return string(out[:n])
}

func isSkyFlatName(name string) bool {
	n := normalizeFlatName(name)
	if n == "" {
		return false
	}
	return strings.Contains(n, "SKY")
}

func (g *game) capturePrevState() {
	g.State.CapturePrev()
	g.prevPX = g.p.x
	g.prevPY = g.p.y
	g.prevPrevAngle = g.prevAngle
	g.prevAngle = g.p.angle
	g.capturePrevWeaponRenderState()
	g.capturePrevSectorLightState()
	g.capturePrevThingRenderState()
	g.capturePrevProjectileRenderState()
}

func (g *game) syncRenderState() {
	g.capturePrevState()
	g.State.SyncRender()
	g.renderPX = float64(g.p.x) / fracUnit
	g.renderPY = float64(g.p.y) / fracUnit
	g.prevPrevAngle = g.p.angle
	g.renderAngle = g.p.angle
	g.renderAlpha = 1
	g.debugAimSS = debugFixedSubsector
	g.markSimUpdate(time.Now())
}

func (g *game) capturePrevWeaponRenderState() {
	if g == nil {
		return
	}
	g.prevWeaponState = g.weaponState
	g.prevWeaponFlashState = g.weaponFlashState
	g.prevWeaponPSpriteY = g.weaponPSpriteY
}

func (g *game) capturePrevSectorLightState() {
	if g == nil {
		return
	}
	for sec := range g.sectorPlaneCache {
		g.sectorPlaneCache[sec].prevLight = g.sectorPlaneCache[sec].light
		g.sectorPlaneCache[sec].prevLightMul = g.sectorPlaneCache[sec].lightMul
	}
}

func (g *game) capturePrevThingRenderState() {
	if g == nil {
		return
	}
	if len(g.prevThingX) != len(g.thingX) {
		g.prevThingX = make([]int64, len(g.thingX))
	}
	copy(g.prevThingX, g.thingX)
	if len(g.prevThingY) != len(g.thingY) {
		g.prevThingY = make([]int64, len(g.thingY))
	}
	copy(g.prevThingY, g.thingY)
	if len(g.prevThingZ) != len(g.thingZState) {
		g.prevThingZ = make([]int64, len(g.thingZState))
	}
	copy(g.prevThingZ, g.thingZState)
}

func (g *game) capturePrevProjectileRenderState() {
	if g == nil {
		return
	}
	for i := range g.projectiles {
		if g.projectiles[i].spawnPrev {
			continue
		}
		g.projectiles[i].prevX = g.projectiles[i].x
		g.projectiles[i].prevY = g.projectiles[i].y
		g.projectiles[i].prevZ = g.projectiles[i].z
	}
}

func (g *game) prepareRenderState() {
	g.prepareRenderStateAt(time.Now())
}

func (g *game) prepareRenderStateAt(now time.Time) {
	alpha := g.interpAlphaAt(now)
	if !g.opts.SourcePortMode {
		alpha = 1
	}
	if g.simTickScale > 1.0 {
		// Multiple sim ticks per frame already advance world state aggressively.
		// Interpolating from prev can make render state lag behind simulation.
		alpha = 1
	}
	g.State.PrepareRender(alpha)
	g.renderPX = lerp(float64(g.prevPX)/fracUnit, float64(g.p.x)/fracUnit, alpha)
	g.renderPY = lerp(float64(g.prevPY)/fracUnit, float64(g.p.y)/fracUnit, alpha)
	g.renderAngle = g.renderCameraAngle(alpha)
	g.renderAlpha = alpha
	g.beginSourcePortSpectreFuzzFrame(alpha)
	g.debugAimSS = debugFixedSubsector
}

func (g *game) renderCameraAngle(alpha float64) uint32 {
	if g == nil {
		return 0
	}
	angle := lerpAngle(g.prevAngle, g.p.angle, alpha)
	if g.opts.SmoothCameraYaw {
		angle = interpolateCameraAngle(g.prevPrevAngle, g.prevAngle, g.p.angle, alpha)
	}
	return angle
}

func (g *game) markSimUpdate(now time.Time) {
	if g == nil {
		return
	}
	g.lastUpdate = now
}

func (g *game) expectedSimStepSeconds() float64 {
	ticRate := float64(doomTicsPerSecond)
	if g != nil && g.simTickScale > 0 {
		ticRate *= g.simTickScale
	}
	if ticRate < 1e-6 {
		ticRate = doomTicsPerSecond
	}
	return 1.0 / ticRate
}

func (g *game) interpAlpha() float64 {
	return g.interpAlphaAt(time.Now())
}

func (g *game) interpAlphaAt(now time.Time) float64 {
	if g.lastUpdate.IsZero() {
		return 1
	}
	dt := now.Sub(g.lastUpdate).Seconds()
	step := g.expectedSimStepSeconds()
	a := dt / step
	if a < 0 {
		return 0
	}
	if a > 1 {
		return 1
	}
	return a
}

func lerp(a, b, t float64) float64 {
	return a + (b-a)*t
}

func applyBlendShutter(alpha, shutterAngle float64) float64 {
	if alpha <= 0 {
		return 0
	}
	if alpha >= 1 {
		return 1
	}
	if shutterAngle <= 0 {
		return 1
	}
	window := shutterAngle / 360.0
	if window >= 1 {
		return alpha
	}
	start := (1 - window) * 0.5
	end := 1 - start
	if alpha <= start {
		return 0
	}
	if alpha >= end {
		return 1
	}
	return (alpha - start) / window
}

func lerpFixed(a, b int64, t float64) int64 {
	return int64(math.Round(lerp(float64(a), float64(b), t)))
}

func (g *game) projectileRenderPosFixed(p projectile, alpha float64) (int64, int64, int64) {
	if g == nil || alpha >= 1 {
		return p.x, p.y, p.z
	}
	return lerpFixed(p.prevX, p.x, alpha), lerpFixed(p.prevY, p.y, alpha), lerpFixed(p.prevZ, p.z, alpha)
}

func lerpAngle(a, b uint32, t float64) uint32 {
	d := int64(int32(b - a))
	v := float64(int64(a)) + float64(d)*t
	return uint32(int64(v))
}

func shortestAngleDelta(a, b uint32) float64 {
	return float64(int64(int32(b - a)))
}

func cubicHermite(y0, y1, m0, m1, t float64) float64 {
	t2 := t * t
	t3 := t2 * t
	h00 := 2*t3 - 3*t2 + 1
	h10 := t3 - 2*t2 + t
	h01 := -2*t3 + 3*t2
	h11 := t3 - t2
	return h00*y0 + h10*m0 + h01*y1 + h11*m1
}

func interpolateCameraAngle(prevPrev, prev, curr uint32, t float64) uint32 {
	if t <= 0 {
		return prev
	}
	if t >= 1 {
		return curr
	}

	delta := shortestAngleDelta(prev, curr)
	if delta == 0 {
		return prev
	}

	prevDelta := shortestAngleDelta(prevPrev, prev)
	// Use a clamped Hermite curve so constant-speed turns stay linear, while
	// changes in mouse-turn rate blend without the hard tick boundary, even
	// through small reversal flicks.
	limit := 5 * math.Abs(delta)
	if math.Abs(prevDelta) > limit {
		prevDelta = math.Copysign(limit, prevDelta)
	}

	offset := cubicHermite(0, delta, prevDelta, delta, t)
	if delta > 0 {
		offset = math.Max(0, math.Min(delta, offset))
	} else {
		offset = math.Max(delta, math.Min(0, offset))
	}

	return uint32(int64(float64(int64(prev)) + offset))
}

func (g *game) linedefDecision(ld mapdata.Linedef) linepolicy.Decision {
	front, back := g.lineSectors(ld)
	st := linepolicy.StateForAutomap(g.automapRevealAll(), g.parity.iddt)
	return linepolicy.ParityDecision(ld, front, back, st)
}

func (g *game) lineSectors(ld mapdata.Linedef) (*mapdata.Sector, *mapdata.Sector) {
	if ld.SideNum[0] < 0 || int(ld.SideNum[0]) >= len(g.m.Sidedefs) {
		return nil, nil
	}
	s0 := g.m.Sidedefs[int(ld.SideNum[0])].Sector
	if int(s0) >= len(g.m.Sectors) {
		return nil, nil
	}
	front := &g.m.Sectors[s0]
	if ld.SideNum[1] < 0 || int(ld.SideNum[1]) >= len(g.m.Sidedefs) {
		return front, nil
	}
	s1 := g.m.Sidedefs[int(ld.SideNum[1])].Sector
	if int(s1) >= len(g.m.Sectors) {
		return front, nil
	}
	return front, &g.m.Sectors[s1]
}

func (g *game) segWorldEndpoints(segIdx int) (x1, y1, x2, y2 float64, ok bool) {
	if segIdx < 0 || segIdx >= len(g.m.Segs) {
		return 0, 0, 0, 0, false
	}
	sg := g.m.Segs[segIdx]
	if int(sg.StartVertex) >= len(g.m.Vertexes) || int(sg.EndVertex) >= len(g.m.Vertexes) {
		return 0, 0, 0, 0, false
	}
	v1 := g.m.Vertexes[sg.StartVertex]
	v2 := g.m.Vertexes[sg.EndVertex]
	return float64(v1.X), float64(v1.Y), float64(v2.X), float64(v2.Y), true
}

func (g *game) segSectors(segIdx int) (*mapdata.Sector, *mapdata.Sector) {
	if segIdx < 0 || segIdx >= len(g.m.Segs) {
		return nil, nil
	}
	sg := g.m.Segs[segIdx]
	li := int(sg.Linedef)
	if li < 0 || li >= len(g.m.Linedefs) {
		return nil, nil
	}
	ld := g.m.Linedefs[li]
	frontSide := int(sg.Direction)
	if frontSide < 0 || frontSide > 1 {
		frontSide = 0
	}
	backSide := frontSide ^ 1
	front := g.sectorFromSideNum(ld.SideNum[frontSide])
	back := g.sectorFromSideNum(ld.SideNum[backSide])
	// WAD seg direction can point at the missing side on one-sided linedefs.
	// Treat reversed one-sided segs as solid walls using the existing side.
	if front == nil && back != nil && (ld.SideNum[0] < 0 || ld.SideNum[1] < 0) {
		front = back
		back = nil
	}
	return front, back
}

func (g *game) segSectorIndices(segIdx int) (int, int) {
	if segIdx < 0 || segIdx >= len(g.m.Segs) {
		return -1, -1
	}
	sg := g.m.Segs[segIdx]
	li := int(sg.Linedef)
	if li < 0 || li >= len(g.m.Linedefs) {
		return -1, -1
	}
	ld := g.m.Linedefs[li]
	frontSide := int(sg.Direction)
	if frontSide < 0 || frontSide > 1 {
		frontSide = 0
	}
	backSide := frontSide ^ 1
	front := g.sectorIndexFromSideNum(ld.SideNum[frontSide])
	back := g.sectorIndexFromSideNum(ld.SideNum[backSide])
	// WAD seg direction can point at the missing side on one-sided linedefs.
	if front < 0 && back >= 0 && (ld.SideNum[0] < 0 || ld.SideNum[1] < 0) {
		front = back
		back = -1
	}
	return front, back
}

func (g *game) sectorFromSideNum(side int16) *mapdata.Sector {
	secIdx := g.sectorIndexFromSideNum(side)
	if secIdx < 0 || secIdx >= len(g.m.Sectors) {
		return nil
	}
	return &g.m.Sectors[secIdx]
}

func (g *game) subSectorSectorFromSeg(segIdx int) (int, bool) {
	if segIdx < 0 || segIdx >= len(g.m.Segs) {
		return 0, false
	}
	sg := g.m.Segs[segIdx]
	if int(sg.Linedef) < 0 || int(sg.Linedef) >= len(g.m.Linedefs) {
		return 0, false
	}
	ld := g.m.Linedefs[sg.Linedef]
	frontSide := int(sg.Direction)
	if frontSide < 0 || frontSide > 1 {
		frontSide = 0
	}
	backSide := frontSide ^ 1
	if sec := g.sectorIndexFromSideNum(ld.SideNum[frontSide]); sec >= 0 {
		return sec, true
	}
	back := g.sectorIndexFromSideNum(ld.SideNum[backSide])
	if back >= 0 && (ld.SideNum[0] < 0 || ld.SideNum[1] < 0) {
		return back, true
	}
	if back >= 0 {
		return back, true
	}
	return 0, false
}

func (g *game) sectorIndexFromSideNum(side int16) int {
	if side < 0 || int(side) >= len(g.m.Sidedefs) {
		return -1
	}
	sec := int(g.m.Sidedefs[int(side)].Sector)
	if sec < 0 || sec >= len(g.m.Sectors) {
		return -1
	}
	return sec
}

func (g *game) mapLineStateKey() mapview.LineCacheKey {
	view := g.State.Snapshot()
	cacheState := view.CacheState(g.viewport())
	return mapview.LineCacheKey{
		CamX:       cacheState.CamX,
		CamY:       cacheState.CamY,
		Zoom:       cacheState.Zoom,
		Angle:      cacheState.Angle,
		RotateView: cacheState.Rotate,
		ViewW:      cacheState.ViewWidth,
		ViewH:      cacheState.ViewHeight,
		Reveal:     int(g.parity.reveal),
		IDDT:       g.parity.iddt,
		MappedRev:  g.mapLines.Revision(),
	}
}

func (g *game) rebuildMapLineCache(key mapview.LineCacheKey) {
	out := mapview.BuildLineCache(g.mapLines.Reuse(), g.visibleLineIndices(), g.resolveMapLineDraw)
	g.mapLines.Reset(out, key)
}

func (g *game) resolveMapLineDraw(li int) (mapview.CachedLine, bool) {
	pi := g.physForLine[li]
	if pi < 0 || pi >= len(g.lines) {
		return mapview.CachedLine{}, false
	}
	ld := g.m.Linedefs[li]
	d := g.linedefDecision(ld)
	if !d.Visible {
		return mapview.CachedLine{}, false
	}
	pl := g.lines[pi]
	x1, y1 := g.worldToScreen(float64(pl.x1)/fracUnit, float64(pl.y1)/fracUnit)
	x2, y2 := g.worldToScreen(float64(pl.x2)/fracUnit, float64(pl.y2)/fracUnit)
	if x1 == x2 && y1 == y2 {
		return mapview.CachedLine{}, false
	}
	style := d.Style(mapLinePalette)
	return mapview.CachedLine{
		X1:  float32(x1),
		Y1:  float32(y1),
		X2:  float32(x2),
		Y2:  float32(y2),
		W:   float32(style.Width),
		Clr: style.Color,
	}, true
}

func (g *game) drawMapLines(screen *ebiten.Image) {
	key := g.mapLineStateKey()
	if g.mapLines.NeedsRebuild(key) {
		g.rebuildMapLineCache(key)
	}
	mapview.DrawCachedLines(screen, g.mapLines.Items(), g.mapVectorAntiAlias())
}

func (g *game) visibleLineIndices() []int {
	view := g.State.Snapshot()
	margin := 2.0 / view.ZoomLevel()
	minXf, maxXf, minYf, maxYf := view.VisibleBounds(g.viewport(), margin)
	minX := floatToFixed(minXf)
	maxX := floatToFixed(maxXf)
	minY := floatToFixed(minYf)
	maxY := floatToFixed(maxYf)

	// Robust automap visibility: trust line bboxes directly.
	// Some BLOCKMAP data can omit candidates and cause line pop/disappear at seams.
	return g.mapLineVisibility.Filter(g.mapVisibleLines, minX, minY, maxX, maxY)
}

func (g *game) sectorVisibleNow(sec int) bool {
	if len(g.m.Nodes) == 0 {
		return true
	}
	if sec < 0 || sec >= len(g.visibleSectorSeen) || g.visibleEpoch == 0 {
		return false
	}
	return g.visibleSectorSeen[sec] == g.visibleEpoch
}

func xInSolidSpans(x int, spans []solidSpan) bool {
	if len(spans) == 0 {
		return true
	}
	for _, sp := range spans {
		if x < sp.L {
			return false
		}
		if x <= sp.R {
			return true
		}
	}
	return false
}

func appendClippedSolidSpan(out []solidSpan, l, r, minX, maxX int) []solidSpan {
	if l < minX {
		l = minX
	}
	if r > maxX {
		r = maxX
	}
	if l > r {
		return out
	}
	return append(out, solidSpan{L: l, R: r})
}

func maskedClipColumnOccludesPointSorted(spans []scene.MaskedClipSpan, y int, depthQ uint16) bool {
	return scene.MaskedClipColumnOccludesPointSorted(spans, y, depthQ)
}

func (g *game) spriteRowVisibleSpansDepthQ(y, x0, x1 int, depthQ uint16, clipSpans, out []solidSpan) []solidSpan {
	out = out[:0]
	if x1 < x0 || g == nil || y < 0 || y >= g.viewH {
		return out
	}
	if !g.billboardClippingEnabled() {
		if len(clipSpans) == 0 {
			return append(out, solidSpan{L: x0, R: x1})
		}
		for _, sp := range clipSpans {
			out = appendClippedSolidSpan(out, sp.L, sp.R, x0, x1)
		}
		return out
	}

	appendVisible := func(l, r int) {
		if l > r {
			return
		}
		runStart := -1
		for x := l; x <= r; x++ {
			occluded := false
			if x >= 0 && x < len(g.wallDepthQCol) && depthQ > g.wallDepthQCol[x] {
				if x < len(g.wallDepthClosedCol) && g.wallDepthClosedCol[x] {
					occluded = true
				} else {
					top := g.viewH
					bottom := -1
					if x < len(g.wallDepthTopCol) {
						top = g.wallDepthTopCol[x]
					}
					if x < len(g.wallDepthBottomCol) {
						bottom = g.wallDepthBottomCol[x]
					}
					if top <= bottom && y >= top && y <= bottom {
						occluded = true
					}
				}
			}
			if !occluded {
				if x >= 0 && x < len(g.maskedClipCols) {
					if x < len(g.maskedClipFirstDepthQ) && g.maskedClipFirstDepthQ[x] != 0 && depthQ > g.maskedClipFirstDepthQ[x] {
						occluded = maskedClipColumnOccludesPointSorted(g.maskedClipCols[x], y, depthQ)
					}
				}
			}
			if occluded {
				if runStart >= 0 {
					out = append(out, solidSpan{L: runStart, R: x - 1})
					runStart = -1
				}
				continue
			}
			if runStart < 0 {
				runStart = x
			}
		}
		if runStart >= 0 {
			out = append(out, solidSpan{L: runStart, R: r})
		}
	}

	if len(clipSpans) == 0 {
		appendVisible(x0, x1)
		return out
	}
	for _, sp := range clipSpans {
		l := sp.L
		r := sp.R
		if l < x0 {
			l = x0
		}
		if r > x1 {
			r = x1
		}
		appendVisible(l, r)
	}
	return out
}

func (g *game) thingSectorCached(i int, th mapdata.Thing) int {
	if i >= 0 && i < len(g.thingSectorCache) {
		sec := g.thingSectorCache[i]
		if sec >= 0 && sec < len(g.m.Sectors) {
			return sec
		}
	}
	x, y := g.thingPosFixed(i, th)
	return g.sectorAt(x, y)
}

func (g *game) thingRenderPosFixed(i int, th mapdata.Thing, alpha float64) (int64, int64, int64) {
	x, y := g.thingPosFixed(i, th)
	z, _, _ := g.thingSupportState(i, th)
	if g == nil || alpha >= 1 || i < 0 {
		return x, y, z
	}
	if blendAlpha, ok := g.thingRenderBlendAlpha(i, th, alpha); ok {
		if i < len(g.thingRenderBlendFromX) {
			x = lerpFixed(g.thingRenderBlendFromX[i], x, blendAlpha)
		}
		if i < len(g.thingRenderBlendFromY) {
			y = lerpFixed(g.thingRenderBlendFromY[i], y, blendAlpha)
		}
		return x, y, z
	}
	if i < len(g.prevThingX) {
		x = lerpFixed(g.prevThingX[i], x, alpha)
	}
	if i < len(g.prevThingY) {
		y = lerpFixed(g.prevThingY[i], y, alpha)
	}
	if i < len(g.prevThingZ) {
		z = lerpFixed(g.prevThingZ[i], z, alpha)
	}
	return x, y, z
}

func (g *game) thingRenderBlendAlpha(i int, th mapdata.Thing, alpha float64) (float64, bool) {
	if g == nil || !g.opts.ZombiemanThinkerBlend || i < 0 {
		return 0, false
	}
	if i >= len(g.thingRenderBlendTics) || g.thingRenderBlendTics[i] <= 1 {
		return 0, false
	}
	if i >= len(g.thingStateTics) {
		return 0, false
	}
	total, ok := 0, false
	if i < len(g.thingDoomState) {
		total, ok = thingThinkerBlendProfileExact(th.Type, g.thingDoomState[i])
	}
	if !ok {
		total, ok = thingThinkerBlendProfileGeneric(g, i, th.Type)
	}
	if !ok {
		return 0, false
	}
	remaining := g.thingStateTics[i]
	if g.thingRenderBlendTics[i] != total || remaining < 1 || remaining > total {
		return 0, false
	}
	blendAlpha := (float64(total-remaining) + alpha) / float64(total)
	if blendAlpha < 0 {
		blendAlpha = 0
	}
	if blendAlpha > 1 {
		blendAlpha = 1
	}
	return blendAlpha, true
}

func thingThinkerBlendProfileExact(typ int16, state int) (int, bool) {
	switch typ {
	case 3004:
		if state >= 176 && state <= 183 {
			return 4, true
		}
	case 9:
		if state >= 210 && state <= 217 {
			return 3, true
		}
	case 65:
		if state >= 408 && state <= 415 {
			return 3, true
		}
	case 3005:
		if state == 503 {
			return 3, true
		}
	case 3006:
		if state == 587 || state == 588 {
			return 6, true
		}
	}
	return 0, false
}

func thingThinkerBlendProfileGeneric(g *game, i int, typ int16) (int, bool) {
	if g == nil || i < 0 || i >= len(g.thingState) || g.thingState[i] != monsterStateSee {
		return 0, false
	}
	switch typ {
	case 3001, 3002, 58:
		return g.monsterSeeStateTicsForPhase(i, typ), true
	default:
		return 0, false
	}
}

func (g *game) startThingRenderBlend(i int, fromX, fromY int64, totalTics int) {
	if g == nil || i < 0 || totalTics <= 1 {
		return
	}
	if i >= len(g.thingRenderBlendFromX) {
		g.thingRenderBlendFromX = append(g.thingRenderBlendFromX, make([]int64, i-len(g.thingRenderBlendFromX)+1)...)
	}
	if i >= len(g.thingRenderBlendFromY) {
		g.thingRenderBlendFromY = append(g.thingRenderBlendFromY, make([]int64, i-len(g.thingRenderBlendFromY)+1)...)
	}
	if i >= len(g.thingRenderBlendTics) {
		g.thingRenderBlendTics = append(g.thingRenderBlendTics, make([]int, i-len(g.thingRenderBlendTics)+1)...)
	}
	g.thingRenderBlendFromX[i] = fromX
	g.thingRenderBlendFromY[i] = fromY
	g.thingRenderBlendTics[i] = totalTics
}

func (g *game) thingPosFixed(i int, th mapdata.Thing) (int64, int64) {
	if i >= 0 && i < len(g.thingX) && i < len(g.thingY) {
		return g.thingX[i], g.thingY[i]
	}
	return int64(th.X) << fracBits, int64(th.Y) << fracBits
}

func (g *game) thingWorldAngle(i int, th mapdata.Thing) uint32 {
	if i >= 0 && i < len(g.thingAngleState) {
		return g.thingAngleState[i]
	}
	return thingDegToWorldAngle(th.Angle)
}

func (g *game) thingSupportState(i int, th mapdata.Thing) (z, floorZ, ceilZ int64) {
	if i >= 0 && i < len(g.thingFloorState) && i < len(g.thingCeilState) {
		valid := i < len(g.thingSupportValid) && g.thingSupportValid[i]
		floorZ = g.thingFloorState[i]
		ceilZ = g.thingCeilState[i]
		if i < len(g.thingZState) {
			z = g.thingZState[i]
		} else {
			z = floorZ
		}
		if valid {
			return z, floorZ, ceilZ
		}
	}
	x, y := g.thingPosFixed(i, th)
	sec := g.thingSectorCached(i, th)
	floorZ = g.thingFloorZ(x, y)
	z = floorZ
	if sec >= 0 && sec < len(g.sectorCeil) {
		ceilZ = g.sectorCeil[sec]
	}
	return z, floorZ, ceilZ
}

func (g *game) setThingSupportState(i int, z, floorZ, ceilZ int64) {
	if g == nil || i < 0 {
		return
	}
	if i >= len(g.thingZState) {
		g.thingZState = append(g.thingZState, make([]int64, i-len(g.thingZState)+1)...)
	}
	if i >= len(g.thingFloorState) {
		g.thingFloorState = append(g.thingFloorState, make([]int64, i-len(g.thingFloorState)+1)...)
	}
	if i >= len(g.thingCeilState) {
		g.thingCeilState = append(g.thingCeilState, make([]int64, i-len(g.thingCeilState)+1)...)
	}
	if i >= len(g.thingSupportValid) {
		g.thingSupportValid = append(g.thingSupportValid, make([]bool, i-len(g.thingSupportValid)+1)...)
	}
	g.thingZState[i] = z
	g.thingFloorState[i] = floorZ
	g.thingCeilState[i] = ceilZ
	g.thingSupportValid[i] = true
}

func (g *game) setThingPosFixed(i int, x, y int64) {
	if g == nil || i < 0 || g.m == nil || i >= len(g.m.Things) {
		return
	}
	if i >= len(g.thingX) {
		g.thingX = append(g.thingX, make([]int64, i-len(g.thingX)+1)...)
	}
	if i >= len(g.thingY) {
		g.thingY = append(g.thingY, make([]int64, i-len(g.thingY)+1)...)
	}
	g.thingX[i] = x
	g.thingY[i] = y
	g.m.Things[i].X = int16(x >> fracBits)
	g.m.Things[i].Y = int16(y >> fracBits)
	if i >= len(g.thingSectorCache) {
		g.thingSectorCache = append(g.thingSectorCache, make([]int, i-len(g.thingSectorCache)+1)...)
	}
	g.thingSectorCache[i] = g.sectorAt(x, y)
	if i >= len(g.thingBlockOrder) {
		g.thingBlockOrder = append(g.thingBlockOrder, make([]int64, i-len(g.thingBlockOrder)+1)...)
	}
	g.thingBlockOrder[i] = g.allocBlockmapOrder()
	g.updateThingBlockmapIndex(i)
}

func (g *game) snapThingRenderState(i int) {
	if g == nil || i < 0 {
		return
	}
	if i < len(g.thingX) {
		if i >= len(g.prevThingX) {
			g.prevThingX = append(g.prevThingX, make([]int64, i-len(g.prevThingX)+1)...)
		}
		g.prevThingX[i] = g.thingX[i]
	}
	if i < len(g.thingY) {
		if i >= len(g.prevThingY) {
			g.prevThingY = append(g.prevThingY, make([]int64, i-len(g.prevThingY)+1)...)
		}
		g.prevThingY[i] = g.thingY[i]
	}
	if i < len(g.thingZState) {
		if i >= len(g.prevThingZ) {
			g.prevThingZ = append(g.prevThingZ, make([]int64, i-len(g.prevThingZ)+1)...)
		}
		g.prevThingZ[i] = g.thingZState[i]
	}
}

func (g *game) setPlayerPosFixed(x, y int64) {
	if g == nil {
		return
	}
	g.p.x = x
	g.p.y = y
	g.playerBlockOrder = g.allocBlockmapOrder()
	g.refreshPlayerSubsectorCache(x, y)
}

func (g *game) thingBlockmapCellFor(x, y int64) int {
	if g == nil || g.bmapWidth <= 0 || g.bmapHeight <= 0 {
		return -1
	}
	bx := int((x - g.bmapOriginX) >> (fracBits + 7))
	by := int((y - g.bmapOriginY) >> (fracBits + 7))
	if bx < 0 || by < 0 || bx >= g.bmapWidth || by >= g.bmapHeight {
		return -1
	}
	return by*g.bmapWidth + bx
}

func (g *game) rebuildThingBlockmap() {
	if g == nil || g.bmapWidth <= 0 || g.bmapHeight <= 0 {
		return
	}
	if len(g.thingBlockCells) != g.bmapWidth*g.bmapHeight {
		g.thingBlockCells = make([][]int, g.bmapWidth*g.bmapHeight)
	}
	for i := range g.thingBlockCells {
		g.thingBlockCells[i] = g.thingBlockCells[i][:0]
	}
	if len(g.thingBlockCell) < len(g.m.Things) {
		g.thingBlockCell = append(g.thingBlockCell, make([]int, len(g.m.Things)-len(g.thingBlockCell))...)
	}
	if len(g.thingBlockOrder) < len(g.m.Things) {
		start := len(g.thingBlockOrder)
		g.thingBlockOrder = append(g.thingBlockOrder, make([]int64, len(g.m.Things)-start)...)
		for i := start; i < len(g.m.Things); i++ {
			g.thingBlockOrder[i] = g.allocBlockmapOrder()
		}
	}
	for i := range g.m.Things {
		g.thingBlockCell[i] = -1
		if g.thingBlockOrder[i] == 0 {
			g.thingBlockOrder[i] = g.allocBlockmapOrder()
		}
	}
	for i, th := range g.m.Things {
		if !thingTypeUsesBlockmap(th.Type) {
			continue
		}
		x, y := g.thingPosFixed(i, th)
		cell := g.thingBlockmapCellFor(x, y)
		g.thingBlockCell[i] = cell
		if cell < 0 {
			continue
		}
		g.thingBlockCells[cell] = append(g.thingBlockCells[cell], i)
	}
	for cell := range g.thingBlockCells {
		slices.SortStableFunc(g.thingBlockCells[cell], func(a, b int) int {
			ao := g.thingBlockOrder[a]
			bo := g.thingBlockOrder[b]
			switch {
			case ao > bo:
				return -1
			case ao < bo:
				return 1
			default:
				return 0
			}
		})
	}
}

func (g *game) removeThingFromBlockCell(cell, thingIdx int) bool {
	if g == nil || cell < 0 || cell >= len(g.thingBlockCells) {
		return false
	}
	items := g.thingBlockCells[cell]
	for pos, idx := range items {
		if idx != thingIdx {
			continue
		}
		copy(items[pos:], items[pos+1:])
		items = items[:len(items)-1]
		g.thingBlockCells[cell] = items
		return true
	}
	return false
}

func (g *game) insertThingIntoBlockCell(cell, thingIdx int) bool {
	if g == nil || cell < 0 || cell >= len(g.thingBlockCells) || thingIdx < 0 {
		return false
	}
	items := g.thingBlockCells[cell]
	order := g.thingBlockOrder[thingIdx]
	insertAt := len(items)
	for pos, idx := range items {
		if order > g.thingBlockOrder[idx] {
			insertAt = pos
			break
		}
	}
	items = append(items, 0)
	copy(items[insertAt+1:], items[insertAt:])
	items[insertAt] = thingIdx
	g.thingBlockCells[cell] = items
	return true
}

func thingTypeUsesBlockmap(typ int16) bool {
	switch typ {
	case teleportThingType:
		// doom-source marks MT_TELEPORTMAN MF_NOBLOCKMAP, so it never enters
		// blocklinks and is skipped by sector height clipping.
		return false
	default:
		return true
	}
}

func (g *game) setThingWorldAngle(i int, angle uint32) {
	if g == nil || i < 0 {
		return
	}
	if i >= len(g.thingAngleState) {
		g.thingAngleState = append(g.thingAngleState, make([]uint32, i-len(g.thingAngleState)+1)...)
	}
	g.thingAngleState[i] = angle
}

func (g *game) updateThingBlockmapIndex(i int) {
	if g == nil || i < 0 || g.m == nil || i >= len(g.m.Things) || g.bmapWidth <= 0 || g.bmapHeight <= 0 {
		return
	}
	if len(g.thingBlockCells) != g.bmapWidth*g.bmapHeight || len(g.thingBlockCell) <= i {
		g.rebuildThingBlockmap()
		return
	}
	oldCell := g.thingBlockCell[i]
	x, y := g.thingPosFixed(i, g.m.Things[i])
	newCell := g.thingBlockmapCellFor(x, y)
	if oldCell == newCell {
		if newCell < 0 {
			g.thingBlockCell[i] = -1
			return
		}
		if !g.removeThingFromBlockCell(newCell, i) || !g.insertThingIntoBlockCell(newCell, i) {
			g.rebuildThingBlockmap()
			return
		}
		g.thingBlockCell[i] = newCell
		return
	}
	if oldCell >= 0 && oldCell < len(g.thingBlockCells) && !g.removeThingFromBlockCell(oldCell, i) {
		g.rebuildThingBlockmap()
		return
	}
	g.thingBlockCell[i] = newCell
	if newCell >= 0 && !g.insertThingIntoBlockCell(newCell, i) {
		g.rebuildThingBlockmap()
		return
	}
}

func (g *game) subsectorFloorCeilAt(x, y int64) (int64, int64, bool) {
	if g == nil || g.m == nil {
		return 0, 0, false
	}
	sec := -1
	if len(g.m.SubSectors) > 0 {
		if ss := g.subSectorAtFixed(x, y); ss >= 0 {
			sec = g.sectorForSubSector(ss)
		}
	}
	if sec < 0 {
		sec = g.sectorAt(x, y)
	}
	if sec < 0 {
		return 0, 0, false
	}
	if sec >= 0 && sec < len(g.sectorFloor) && sec < len(g.sectorCeil) {
		return g.sectorFloor[sec], g.sectorCeil[sec], true
	}
	if sec >= 0 && sec < len(g.m.Sectors) {
		return int64(g.m.Sectors[sec].FloorHeight) << fracBits, int64(g.m.Sectors[sec].CeilingHeight) << fracBits, true
	}
	return 0, 0, false
}

func spriteSectorClipYBounds(viewH int, eyeZ, depth, focal float64, floorZFixed, ceilZFixed int64) (int, int, bool) {
	if viewH <= 0 || depth <= 0 || !isFinite(depth) || !isFinite(focal) || focal <= 0 {
		return 0, -1, false
	}
	floorZ := float64(floorZFixed) / fracUnit
	ceilZ := float64(ceilZFixed) / fracUnit
	halfH := float64(viewH) * 0.5
	top := int(math.Ceil(halfH - ((ceilZ-eyeZ)/depth)*focal))
	bottom := int(math.Floor(halfH - ((floorZ-eyeZ)/depth)*focal))
	if top < 0 {
		top = 0
	}
	if bottom >= viewH {
		bottom = viewH - 1
	}
	if top > bottom {
		return top, bottom, false
	}
	return top, bottom, true
}

func projectedScreenWidthToWorldRadiusFixed(screenW, depth, focal float64) int64 {
	if screenW <= 0 || depth <= 0 || focal <= 0 {
		return 0
	}
	r := (screenW * 0.5) * (depth / focal)
	if r < 1.0 {
		r = 1.0
	}
	return int64(r * fracUnit)
}

func monsterSpriteClipRadius(typ int16) int64 {
	r := monsterRadius(typ)
	if r <= 0 {
		return 20 * fracUnit
	}
	return r
}

func worldThingSpriteClipRadius(typ int16) int64 {
	r := thingTypeRadius(typ)
	if r <= 0 {
		return 20 * fracUnit
	}
	return r
}

func (g *game) spriteFootprintClipYBounds(x, y, radius int64, viewH int, eyeZ, depth, focal float64) (int, int, bool) {
	if !g.billboardClippingEnabled() {
		return 0, viewH - 1, true
	}
	centerSec := g.sectorAt(x, y)
	if radius < 0 {
		radius = -radius
	}
	samples := [9][2]int64{
		{0, 0},
		{radius, 0},
		{-radius, 0},
		{0, radius},
		{0, -radius},
		{radius, radius},
		{radius, -radius},
		{-radius, radius},
		{-radius, -radius},
	}
	top := 0
	bottom := viewH - 1
	have := false
	for _, off := range samples {
		// With zero radius, center sample is sufficient.
		if radius == 0 && (off[0] != 0 || off[1] != 0) {
			continue
		}
		sampleX := x + off[0]
		sampleY := y + off[1]
		if centerSec >= 0 && g.sectorAt(sampleX, sampleY) != centerSec {
			continue
		}
		floorZ, ceilZ, ok := g.subsectorFloorCeilAt(sampleX, sampleY)
		if !ok {
			continue
		}
		t, b, clipOK := spriteSectorClipYBounds(viewH, eyeZ, depth, focal, floorZ, ceilZ)
		if !clipOK {
			continue
		}
		if !have {
			top = t
			bottom = b
			have = true
			continue
		}
		if t > top {
			top = t
		}
		if b < bottom {
			bottom = b
		}
	}
	if !have {
		return 0, viewH - 1, true
	}
	if top > bottom {
		return top, bottom, false
	}
	return top, bottom, true
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func (g *game) menuPatch(name string) (*ebiten.Image, int, int, int, int, bool) {
	key := strings.ToUpper(strings.TrimSpace(name))
	p, ok := g.opts.MenuPatchBank[key]
	if !ok || p.Width <= 0 || p.Height <= 0 || len(p.RGBA) != p.Width*p.Height*4 {
		return nil, 0, 0, 0, 0, false
	}
	if g.menuPatchImg == nil {
		g.menuPatchImg = make(map[string]*ebiten.Image, 32)
	}
	if img, ok := g.menuPatchImg[key]; ok {
		return img, p.Width, p.Height, p.OffsetX, p.OffsetY, true
	}
	g.debugImageAlloc("menu-patch:"+key, p.Width, p.Height)
	img := newDebugImage("menu-patch:"+key, p.Width, p.Height)
	img.WritePixels(p.RGBA)
	g.menuPatchImg[key] = img
	return img, p.Width, p.Height, p.OffsetX, p.OffsetY, true
}

func (g *game) drawMenuPatch(screen *ebiten.Image, name string, x, y, sx, sy float64) bool {
	img, _, _, ox, oy, ok := g.menuPatch(name)
	if !ok {
		return false
	}
	op := &ebiten.DrawImageOptions{}
	op.Filter = ebiten.FilterNearest
	op.GeoM.Scale(sx, sy)
	op.GeoM.Translate(x-float64(ox)*sx, y-float64(oy)*sy)
	screen.DrawImage(img, op)
	return true
}

func (g *game) drawPauseOverlay(screen *ebiten.Image) {
	if g == nil || screen == nil || !g.pauseMenuActive || g.quitPromptActive {
		return
	}
	sw := max(screen.Bounds().Dx(), 1)
	sh := max(screen.Bounds().Dy(), 1)
	scale := float64(sw) / 320.0
	scaleY := float64(sh) / 200.0
	if scaleY < scale {
		scale = scaleY
	}
	if scale < 1 {
		scale = 1
	}
	ox := (float64(sw) - 320.0*scale) * 0.5
	oy := (float64(sh) - 200.0*scale) * 0.5
	drawPatch := func(name string, x, y float64) bool {
		return g.drawMenuPatch(screen, name, ox+x*scale, oy+y*scale, scale, scale)
	}
	drawText := func(text string, x, y, textScale float64) {
		g.drawHUTextAt(screen, text, ox+x*scale, oy+y*scale, scale*textScale, scale*textScale)
	}
	drawRect := func(x, y, w, h int, clr color.Color) {
		ebitenutil.DrawRect(screen, ox+float64(x)*scale, oy+float64(y)*scale, float64(w)*scale, float64(h)*scale, clr)
	}
	drawSkull := func(x, y int) {
		name := "M_SKULL1"
		if g.pauseMenuWhichSkull != 0 {
			name = "M_SKULL2"
		}
		_ = drawPatch(name, float64(x), float64(y))
	}

	switch g.pauseMenuMode {
	case pauseMenuModeOptions:
		const menuX = 36
		const menuY = 37
		const lineHeight = 16
		drawPatch("M_OPTTTL", menuX, 15)
		backLabel := "BACK: ESC"
		backX := 320 - 8 - int(math.Ceil(float64(g.huTextWidth(backLabel))*1.2))
		drawText(backLabel, float64(backX), 17, 1.2)
		drawText("MESSAGES", menuX, menuY+0*lineHeight+2, 1.2)
		drawText("STATUS BAR MODE", menuX, menuY+1*lineHeight+2, 1.2)
		msgLabel := "OFF"
		if g.hudMessagesEnabled {
			msgLabel = "ON"
		}
		drawText(msgLabel, menuX+215, menuY+0*lineHeight+2, 1.2)
		drawText(g.screenSizeLabel(), menuX+215, menuY+1*lineHeight+2, 1.2)
		drawText("HUD SIZE", menuX, menuY+2*lineHeight+2, 1.2)
		drawText(g.hudScaleLabel(), menuX+215, menuY+2*lineHeight+2, 1.2)
		drawText("FPS", menuX, menuY+3*lineHeight+2, 1.2)
		fpsLabel := "OFF"
		if !g.opts.NoFPS {
			fpsLabel = "ON"
		}
		drawText(fpsLabel, menuX+215, menuY+3*lineHeight+2, 1.2)
		drawText("MOUSE SENSITIVITY", menuX, menuY+4*lineHeight+2, 1.2)
		drawText(formatFloat2(g.opts.MouseLookSpeed), menuX+215, menuY+4*lineHeight+2, 1.2)
		drawText("SOUND OPTIONS", menuX, menuY+5*lineHeight+2, 1.2)
		drawText("OPEN", menuX+215, menuY+5*lineHeight+2, 1.2)
		drawText("KEY BINDINGS", menuX, menuY+6*lineHeight+2, 1.2)
		drawText("OPEN", menuX+215, menuY+6*lineHeight+2, 1.2)
		drawSkull(g.pauseOptionsSkullX(menuX), menuY+g.pauseMenuOptionsOn*lineHeight)
	case pauseMenuModeSound:
		const menuX = 80
		const menuY = 64
		const lineHeight = 16
		drawPatch("M_SVOL", 60, 38)
		drawPatch("M_SFXVOL", menuX, menuY)
		drawPatch("M_MUSVOL", menuX, menuY+2*lineHeight)
		drawText(formatInt(frontendVolumeDot(g.opts.SFXVolume)), 235, menuY+2, 1.2)
		drawText(formatInt(frontendVolumeDot(g.opts.MusicVolume)), 235, menuY+2*lineHeight+2, 1.2)
		skullY := menuY
		if g.pauseMenuSoundOn != 0 {
			skullY += 2 * lineHeight
		}
		drawSkull(menuX-32, skullY)
	case pauseMenuModeVoice:
		const menuX = 24
		const menuY = 44
		const lineHeight = 16
		backLabel := "BACK: ESC"
		backX := 320 - 8 - int(math.Ceil(float64(g.huTextWidth(backLabel))*1.2))
		drawText("VOICE", menuX, 18, 1.4)
		drawText(backLabel, float64(backX), 18, 1.2)
		labels := []string{"PRESET"}
		values := []string{(&sessionGame{opts: g.opts}).voicePresetLabel()}
		labels = append(labels, voiceCodecDetailMenuLabel(g.opts.VoiceCodec), "SAMPLE RATE", "AUTOMATIC VOLUME", "NOISE GATE", "GATE STRENGTH")
		values = append(values, (&sessionGame{opts: g.opts}).voiceCodecDetailLabel(), voiceSampleRateMenuLabel(g.opts.VoiceSampleRate), voiceAGCLabel(g.opts.VoiceAGCEnabled), voiceGateLabel(g.opts.VoiceGateEnabled), voiceGateThresholdLabel(g.opts.VoiceGateThreshold))
		for i := 0; i < len(labels); i++ {
			y := float64(menuY + i*lineHeight + 2)
			drawText(labels[i], menuX, y, 1.2)
			drawText(values[i], menuX+170, y, 1.2)
		}
		infoY := menuY + len(labels)*lineHeight + 12
		drawText("MIC METER", menuX, float64(infoY), 1.0)
		const meterDots = 12
		level := 0.0
		if g.opts.VoiceInputLevel != nil {
			level = g.opts.VoiceInputLevel()
			if level < 0 {
				level = 0
			}
			if level > 1 {
				level = 1
			}
		}
		gateActive := false
		if g.opts.VoiceInputGateActive != nil {
			gateActive = g.opts.VoiceInputGateActive()
		}
		const barX = menuX + 86
		barY := infoY - 4
		const barW = 108
		const barH = 10
		frame := color.RGBA{R: 160, G: 32, B: 24, A: 255}
		if gateActive {
			frame = color.RGBA{R: 112, G: 112, B: 112, A: 255}
		}
		drawRect(barX, barY, barW, barH, frame)
		drawRect(barX+1, barY+1, barW-2, barH-2, color.RGBA{R: 8, G: 8, B: 8, A: 255})
		fillW := int(math.Round(float64(barW-2) * level))
		if fillW > 0 {
			fill := color.RGBA{R: 172, G: 124, B: 48, A: 255}
			if gateActive {
				fill = color.RGBA{R: 112, G: 112, B: 112, A: 255}
			}
			drawRect(barX+1, barY+1, fillW, barH-2, fill)
		}
		deviceLabel := "USES SYSTEM DEFAULT INPUT"
		if strings.TrimSpace(g.opts.VoiceInputDevice) != "" {
			deviceLabel = "INPUT: " + strings.ToUpper(strings.TrimSpace(g.opts.VoiceInputDevice))
		}
		drawText(deviceLabel, menuX, float64(infoY+12), 1.0)
		drawText("LEFT/RIGHT CHANGE  ENTER SELECT", menuX, float64(infoY+28), 1.0)
		drawSkull(menuX-32, menuY+g.pauseMenuVoiceOn*lineHeight)
	case pauseMenuModeEpisode:
		drawPatch("M_NEWG", 96, 14)
		drawPatch("M_EPISOD", 54, 38)
		episodeNames := g.pauseEpisodeNamesScratch[:0]
		for _, ep := range g.availableEpisodeChoices() {
			if name, ok := inGameEpisodeMenuNames[ep]; ok {
				episodeNames = append(episodeNames, name)
			}
		}
		g.pauseEpisodeNamesScratch = episodeNames
		for i, name := range episodeNames {
			drawPatch(name, 48, float64(63+i*16))
		}
		drawSkull(16, 63+g.pauseMenuEpisodeOn*16)
	case pauseMenuModeSkill:
		drawPatch("M_NEWG", 96, 14)
		drawPatch("M_SKILL", 54, 38)
		for i, name := range frontendSkillMenuNames {
			drawPatch(name, 48, float64(63+i*16))
		}
		drawSkull(16, 63+g.pauseMenuSkillOn*16)
	case pauseMenuModeKeybinds:
		g.drawPauseKeybindMenu(screen, drawText, drawSkull)
	default:
		drawPatch("M_PAUSE", 126, 4)
		drawPatch("M_DOOM", 94, 2)
		for i, name := range inGamePauseMenuNames {
			drawPatch(name, 97, float64(64+i*16))
		}
		drawSkull(65, 64+g.pauseMenuItemOn*16)
	}
	if msg := strings.TrimSpace(g.pauseMenuStatus); msg != "" {
		drawText(msg, 160-float64(g.huTextWidth(msg))/2.0, 178, 1.0)
	}
}

func (g *game) pauseMouseSensitivityLayout(menuX int, label string) (thermoX, thermoCount, valueX int) {
	const (
		menuRightEdge = 320
		rightMargin   = 8
		valueWidth    = 28
		labelGap      = 2
	)
	thermoCount = frontendMouseSensitivitySliderDots()
	labelRight := menuX + int(math.Ceil(float64(g.huTextWidth(label))*1.2))
	thermoX = labelRight + labelGap
	maxAvailable := menuRightEdge - rightMargin - valueWidth - thermoX
	fitCount := (maxAvailable - 16) / 8
	if fitCount < 3 {
		fitCount = 3
	}
	if fitCount > thermoCount {
		fitCount = thermoCount
	}
	if fitCount%2 == 0 {
		fitCount--
	}
	if fitCount < 3 {
		fitCount = 3
	}
	thermoCount = fitCount
	valueX = thermoX + 16 + thermoCount*8 + 4
	return thermoX, thermoCount, valueX
}

func (g *game) pauseOptionsSkullX(menuX int) int {
	const gap = 8
	leftEdge := menuX
	haveLabel := false
	for _, name := range frontendOptionsMenuNames {
		if strings.TrimSpace(name) == "" {
			continue
		}
		_, w, _, ox, _, ok := g.menuPatch(name)
		if !ok {
			continue
		}
		labelLeft := menuX - ox
		if !haveLabel || labelLeft < leftEdge {
			leftEdge = labelLeft
			haveLabel = true
		}
		_ = w
	}
	_, skullW, _, skullOX, _, ok := g.menuPatch("M_SKULL1")
	if ok {
		return leftEdge - gap - skullW + skullOX
	}
	return leftEdge - 32
}

func (g *game) finishPerfCounter(drawStart time.Time) {
	now := time.Now()
	if g.fpsStamp.IsZero() {
		g.fpsStamp = now
	}
	g.fpsFrames++
	renderDur := now.Sub(drawStart) - g.frameUpload
	if renderDur < 0 {
		renderDur = 0
	}
	g.renderAccum += renderDur
	if g.demoBenchmarkActive() {
		g.recordDemoBenchFrame(renderDur)
	}
	elapsed := now.Sub(g.fpsStamp)
	if elapsed >= time.Second {
		fps := float64(g.fpsFrames) / elapsed.Seconds()
		g.ticRateDisplay = float64(g.worldTic-g.worldTicSample) / elapsed.Seconds()
		g.worldTicSample = g.worldTic
		if g.demoBenchmarkActive() {
			g.benchLow1MS = float64(demoBenchLowFrameNS(g.demoBenchFrameNS, 0.99)) / float64(time.Millisecond)
			g.benchLow01MS = float64(demoBenchLowFrameNS(g.demoBenchFrameNS, 0.999)) / float64(time.Millisecond)
		} else {
			g.benchLow1MS = 0
			g.benchLow01MS = 0
		}
		g.fpsDisplay = fps
		if g.fpsFrames > 0 {
			g.renderMSAvg = float64(g.renderAccum) / float64(time.Millisecond) / float64(g.fpsFrames)
			for i := range g.renderStageAccum {
				g.renderStageMS[i] = float64(g.renderStageAccum[i]) / float64(time.Millisecond) / float64(g.fpsFrames)
				g.renderStageAccum[i] = 0
			}
		} else {
			g.renderMSAvg = 0
			for i := range g.renderStageMS {
				g.renderStageMS[i] = 0
				g.renderStageAccum[i] = 0
			}
		}
		g.fpsDisplayText = formatFPSDisplay(g.fpsDisplay, g.renderMSAvg)
		g.ticDisplayText = formatTicDisplay(g.worldTic, g.ticRateDisplay)
		g.renderStageText = formatRenderStageDisplay(g.renderStageMS)
		g.applyAutoDetailSample(g.fpsDisplay, g.renderMSAvg)
		g.fpsFrames = 0
		g.renderAccum = 0
		g.fpsStamp = now
	}
}

func (g *game) addRenderStageDur(stage renderStage, dur time.Duration) {
	if g == nil || stage < 0 || stage >= renderStageCount || dur <= 0 {
		return
	}
	g.renderStageAccum[stage] += dur
}

func (g *game) writePixelsTimed(img *ebiten.Image, pix []byte) {
	start := time.Now()
	img.WritePixels(pix)
	if g.perfInDraw {
		g.frameUpload += time.Since(start)
	}
}

func (g *game) drawPerfOverlay(screen *ebiten.Image) {
	benchDisplay := ""
	if g.demoBenchmarkActive() {
		benchDisplay = formatBenchDisplay(g.benchLow1MS, g.benchLow01MS)
	}
	ticDisplay, hostDisplay := perfOverlayTimingDisplays(
		g.opts.ShowTPS,
		g.ticDisplayText,
		ebiten.ActualTPS(),
		ebiten.ActualFPS(),
	)
	hud.DrawPerfOverlay(screen, hud.PerfInputs{
		ViewW:       g.viewW,
		ViewH:       g.viewH,
		SourcePort:  g.hudUsesLogicalLayout(),
		HUDScale:    g.hudScaleValue(),
		FPSDisplay:  g.fpsDisplayText,
		TicDisplay:  ticDisplay,
		HostDisplay: hostDisplay,
		BenchLine:   benchDisplay,
	}, g.huTextWidth, g.drawHUTextAt)
}

func (g *game) drawNetBandwidth(screen *ebiten.Image) {
	if g == nil || screen == nil {
		return
	}
	if !netBandwidthOverlayEnabled() {
		return
	}
	label := formatNetBandwidthLabel(g.opts.NetBandwidthMeter, g.opts.VoiceBandwidthMeter, g.opts.VoiceSyncMeter, g.opts.LiveTicSink != nil && g.opts.LiveTicSource == nil)
	if label == "" {
		return
	}
	x := 8
	y := 8
	ebitenutil.DebugPrintAt(screen, label, x, y)
}

func formatNetBandwidthLabel(gameMeter, voiceMeter runtimecfg.NetBandwidthMeter, voiceSync runtimecfg.VoiceSyncMeter, preferUpload bool) string {
	var parts []string
	total := bandwidthMeterValue(gameMeter, preferUpload) + bandwidthMeterValue(voiceMeter, preferUpload)
	if total > 0 {
		parts = append(parts, formatCompactBandwidth(total))
	}
	if voiceSync != nil && voiceSyncOverlayEnabled() {
		if offsetMillis, ok := voiceSync.VoiceSyncOffsetMillis(); ok {
			parts = append(parts, "sync "+formatSignedMillis(offsetMillis))
		}
	}
	return strings.Join(parts, "  ")
}

func voiceSyncOverlayEnabled() bool {
	return strings.TrimSpace(os.Getenv("GD_DOOM_VOICE_SYNC_OVERLAY")) != ""
}

func netBandwidthOverlayEnabled() bool {
	return strings.TrimSpace(os.Getenv("GD_DOOM_NET_BANDWIDTH_OVERLAY")) != ""
}

func bandwidthMeterValue(m runtimecfg.NetBandwidthMeter, preferUpload bool) float64 {
	if m == nil {
		return 0
	}
	up, down := m.BandwidthStats()
	if preferUpload {
		return up
	}
	return down
}

func formatCompactBandwidth(v float64) string {
	return strings.ReplaceAll(humanize.SIWithDigits(v, 2, "B/s"), " ", "")
}

func formatSignedMillis(v int) string {
	if v >= 0 {
		return "+" + strconv.Itoa(v) + "ms"
	}
	return strconv.Itoa(v) + "ms"
}

func perfOverlayTimingDisplays(showTPS bool, ticDisplay string, actualTPS, actualFPS float64) (string, string) {
	if !showTPS {
		return "", ""
	}
	return ticDisplay, formatHostDisplay(actualTPS, actualFPS)
}

func (g *game) demoBenchmarkActive() bool {
	return g != nil && g.opts.DemoScript != nil && g.opts.DemoQuitOnComplete
}

func formatFloat2(v float64) string {
	return strconv.FormatFloat(v, 'f', 2, 64)
}

func formatInt(v int) string {
	return strconv.Itoa(v)
}

func formatTicDisplay(tic int, rate float64) string {
	var b []byte
	b = append(b, 't', 'i', 'c', ' ')
	b = strconv.AppendInt(b, int64(tic), 10)
	b = append(b, ' ', '|', ' ', 't', 'p', 's', ' ')
	b = strconv.AppendFloat(b, rate, 'f', 2, 64)
	return string(b)
}

func formatFPSDisplay(fps, renderMS float64) string {
	var b []byte
	b = strconv.AppendFloat(b, fps, 'f', 2, 64)
	b = append(b, ',', ' ')
	b = strconv.AppendFloat(b, renderMS, 'f', 2, 64)
	b = append(b, 'm', 's')
	return string(b)
}

func formatHostDisplay(actualTPS, actualFPS float64) string {
	var b []byte
	b = append(b, 'e', 'b', 'i', ' ')
	b = strconv.AppendFloat(b, actualTPS, 'f', 2, 64)
	b = append(b, ' ', 't', 'p', 's', ' ', '|', ' ', 'f', 'p', 's', ' ')
	b = strconv.AppendFloat(b, actualFPS, 'f', 2, 64)
	return string(b)
}

func formatBenchDisplay(low1MS, low01MS float64) string {
	var b []byte
	b = append(b, '1', '%', ' ')
	b = strconv.AppendFloat(b, low1MS, 'f', 2, 64)
	b = append(b, 'm', 's', ' ', '|', ' ', '0', '.', '1', '%', ' ')
	b = strconv.AppendFloat(b, low01MS, 'f', 2, 64)
	b = append(b, 'm', 's')
	return string(b)
}

func formatRenderStageDisplay(ms [renderStageCount]float64) string {
	var b []byte
	for i, label := range renderStageLabels {
		if i > 0 {
			b = append(b, ' ')
		}
		b = append(b, label...)
		b = append(b, '=')
		b = strconv.AppendFloat(b, ms[i], 'f', 2, 64)
	}
	return string(b)
}

func (g *game) recordDemoBenchFrame(renderDur time.Duration) {
	if g == nil {
		return
	}
	ns := renderDur.Nanoseconds()
	if ns < 0 {
		ns = 0
	}
	g.demoBenchFrameNS = append(g.demoBenchFrameNS, ns)
	fmt.Printf("demo-frame tick=%d draw=%d ns=%d ms=%.3f\n", g.worldTic, g.demoBenchDraws, ns, float64(ns)/float64(time.Millisecond))
}

func demoBenchLowFrameNS(frameNS []int64, quantile float64) int64 {
	if len(frameNS) == 0 {
		return 0
	}
	if quantile < 0 {
		quantile = 0
	}
	if quantile > 1 {
		quantile = 1
	}
	sorted := append([]int64(nil), frameNS...)
	slices.Sort(sorted)
	idx := int(math.Ceil(quantile*float64(len(sorted)))) - 1
	if idx < 0 {
		idx = 0
	}
	if idx >= len(sorted) {
		idx = len(sorted) - 1
	}
	return sorted[idx]
}

func demoBenchFPSFromFrameNS(frameNS int64) float64 {
	if frameNS <= 0 {
		return 0
	}
	return 1e9 / float64(frameNS)
}

func (g *game) huTextWidth(text string) int {
	if len(g.opts.MessageFontBank) == 0 {
		return len(text) * 7
	}
	w := 0
	for _, ch := range text {
		uc := ch
		if uc >= 'a' && uc <= 'z' {
			uc -= 'a' - 'A'
		}
		if uc == ' ' || uc < huFontStart || uc > huFontEnd {
			w += 4
			continue
		}
		_, gw, _, _, _, ok := g.messageFontGlyph(uc)
		if !ok {
			w += 4
			continue
		}
		w += gw
	}
	return w
}

func (g *game) drawHUTextAt(screen *ebiten.Image, text string, x, y, sx, sy float64) {
	if strings.TrimSpace(text) == "" {
		return
	}
	if len(g.opts.MessageFontBank) == 0 {
		ebitenutil.DebugPrintAt(screen, text, int(x), int(y))
		return
	}
	px := x
	py := y
	lineAdvance := 9 * sy
	for _, ch := range text {
		if ch == '\n' {
			px = x
			py += lineAdvance
			continue
		}
		uc := ch
		if uc >= 'a' && uc <= 'z' {
			uc -= 'a' - 'A'
		}
		if uc == ' ' || uc < huFontStart || uc > huFontEnd {
			px += 4 * sx
			continue
		}
		img, w, _, ox, oy, ok := g.messageFontGlyph(uc)
		if !ok {
			px += 4 * sx
			continue
		}
		op := &ebiten.DrawImageOptions{}
		op.Filter = ebiten.FilterNearest
		op.GeoM.Scale(sx, sy)
		op.GeoM.Translate(px-float64(ox)*sx, py-float64(oy)*sy)
		screen.DrawImage(img, op)
		px += float64(w) * sx
	}
}
