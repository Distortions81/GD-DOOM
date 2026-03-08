package automap

import (
	"fmt"
	"image"
	"image/color"
	"math"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"
	"unsafe"

	"gddoom/internal/doomrand"
	"gddoom/internal/mapdata"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

const (
	doomLogicalW = 320
	doomLogicalH = 200

	lineOneSidedWidth  = 1.8
	lineTwoSidedWidth  = 1.2
	doomInitialZoomMul = 1.0 / 0.7
	// Give cursor capture/resizing a couple of frames to settle after detail changes.
	detailMouseSuppressTicks         = 3
	mlDontPegTop                     = 1 << 3
	mlDontPegBottom                  = 1 << 4
	statusBaseW                      = 320.0
	statusBaseH                      = 200.0
	statusBarY                       = 168.0
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
	doomColormapRGBA     []uint32
	doomPalIndexLUT32    []uint8
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
	{doomLogicalW, doomLogicalH}, // high detail
	{doomLogicalW, doomLogicalH}, // low detail (column-doubled)
}

var sourcePortDetailDivisors = []int{1, 2, 3, 4}

type projectedProjectileItem struct {
	dist       float64
	sx         float64
	yb         float64
	h          float64
	clipSpans  []solidSpan
	clipTop    int
	clipBottom int
	clr        color.RGBA
	lightMul   uint32
	fullBright bool
	spriteKey  string
	opaque     spriteOpaqueShape
	hasOpaque  bool
	spriteTex  WallTexture
	hasSprite  bool
}

type projectedMonsterItem struct {
	dist       float64
	sx         float64
	yb         float64
	h          float64
	w          float64
	clipSpans  []solidSpan
	clipTop    int
	clipBottom int
	tex        WallTexture
	flip       bool
	lightMul   uint32
	fullBright bool
	spriteKey  string
	opaque     spriteOpaqueShape
	hasOpaque  bool
	shadow     bool
}

var doomFuzzOffsets = [...]int{
	1, -1, 1, -1, 1, 1, -1,
	1, 1, -1, 1, 1, 1, -1,
	1, 1, 1, -1, -1, -1, -1,
	1, -1, -1, 1, 1, 1, 1, -1,
	1, -1, 1, 1, -1, -1, 1,
	1, -1, -1, -1, -1, 1, 1,
	1, 1, -1, 1, 1, -1, 1,
}

type projectedThingItem struct {
	dist       float64
	sx         float64
	yb         float64
	h          float64
	clipSpans  []solidSpan
	clipTop    int
	clipBottom int
	tex        WallTexture
	lightMul   uint32
	fullBright bool
	spriteKey  string
	opaque     spriteOpaqueShape
	hasOpaque  bool
}

type projectedPuffItem struct {
	dist       float64
	sx         float64
	sy         float64
	r          float64
	clipSpans  []solidSpan
	clipTop    int
	clipBottom int
	kind       uint8
	spriteTex  WallTexture
	hasSprite  bool
}

type spriteOpaqueBounds struct {
	minX int
	maxX int
	minY int
	maxY int
}

type spriteOpaqueRect struct {
	minX int16
	maxX int16
	minY int16
	maxY int16
}

type spriteOpaqueShape struct {
	bounds spriteOpaqueBounds
	rowMin []int16
	rowMax []int16
	rects  []spriteOpaqueRect
}

type billboardQueueKind uint8

const (
	billboardQueueProjectiles billboardQueueKind = iota
	billboardQueueMonsters
	billboardQueueWorldThings
	billboardQueuePuffs
)

type billboardQueueItem struct {
	dist float64
	kind billboardQueueKind
	idx  int
}

type hitscanPuff struct {
	x    int64
	y    int64
	z    int64
	tics int
	kind uint8
}

const (
	hitscanFxPuff uint8 = iota
	hitscanFxBlood
)

const (
	// Fallback circle size when sprite patches are unavailable.
	hitscanPuffWorldHeight  = 16.0
	hitscanBloodWorldHeight = 16.0
)

type maskedMidSeg struct {
	dist      float64
	x0        int
	x1        int
	sx1       float64
	sx2       float64
	invF1     float64
	invF2     float64
	uOverF1   float64
	uOverF2   float64
	worldHigh float64
	worldLow  float64
	texUOff   float64
	texMid    float64
	tex       WallTexture
	light     int16
	lightBias int
}

type maskedClipSpan struct {
	y0      int16
	y1      int16
	openY0  int16
	openY1  int16
	depthQ  uint16
	closed  bool
	hasOpen bool
}

type mapLineDraw struct {
	x1  float32
	y1  float32
	x2  float32
	y2  float32
	w   float32
	clr color.RGBA
}

type mapLineCacheKey struct {
	camX          float64
	camY          float64
	zoom          float64
	angle         uint32
	rotateView    bool
	viewW         int
	viewH         int
	reveal        revealMode
	iddt          int
	lineColorMode string
	mappedRev     uint32
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
	camX              float64
	camY              float64
	zoom              float64
	fitZoom           float64

	mode                      viewMode
	followMode                bool
	rotateView                bool
	parity                    automapParityState
	showGrid                  bool
	showLegend                bool
	bigMap                    bool
	paused                    bool
	pauseMenuActive           bool
	pauseMenuMode             int
	pauseMenuItemOn           int
	pauseMenuOptionsOn        int
	pauseMenuSoundOn          int
	pauseMenuEpisodeOn        int
	pauseMenuSelectedEpisode  int
	pauseMenuSkillOn          int
	pauseMenuSkullAnimCounter int
	pauseMenuWhichSkull       int
	pauseMenuStatus           string
	pauseMenuStatusTics       int
	quitPromptRequested       bool
	readThisRequested         bool
	quitPromptActive          bool
	newGameRequestedMap       *mapdata.Map
	newGameRequestedSkill     int
	savedView                 savedMapView
	marks                     []mapMark
	nextMarkID                int
	p                         player
	localSlot                 int
	peerStarts                []playerStart

	lines                []physLine
	lineValid            []int
	validCount           int
	bmapOriginX          int64
	bmapOriginY          int64
	bmapWidth            int
	bmapHeight           int
	physForLine          []int
	renderSeen           []int
	renderEpoch          int
	visibleBuf           []int
	bspOccBuf            []solidSpan
	visibleSectorSeen    []int
	visibleSubSectorSeen []int
	visibleEpoch         int
	nodeChildRangeEpoch  []int
	nodeChildRangeL      []int
	nodeChildRangeR      []int
	nodeChildRangeOK     []uint8
	thingSectorCache     []int
	mapLineBuf           []mapLineDraw
	mapLineKey           mapLineCacheKey
	mapLineRev           uint32
	mapLineInit          bool
	sectorFloor          []int64
	sectorCeil           []int64
	lineSpecial          []uint16
	doors                map[int]*doorThinker
	floors               map[int]*floorThinker
	plats                map[int]*platThinker
	ceilings             map[int]*ceilingThinker
	useFlash             int
	useText              string
	hudMessagesEnabled   bool
	turnHeld             int
	snd                  *soundSystem
	soundQueue           []soundEvent
	soundQueueOrigin     []queuedSoundOrigin
	delayedSfx           []delayedSoundEvent
	delayedSwitchReverts []delayedSwitchTexture

	prevCamX  float64
	prevCamY  float64
	prevPX    int64
	prevPY    int64
	prevAngle uint32

	renderCamX  float64
	renderCamY  float64
	renderPX    float64
	renderPY    float64
	renderAngle uint32
	renderAlpha float64
	debugAimSS  int

	lastUpdate    time.Time
	fpsFrames     int
	fpsStamp      time.Time
	fpsDisplay    float64
	renderAccum   time.Duration
	renderMSAvg   float64
	frameUpload   time.Duration
	perfInDraw    bool
	interpAutoOff bool
	simTickScale  float64
	simTickAccum  float64
	edgeInputPass bool
	pendingUse    bool

	lastMouseX             int
	mouseLookSet           bool
	mouseLookSuppressTicks int

	levelExitRequested    bool
	secretLevelExit       bool
	levelRestartRequested bool

	thingCollected       []bool
	thingDropped         []bool
	thingX               []int64
	thingY               []int64
	thingHP              []int
	thingAggro           []bool
	thingCooldown        []int
	thingMoveDir         []monsterMoveDir
	thingMoveCount       []int
	thingJustAtk         []bool
	thingJustHit         []bool
	thingReactionTics    []int
	thingDead            []bool
	thingDeathTics       []int
	thingAttackTics      []int
	thingAttackFireTics  []int
	thingPainTics        []int
	thingThinkWait       []int
	projectiles          []projectile
	projectileImpacts    []projectileImpact
	hitscanPuffs         []hitscanPuff
	cheatLevel           int
	invulnerable         bool
	inventory            playerInventory
	alwaysRun            bool
	autoWeaponSwitch     bool
	weaponRefire         bool
	weaponAttackDown     bool
	weaponState          weaponPspriteState
	weaponStateTics      int
	weaponFlashState     weaponPspriteState
	weaponFlashTics      int
	stats                playerStats
	worldTic             int
	spectreFuzzPos       int
	spectreFuzzAccum     float64
	spectreFuzzStamp     time.Time
	spectreFuzzCoarseX   int
	spectreFuzzCoarseY   int
	spectreFuzzCoarseSet bool
	playerViewZ          int64
	secretFound          []bool
	secretsFound         int
	secretsTotal         int
	sectorSoundTarget    []bool
	isDead               bool
	damageFlashTic       int
	bonusFlashTic        int
	sectorLightFx        []sectorLightEffect
	subSectorSec         []int
	sectorBBox           []worldBBox
	subSectorLoopVerts   [][]uint16
	subSectorLoopDiag    []loopBuildDiag
	subSectorPoly        [][]worldPt
	subSectorTris        [][][3]int
	subSectorBBox        []worldBBox
	dynamicSectorMask    []bool
	staticSubSectorMask  []bool
	subSectorPlaneID     []int
	sectorSubSectors     [][]int
	holeFillPolys        []holeFillPoly
	sectorPlaneTris      [][]worldTri
	sectorPlaneCache     []sectorPlaneCacheEntry
	orphanSubSector      []bool
	orphanRepairQueue    []orphanRepairCandidate

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
	textureAnimCrossfadeFrames   int
	spriteOpaqueShapeCache       map[string]spriteOpaqueShape
	thingSpriteExpandCache       map[string][]string
	planeFlatCache32Scratch      map[string][]uint32
	planeFlatCacheIndexedScratch map[string][]byte
	planeFBPackedScratch         []uint32
	planeFlatTex32Scratch        [][]uint32
	planeFlatTexIndexedScratch   [][]byte
	planeFlatReadyScratch        []bool
	projectileItemsScratch       []projectedProjectileItem
	monsterItemsScratch          []projectedMonsterItem
	thingItemsScratch            []projectedThingItem
	puffItemsScratch             []projectedPuffItem
	billboardQueueScratch        []billboardQueueItem
	billboardQueueCollect        bool
	billboardReplayActive        bool
	billboardReplayKind          billboardQueueKind
	billboardReplayIndex         int
	maskedMidSegsScratch         []maskedMidSeg
	spriteTXScratch              []int
	spriteTYScratch              []int
	spriteSourceColumnScratch    []uint32
	spriteColumnScratch          []uint32
	billboardColumnRunsScratch   []billboardColumnRun
	wallLayer                    *ebiten.Image
	wallPix                      []byte
	wallPix32                    []uint32
	wallW                        int
	wallH                        int
	depthPix3D                   []uint32
	depthPlanePix3D              []uint32
	depthFrameStamp              uint16
	wallDepthQCol                []uint16
	wallDepthTopCol              []int
	wallDepthBottomCol           []int
	wallDepthClosedCol           []bool
	maskedClipCols               [][]maskedClipSpan
	wallTop3D                    []int
	wallBottom3D                 []int
	ceilingClip3D                []int
	floorClip3D                  []int
	buffers3DW                   int
	buffers3DH                   int
	flatImgCache                 map[string]*ebiten.Image
	statusPatchImg               map[string]*ebiten.Image
	menuPatchImg                 map[string]*ebiten.Image
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
	spriteClipDiag               bool
	spriteClipDiagOnly           bool
	spriteClipDiagGreenOnly      bool
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
	plane3DPool                  []*plane3DVisplane
	plane3DPoolUsed              int
	plane3DPoolViewW             int
	wallSegStaticCache           []wallSegStatic
	wallPrepassBuf               []wallSegPrepass
	solid3DBuf                   []solidSpan
	solidClipScratch             []solidSpan
	demoTick                     int
	demoDoneReported             bool
	demoBenchStarted             bool
	statusFaceIndex              int
	statusFaceCount              int
	statusFacePriority           int
	statusOldHealth              int
	statusRandom                 int
	statusLastAttack             int
	statusAttackDown             bool
	statusAttackerX              int64
	statusAttackerY              int64
	statusHasAttacker            bool
	statusOldWeapons             [8]bool
	statusDamageCount            int
	statusBonusCount             int
	demoBenchStart               time.Time
	demoBenchDraws               int
	demoStartRnd                 int
	demoStartPRnd                int
	demoRNGCaptured              bool
	demoRecord                   []DemoTic
}

type billboardColumnRun struct {
	x0 int
	x1 int
	tx int
}

type savedMapView struct {
	camX   float64
	camY   float64
	zoom   float64
	follow bool
	valid  bool
}

type mapMark struct {
	id int
	x  float64
	y  float64
}

type delayedSoundEvent struct {
	ev         soundEvent
	tics       int
	x          int64
	y          int64
	positioned bool
}

type queuedSoundOrigin struct {
	x          int64
	y          int64
	positioned bool
}

type delayedSwitchTexture struct {
	sidedef int
	top     string
	bottom  string
	mid     string
	tics    int
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
	sx1             float64
	sx2             float64
	minSX           int
	maxSX           int
	invF1           float64
	invF2           float64
	uOverF1         float64
	uOverF2         float64
	logReason       string
	logZ1           float64
	logZ2           float64
	logX1           float64
	logX2           float64
	ok              bool
}

type wallSegStatic struct {
	valid             bool
	ld                mapdata.Linedef
	frontSide         int
	frontSideDefIdx   int
	frontSectorIdx    int
	backSectorIdx     int
	x1w               float64
	y1w               float64
	x2w               float64
	y2w               float64
	segLen            float64
	uBase             float64
	hasTwoSidedMidTex bool
}

type worldTri struct {
	a worldPt
	b worldPt
	c worldPt
}

type sectorPlaneCacheEntry struct {
	tris      []worldTri
	dynamic   bool
	lastFloor int64
	lastCeil  int64
	dirty     bool
	lightKind sectorLightEffectKind
	light     int16
	lightMul  uint8
	texID     string
	tex       *ebiten.Image
	flatRGBA  []byte
	texTick   int
}

type orphanRepairCandidate struct {
	ss    int
	sec   int
	votes int
}

type wallPortalState struct {
	worldTop    float64
	worldBottom float64
	worldHigh   float64
	worldLow    float64
	topWall     bool
	bottomWall  bool
	markCeiling bool
	markFloor   bool
	solidWall   bool
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
	ensurePositiveRenderSize(&opts)
	opts.SkillLevel = normalizeSkillLevel(opts.SkillLevel)
	opts.GameMode = normalizeGameMode(opts.GameMode)
	opts.MouseLookSpeed = normalizeMouseLookSpeed(opts.MouseLookSpeed)
	opts.KeyboardTurnSpeed = normalizeKeyboardTurnSpeed(opts.KeyboardTurnSpeed)
	opts.MusicVolume = clampVolume(opts.MusicVolume)
	opts.SFXVolume = clampVolume(opts.SFXVolume)
	opts.OPLVolume = clampOPLVolume(opts.OPLVolume)
	if !opts.SourcePortMode {
		// Doom mode keeps strict parity color semantics.
		opts.LineColorMode = "parity"
	}
	opts.SourcePortThingRenderMode = normalizeSourcePortThingRenderMode(opts.SourcePortThingRenderMode, opts.SourcePortMode)
	opts.SkyUpscaleMode = normalizeSkyUpscaleMode(opts.SkyUpscaleMode, opts.SourcePortMode)
	if opts.PlayerSlot < 1 || opts.PlayerSlot > 4 {
		opts.PlayerSlot = 1
	}
	p, localSlot, starts := spawnPlayer(m, opts.PlayerSlot)
	g := &game{
		m:                 m,
		opts:              opts,
		bounds:            mapBounds(m),
		paletteLUTEnabled: !opts.SourcePortMode,
		gammaLevel:        2,
		crtEnabled:        opts.CRTEffect,
		viewW:             opts.Width,
		viewH:             opts.Height,
		skyOutputW:        max(opts.Width, 1),
		skyOutputH:        max(opts.Height, 1),
		mode:              viewMap,
		followMode:        true,
		rotateView:        opts.SourcePortMode,
		parity: automapParityState{
			reveal: revealNormal,
			iddt:   0,
		},
		showGrid:           false,
		showLegend:         opts.SourcePortMode,
		bigMap:             false,
		hudMessagesEnabled: true,
		marks:              make([]mapMark, 0, 16),
		nextMarkID:         1,
		p:                  p,
		localSlot:          localSlot,
		peerStarts:         nonLocalStarts(starts, localSlot),
		cullLogBudget:      0,
		floorDbgMode:       floorDebugTextured,
		floorVisDiag:       floorVisDiagOff,
		alwaysRun:          opts.AlwaysRun,
		autoWeaponSwitch:   opts.AutoWeaponSwitch,
		simTickScale:       1.0,
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
	g.textureAnimCrossfadeFrames = 0
	g.thingSpriteExpandCache = make(map[string][]string, 256)
	g.detailLevel = defaultDetailLevelForMode(g.viewW, g.viewH, opts.SourcePortMode)
	if opts.InitialDetailLevel >= 0 {
		g.detailLevel = clampDetailLevelForMode(opts.InitialDetailLevel, opts.SourcePortMode)
	}
	if opts.InitialGammaLevel >= 0 {
		g.gammaLevel = clampGamma(opts.InitialGammaLevel)
	}
	g.initPlayerState()
	g.initStatusFaceState()
	g.thingCollected = make([]bool, len(m.Things))
	g.thingDropped = make([]bool, len(m.Things))
	g.thingX = make([]int64, len(m.Things))
	g.thingY = make([]int64, len(m.Things))
	g.thingHP = make([]int, len(m.Things))
	g.thingAggro = make([]bool, len(m.Things))
	g.thingCooldown = make([]int, len(m.Things))
	g.thingMoveDir = make([]monsterMoveDir, len(m.Things))
	g.thingMoveCount = make([]int, len(m.Things))
	g.thingJustAtk = make([]bool, len(m.Things))
	g.thingJustHit = make([]bool, len(m.Things))
	g.thingReactionTics = make([]int, len(m.Things))
	g.thingDead = make([]bool, len(m.Things))
	g.thingDeathTics = make([]int, len(m.Things))
	g.thingAttackTics = make([]int, len(m.Things))
	g.thingAttackFireTics = make([]int, len(m.Things))
	for i := range g.thingAttackFireTics {
		g.thingAttackFireTics[i] = -1
	}
	g.thingPainTics = make([]int, len(m.Things))
	g.thingThinkWait = make([]int, len(m.Things))
	g.secretFound = make([]bool, len(m.Sectors))
	g.sectorSoundTarget = make([]bool, len(m.Sectors))
	for _, sec := range m.Sectors {
		if sec.Special == 9 {
			g.secretsTotal++
		}
	}
	g.initSectorLightEffects()
	g.initThingCombatState()
	g.applyThingSpawnFiltering()
	g.cheatLevel = normalizeCheatLevel(opts.CheatLevel)
	g.invulnerable = opts.Invulnerable
	if !g.opts.StartInMapMode {
		g.mode = viewWalk
	}
	if g.opts.DemoScript != nil {
		// Demo benchmark mode is intentionally isolated from interactive controls.
		g.mode = viewWalk
		g.followMode = true
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
	g.initSubSectorSectorCache()
	g.snd = newSoundSystem(opts.SoundBank, opts.SFXVolume, opts.SourcePortMode)
	g.soundQueue = make([]soundEvent, 0, 8)
	g.soundQueueOrigin = make([]queuedSoundOrigin, 0, 8)
	g.delayedSfx = make([]delayedSoundEvent, 0, 8)
	g.delayedSwitchReverts = make([]delayedSwitchTexture, 0, 4)
	if g.opts.SourcePortMode {
		// Source-port defaults: reveal full map style and heading-follow at startup.
		g.parity.reveal = revealAllMap
	}
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
	g.physForLine = make([]int, len(g.m.Linedefs))
	for i := range g.physForLine {
		g.physForLine[i] = -1
	}
	for i, pl := range g.lines {
		if pl.idx >= 0 && pl.idx < len(g.physForLine) {
			g.physForLine[pl.idx] = i
		}
	}
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
		g.thingX[i] = int64(th.X) << fracBits
		g.thingY[i] = int64(th.Y) << fracBits
		g.thingSectorCache[i] = g.sectorAt(g.thingX[i], g.thingY[i])
	}
	g.discoverLinesAroundPlayer()
	g.resetView()
	if opts.StartZoom > 0 {
		g.zoom = opts.StartZoom
	}
	g.syncRenderState()
	if g.mode == viewWalk {
		// Avoid startup cursor-capture deltas rotating the initial spawn heading.
		g.mouseLookSet = false
		g.mouseLookSuppressTicks = detailMouseSuppressTicks
	}
	g.runtimeSettingsSeen = true
	g.runtimeSettingsLast = g.runtimeSettingsSnapshot()
	return g
}

func (g *game) clearSpritePatchCache() {
	if g == nil {
		return
	}
	g.spritePatchImg = nil
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
		if len(sourcePortDetailDivisors) > 1 {
			return 1
		}
		return 0
	}
	return detailPresetIndex(viewW, viewH)
}

func (g *game) resetView() {
	g.camX = (g.bounds.minX + g.bounds.maxX) / 2
	g.camY = (g.bounds.minY + g.bounds.maxY) / 2

	worldW := math.Max(g.bounds.maxX-g.bounds.minX, 1)
	worldH := math.Max(g.bounds.maxY-g.bounds.minY, 1)
	margin := 0.9
	zx := float64(max(g.viewW, 1)) * margin / worldW
	zy := float64(max(g.viewH, 1)) * margin / worldH
	g.fitZoom = math.Max(math.Min(zx, zy), 0.0001)
	g.zoom = g.fitZoom * doomInitialZoomMul
}

func detailPresetIndex(w, h int) int {
	for i, p := range detailPresets {
		if p[0] == w && p[1] == h {
			return i
		}
	}
	return 0
}

func (g *game) cycleDetailLevel() {
	if len(detailPresets) == 0 {
		return
	}
	g.detailLevel = (g.detailLevel + 1) % len(detailPresets)
	p := detailPresets[g.detailLevel]
	oldFit := g.fitZoom
	g.viewW = p[0]
	g.viewH = p[1]

	worldW := math.Max(g.bounds.maxX-g.bounds.minX, 1)
	worldH := math.Max(g.bounds.maxY-g.bounds.minY, 1)
	margin := 0.9
	zx := float64(max(g.viewW, 1)) * margin / worldW
	zy := float64(max(g.viewH, 1)) * margin / worldH
	g.fitZoom = math.Max(math.Min(zx, zy), 0.0001)
	if oldFit > 0 {
		g.zoom = (g.zoom / oldFit) * g.fitZoom
	} else {
		g.zoom = g.fitZoom * doomInitialZoomMul
	}
	label := "HIGH"
	if g.lowDetailMode() {
		label = "LOW"
	}
	g.setHUDMessage(fmt.Sprintf("Detail: %s", label), 70)
	// Avoid a large turn delta on the next walk-mode update after viewport size changes.
	g.mouseLookSet = false
	g.mouseLookSuppressTicks = detailMouseSuppressTicks
	// Keep interpolation state aligned to current state to prevent one-frame render pops.
	g.syncRenderState()
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
	g.detailLevel = (g.detailLevel + 1) % len(sourcePortDetailDivisors)
	div := g.sourcePortDetailDivisor()
	if div <= 1 {
		g.setHUDMessage("Detail: 1x", 70)
	} else {
		g.setHUDMessage(fmt.Sprintf("Detail: 1/%dx", div), 70)
	}
	// Detail ratio changes rewire sourceport internal resolution, so force a
	// clean sky shader/image state before the next frame.
	g.resetSkyLayerPipeline(true)
	g.mouseLookSet = false
	g.mouseLookSuppressTicks = detailMouseSuppressTicks
}

func (g *game) mouseLookTurnRaw(dx int) int64 {
	return mouseLookTurnRawWithWidth(dx, g.opts.MouseLookSpeed, g.viewW)
}

func mouseLookTurnRawWithWidth(dx int, speed float64, renderW int) int64 {
	if dx == 0 {
		return 0
	}
	base := float64(40 << 16)
	scale := mouseLookResolutionScale(renderW)
	raw := int64(math.Round(float64(dx) * scale * base * speed))
	if raw == 0 {
		if dx > 0 {
			raw = 1
		} else {
			raw = -1
		}
	}
	// Positive mouse delta should turn right (decrease world angle).
	return -raw
}

func mouseLookResolutionScale(renderW int) float64 {
	refW := doomLogicalW
	if renderW <= 0 {
		renderW = refW
	}
	if renderW < 1 {
		renderW = 1
	}
	return float64(refW) / float64(renderW)
}

func (g *game) runtimeSettingsSnapshot() RuntimeSettings {
	return RuntimeSettings{
		DetailLevel:      g.detailLevel,
		GammaLevel:       g.gammaLevel,
		MusicVolume:      g.opts.MusicVolume,
		MUSPanMax:        g.opts.MUSPanMax,
		OPLVolume:        g.opts.OPLVolume,
		SFXVolume:        g.opts.SFXVolume,
		MouseLook:        g.opts.MouseLook,
		AlwaysRun:        g.alwaysRun,
		AutoWeaponSwitch: g.autoWeaponSwitch,
		LineColorMode:    g.opts.LineColorMode,
		ThingRenderMode:  g.opts.SourcePortThingRenderMode,
		CRTEffect:        g.crtEnabled,
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

func (g *game) Update() error {
	if g.levelExitRequested {
		return ebiten.Termination
	}
	if g.opts.DemoScript != nil {
		return g.updateDemoMode()
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyF4) || inpututil.IsKeyJustPressed(ebiten.KeyF10) {
		g.quitPromptRequested = true
		return nil
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		g.togglePauseMenu()
		if g.pauseMenuActive {
			ebiten.SetCursorMode(ebiten.CursorModeVisible)
			return nil
		}
	}
	if g.pauseMenuActive {
		g.tickPauseMenu()
	}
	if g.paused {
		ebiten.SetCursorMode(ebiten.CursorModeVisible)
		return nil
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyTab) {
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
	if g.opts.SourcePortMode && inpututil.IsKeyJustPressed(ebiten.KeyR) {
		g.rotateView = !g.rotateView
		if g.rotateView {
			g.setHUDMessage("Heading-Up ON", 70)
		} else {
			g.setHUDMessage("Heading-Up OFF", 70)
		}
	}
	if g.opts.SourcePortMode && inpututil.IsKeyJustPressed(ebiten.KeyBackslash) {
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
	if inpututil.IsKeyJustPressed(ebiten.KeyF1) {
		g.readThisRequested = true
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyComma) {
		g.setSimTickScale(g.simTickScale - 0.1)
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyPeriod) {
		g.setSimTickScale(g.simTickScale + 0.1)
	}
	if inpututil.IsKeyJustPressed(ebiten.KeySlash) {
		g.setSimTickScale(1.0)
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyE) || inpututil.IsKeyJustPressed(ebiten.KeySpace) {
		g.pendingUse = true
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyF5) {
		if g.opts.SourcePortMode {
			g.cycleSourcePortDetailLevel()
		} else {
			g.cycleDetailLevel()
		}
	}
	if g.isDead && (inpututil.IsKeyJustPressed(ebiten.KeyEnter) || inpututil.IsKeyJustPressed(ebiten.KeyKPEnter)) {
		g.requestLevelRestart()
	}
	ticks := g.consumeSimTicks()
	for i := 0; i < ticks; i++ {
		g.edgeInputPass = i == 0
		g.capturePrevState()
		if g.mode == viewMap {
			if g.opts.SourcePortMode {
				ebiten.SetCursorMode(ebiten.CursorModeCaptured)
			} else {
				ebiten.SetCursorMode(ebiten.CursorModeVisible)
			}
			g.updateMapMode()
		} else {
			ebiten.SetCursorMode(ebiten.CursorModeCaptured)
			g.updateWalkMode()
		}
		g.tickStatusWidgets()
		if g.useFlash > 0 {
			g.useFlash--
		}
		if g.damageFlashTic > 0 {
			g.damageFlashTic--
		}
		if g.bonusFlashTic > 0 {
			g.bonusFlashTic--
		}
		g.tickHitscanPuffs()
		g.tickDelayedSounds()
		g.tickDelayedSwitchReverts()
		g.flushSoundEvents()
		g.lastUpdate = time.Now()
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
	g.capturePrevState()
	if !g.demoBenchStarted {
		g.demoBenchStarted = true
		g.demoBenchStart = time.Now()
	}
	if !g.demoRNGCaptured {
		g.demoStartRnd, g.demoStartPRnd = doomrand.State()
		g.demoRNGCaptured = true
	}
	if g.demoTick >= len(script.Tics) {
		if !g.demoDoneReported {
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
			label := "demo"
			if strings.TrimSpace(script.Path) != "" {
				label = script.Path
			}
			fmt.Printf("demo-bench path=%s wad=%s map=%s rng_start=%d/%d tics=%d draws=%d elapsed=%s tps=%.2f fps=%.2f ms_per_tic=%.3f\n",
				label, g.opts.WADHash, g.m.Name, g.demoStartRnd, g.demoStartPRnd, tics, g.demoBenchDraws, elapsed.Round(time.Millisecond), tps, fps, msPerTic)
		}
		return ebiten.Termination
	}
	tc := script.Tics[g.demoTick]
	g.demoTick++
	cmd := moveCmd{
		forward: tc.Forward,
		side:    tc.Side,
		turn:    tc.Turn,
		turnRaw: tc.TurnRaw,
		run:     tc.Run,
	}
	g.runGameplayTic(cmd, tc.Use, tc.Fire)
	g.discoverLinesAroundPlayer()
	g.camX = float64(g.p.x) / fracUnit
	g.camY = float64(g.p.y) / fracUnit
	g.tickStatusWidgets()
	if g.useFlash > 0 {
		g.useFlash--
	}
	if g.damageFlashTic > 0 {
		g.damageFlashTic--
	}
	if g.bonusFlashTic > 0 {
		g.bonusFlashTic--
	}
	g.tickHitscanPuffs()
	g.tickDelayedSounds()
	g.tickDelayedSwitchReverts()
	g.flushSoundEvents()
	g.lastUpdate = time.Now()
	return nil
}

func (g *game) requestLevelExit(secret bool, msg string) {
	g.levelExitRequested = true
	g.secretLevelExit = secret
	g.setHUDMessage(msg, 35)
}

func (g *game) requestLevelRestart() {
	g.levelRestartRequested = true
	g.setHUDMessage("Restarting level...", 20)
}

func (g *game) updateMapMode() {
	g.updateParityControls()
	g.updateWeaponHotkeys(false)
	if g.edgeInputPass && inpututil.IsKeyJustPressed(ebiten.KeyF) {
		g.followMode = !g.followMode
		if g.followMode {
			g.setHUDMessage("Follow ON", 70)
		} else {
			g.setHUDMessage("Follow OFF", 70)
		}
	}
	if g.edgeInputPass && g.opts.SourcePortMode && inpututil.IsKeyJustPressed(ebiten.KeyB) {
		g.toggleBigMap()
	}
	if g.edgeInputPass && (inpututil.IsKeyJustPressed(ebiten.Key0) || inpututil.IsKeyJustPressed(ebiten.KeyKP0)) {
		g.toggleBigMap()
	}
	if g.edgeInputPass && inpututil.IsKeyJustPressed(ebiten.KeyM) {
		g.addMark()
	}
	if g.edgeInputPass && inpututil.IsKeyJustPressed(ebiten.KeyC) {
		g.clearMarks()
	}
	if g.edgeInputPass && g.opts.SourcePortMode && inpututil.IsKeyJustPressed(ebiten.KeyHome) {
		g.resetView()
	}
	g.updateZoom()

	// Keep gameplay simulation active while automap is open.
	cmd := moveCmd{}
	usePressed := false
	firePressed := false
	speed := g.currentRunSpeed()
	if ebiten.IsKeyPressed(ebiten.KeyW) {
		cmd.forward += forwardMove[speed]
	}
	if ebiten.IsKeyPressed(ebiten.KeyS) {
		cmd.forward -= forwardMove[speed]
	}
	if ebiten.IsKeyPressed(ebiten.KeyA) {
		cmd.side -= sideMove[speed]
	}
	if ebiten.IsKeyPressed(ebiten.KeyD) {
		cmd.side += sideMove[speed]
	}
	// Keep map panning on arrow keys; use Q/E turning in map mode.
	if ebiten.IsKeyPressed(ebiten.KeyQ) {
		cmd.turn += 1
	}
	if ebiten.IsKeyPressed(ebiten.KeyE) {
		cmd.turn -= 1
	}
	if g.edgeInputPass && g.pendingUse {
		usePressed = true
		g.pendingUse = false
	}
	fireHeld := ebiten.IsKeyPressed(ebiten.KeyControlLeft) || ebiten.IsKeyPressed(ebiten.KeyControlRight) || ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft)
	firePressed = fireHeld
	if g.opts.SourcePortMode && g.opts.MouseLook {
		mx, _ := ebiten.CursorPosition()
		if g.mouseLookSuppressTicks > 0 {
			g.mouseLookSuppressTicks--
		} else if g.mouseLookSet {
			dx := mx - g.lastMouseX
			cmd.turnRaw += g.mouseLookTurnRaw(dx)
		}
		g.lastMouseX = mx
		g.mouseLookSet = true
	} else {
		g.mouseLookSet = false
	}
	cmd.run = speed == 1
	g.runGameplayTic(cmd, usePressed, fireHeld)
	g.recordDemoTic(cmd, usePressed, firePressed)
	g.discoverLinesAroundPlayer()

	if g.followMode {
		g.camX = float64(g.p.x) / fracUnit
		g.camY = float64(g.p.y) / fracUnit
		return
	}

	panStep := 32.0 / g.zoom
	if ebiten.IsKeyPressed(ebiten.KeyArrowUp) {
		g.camY += panStep
	}
	if ebiten.IsKeyPressed(ebiten.KeyArrowDown) {
		g.camY -= panStep
	}
	if ebiten.IsKeyPressed(ebiten.KeyArrowLeft) {
		g.camX -= panStep
	}
	if ebiten.IsKeyPressed(ebiten.KeyArrowRight) {
		g.camX += panStep
	}
}

func (g *game) updateWalkMode() {
	g.updateParityControls()
	g.updateWeaponHotkeys(true)
	g.updateZoom()
	cmd := moveCmd{}
	usePressed := false
	firePressed := false
	speed := g.currentRunSpeed()
	strafeMod := ebiten.IsKeyPressed(ebiten.KeyAltLeft) || ebiten.IsKeyPressed(ebiten.KeyAltRight)
	if ebiten.IsKeyPressed(ebiten.KeyW) || ebiten.IsKeyPressed(ebiten.KeyArrowUp) {
		cmd.forward += forwardMove[speed]
	}
	if ebiten.IsKeyPressed(ebiten.KeyS) || ebiten.IsKeyPressed(ebiten.KeyArrowDown) {
		cmd.forward -= forwardMove[speed]
	}
	if ebiten.IsKeyPressed(ebiten.KeyA) {
		cmd.side -= sideMove[speed]
	}
	if ebiten.IsKeyPressed(ebiten.KeyD) {
		cmd.side += sideMove[speed]
	}
	if ebiten.IsKeyPressed(ebiten.KeyArrowLeft) {
		if strafeMod {
			cmd.side -= sideMove[speed]
		} else {
			cmd.turn += 1
		}
	}
	if ebiten.IsKeyPressed(ebiten.KeyArrowRight) {
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
	fireHeld := ebiten.IsKeyPressed(ebiten.KeyControlLeft) || ebiten.IsKeyPressed(ebiten.KeyControlRight) || ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft)
	firePressed = fireHeld

	if g.opts.MouseLook {
		mx, _ := ebiten.CursorPosition()
		if g.mouseLookSuppressTicks > 0 {
			g.mouseLookSuppressTicks--
		} else if g.mouseLookSet {
			dx := mx - g.lastMouseX
			// Keep vanilla-feeling turn quantization while using modern mouse-look default.
			cmd.turnRaw += g.mouseLookTurnRaw(dx)
		}
		g.lastMouseX = mx
		g.mouseLookSet = true
	} else {
		g.mouseLookSet = false
	}

	cmd.run = speed == 1
	g.runGameplayTic(cmd, usePressed, fireHeld)
	g.recordDemoTic(cmd, usePressed, firePressed)
	g.discoverLinesAroundPlayer()
	g.camX = float64(g.p.x) / fracUnit
	g.camY = float64(g.p.y) / fracUnit
}

func (g *game) mapRotationActive() bool {
	if g == nil || !g.rotateView {
		return false
	}
	if g.mode == viewMap && !g.followMode {
		return false
	}
	return true
}

func (g *game) currentRunSpeed() int {
	runHeld := ebiten.IsKeyPressed(ebiten.KeyShiftLeft) || ebiten.IsKeyPressed(ebiten.KeyShiftRight)
	runActive := g.alwaysRun
	if runHeld {
		runActive = !runActive
	}
	if runActive {
		return 1
	}
	return 0
}

func (g *game) recordDemoTic(cmd moveCmd, usePressed, firePressed bool) {
	if g.opts.DemoScript != nil || strings.TrimSpace(g.opts.RecordDemoPath) == "" {
		return
	}
	g.demoRecord = append(g.demoRecord, DemoTic{
		Forward: cmd.forward,
		Side:    cmd.side,
		Turn:    cmd.turn,
		TurnRaw: cmd.turnRaw,
		Run:     cmd.run,
		Use:     usePressed,
		Fire:    firePressed,
	})
}

func (g *game) updateWeaponHotkeys(allowCycleInput bool) {
	if !g.edgeInputPass {
		return
	}
	if inpututil.IsKeyJustPressed(ebiten.Key1) {
		g.selectWeaponSlot(1)
	}
	if inpututil.IsKeyJustPressed(ebiten.Key2) {
		g.selectWeaponSlot(2)
	}
	if inpututil.IsKeyJustPressed(ebiten.Key3) {
		g.selectWeaponSlot(3)
	}
	if inpututil.IsKeyJustPressed(ebiten.Key4) {
		g.selectWeaponSlot(4)
	}
	if inpututil.IsKeyJustPressed(ebiten.Key5) {
		g.selectWeaponSlot(5)
	}
	if inpututil.IsKeyJustPressed(ebiten.Key6) {
		g.selectWeaponSlot(6)
	}
	if inpututil.IsKeyJustPressed(ebiten.Key7) {
		g.selectWeaponSlot(7)
	}
	if !allowCycleInput {
		return
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyBracketRight) ||
		inpututil.IsKeyJustPressed(ebiten.KeyPageDown) ||
		inpututil.IsMouseButtonJustPressed(ebiten.MouseButton4) {
		g.cycleWeapon(1)
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyBracketLeft) ||
		inpututil.IsKeyJustPressed(ebiten.KeyPageUp) ||
		inpututil.IsMouseButtonJustPressed(ebiten.MouseButton3) {
		g.cycleWeapon(-1)
	}
	_, wheelY := ebiten.Wheel()
	if wheelY < 0 {
		g.cycleWeapon(1)
	}
	if wheelY > 0 {
		g.cycleWeapon(-1)
	}
}

func (g *game) updateParityControls() {
	if !g.edgeInputPass {
		return
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyCapsLock) {
		g.alwaysRun = !g.alwaysRun
		if g.alwaysRun {
			g.setHUDMessage("Always Run ON", 70)
		} else {
			g.setHUDMessage("Always Run OFF", 70)
		}
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyF12) {
		g.autoWeaponSwitch = !g.autoWeaponSwitch
		if g.autoWeaponSwitch {
			g.setHUDMessage("Auto Weapon Switch ON", 70)
		} else {
			g.setHUDMessage("Auto Weapon Switch OFF", 70)
		}
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyG) {
		g.showGrid = !g.showGrid
		if g.showGrid {
			g.setHUDMessage("Grid ON", 70)
		} else {
			g.setHUDMessage("Grid OFF", 70)
		}
	}
	if !g.opts.SourcePortMode {
		if inpututil.IsKeyJustPressed(ebiten.KeyF2) {
			g.setHUDMessage("Save menu not wired yet", 70)
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyF3) {
			g.setHUDMessage("Load menu not wired yet", 70)
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyF6) {
			g.setHUDMessage("Quicksave not wired yet", 70)
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyF7) {
			g.setHUDMessage("End game flow not wired yet", 70)
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyF8) {
			g.hudMessagesEnabled = !g.hudMessagesEnabled
			if g.hudMessagesEnabled {
				g.setHUDMessage("Messages ON", 70)
			} else {
				g.useText = "Messages OFF"
				g.useFlash = 70
			}
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyF9) {
			g.setHUDMessage("Quickload not wired yet", 70)
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyF11) {
			if !g.opts.KageShader {
				g.setHUDMessage("Gamma unavailable (-kage-shader off)", 70)
			} else if len(g.opts.DoomPaletteRGBA) != 256*4 {
				g.setHUDMessage("Gamma unavailable", 70)
			} else {
				g.gammaLevel = (g.gammaLevel + 1) % len(gammaTargets)
				g.setHUDMessage(fmt.Sprintf("Gamma %d [%.1f]", g.gammaLevel, gammaTargetForLevel(g.gammaLevel)), 70)
			}
		}
	}
	if g.opts.SourcePortMode {
		if inpututil.IsKeyJustPressed(ebiten.KeyO) {
			if g.parity.reveal == revealNormal {
				g.parity.reveal = revealAllMap
				g.setHUDMessage("Allmap ON", 70)
			} else {
				g.parity.reveal = revealNormal
				g.setHUDMessage("Allmap OFF", 70)
			}
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyI) {
			g.parity.iddt = (g.parity.iddt + 1) % 3
			g.setHUDMessage(fmt.Sprintf("IDDT %d", g.parity.iddt), 70)
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyL) {
			g.opts.LineColorMode = toggledLineColorMode(g.opts.LineColorMode)
			g.setHUDMessage(fmt.Sprintf("Line Colors: %s", g.opts.LineColorMode), 70)
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyV) {
			g.showLegend = !g.showLegend
			if g.showLegend {
				g.setHUDMessage("Thing Legend ON", 70)
			} else {
				g.setHUDMessage("Thing Legend OFF", 70)
			}
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyT) {
			g.opts.SourcePortThingRenderMode = cycleSourcePortThingRenderMode(g.opts.SourcePortThingRenderMode)
			g.setHUDMessage(fmt.Sprintf("Thing Render: %s", sourcePortThingRenderModeLabel(g.opts.SourcePortThingRenderMode)), 70)
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyY) {
			if !g.spriteClipDiag {
				g.spriteClipDiag = true
				g.spriteClipDiagOnly = false
				g.spriteClipDiagGreenOnly = false
				g.setHUDMessage("Sprite Clip Diag ON", 70)
			} else if !g.spriteClipDiagOnly {
				g.spriteClipDiagOnly = true
				g.spriteClipDiagGreenOnly = false
				g.setHUDMessage("Sprite Clip Diag DEBUG-ONLY", 70)
			} else if !g.spriteClipDiagGreenOnly {
				g.spriteClipDiagOnly = true
				g.spriteClipDiagGreenOnly = true
				g.setHUDMessage("Sprite Clip Diag GREEN-ONLY", 70)
			} else {
				g.spriteClipDiag = false
				g.spriteClipDiagOnly = false
				g.spriteClipDiagGreenOnly = false
				g.setHUDMessage("Sprite Clip Diag OFF", 70)
			}
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyF6) {
			if !g.opts.KageShader {
				g.setHUDMessage("Kage shader disabled (-kage-shader)", 70)
				return
			}
			if len(g.opts.DoomPaletteRGBA) != 256*4 {
				g.setHUDMessage("Palette LUT unavailable", 70)
				return
			}
			g.paletteLUTEnabled = !g.paletteLUTEnabled
			if g.paletteLUTEnabled {
				g.setHUDMessage("Palette LUT ON", 70)
			} else {
				g.setHUDMessage("Palette LUT OFF", 70)
			}
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyF7) {
			if !g.opts.KageShader {
				g.setHUDMessage("Kage shader disabled (-kage-shader)", 70)
				return
			}
			if len(g.opts.DoomPaletteRGBA) != 256*4 {
				g.setHUDMessage("Gamma unavailable", 70)
				return
			}
			g.gammaLevel = (g.gammaLevel + 1) % len(gammaTargets)
			g.setHUDMessage(fmt.Sprintf("Gamma %d [%.1f]", g.gammaLevel, gammaTargetForLevel(g.gammaLevel)), 70)
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyF8) {
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

func (g *game) updateZoom() {
	zoomStep := 1.03
	if ebiten.IsKeyPressed(ebiten.KeyEqual) || ebiten.IsKeyPressed(ebiten.KeyKPAdd) {
		g.zoom *= zoomStep
	}
	if ebiten.IsKeyPressed(ebiten.KeyMinus) || ebiten.IsKeyPressed(ebiten.KeyKPSubtract) {
		g.zoom /= zoomStep
	}
	_, wheelY := ebiten.Wheel()
	if wheelY > 0 {
		g.zoom *= 1.1
	}
	if wheelY < 0 {
		g.zoom /= 1.1
	}
	if g.zoom < g.fitZoom*0.05 {
		g.zoom = g.fitZoom * 0.05
	}
	if g.zoom > g.fitZoom*200 {
		g.zoom = g.fitZoom * 200
	}
}

func (g *game) Draw(screen *ebiten.Image) {
	drawStart := time.Now()
	if g.opts.DemoScript != nil {
		g.demoBenchDraws++
	}
	g.frameUpload = 0
	g.perfInDraw = true
	defer func() { g.perfInDraw = false }()
	defer g.finishPerfCounter(drawStart)
	screen.Fill(bgColor)
	if g.mode != viewMap {
		debugPos := fmt.Sprintf(
			"pos=(%.2f, %.2f) ang=%.1f",
			float64(g.p.x)/fracUnit,
			float64(g.p.y)/fracUnit,
			normalizeDeg360(float64(g.p.angle)*360.0/4294967296.0),
		)
		aimSS := g.debugAimSS
		g.prepareRenderState()
		g.drawDoomBasic3D(screen)
		if g.opts.Debug {
			ebitenutil.DebugPrintAt(screen, fmt.Sprintf("profile=%s", g.profileLabel()), 12, 28)
			if g.opts.SourcePortMode {
				ebitenutil.DebugPrintAt(screen, "renderer=doom-basic | TAB automap", 12, 12)
				ebitenutil.DebugPrintAt(screen, "TAB automap | J planes | Y clipdiag | F1 help", 12, 44)
			} else {
				ebitenutil.DebugPrintAt(screen, "renderer=doom-basic | TAB automap", 12, 12)
				ebitenutil.DebugPrintAt(screen, "TAB automap | F5 detail | F1 help", 12, 44)
			}
			planes3DOn := len(g.opts.FlatBank) > 0
			ebitenutil.DebugPrintAt(screen, fmt.Sprintf("planes3d=%t flats=%d detail=%dx%d", planes3DOn, len(g.opts.FlatBank), g.viewW, g.viewH), 12, 60)
			ebitenutil.DebugPrintAt(screen, debugPos, 12, 76)
			ebitenutil.DebugPrintAt(screen, fmt.Sprintf("ss=%d", aimSS), 12, 92)
			if g.spriteClipDiag {
				mode := "ON"
				if g.spriteClipDiagOnly {
					mode = "DEBUG-ONLY"
				}
				if g.spriteClipDiagGreenOnly {
					mode = "GREEN-ONLY"
				}
				ebitenutil.DebugPrintAt(screen, fmt.Sprintf("sprite-clip-diag=%s (Y cycle)", mode), 12, 108)
			}
		}
		if g.opts.SourcePortMode && g.spriteClipDiagOnly {
			screen.Fill(bgColor)
			g.drawSpriteClipDiagOverlay(screen)
			if !g.opts.NoFPS {
				g.drawPerfOverlay(screen)
			}
			return
		}
		if g.opts.SourcePortMode && g.spriteClipDiag {
			g.drawSpriteClipDiagOverlay(screen)
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
		if g.paused {
			g.drawPauseOverlay(screen)
		}
		if !g.opts.NoFPS {
			g.drawPerfOverlay(screen)
		}
		return
	}
	g.prepareRenderState()
	if g.opts.SourcePortMode && len(g.opts.FlatBank) > 0 {
		g.drawMapFloorTextures2D(screen)
	}
	if g.showGrid {
		g.drawGrid(screen)
	}

	g.drawMapLines(screen)
	if g.opts.SourcePortMode {
		g.drawUseSpecialLines(screen)
	}
	if g.opts.SourcePortMode {
		g.drawUseTargetHighlight(screen)
	}

	if shouldDrawThings(g.parity) {
		g.drawThings(screen)
	}
	g.drawMarks(screen)
	g.drawPlayer(screen)
	g.drawPeerPlayers(screen)

	modeText := "MAP"
	if g.mode == viewWalk {
		modeText = "WALK"
	}
	revealText := "normal"
	if g.parity.reveal == revealAllMap {
		revealText = "allmap"
	}
	if g.opts.SourcePortMode {
		overlay := fmt.Sprintf("map=%s mode=%s skill=%d zoom=%.2f reveal=%s iddt=%d grid=%t marks=%d colors=%s",
			g.m.Name,
			modeText,
			g.opts.SkillLevel,
			g.zoom,
			revealText,
			g.parity.iddt,
			g.showGrid,
			len(g.marks),
			g.opts.LineColorMode,
		)
		ebitenutil.DebugPrintAt(screen, overlay, 12, 12)
		stats := fmt.Sprintf("hp=%d ar=%d am=%d sh=%d ro=%d ce=%d keys=%s wp=%s",
			g.stats.Health,
			g.stats.Armor,
			g.stats.Bullets,
			g.stats.Shells,
			g.stats.Rockets,
			g.stats.Cells,
			g.inventory.keySummary(),
			weaponName(g.inventory.ReadyWeapon),
		)
		ebitenutil.DebugPrintAt(screen, stats, 12, 28)
		cheat := fmt.Sprintf("cheat=%d invuln=%t", g.cheatLevel, g.invulnerable)
		ebitenutil.DebugPrintAt(screen, cheat, 12, 60)
		floor2D := fmt.Sprintf("floor2d=textured %s", g.mapFloorWorldState)
		ebitenutil.DebugPrintAt(screen, floor2D, 12, 76)
		thingRender := fmt.Sprintf("things=%s", strings.ToLower(sourcePortThingRenderModeLabel(g.opts.SourcePortThingRenderMode)))
		ebitenutil.DebugPrintAt(screen, thingRender, 12, 92)
		if g.showLegend {
			g.drawThingLegend(screen)
		}
	}
	if g.useFlash > 0 {
		g.drawHUDMessage(screen, g.useText, 0, 0)
	}
	if g.isDead {
		g.drawDeathOverlay(screen)
	}
	g.drawFlashOverlay(screen)
	if g.paused {
		g.drawPauseOverlay(screen)
	}
	if !g.opts.NoFPS {
		g.drawPerfOverlay(screen)
	}
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
	pauseMenuModeEpisode
	pauseMenuModeSkill
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
		g.pauseMenuEpisodeOn = 0
		g.pauseMenuSkillOn = max(0, normalizeSkillLevel(g.opts.SkillLevel)-1)
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
	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		if g.pauseMenuMode != pauseMenuModeRoot {
			g.pauseMenuMode = pauseMenuModeRoot
			return
		}
		g.togglePauseMenu()
		return
	}
	switch g.pauseMenuMode {
	case pauseMenuModeOptions:
		if inpututil.IsKeyJustPressed(ebiten.KeyArrowUp) {
			g.pauseMenuOptionsOn = frontendNextSelectableOptionRow(g.pauseMenuOptionsOn, -1)
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyArrowDown) {
			g.pauseMenuOptionsOn = frontendNextSelectableOptionRow(g.pauseMenuOptionsOn, 1)
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyArrowLeft) {
			g.adjustPauseOption(-1)
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyArrowRight) {
			g.adjustPauseOption(1)
		}
	case pauseMenuModeSound:
		if inpututil.IsKeyJustPressed(ebiten.KeyArrowUp) || inpututil.IsKeyJustPressed(ebiten.KeyArrowDown) {
			g.pauseMenuSoundOn ^= 1
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyArrowLeft) {
			g.adjustPauseSound(-1)
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyArrowRight) {
			g.adjustPauseSound(1)
		}
	case pauseMenuModeEpisode:
		if n := len(g.availableEpisodeChoices()); n > 0 {
			if inpututil.IsKeyJustPressed(ebiten.KeyArrowUp) {
				g.pauseMenuEpisodeOn = (g.pauseMenuEpisodeOn + n - 1) % n
			}
			if inpututil.IsKeyJustPressed(ebiten.KeyArrowDown) {
				g.pauseMenuEpisodeOn = (g.pauseMenuEpisodeOn + 1) % n
			}
		}
	case pauseMenuModeSkill:
		if inpututil.IsKeyJustPressed(ebiten.KeyArrowUp) {
			g.pauseMenuSkillOn = (g.pauseMenuSkillOn + len(frontendSkillMenuNames) - 1) % len(frontendSkillMenuNames)
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyArrowDown) {
			g.pauseMenuSkillOn = (g.pauseMenuSkillOn + 1) % len(frontendSkillMenuNames)
		}
	default:
		if inpututil.IsKeyJustPressed(ebiten.KeyArrowUp) {
			g.pauseMenuItemOn = (g.pauseMenuItemOn + len(inGamePauseMenuNames) - 1) % len(inGamePauseMenuNames)
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyArrowDown) {
			g.pauseMenuItemOn = (g.pauseMenuItemOn + 1) % len(inGamePauseMenuNames)
		}
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyEnter) || inpututil.IsKeyJustPressed(ebiten.KeyKPEnter) {
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
		g.pauseMenuMode = pauseMenuModeOptions
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
	case 2:
		if g.opts.SourcePortMode {
			g.cycleSourcePortDetailLevel()
		} else {
			g.cycleDetailLevel()
		}
	case 5:
		next := frontendNextMouseSensitivity(g.opts.MouseLookSpeed, dir)
		if next != g.opts.MouseLookSpeed {
			g.opts.MouseLookSpeed = next
			g.publishRuntimeSettingsIfChanged()
		}
	}
}

func (g *game) adjustPauseSound(dir int) {
	if g == nil || dir == 0 {
		return
	}
	switch g.pauseMenuSoundOn {
	case 0:
		next := clampVolume(g.opts.SFXVolume + float64(dir)*(1.0/15.0))
		if next != g.opts.SFXVolume {
			g.opts.SFXVolume = next
			if g.snd != nil {
				g.snd.setSFXVolume(next)
			}
			g.publishRuntimeSettingsIfChanged()
		}
	default:
		next := clampVolume(g.opts.MusicVolume + float64(dir)*(1.0/15.0))
		if next != g.opts.MusicVolume {
			g.opts.MusicVolume = next
			g.publishRuntimeSettingsIfChanged()
		}
	}
}

func (g *game) activatePauseOptionsItem() {
	if g == nil {
		return
	}
	switch g.pauseMenuOptionsOn {
	case 0:
		g.pauseMenuStatus = "END GAME NOT WIRED YET"
		g.pauseMenuStatusTics = doomTicsPerSecond
	case 1:
		g.hudMessagesEnabled = !g.hudMessagesEnabled
		g.publishRuntimeSettingsIfChanged()
	case 2:
		g.adjustPauseOption(1)
	case 5:
		g.adjustPauseOption(1)
	case 7:
		g.pauseMenuMode = pauseMenuModeSound
	}
}

func (g *game) profileLabel() string {
	if g.opts.SourcePortMode {
		return "sourceport"
	}
	return "doom"
}

func (g *game) emitSoundEvent(ev soundEvent) {
	g.soundQueue = append(g.soundQueue, ev)
	g.soundQueueOrigin = append(g.soundQueueOrigin, queuedSoundOrigin{})
}

func (g *game) emitSoundEventAt(ev soundEvent, x, y int64) {
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
				g.m.Sidedefs[s.sidedef].Top = s.top
				g.m.Sidedefs[s.sidedef].Bottom = s.bottom
				g.m.Sidedefs[s.sidedef].Mid = s.mid
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
		if !thingSpawnsInSession(th, g.opts.SkillLevel, g.opts.GameMode, g.opts.ShowNoSkillItems, g.opts.ShowAllItems) {
			g.thingCollected[i] = true
		}
	}
}

func (g *game) flushSoundEvents() {
	if g.snd != nil {
		for i, ev := range g.soundQueue {
			origin := queuedSoundOrigin{}
			if i >= 0 && i < len(g.soundQueueOrigin) {
				origin = g.soundQueueOrigin[i]
			}
			g.snd.playEventSpatial(ev, origin, g.p.x, g.p.y, g.p.angle, soundMapUsesFullClip(g.m.Name))
		}
		g.snd.tick()
	}
	g.soundQueue = g.soundQueue[:0]
	g.soundQueueOrigin = g.soundQueueOrigin[:0]
}

func shouldDrawThings(st automapParityState) bool {
	return st.iddt >= 2
}

func toggledLineColorMode(mode string) string {
	if mode == "parity" {
		return "doom"
	}
	return "parity"
}

func (g *game) mapVectorAntiAlias() bool {
	// Faithful mode targets Doom-like crisp map vectors.
	return g.opts.SourcePortMode
}

func (g *game) drawThingLegend(screen *ebiten.Image) {
	type legendEntry struct {
		label string
		style thingStyle
	}
	entries := []legendEntry{
		{label: "player starts", style: thingStyle{glyph: thingGlyphSquare, clr: thingPlayerColor}},
		{label: "monsters", style: thingStyle{glyph: thingGlyphTriangle, clr: thingMonsterColor}},
		{label: "items/pickups", style: thingStyle{glyph: thingGlyphDiamond, clr: thingItemColor}},
		{label: "keys", style: thingStyle{glyph: thingGlyphStar, clr: thingKeyBlue}},
		{label: "misc", style: thingStyle{glyph: thingGlyphCross, clr: thingMiscColor}},
	}
	if g.opts.SourcePortMode {
		entries = append(entries, legendEntry{
			label: fmt.Sprintf("render: %s", strings.ToLower(sourcePortThingRenderModeLabel(g.opts.SourcePortThingRenderMode))),
			style: thingStyle{glyph: thingGlyphCross, clr: thingMiscColor},
		})
	}
	type lineLegendEntry struct {
		label string
		clr   color.Color
	}
	lineEntries := []lineLegendEntry{
		{label: "one-sided wall", clr: wallOneSided},
		{label: "floor delta", clr: wallFloorChange},
		{label: "ceiling delta", clr: wallCeilChange},
		{label: "teleporter", clr: wallTeleporter},
		{label: "use switch/button", clr: wallUseSpecial},
	}
	if g.opts.LineColorMode == "parity" {
		lineEntries = append(lineEntries, lineLegendEntry{label: "unrevealed (allmap)", clr: wallUnrevealed})
	}

	maxLen := len("THING LEGEND")
	for _, e := range entries {
		if len(e.label) > maxLen {
			maxLen = len(e.label)
		}
	}
	if len("LINE COLORS") > maxLen {
		maxLen = len("LINE COLORS")
	}
	for _, e := range lineEntries {
		if len(e.label) > maxLen {
			maxLen = len(e.label)
		}
	}
	x := g.viewW - maxLen*7 - 36
	if x < 10 {
		x = 10
	}
	y := 28
	aa := g.mapVectorAntiAlias()
	ebitenutil.DebugPrintAt(screen, "THING LEGEND", x, y)
	for i, e := range entries {
		ly := y + 16 + i*14
		drawThingGlyph(screen, e.style, float64(x+8), float64(ly+5), 0, 4.6, aa)
		ebitenutil.DebugPrintAt(screen, e.label, x+18, ly)
	}

	ly0 := y + 16 + len(entries)*14 + 8
	ebitenutil.DebugPrintAt(screen, "LINE COLORS", x, ly0)
	for i, e := range lineEntries {
		ly := ly0 + 16 + i*14
		vector.StrokeLine(screen, float32(x+2), float32(ly+5), float32(x+14), float32(ly+5), 2.4, e.clr, aa)
		ebitenutil.DebugPrintAt(screen, e.label, x+18, ly)
	}
}

func (g *game) addMark() {
	if len(g.marks) >= 10 {
		g.setHUDMessage("Marks Full", 70)
		return
	}
	id := g.nextMarkID
	g.marks = append(g.marks, mapMark{
		id: g.nextMarkID,
		x:  g.camX,
		y:  g.camY,
	})
	g.nextMarkID++
	g.setHUDMessage(fmt.Sprintf("Marked Spot %d", id), 70)
}

func (g *game) clearMarks() {
	g.marks = g.marks[:0]
	g.setHUDMessage("Marks Cleared", 70)
}

func (g *game) toggleBigMap() {
	if !g.bigMap {
		g.savedView = savedMapView{
			camX:   g.camX,
			camY:   g.camY,
			zoom:   g.zoom,
			follow: g.followMode,
			valid:  true,
		}
		g.bigMap = true
		g.followMode = false
		g.camX = (g.bounds.minX + g.bounds.maxX) / 2
		g.camY = (g.bounds.minY + g.bounds.maxY) / 2
		g.zoom = g.fitZoom
		g.setHUDMessage("Big Map ON", 70)
		return
	}
	g.bigMap = false
	if g.savedView.valid {
		g.camX = g.savedView.camX
		g.camY = g.savedView.camY
		g.zoom = g.savedView.zoom
		g.followMode = g.savedView.follow
	}
	g.setHUDMessage("Big Map OFF", 70)
}

func (g *game) drawGrid(screen *ebiten.Image) {
	const cell = 128.0
	left := g.renderCamX - float64(g.viewW)/(2*g.zoom)
	right := g.renderCamX + float64(g.viewW)/(2*g.zoom)
	bottom := g.renderCamY - float64(g.viewH)/(2*g.zoom)
	top := g.renderCamY + float64(g.viewH)/(2*g.zoom)
	grid := color.RGBA{R: 40, G: 50, B: 60, A: 255}
	aa := g.mapVectorAntiAlias()

	startX := math.Floor(left/cell) * cell
	for x := startX; x <= right; x += cell {
		x1, y1 := g.worldToScreen(x, bottom)
		x2, y2 := g.worldToScreen(x, top)
		vector.StrokeLine(screen, float32(x1), float32(y1), float32(x2), float32(y2), 1, grid, aa)
	}
	startY := math.Floor(bottom/cell) * cell
	for y := startY; y <= top; y += cell {
		x1, y1 := g.worldToScreen(left, y)
		x2, y2 := g.worldToScreen(right, y)
		vector.StrokeLine(screen, float32(x1), float32(y1), float32(x2), float32(y2), 1, grid, aa)
	}
}

func (g *game) drawThings(screen *ebiten.Image) {
	aa := g.mapVectorAntiAlias()
	for i, th := range g.m.Things {
		if i >= 0 && i < len(g.thingCollected) && g.thingCollected[i] {
			continue
		}
		fx, fy := g.thingPosFixed(i, th)
		x := float64(fx) / fracUnit
		y := float64(fy) / fracUnit
		sx, sy := g.worldToScreen(x, y)
		if g.drawMapThingSprite(screen, i, th, sx, sy) {
			continue
		}
		size := thingGlyphSize(g.zoom)
		angle := worldThingAngle(th.Angle)
		if g.mapRotationActive() {
			angle = relativeThingAngle(th.Angle, g.renderAngle)
		}
		drawThingGlyph(screen, styleForThing(th), sx, sy, angle, size, aa)
	}
}

func (g *game) shouldDrawMapThingSprite(th mapdata.Thing) bool {
	if g == nil || !g.opts.SourcePortMode {
		return false
	}
	switch sourcePortThingRenderMode(normalizeSourcePortThingRenderMode(g.opts.SourcePortThingRenderMode, g.opts.SourcePortMode)) {
	case sourcePortThingRenderItems:
		return isItemOrPickup(th.Type)
	case sourcePortThingRenderSprites:
		return true
	default:
		return false
	}
}

func (g *game) drawMapThingSprite(screen *ebiten.Image, thingIdx int, th mapdata.Thing, sx, sy float64) bool {
	if !g.shouldDrawMapThingSprite(th) {
		return false
	}
	name := g.mapThingSpriteName(thingIdx, th)
	if name == "" {
		return false
	}
	img, w, h, _, _, ok := g.spritePatch(name)
	if !ok || w <= 0 || h <= 0 {
		return false
	}
	target := thingGlyphSize(g.zoom) * 2.4
	if target < 6 {
		target = 6
	}
	scale := math.Min(target/float64(w), target/float64(h))
	if scale <= 0 {
		return false
	}
	op := &ebiten.DrawImageOptions{}
	op.Filter = ebiten.FilterNearest
	op.GeoM.Scale(scale, scale)
	op.GeoM.Translate(sx-float64(w)*scale*0.5, sy-float64(h)*scale*0.5)
	screen.DrawImage(img, op)
	return true
}

func (g *game) mapThingSpriteName(thingIdx int, th mapdata.Thing) string {
	if g == nil {
		return ""
	}
	if isPlayerStart(th.Type) {
		return "PLAYN0"
	}
	if isMonster(th.Type) {
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
		cf := g.textureAnimCrossfadeFrames
		g.textureAnimCrossfadeFrames = 0
		name := g.worldThingSpriteName(th.Type, g.worldTic)
		g.textureAnimCrossfadeFrames = cf
		return name
	}
	animTickUnits, animUnitsPerTic := g.worldThingAnimTickUnits()
	return g.worldThingSpriteNameScaled(th.Type, animTickUnits, animUnitsPerTic)
}

func thingGlyphSize(zoom float64) float64 {
	// Doom-like behavior: thing markers scale with map zoom (map-space vectors).
	const worldHalfUnits = 16.0
	s := worldHalfUnits * zoom
	if s < 1.5 {
		return 1.5
	}
	if s > 40 {
		return 40
	}
	return s
}

func (g *game) drawMarks(screen *ebiten.Image) {
	mc := color.RGBA{R: 120, G: 210, B: 255, A: 255}
	aa := g.mapVectorAntiAlias()
	for _, mk := range g.marks {
		sx, sy := g.worldToScreen(mk.x, mk.y)
		r := 5.0
		vector.StrokeLine(screen, float32(sx-r), float32(sy-r), float32(sx+r), float32(sy+r), 1.3, mc, aa)
		vector.StrokeLine(screen, float32(sx-r), float32(sy+r), float32(sx+r), float32(sy-r), 1.3, mc, aa)
		ebitenutil.DebugPrintAt(screen, fmt.Sprintf("%d", mk.id), int(sx)+6, int(sy)-6)
	}
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
	scale := g.zoom
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
	vector.StrokeLine(screen, float32(x1), float32(y1), float32(x2), float32(y2), 3.0, useTargetColor, g.mapVectorAntiAlias())
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

	wallTop, wallBottom, ceilingClip, floorClip := g.ensure3DFrameBuffers()
	planesEnabled := len(g.opts.FlatBank) > 0
	planeOrder := g.beginPlane3DFrame(g.viewW)
	solid := g.beginSolid3DFrame()
	prepass := g.buildWallSegPrepassParallel(g.visibleSegIndicesPseudo3D(), camX, camY, ca, sa, focal, near)
	maskedMids := g.ensureMaskedMidSegScratch(len(prepass))
	for _, pp := range prepass {
		si := pp.segIdx
		if si < 0 || si >= len(g.m.Segs) {
			continue
		}
		if !pp.ok {
			if pp.logReason != "" {
				g.logWallCull(si, pp.logReason, pp.logZ1, pp.logZ2, pp.logX1, pp.logX2)
			}
			continue
		}
		d := g.linedefDecisionPseudo3D(pp.ld)
		base, _ := g.decisionStyle(d)
		baseRGBA := color.RGBAModel.Convert(base).(color.RGBA)
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
		ws := classifyWallPortal(front, back, eyeZ, frontFloor, frontCeil, backFloor, backCeil)
		worldTop := ws.worldTop
		worldBottom := ws.worldBottom
		worldHigh := ws.worldHigh
		worldLow := ws.worldLow
		topWall := ws.topWall
		bottomWall := ws.bottomWall
		markCeiling := ws.markCeiling
		markFloor := ws.markFloor
		solidWall := ws.solidWall
		if solidWall && g.wallSpanRejectEnabled() && solidFullyCoveredFast(solid, pp.minSX, pp.maxSX) {
			g.logWallCull(si, "OCCLUDED", pp.logZ1, pp.logZ2, pp.logX1, pp.logX2)
			continue
		}
		var midTex WallTexture
		var topTex WallTexture
		var botTex WallTexture
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
			midTex, hasMidTex = g.wallTexture(frontSideDef.Mid)
			if hasMidTex {
				if (ld.Flags & mlDontPegBottom) != 0 {
					midTexMid = frontFloor + float64(midTex.Height) - eyeZ
				} else {
					midTexMid = frontCeil - eyeZ
				}
				midTexMid += rowOffset
			}
			if topWall {
				topTex, hasTopTex = g.wallTexture(frontSideDef.Top)
				if hasTopTex {
					if (ld.Flags & mlDontPegTop) != 0 {
						topTexMid = frontCeil - eyeZ
					} else if back != nil {
						topTexMid = backCeil + float64(topTex.Height) - eyeZ
					} else {
						topTexMid = frontCeil - eyeZ
					}
					topTexMid += rowOffset
				}
			}
			if bottomWall {
				botTex, hasBotTex = g.wallTexture(frontSideDef.Bottom)
				if hasBotTex {
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
			floorPlane, created = g.ensurePlane3DForRangeCached(g.plane3DKeyForSector(front, true), pp.minSX, pp.maxSX, g.viewW)
			if created && floorPlane != nil {
				planeOrder = append(planeOrder, floorPlane)
			}
			ceilPlane, created = g.ensurePlane3DForRangeCached(g.plane3DKeyForSector(front, false), pp.minSX, pp.maxSX, g.viewW)
			if created && ceilPlane != nil {
				planeOrder = append(planeOrder, ceilPlane)
			}
		}

		visibleRanges := g.solidClipScratch[:0]
		if solidWall && g.wallSpanClipEnabled() {
			visibleRanges = clipRangeAgainstSolidSpans(pp.minSX, pp.maxSX, solid, visibleRanges)
		} else {
			visibleRanges = append(visibleRanges, solidSpan{l: pp.minSX, r: pp.maxSX})
		}
		g.solidClipScratch = visibleRanges
		if len(visibleRanges) == 0 {
			g.logWallCull(si, "OCCLUDED", pp.logZ1, pp.logZ2, pp.logX1, pp.logX2)
			continue
		}
		if solidWall && g.wallSliceOcclusionEnabled() && !g.depthOcclusionEnabled() {
			allOcc := true
			for _, vis := range visibleRanges {
				visOcc := g.wallSliceRangeTriFullyOccludedByWallsOnly(pp, vis.l, vis.r, worldTop, worldBottom, focal)
				if !visOcc {
					allOcc = false
					break
				}
			}
			if allOcc {
				g.logWallCull(si, "OCCLUDED", pp.logZ1, pp.logZ2, pp.logX1, pp.logX2)
				continue
			}
		}
		for _, vis := range visibleRanges {
			for x := vis.l; x <= vis.r; x++ {
				t := (float64(x) - pp.sx1) / (pp.sx2 - pp.sx1)
				if t < 0 {
					t = 0
				}
				if t > 1 {
					t = 1
				}
				invF := pp.invF1 + (pp.invF2-pp.invF1)*t
				if invF <= 0 {
					continue
				}
				f := 1.0 / invF
				if f <= 0 {
					continue
				}
				texU := (pp.uOverF1 + (pp.uOverF2-pp.uOverF1)*t) * f
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
					useTex := hasMidTex
					// Closed two-sided doors often have upper/lower textures but no middle texture.
					if back != nil && !useTex {
						if topWall && hasTopTex {
							tex = topTex
							texMid = topTexMid
							useTex = true
						} else if bottomWall && hasBotTex {
							tex = botTex
							texMid = botTexMid
							useTex = true
						}
					}
					g.drawBasicWallColumn(wallTop, wallBottom, x, yl, yh, f, front.Light, wallLightBias, baseRGBA, texU, texMid, focal, tex, useTex)
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
						g.drawBasicWallColumn(wallTop, wallBottom, x, yl, mid, f, front.Light, wallLightBias, baseRGBA, texU, topTexMid, focal, topTex, hasTopTex)
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
						g.drawBasicWallColumn(wallTop, wallBottom, x, mid, yh, f, front.Light, wallLightBias, baseRGBA, texU, botTexMid, focal, botTex, hasBotTex)
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
			for _, vis := range visibleRanges {
				if vis.l > vis.r {
					continue
				}
				dist := 0.0
				if pp.invF1+pp.invF2 > 0 {
					dist = 2.0 / (pp.invF1 + pp.invF2)
				}
				maskedMids = append(maskedMids, maskedMidSeg{
					dist:      dist,
					x0:        vis.l,
					x1:        vis.r,
					sx1:       pp.sx1,
					sx2:       pp.sx2,
					invF1:     pp.invF1,
					invF2:     pp.invF2,
					uOverF1:   pp.uOverF1,
					uOverF2:   pp.uOverF2,
					worldHigh: worldHigh,
					worldLow:  worldLow,
					texUOff:   texUOffset,
					texMid:    midTexMid,
					tex:       midTex,
					light:     front.Light,
					lightBias: wallLightBias,
				})
			}
		}

		if solidWall {
			solid = addSolidSpan(solid, pp.minSX, pp.maxSX)
		}
	}
	g.maskedMidSegsScratch = maskedMids
	g.solid3DBuf = solid
	usedSkyLayer := false
	if planesEnabled && hasMarkedPlane3DData(planeOrder) {
		usedSkyLayer = g.drawDoomBasicTexturedPlanesVisplanePass(g.wallPix, camX, camY, ca, sa, eyeZ, focal, ceilClr, floorClr, planeOrder)
	}
	g.drawMaskedMidSegs(focal)
	if !g.depthOcclusionEnabled() {
		g.buildMaskedMidClipColumns(focal)
		g.billboardQueueCollect = true
		g.billboardQueueScratch = g.billboardQueueScratch[:0]
		g.drawBillboardProjectilesToBuffer(camX, camY, camAng, focal, near)
		g.drawBillboardMonstersToBuffer(camX, camY, camAng, focal, near)
		g.drawBillboardWorldThingsToBuffer(camX, camY, camAng, focal, near)
		g.drawHitscanPuffsToBuffer(camX, camY, camAng, focal, near)
		g.billboardQueueCollect = false
		sort.Slice(g.billboardQueueScratch, func(i, j int) bool {
			return g.billboardQueueScratch[i].dist > g.billboardQueueScratch[j].dist
		})
		for _, qi := range g.billboardQueueScratch {
			g.billboardReplayActive = true
			g.billboardReplayKind = qi.kind
			g.billboardReplayIndex = qi.idx
			switch qi.kind {
			case billboardQueueProjectiles:
				g.drawBillboardProjectilesToBuffer(camX, camY, camAng, focal, near)
			case billboardQueueMonsters:
				g.drawBillboardMonstersToBuffer(camX, camY, camAng, focal, near)
			case billboardQueueWorldThings:
				g.drawBillboardWorldThingsToBuffer(camX, camY, camAng, focal, near)
			case billboardQueuePuffs:
				g.drawHitscanPuffsToBuffer(camX, camY, camAng, focal, near)
			}
		}
		g.billboardReplayActive = false
		g.billboardQueueScratch = g.billboardQueueScratch[:0]
	} else {
		g.drawBillboardProjectilesToBuffer(camX, camY, camAng, focal, near)
		g.drawBillboardMonstersToBuffer(camX, camY, camAng, focal, near)
		g.drawBillboardWorldThingsToBuffer(camX, camY, camAng, focal, near)
		g.drawHitscanPuffsToBuffer(camX, camY, camAng, focal, near)
	}
	if g.lowDetailMode() {
		g.duplicateLowDetailColumns()
	}
	if usedSkyLayer {
		g.drawSkyLayerFrame(screen)
	}
	g.writePixelsTimed(g.wallLayer, g.wallPix)
	screen.DrawImage(g.wallLayer, nil)
}

func classifyWallPortal(front, back *mapdata.Sector, eyeZ, frontFloor, frontCeil, backFloor, backCeil float64) wallPortalState {
	if front == nil {
		return wallPortalState{}
	}
	s := wallPortalState{
		worldTop:    frontCeil - eyeZ,
		worldBottom: frontFloor - eyeZ,
		markCeiling: true,
		markFloor:   true,
		solidWall:   back == nil,
	}
	s.worldHigh = s.worldTop
	s.worldLow = s.worldBottom

	if back != nil {
		s.worldHigh = backCeil - eyeZ
		s.worldLow = backFloor - eyeZ
		skyPortal := isSkyFlatName(front.CeilingPic) && isSkyFlatName(back.CeilingPic)
		if skyPortal {
			// Doom sky hack: keep upper portal open when both sides are sky.
			s.worldTop = s.worldHigh
		}
		lightDiff := back.Light != front.Light && doomSectorLighting
		s.markFloor = s.worldLow != s.worldBottom ||
			normalizeFlatName(back.FloorPic) != normalizeFlatName(front.FloorPic) ||
			lightDiff
		s.markCeiling = s.worldHigh != s.worldTop ||
			normalizeFlatName(back.CeilingPic) != normalizeFlatName(front.CeilingPic) ||
			lightDiff
		if skyPortal && backCeil != frontCeil {
			// Keep sky-marking active so the portal reliably masks farther geometry.
			s.markCeiling = true
		}
		// Portal solidity should follow the current tic state, not the render
		// look-ahead height, or doors can close floor visibility a fraction of a
		// tic early and wipe the whole floor behind them.
		if float64(back.CeilingHeight) <= float64(front.FloorHeight) ||
			float64(back.FloorHeight) >= float64(front.CeilingHeight) {
			s.markFloor = true
			s.markCeiling = true
			s.solidWall = true
		}
		s.topWall = s.worldHigh < s.worldTop
		s.bottomWall = s.worldLow > s.worldBottom
	}

	if frontFloor >= eyeZ {
		s.markFloor = false
	}
	if frontCeil <= eyeZ && !isSkyFlatName(front.CeilingPic) {
		s.markCeiling = false
	}
	return s
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

func (g *game) drawDepthBufferView() {
	n := g.viewW * g.viewH
	if n <= 0 || len(g.wallPix32) != n {
		return
	}
	stamp := g.depthFrameStamp
	var maxQ uint16
	for i := 0; i < n; i++ {
		d, ok := g.depthQAtStamped(i, stamp)
		if !ok {
			continue
		}
		if d > maxQ {
			maxQ = d
		}
	}
	if maxQ == 0 {
		clear(g.wallPix32)
		return
	}
	invMax := 1.0 / float64(maxQ)
	for i := 0; i < n; i++ {
		d, ok := g.depthQAtStamped(i, stamp)
		if !ok {
			g.wallPix32[i] = packRGBA(0, 0, 0)
			continue
		}
		norm := float64(d) * invMax
		if norm < 0 {
			norm = 0
		}
		if norm > 1 {
			norm = 1
		}
		// Near is bright, far is dark.
		v := uint8((1.0 - norm) * 255.0)
		g.wallPix32[i] = packRGBA(v, v, v)
	}
}

func (g *game) drawSpriteClipDiagOverlay(screen *ebiten.Image) {
	if g == nil || screen == nil || g.viewW <= 0 || g.viewH <= 0 {
		return
	}
	visibleClr := color.RGBA{R: 72, G: 245, B: 96, A: 255}
	occludedClr := color.RGBA{R: 255, G: 64, B: 64, A: 255}
	type edge2 struct {
		x0 float64
		y0 float64
		x1 float64
		y1 float64
		// owner is the wall triangle index that produced this edge.
		owner int
		pair  int
	}
	triEdges := make([]edge2, 0, 2048)
	inViewF := func(x, y float64) bool {
		return x >= 0 && x < float64(g.viewW) && y >= 0 && y < float64(g.viewH)
	}
	spriteOccAtFloat := func(x, y float64, depthQ uint16) bool {
		if !inViewF(x, y) {
			return false
		}
		x0 := int(math.Floor(x))
		y0 := int(math.Floor(y))
		x1 := x0 + 1
		y1 := y0 + 1
		occ := func(px, py int) bool {
			if px < 0 || px >= g.viewW || py < 0 || py >= g.viewH {
				return false
			}
			return g.spriteWallClipPointOccluded(px, py, depthQ)
		}
		n := 0
		if occ(x0, y0) {
			n++
		}
		if occ(x1, y0) {
			n++
		}
		if occ(x0, y1) {
			n++
		}
		if occ(x1, y1) {
			n++
		}
		return n >= 3
	}
	lineIntersectT := func(ax, ay, bx, by, cx, cy, dx, dy float64) (float64, bool) {
		rx := bx - ax
		ry := by - ay
		sx := dx - cx
		sy := dy - cy
		den := rx*sy - ry*sx
		if math.Abs(den) < 1e-9 {
			return 0, false
		}
		qpx := cx - ax
		qpy := cy - ay
		t := (qpx*sy - qpy*sx) / den
		u := (qpx*ry - qpy*rx) / den
		if t <= 0 || t >= 1 || u <= 0 || u >= 1 {
			return 0, false
		}
		return t, true
	}
	drawVisibleLine := func(x0, y0, x1, y1 float64, edgeOwner int, isOccluded func(float64, float64, float64) bool) {
		clipParamRangeToView := func(ta, tb float64) (float64, float64, bool) {
			if tb < ta {
				ta, tb = tb, ta
			}
			xa := x0 + (x1-x0)*ta
			ya := y0 + (y1-y0)*ta
			xb := x0 + (x1-x0)*tb
			yb := y0 + (y1-y0)*tb
			dx := xb - xa
			dy := yb - ya
			u0 := 0.0
			u1 := 1.0
			clip := func(p, q float64) bool {
				if math.Abs(p) < 1e-12 {
					return q >= 0
				}
				r := q / p
				if p < 0 {
					if r > u1 {
						return false
					}
					if r > u0 {
						u0 = r
					}
				} else {
					if r < u0 {
						return false
					}
					if r < u1 {
						u1 = r
					}
				}
				return true
			}
			maxX := float64(g.viewW) - 1.0
			maxY := float64(g.viewH) - 1.0
			if !clip(-dx, xa) || !clip(dx, maxX-xa) || !clip(-dy, ya) || !clip(dy, maxY-ya) {
				return 0, 0, false
			}
			if u1 < u0 {
				return 0, 0, false
			}
			t0 := ta + (tb-ta)*u0
			t1 := ta + (tb-ta)*u1
			if t1 <= t0 {
				return 0, 0, false
			}
			return t0, t1, true
		}
		ts := make([]float64, 0, 16)
		ts = append(ts, 0.0, 1.0)
		for _, e := range triEdges {
			// Avoid self-intersection cuts from the same triangle, and from the
			// paired triangle that came from the same wall quad split.
			if edgeOwner >= 0 && (e.owner == edgeOwner || e.pair == edgeOwner) {
				continue
			}
			if t, ok := lineIntersectT(x0, y0, x1, y1, e.x0, e.y0, e.x1, e.y1); ok {
				// Pure geometry split: intersections define cut candidates.
				ts = append(ts, t)
			}
		}
		sort.Float64s(ts)
		uniq := ts[:0]
		const eps = 1e-5
		for _, t := range ts {
			if len(uniq) == 0 || math.Abs(t-uniq[len(uniq)-1]) > eps {
				uniq = append(uniq, t)
			}
		}
		ts = uniq
		for i := 0; i+1 < len(ts); i++ {
			ta := ts[i]
			tb := ts[i+1]
			if tb-ta < eps {
				continue
			}
			t0, t1, ok := clipParamRangeToView(ta, tb)
			if !ok {
				continue
			}
			tm := (t0 + t1) * 0.5
			mx := x0 + (x1-x0)*tm
			my := y0 + (y1-y0)*tm
			if !inViewF(mx, my) {
				continue
			}
			clr := visibleClr
			if isOccluded(mx, my, tm) {
				if g.spriteClipDiagGreenOnly {
					continue
				}
				clr = occludedClr
			}
			ax := x0 + (x1-x0)*t0
			ay := y0 + (y1-y0)*t0
			bx := x0 + (x1-x0)*t1
			by := y0 + (y1-y0)*t1
			vector.StrokeLine(screen, float32(ax), float32(ay), float32(bx), float32(by), 1.2, clr, true)
		}
	}
	drawVisibleBox := func(x0, x1, y0, y1 int, isOccluded func(float64, float64, float64) bool) {
		drawVisibleLine(float64(x0), float64(y0), float64(x1), float64(y0), -1, isOccluded)
		drawVisibleLine(float64(x1), float64(y0), float64(x1), float64(y1), -1, isOccluded)
		drawVisibleLine(float64(x1), float64(y1), float64(x0), float64(y1), -1, isOccluded)
		drawVisibleLine(float64(x0), float64(y1), float64(x0), float64(y0), -1, isOccluded)
		drawVisibleLine(float64(x0), float64(y0), float64(x1), float64(y1), -1, isOccluded)
	}
	type wallTri struct {
		ax, ay float64
		bx, by float64
		cx, cy float64
		az     float64
		bz     float64
		cz     float64
		depthQ uint16
		state  int
		pair   int
		isWall bool
	}
	wallTris := make([]wallTri, 0, max(128, len(g.visibleBuf)*4))

	focal := doomFocalLength(g.viewW)
	camX := g.renderPX
	camY := g.renderPY
	camAng := angleToRadians(g.renderAngle)
	ca := math.Cos(camAng)
	sa := math.Sin(camAng)
	near := 2.0
	eyeZ := g.playerEyeZ()
	for _, si := range g.visibleBuf {
		pp := g.buildWallSegPrepassSingle(si, camX, camY, ca, sa, focal, near)
		if !pp.ok || pp.invF1 <= 0 || pp.invF2 <= 0 {
			continue
		}
		front, back := g.segSectors(si)
		if front == nil {
			continue
		}
		frontIdx, backIdx := g.segSectorIndices(si)
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
		ws := classifyWallPortal(front, back, eyeZ, frontFloor, frontCeil, backFloor, backCeil)
		f1 := 1.0 / pp.invF1
		f2 := 1.0 / pp.invF2
		if f1 <= 0 || f2 <= 0 {
			continue
		}
		drawWallSlice := func(zTop, zBottom float64) {
			yt1 := float64(g.viewH)/2 - (zTop/f1)*focal
			yt2 := float64(g.viewH)/2 - (zTop/f2)*focal
			yb1 := float64(g.viewH)/2 - (zBottom/f1)*focal
			yb2 := float64(g.viewH)/2 - (zBottom/f2)*focal
			if math.Abs(yt1-yb1) < 0.5 && math.Abs(yt2-yb2) < 0.5 {
				return
			}
			depthQ := encodeDepthQ((f1 + f2) * 0.5)
			ax := int(math.Floor(pp.sx1))
			ay := int(math.Floor(yt1))
			bx := int(math.Floor(pp.sx2))
			by := int(math.Floor(yt2))
			cx := int(math.Floor(pp.sx2))
			cy := int(math.Floor(yb2))
			dx := int(math.Floor(pp.sx1))
			dy := int(math.Floor(yb1))
			triAState := g.spriteWallClipTriangleOcclusionState(ax, ay, bx, by, cx, cy, depthQ)
			triBState := g.spriteWallClipTriangleOcclusionState(ax, ay, cx, cy, dx, dy, depthQ)
			i0 := len(wallTris)
			wallTris = append(wallTris,
				wallTri{
					ax: pp.sx1, ay: yt1,
					bx: pp.sx2, by: yt2,
					cx: pp.sx2, cy: yb2,
					az: f1, bz: f2, cz: f2,
					depthQ: depthQ,
					state:  triAState,
					pair:   i0 + 1,
					isWall: true,
				},
				wallTri{
					ax: pp.sx1, ay: yt1,
					bx: pp.sx2, by: yb2,
					cx: pp.sx1, cy: yb1,
					az: f1, bz: f2, cz: f1,
					depthQ: depthQ,
					state:  triBState,
					pair:   i0,
					isWall: true,
				},
			)
		}
		if ws.solidWall {
			drawWallSlice(ws.worldTop, ws.worldBottom)
			continue
		}
		if ws.topWall {
			drawWallSlice(ws.worldTop, ws.worldHigh)
		}
		if ws.bottomWall {
			drawWallSlice(ws.worldLow, ws.worldBottom)
		}
	}
	// Add floor/ceiling triangles from currently visible subsectors.
	type camPt struct {
		f float64
		s float64
	}
	clipCamPolyToNear := func(in []camPt, near float64) []camPt {
		if len(in) < 3 {
			return nil
		}
		out := make([]camPt, 0, len(in)+2)
		prev := in[len(in)-1]
		prevIn := prev.f >= near
		for _, cur := range in {
			curIn := cur.f >= near
			if prevIn && curIn {
				out = append(out, cur)
			} else if prevIn && !curIn {
				df := cur.f - prev.f
				if math.Abs(df) > 1e-9 {
					t := (near - prev.f) / df
					out = append(out, camPt{
						f: near,
						s: prev.s + (cur.s-prev.s)*t,
					})
				}
			} else if !prevIn && curIn {
				df := cur.f - prev.f
				if math.Abs(df) > 1e-9 {
					t := (near - prev.f) / df
					out = append(out, camPt{
						f: near,
						s: prev.s + (cur.s-prev.s)*t,
					})
				}
				out = append(out, cur)
			}
			prev = cur
			prevIn = curIn
		}
		if len(out) < 3 {
			return nil
		}
		return out
	}
	halfW := float64(g.viewW) * 0.5
	halfH := float64(g.viewH) * 0.5
	appendPlaneTri := func(ax, ay, az, bx, by, bz, cx, cy, cz float64) {
		depthQ := encodeDepthQ((az + bz + cz) / 3.0)
		// Keep floor/ceiling tris in the unified test set so plane-vs-plane
		// occlusion can happen between sectors.
		state := 0
		wallTris = append(wallTris, wallTri{
			ax: ax, ay: ay,
			bx: bx, by: by,
			cx: cx, cy: cy,
			az: az, bz: bz, cz: cz,
			depthQ: depthQ,
			state:  state,
			pair:   -1,
			isWall: false,
		})
	}
	g.ensureSectorPlaneLevelCacheFresh()
	emitSectorTri := func(zWorld float64, tri worldTri) {
		toCam := func(p worldPt) camPt {
			dx := p.x - camX
			dy := p.y - camY
			return camPt{
				f: dx*ca + dy*sa,
				s: -dx*sa + dy*ca,
			}
		}
		clipped := clipCamPolyToNear([]camPt{toCam(tri.a), toCam(tri.b), toCam(tri.c)}, near)
		if len(clipped) < 3 {
			return
		}
		screenX := make([]float64, len(clipped))
		screenY := make([]float64, len(clipped))
		depth := make([]float64, len(clipped))
		for i := range clipped {
			fv := clipped[i].f
			if fv <= 0 {
				return
			}
			screenX[i] = halfW - (clipped[i].s/fv)*focal
			screenY[i] = halfH - (zWorld/fv)*focal
			depth[i] = fv
		}
		for i := 1; i+1 < len(clipped); i++ {
			appendPlaneTri(
				screenX[0], screenY[0], depth[0],
				screenX[i], screenY[i], depth[i],
				screenX[i+1], screenY[i+1], depth[i+1],
			)
		}
	}
	sectorVisible := make([]bool, len(g.m.Sectors))
	for ss := 0; ss < len(g.m.SubSectors) && ss < len(g.visibleSubSectorSeen); ss++ {
		if g.visibleSubSectorSeen[ss] != g.visibleEpoch {
			continue
		}
		sec := -1
		if ss < len(g.subSectorPlaneID) {
			sec = g.subSectorPlaneID[ss]
		}
		if sec < 0 {
			sec = g.sectorForSubSector(ss)
		}
		if sec >= 0 && sec < len(sectorVisible) {
			sectorVisible[sec] = true
		}
	}
	for sec := 0; sec < len(g.m.Sectors); sec++ {
		if !sectorVisible[sec] {
			continue
		}
		tris := g.sectorPlaneTrisCached(sec)
		if len(tris) == 0 {
			continue
		}
		secRef := g.m.Sectors[sec]
		floorZ := float64(secRef.FloorHeight) - eyeZ
		for _, tri := range tris {
			emitSectorTri(floorZ, tri)
		}
		if !isSkyFlatName(secRef.CeilingPic) {
			ceilZ := float64(secRef.CeilingHeight) - eyeZ
			for _, tri := range tris {
				emitSectorTri(ceilZ, tri)
			}
		}
	}
	// Pass 1: build cut edges from non-culled wall triangles, but only on
	// true outer bounds (exclude internal split diagonal).
	for i, tr := range wallTris {
		// 2 = fully occluded; 0/1 can still contribute visible fragments.
		if tr.state == 2 {
			continue
		}
		if tr.isWall && tr.pair >= 0 && i%2 == 0 {
			// Triangle A (A-B-C): keep A-B and B-C; drop C-A diagonal.
			triEdges = append(triEdges,
				edge2{x0: tr.ax, y0: tr.ay, x1: tr.bx, y1: tr.by, owner: i, pair: tr.pair},
				edge2{x0: tr.bx, y0: tr.by, x1: tr.cx, y1: tr.cy, owner: i, pair: tr.pair},
			)
		} else if tr.isWall && tr.pair >= 0 {
			// Triangle B (A-C-D): keep C-D and D-A; drop A-C diagonal.
			triEdges = append(triEdges,
				edge2{x0: tr.bx, y0: tr.by, x1: tr.cx, y1: tr.cy, owner: i, pair: tr.pair},
				edge2{x0: tr.cx, y0: tr.cy, x1: tr.ax, y1: tr.ay, owner: i, pair: tr.pair},
			)
		} else {
			triEdges = append(triEdges,
				edge2{x0: tr.ax, y0: tr.ay, x1: tr.bx, y1: tr.by, owner: i, pair: tr.pair},
				edge2{x0: tr.bx, y0: tr.by, x1: tr.cx, y1: tr.cy, owner: i, pair: tr.pair},
				edge2{x0: tr.cx, y0: tr.cy, x1: tr.ax, y1: tr.ay, owner: i, pair: tr.pair},
			)
		}
	}
	// Pass 2: draw non-culled wall-triangle edges, cut against all cut edges.
	pointInTri2D := func(px, py float64, tr wallTri) bool {
		den := (tr.by-tr.cy)*(tr.ax-tr.cx) + (tr.cx-tr.bx)*(tr.ay-tr.cy)
		if math.Abs(den) < 1e-9 {
			return false
		}
		a := ((tr.by-tr.cy)*(px-tr.cx) + (tr.cx-tr.bx)*(py-tr.cy)) / den
		b := ((tr.cy-tr.ay)*(px-tr.cx) + (tr.ax-tr.cx)*(py-tr.cy)) / den
		c := 1.0 - a - b
		const eps = 1e-6
		return a >= -eps && b >= -eps && c >= -eps
	}
	triDepthAt := func(px, py float64, tr wallTri) (float64, bool) {
		den := (tr.by-tr.cy)*(tr.ax-tr.cx) + (tr.cx-tr.bx)*(tr.ay-tr.cy)
		if math.Abs(den) < 1e-9 {
			return 0, false
		}
		a := ((tr.by-tr.cy)*(px-tr.cx) + (tr.cx-tr.bx)*(py-tr.cy)) / den
		b := ((tr.cy-tr.ay)*(px-tr.cx) + (tr.ax-tr.cx)*(py-tr.cy)) / den
		c := 1.0 - a - b
		const eps = 1e-6
		if a < -eps || b < -eps || c < -eps {
			return 0, false
		}
		if tr.az <= 0 || tr.bz <= 0 || tr.cz <= 0 {
			return 0, false
		}
		// Perspective-correct depth interpolation in screen space.
		invZ := a*(1.0/tr.az) + b*(1.0/tr.bz) + c*(1.0/tr.cz)
		if invZ <= 0 {
			return 0, false
		}
		z := 1.0 / invZ
		if z <= 0 {
			return 0, false
		}
		return z, true
	}
	triOccludedAt := func(px, py, z float64, owner, pair int) bool {
		// Unified geometry occlusion: any nearer triangle can occlude this sample.
		for j, ot := range wallTris {
			if j == owner || j == pair {
				continue
			}
			if ot.state == 2 {
				continue
			}
			if !pointInTri2D(px, py, ot) {
				continue
			}
			oz, ok := triDepthAt(px, py, ot)
			if !ok {
				continue
			}
			// Smaller z is nearer camera.
			if oz+0.05 < z {
				return true
			}
		}
		return false
	}
	// Simplistic Ebiten triangle fill test: render projected tri surfaces.
	// Draw far-to-near and skip tris that are fully hidden by nearer tris.
	fillOpaqueOnly := g.spriteClipDiagOnly
	useSoftwareFill := fillOpaqueOnly
	if g.whitePixel == nil {
		g.whitePixel = ebiten.NewImage(1, 1)
		g.whitePixel.Fill(color.White)
	}
	var softPix []byte
	if useSoftwareFill {
		softPix = make([]byte, g.viewW*g.viewH*4)
	}
	fillOrder := make([]int, 0, len(wallTris))
	for i, tr := range wallTris {
		_ = tr
		fillOrder = append(fillOrder, i)
	}
	sort.Slice(fillOrder, func(i, j int) bool {
		a := wallTris[fillOrder[i]]
		b := wallTris[fillOrder[j]]
		za := (a.az + a.bz + a.cz) / 3.0
		zb := (b.az + b.bz + b.cz) / 3.0
		return za > zb
	})
	drawTriFill := func(tr wallTri, clr color.RGBA) {
		if clr.A == 0 {
			return
		}
		if useSoftwareFill {
			ax := tr.ax
			ay := tr.ay
			bx := tr.bx
			by := tr.by
			cx := tr.cx
			cy := tr.cy
			area := (bx-ax)*(cy-ay) - (by-ay)*(cx-ax)
			if math.Abs(area) < 1e-9 {
				return
			}
			minX := int(math.Floor(math.Min(ax, math.Min(bx, cx))))
			maxX := int(math.Ceil(math.Max(ax, math.Max(bx, cx))))
			minY := int(math.Floor(math.Min(ay, math.Min(by, cy))))
			maxY := int(math.Ceil(math.Max(ay, math.Max(by, cy))))
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
				return
			}
			edge := func(x0, y0, x1, y1, px, py float64) float64 {
				return (px-x0)*(y1-y0) - (py-y0)*(x1-x0)
			}
			sa := float64(clr.A) / 255.0
			sr := float64(clr.R)
			sg := float64(clr.G)
			sb := float64(clr.B)
			for y := minY; y <= maxY; y++ {
				py := float64(y) + 0.5
				for x := minX; x <= maxX; x++ {
					px := float64(x) + 0.5
					e0 := edge(ax, ay, bx, by, px, py)
					e1 := edge(bx, by, cx, cy, px, py)
					e2 := edge(cx, cy, ax, ay, px, py)
					if !((e0 >= 0 && e1 >= 0 && e2 >= 0) || (e0 <= 0 && e1 <= 0 && e2 <= 0)) {
						continue
					}
					i := (y*g.viewW + x) * 4
					dr := float64(softPix[i+0])
					dg := float64(softPix[i+1])
					db := float64(softPix[i+2])
					da := float64(softPix[i+3]) / 255.0
					oa := sa + da*(1.0-sa)
					if oa <= 1e-9 {
						continue
					}
					or := (sr*sa + dr*da*(1.0-sa)) / oa
					og := (sg*sa + dg*da*(1.0-sa)) / oa
					ob := (sb*sa + db*da*(1.0-sa)) / oa
					ri := int(math.Round(or))
					gi := int(math.Round(og))
					bi := int(math.Round(ob))
					ai := int(math.Round(oa * 255.0))
					if ri < 0 {
						ri = 0
					} else if ri > 255 {
						ri = 255
					}
					if gi < 0 {
						gi = 0
					} else if gi > 255 {
						gi = 255
					}
					if bi < 0 {
						bi = 0
					} else if bi > 255 {
						bi = 255
					}
					if ai < 0 {
						ai = 0
					} else if ai > 255 {
						ai = 255
					}
					softPix[i+0] = uint8(ri)
					softPix[i+1] = uint8(gi)
					softPix[i+2] = uint8(bi)
					softPix[i+3] = uint8(ai)
				}
			}
			return
		}
		r := float32(clr.R) / 255.0
		gc := float32(clr.G) / 255.0
		b := float32(clr.B) / 255.0
		a := float32(clr.A) / 255.0
		vtx := []ebiten.Vertex{
			// Solid-color fill from a 1x1 white texture: keep UV fixed to a
			// valid texel to avoid clamp-to-zero sampling artifacts.
			{DstX: float32(tr.ax), DstY: float32(tr.ay), SrcX: 0, SrcY: 0, ColorR: r, ColorG: gc, ColorB: b, ColorA: a},
			{DstX: float32(tr.bx), DstY: float32(tr.by), SrcX: 0, SrcY: 0, ColorR: r, ColorG: gc, ColorB: b, ColorA: a},
			{DstX: float32(tr.cx), DstY: float32(tr.cy), SrcX: 0, SrcY: 0, ColorR: r, ColorG: gc, ColorB: b, ColorA: a},
		}
		screen.DrawTriangles(vtx, []uint16{0, 1, 2}, g.whitePixel, &ebiten.DrawTrianglesOptions{
			Filter:    ebiten.FilterNearest,
			Address:   ebiten.AddressClampToZero,
			AntiAlias: false,
		})
	}
	for _, idx := range fillOrder {
		tr := wallTris[idx]
		if tr.az <= 0 || tr.bz <= 0 || tr.cz <= 0 {
			continue
		}
		alpha := uint8(56)
		if fillOpaqueOnly {
			alpha = 255
		}
		clr := color.RGBA{R: 72, G: 245, B: 96, A: alpha}
		if tr.state == 1 {
			if g.spriteClipDiagGreenOnly {
				continue
			}
			clr = color.RGBA{R: 255, G: 166, B: 64, A: alpha}
		}
		drawTriFill(tr, clr)
	}
	if useSoftwareFill {
		softImg := ebiten.NewImage(g.viewW, g.viewH)
		softImg.WritePixels(softPix)
		screen.DrawImage(softImg, nil)
	}
	if fillOpaqueOnly {
		return
	}
	for i, tr := range wallTris {
		if tr.state == 2 {
			if !g.spriteClipDiagGreenOnly {
				alwaysOcc := func(_, _, _ float64) bool { return true }
				drawVisibleLine(tr.ax, tr.ay, tr.bx, tr.by, i, alwaysOcc)
				drawVisibleLine(tr.bx, tr.by, tr.cx, tr.cy, i, alwaysOcc)
				drawVisibleLine(tr.cx, tr.cy, tr.ax, tr.ay, i, alwaysOcc)
			}
			continue
		}
		// Wall vectors use depth-aware per-point occlusion so only nearer BSP
		// surfaces cut the edge at that sample point.
		drawWallEdge := func(x0, y0, z0, x1, y1, z1 float64) {
			invZ0 := 0.0
			invZ1 := 0.0
			if z0 > 0 {
				invZ0 = 1.0 / z0
			}
			if z1 > 0 {
				invZ1 = 1.0 / z1
			}
			wallOcc := func(x, y, t float64) bool {
				if t < 0 {
					t = 0
				} else if t > 1 {
					t = 1
				}
				// Perspective-correct depth interpolation along the edge.
				invZ := invZ0 + (invZ1-invZ0)*t
				if invZ <= 0 {
					return true
				}
				z := 1.0 / invZ
				if z <= 0 {
					return true
				}
				px := x
				py := y
				n := 0
				if triOccludedAt(px, py, z, i, tr.pair) {
					n++
				}
				if triOccludedAt(px-0.5, py, z, i, tr.pair) {
					n++
				}
				if triOccludedAt(px+0.5, py, z, i, tr.pair) {
					n++
				}
				if triOccludedAt(px, py-0.5, z, i, tr.pair) {
					n++
				}
				if triOccludedAt(px, py+0.5, z, i, tr.pair) {
					n++
				}
				return n >= 3
			}
			drawVisibleLine(x0, y0, x1, y1, i, wallOcc)
		}
		drawWallEdge(tr.ax, tr.ay, tr.az, tr.bx, tr.by, tr.bz)
		drawWallEdge(tr.bx, tr.by, tr.bz, tr.cx, tr.cy, tr.cz)
		drawWallEdge(tr.cx, tr.cy, tr.cz, tr.ax, tr.ay, tr.az)
	}
	for _, it := range g.projectileItemsScratch {
		x0, x1, y0, y1, ok := projectileItemScreenBounds(it, g.viewW, g.viewH)
		if ok {
			dq := encodeDepthQ(it.dist)
			occ := func(x, y, _ float64) bool { return spriteOccAtFloat(x, y, dq) }
			drawVisibleBox(x0, x1, y0, y1, occ)
		}
	}
	for _, it := range g.monsterItemsScratch {
		x0, x1, y0, y1, ok := monsterItemScreenBounds(it, g.viewW, g.viewH)
		if ok {
			dq := encodeDepthQ(it.dist)
			occ := func(x, y, _ float64) bool { return spriteOccAtFloat(x, y, dq) }
			drawVisibleBox(x0, x1, y0, y1, occ)
		}
	}
	for _, it := range g.thingItemsScratch {
		x0, x1, y0, y1, ok := thingItemScreenBounds(it, g.viewW, g.viewH)
		if ok {
			dq := encodeDepthQ(it.dist)
			occ := func(x, y, _ float64) bool { return spriteOccAtFloat(x, y, dq) }
			drawVisibleBox(x0, x1, y0, y1, occ)
		}
	}
	for _, it := range g.puffItemsScratch {
		x0, x1, y0, y1, ok := puffItemScreenBounds(it, focal, g.viewW, g.viewH)
		if ok {
			dq := encodeDepthQ(it.dist)
			occ := func(x, y, _ float64) bool { return spriteOccAtFloat(x, y, dq) }
			drawVisibleBox(x0, x1, y0, y1, occ)
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
	key := plane3DKey{
		light:    160,
		fallback: true,
		floor:    floor,
	}
	if sec == nil {
		return key
	}
	key.light = sec.Light
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
		key.flat = "SKY"
		key.fallback = true
		return key
	}
	key.flat = g.resolveAnimatedFlatName(pic)
	tex, ok := g.flatRGBAResolvedKey(key.flat)
	key.fallback = !ok || len(tex) != 64*64*4
	return key
}

func (g *game) drawBasicWallColumn(wallTop, wallBottom []int, x, y0, y1 int, depth float64, sectorLight int16, lightBias int, base color.RGBA, texU, texMid, focal float64, tex WallTexture, useTex bool) {
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
			shadeMul = doomShadeMulFromRowF(doomWallLightRowF(sectorLight, lightBias, depth, focal))
		}
	}
	if useTex {
		g.drawBasicWallColumnTextured(x, y0, y1, depth, texU, texMid, focal, tex, shadeMul, doomRow)
		g.writeDepthColumn(x, y0, y1, depth)
		return
	}
	rowStridePix := g.viewW
	pixI := y0*rowStridePix + x
	pix32 := g.wallPix32
	baseR := base.R
	baseG := base.G
	baseB := base.B
	if doomColormapEnabled {
		basePacked := shadePackedDOOMColormapRow(packRGBA(base.R, base.G, base.B), doomRow)
		for y := y0; y <= y1; y++ {
			pix32[pixI] = basePacked
			pixI += rowStridePix
		}
		g.writeDepthColumn(x, y0, y1, depth)
		return
	}
	if shadeMul != 256 {
		wallShadeLUTOnce.Do(initWallShadeLUT)
		shade := &wallShadeLUT[shadeMul]
		baseR = shade[base.R]
		baseG = shade[base.G]
		baseB = shade[base.B]
	}
	basePacked := packRGBA(baseR, baseG, baseB)
	for y := y0; y <= y1; y++ {
		pix32[pixI] = basePacked
		pixI += rowStridePix
	}
	g.writeDepthColumn(x, y0, y1, depth)
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
	if g == nil || g.depthOcclusionEnabled() || x < 0 || x >= len(g.maskedClipCols) || y0 > y1 {
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
	g.maskedClipCols[x] = append(g.maskedClipCols[x], maskedClipSpan{
		y0:      int16(y0),
		y1:      int16(y1),
		depthQ:  depthQ,
		closed:  false,
		hasOpen: false,
	})
}

func (g *game) markSpriteClipColumnClosed(x int, depthQ uint16) {
	if g == nil || g.depthOcclusionEnabled() || x < 0 || x >= len(g.maskedClipCols) {
		return
	}
	g.maskedClipCols[x] = append(g.maskedClipCols[x], maskedClipSpan{
		depthQ:  depthQ,
		closed:  true,
		hasOpen: false,
	})
}

func (g *game) appendSpritePortalColumnGap(x, openY0, openY1 int, depthQ uint16) {
	if g == nil || g.depthOcclusionEnabled() || x < 0 || x >= len(g.maskedClipCols) {
		return
	}
	if openY0 < 0 {
		openY0 = 0
	}
	if openY1 >= g.viewH {
		openY1 = g.viewH - 1
	}
	g.maskedClipCols[x] = append(g.maskedClipCols[x], maskedClipSpan{
		openY0:  int16(openY0),
		openY1:  int16(openY1),
		depthQ:  depthQ,
		closed:  false,
		hasOpen: true,
	})
}

func (g *game) depthOcclusionEnabled() bool {
	return false
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

func (g *game) writeDepthColumn(x, y0, y1 int, depth float64) {
	if !g.depthOcclusionEnabled() {
		return
	}
	if x < 0 || x >= g.viewW || y0 > y1 || len(g.depthPix3D) != g.viewW*g.viewH {
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
	pixI := y0*g.viewW + x
	stamp := g.depthFrameStamp
	d := encodeDepthQ(depth)
	packed := packDepthStamped(d, stamp)
	for y := y0; y <= y1; y++ {
		g.depthPix3D[pixI] = packed
		pixI += g.viewW
	}
}

func (g *game) setDepthPixel(idx int, depth float64) {
	if !g.depthOcclusionEnabled() {
		return
	}
	g.setDepthPixelEncoded(idx, packDepthStamped(encodeDepthQ(depth), g.depthFrameStamp))
}

func (g *game) setDepthPixelEncoded(idx int, packed uint32) {
	if !g.depthOcclusionEnabled() {
		return
	}
	if idx < 0 || idx >= len(g.depthPix3D) {
		return
	}
	g.depthPix3D[idx] = packed
}

func (g *game) setDepthPixelPairEncoded(idx int, packed uint32) {
	if !g.depthOcclusionEnabled() {
		return
	}
	if idx < 0 || idx+1 >= len(g.depthPix3D) {
		return
	}
	ptr := unsafe.Pointer(&g.depthPix3D[idx])
	if uintptr(ptr)%unsafe.Alignof(uint64(0)) == 0 {
		pair := (uint64(packed) << 32) | uint64(packed)
		*(*uint64)(ptr) = pair
		return
	}
	g.depthPix3D[idx] = packed
	g.depthPix3D[idx+1] = packed
}

func (g *game) setPlaneDepthMin(idx int, depth float64) {
	if !g.depthOcclusionEnabled() {
		return
	}
	if idx < 0 || idx >= len(g.depthPlanePix3D) {
		return
	}
	stamp := g.depthFrameStamp
	d := encodeDepthQ(depth)
	g.setPlaneDepthMinEncoded(idx, stamp, d, packDepthStamped(d, stamp))
}

func (g *game) setPlaneDepthMinEncoded(idx int, stamp, d uint16, packed uint32) {
	if !g.depthOcclusionEnabled() {
		return
	}
	if idx < 0 || idx >= len(g.depthPlanePix3D) {
		return
	}
	cur := g.depthPlanePix3D[idx]
	if unpackDepthStamp(cur) != stamp || d < unpackDepthQ(cur) {
		g.depthPlanePix3D[idx] = packed
	}
}

func (g *game) setPlaneDepthMinPairEncoded(idx int, stamp, d uint16, packed uint32) {
	if !g.depthOcclusionEnabled() {
		return
	}
	if idx < 0 || idx+1 >= len(g.depthPlanePix3D) {
		return
	}
	cur0 := g.depthPlanePix3D[idx]
	cur1 := g.depthPlanePix3D[idx+1]
	update0 := unpackDepthStamp(cur0) != stamp || d < unpackDepthQ(cur0)
	update1 := unpackDepthStamp(cur1) != stamp || d < unpackDepthQ(cur1)
	if !update0 && !update1 {
		return
	}
	if update0 && update1 {
		ptr := unsafe.Pointer(&g.depthPlanePix3D[idx])
		if uintptr(ptr)%unsafe.Alignof(uint64(0)) == 0 {
			pair := (uint64(packed) << 32) | uint64(packed)
			*(*uint64)(ptr) = pair
			return
		}
	}
	if update0 {
		g.depthPlanePix3D[idx] = packed
	}
	if update1 {
		g.depthPlanePix3D[idx+1] = packed
	}
}

func (g *game) depthQAtStamped(idx int, stamp uint16) (uint16, bool) {
	if !g.depthOcclusionEnabled() {
		return 0, false
	}
	if idx < 0 || idx >= len(g.depthPix3D) {
		return 0, false
	}
	var (
		best uint16
		ok   bool
	)
	if cur := g.depthPix3D[idx]; unpackDepthStamp(cur) == stamp {
		best = unpackDepthQ(cur)
		ok = true
	}
	if idx < len(g.depthPlanePix3D) {
		cur := g.depthPlanePix3D[idx]
		if unpackDepthStamp(cur) != stamp {
			return best, ok
		}
		pd := unpackDepthQ(cur)
		if !ok || pd < best {
			best = pd
			ok = true
		}
	}
	return best, ok
}

func (g *game) rowFullyOccludedDepthQ(depthQ, planeBiasQ uint16, rowBase, x0, x1 int) bool {
	if !g.billboardClippingEnabled() {
		return false
	}
	if !g.depthOcclusionEnabled() {
		if g == nil || g.viewW <= 0 {
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
	if x1 < x0 || rowBase < 0 {
		return false
	}
	stamp := g.depthFrameStamp
	for x := x0; x <= x1; x++ {
		idx := rowBase + x
		if idx < 0 || idx >= len(g.depthPix3D) {
			return false
		}
		if cur := g.depthPix3D[idx]; unpackDepthStamp(cur) == stamp && depthQ > unpackDepthQ(cur) {
			continue
		}
		if idx >= len(g.depthPlanePix3D) {
			return false
		}
		cur := g.depthPlanePix3D[idx]
		if unpackDepthStamp(cur) != stamp {
			return false
		}
		threshold := addDepthQ(unpackDepthQ(cur), planeBiasQ)
		if depthQ > threshold {
			continue
		}
		return false
	}
	return true
}

func (g *game) rowFullyOccludedByWallsDepthQ(depthQ uint16, rowBase, x0, x1 int) bool {
	if !g.depthOcclusionEnabled() {
		return false
	}
	if x1 < x0 || rowBase < 0 {
		return false
	}
	stamp := g.depthFrameStamp
	for x := x0; x <= x1; x++ {
		idx := rowBase + x
		if idx < 0 || idx >= len(g.depthPix3D) {
			return false
		}
		cur := g.depthPix3D[idx]
		if unpackDepthStamp(cur) != stamp || depthQ <= unpackDepthQ(cur) {
			return false
		}
	}
	return true
}

func solidSpanContainsX(spans []solidSpan, x int) bool {
	for _, s := range spans {
		if x < s.l {
			return false
		}
		if x <= s.r {
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

func (g *game) rowPointOccludedByWallsDepthQ(depthQ uint16, rowBase, x int) bool {
	if g == nil || g.viewW <= 0 || g.viewH <= 0 || x < 0 || x >= g.viewW || rowBase < 0 {
		return true
	}
	if g.depthOcclusionEnabled() {
		idx := rowBase + x
		if idx < 0 || idx >= len(g.depthPix3D) {
			return true
		}
		cur := g.depthPix3D[idx]
		return unpackDepthStamp(cur) == g.depthFrameStamp && depthQ > unpackDepthQ(cur)
	}
	y := rowBase / g.viewW
	return g.spriteWallClipOccludedAtXYDepth(x, y, depthQ)
}

func (g *game) rowFullyOccludedByWallsFastDepthQ(depthQ uint16, rowBase, x0, x1 int) bool {
	if x1 < x0 || rowBase < 0 {
		return false
	}
	mid := (x0 + x1) >> 1
	if !g.rowPointOccludedByWallsDepthQ(depthQ, rowBase, x0) ||
		!g.rowPointOccludedByWallsDepthQ(depthQ, rowBase, x1) ||
		!g.rowPointOccludedByWallsDepthQ(depthQ, rowBase, mid) {
		return false
	}
	if g.depthOcclusionEnabled() {
		return g.rowFullyOccludedByWallsDepthQ(depthQ, rowBase, x0, x1)
	}
	y := rowBase / g.viewW
	for x := x0; x <= x1; x++ {
		if !g.spriteWallClipOccludedAtXYDepth(x, y, depthQ) {
			return false
		}
	}
	return true
}

func (g *game) spriteOccludedAt(depth float64, idx int, planeBias float64) bool {
	if !g.billboardClippingEnabled() {
		return false
	}
	if !g.depthOcclusionEnabled() {
		return g.spriteWallClipOccludedAtIndexDepth(idx, encodeDepthQ(depth))
	}
	return g.spriteOccludedDepthQAt(g.depthPix3D, g.depthPlanePix3D, g.depthFrameStamp, encodeDepthQ(depth), encodeDepthBiasQ(planeBias), idx)
}

func (g *game) spriteOccludedDepthQAt(depthPix, depthPlanePix []uint32, stamp, depthQ, planeBiasQ uint16, idx int) bool {
	if !g.billboardClippingEnabled() {
		return false
	}
	if !g.depthOcclusionEnabled() {
		return g.spriteWallClipOccludedAtIndexDepth(idx, depthQ)
	}
	return spriteOccludedDepthQAtCore(depthPix, depthPlanePix, stamp, depthQ, planeBiasQ, idx)
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
	wq := g.wallDepthQCol[x]
	if wq != 0xFFFF && depthQ > wq {
		if x < len(g.wallDepthClosedCol) && g.wallDepthClosedCol[x] {
			return true
		}
		if x < len(g.wallDepthTopCol) && x < len(g.wallDepthBottomCol) {
			top := g.wallDepthTopCol[x]
			bottom := g.wallDepthBottomCol[x]
			if y >= top && y <= bottom {
				return true
			}
		} else {
			return true
		}
	}
	if x >= len(g.maskedClipCols) {
		return false
	}
	for _, sp := range g.maskedClipCols[x] {
		if depthQ <= sp.depthQ {
			continue
		}
		if sp.closed {
			return true
		}
		if sp.hasOpen {
			if y < int(sp.openY0) || y > int(sp.openY1) {
				return true
			}
			continue
		}
		if y >= int(sp.y0) && y <= int(sp.y1) {
			return true
		}
	}
	return false
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
	wq := g.wallDepthQCol[x]
	if wq != 0xFFFF && depthQ > wq {
		if x < len(g.wallDepthClosedCol) && g.wallDepthClosedCol[x] {
			return true
		}
		if x < len(g.wallDepthTopCol) && x < len(g.wallDepthBottomCol) {
			top := g.wallDepthTopCol[x]
			bottom := g.wallDepthBottomCol[x]
			if y0 >= top && y1 <= bottom {
				return true
			}
		} else {
			return true
		}
	}
	// Transparent/masked mid textures should not act as hard occluders.
	return false
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
	wq := g.wallDepthQCol[x]
	if wq != 0xFFFF && depthQ > wq {
		if x < len(g.wallDepthClosedCol) && g.wallDepthClosedCol[x] {
			return true
		}
		if x < len(g.wallDepthTopCol) && x < len(g.wallDepthBottomCol) {
			top := g.wallDepthTopCol[x]
			bottom := g.wallDepthBottomCol[x]
			if y0 >= top && y1 <= bottom {
				return true
			}
		} else {
			return true
		}
	}
	return false
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
	for x := x0; x <= x1; x++ {
		if !g.wallClipColumnOccludedBBoxByWallsOnly(x, y0, y1, depthQ) {
			return false
		}
	}
	return true
}

func (g *game) spriteWallClipBBoxFullyOccluded(x0, x1, y0, y1 int, depthQ uint16) bool {
	if g == nil || g.viewW <= 0 || x0 > x1 || y0 > y1 {
		return true
	}
	if x0 < 0 {
		x0 = 0
	}
	if x1 >= g.viewW {
		x1 = g.viewW - 1
	}
	if x0 > x1 {
		return true
	}
	for x := x0; x <= x1; x++ {
		if !g.spriteWallClipColumnOccludedBBox(x, y0, y1, depthQ) {
			return false
		}
	}
	return true
}

func (g *game) spriteWallClipBBoxHasAnyOccluder(x0, x1, y0, y1 int, depthQ uint16) bool {
	if !g.billboardClippingEnabled() || g == nil || g.viewW <= 0 || x0 > x1 || y0 > y1 {
		return false
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
		return false
	}
	for x := x0; x <= x1; x++ {
		if x < len(g.wallDepthQCol) {
			wq := g.wallDepthQCol[x]
			if wq != 0xFFFF && depthQ > wq {
				if x < len(g.wallDepthClosedCol) && g.wallDepthClosedCol[x] {
					return true
				}
				if x < len(g.wallDepthTopCol) && x < len(g.wallDepthBottomCol) {
					top := g.wallDepthTopCol[x]
					bottom := g.wallDepthBottomCol[x]
					if y0 <= bottom && y1 >= top {
						return true
					}
				} else {
					return true
				}
			}
		}
		if x >= len(g.maskedClipCols) {
			continue
		}
		for _, sp := range g.maskedClipCols[x] {
			if depthQ <= sp.depthQ {
				continue
			}
			if sp.closed {
				return true
			}
			if sp.hasOpen {
				if y0 < int(sp.openY0) || y1 > int(sp.openY1) {
					return true
				}
				continue
			}
			if y0 <= int(sp.y1) && y1 >= int(sp.y0) {
				return true
			}
		}
	}
	return false
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

func (g *game) spriteOpaqueRectsHaveAnyOccluder(rects []spriteOpaqueRect, dstX, dstY, scale float64, clipTop, clipBottom, viewW, viewH int, depthQ uint16) bool {
	if len(rects) == 0 {
		return false
	}
	for _, rect := range rects {
		x0, x1, y0, y1, ok := spriteRectScreenBounds(rect, dstX, dstY, scale, clipTop, clipBottom, viewW, viewH)
		if !ok {
			continue
		}
		if g.spriteWallClipBBoxHasAnyOccluder(x0, x1, y0, y1, depthQ) {
			return true
		}
	}
	return false
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
	// Split quad into two triangles:
	// T0: (x0,y0), (x1,y0), (x1,y1)
	// T1: (x0,y0), (x1,y1), (x0,y1)
	if !g.spriteWallClipPointOccluded(x0, y0, depthQ) {
		return true
	}
	if !g.spriteWallClipPointOccluded(x1, y0, depthQ) {
		return true
	}
	if !g.spriteWallClipPointOccluded(x1, y1, depthQ) {
		return true
	}
	if !g.spriteWallClipPointOccluded(x0, y1, depthQ) {
		return true
	}
	// Catch center openings before falling back to full-column coverage.
	cx := (x0 + x1) >> 1
	cy := (y0 + y1) >> 1
	if !g.spriteWallClipPointOccluded(cx, cy, depthQ) {
		return true
	}
	return false
}

func (g *game) spriteWallClipTriangleFullyOccludedFast(ax, ay, bx, by, cx, cy int, depthQ uint16) bool {
	return g.spriteWallClipTriangleOcclusionState(ax, ay, bx, by, cx, cy, depthQ) == 2
}

// Returns:
// 0 = visible
// 1 = maybe occluded (fast tests say maybe, exact confirms not fully occluded)
// 2 = fully occluded
func (g *game) spriteWallClipTriangleOcclusionState(ax, ay, bx, by, cx, cy int, depthQ uint16) int {
	if g == nil || g.viewW <= 0 || g.viewH <= 0 {
		return 2
	}
	edgeMaybeVisible := func(x0, y0, x1, y1 int) bool {
		dx := x1 - x0
		if dx < 0 {
			dx = -dx
		}
		dy := y1 - y0
		if dy < 0 {
			dy = -dy
		}
		steps := dx
		if dy > steps {
			steps = dy
		}
		if steps < 1 {
			steps = 1
		}
		// Limit per-edge sampling cost while still catching sliver visibility.
		if steps > 32 {
			steps = 32
		}
		for i := 0; i <= steps; i++ {
			t := float64(i) / float64(steps)
			x := int(math.Floor(float64(x0) + float64(x1-x0)*t))
			y := int(math.Floor(float64(y0) + float64(y1-y0)*t))
			if !g.spriteWallClipPointOccluded(x, y, depthQ) {
				return true
			}
		}
		return false
	}
	if !g.spriteWallClipPointOccluded(ax, ay, depthQ) {
		return 0
	}
	if !g.spriteWallClipPointOccluded(bx, by, depthQ) {
		return 0
	}
	if !g.spriteWallClipPointOccluded(cx, cy, depthQ) {
		return 0
	}
	mx := (ax + bx + cx) / 3
	my := (ay + by + cy) / 3
	if !g.spriteWallClipPointOccluded(mx, my, depthQ) {
		return 0
	}
	// If any triangle edge has a visible sample, don't cull yet.
	if edgeMaybeVisible(ax, ay, bx, by) || edgeMaybeVisible(bx, by, cx, cy) || edgeMaybeVisible(cx, cy, ax, ay) {
		return 0
	}
	// Point sampling can miss small visible slivers inside the triangle.
	// Require an exact per-column occlusion confirmation over the triangle AABB
	// before declaring full occlusion.
	x0 := ax
	if bx < x0 {
		x0 = bx
	}
	if cx < x0 {
		x0 = cx
	}
	x1 := ax
	if bx > x1 {
		x1 = bx
	}
	if cx > x1 {
		x1 = cx
	}
	y0 := ay
	if by < y0 {
		y0 = by
	}
	if cy < y0 {
		y0 = cy
	}
	y1 := ay
	if by > y1 {
		y1 = by
	}
	if cy > y1 {
		y1 = cy
	}
	if g.spriteWallClipBBoxFullyOccluded(x0, x1, y0, y1, depthQ) {
		return 2
	}
	return 1
}

func (g *game) wallSliceYDepthAtX(pp wallSegPrepass, x int, z, focal float64) (float64, float64, bool) {
	if pp.sx2 == pp.sx1 {
		return 0, 0, false
	}
	t := (float64(x) - pp.sx1) / (pp.sx2 - pp.sx1)
	if t < 0 {
		t = 0
	}
	if t > 1 {
		t = 1
	}
	invF := pp.invF1 + (pp.invF2-pp.invF1)*t
	if invF <= 0 {
		return 0, 0, false
	}
	f := 1.0 / invF
	if f <= 0 {
		return 0, 0, false
	}
	y := float64(g.viewH)/2 - (z/f)*focal
	return y, f, true
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
	depthQAtX := func(x int) uint16 {
		if pp.sx2 == pp.sx1 {
			return encodeDepthQ((fL + fR) * 0.5)
		}
		t := (float64(x) - pp.sx1) / (pp.sx2 - pp.sx1)
		if t < 0 {
			t = 0
		}
		if t > 1 {
			t = 1
		}
		invF := pp.invF1 + (pp.invF2-pp.invF1)*t
		if invF <= 0 {
			return encodeDepthQ((fL + fR) * 0.5)
		}
		return encodeDepthQ(1.0 / invF)
	}
	triOccState := func(ax, ay, bx, by, cx, cy int) int {
		inView := func(x, y int) bool {
			return x >= 0 && x < g.viewW && y >= 0 && y < g.viewH
		}
		edgeMaybeVisible := func(x0, y0, x1, y1 int) bool {
			dx := x1 - x0
			if dx < 0 {
				dx = -dx
			}
			dy := y1 - y0
			if dy < 0 {
				dy = -dy
			}
			steps := dx
			if dy > steps {
				steps = dy
			}
			if steps < 1 {
				steps = 1
			}
			if steps > 32 {
				steps = 32
			}
			tested := false
			for i := 0; i <= steps; i++ {
				t := float64(i) / float64(steps)
				x := int(math.Floor(float64(x0) + float64(x1-x0)*t))
				y := int(math.Floor(float64(y0) + float64(y1-y0)*t))
				if !inView(x, y) {
					continue
				}
				tested = true
				if !g.wallClipPointOccludedByWallsOnly(x, y, depthQAtX(x)) {
					return true
				}
			}
			// No in-view sample means this edge can't prove full cull.
			if !tested {
				return true
			}
			return false
		}
		tested := false
		testPointOccluded := func(x, y int) bool {
			if !inView(x, y) {
				return true
			}
			tested = true
			return g.wallClipPointOccludedByWallsOnly(x, y, depthQAtX(x))
		}
		if !testPointOccluded(ax, ay) ||
			!testPointOccluded(bx, by) ||
			!testPointOccluded(cx, cy) {
			return 0
		}
		mx := (ax + bx + cx) / 3
		my := (ay + by + cy) / 3
		if !testPointOccluded(mx, my) {
			return 0
		}
		// If no point landed on-screen, keep for raster; don't cull.
		if !tested {
			return 0
		}
		if edgeMaybeVisible(ax, ay, bx, by) || edgeMaybeVisible(bx, by, cx, cy) || edgeMaybeVisible(cx, cy, ax, ay) {
			return 0
		}
		x0 := ax
		if bx < x0 {
			x0 = bx
		}
		if cx < x0 {
			x0 = cx
		}
		x1 := ax
		if bx > x1 {
			x1 = bx
		}
		if cx > x1 {
			x1 = cx
		}
		y0 := ay
		if by < y0 {
			y0 = by
		}
		if cy < y0 {
			y0 = cy
		}
		y1 := ay
		if by > y1 {
			y1 = by
		}
		if cy > y1 {
			y1 = cy
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
			return 0
		}
		allColsOcc := true
		for x := x0; x <= x1; x++ {
			if !g.wallClipColumnOccludedBBoxByWallsOnly(x, y0, y1, depthQAtX(x)) {
				allColsOcc = false
				break
			}
		}
		if allColsOcc {
			return 2
		}
		return 1
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

func projectileItemScreenBounds(it projectedProjectileItem, viewW, viewH int) (int, int, int, int, bool) {
	if viewW <= 0 || viewH <= 0 {
		return 0, -1, 0, -1, false
	}
	x0, x1, y0, y1 := 0, -1, 0, -1
	if it.hasSprite && it.spriteTex.Width > 0 && it.spriteTex.Height > 0 {
		scale := it.h / float64(it.spriteTex.Height)
		if scale <= 0 {
			return 0, -1, 0, -1, false
		}
		dstW := float64(it.spriteTex.Width) * scale
		dstH := float64(it.spriteTex.Height) * scale
		dstX := it.sx - float64(it.spriteTex.OffsetX)*scale
		dstY := it.yb - float64(it.spriteTex.OffsetY)*scale
		x0 = int(math.Floor(dstX))
		y0 = int(math.Floor(dstY))
		x1 = int(math.Ceil(dstX+dstW)) - 1
		y1 = int(math.Ceil(dstY+dstH)) - 1
	} else {
		rad := it.h * 0.5
		if rad <= 0 {
			return 0, -1, 0, -1, false
		}
		cy := it.yb - rad
		x0 = int(math.Floor(it.sx - rad))
		x1 = int(math.Ceil(it.sx + rad))
		y0 = int(math.Floor(cy - rad))
		y1 = int(math.Ceil(cy + rad))
	}
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
	if x0 > x1 || y0 > y1 {
		return x0, x1, y0, y1, false
	}
	return x0, x1, y0, y1, true
}

func monsterItemScreenBounds(it projectedMonsterItem, viewW, viewH int) (int, int, int, int, bool) {
	if viewW <= 0 || viewH <= 0 || it.tex.Width <= 0 || it.tex.Height <= 0 {
		return 0, -1, 0, -1, false
	}
	scale := it.h / float64(it.tex.Height)
	if scale <= 0 {
		return 0, -1, 0, -1, false
	}
	dstW := float64(it.tex.Width) * scale
	dstH := float64(it.tex.Height) * scale
	dstX := it.sx - float64(it.tex.OffsetX)*scale
	dstY := it.yb - float64(it.tex.OffsetY)*scale
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
	if x0 > x1 || y0 > y1 {
		return x0, x1, y0, y1, false
	}
	return x0, x1, y0, y1, true
}

func thingItemScreenBounds(it projectedThingItem, viewW, viewH int) (int, int, int, int, bool) {
	if viewW <= 0 || viewH <= 0 || it.tex.Width <= 0 || it.tex.Height <= 0 {
		return 0, -1, 0, -1, false
	}
	scale := it.h / float64(it.tex.Height)
	if scale <= 0 {
		return 0, -1, 0, -1, false
	}
	dstW := float64(it.tex.Width) * scale
	dstH := float64(it.tex.Height) * scale
	dstX := it.sx - float64(it.tex.OffsetX)*scale
	dstY := it.yb - float64(it.tex.OffsetY)*scale
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
	if x0 > x1 || y0 > y1 {
		return x0, x1, y0, y1, false
	}
	return x0, x1, y0, y1, true
}

func spriteRectScreenBounds(rect spriteOpaqueRect, dstX, dstY, scale float64, clipTop, clipBottom, viewW, viewH int) (int, int, int, int, bool) {
	if scale <= 0 || viewW <= 0 || viewH <= 0 {
		return 0, -1, 0, -1, false
	}
	x0 := int(math.Floor(dstX + float64(rect.minX)*scale))
	y0 := int(math.Floor(dstY + float64(rect.minY)*scale))
	x1 := int(math.Ceil(dstX+float64(int(rect.maxX)+1)*scale)) - 1
	y1 := int(math.Ceil(dstY+float64(int(rect.maxY)+1)*scale)) - 1
	if x1 < 0 || y1 < 0 || x0 >= viewW || y0 >= viewH {
		return 0, -1, 0, -1, false
	}
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
	if y0 < clipTop {
		y0 = clipTop
	}
	if y1 > clipBottom {
		y1 = clipBottom
	}
	if x0 > x1 || y0 > y1 {
		return 0, -1, 0, -1, false
	}
	return x0, x1, y0, y1, true
}

func puffItemScreenBounds(it projectedPuffItem, focal float64, viewW, viewH int) (int, int, int, int, bool) {
	if viewW <= 0 || viewH <= 0 {
		return 0, -1, 0, -1, false
	}
	x0, x1, y0, y1 := 0, -1, 0, -1
	if it.hasSprite && it.spriteTex.Width > 0 && it.spriteTex.Height > 0 {
		scale := focal / it.dist
		if scale <= 0 {
			return 0, -1, 0, -1, false
		}
		dstW := float64(it.spriteTex.Width) * scale
		dstH := float64(it.spriteTex.Height) * scale
		dstX := it.sx - float64(it.spriteTex.OffsetX)*scale
		dstY := it.sy - float64(it.spriteTex.OffsetY)*scale
		x0 = int(math.Floor(dstX))
		y0 = int(math.Floor(dstY))
		x1 = int(math.Ceil(dstX+dstW)) - 1
		y1 = int(math.Ceil(dstY+dstH)) - 1
	} else {
		if it.r <= 0 {
			return 0, -1, 0, -1, false
		}
		x0 = int(math.Floor(it.sx - it.r))
		x1 = int(math.Ceil(it.sx + it.r))
		y0 = int(math.Floor(it.sy - it.r))
		y1 = int(math.Ceil(it.sy + it.r))
	}
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
	if x0 > x1 || y0 > y1 {
		return x0, x1, y0, y1, false
	}
	return x0, x1, y0, y1, true
}

func spriteOccludedDepthQAtCore(depthPix, depthPlanePix []uint32, stamp, depthQ, planeBiasQ uint16, idx int) bool {
	if len(depthPix) == 0 {
		return false
	}
	if idx < 0 || idx >= len(depthPix) {
		return true
	}
	// Walls and already-drawn sprites occlude strictly.
	if cur := depthPix[idx]; unpackDepthStamp(cur) == stamp && depthQ > unpackDepthQ(cur) {
		return true
	}
	// Floor/ceiling depth is used with bias because billboard depth is constant
	// across Y while plane depth varies by scanline.
	if idx < len(depthPlanePix) {
		if cur := depthPlanePix[idx]; unpackDepthStamp(cur) == stamp {
			threshold := addDepthQ(unpackDepthQ(cur), planeBiasQ)
			if depthQ > threshold {
				return true
			}
		}
	}
	return false
}

func (g *game) drawBasicWallColumnTextured(x, y0, y1 int, depth, texU, texMid, focal float64, tex WallTexture, shadeMul, doomRow int) {
	rowStridePix := g.viewW
	pixI := y0*rowStridePix + x
	pix32 := g.wallPix32
	texIndexed := tex.Indexed
	texIndexedCol := tex.IndexedColMajor
	indexedReady := len(texIndexed) == tex.Width*tex.Height && (wallShadePackedOK || doomColormapEnabled)
	tex32 := tex.RGBA32
	if len(tex32) != tex.Width*tex.Height {
		if len(tex.RGBA) != tex.Width*tex.Height*4 || len(tex.RGBA) < 4 {
			return
		}
		tex32 = unsafe.Slice((*uint32)(unsafe.Pointer(unsafe.SliceData(tex.RGBA))), len(tex.RGBA)/4)
	}
	texCol := tex.ColMajor
	useColMajor := len(texCol) == tex.Width*tex.Height
	txi := int(floorFixed(texU) >> fracBits)
	tx := 0
	if tex.Width > 0 && (tex.Width&(tex.Width-1)) == 0 {
		tx = txi & (tex.Width - 1)
	} else {
		tx = wrapIndex(txi, tex.Width)
	}
	rowScale := depth / focal
	cy := float64(g.viewH) * 0.5
	texV := texMid - ((cy - (float64(y0) + 0.5)) * rowScale)
	texVFixed := floorFixed(texV)
	texVStepFixed := floorFixed(rowScale)
	pow2H := tex.Height > 0 && (tex.Height&(tex.Height-1)) == 0
	hmask := tex.Height - 1
	colBase := tx * tex.Height
	// Dominant hot path: little-endian packed output + pretransposed column-major texture + pow2 height.
	if pixelLittleEndian && pow2H && len(texIndexedCol) == tex.Width*tex.Height && indexedReady {
		col := texIndexedCol[colBase : colBase+tex.Height]
		if doomColormapEnabled {
			rows := doomShadeRows()
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
	if pixelLittleEndian && useColMajor && pow2H {
		col := texCol[colBase : colBase+tex.Height]
		drawWallColumnTexturedLEColPow2(pix32, pixI, rowStridePix, col, texVFixed, texVStepFixed, hmask, y1-y0+1, shadeMul, doomRow)
		return
	}
	if indexedReady {
		if doomColormapEnabled {
			if len(texIndexedCol) == tex.Width*tex.Height {
				for y := y0; y <= y1; y++ {
					ty := wrapIndex(int(texVFixed>>fracBits), tex.Height)
					pix32[pixI] = shadePaletteIndexDOOMRow(texIndexedCol[colBase+ty], doomRow)
					pixI += rowStridePix
					texVFixed += texVStepFixed
				}
				return
			}
			if pow2H {
				for y := y0; y <= y1; y++ {
					ty := int((texVFixed >> fracBits) & int64(hmask))
					ti := ty*tex.Width + tx
					pix32[pixI] = shadePaletteIndexDOOMRow(texIndexed[ti], doomRow)
					pixI += rowStridePix
					texVFixed += texVStepFixed
				}
				return
			}
			for y := y0; y <= y1; y++ {
				ty := wrapIndex(int(texVFixed>>fracBits), tex.Height)
				ti := ty*tex.Width + tx
				pix32[pixI] = shadePaletteIndexDOOMRow(texIndexed[ti], doomRow)
				pixI += rowStridePix
				texVFixed += texVStepFixed
			}
			return
		}
		if len(texIndexedCol) == tex.Width*tex.Height {
			for y := y0; y <= y1; y++ {
				ty := wrapIndex(int(texVFixed>>fracBits), tex.Height)
				pix32[pixI] = shadePaletteIndexPacked(texIndexedCol[colBase+ty], uint32(shadeMul))
				pixI += rowStridePix
				texVFixed += texVStepFixed
			}
			return
		}
		if pow2H {
			for y := y0; y <= y1; y++ {
				ty := int((texVFixed >> fracBits) & int64(hmask))
				ti := ty*tex.Width + tx
				pix32[pixI] = shadePaletteIndexPacked(texIndexed[ti], uint32(shadeMul))
				pixI += rowStridePix
				texVFixed += texVStepFixed
			}
			return
		}
		for y := y0; y <= y1; y++ {
			ty := wrapIndex(int(texVFixed>>fracBits), tex.Height)
			ti := ty*tex.Width + tx
			pix32[pixI] = shadePaletteIndexPacked(texIndexed[ti], uint32(shadeMul))
			pixI += rowStridePix
			texVFixed += texVStepFixed
		}
		return
	}
	if doomColormapEnabled {
		if useColMajor {
			for y := y0; y <= y1; y++ {
				ty := wrapIndex(int(texVFixed>>fracBits), tex.Height)
				pix32[pixI] = shadePackedDOOMColormapRow(texCol[colBase+ty], doomRow)
				pixI += rowStridePix
				texVFixed += texVStepFixed
			}
			return
		}
		if pow2H {
			for y := y0; y <= y1; y++ {
				ty := int((texVFixed >> fracBits) & int64(hmask))
				ti := ty*tex.Width + tx
				pix32[pixI] = shadePackedDOOMColormapRow(tex32[ti], doomRow)
				pixI += rowStridePix
				texVFixed += texVStepFixed
			}
			return
		}
		for y := y0; y <= y1; y++ {
			ty := wrapIndex(int(texVFixed>>fracBits), tex.Height)
			ti := ty*tex.Width + tx
			pix32[pixI] = shadePackedDOOMColormapRow(tex32[ti], doomRow)
			pixI += rowStridePix
			texVFixed += texVStepFixed
		}
		return
	}
	if shadeMul == 256 {
		if useColMajor {
			for y := y0; y <= y1; y++ {
				ty := wrapIndex(int(texVFixed>>fracBits), tex.Height)
				pix32[pixI] = texCol[colBase+ty] | pixelOpaqueA
				pixI += rowStridePix
				texVFixed += texVStepFixed
			}
			return
		}
		if pow2H {
			for y := y0; y <= y1; y++ {
				ty := int((texVFixed >> fracBits) & int64(hmask))
				ti := ty*tex.Width + tx
				pix32[pixI] = tex32[ti] | pixelOpaqueA
				pixI += rowStridePix
				texVFixed += texVStepFixed
			}
			return
		}
		for y := y0; y <= y1; y++ {
			ty := wrapIndex(int(texVFixed>>fracBits), tex.Height)
			ti := ty*tex.Width + tx
			pix32[pixI] = tex32[ti] | pixelOpaqueA
			pixI += rowStridePix
			texVFixed += texVStepFixed
		}
		return
	}
	shadeMulU := uint32(shadeMul)
	if pixelLittleEndian {
		if useColMajor {
			for y := y0; y <= y1; y++ {
				ty := wrapIndex(int(texVFixed>>fracBits), tex.Height)
				src := texCol[colBase+ty]
				rb := ((src & 0x00FF00FF) * shadeMulU) >> 8
				gg := ((src & 0x0000FF00) * shadeMulU) >> 8
				pix32[pixI] = pixelOpaqueA | (rb & 0x00FF00FF) | (gg & 0x0000FF00)
				pixI += rowStridePix
				texVFixed += texVStepFixed
			}
			return
		}
		if pow2H {
			for y := y0; y <= y1; y++ {
				ty := int((texVFixed >> fracBits) & int64(hmask))
				ti := ty*tex.Width + tx
				src := tex32[ti]
				rb := ((src & 0x00FF00FF) * shadeMulU) >> 8
				gg := ((src & 0x0000FF00) * shadeMulU) >> 8
				pix32[pixI] = pixelOpaqueA | (rb & 0x00FF00FF) | (gg & 0x0000FF00)
				pixI += rowStridePix
				texVFixed += texVStepFixed
			}
			return
		}
		for y := y0; y <= y1; y++ {
			ty := wrapIndex(int(texVFixed>>fracBits), tex.Height)
			ti := ty*tex.Width + tx
			src := tex32[ti]
			rb := ((src & 0x00FF00FF) * shadeMulU) >> 8
			gg := ((src & 0x0000FF00) * shadeMulU) >> 8
			pix32[pixI] = pixelOpaqueA | (rb & 0x00FF00FF) | (gg & 0x0000FF00)
			pixI += rowStridePix
			texVFixed += texVStepFixed
		}
		return
	}
	if useColMajor {
		if pow2H {
			for y := y0; y <= y1; y++ {
				ty := int((texVFixed >> fracBits) & int64(hmask))
				pix32[pixI] = shadePackedRGBA(texCol[colBase+ty], shadeMulU)
				pixI += rowStridePix
				texVFixed += texVStepFixed
			}
			return
		}
		for y := y0; y <= y1; y++ {
			ty := wrapIndex(int(texVFixed>>fracBits), tex.Height)
			pix32[pixI] = shadePackedRGBA(texCol[colBase+ty], shadeMulU)
			pixI += rowStridePix
			texVFixed += texVStepFixed
		}
		return
	}
	if pow2H {
		for y := y0; y <= y1; y++ {
			ty := int((texVFixed >> fracBits) & int64(hmask))
			ti := ty*tex.Width + tx
			pix32[pixI] = shadePackedRGBA(tex32[ti], shadeMulU)
			pixI += rowStridePix
			texVFixed += texVStepFixed
		}
		return
	}
	for y := y0; y <= y1; y++ {
		ty := wrapIndex(int(texVFixed>>fracBits), tex.Height)
		ti := ty*tex.Width + tx
		pix32[pixI] = shadePackedRGBA(tex32[ti], shadeMulU)
		pixI += rowStridePix
		texVFixed += texVStepFixed
	}
}

func (g *game) drawBasicWallColumnTexturedMasked(x, y0, y1 int, depth, texU, texMid, focal float64, tex WallTexture, shadeMul, doomRow int) {
	if x < 0 || x >= g.viewW || y0 > y1 {
		return
	}
	if y0 < 0 {
		y0 = 0
	}
	if y1 >= g.viewH {
		y1 = g.viewH - 1
	}
	if y0 > y1 || tex.Width <= 0 || tex.Height <= 0 {
		return
	}
	rowStridePix := g.viewW
	pixI := y0*rowStridePix + x
	pix32 := g.wallPix32
	texRGBA := tex.RGBA
	hasRGBA := len(texRGBA) == tex.Width*tex.Height*4
	tex32 := tex.RGBA32
	if len(tex32) != tex.Width*tex.Height {
		if len(tex.RGBA) != tex.Width*tex.Height*4 || len(tex.RGBA) < 4 {
			return
		}
		tex32 = unsafe.Slice((*uint32)(unsafe.Pointer(unsafe.SliceData(tex.RGBA))), len(tex.RGBA)/4)
	}
	txi := int(floorFixed(texU) >> fracBits)
	tx := 0
	if tex.Width > 0 && (tex.Width&(tex.Width-1)) == 0 {
		tx = txi & (tex.Width - 1)
	} else {
		tx = wrapIndex(txi, tex.Width)
	}
	rowScale := depth / focal
	cy := float64(g.viewH) * 0.5
	texV := texMid - ((cy - (float64(y0) + 0.5)) * rowScale)
	texVFixed := floorFixed(texV)
	texVStepFixed := floorFixed(rowScale)
	stamp := g.depthFrameStamp
	depthQ := encodeDepthQ(depth)
	depthPacked := packDepthStamped(depthQ, stamp)
	shadeMulU := uint32(shadeMul)
	for y := y0; y <= y1; y++ {
		ty := wrapIndex(int(texVFixed>>fracBits), tex.Height)
		ti := ty*tex.Width + tx
		src := tex32[ti]
		opaque := ((src >> pixelAShift) & 0xFF) != 0
		if hasRGBA {
			opaque = texRGBA[ti*4+3] != 0
		}
		if opaque && !g.spriteOccludedDepthQAt(g.depthPix3D, g.depthPlanePix3D, stamp, depthQ, 0, pixI) {
			if doomColormapEnabled {
				pix32[pixI] = shadePackedDOOMColormapRow(src, doomRow)
			} else {
				pix32[pixI] = shadePackedRGBA(src, shadeMulU)
			}
			g.setDepthPixelEncoded(pixI, depthPacked)
		}
		pixI += rowStridePix
		texVFixed += texVStepFixed
	}
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

func encodeDepthBiasQ(bias float64) uint16 {
	if bias <= 0 {
		return 0
	}
	scaled := bias * depthQuantScale
	q := int(scaled)
	if float64(q) < scaled {
		q++
	}
	if q <= 0 {
		return 0
	}
	if q >= 0xFFFF {
		return 0xFFFF
	}
	return uint16(q)
}

func addDepthQ(a, b uint16) uint16 {
	sum := uint32(a) + uint32(b)
	if sum >= 0xFFFF {
		return 0xFFFF
	}
	return uint16(sum)
}

func packDepthStamped(depth, stamp uint16) uint32 {
	return (uint32(stamp) << 16) | uint32(depth)
}

func unpackDepthStamp(v uint32) uint16 {
	return uint16(v >> 16)
}

func unpackDepthQ(v uint32) uint16 {
	return uint16(v & 0xFFFF)
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
	rows := doomShadeRows()
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
	rows := doomShadeRows()
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
	if g == nil || !g.opts.SourcePortMode {
		return shadePackedDOOMRowOrLUT(src, 6)
	}
	if doomColormapEnabled {
		return shadePackedDOOMColormapRow(src, 6)
	}
	if !doomLightingEnabled {
		return src | pixelOpaqueA
	}
	mul := doomShadeMulFromRow(6) + 18
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
	src := g.wallPix32[srcI]
	if src == 0 {
		src = packRGBA(0, 0, 0)
	}
	g.writeWallPixel(i, g.shadePackedSpectreFuzz(src))
}

func (g *game) beginSourcePortSpectreFuzzFrame(now time.Time) {
	if g == nil || !g.opts.SourcePortMode || len(doomFuzzOffsets) == 0 {
		return
	}
	if g.spectreFuzzStamp.IsZero() {
		g.spectreFuzzStamp = now
	}
	dt := now.Sub(g.spectreFuzzStamp).Seconds()
	g.spectreFuzzStamp = now
	g.spectreFuzzAccum += dt * float64(doomTicsPerSecond)
	steps := int(g.spectreFuzzAccum)
	if steps > 0 {
		g.spectreFuzzAccum -= float64(steps)
		g.spectreFuzzPos = (g.spectreFuzzPos + steps) % len(doomFuzzOffsets)
	}
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

func spritePixels32(tex WallTexture) ([]uint32, bool) {
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

func spriteIndexedPixels(tex WallTexture) ([]byte, []byte, bool) {
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
	if len(tex.Indexed) == n && len(tex.OpaqueMask) == n {
		return tex, true
	}
	src32, ok := spritePixels32(tex)
	if !ok || len(src32) != n {
		return tex, false
	}
	indexed := make([]byte, n)
	mask := make([]byte, n)
	for i, p := range src32 {
		if ((p >> pixelAShift) & 0xFF) == 0 {
			continue
		}
		mask[i] = 1
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
	spriteOpaqueRectMaxCount       = 4
	spriteOpaqueRectMinExtraPixels = 8
	spriteOpaqueRectExtraDivisor   = 64
	spriteOpaqueRectExpectedScale  = 6
	spriteOpaqueRectMinScreenGain  = 384
)

func spriteOpaqueRectArea(rect spriteOpaqueRect) int {
	return (int(rect.maxX) - int(rect.minX) + 1) * (int(rect.maxY) - int(rect.minY) + 1)
}

func keepSpriteOpaqueRect(area, opaquePixels, coveredPixels, rectIndex int) bool {
	if area <= 0 {
		return false
	}
	if rectIndex == 0 {
		return true
	}
	minArea := max(spriteOpaqueRectMinExtraPixels, opaquePixels/spriteOpaqueRectExtraDivisor)
	if area < minArea {
		return false
	}
	if area*spriteOpaqueRectExpectedScale*spriteOpaqueRectExpectedScale < spriteOpaqueRectMinScreenGain {
		return false
	}
	return true
}

func largestOpaqueRect(mask []bool, w, h int) (spriteOpaqueRect, int, bool) {
	if w <= 0 || h <= 0 || len(mask) != w*h {
		return spriteOpaqueRect{}, 0, false
	}
	heights := make([]int, w)
	best := spriteOpaqueRect{}
	bestArea := 0
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if mask[y*w+x] {
				heights[x]++
			} else {
				heights[x] = 0
			}
		}
		stack := make([]int, 0, w+1)
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
					best = spriteOpaqueRect{
						minX: int16(left),
						maxX: int16(right),
						minY: int16(y - height + 1),
						maxY: int16(y),
					}
				}
			}
			stack = append(stack, i)
		}
	}
	if bestArea <= 0 {
		return spriteOpaqueRect{}, 0, false
	}
	return best, bestArea, true
}

func buildSpriteOpaqueRects(pix32 []uint32, w, h int) []spriteOpaqueRect {
	if w <= 0 || h <= 0 || len(pix32) != w*h {
		return nil
	}
	mask := make([]bool, w*h)
	opaquePixels := 0
	for i, p := range pix32 {
		if ((p >> pixelAShift) & 0xFF) != 0xFF {
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
		rect, area, ok := largestOpaqueRect(mask, w, h)
		if !ok {
			break
		}
		if !keepSpriteOpaqueRect(area, opaquePixels, coveredPixels, len(rects)) {
			break
		}
		rects = append(rects, rect)
		coveredPixels += area
		for y := int(rect.minY); y <= int(rect.maxY); y++ {
			row := y * w
			for x := int(rect.minX); x <= int(rect.maxX); x++ {
				mask[row+x] = false
			}
		}
	}
	return rects
}

func (g *game) spriteOpaqueShapeForKey(key string, tex WallTexture) (spriteOpaqueShape, bool) {
	if key == "" {
		return spriteOpaqueShape{}, false
	}
	if g.spriteOpaqueShapeCache == nil {
		g.spriteOpaqueShapeCache = make(map[string]spriteOpaqueShape, 128)
	}
	if shape, ok := g.spriteOpaqueShapeCache[key]; ok {
		return shape, true
	}
	pix32, ok := spritePixels32(tex)
	if !ok {
		return spriteOpaqueShape{}, false
	}
	shape := spriteOpaqueShape{
		bounds: spriteOpaqueBounds{minX: tex.Width, minY: tex.Height, maxX: -1, maxY: -1},
		rowMin: make([]int16, tex.Height),
		rowMax: make([]int16, tex.Height),
		rects:  buildSpriteOpaqueRects(pix32, tex.Width, tex.Height),
	}
	for y := 0; y < tex.Height; y++ {
		shape.rowMin[y] = int16(tex.Width)
		shape.rowMax[y] = -1
	}
	for y := 0; y < tex.Height; y++ {
		row := y * tex.Width
		for x := 0; x < tex.Width; x++ {
			if ((pix32[row+x] >> pixelAShift) & 0xFF) == 0 {
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

func (g *game) ensureSpriteColumnScratch(n int) []uint32 {
	if n <= 0 {
		return nil
	}
	if cap(g.spriteColumnScratch) < n {
		g.spriteColumnScratch = make([]uint32, n)
	} else {
		g.spriteColumnScratch = g.spriteColumnScratch[:n]
		clear(g.spriteColumnScratch)
	}
	return g.spriteColumnScratch
}

func (g *game) ensureSpriteSourceColumnScratch(n int) []uint32 {
	if n <= 0 {
		return nil
	}
	if cap(g.spriteSourceColumnScratch) < n {
		g.spriteSourceColumnScratch = make([]uint32, n)
	} else {
		g.spriteSourceColumnScratch = g.spriteSourceColumnScratch[:n]
		clear(g.spriteSourceColumnScratch)
	}
	return g.spriteSourceColumnScratch
}

func (g *game) ensureBillboardColumnRunsScratch(n int) []billboardColumnRun {
	if n <= 0 {
		return nil
	}
	if cap(g.billboardColumnRunsScratch) < n {
		g.billboardColumnRunsScratch = make([]billboardColumnRun, n)
	} else {
		g.billboardColumnRunsScratch = g.billboardColumnRunsScratch[:n]
	}
	return g.billboardColumnRunsScratch
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

func (g *game) ensureProjectileItemsScratch(n int) []projectedProjectileItem {
	if n <= 0 {
		return nil
	}
	if cap(g.projectileItemsScratch) < n {
		g.projectileItemsScratch = make([]projectedProjectileItem, 0, n)
	}
	g.projectileItemsScratch = g.projectileItemsScratch[:0]
	return g.projectileItemsScratch
}

func (g *game) ensureMonsterItemsScratch(n int) []projectedMonsterItem {
	if n <= 0 {
		return nil
	}
	if cap(g.monsterItemsScratch) < n {
		g.monsterItemsScratch = make([]projectedMonsterItem, 0, n)
	}
	g.monsterItemsScratch = g.monsterItemsScratch[:0]
	return g.monsterItemsScratch
}

func (g *game) ensureThingItemsScratch(n int) []projectedThingItem {
	if n <= 0 {
		return nil
	}
	if cap(g.thingItemsScratch) < n {
		g.thingItemsScratch = make([]projectedThingItem, 0, n)
	}
	g.thingItemsScratch = g.thingItemsScratch[:0]
	return g.thingItemsScratch
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

func (g *game) drawBillboardSpriteRowsDirect(src32 []uint32, srcIndexed, srcMask []byte, tw int, txLUT, tyLUT []int, x0, x1, y0, y1 int, shadeMul uint32) {
	if g == nil || x0 > x1 || y0 > y1 || tw <= 0 {
		return
	}
	useIndexed := len(srcIndexed) == len(srcMask) && len(srcIndexed) >= tw
	viewW := g.viewW
	pix32 := g.wallPix32
	fullbright := shadeMul >= 256
	var shadeRow []uint32
	if useIndexed && wallShadePackedOK {
		if shadeMul > 256 {
			shadeMul = 256
		}
		shadeRow = wallShadePackedLUT[shadeMul][:]
	}
	useShadeRow := len(shadeRow) == 256
	height := y1 - y0 + 1
	th := 0
	if useIndexed && tw > 0 && len(srcIndexed) >= tw {
		th = len(srcIndexed) / tw
	} else if len(src32) >= tw && tw > 0 {
		th = len(src32) / tw
	}
	if th <= 0 {
		return
	}
	sourceColumnScratch := g.ensureSpriteSourceColumnScratch(th)
	columnScratch := g.ensureSpriteColumnScratch(height)
	runs := g.ensureBillboardColumnRunsScratch(len(txLUT))
	runCount := 0
	for x := x0; x <= x1; {
		tx := txLUT[x-x0]
		runEnd := x
		for runEnd+1 <= x1 && txLUT[runEnd+1-x0] == tx {
			runEnd++
		}
		runs[runCount] = billboardColumnRun{x0: x, x1: runEnd, tx: tx}
		runCount++
		x = runEnd + 1
	}
	for i := 0; i < runCount; i++ {
		run := runs[i]
		runLen := run.x1 - run.x0 + 1
		clear(sourceColumnScratch)
		for ty := 0; ty < th; ty++ {
			s := ty*tw + run.tx
			if useIndexed {
				if srcMask[s] == 0 {
					continue
				}
				if useShadeRow {
					sourceColumnScratch[ty] = shadeRow[srcIndexed[s]]
				} else {
					sourceColumnScratch[ty] = shadePaletteIndexPacked(srcIndexed[s], shadeMul)
				}
				continue
			}
			p := src32[s]
			if ((p >> pixelAShift) & 0xFF) == 0 {
				continue
			}
			if fullbright {
				sourceColumnScratch[ty] = p | pixelOpaqueA
			} else {
				sourceColumnScratch[ty] = shadePackedRGBA(p, shadeMul)
			}
		}
		if runLen == 1 {
			for y := y0; y <= y1; y++ {
				if p := sourceColumnScratch[tyLUT[y-y0]]; p != 0 {
					pix32[y*viewW+run.x0] = p
				}
			}
			continue
		}
		clear(columnScratch)
		for y := y0; y <= y1; y++ {
			if p := sourceColumnScratch[tyLUT[y-y0]]; p != 0 {
				columnScratch[y-y0] = p
			}
		}
		for dx := run.x0; dx <= run.x1; dx++ {
			pixI := y0*viewW + dx
			for i := 0; i < height; i++ {
				if p := columnScratch[i]; p != 0 {
					pix32[pixI] = p
				}
				pixI += viewW
			}
		}
	}
}

func (g *game) drawBillboardRowSpans(row, ty, tw, x0 int, txLUT []int, spans []solidSpan, src32 []uint32, srcIndexed, srcMask []byte, shadeMul uint32, shadeRow []uint32) {
	useIndexed := len(srcIndexed) == len(srcMask) && len(srcIndexed) == tw*max(ty+1, 1)
	useShadeRow := len(shadeRow) == 256
	fullbright := shadeMul >= 256
	base := ty * tw
	for _, sp := range spans {
		if useIndexed {
			for x := sp.l; x <= sp.r; {
				if x+1 <= sp.r {
					i0 := row + x
					i1 := i0 + 1
					s0 := base + txLUT[x-x0]
					s1 := base + txLUT[x+1-x0]
					a0 := srcMask[s0] != 0
					a1 := srcMask[s1] != 0
					if a0 && a1 {
						if useShadeRow {
							g.writeWallPixelPair(i0, shadeRow[srcIndexed[s0]], shadeRow[srcIndexed[s1]])
						} else {
							g.writeWallPixelPair(i0, shadePaletteIndexPacked(srcIndexed[s0], shadeMul), shadePaletteIndexPacked(srcIndexed[s1], shadeMul))
						}
						x += 2
						continue
					}
					if a0 {
						if useShadeRow {
							g.writeWallPixel(i0, shadeRow[srcIndexed[s0]])
						} else {
							g.writeWallPixel(i0, shadePaletteIndexPacked(srcIndexed[s0], shadeMul))
						}
					}
					if a1 {
						if useShadeRow {
							g.writeWallPixel(i1, shadeRow[srcIndexed[s1]])
						} else {
							g.writeWallPixel(i1, shadePaletteIndexPacked(srcIndexed[s1], shadeMul))
						}
					}
					x += 2
					continue
				}
				i := row + x
				s := base + txLUT[x-x0]
				if srcMask[s] != 0 {
					if useShadeRow {
						g.writeWallPixel(i, shadeRow[srcIndexed[s]])
					} else {
						g.writeWallPixel(i, shadePaletteIndexPacked(srcIndexed[s], shadeMul))
					}
				}
				x++
			}
			continue
		}
		for x := sp.l; x <= sp.r; {
			if x+1 <= sp.r {
				i0 := row + x
				i1 := i0 + 1
				p0 := src32[base+txLUT[x-x0]]
				p1 := src32[base+txLUT[x+1-x0]]
				a0 := ((p0 >> pixelAShift) & 0xFF) != 0
				a1 := ((p1 >> pixelAShift) & 0xFF) != 0
				if a0 && a1 {
					if fullbright {
						g.writeWallPixelPair(i0, p0|pixelOpaqueA, p1|pixelOpaqueA)
					} else {
						g.writeWallPixelPair(i0, shadePackedRGBA(p0, shadeMul), shadePackedRGBA(p1, shadeMul))
					}
					x += 2
					continue
				}
				if a0 {
					if fullbright {
						g.writeWallPixel(i0, p0|pixelOpaqueA)
					} else {
						g.writeWallPixel(i0, shadePackedRGBA(p0, shadeMul))
					}
				}
				if a1 {
					if fullbright {
						g.writeWallPixel(i1, p1|pixelOpaqueA)
					} else {
						g.writeWallPixel(i1, shadePackedRGBA(p1, shadeMul))
					}
				}
				x += 2
				continue
			}
			i := row + x
			p := src32[base+txLUT[x-x0]]
			if ((p >> pixelAShift) & 0xFF) != 0 {
				if fullbright {
					g.writeWallPixel(i, p|pixelOpaqueA)
				} else {
					g.writeWallPixel(i, shadePackedRGBA(p, shadeMul))
				}
			}
			x++
		}
	}
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

func (g *game) drawMaskedMidSegs(focal float64) {
	if len(g.maskedMidSegsScratch) == 0 {
		return
	}
	sort.Slice(g.maskedMidSegsScratch, func(i, j int) bool {
		return g.maskedMidSegsScratch[i].dist > g.maskedMidSegsScratch[j].dist
	})
	halfH := float64(g.viewH) * 0.5
	for _, ms := range g.maskedMidSegsScratch {
		if ms.tex.Width <= 0 || ms.tex.Height <= 0 {
			continue
		}
		for x := ms.x0; x <= ms.x1; x++ {
			t := 0.0
			if math.Abs(ms.sx2-ms.sx1) > 1e-9 {
				t = (float64(x) - ms.sx1) / (ms.sx2 - ms.sx1)
			}
			if t < 0 {
				t = 0
			}
			if t > 1 {
				t = 1
			}
			invF := ms.invF1 + (ms.invF2-ms.invF1)*t
			if invF <= 0 {
				continue
			}
			f := 1.0 / invF
			if f <= 0 {
				continue
			}
			texU := (ms.uOverF1 + (ms.uOverF2-ms.uOverF1)*t) * f
			texU += ms.texUOff
			y0 := int(math.Ceil(halfH - (ms.worldHigh/f)*focal))
			y1 := int(math.Floor(halfH - (ms.worldLow/f)*focal))
			if y0 > y1 {
				continue
			}
			shadeMul := sectorDistanceShadeMul(ms.light, ms.dist, doomLightingEnabled)
			doomRow := 0
			if doomLightingEnabled {
				doomRow = doomWallLightRow(ms.light, ms.lightBias, f, focal)
				if !doomColormapEnabled {
					shadeMul = doomShadeMulFromRowF(doomWallLightRowF(ms.light, ms.lightBias, f, focal))
				}
			}
			g.drawBasicWallColumnTexturedMasked(x, y0, y1, f, texU, ms.texMid, focal, ms.tex, shadeMul, doomRow)
		}
	}
}

func (g *game) buildMaskedMidClipColumns(focal float64) {
	if g == nil || focal <= 0 || len(g.maskedMidSegsScratch) == 0 || len(g.maskedClipCols) != g.viewW {
		return
	}
	halfH := float64(g.viewH) * 0.5
	for _, ms := range g.maskedMidSegsScratch {
		if ms.x0 > ms.x1 || math.Abs(ms.sx2-ms.sx1) < 1e-9 {
			continue
		}
		x0 := ms.x0
		x1 := ms.x1
		if x0 < 0 {
			x0 = 0
		}
		if x1 >= g.viewW {
			x1 = g.viewW - 1
		}
		for x := x0; x <= x1; x++ {
			t := (float64(x) - ms.sx1) / (ms.sx2 - ms.sx1)
			if t < 0 {
				t = 0
			}
			if t > 1 {
				t = 1
			}
			invF := ms.invF1 + (ms.invF2-ms.invF1)*t
			if invF <= 0 {
				continue
			}
			f := 1.0 / invF
			if f <= 0 {
				continue
			}
			y0 := int(math.Ceil(halfH - (ms.worldHigh/f)*focal))
			y1 := int(math.Floor(halfH - (ms.worldLow/f)*focal))
			if y0 < 0 {
				y0 = 0
			}
			if y1 >= g.viewH {
				y1 = g.viewH - 1
			}
			if y1 < y0 {
				continue
			}
			g.maskedClipCols[x] = append(g.maskedClipCols[x], maskedClipSpan{
				y0:     int16(y0),
				y1:     int16(y1),
				depthQ: encodeDepthQ(f),
			})
		}
	}
}

func wallSpecialScrollXOffset(special uint16, worldTic int) float64 {
	// Doom linedef special 48: first-column wall scroll.
	if special == 48 {
		return float64(worldTic)
	}
	return 0
}

func drawWallColumnTexturedLEColPow2(pix32 []uint32, pixI, rowStridePix int, col []uint32, texVFixed, texVStepFixed int64, hmask, count, shadeMul, doomRow int) {
	if doomColormapEnabled {
		for ; count > 0; count-- {
			ty := int((texVFixed >> fracBits) & int64(hmask))
			pix32[pixI] = shadePackedDOOMColormapRow(col[ty], doomRow)
			pixI += rowStridePix
			texVFixed += texVStepFixed
		}
		return
	}
	if shadeMul == 256 {
		for ; count > 0; count-- {
			ty := int((texVFixed >> fracBits) & int64(hmask))
			pix32[pixI] = col[ty] | pixelOpaqueA
			pixI += rowStridePix
			texVFixed += texVStepFixed
		}
		return
	}
	shadeMulU := uint32(shadeMul)
	for ; count > 0; count-- {
		ty := int((texVFixed >> fracBits) & int64(hmask))
		src := col[ty]
		rb := ((src & 0x00FF00FF) * shadeMulU) >> 8
		gg := ((src & 0x0000FF00) * shadeMulU) >> 8
		pix32[pixI] = pixelOpaqueA | (rb & 0x00FF00FF) | (gg & 0x0000FF00)
		pixI += rowStridePix
		texVFixed += texVStepFixed
	}
}

func drawWallColumnTexturedIndexedLEColPow2Row(pix32 []uint32, pixI, rowStridePix int, col []byte, texVFixed, texVStepFixed int64, hmask, count int, row []uint32) {
	const fracMask = fracUnit - 1
	ty := int(texVFixed >> fracBits)
	frac := int(texVFixed & fracMask)
	stepInt := int(texVStepFixed >> fracBits)
	stepFrac := int(texVStepFixed & fracMask)
	if texVStepFixed > -fracUnit && texVStepFixed < fracUnit {
		for count > 0 {
			ty0 := ty & hmask
			p := row[col[ty0]]
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
				if (ty & hmask) != ty0 {
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
		pixI += rowStridePix * 2
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
		clear(wallShadePackedLUT[:])
		return
	}
	paletteIndexByPacked = make(map[uint32]uint8, 256)
	for mul := 0; mul <= 256; mul++ {
		row := &wallShadePackedLUT[mul]
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
			row[idx] = pixelOpaqueA | (r << pixelRShift) | (g << pixelGShift) | (b << pixelBShift)
			if mul == 256 {
				paletteIndexByPacked[row[idx]] = uint8(idx)
			}
		}
	}
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
	if !enableColormapRemap || len(paletteRGBA) < 256*4 {
		return
	}
	doomColormapRGBA = make([]uint32, rows*256)
	for r := 0; r < rows; r++ {
		rowBase := r * 256
		for i := 0; i < 256; i++ {
			pi := int(colorMap[rowBase+i]) * 4
			if pi+3 >= len(paletteRGBA) {
				doomColormapRGBA[rowBase+i] = packRGBA(0, 0, 0)
				continue
			}
			doomColormapRGBA[rowBase+i] = packRGBA(paletteRGBA[pi], paletteRGBA[pi+1], paletteRGBA[pi+2])
		}
	}
	doomPalIndexLUT32 = buildPaletteIndexLUT32(paletteRGBA)
	doomColormapEnabled = len(doomPalIndexLUT32) == 32*32*32
}

func disableDoomLighting() {
	fullbrightNoLighting = true
	doomLightingEnabled = false
	doomSectorLighting = false
	doomColormapEnabled = false
	doomColormapRows = 0
	doomRowShadeMulLUT = nil
	doomColormapRGBA = nil
	doomPalIndexLUT32 = nil
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
	spansByPlane, _, _, hasSky := g.buildPlaneSpansParallel(planes, h)
	cx := float64(w) * 0.5
	cy := float64(h) * 0.5
	if g.planeFlatCache32Scratch == nil {
		g.planeFlatCache32Scratch = make(map[string][]uint32, max(len(planes), 64))
	} else {
		clear(g.planeFlatCache32Scratch)
	}
	if g.planeFlatCacheIndexedScratch == nil {
		g.planeFlatCacheIndexedScratch = make(map[string][]byte, max(len(planes), 64))
	} else {
		clear(g.planeFlatCacheIndexedScratch)
	}
	flatCache32 := g.planeFlatCache32Scratch
	flatCacheIndexed := g.planeFlatCacheIndexedScratch
	planeFBPacked, planeFlatTex32, planeFlatTexIndexed, planeFlatReady := g.ensurePlaneRenderScratch(len(planes))
	skyTexKey := ""
	skyTex, skyTexOK := WallTexture{}, false
	skyTex32 := []uint32(nil)
	skyColU := make([]int, 0)
	skyRowV := make([]int, 0)
	if hasSky {
		skyTexKey, skyTex, skyTexOK = skyTextureEntryForMap(g.m.Name, g.opts.WallTexBank)
		if skyTexOK {
			camAng := math.Atan2(sa, ca)
			skyTexH := effectiveSkyTexHeight(skyTex)
			skyColU, skyRowV = g.buildSkyLookupParallel(w, h, focal, camAng, skyTex.Width, skyTexH)
		}
	}
	skyTexReady := skyTexOK &&
		len(skyColU) == w &&
		len(skyRowV) == h
	if skyTexReady {
		skyTex32 = skyTex.RGBA32
		if len(skyTex32) != skyTex.Width*skyTex.Height {
			if len(skyTex.RGBA) != skyTex.Width*skyTex.Height*4 || len(skyTex.RGBA) < 4 {
				skyTexReady = false
			} else {
				skyTex32 = unsafe.Slice((*uint32)(unsafe.Pointer(unsafe.SliceData(skyTex.RGBA))), len(skyTex.RGBA)/4)
			}
		}
	}
	skyLayerEnabled := false
	if skyTexReady {
		camAng := math.Atan2(sa, ca)
		skyLayerEnabled = g.enableSkyLayerFrame(camAng, focal, skyTexKey, skyTex, effectiveSkyTexHeight(skyTex))
	}
	for planeIdx, pl := range planes {
		key := pl.key
		fb := ceilFallback
		if key.floor {
			fb = floorFallback
		}
		planeFBPacked[planeIdx] = packRGBA(fb.R, fb.G, fb.B)
		if key.sky || key.fallback {
			continue
		}
		tex32, ok := flatCache32[key.flat]
		if !ok {
			tex, _ := g.flatRGBAResolvedKey(key.flat)
			if len(tex) == 64*64*4 {
				tex32 = unsafe.Slice((*uint32)(unsafe.Pointer(unsafe.SliceData(tex))), len(tex)/4)
			}
			flatCache32[key.flat] = tex32
		}
		indexed, ok := flatCacheIndexed[key.flat]
		if !ok {
			indexed, _ = g.flatIndexedResolvedKey(key.flat)
			flatCacheIndexed[key.flat] = indexed
		}
		if len(tex32) == 64*64 {
			planeFlatTex32[planeIdx] = tex32
			planeFlatTexIndexed[planeIdx] = indexed
			planeFlatReady[planeIdx] = true
		}
	}

	renderRows := func(yStart, yEnd int) {
		for planeIdx, pl := range planes {
			spans := spansByPlane[planeIdx]
			if len(spans) == 0 {
				continue
			}
			key := pl.key
			fbPacked := planeFBPacked[planeIdx]
			tex32 := planeFlatTex32[planeIdx]
			texIndexed := planeFlatTexIndexed[planeIdx]
			flatTexReady := planeFlatReady[planeIdx]
			flatIndexedReady := len(texIndexed) == 64*64 && (wallShadePackedOK || doomColormapEnabled)
			for _, sp := range spans {
				if sp.y < yStart || sp.y >= yEnd || sp.y < 0 || sp.y >= h {
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
				rowPix := sp.y * w
				if key.sky {
					if skyLayerEnabled {
						clear(pix32[rowPix+x1 : rowPix+x2+1])
						continue
					}
					pixI := rowPix + x1
					if skyTexReady {
						v := skyRowV[sp.y]
						x := x1
						for ; x+1 <= x2; x += 2 {
							u0 := skyColU[x]
							u1 := skyColU[x+1]
							ti0 := v*skyTex.Width + u0
							ti1 := v*skyTex.Width + u1
							pix32[pixI] = skyTex32[ti0]
							pix32[pixI+1] = skyTex32[ti1]
							pixI += 2
						}
						if x <= x2 {
							u := skyColU[x]
							ti := v*skyTex.Width + u
							pix32[pixI] = skyTex32[ti]
						}
					} else {
						x := x1
						for ; x+1 <= x2; x += 2 {
							pix32[pixI] = fbPacked
							pix32[pixI+1] = fbPacked
							pixI += 2
						}
						if x <= x2 {
							pix32[pixI] = fbPacked
						}
					}
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
				depthQ := encodeDepthQ(depth)
				if g.rowFullyOccludedByWallsFastDepthQ(depthQ, rowPix, x1, x2) {
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
				defaultShade := uint32(sectorLightMul(key.light))
				defaultRow := 0
				if doomLightingEnabled {
					defaultRow = doomPlaneLightRow(key.light, depth)
					if !doomColormapEnabled {
						defaultShade = uint32(doomShadeMulFromRowF(doomPlaneLightRowF(key.light, depth)))
					}
				}
				var packedShadeRow []uint32
				if !doomColormapEnabled && wallShadePackedOK {
					if defaultShade > 256 {
						defaultShade = 256
					}
					packedShadeRow = wallShadePackedLUT[defaultShade][:]
				}
				var fullbrightPackedRow []uint32
				if wallShadePackedOK {
					fullbrightPackedRow = wallShadePackedLUT[256][:]
				}
				pixI := rowPix + x1
				if !flatTexReady {
					if doomColormapEnabled {
						x := x1
						for ; x+1 <= x2; x += 2 {
							row0 := defaultRow
							wxFixed += stepWXFixed
							wyFixed += stepWYFixed
							row1 := defaultRow
							wxFixed += stepWXFixed
							wyFixed += stepWYFixed
							pix32[pixI] = shadePackedDOOMColormapRow(fbPacked, row0)
							pix32[pixI+1] = shadePackedDOOMColormapRow(fbPacked, row1)
							pixI += 2
						}
						if x <= x2 {
							pix32[pixI] = shadePackedDOOMColormapRow(fbPacked, defaultRow)
						}
						continue
					}
					if fullbrightNoLighting {
						x := x1
						for ; x+1 <= x2; x += 2 {
							wxFixed += stepWXFixed
							wyFixed += stepWYFixed
							wxFixed += stepWXFixed
							wyFixed += stepWYFixed
							pix32[pixI] = fbPacked
							pix32[pixI+1] = fbPacked
							pixI += 2
						}
						if x <= x2 {
							pix32[pixI] = fbPacked
						}
						continue
					}
					if doomLightingEnabled {
						x := x1
						for ; x+1 <= x2; x += 2 {
							wxFixed += stepWXFixed
							wyFixed += stepWYFixed
							wxFixed += stepWXFixed
							wyFixed += stepWYFixed
							if defaultShade == 256 {
								pix32[pixI] = fbPacked
								pix32[pixI+1] = fbPacked
							} else {
								pix32[pixI] = shadePackedRGBA(fbPacked, defaultShade)
								pix32[pixI+1] = shadePackedRGBA(fbPacked, defaultShade)
							}
							pixI += 2
						}
						if x <= x2 {
							if defaultShade == 256 {
								pix32[pixI] = fbPacked
							} else {
								pix32[pixI] = shadePackedRGBA(fbPacked, defaultShade)
							}
						}
						continue
					}
					x := x1
					for ; x+1 <= x2; x += 2 {
						wxFixed += stepWXFixed
						wyFixed += stepWYFixed
						wxFixed += stepWXFixed
						wyFixed += stepWYFixed
						if defaultShade == 256 {
							pix32[pixI] = fbPacked
						} else {
							pix32[pixI] = shadePackedRGBA(fbPacked, defaultShade)
						}
						if defaultShade == 256 {
							pix32[pixI+1] = fbPacked
						} else {
							pix32[pixI+1] = shadePackedRGBA(fbPacked, defaultShade)
						}
						pixI += 2
					}
					if x <= x2 {
						if defaultShade == 256 {
							pix32[pixI] = fbPacked
						} else {
							pix32[pixI] = shadePackedRGBA(fbPacked, defaultShade)
						}
					}
					continue
				}
				if doomColormapEnabled && flatIndexedReady {
					const fracMask = fracUnit - 1
					uInt := int(wxFixed >> fracBits)
					vInt := int(wyFixed >> fracBits)
					uFrac := int(wxFixed & fracMask)
					vFrac := int(wyFixed & fracMask)
					stepUInt := int(stepWXFixed >> fracBits)
					stepVInt := int(stepWYFixed >> fracBits)
					stepUFrac := int(stepWXFixed & fracMask)
					stepVFrac := int(stepWYFixed & fracMask)
					canRepeatFill := stepWXFixed > -fracUnit && stepWXFixed < fracUnit && stepWYFixed > -fracUnit && stepWYFixed < fracUnit
					if canRepeatFill {
						x := x1
						for x <= x2 {
							texIdx := ((vInt & 63) << 6) + (uInt & 63)
							packed := shadePaletteIndexDOOMRow(texIndexed[texIdx], defaultRow)
							for {
								pix32[pixI] = packed
								x++
								pixI++
								if x > x2 {
									break
								}
								uFrac += stepUFrac
								vFrac += stepVFrac
								uInt += stepUInt + (uFrac >> fracBits)
								vInt += stepVInt + (vFrac >> fracBits)
								uFrac &= fracMask
								vFrac &= fracMask
								nextTexIdx := ((vInt & 63) << 6) + (uInt & 63)
								if nextTexIdx != texIdx {
									break
								}
							}
						}
						continue
					}
					x := x1
					for ; x+1 <= x2; x += 2 {
						u0 := uInt & 63
						v0 := vInt & 63
						p0 := texIndexed[(v0<<6)+u0]
						row0 := defaultRow
						uFrac += stepUFrac
						vFrac += stepVFrac
						uInt += stepUInt + (uFrac >> fracBits)
						vInt += stepVInt + (vFrac >> fracBits)
						uFrac &= fracMask
						vFrac &= fracMask
						u1 := uInt & 63
						v1 := vInt & 63
						p1 := texIndexed[(v1<<6)+u1]
						row1 := defaultRow
						pix32[pixI] = shadePaletteIndexDOOMRow(p0, row0)
						pix32[pixI+1] = shadePaletteIndexDOOMRow(p1, row1)
						uFrac += stepUFrac
						vFrac += stepVFrac
						uInt += stepUInt + (uFrac >> fracBits)
						vInt += stepVInt + (vFrac >> fracBits)
						uFrac &= fracMask
						vFrac &= fracMask
						pixI += 2
					}
					if x <= x2 {
						u := uInt & 63
						v := vInt & 63
						pix32[pixI] = shadePaletteIndexDOOMRow(texIndexed[(v<<6)+u], defaultRow)
					}
					continue
				}
				if doomColormapEnabled {
					x := x1
					for ; x+1 <= x2; x += 2 {
						u0 := int(wxFixed>>fracBits) & 63
						v0 := int(wyFixed>>fracBits) & 63
						p0 := tex32[(v0<<6)+u0]
						row0 := defaultRow
						wxFixed += stepWXFixed
						wyFixed += stepWYFixed
						u1 := int(wxFixed>>fracBits) & 63
						v1 := int(wyFixed>>fracBits) & 63
						p1 := tex32[(v1<<6)+u1]
						row1 := defaultRow
						pix32[pixI] = shadePackedDOOMColormapRow(p0, row0)
						pix32[pixI+1] = shadePackedDOOMColormapRow(p1, row1)
						wxFixed += stepWXFixed
						wyFixed += stepWYFixed
						pixI += 2
					}
					if x <= x2 {
						u := int(wxFixed>>fracBits) & 63
						v := int(wyFixed>>fracBits) & 63
						pix32[pixI] = shadePackedDOOMColormapRow(tex32[(v<<6)+u], defaultRow)
					}
					continue
				}
				if fullbrightNoLighting {
					const fracMask = fracUnit - 1
					uInt := int(wxFixed >> fracBits)
					vInt := int(wyFixed >> fracBits)
					uFrac := int(wxFixed & fracMask)
					vFrac := int(wyFixed & fracMask)
					stepUInt := int(stepWXFixed >> fracBits)
					stepVInt := int(stepWYFixed >> fracBits)
					stepUFrac := int(stepWXFixed & fracMask)
					stepVFrac := int(stepWYFixed & fracMask)
					canRepeatFill := stepWXFixed > -fracUnit && stepWXFixed < fracUnit && stepWYFixed > -fracUnit && stepWYFixed < fracUnit
					if flatIndexedReady && canRepeatFill {
						x := x1
						for x <= x2 {
							texIdx := ((vInt & 63) << 6) + (uInt & 63)
							var packed uint32
							if len(fullbrightPackedRow) == 256 {
								packed = fullbrightPackedRow[texIndexed[texIdx]]
							} else {
								packed = shadePaletteIndexPacked(texIndexed[texIdx], 256)
							}
							for {
								pix32[pixI] = packed
								x++
								pixI++
								if x > x2 {
									break
								}
								uFrac += stepUFrac
								vFrac += stepVFrac
								uInt += stepUInt + (uFrac >> fracBits)
								vInt += stepVInt + (vFrac >> fracBits)
								uFrac &= fracMask
								vFrac &= fracMask
								nextTexIdx := ((vInt & 63) << 6) + (uInt & 63)
								if nextTexIdx != texIdx {
									break
								}
							}
						}
						continue
					}
					x := x1
					for ; x+1 <= x2; x += 2 {
						u0 := uInt & 63
						v0 := vInt & 63
						if flatIndexedReady {
							if len(fullbrightPackedRow) == 256 {
								pix32[pixI] = fullbrightPackedRow[texIndexed[(v0<<6)+u0]]
							} else {
								pix32[pixI] = shadePaletteIndexPacked(texIndexed[(v0<<6)+u0], 256)
							}
						} else {
							pix32[pixI] = tex32[(v0<<6)+u0]
						}
						uFrac += stepUFrac
						vFrac += stepVFrac
						uInt += stepUInt + (uFrac >> fracBits)
						vInt += stepVInt + (vFrac >> fracBits)
						uFrac &= fracMask
						vFrac &= fracMask
						u1 := uInt & 63
						v1 := vInt & 63
						if flatIndexedReady {
							if len(fullbrightPackedRow) == 256 {
								pix32[pixI+1] = fullbrightPackedRow[texIndexed[(v1<<6)+u1]]
							} else {
								pix32[pixI+1] = shadePaletteIndexPacked(texIndexed[(v1<<6)+u1], 256)
							}
						} else {
							pix32[pixI+1] = tex32[(v1<<6)+u1]
						}
						uFrac += stepUFrac
						vFrac += stepVFrac
						uInt += stepUInt + (uFrac >> fracBits)
						vInt += stepVInt + (vFrac >> fracBits)
						uFrac &= fracMask
						vFrac &= fracMask
						pixI += 2
					}
					if x <= x2 {
						u := uInt & 63
						v := vInt & 63
						if flatIndexedReady {
							if len(fullbrightPackedRow) == 256 {
								pix32[pixI] = fullbrightPackedRow[texIndexed[(v<<6)+u]]
							} else {
								pix32[pixI] = shadePaletteIndexPacked(texIndexed[(v<<6)+u], 256)
							}
						} else {
							pix32[pixI] = tex32[(v<<6)+u]
						}
					}
					continue
				}
				if flatIndexedReady {
					if len(packedShadeRow) == 256 {
						const fracMask = fracUnit - 1
						uInt := int(wxFixed >> fracBits)
						vInt := int(wyFixed >> fracBits)
						uFrac := int(wxFixed & fracMask)
						vFrac := int(wyFixed & fracMask)
						stepUInt := int(stepWXFixed >> fracBits)
						stepVInt := int(stepWYFixed >> fracBits)
						stepUFrac := int(stepWXFixed & fracMask)
						stepVFrac := int(stepWYFixed & fracMask)
						canRepeatFill := stepWXFixed > -fracUnit && stepWXFixed < fracUnit && stepWYFixed > -fracUnit && stepWYFixed < fracUnit
						if canRepeatFill {
							x := x1
							for x <= x2 {
								texIdx := ((vInt & 63) << 6) + (uInt & 63)
								packed := packedShadeRow[texIndexed[texIdx]]
								for {
									pix32[pixI] = packed
									x++
									pixI++
									if x > x2 {
										break
									}
									uFrac += stepUFrac
									vFrac += stepVFrac
									uInt += stepUInt + (uFrac >> fracBits)
									vInt += stepVInt + (vFrac >> fracBits)
									uFrac &= fracMask
									vFrac &= fracMask
									nextTexIdx := ((vInt & 63) << 6) + (uInt & 63)
									if nextTexIdx != texIdx {
										break
									}
								}
							}
							continue
						}
						x := x1
						for ; x+3 <= x2; x += 4 {
							u0 := uInt & 63
							v0 := vInt & 63
							p0 := texIndexed[(v0<<6)+u0]
							uFrac += stepUFrac
							vFrac += stepVFrac
							uInt += stepUInt + (uFrac >> fracBits)
							vInt += stepVInt + (vFrac >> fracBits)
							uFrac &= fracMask
							vFrac &= fracMask
							u1 := uInt & 63
							v1 := vInt & 63
							p1 := texIndexed[(v1<<6)+u1]
							uFrac += stepUFrac
							vFrac += stepVFrac
							uInt += stepUInt + (uFrac >> fracBits)
							vInt += stepVInt + (vFrac >> fracBits)
							uFrac &= fracMask
							vFrac &= fracMask
							u2 := uInt & 63
							v2 := vInt & 63
							p2 := texIndexed[(v2<<6)+u2]
							uFrac += stepUFrac
							vFrac += stepVFrac
							uInt += stepUInt + (uFrac >> fracBits)
							vInt += stepVInt + (vFrac >> fracBits)
							uFrac &= fracMask
							vFrac &= fracMask
							u3 := uInt & 63
							v3 := vInt & 63
							p3 := texIndexed[(v3<<6)+u3]
							pix32[pixI] = packedShadeRow[p0]
							pix32[pixI+1] = packedShadeRow[p1]
							pix32[pixI+2] = packedShadeRow[p2]
							pix32[pixI+3] = packedShadeRow[p3]
							uFrac += stepUFrac
							vFrac += stepVFrac
							uInt += stepUInt + (uFrac >> fracBits)
							vInt += stepVInt + (vFrac >> fracBits)
							uFrac &= fracMask
							vFrac &= fracMask
							pixI += 4
						}
						for ; x+1 <= x2; x += 2 {
							u0 := uInt & 63
							v0 := vInt & 63
							p0 := texIndexed[(v0<<6)+u0]
							uFrac += stepUFrac
							vFrac += stepVFrac
							uInt += stepUInt + (uFrac >> fracBits)
							vInt += stepVInt + (vFrac >> fracBits)
							uFrac &= fracMask
							vFrac &= fracMask
							u1 := uInt & 63
							v1 := vInt & 63
							p1 := texIndexed[(v1<<6)+u1]
							pix32[pixI] = packedShadeRow[p0]
							pix32[pixI+1] = packedShadeRow[p1]
							uFrac += stepUFrac
							vFrac += stepVFrac
							uInt += stepUInt + (uFrac >> fracBits)
							vInt += stepVInt + (vFrac >> fracBits)
							uFrac &= fracMask
							vFrac &= fracMask
							pixI += 2
						}
						if x <= x2 {
							u := uInt & 63
							v := vInt & 63
							pix32[pixI] = packedShadeRow[texIndexed[(v<<6)+u]]
						}
						continue
					}
				}
				if doomLightingEnabled {
					x := x1
					for ; x+1 <= x2; x += 2 {
						u0 := int(wxFixed>>fracBits) & 63
						v0 := int(wyFixed>>fracBits) & 63
						p0 := tex32[(v0<<6)+u0]
						wxFixed += stepWXFixed
						wyFixed += stepWYFixed
						u1 := int(wxFixed>>fracBits) & 63
						v1 := int(wyFixed>>fracBits) & 63
						p1 := tex32[(v1<<6)+u1]
						if defaultShade == 256 {
							pix32[pixI] = p0
							pix32[pixI+1] = p1
						} else {
							pix32[pixI] = shadePackedRGBA(p0, defaultShade)
							pix32[pixI+1] = shadePackedRGBA(p1, defaultShade)
						}
						wxFixed += stepWXFixed
						wyFixed += stepWYFixed
						pixI += 2
					}
					if x <= x2 {
						u := int(wxFixed>>fracBits) & 63
						v := int(wyFixed>>fracBits) & 63
						if defaultShade == 256 {
							pix32[pixI] = tex32[(v<<6)+u]
						} else {
							pix32[pixI] = shadePackedRGBA(tex32[(v<<6)+u], defaultShade)
						}
					}
					continue
				}
				x := x1
				for ; x+1 <= x2; x += 2 {
					u0 := int(wxFixed>>fracBits) & 63
					v0 := int(wyFixed>>fracBits) & 63
					p0 := tex32[(v0<<6)+u0]
					wxFixed += stepWXFixed
					wyFixed += stepWYFixed
					u1 := int(wxFixed>>fracBits) & 63
					v1 := int(wyFixed>>fracBits) & 63
					p1 := tex32[(v1<<6)+u1]
					if defaultShade == 256 {
						pix32[pixI] = p0
					} else {
						pix32[pixI] = shadePackedRGBA(p0, defaultShade)
					}
					if defaultShade == 256 {
						pix32[pixI+1] = p1
					} else {
						pix32[pixI+1] = shadePackedRGBA(p1, defaultShade)
					}
					wxFixed += stepWXFixed
					wyFixed += stepWYFixed
					pixI += 2
				}
				if x <= x2 {
					u := int(wxFixed>>fracBits) & 63
					v := int(wyFixed>>fracBits) & 63
					if defaultShade == 256 {
						pix32[pixI] = tex32[(v<<6)+u]
					} else {
						pix32[pixI] = shadePackedRGBA(tex32[(v<<6)+u], defaultShade)
					}
				}
			}
		}
	}

	renderRows(0, h)
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
				height:   int16(math.Round(planeZ)),
				light:    160,
				fallback: true,
				floor:    isFloor,
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
				k := g.resolveAnimatedFlatName(pic)
				pkey.flat = k
				pkey.fallback = len(g.opts.FlatBank[k]) != 64*64*4
				if !isFloor && isSkyFlatName(pic) {
					pkey.sky = true
					pkey.fallback = true
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
	sort.Slice(keyOrder, func(i, j int) bool {
		if keyOrder[i].floor != keyOrder[j].floor {
			return !keyOrder[i].floor
		}
		if keyOrder[i].sky != keyOrder[j].sky {
			return !keyOrder[i].sky
		}
		if keyOrder[i].height != keyOrder[j].height {
			return keyOrder[i].height < keyOrder[j].height
		}
		if keyOrder[i].light != keyOrder[j].light {
			return keyOrder[i].light < keyOrder[j].light
		}
		if keyOrder[i].flat != keyOrder[j].flat {
			return keyOrder[i].flat < keyOrder[j].flat
		}
		if keyOrder[i].fallback != keyOrder[j].fallback {
			return keyOrder[j].fallback
		}
		return false
	})
	skyTex, skyTexOK := skyTextureForMap(g.m.Name, g.opts.WallTexBank)
	skyColU := make([]int, 0)
	skyRowV := make([]int, 0)
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
		tex := flatCache[key.flat]
		if !key.fallback && tex == nil {
			tex, _ = g.flatRGBAResolvedKey(key.flat)
			flatCache[key.flat] = tex
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
					} else if key.fallback {
						pix[i+0] = fb.R
						pix[i+1] = fb.G
						pix[i+2] = fb.B
						pix[i+3] = 255
					} else if len(tex) == 64*64*4 {
						u := int(math.Floor(wxSpan)) & 63
						v := int(math.Floor(wySpan)) & 63
						ti := (v*64 + u) * 4
						pix[i+0] = tex[ti+0]
						pix[i+1] = tex[ti+1]
						pix[i+2] = tex[ti+2]
						pix[i+3] = 255
					} else {
						pix[i+0] = fb.R
						pix[i+1] = fb.G
						pix[i+2] = fb.B
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
				} else if tex, ok := g.flatRGBA(name); ok {
					u := int(math.Floor(wx)) & 63
					v := int(math.Floor(wy)) & 63
					ti := (v*64 + u) * 4
					pix[i+0] = tex[ti+0]
					pix[i+1] = tex[ti+1]
					pix[i+2] = tex[ti+2]
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
				if tex, ok := g.flatRGBA(g.m.Sectors[sec].FloorPic); ok {
					u := int(math.Floor(wx)) & 63
					v := int(math.Floor(wy)) & 63
					ti := (v*64 + u) * 4
					pix[i+0] = tex[ti+0]
					pix[i+1] = tex[ti+1]
					pix[i+2] = tex[ti+2]
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
	const eps = 0.125
	clipNear := near + eps
	if f1 <= near && f2 <= near {
		return 0, 0, 0, 0, false
	}
	// Work from originals so we never interpolate from already-mutated values.
	of1, os1 := f1, s1
	of2, os2 := f2, s2
	if of1 < near {
		den := of2 - of1
		if math.Abs(den) < 1e-9 {
			return 0, 0, 0, 0, false
		}
		t := (clipNear - of1) / den
		if t < 0 {
			t = 0
		}
		if t > 1 {
			t = 1
		}
		f1 = clipNear
		s1 = os1 + (os2-os1)*t
	}
	if of2 < near {
		den := of1 - of2
		if math.Abs(den) < 1e-9 {
			return 0, 0, 0, 0, false
		}
		t := (clipNear - of2) / den
		if t < 0 {
			t = 0
		}
		if t > 1 {
			t = 1
		}
		f2 = clipNear
		s2 = os2 + (os1-os2)*t
	}
	if f1 < near || f2 < near {
		return 0, 0, 0, 0, false
	}
	return f1, s1, f2, s2, true
}

func clipSegmentToNearWithAttr(f1, s1, a1, f2, s2, a2, near float64) (float64, float64, float64, float64, float64, float64, bool) {
	const eps = 0.125
	clipNear := near + eps
	if f1 <= near && f2 <= near {
		return 0, 0, 0, 0, 0, 0, false
	}
	of1, os1, oa1 := f1, s1, a1
	of2, os2, oa2 := f2, s2, a2
	if of1 < near {
		den := of2 - of1
		if math.Abs(den) < 1e-9 {
			return 0, 0, 0, 0, 0, 0, false
		}
		t := (clipNear - of1) / den
		if t < 0 {
			t = 0
		}
		if t > 1 {
			t = 1
		}
		f1 = clipNear
		s1 = os1 + (os2-os1)*t
		a1 = oa1 + (oa2-oa1)*t
	}
	if of2 < near {
		den := of1 - of2
		if math.Abs(den) < 1e-9 {
			return 0, 0, 0, 0, 0, 0, false
		}
		t := (clipNear - of2) / den
		if t < 0 {
			t = 0
		}
		if t > 1 {
			t = 1
		}
		f2 = clipNear
		s2 = os2 + (os1-os2)*t
		a2 = oa2 + (oa1-oa2)*t
	}
	if f1 < near || f2 < near {
		return 0, 0, 0, 0, 0, 0, false
	}
	return f1, s1, a1, f2, s2, a2, true
}

type solidSpan struct {
	l int
	r int
}

func solidFullyCovered(spans []solidSpan, l, r int) bool {
	if l > r {
		return true
	}
	cur := l
	for _, s := range spans {
		if s.r < cur {
			continue
		}
		if s.l > cur {
			return false
		}
		if s.r+1 > cur {
			cur = s.r + 1
		}
		if cur > r {
			return true
		}
	}
	return false
}

func addSolidSpan(spans []solidSpan, l, r int) []solidSpan {
	if l > r {
		return spans
	}
	ns := solidSpan{l: l, r: r}
	out := make([]solidSpan, 0, len(spans)+1)
	inserted := false
	for _, s := range spans {
		if s.r+1 < ns.l {
			out = append(out, s)
			continue
		}
		if ns.r+1 < s.l {
			if !inserted {
				out = append(out, ns)
				inserted = true
			}
			out = append(out, s)
			continue
		}
		if s.l < ns.l {
			ns.l = s.l
		}
		if s.r > ns.r {
			ns.r = s.r
		}
	}
	if !inserted {
		out = append(out, ns)
	}
	return out
}

func clipRangeAgainstSolidSpans(l, r int, covered []solidSpan, out []solidSpan) []solidSpan {
	out = out[:0]
	if r < l {
		return out
	}
	if len(covered) == 0 {
		return append(out, solidSpan{l: l, r: r})
	}
	cur := l
	for _, s := range covered {
		if s.r < cur {
			continue
		}
		if s.l > r {
			break
		}
		if s.l > cur {
			right := min(r, s.l-1)
			if right >= cur {
				out = append(out, solidSpan{l: cur, r: right})
			}
		}
		if s.r+1 > cur {
			cur = s.r + 1
		}
		if cur > r {
			break
		}
	}
	if cur <= r {
		out = append(out, solidSpan{l: cur, r: r})
	}
	return out
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

func (g *game) drawBillboardProjectiles(screen *ebiten.Image, camX, camY, camAng, focal, near float64) {
	type projectedProjectile struct {
		dist  float64
		sx    float64
		sy    float64
		r     float64
		outer color.RGBA
		inner color.RGBA
	}
	if len(g.projectiles) == 0 {
		return
	}
	items := make([]projectedProjectile, 0, len(g.projectiles))
	ca := math.Cos(camAng)
	sa := math.Sin(camAng)
	eyeZ := g.playerEyeZ()

	for _, p := range g.projectiles {
		px := float64(p.x)/fracUnit - camX
		py := float64(p.y)/fracUnit - camY
		f := px*ca + py*sa
		s := -px*sa + py*ca
		if f <= near {
			continue
		}
		// Coarse occlusion check against solid map geometry.
		sx := float64(g.viewW)/2 - (s/f)*focal
		centerZ := float64(p.z+p.height/2) / fracUnit
		sy := float64(g.viewH)/2 - ((centerZ-eyeZ)/f)*focal
		r := (projectileViewRadius(p) / f) * focal
		if r < 1.2 {
			r = 1.2
		}
		xPad := r + 8
		yPad := r + 8
		if sx+xPad < 0 || sx-xPad > float64(g.viewW) || sy+yPad < 0 || sy-yPad > float64(g.viewH) {
			continue
		}
		cr := projectileColor(p.kind)
		items = append(items, projectedProjectile{
			dist:  f,
			sx:    sx,
			sy:    sy,
			r:     r,
			outer: color.RGBA{R: cr[0], G: cr[1], B: 24, A: 255},
			inner: color.RGBA{R: 255, G: 236, B: 120, A: 232},
		})
	}

	sort.Slice(items, func(i, j int) bool { return items[i].dist > items[j].dist })
	for _, it := range items {
		drawCircleApprox(screen, it.sx, it.sy, it.r, it.outer)
		drawCircleApprox(screen, it.sx, it.sy, it.r*0.52, it.inner)
	}
}

func drawCircleApprox(screen *ebiten.Image, cx, cy, r float64, clr color.RGBA) {
	if r <= 1.2 {
		ebitenutil.DrawRect(screen, cx-1, cy-1, 2, 2, clr)
		return
	}
	const segs = 18
	prevX := cx + r
	prevY := cy
	for i := 1; i <= segs; i++ {
		a := (2 * math.Pi * float64(i)) / segs
		x := cx + math.Cos(a)*r
		y := cy + math.Sin(a)*r
		vector.StrokeLine(screen, float32(prevX), float32(prevY), float32(x), float32(y), 1.2, clr, true)
		prevX = x
		prevY = y
	}
}

func (g *game) drawBillboardProjectilesToBuffer(camX, camY, camAng, focal, near float64) {
	const planeDepthBias = 24.0
	planeBiasQ := encodeDepthBiasQ(planeDepthBias)
	useDepth := g.depthOcclusionEnabled()
	depthPix := g.depthPix3D
	depthPlanePix := g.depthPlanePix3D
	wallPix := g.wallPix32
	viewW := g.viewW
	viewH := g.viewH
	stamp := g.depthFrameStamp
	if len(wallPix) != viewW*viewH {
		return
	}
	if useDepth && len(depthPix) != viewW*viewH {
		return
	}
	replay := g.billboardReplayActive && g.billboardReplayKind == billboardQueueProjectiles
	var items []projectedProjectileItem
	if replay {
		i := g.billboardReplayIndex
		if i < 0 || i >= len(g.projectileItemsScratch) {
			return
		}
		items = g.projectileItemsScratch[i : i+1]
	} else {
		if len(g.projectiles) == 0 && len(g.projectileImpacts) == 0 {
			return
		}
		items = g.ensureProjectileItemsScratch(len(g.projectiles) + len(g.projectileImpacts))
	}
	ca := math.Cos(camAng)
	sa := math.Sin(camAng)
	eyeZ := g.playerEyeZ()
	if !replay {
		for _, p := range g.projectiles {
			px := float64(p.x)/fracUnit - camX
			py := float64(p.y)/fracUnit - camY
			f := px*ca + py*sa
			s := -px*sa + py*ca
			if f <= near {
				continue
			}
			sec := g.sectorAt(p.x, p.y)
			clipTop := 0
			clipBottom := viewH - 1
			clipRadius := p.radius
			if clipRadius <= 0 {
				clipRadius = 8 * fracUnit
			}
			var clipOK bool
			clipTop, clipBottom, clipOK = g.spriteFootprintClipYBounds(p.x, p.y, clipRadius, viewH, eyeZ, f, focal)
			if !clipOK {
				continue
			}
			scale := focal / f
			if scale <= 0 {
				continue
			}
			sx := float64(viewW)/2 - (s/f)*focal
			yb := float64(viewH)/2 - ((float64(p.z)/fracUnit-eyeZ)/f)*focal
			cr := projectileColor(p.kind)
			spriteKey := g.projectileSpriteName(p.kind, g.worldTic)
			spriteTex, hasSprite := g.monsterSpriteTexture(spriteKey)
			h := 0.0
			w := 0.0
			if hasSprite && spriteTex.Height > 0 && spriteTex.Width > 0 {
				h = float64(spriteTex.Height) * scale
				w = float64(spriteTex.Width) * scale
			} else {
				r := (projectileViewRadius(p) / f) * focal
				if r < 1.2 {
					r = 1.2
				}
				w = r * 2
				h = r * 2
			}
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
			opaque, hasOpaque := g.spriteOpaqueShapeForKey(spriteKey, spriteTex)
			items = append(items, projectedProjectileItem{
				dist:       f,
				sx:         sx,
				yb:         yb,
				h:          h,
				clipTop:    clipTop,
				clipBottom: clipBottom,
				clr:        color.RGBA{R: cr[0], G: cr[1], B: 24, A: 255},
				lightMul:   lightMul,
				fullBright: true,
				spriteKey:  spriteKey,
				opaque:     opaque,
				hasOpaque:  hasOpaque,
				spriteTex:  spriteTex,
				hasSprite:  hasSprite,
			})
		}
		for _, fx := range g.projectileImpacts {
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
			cr := projectileColor(fx.kind)
			spriteKey := g.projectileImpactSpriteName(fx.kind, fx.totalTics-fx.tics)
			spriteTex, hasSprite := g.monsterSpriteTexture(spriteKey)
			h := 0.0
			w := 0.0
			if hasSprite && spriteTex.Height > 0 && spriteTex.Width > 0 {
				h = float64(spriteTex.Height) * scale
				w = float64(spriteTex.Width) * scale
			}
			if h <= 0 || w <= 0 {
				// Sprite fallback only; impacts should always use a sprite.
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
			opaque, hasOpaque := g.spriteOpaqueShapeForKey(spriteKey, spriteTex)
			items = append(items, projectedProjectileItem{
				dist:       f,
				sx:         sx,
				yb:         yb,
				h:          h,
				clipTop:    clipTop,
				clipBottom: clipBottom,
				clr:        color.RGBA{R: cr[0], G: cr[1], B: 24, A: 255},
				lightMul:   lightMul,
				fullBright: true,
				spriteKey:  spriteKey,
				opaque:     opaque,
				hasOpaque:  hasOpaque,
				spriteTex:  spriteTex,
				hasSprite:  hasSprite,
			})
		}
	}
	if !replay {
		g.projectileItemsScratch = items
		sort.Slice(items, func(i, j int) bool { return items[i].dist > items[j].dist })
		if g.billboardQueueCollect {
			for i := range items {
				if !useDepth {
					x0, x1, y0, y1, ok := projectileItemScreenBounds(items[i], viewW, viewH)
					if ok {
						depthQ := encodeDepthQ(items[i].dist)
						if items[i].hasOpaque && len(items[i].opaque.rects) > 0 {
							th := items[i].spriteTex.Height
							if th > 0 {
								scale := items[i].h / float64(th)
								dstX := items[i].sx - float64(items[i].spriteTex.OffsetX)*scale
								dstY := items[i].yb - float64(items[i].spriteTex.OffsetY)*scale
								if g.spriteOpaqueRectsFullyOccluded(items[i].opaque.rects, dstX, dstY, scale, items[i].clipTop, items[i].clipBottom, viewW, viewH, depthQ) {
									continue
								}
							}
						}
						if g.spriteWallClipQuadFullyOccluded(x0, x1, y0, y1, depthQ) {
							continue
						}
					}
				}
				g.billboardQueueScratch = append(g.billboardQueueScratch, billboardQueueItem{
					dist: items[i].dist,
					kind: billboardQueueProjectiles,
					idx:  i,
				})
			}
			return
		}
	}
	for _, it := range items {
		depthQ := encodeDepthQ(it.dist)
		depthPacked := packDepthStamped(depthQ, stamp)
		shadeMul := uint32(256)
		if !it.fullBright {
			shadeMul = uint32(combineShadeMul(int(monsterShadeFactor(it.dist, near)*256.0), int(it.lightMul)))
		}
		if it.hasSprite {
			th := it.spriteTex.Height
			tw := it.spriteTex.Width
			if th > 0 && tw > 0 {
				src32, ok32 := spritePixels32(it.spriteTex)
				srcIndexed, srcMask, _ := spriteIndexedPixels(it.spriteTex)
				useIndexed := len(srcIndexed) == len(srcMask) && len(srcIndexed) == tw*th
				var shadeRow []uint32
				if useIndexed && wallShadePackedOK {
					if shadeMul > 256 {
						shadeMul = 256
					}
					shadeRow = wallShadePackedLUT[shadeMul][:]
				}
				if !ok32 && !useIndexed {
					continue
				}
				scale := it.h / float64(th)
				if scale <= 0 {
					continue
				}
				dstW := float64(tw) * scale
				dstH := float64(th) * scale
				dstX := it.sx - float64(it.spriteTex.OffsetX)*scale
				dstY := it.yb - float64(it.spriteTex.OffsetY)*scale
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
					continue
				}
				if !useDepth && ((it.hasOpaque && len(it.opaque.rects) > 0 && g.spriteOpaqueRectsFullyOccluded(it.opaque.rects, dstX, dstY, scale, it.clipTop, it.clipBottom, viewW, viewH, depthQ)) || g.spriteWallClipQuadFullyOccluded(x0, x1, y0, y1, depthQ)) {
					continue
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
				if it.hasOpaque {
					x0, x1, txLUT = trimScreenRangeToOpaqueLUT(x0, x1, txLUT, it.opaque.bounds.minX, it.opaque.bounds.maxX)
					y0, y1, tyLUT = trimScreenRangeToOpaqueLUT(y0, y1, tyLUT, it.opaque.bounds.minY, it.opaque.bounds.maxY)
					if x0 > x1 || y0 > y1 {
						continue
					}
				}
				if !useDepth && len(it.clipSpans) == 0 && !((it.hasOpaque && len(it.opaque.rects) > 0 && g.spriteOpaqueRectsHaveAnyOccluder(it.opaque.rects, dstX, dstY, scale, it.clipTop, it.clipBottom, viewW, viewH, depthQ)) || g.spriteWallClipBBoxHasAnyOccluder(x0, x1, y0, y1, depthQ)) {
					g.drawBillboardSpriteRowsDirect(src32, srcIndexed, srcMask, tw, txLUT, tyLUT, x0, x1, y0, y1, shadeMul)
					continue
				}
				for y := y0; y <= y1; y++ {
					ty := tyLUT[y-y0]
					row := y * viewW
					if len(it.clipSpans) == 0 && x1-x0 >= spriteRowOcclusionMinSpan && g.rowFullyOccludedDepthQ(depthQ, planeBiasQ, row, x0, x1) {
						continue
					}
					if !useDepth {
						rowSpans := g.spriteRowVisibleSpansDepthQ(y, x0, x1, depthQ, it.clipSpans, g.solidClipScratch[:0])
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
								l, r, ok := trimSpanToOpaqueLUTRange(sp.l, sp.r, x0, txLUT, minTex, maxTex)
								if ok {
									filtered = append(filtered, solidSpan{l: l, r: r})
								}
							}
							g.solidClipScratch = filtered
							rowSpans = filtered
							if len(rowSpans) == 0 {
								continue
							}
						}
						g.drawBillboardRowSpans(row, ty, tw, x0, txLUT, rowSpans, src32, srcIndexed, srcMask, shadeMul, shadeRow)
						continue
					}
					for x := x0; x <= x1; {
						if x+1 <= x1 {
							in0 := xInSolidSpans(x, it.clipSpans)
							in1 := xInSolidSpans(x+1, it.clipSpans)
							if !in0 && !in1 {
								x += 2
								continue
							}
							i0 := row + x
							i1 := i0 + 1
							occ0 := !in0 || g.spriteOccludedDepthQAt(depthPix, depthPlanePix, stamp, depthQ, planeBiasQ, i0)
							occ1 := !in1 || g.spriteOccludedDepthQAt(depthPix, depthPlanePix, stamp, depthQ, planeBiasQ, i1)
							if !occ0 && !occ1 {
								p0 := src32[ty*tw+txLUT[x-x0]]
								p1 := src32[ty*tw+txLUT[x+1-x0]]
								a0 := ((p0 >> pixelAShift) & 0xFF) != 0
								a1 := ((p1 >> pixelAShift) & 0xFF) != 0
								if a0 && a1 {
									if shadeMul >= 256 {
										g.writeWallPixelPair(i0, p0|pixelOpaqueA, p1|pixelOpaqueA)
									} else {
										g.writeWallPixelPair(i0, shadePackedRGBA(p0, shadeMul), shadePackedRGBA(p1, shadeMul))
									}
									g.setDepthPixelPairEncoded(i0, depthPacked)
									x += 2
									continue
								}
								if a0 {
									if shadeMul >= 256 {
										g.writeWallPixel(i0, p0|pixelOpaqueA)
									} else {
										g.writeWallPixel(i0, shadePackedRGBA(p0, shadeMul))
									}
									g.setDepthPixelEncoded(i0, depthPacked)
								}
								if a1 {
									if shadeMul >= 256 {
										g.writeWallPixel(i1, p1|pixelOpaqueA)
									} else {
										g.writeWallPixel(i1, shadePackedRGBA(p1, shadeMul))
									}
									g.setDepthPixelEncoded(i1, depthPacked)
								}
								x += 2
								continue
							}
							if !occ0 {
								p0 := src32[ty*tw+txLUT[x-x0]]
								if ((p0 >> pixelAShift) & 0xFF) != 0 {
									g.writeWallPixel(i0, shadePackedRGBA(p0, shadeMul))
									g.setDepthPixelEncoded(i0, depthPacked)
								}
							}
							if !occ1 {
								p1 := src32[ty*tw+txLUT[x+1-x0]]
								if ((p1 >> pixelAShift) & 0xFF) != 0 {
									g.writeWallPixel(i1, shadePackedRGBA(p1, shadeMul))
									g.setDepthPixelEncoded(i1, depthPacked)
								}
							}
							x += 2
							continue
						}
						i := row + x
						if !xInSolidSpans(x, it.clipSpans) {
							x++
							continue
						}
						if !g.spriteOccludedDepthQAt(depthPix, depthPlanePix, stamp, depthQ, planeBiasQ, i) {
							p := src32[ty*tw+txLUT[x-x0]]
							if ((p >> pixelAShift) & 0xFF) != 0 {
								g.writeWallPixel(i, shadePackedRGBA(p, shadeMul))
								g.setDepthPixelEncoded(i, depthPacked)
							}
						}
						x++
					}
				}
				continue
			}
		}
		rad := it.h * 0.5
		r2 := rad * rad
		cy := it.yb - rad
		x0 := int(math.Floor(it.sx - rad))
		x1 := int(math.Ceil(it.sx + rad))
		y0 := int(math.Floor(cy - rad))
		y1 := int(math.Ceil(cy + rad))
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
			continue
		}
		if !useDepth && g.spriteWallClipQuadFullyOccluded(x0, x1, y0, y1, depthQ) {
			continue
		}
		rc := uint8((uint32(it.clr.R) * shadeMul) >> 8)
		gc := uint8((uint32(it.clr.G) * shadeMul) >> 8)
		b := uint8((uint32(it.clr.B) * shadeMul) >> 8)
		for y := y0; y <= y1; y++ {
			dy := (float64(y) + 0.5) - cy
			row := y * viewW
			if len(it.clipSpans) == 0 && x1-x0 >= spriteRowOcclusionMinSpan && g.rowFullyOccludedDepthQ(depthQ, planeBiasQ, row, x0, x1) {
				continue
			}
			for x := x0; x <= x1; x++ {
				if !xInSolidSpans(x, it.clipSpans) {
					continue
				}
				dx := (float64(x) + 0.5) - it.sx
				if dx*dx+dy*dy > r2 {
					continue
				}
				i := row + x
				if g.spriteOccludedDepthQAt(depthPix, depthPlanePix, stamp, depthQ, planeBiasQ, i) {
					continue
				}
				g.writeWallPixel(i, packRGBA(rc, gc, b))
				g.setDepthPixelEncoded(i, depthPacked)
			}
		}
	}
}

func (g *game) projectileSpriteTexture(kind projectileKind, tic int) (WallTexture, bool) {
	name := g.projectileSpriteName(kind, tic)
	if name == "" {
		return WallTexture{}, false
	}
	return g.monsterSpriteTexture(name)
}

func (g *game) projectileImpactSpriteTexture(kind projectileKind, elapsed int) (WallTexture, bool) {
	name := g.projectileImpactSpriteName(kind, elapsed)
	if name == "" {
		return WallTexture{}, false
	}
	return g.monsterSpriteTexture(name)
}

func (g *game) projectileImpactSpriteName(kind projectileKind, elapsed int) string {
	if elapsed < 0 {
		elapsed = 0
	}
	prefix := "BAL1"
	frame := byte('C')
	switch kind {
	case projectileRocket:
		prefix = "MISL"
		switch {
		case elapsed < 8:
			frame = 'B'
		case elapsed < 14:
			frame = 'C'
		default:
			frame = 'D'
		}
	case projectileBaronBall:
		prefix = "BAL7"
		switch {
		case elapsed < 6:
			frame = 'C'
		case elapsed < 12:
			frame = 'D'
		default:
			frame = 'E'
		}
	case projectilePlasmaBall:
		prefix = "BAL2"
		switch {
		case elapsed < 6:
			frame = 'C'
		case elapsed < 12:
			frame = 'D'
		default:
			frame = 'E'
		}
	default:
		switch {
		case elapsed < 6:
			frame = 'C'
		case elapsed < 12:
			frame = 'D'
		default:
			frame = 'E'
		}
	}
	name0 := spriteFrameName(prefix, byte(frame), '0')
	if _, ok := g.opts.SpritePatchBank[name0]; ok {
		return name0
	}
	if name, _, ok := g.monsterSpriteRotFrame(prefix, byte(frame), 1); ok {
		return name
	}
	// Fallback to flight sprite if impact frame is unavailable in the bank.
	return g.projectileSpriteName(kind, g.worldTic)
}

func (g *game) projectileSpriteName(kind projectileKind, tic int) string {
	pickPrefixFrame := func(prefix string, frameLetters []byte, frame int) string {
		if len(frameLetters) == 0 {
			return ""
		}
		for i := 0; i < len(frameLetters); i++ {
			fl := frameLetters[(frame+i)%len(frameLetters)]
			// Some assets use non-rotating frame notation (e.g. BAL1A0).
			name0 := spriteFrameName(prefix, fl, '0')
			if _, ok := g.opts.SpritePatchBank[name0]; ok {
				return name0
			}
			// Standard Doom rotating/projectile frames (e.g. BAL1A1, paired lumps, etc).
			if name, _, ok := g.monsterSpriteRotFrame(prefix, fl, 1); ok {
				return name
			}
		}
		return ""
	}
	frame2 := (tic / 4) & 1
	switch kind {
	case projectileRocket:
		return pickPrefixFrame("MISL", []byte{'A'}, 0)
	case projectileBaronBall:
		return pickPrefixFrame("BAL7", []byte{'A', 'B'}, frame2)
	case projectilePlasmaBall:
		return pickPrefixFrame("BAL2", []byte{'A', 'B'}, frame2)
	default:
		return pickPrefixFrame("BAL1", []byte{'A', 'B'}, frame2)
	}
}

func (g *game) spawnHitscanPuff(x, y, z int64) {
	const maxPuffs = 64
	if len(g.hitscanPuffs) >= maxPuffs {
		copy(g.hitscanPuffs, g.hitscanPuffs[1:])
		g.hitscanPuffs = g.hitscanPuffs[:maxPuffs-1]
	}
	tics := 4 + (doomrand.PRandom() & 3)
	g.hitscanPuffs = append(g.hitscanPuffs, hitscanPuff{x: x, y: y, z: z, tics: tics, kind: hitscanFxPuff})
}

func (g *game) spawnHitscanBlood(x, y, z int64) {
	const maxPuffs = 64
	if len(g.hitscanPuffs) >= maxPuffs {
		copy(g.hitscanPuffs, g.hitscanPuffs[1:])
		g.hitscanPuffs = g.hitscanPuffs[:maxPuffs-1]
	}
	tics := 6 + (doomrand.PRandom() & 3)
	g.hitscanPuffs = append(g.hitscanPuffs, hitscanPuff{x: x, y: y, z: z, tics: tics, kind: hitscanFxBlood})
}

func (g *game) hitscanEffectSprite(p hitscanPuff) (WallTexture, bool) {
	find := func(names ...string) (WallTexture, bool) {
		for _, name := range names {
			if tex, ok := g.monsterSpriteTexture(name); ok {
				return tex, true
			}
		}
		return WallTexture{}, false
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
		p.tics--
		if p.tics <= 0 {
			continue
		}
		keep = append(keep, p)
	}
	g.hitscanPuffs = keep
}

func (g *game) drawHitscanPuffsToBuffer(camX, camY, camAng, focal, near float64) {
	const planeDepthBias = 16.0
	planeBiasQ := encodeDepthBiasQ(planeDepthBias)
	useDepth := g.depthOcclusionEnabled()
	depthPix := g.depthPix3D
	depthPlanePix := g.depthPlanePix3D
	wallPix := g.wallPix32
	viewW := g.viewW
	viewH := g.viewH
	stamp := g.depthFrameStamp
	if len(wallPix) != viewW*viewH {
		return
	}
	if useDepth && len(depthPix) != viewW*viewH {
		return
	}
	replay := g.billboardReplayActive && g.billboardReplayKind == billboardQueuePuffs
	var items []projectedPuffItem
	if replay {
		i := g.billboardReplayIndex
		if i < 0 || i >= len(g.puffItemsScratch) {
			return
		}
		items = g.puffItemsScratch[i : i+1]
	} else {
		if len(g.hitscanPuffs) == 0 {
			return
		}
		items = g.ensurePuffItemsScratch(len(g.hitscanPuffs))
	}
	ca := math.Cos(camAng)
	sa := math.Sin(camAng)
	eyeZ := g.playerEyeZ()
	if !replay {
		for _, p := range g.hitscanPuffs {
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
			r := 0.0
			if hasSprite && spriteTex.Width > 0 && spriteTex.Height > 0 {
				scale := focal / f
				if scale <= 0 {
					continue
				}
				dstX := sx - float64(spriteTex.OffsetX)*scale
				dstY := sy - float64(spriteTex.OffsetY)*scale
				dstW := float64(spriteTex.Width) * scale
				dstH := float64(spriteTex.Height) * scale
				if dstX+dstW < 0 || dstX > float64(viewW) || dstY+dstH < 0 || dstY > float64(viewH) {
					continue
				}
				r = dstH * 0.5
			} else {
				worldH := hitscanPuffWorldHeight
				if p.kind == hitscanFxBlood {
					worldH = hitscanBloodWorldHeight
				}
				spriteH := (worldH / f) * focal
				r = spriteH * 0.5
				xPad := r + 2
				yPad := r + 2
				if sx+xPad < 0 || sx-xPad > float64(viewW) || sy+yPad < 0 || sy-yPad > float64(viewH) {
					continue
				}
			}
			items = append(items, projectedPuffItem{
				dist:       f,
				sx:         sx,
				sy:         sy,
				r:          r,
				clipTop:    clipTop,
				clipBottom: clipBottom,
				kind:       p.kind,
				spriteTex:  spriteTex,
				hasSprite:  hasSprite,
			})
		}
	}
	if !replay {
		g.puffItemsScratch = items
		sort.Slice(items, func(i, j int) bool { return items[i].dist > items[j].dist })
		if g.billboardQueueCollect {
			for i := range items {
				if !useDepth {
					x0, x1, y0, y1, ok := puffItemScreenBounds(items[i], focal, viewW, viewH)
					if ok && g.spriteWallClipQuadFullyOccluded(x0, x1, y0, y1, encodeDepthQ(items[i].dist)) {
						continue
					}
				}
				g.billboardQueueScratch = append(g.billboardQueueScratch, billboardQueueItem{
					dist: items[i].dist,
					kind: billboardQueuePuffs,
					idx:  i,
				})
			}
			return
		}
	}
	for _, it := range items {
		depthQ := encodeDepthQ(it.dist)
		if it.hasSprite {
			th := it.spriteTex.Height
			tw := it.spriteTex.Width
			if th > 0 && tw > 0 {
				src32, ok32 := spritePixels32(it.spriteTex)
				if ok32 {
					scale := focal / it.dist
					if scale <= 0 {
						continue
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
						continue
					}
					if !useDepth && g.spriteWallClipQuadFullyOccluded(x0, x1, y0, y1, depthQ) {
						continue
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
						if len(it.clipSpans) == 0 && x1-x0 >= spriteRowOcclusionMinSpan && g.rowFullyOccludedDepthQ(depthQ, planeBiasQ, row, x0, x1) {
							continue
						}
						for x := x0; x <= x1; x++ {
							if !xInSolidSpans(x, it.clipSpans) {
								continue
							}
							i := row + x
							if g.spriteOccludedDepthQAt(depthPix, depthPlanePix, stamp, depthQ, planeBiasQ, i) {
								continue
							}
							pix := src32[ty*tw+txLUT[x-x0]]
							if ((pix >> pixelAShift) & 0xFF) == 0 {
								continue
							}
							g.writeWallPixel(i, pix)
						}
					}
					continue
				}
			}
		}
		r2 := it.r * it.r
		x0 := int(math.Floor(it.sx - it.r))
		x1 := int(math.Ceil(it.sx + it.r))
		y0 := int(math.Floor(it.sy - it.r))
		y1 := int(math.Ceil(it.sy + it.r))
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
			continue
		}
		if !useDepth && g.spriteWallClipQuadFullyOccluded(x0, x1, y0, y1, depthQ) {
			continue
		}
		for y := y0; y <= y1; y++ {
			dy := (float64(y) + 0.5) - it.sy
			row := y * viewW
			if len(it.clipSpans) == 0 && x1-x0 >= spriteRowOcclusionMinSpan && g.rowFullyOccludedDepthQ(depthQ, planeBiasQ, row, x0, x1) {
				continue
			}
			for x := x0; x <= x1; x++ {
				if !xInSolidSpans(x, it.clipSpans) {
					continue
				}
				dx := (float64(x) + 0.5) - it.sx
				if dx*dx+dy*dy > r2 {
					continue
				}
				i := row + x
				if g.spriteOccludedDepthQAt(depthPix, depthPlanePix, stamp, depthQ, planeBiasQ, i) {
					continue
				}
				if it.kind == hitscanFxBlood {
					g.writeWallPixel(i, packRGBA(164, 30, 30))
				} else {
					g.writeWallPixel(i, packRGBA(236, 236, 236))
				}
			}
		}
	}
}

func (g *game) drawBillboardMonstersToBuffer(camX, camY, camAng, focal, near float64) {
	const planeDepthBias = 32.0
	planeBiasQ := encodeDepthBiasQ(planeDepthBias)
	useDepth := g.depthOcclusionEnabled()
	depthPix := g.depthPix3D
	depthPlanePix := g.depthPlanePix3D
	wallPix := g.wallPix32
	viewW := g.viewW
	viewH := g.viewH
	stamp := g.depthFrameStamp
	if len(wallPix) != viewW*viewH {
		return
	}
	if useDepth && len(depthPix) != viewW*viewH {
		return
	}
	replay := g.billboardReplayActive && g.billboardReplayKind == billboardQueueMonsters
	var items []projectedMonsterItem
	if replay {
		i := g.billboardReplayIndex
		if i < 0 || i >= len(g.monsterItemsScratch) {
			return
		}
		items = g.monsterItemsScratch[i : i+1]
	} else {
		items = g.ensureMonsterItemsScratch(32)
	}
	ca := math.Cos(camAng)
	sa := math.Sin(camAng)
	eyeZ := g.playerEyeZ()

	if !replay {
		for i, th := range g.m.Things {
			if i < 0 || i >= len(g.thingCollected) || g.thingCollected[i] {
				continue
			}
			if !isMonster(th.Type) {
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
			sx := float64(viewW)/2 - (s/f)*focal
			floorZFixed := g.thingFloorZ(txFixed, tyFixed)
			floorZ := float64(floorZFixed) / fracUnit
			yb := float64(viewH)/2 - ((floorZ-eyeZ)/f)*focal
			sprite, flip := g.monsterSpriteNameForView(i, th, g.worldTic, camX, camY)
			tex, ok := g.monsterSpriteTexture(sprite)
			if !ok || tex.Height <= 0 || tex.Width <= 0 {
				continue
			}
			h := 0.0
			w := 0.0
			if i >= 0 && i < len(g.thingDead) && g.thingDead[i] {
				// Doom-like corpse projection: use sprite-native scale instead of standing actor height.
				scale := focal / f
				if scale <= 0 {
					continue
				}
				h = float64(tex.Height) * scale
				w = float64(tex.Width) * scale
			} else {
				monsterH := monsterRenderHeight(th.Type)
				yt := float64(viewH)/2 - ((floorZ+monsterH-eyeZ)/f)*focal
				if yb <= yt {
					continue
				}
				h = yb - yt
				w = math.Max(6, math.Min(120, h*0.45))
			}
			if h <= 0 || w <= 0 {
				continue
			}
			xPad := w/2 + 8
			if sx+xPad < 0 || sx-xPad > float64(viewW) {
				continue
			}
			clipRadius := projectedScreenWidthToWorldRadiusFixed(w, f, focal)
			clipTop, clipBottom, clipOK := g.spriteFootprintClipYBounds(txFixed, tyFixed, clipRadius, viewH, eyeZ, f, focal)
			if !clipOK {
				continue
			}
			sec := g.thingSectorCached(i, th)
			lightMul := uint32(256)
			if sec >= 0 && sec < len(g.m.Sectors) {
				lightMul = g.sectorLightMulCached(sec)
			}
			opaque, hasOpaque := g.spriteOpaqueShapeForKey(sprite, tex)
			items = append(items, projectedMonsterItem{
				dist:       f,
				sx:         sx,
				yb:         yb,
				h:          h,
				w:          w,
				clipTop:    clipTop,
				clipBottom: clipBottom,
				tex:        tex,
				flip:       flip,
				lightMul:   lightMul,
				fullBright: monsterSpriteFullBright(sprite),
				spriteKey:  sprite,
				opaque:     opaque,
				hasOpaque:  hasOpaque,
				shadow:     monsterUsesShadow(th.Type),
			})
		}
	}
	if !replay {
		g.monsterItemsScratch = items
		// Draw far-to-near.
		sort.Slice(items, func(i, j int) bool { return items[i].dist > items[j].dist })
		if g.billboardQueueCollect {
			for i := range items {
				if !useDepth {
					x0, x1, y0, y1, ok := monsterItemScreenBounds(items[i], viewW, viewH)
					if ok {
						depthQ := encodeDepthQ(items[i].dist)
						th := items[i].tex.Height
						if items[i].hasOpaque && len(items[i].opaque.rects) > 0 && th > 0 {
							scale := items[i].h / float64(th)
							dstX := items[i].sx - float64(items[i].tex.OffsetX)*scale
							dstY := items[i].yb - float64(items[i].tex.OffsetY)*scale
							if g.spriteOpaqueRectsFullyOccluded(items[i].opaque.rects, dstX, dstY, scale, items[i].clipTop, items[i].clipBottom, viewW, viewH, depthQ) {
								continue
							}
						}
						if g.spriteWallClipQuadFullyOccluded(x0, x1, y0, y1, depthQ) {
							continue
						}
					}
				}
				g.billboardQueueScratch = append(g.billboardQueueScratch, billboardQueueItem{
					dist: items[i].dist,
					kind: billboardQueueMonsters,
					idx:  i,
				})
			}
			return
		}
	}
	for _, it := range items {
		depthQ := encodeDepthQ(it.dist)
		depthPacked := packDepthStamped(depthQ, stamp)
		th := it.tex.Height
		tw := it.tex.Width
		if th <= 0 || tw <= 0 {
			continue
		}
		src32, ok32 := spritePixels32(it.tex)
		srcIndexed, srcMask, _ := spriteIndexedPixels(it.tex)
		useIndexed := len(srcIndexed) == len(srcMask) && len(srcIndexed) == tw*th
		if !ok32 && !useIndexed {
			continue
		}
		scale := it.h / float64(th)
		if scale <= 0 {
			continue
		}
		dstX := it.sx - float64(it.tex.OffsetX)*scale
		dstY := it.yb - float64(it.tex.OffsetY)*scale
		dstW := float64(tw) * scale
		dstH := float64(th) * scale
		x0 := int(math.Floor(dstX))
		y0 := int(math.Floor(dstY))
		x1 := int(math.Ceil(dstX+dstW)) - 1
		y1 := int(math.Ceil(dstY+dstH)) - 1
		if x1 < 0 || y1 < 0 || x0 >= viewW || y0 >= viewH {
			continue
		}
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
			continue
		}
		if !useDepth && ((it.hasOpaque && len(it.opaque.rects) > 0 && g.spriteOpaqueRectsFullyOccluded(it.opaque.rects, dstX, dstY, scale, it.clipTop, it.clipBottom, viewW, viewH, depthQ)) || g.spriteWallClipQuadFullyOccluded(x0, x1, y0, y1, depthQ)) {
			continue
		}
		shadeMul := uint32(256)
		if !it.fullBright {
			shadeMul = uint32(combineShadeMul(int(monsterShadeFactor(it.dist, near)*256.0), int(it.lightMul)))
		}
		var shadeRow []uint32
		if useIndexed && wallShadePackedOK {
			if shadeMul > 256 {
				shadeMul = 256
			}
			shadeRow = wallShadePackedLUT[shadeMul][:]
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
			if it.flip {
				tx = tw - 1 - tx
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
		if it.hasOpaque {
			x0, x1, txLUT = trimScreenRangeToOpaqueLUT(x0, x1, txLUT, it.opaque.bounds.minX, it.opaque.bounds.maxX)
			y0, y1, tyLUT = trimScreenRangeToOpaqueLUT(y0, y1, tyLUT, it.opaque.bounds.minY, it.opaque.bounds.maxY)
			if x0 > x1 || y0 > y1 {
				continue
			}
		}
		if it.shadow && g.opts.SourcePortMode {
			g.drawShadowMonsterSourcePort(
				it,
				src32,
				tw,
				txLUT,
				tyLUT,
				x0,
				x1,
				y0,
				y1,
				depthQ,
				depthPacked,
				stamp,
				planeBiasQ,
				depthPix,
				depthPlanePix,
			)
			continue
		}
		if !it.shadow && !useDepth && len(it.clipSpans) == 0 && !((it.hasOpaque && len(it.opaque.rects) > 0 && g.spriteOpaqueRectsHaveAnyOccluder(it.opaque.rects, dstX, dstY, scale, it.clipTop, it.clipBottom, viewW, viewH, depthQ)) || g.spriteWallClipBBoxHasAnyOccluder(x0, x1, y0, y1, depthQ)) {
			g.drawBillboardSpriteRowsDirect(src32, srcIndexed, srcMask, tw, txLUT, tyLUT, x0, x1, y0, y1, shadeMul)
			continue
		}
		if it.shadow {
			for y := y0; y <= y1; y++ {
				row := y * viewW
				rowSpans := g.spriteRowVisibleSpansDepthQ(y, x0, x1, depthQ, it.clipSpans, g.solidClipScratch[:0])
				g.solidClipScratch = rowSpans
				for _, sp := range rowSpans {
					for x := sp.l; x <= sp.r; x++ {
						i := row + x
						p := src32[tyLUT[y-y0]*tw+txLUT[x-x0]]
						if ((p >> pixelAShift) & 0xFF) == 0 {
							continue
						}
						g.writeFuzzPixel(x, y, i)
					}
				}
			}
			continue
		}
		for y := y0; y <= y1; y++ {
			ty := tyLUT[y-y0]
			row := y * viewW
			if len(it.clipSpans) == 0 && x1-x0 >= spriteRowOcclusionMinSpan && g.rowFullyOccludedDepthQ(depthQ, planeBiasQ, row, x0, x1) {
				continue
			}
			if !useDepth {
				rowSpans := g.spriteRowVisibleSpansDepthQ(y, x0, x1, depthQ, it.clipSpans, g.solidClipScratch[:0])
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
						l, r, ok := trimSpanToOpaqueLUTRange(sp.l, sp.r, x0, txLUT, minTex, maxTex)
						if ok {
							filtered = append(filtered, solidSpan{l: l, r: r})
						}
					}
					g.solidClipScratch = filtered
					rowSpans = filtered
					if len(rowSpans) == 0 {
						continue
					}
				}
				g.drawBillboardRowSpans(row, ty, tw, x0, txLUT, rowSpans, src32, srcIndexed, srcMask, shadeMul, shadeRow)
				continue
			}
			for x := x0; x <= x1; {
				if x+1 <= x1 {
					in0 := xInSolidSpans(x, it.clipSpans)
					in1 := xInSolidSpans(x+1, it.clipSpans)
					if !in0 && !in1 {
						x += 2
						continue
					}
					i0 := row + x
					i1 := i0 + 1
					occ0 := !in0 || g.spriteOccludedDepthQAt(depthPix, depthPlanePix, stamp, depthQ, planeBiasQ, i0)
					occ1 := !in1 || g.spriteOccludedDepthQAt(depthPix, depthPlanePix, stamp, depthQ, planeBiasQ, i1)
					if !occ0 && !occ1 {
						p0 := src32[ty*tw+txLUT[x-x0]]
						p1 := src32[ty*tw+txLUT[x+1-x0]]
						a0 := ((p0 >> pixelAShift) & 0xFF) != 0
						a1 := ((p1 >> pixelAShift) & 0xFF) != 0
						if a0 && a1 {
							if it.shadow {
								g.writeFuzzPixel(x, y, i0)
								g.writeFuzzPixel(x+1, y, i1)
							} else {
								g.writeWallPixelPair(i0, shadePackedRGBA(p0, shadeMul), shadePackedRGBA(p1, shadeMul))
							}
							g.setDepthPixelPairEncoded(i0, depthPacked)
							x += 2
							continue
						}
						if a0 {
							if it.shadow {
								g.writeFuzzPixel(x, y, i0)
							} else {
								g.writeWallPixel(i0, shadePackedRGBA(p0, shadeMul))
							}
							g.setDepthPixelEncoded(i0, depthPacked)
						}
						if a1 {
							if it.shadow {
								g.writeFuzzPixel(x+1, y, i1)
							} else {
								g.writeWallPixel(i1, shadePackedRGBA(p1, shadeMul))
							}
							g.setDepthPixelEncoded(i1, depthPacked)
						}
						x += 2
						continue
					}
					if !occ0 {
						p0 := src32[ty*tw+txLUT[x-x0]]
						if ((p0 >> pixelAShift) & 0xFF) != 0 {
							if it.shadow {
								g.writeFuzzPixel(x, y, i0)
							} else {
								g.writeWallPixel(i0, shadePackedRGBA(p0, shadeMul))
							}
							g.setDepthPixelEncoded(i0, depthPacked)
						}
					}
					if !occ1 {
						p1 := src32[ty*tw+txLUT[x+1-x0]]
						if ((p1 >> pixelAShift) & 0xFF) != 0 {
							if it.shadow {
								g.writeFuzzPixel(x+1, y, i1)
							} else {
								g.writeWallPixel(i1, shadePackedRGBA(p1, shadeMul))
							}
							g.setDepthPixelEncoded(i1, depthPacked)
						}
					}
					x += 2
					continue
				}
				i := row + x
				if !xInSolidSpans(x, it.clipSpans) {
					x++
					continue
				}
				if !g.spriteOccludedDepthQAt(depthPix, depthPlanePix, stamp, depthQ, planeBiasQ, i) {
					p := src32[ty*tw+txLUT[x-x0]]
					if ((p >> pixelAShift) & 0xFF) != 0 {
						if it.shadow {
							g.writeFuzzPixel(x, y, i)
						} else {
							g.writeWallPixel(i, shadePackedRGBA(p, shadeMul))
						}
						g.setDepthPixelEncoded(i, depthPacked)
					}
				}
				x++
			}
		}
	}
}

func (g *game) drawShadowMonsterSourcePort(
	it projectedMonsterItem,
	src32 []uint32,
	tw int,
	txLUT, tyLUT []int,
	x0, x1, y0, y1 int,
	depthQ uint16,
	depthPacked uint32,
	stamp uint16,
	planeBiasQ uint16,
	depthPix, depthPlanePix []uint32,
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
					if g.spriteOccludedDepthQAt(depthPix, depthPlanePix, stamp, depthQ, planeBiasQ, i) {
						continue
					}
					g.writeWallPixel(i, fuzzPix)
					g.setDepthPixelEncoded(i, depthPacked)
				}
			}
		}
	}
}

func (g *game) drawBillboardWorldThingsToBuffer(camX, camY, camAng, focal, near float64) {
	const planeDepthBias = 64.0
	planeBiasQ := encodeDepthBiasQ(planeDepthBias)
	useDepth := g.depthOcclusionEnabled()
	depthPix := g.depthPix3D
	depthPlanePix := g.depthPlanePix3D
	wallPix := g.wallPix32
	viewW := g.viewW
	viewH := g.viewH
	stamp := g.depthFrameStamp
	if len(wallPix) != viewW*viewH {
		return
	}
	if useDepth && len(depthPix) != viewW*viewH {
		return
	}
	replay := g.billboardReplayActive && g.billboardReplayKind == billboardQueueWorldThings
	var items []projectedThingItem
	if replay {
		i := g.billboardReplayIndex
		if i < 0 || i >= len(g.thingItemsScratch) {
			return
		}
		items = g.thingItemsScratch[i : i+1]
	} else {
		items = g.ensureThingItemsScratch(64)
	}
	ca := math.Cos(camAng)
	sa := math.Sin(camAng)
	eyeZ := g.playerEyeZ()
	animTickUnits, animUnitsPerTic := g.worldThingAnimTickUnits()
	if !replay {
		for i, th := range g.m.Things {
			if i < 0 || i >= len(g.thingCollected) || g.thingCollected[i] {
				continue
			}
			if isMonster(th.Type) || isPlayerStart(th.Type) {
				continue
			}
			sec := g.thingSectorCached(i, th)
			sprite := g.worldThingSpriteNameScaled(th.Type, animTickUnits, animUnitsPerTic)
			tex, ok := g.monsterSpriteTexture(sprite)
			if !ok || tex.Height <= 0 || tex.Width <= 0 {
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
			h := float64(tex.Height) * scale
			if h <= 0 {
				continue
			}
			sx := float64(viewW)/2 - (s/f)*focal
			w := float64(tex.Width) * scale
			xPad := w/2 + 4
			if sx+xPad < 0 || sx-xPad > float64(viewW) {
				continue
			}
			clipRadius := projectedScreenWidthToWorldRadiusFixed(w, f, focal)
			clipTop, clipBottom, clipOK := g.spriteFootprintClipYBounds(txFixed, tyFixed, clipRadius, viewH, eyeZ, f, focal)
			if !clipOK {
				continue
			}
			lightMul := uint32(256)
			if sec >= 0 && sec < len(g.m.Sectors) {
				lightMul = g.sectorLightMulCached(sec)
			}
			opaque, hasOpaque := g.spriteOpaqueShapeForKey(sprite, tex)
			items = append(items, projectedThingItem{
				dist:       f,
				sx:         sx,
				yb:         yb,
				h:          h,
				clipTop:    clipTop,
				clipBottom: clipBottom,
				tex:        tex,
				lightMul:   lightMul,
				fullBright: worldThingSpriteFullBright(sprite),
				spriteKey:  sprite,
				opaque:     opaque,
				hasOpaque:  hasOpaque,
			})
		}
	}
	if !replay {
		g.thingItemsScratch = items
		sort.Slice(items, func(i, j int) bool { return items[i].dist > items[j].dist })
		if g.billboardQueueCollect {
			for i := range items {
				if !useDepth {
					x0, x1, y0, y1, ok := thingItemScreenBounds(items[i], viewW, viewH)
					if ok {
						depthQ := encodeDepthQ(items[i].dist)
						th := items[i].tex.Height
						if items[i].hasOpaque && len(items[i].opaque.rects) > 0 && th > 0 {
							scale := items[i].h / float64(th)
							dstX := items[i].sx - float64(items[i].tex.OffsetX)*scale
							dstY := items[i].yb - float64(items[i].tex.OffsetY)*scale
							if g.spriteOpaqueRectsFullyOccluded(items[i].opaque.rects, dstX, dstY, scale, items[i].clipTop, items[i].clipBottom, viewW, viewH, depthQ) {
								continue
							}
						}
						if g.spriteWallClipQuadFullyOccluded(x0, x1, y0, y1, depthQ) {
							continue
						}
					}
				}
				g.billboardQueueScratch = append(g.billboardQueueScratch, billboardQueueItem{
					dist: items[i].dist,
					kind: billboardQueueWorldThings,
					idx:  i,
				})
			}
			return
		}
	}
	for _, it := range items {
		depthQ := encodeDepthQ(it.dist)
		depthPacked := packDepthStamped(depthQ, stamp)
		th := it.tex.Height
		tw := it.tex.Width
		if th <= 0 || tw <= 0 {
			continue
		}
		src32, ok32 := spritePixels32(it.tex)
		srcIndexed, srcMask, _ := spriteIndexedPixels(it.tex)
		useIndexed := len(srcIndexed) == len(srcMask) && len(srcIndexed) == tw*th
		if !ok32 && !useIndexed {
			continue
		}
		scale := it.h / float64(th)
		if scale <= 0 {
			continue
		}
		dstX := it.sx - float64(it.tex.OffsetX)*scale
		dstY := it.yb - float64(it.tex.OffsetY)*scale
		dstW := float64(tw) * scale
		dstH := float64(th) * scale
		x0 := int(math.Floor(dstX))
		y0 := int(math.Floor(dstY))
		x1 := int(math.Ceil(dstX+dstW)) - 1
		y1 := int(math.Ceil(dstY+dstH)) - 1
		if x1 < 0 || y1 < 0 || x0 >= viewW || y0 >= viewH {
			continue
		}
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
			continue
		}
		if !useDepth && ((it.hasOpaque && len(it.opaque.rects) > 0 && g.spriteOpaqueRectsFullyOccluded(it.opaque.rects, dstX, dstY, scale, it.clipTop, it.clipBottom, viewW, viewH, depthQ)) || g.spriteWallClipQuadFullyOccluded(x0, x1, y0, y1, depthQ)) {
			continue
		}
		shadeMul := uint32(256)
		if !it.fullBright {
			shadeMul = uint32(combineShadeMul(int(monsterShadeFactor(it.dist, near)*256.0), int(it.lightMul)))
		}
		var shadeRow []uint32
		if useIndexed && wallShadePackedOK {
			if shadeMul > 256 {
				shadeMul = 256
			}
			shadeRow = wallShadePackedLUT[shadeMul][:]
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
		if it.hasOpaque {
			x0, x1, txLUT = trimScreenRangeToOpaqueLUT(x0, x1, txLUT, it.opaque.bounds.minX, it.opaque.bounds.maxX)
			y0, y1, tyLUT = trimScreenRangeToOpaqueLUT(y0, y1, tyLUT, it.opaque.bounds.minY, it.opaque.bounds.maxY)
			if x0 > x1 || y0 > y1 {
				continue
			}
		}
		if !useDepth && len(it.clipSpans) == 0 && !((it.hasOpaque && len(it.opaque.rects) > 0 && g.spriteOpaqueRectsHaveAnyOccluder(it.opaque.rects, dstX, dstY, scale, it.clipTop, it.clipBottom, viewW, viewH, depthQ)) || g.spriteWallClipBBoxHasAnyOccluder(x0, x1, y0, y1, depthQ)) {
			g.drawBillboardSpriteRowsDirect(src32, srcIndexed, srcMask, tw, txLUT, tyLUT, x0, x1, y0, y1, shadeMul)
			continue
		}
		for y := y0; y <= y1; y++ {
			ty := tyLUT[y-y0]
			row := y * viewW
			if len(it.clipSpans) == 0 && x1-x0 >= spriteRowOcclusionMinSpan && g.rowFullyOccludedDepthQ(depthQ, planeBiasQ, row, x0, x1) {
				continue
			}
			if !useDepth {
				rowSpans := g.spriteRowVisibleSpansDepthQ(y, x0, x1, depthQ, it.clipSpans, g.solidClipScratch[:0])
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
						l, r, ok := trimSpanToOpaqueLUTRange(sp.l, sp.r, x0, txLUT, minTex, maxTex)
						if ok {
							filtered = append(filtered, solidSpan{l: l, r: r})
						}
					}
					g.solidClipScratch = filtered
					rowSpans = filtered
					if len(rowSpans) == 0 {
						continue
					}
				}
				g.drawBillboardRowSpans(row, ty, tw, x0, txLUT, rowSpans, src32, srcIndexed, srcMask, shadeMul, shadeRow)
				continue
			}
			for x := x0; x <= x1; {
				if x+1 <= x1 {
					in0 := xInSolidSpans(x, it.clipSpans)
					in1 := xInSolidSpans(x+1, it.clipSpans)
					if !in0 && !in1 {
						x += 2
						continue
					}
					i0 := row + x
					i1 := i0 + 1
					occ0 := !in0 || g.spriteOccludedDepthQAt(depthPix, depthPlanePix, stamp, depthQ, planeBiasQ, i0)
					occ1 := !in1 || g.spriteOccludedDepthQAt(depthPix, depthPlanePix, stamp, depthQ, planeBiasQ, i1)
					if !occ0 && !occ1 {
						p0 := src32[ty*tw+txLUT[x-x0]]
						p1 := src32[ty*tw+txLUT[x+1-x0]]
						a0 := ((p0 >> pixelAShift) & 0xFF) != 0
						a1 := ((p1 >> pixelAShift) & 0xFF) != 0
						if a0 && a1 {
							g.writeWallPixelPair(i0, shadePackedRGBA(p0, shadeMul), shadePackedRGBA(p1, shadeMul))
							g.setDepthPixelPairEncoded(i0, depthPacked)
							x += 2
							continue
						}
						if a0 {
							g.writeWallPixel(i0, shadePackedRGBA(p0, shadeMul))
							g.setDepthPixelEncoded(i0, depthPacked)
						}
						if a1 {
							g.writeWallPixel(i1, shadePackedRGBA(p1, shadeMul))
							g.setDepthPixelEncoded(i1, depthPacked)
						}
						x += 2
						continue
					}
					if !occ0 {
						p0 := src32[ty*tw+txLUT[x-x0]]
						if ((p0 >> pixelAShift) & 0xFF) != 0 {
							if shadeMul >= 256 {
								g.writeWallPixel(i0, p0|pixelOpaqueA)
							} else {
								g.writeWallPixel(i0, shadePackedRGBA(p0, shadeMul))
							}
							g.setDepthPixelEncoded(i0, depthPacked)
						}
					}
					if !occ1 {
						p1 := src32[ty*tw+txLUT[x+1-x0]]
						if ((p1 >> pixelAShift) & 0xFF) != 0 {
							if shadeMul >= 256 {
								g.writeWallPixel(i1, p1|pixelOpaqueA)
							} else {
								g.writeWallPixel(i1, shadePackedRGBA(p1, shadeMul))
							}
							g.setDepthPixelEncoded(i1, depthPacked)
						}
					}
					x += 2
					continue
				}
				i := row + x
				if !xInSolidSpans(x, it.clipSpans) {
					x++
					continue
				}
				if !g.spriteOccludedDepthQAt(depthPix, depthPlanePix, stamp, depthQ, planeBiasQ, i) {
					p := src32[ty*tw+txLUT[x-x0]]
					if ((p >> pixelAShift) & 0xFF) != 0 {
						if shadeMul >= 256 {
							g.writeWallPixel(i, p|pixelOpaqueA)
						} else {
							g.writeWallPixel(i, shadePackedRGBA(p, shadeMul))
						}
						g.setDepthPixelEncoded(i, depthPacked)
					}
				}
				x++
			}
		}
	}
}

func (g *game) worldThingAnimTickUnits() (tickUnits int, unitsPerTic int) {
	unitsPerTic = 1
	tickUnits = g.worldTic
	if g == nil || !g.opts.SourcePortMode || sourcePortThingAnimSubsteps <= 1 {
		return tickUnits, unitsPerTic
	}
	unitsPerTic = sourcePortThingAnimSubsteps
	alpha := g.interpAlpha()
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
		return pick("SKULL0")
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
	case 70:
		return pickState(
			thingAnimState{name: "FCANA0", tics: 4},
			thingAnimState{name: "FCANB0", tics: 4},
			thingAnimState{name: "FCANC0", tics: 4},
		)
	case 72:
		return pickState(thingAnimState{name: "KEENA0", tics: -1})
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
		if _, ok := g.opts.SpritePatchBank[name]; !ok {
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
		if _, ok := g.opts.SpritePatchBank[name]; ok {
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
			if _, ok := g.opts.SpritePatchBank[name]; ok {
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
			if _, ok := g.opts.SpritePatchBank[key]; ok {
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
		if _, ok := g.opts.SpritePatchBank[key]; ok {
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

func (g *game) monsterSpriteTexture(name string) (WallTexture, bool) {
	key := strings.ToUpper(strings.TrimSpace(name))
	if key == "" {
		return WallTexture{}, false
	}
	p, ok := g.opts.SpritePatchBank[key]
	if !ok || p.Width <= 0 || p.Height <= 0 {
		return WallTexture{}, false
	}
	n := p.Width * p.Height
	if len(p.RGBA) != n*4 && (len(p.Indexed) != n || len(p.OpaqueMask) != n) {
		return WallTexture{}, false
	}
	if tex, built := synthesizeIndexedSpriteTexture(p); built {
		p = tex
		if g.opts.SpritePatchBank != nil {
			g.opts.SpritePatchBank[key] = p
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
			if _, ok := g.opts.SpritePatchBank[name]; ok {
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
	case 16:
		return pick("CYBRA1", "CYBRB1", "CYBRC1", "CYBRD1")
	case 7:
		return pick("SPIDA1", "SPIDB1", "SPIDC1", "SPIDD1")
	default:
		return ""
	}
}

func monsterAttackFrameSeq(typ int16) []byte {
	switch typ {
	case 3004, 9, 3001, 3002, 3006, 3005, 3003:
		return []byte{'E', 'F'}
	case 16:
		return []byte{'E', 'F', 'G'}
	case 7:
		return []byte{'E', 'F'}
	default:
		return nil
	}
}

func monsterAttackFrameTics(typ int16) []int {
	switch typ {
	case 3004: // zombieman
		return []int{10, 8}
	case 9: // shotgun guy
		return []int{10, 8}
	case 3001: // imp
		return []int{8, 8}
	case 3002: // demon
		return []int{8, 8}
	case 3006: // lost soul
		return []int{6, 6}
	case 3005: // cacodemon
		return []int{8, 8}
	case 3003: // baron
		return []int{8, 8}
	case 16: // cyberdemon
		return []int{8, 8, 8}
	case 7: // spider mastermind
		return []int{8, 8}
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
	case 3004, 9, 3001, 3002, 58, 3006, 3005, 3003, 16, 7:
		return []byte{'G'}
	default:
		return nil
	}
}

func monsterPainFrameTics(typ int16) []int {
	switch typ {
	case 16:
		return []int{10}
	case 7:
		return []int{8}
	case 3004, 9, 3001, 3002, 58, 3006, 3005, 3003:
		return []int{6}
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

func monsterDeathFrameSeq(typ int16) []byte {
	switch typ {
	case 3004:
		return []byte{'H', 'I', 'J', 'K', 'L'}
	case 9:
		return []byte{'H', 'I', 'J', 'K', 'L'}
	case 3001:
		return []byte{'I', 'J', 'K', 'L', 'M'}
	case 3002, 58:
		return []byte{'I', 'J', 'K', 'L', 'M', 'N'}
	case 3006:
		return []byte{'F', 'G', 'H', 'I', 'J', 'K'}
	case 3005:
		return []byte{'G', 'H', 'I', 'J', 'K', 'L'}
	case 3003:
		return []byte{'I', 'J', 'K', 'L', 'M', 'N', 'O'}
	case 16:
		return []byte{'H', 'I', 'J', 'K', 'L', 'M', 'N', 'O', 'P'}
	case 7:
		return []byte{'J', 'K', 'L', 'M', 'N', 'O', 'P', 'Q', 'R', 'S'}
	default:
		return nil
	}
}

func monsterDeathFrameTics(typ int16) []int {
	switch typ {
	case 3004:
		return []int{5, 5, 5, 5, 5}
	case 9:
		return []int{5, 5, 5, 5, 5}
	case 3001:
		return []int{8, 8, 6, 6, 6}
	case 3002, 58:
		return []int{8, 8, 4, 4, 4, 4}
	case 3006:
		return []int{6, 6, 6, 6, 6, 6}
	case 3005:
		return []int{8, 8, 8, 8, 8, 8}
	case 3003:
		return []int{8, 8, 8, 8, 8, 8, 8}
	case 16:
		return []int{10, 10, 10, 10, 10, 10, 10, 10, 30}
	case 7:
		return []int{20, 10, 10, 10, 10, 10, 10, 10, 10, 30}
	default:
		return nil
	}
}

func monsterDeathSoundDelayTics(typ int16) int {
	// Doom plays death sounds on A_Scream, which is usually the 2nd death frame.
	switch typ {
	case 3004, 9:
		return 5
	case 3001, 3002, 58, 3003, 3005:
		return 8
	case 3006:
		return 6
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
	tics := monsterDeathFrameTics(typ)
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
		seq := monsterDeathFrameSeq(th.Type)
		frameTics := monsterDeathFrameTics(th.Type)
		if len(seq) > 0 && len(seq) == len(frameTics) {
			total := monsterDeathAnimTotalTics(th.Type)
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
	return byte('A' + ((tic / 8) & 3))
}

func (g *game) monsterSpriteNameForView(i int, th mapdata.Thing, tic int, viewX, viewY float64) (string, bool) {
	prefix, ok := monsterSpritePrefix(th.Type)
	if !ok {
		return g.monsterSpriteName(th.Type, tic), false
	}
	frameLetter := g.monsterFrameLetter(i, th, tic)
	if i >= 0 && i < len(g.thingDead) && g.thingDead[i] {
		name0 := spriteFrameName(prefix, frameLetter, '0')
		if _, ok := g.opts.SpritePatchBank[name0]; ok {
			return name0, false
		}
	}
	fx, fy := g.thingPosFixed(i, th)
	rot := monsterSpriteRotationIndexAt(th, float64(fx)/fracUnit, float64(fy)/fracUnit, viewX, viewY)
	if name, flip, ok := g.monsterSpriteRotFrame(prefix, frameLetter, rot); ok {
		return name, flip
	}
	if name, flip, ok := g.monsterSpriteRotFrame(prefix, frameLetter, 1); ok {
		return name, flip
	}
	return g.monsterSpriteName(th.Type, tic), false
}

func monsterSpriteRotationIndex(th mapdata.Thing, viewX, viewY float64) int {
	return monsterSpriteRotationIndexAt(th, float64(th.X), float64(th.Y), viewX, viewY)
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
	case 16:
		return "CYBR", true
	case 7:
		return "SPID", true
	default:
		return "", false
	}
}

func monsterSpriteRotationIndexAt(th mapdata.Thing, thingX, thingY, viewX, viewY float64) int {
	facing := normalizeDeg360(float64(th.Angle))
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
	if _, ok := g.opts.SpritePatchBank[name]; ok {
		return name, false, true
	}
	if rot == 1 {
		return "", false, false
	}
	opp := 10 - rot
	oppDigit := byte('0' + opp)
	// Doom paired-rotation patch, e.g. TROOA2A8.
	pairA := spriteFramePairName(prefix, frame, rotDigit, oppDigit)
	if _, ok := g.opts.SpritePatchBank[pairA]; ok {
		return pairA, false, true
	}
	// Reverse order pair (some content uses the opposite declaration order).
	pairB := spriteFramePairName(prefix, frame, oppDigit, rotDigit)
	if _, ok := g.opts.SpritePatchBank[pairB]; ok {
		return pairB, true, true
	}
	return "", false, false
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
	if len(name) < 5 {
		return false
	}
	prefix := strings.ToUpper(name[:4])
	frame := name[4]
	switch prefix {
	case "POSS", "SPOS":
		return frame == 'E'
	case "TROO", "HEAD", "BOSS", "SPID":
		return frame == 'F'
	case "CYBR":
		return frame == 'F' || frame == 'G'
	default:
		return false
	}
}

func worldThingSpriteFullBright(name string) bool {
	if len(name) < 5 {
		return false
	}
	prefix := strings.ToUpper(name[:4])
	frame := byte(unicode.ToUpper(rune(name[4])))
	switch prefix {
	case "SOUL", "PINV", "PSTR", "PINS", "MEGA", "SUIT", "PMAP", "COLU", "TBLU", "TGRN", "TRED", "SMBT", "SMGT", "SMRT", "CAND", "CBRA", "CEYE", "FSKU", "TLMP", "TLP2", "POL3", "FCAN":
		return true
	case "ARM1", "ARM2", "BKEY", "RKEY", "YKEY", "BSKU", "RSKU", "YSKU":
		return frame == 'B'
	case "PVIS":
		return frame == 'A'
	default:
		return false
	}
}

func monsterRenderHeight(typ int16) float64 {
	switch typ {
	case 3002:
		return 56
	case 3006:
		return 56
	case 3005:
		return 56
	case 3003:
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
		if !d.visible {
			continue
		}
		pl := g.lines[pi]
		x1, y1 := g.worldToScreen(float64(pl.x1)/fracUnit, float64(pl.y1)/fracUnit)
		x2, y2 := g.worldToScreen(float64(pl.x2)/fracUnit, float64(pl.y2)/fracUnit)
		vector.StrokeLine(screen, float32(x1), float32(y1), float32(x2), float32(y2), 2.4, wallUseSpecial, true)
	}
}

func buttonHighlightEligible(special uint16) bool {
	if special == 0 {
		return false
	}
	info := mapdata.LookupLineSpecial(special)
	return info.Trigger == mapdata.TriggerUse
}

func (g *game) drawDeathOverlay(screen *ebiten.Image) {
	ebitenutil.DrawRect(screen, 0, 0, float64(g.viewW), float64(g.viewH), color.RGBA{R: 25, G: 0, B: 0, A: 130})
	msg1 := "YOU DIED"
	msg2 := "PRESS ENTER TO RESTART"
	x1 := g.viewW/2 - len(msg1)*7/2
	x2 := g.viewW/2 - len(msg2)*7/2
	y := g.viewH / 2
	ebitenutil.DebugPrintAt(screen, msg1, x1, y)
	ebitenutil.DebugPrintAt(screen, msg2, x2, y+16)
}

func (g *game) drawFlashOverlay(screen *ebiten.Image) {
	if g.damageFlashTic > 0 {
		a := uint8(40 + min(120, g.damageFlashTic*8))
		ebitenutil.DrawRect(screen, 0, 0, float64(g.viewW), float64(g.viewH), color.RGBA{R: 180, G: 20, B: 20, A: a})
	}
	if g.bonusFlashTic > 0 {
		a := uint8(20 + min(80, g.bonusFlashTic*6))
		ebitenutil.DrawRect(screen, 0, 0, float64(g.viewW), float64(g.viewH), color.RGBA{R: 210, G: 190, B: 80, A: a})
	}
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
		g.resetSkyLayerPipeline(true)
	}
}

func (g *game) Layout(outsideWidth, outsideHeight int) (int, int) {
	if g.opts.SourcePortMode {
		w := max(outsideWidth, 1)
		h := max(outsideHeight, 1)
		if w != g.viewW || h != g.viewH {
			oldFit := g.fitZoom
			g.viewW = w
			g.viewH = h
			worldW := math.Max(g.bounds.maxX-g.bounds.minX, 1)
			worldH := math.Max(g.bounds.maxY-g.bounds.minY, 1)
			margin := 0.9
			zx := float64(g.viewW) * margin / worldW
			zy := float64(g.viewH) * margin / worldH
			g.fitZoom = math.Max(math.Min(zx, zy), 0.0001)
			if oldFit > 0 {
				g.zoom = (g.zoom / oldFit) * g.fitZoom
			} else {
				g.zoom = g.fitZoom * doomInitialZoomMul
			}
			// Resolution changes can invalidate shader-side projection assumptions.
			// Rebuild full sky GPU pipeline (shader + textures + caches).
			g.resetSkyLayerPipeline(true)
			g.mouseLookSet = false
			g.mouseLookSuppressTicks = detailMouseSuppressTicks
			g.syncRenderState()
		}
		return g.viewW, g.viewH
	}
	if g.viewW < 1 {
		g.viewW = 1
	}
	if g.viewH < 1 {
		g.viewH = 1
	}
	return g.viewW, g.viewH
}

func (g *game) worldToScreen(x, y float64) (float64, float64) {
	dx := x - g.renderCamX
	dy := y - g.renderCamY
	if g.mapRotationActive() {
		rot := (math.Pi / 2) - angleToRadians(g.renderAngle)
		cr := math.Cos(rot)
		sr := math.Sin(rot)
		rdx := dx*cr - dy*sr
		rdy := dx*sr + dy*cr
		dx = rdx
		dy = rdy
	}
	sx := dx*g.zoom + float64(g.viewW)/2
	sy := float64(g.viewH)/2 - dy*g.zoom
	return sx, sy
}

func (g *game) screenToWorld(sx, sy float64) (float64, float64) {
	dx := (sx - float64(g.viewW)/2) / g.zoom
	dy := (float64(g.viewH)/2 - sy) / g.zoom
	if g.mapRotationActive() {
		rot := (math.Pi / 2) - angleToRadians(g.renderAngle)
		cr := math.Cos(rot)
		sr := math.Sin(rot)
		// Inverse of worldToScreen's rotation.
		wdx := dx*cr + dy*sr
		wdy := -dx*sr + dy*cr
		dx = wdx
		dy = wdy
	}
	return g.renderCamX + dx, g.renderCamY + dy
}

func (g *game) ensureMapFloorLayer() {
	need := g.viewW * g.viewH * 4
	if g.mapFloorLayer == nil || g.mapFloorW != g.viewW || g.mapFloorH != g.viewH || len(g.mapFloorPix) != need {
		g.mapFloorLayer = ebiten.NewImageWithOptions(image.Rect(0, 0, g.viewW, g.viewH), &ebiten.NewImageOptions{
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
		g.wallLayer = ebiten.NewImageWithOptions(image.Rect(0, 0, g.viewW, g.viewH), &ebiten.NewImageOptions{
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
		g.maskedClipCols = make([][]maskedClipSpan, w)
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

func (g *game) wallTexture(name string) (WallTexture, bool) {
	key, _ := g.resolveAnimatedWallSample(name)
	if key == "" || key == "-" {
		return WallTexture{}, false
	}
	tex, ok := g.opts.WallTexBank[key]
	if !ok || tex.Width <= 0 || tex.Height <= 0 || len(tex.RGBA) != tex.Width*tex.Height*4 {
		return WallTexture{}, false
	}
	return tex, true
}

func skyTextureForMap(mapName mapdata.MapName, wallTexBank map[string]WallTexture) (WallTexture, bool) {
	_, tex, ok := skyTextureEntryForMap(mapName, wallTexBank)
	return tex, ok
}

func skyTextureEntryForMap(mapName mapdata.MapName, wallTexBank map[string]WallTexture) (string, WallTexture, bool) {
	for _, name := range skyTextureCandidates(mapName) {
		key := normalizeFlatName(name)
		tex, ok := wallTexBank[key]
		if !ok || tex.Width <= 0 || tex.Height <= 0 || len(tex.RGBA) != tex.Width*tex.Height*4 {
			continue
		}
		return key, tex, true
	}
	return "", WallTexture{}, false
}

func skyTextureCandidates(mapName mapdata.MapName) []string {
	name := strings.ToUpper(strings.TrimSpace(string(mapName)))
	out := make([]string, 0, 5)
	add := func(c string) {
		c = normalizeFlatName(c)
		if c == "" {
			return
		}
		for _, ex := range out {
			if ex == c {
				return
			}
		}
		out = append(out, c)
	}
	if len(name) == 4 && name[0] == 'E' && name[2] == 'M' && name[1] >= '0' && name[1] <= '9' {
		switch int(name[1] - '0') {
		case 1:
			add("SKY1")
		case 2:
			add("SKY2")
		case 3:
			add("SKY3")
		case 4:
			add("SKY4")
		}
	}
	if strings.HasPrefix(name, "MAP") && len(name) >= 5 {
		if n, err := strconv.Atoi(name[3:]); err == nil {
			switch {
			case n >= 1 && n <= 11:
				add("SKY1")
			case n >= 12 && n <= 20:
				add("SKY2")
			case n >= 21:
				add("SKY3")
			}
		}
	}
	add("SKY1")
	add("SKY2")
	add("SKY3")
	add("SKY4")
	return out
}

func skySampleUV(screenX, screenY, viewW, viewH int, focal, camAngle float64, texW, texH int) (u, v int) {
	if texW <= 0 || texH <= 0 {
		return 0, 0
	}
	if focal <= 1e-6 {
		focal = 1
	}
	angle := skySampleAngle(screenX, viewW, focal, camAngle)
	uScale := float64(texW*4) / (2 * math.Pi)
	u = wrapIndex(int(math.Floor(angle*uScale)), texW)

	cy := float64(viewH) * 0.5
	if cy <= 1e-6 {
		return u, 0
	}
	yn := (float64(screenY) + 0.5) / cy
	if yn < 0 {
		yn = 0
	}
	if yn > 1 {
		yn = 1
	}
	v = int(math.Floor(yn * float64(texH-1)))
	if v < 0 {
		v = 0
	}
	if v >= texH {
		v = texH - 1
	}
	return u, v
}

func skyProjectedSampleIndex(drawCoord, drawSize, sampleSize int) int {
	if drawSize <= 0 || sampleSize <= 0 {
		return 0
	}
	sample := int((float64(drawCoord) + 0.5) * float64(sampleSize) / float64(drawSize))
	if sample < 0 {
		return 0
	}
	if sample >= sampleSize {
		return sampleSize - 1
	}
	return sample
}

func skyProjectedSampleUV(drawX, drawY, drawW, drawH, sampleW, sampleH int, focal, camAngle float64, texW, texH int) (u, v int) {
	if drawW <= 0 || drawH <= 0 || sampleW <= 0 || sampleH <= 0 || texW <= 0 || texH <= 0 {
		return 0, 0
	}
	sampleX := skyProjectedSampleIndex(drawX, drawW, sampleW)
	sampleY := skyProjectedSampleIndex(drawY, drawH, sampleH)
	angle := skySampleAngle(sampleX, sampleW, focal, camAngle)
	uScale := float64(texW*4) / (2 * math.Pi)
	u = wrapIndex(int(math.Floor(angle*uScale)), texW)
	iscale := doomSkyIScale(sampleW)
	frac := 100.0 + ((float64(sampleY) - float64(sampleH)*0.5) * iscale)
	v = wrapIndex(int(math.Floor(frac)), texH)
	return u, v
}

func skySampleAngle(screenX, viewW int, focal, camAngle float64) float64 {
	if focal <= 1e-6 {
		focal = 1
	}
	cx := float64(viewW) * 0.5
	sampleX := float64(screenX) + 0.5
	// Match wall projection sign convention: screen x = cx - tan(rel)*focal,
	// so rel = atan((cx-x)/focal). Using this keeps sky panning direction aligned.
	return camAngle + math.Atan((cx-sampleX)/focal)
}

func effectiveSkyTexHeight(tex WallTexture) int {
	if tex.Width <= 0 || tex.Height <= 0 || len(tex.RGBA) != tex.Width*tex.Height*4 {
		return 1
	}
	for y := tex.Height - 1; y >= 0; y-- {
		rowStart := y * tex.Width * 4
		opaque := false
		for x := 0; x < tex.Width; x++ {
			if tex.RGBA[rowStart+x*4+3] != 0 {
				opaque = true
				break
			}
		}
		if opaque {
			return y + 1
		}
	}
	return 1
}

func (g *game) beginSkyLayerFrame() {
	g.skyLayerFrameActive = false
}

func (g *game) resetSkyLayerPipeline(rebuildShader bool) {
	g.skyLayerFrameActive = false
	g.skyLayerTex = nil
	g.skyLayerTexKey = ""
	g.skyLayerTexW = 0
	g.skyLayerTexH = 0

	// Clear sky lookup caches so the next frame recomputes against current
	// resolution/focal/texture parameters.
	g.skyColUCache = nil
	g.skyColViewW = 0
	g.skyAngleOff = nil
	g.skyAngleViewW = 0
	g.skyAngleFocal = 0
	g.skyRowVCache = nil
	g.skyRowViewH = 0
	g.skyRowTexH = 0
	g.skyRowIScale = 0
	g.skyRowDrawCache = nil
	g.skyRowDrawH = 0
	g.skyLayerProjDrawW = 0
	g.skyLayerProjDrawH = 0
	g.skyLayerProjSampleW = 0
	g.skyLayerProjSampleH = 0

	if rebuildShader && g.opts.GPUSky && g.opts.SourcePortMode {
		g.skyLayerShader = nil
		if sh, err := ebiten.NewShader(skyBackdropShaderSrc); err == nil {
			g.skyLayerShader = sh
		}
	}
}

func (g *game) enableSkyLayerFrame(camAng, focal float64, texKey string, tex WallTexture, texH int) bool {
	if g.skyLayerShader == nil || !g.opts.SourcePortMode {
		return false
	}
	if texKey == "" || tex.Width <= 0 || tex.Height <= 0 || len(tex.RGBA) != tex.Width*tex.Height*4 {
		return false
	}
	drawW, drawH, sampleW, sampleH := g.skyProjectionSize()
	if g.skyLayerProjDrawW != drawW || g.skyLayerProjDrawH != drawH || g.skyLayerProjSampleW != sampleW || g.skyLayerProjSampleH != sampleH {
		g.skyLayerTex = nil
		g.skyLayerTexKey = ""
		g.skyLayerTexW = 0
		g.skyLayerTexH = 0
		g.skyLayerProjDrawW = drawW
		g.skyLayerProjDrawH = drawH
		g.skyLayerProjSampleW = sampleW
		g.skyLayerProjSampleH = sampleH
	}
	if g.skyLayerTex == nil || g.skyLayerTexKey != texKey || g.skyLayerTexW != tex.Width || g.skyLayerTexH != tex.Height {
		img := ebiten.NewImage(tex.Width, tex.Height)
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
	v := []ebiten.Vertex{
		{DstX: 0, DstY: 0, SrcX: 0, SrcY: 0, ColorR: 1, ColorG: 1, ColorB: 1, ColorA: 1, Custom0: 0, Custom1: 0},
		{DstX: float32(w), DstY: 0, SrcX: float32(texW), SrcY: 0, ColorR: 1, ColorG: 1, ColorB: 1, ColorA: 1, Custom0: float32(w), Custom1: 0},
		{DstX: 0, DstY: float32(h), SrcX: 0, SrcY: float32(texH), ColorR: 1, ColorG: 1, ColorB: 1, ColorA: 1, Custom0: 0, Custom1: float32(h)},
		{DstX: float32(w), DstY: float32(h), SrcX: float32(texW), SrcY: float32(texH), ColorR: 1, ColorG: 1, ColorB: 1, ColorA: 1, Custom0: float32(w), Custom1: float32(h)},
	}
	idx := []uint16{0, 1, 2, 1, 2, 3}
	op := &ebiten.DrawTrianglesShaderOptions{}
	op.Images[0] = g.skyLayerTex
	_, _, sampleW, sampleH := g.skyProjectionSize()
	op.Uniforms = map[string]any{
		"CamAngle": g.skyLayerFrameCamAng,
		"Focal":    g.skyLayerFrameFocal,
		"DrawW":    float64(w),
		"DrawH":    float64(h),
		"SampleW":  float64(sampleW),
		"SampleH":  float64(sampleH),
		"SkyTexW":  float64(texW),
		"SkyTexH":  g.skyLayerFrameTexH,
		"SharpUpscale": func() float64 {
			if g.opts.SkyUpscaleMode == "sharp" {
				return 1
			}
			return 0
		}(),
	}
	screen.DrawTrianglesShader(v, idx, g.skyLayerShader, op)
	return true
}

func (g *game) buildPlaneSpansParallel(planes []*plane3DVisplane, viewH int) ([][]plane3DSpan, int, int, bool) {
	spansByPlane := make([][]plane3DSpan, len(planes))
	if len(planes) == 0 {
		return spansByPlane, 0, 0, false
	}
	for i := range planes {
		spansByPlane[i] = makePlane3DSpans(planes[i], viewH, nil)
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
	for i, si := range visible {
		out[i] = g.buildWallSegPrepassSingle(si, camX, camY, ca, sa, focal, near)
	}
	return out
}

func (g *game) buildWallSegPrepassSingle(si int, camX, camY, ca, sa, focal, near float64) wallSegPrepass {
	pp := wallSegPrepass{
		segIdx:          si,
		frontSideDefIdx: -1,
	}
	cacheOK := si >= 0 && si < len(g.wallSegStaticCache) && g.wallSegStaticCache[si].valid
	var (
		ld                 mapdata.Linedef
		x1w, y1w, x2w, y2w float64
		u1, u2             float64
		hasTwoSidedMid     bool
		frontSectorIdx     = -1
		backSectorIdx      = -1
	)
	if cacheOK {
		c := g.wallSegStaticCache[si]
		ld = c.ld
		x1w, y1w, x2w, y2w = c.x1w, c.y1w, c.x2w, c.y2w
		pp.frontSideDefIdx = c.frontSideDefIdx
		u1 = c.uBase
		u2 = u1 + c.segLen
		if c.frontSide == 1 {
			u2 = u1 - c.segLen
		}
		hasTwoSidedMid = c.hasTwoSidedMidTex
		frontSectorIdx = c.frontSectorIdx
		backSectorIdx = c.backSectorIdx
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
		x1w, y1w, x2w, y2w, ok = g.segWorldEndpoints(si)
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
		u1 = float64(seg.Offset)
		if pp.frontSideDefIdx >= 0 {
			u1 += float64(g.m.Sidedefs[pp.frontSideDefIdx].TextureOffset)
		}
		u2 = u1 + segLen
		if frontSide == 1 {
			u2 = u1 - segLen
		}
		hasTwoSidedMid = g.segHasTwoSidedMidTexture(si)
		frontSectorIdx = g.sectorIndexFromSideNum(ld.SideNum[frontSide])
		backSectorIdx = g.sectorIndexFromSideNum(ld.SideNum[backSide])
	}
	pp.ld = ld
	d := g.linedefDecisionPseudo3D(ld)
	portalSplit := false
	if frontSectorIdx >= 0 && backSectorIdx >= 0 &&
		frontSectorIdx < len(g.m.Sectors) && backSectorIdx < len(g.m.Sectors) {
		front := &g.m.Sectors[frontSectorIdx]
		back := &g.m.Sectors[backSectorIdx]
		portalSplit = front.FloorHeight != back.FloorHeight ||
			front.CeilingHeight != back.CeilingHeight ||
			normalizeFlatName(front.FloorPic) != normalizeFlatName(back.FloorPic) ||
			normalizeFlatName(front.CeilingPic) != normalizeFlatName(back.CeilingPic) ||
			(front.Light != back.Light && doomSectorLighting)
	}
	if !d.visible && !hasTwoSidedMid && !portalSplit {
		return pp
	}
	x1 := x1w - camX
	y1 := y1w - camY
	x2 := x2w - camX
	y2 := y2w - camY
	f1 := x1*ca + y1*sa
	s1 := -x1*sa + y1*ca
	f2 := x2*ca + y2*sa
	s2 := -x2*sa + y2*ca
	origF1, origS1, origF2, origS2 := f1, s1, f2, s2
	preSX1 := float64(g.viewW) / 2
	preSX2 := float64(g.viewW) / 2
	if math.Abs(origF1) > 1e-9 {
		preSX1 -= (origS1 / origF1) * focal
	}
	if math.Abs(origF2) > 1e-9 {
		preSX2 -= (origS2 / origF2) * focal
	}
	var ok bool
	f1, s1, u1, f2, s2, u2, ok = clipSegmentToNearWithAttr(f1, s1, u1, f2, s2, u2, near)
	if !ok {
		pp.logReason = "BEHIND"
		pp.logZ1, pp.logZ2, pp.logX1, pp.logX2 = origF1, origF2, preSX1, preSX2
		return pp
	}
	if f1*s2-s1*f2 >= 0 {
		pp.logReason = "BACKFACE"
		pp.logZ1, pp.logZ2, pp.logX1, pp.logX2 = f1, f2, s1, s2
		return pp
	}
	sx1 := float64(g.viewW)/2 - (s1/f1)*focal
	sx2 := float64(g.viewW)/2 - (s2/f2)*focal
	if !isFinite(sx1) || !isFinite(sx2) {
		pp.logReason = "FLIPPED"
		pp.logZ1, pp.logZ2, pp.logX1, pp.logX2 = f1, f2, sx1, sx2
		return pp
	}
	minSX := int(math.Floor(math.Min(sx1, sx2)))
	maxSX := int(math.Ceil(math.Max(sx1, sx2)))
	if minSX < 0 {
		minSX = 0
	}
	if maxSX >= g.viewW {
		maxSX = g.viewW - 1
	}
	if minSX > maxSX {
		pp.logReason = "OFFSCREEN"
		pp.logZ1, pp.logZ2, pp.logX1, pp.logX2 = f1, f2, sx1, sx2
		return pp
	}
	invF1 := 1.0 / f1
	invF2 := 1.0 / f2
	pp.sx1 = sx1
	pp.sx2 = sx2
	pp.minSX = minSX
	pp.maxSX = maxSX
	pp.invF1 = invF1
	pp.invF2 = invF2
	pp.uOverF1 = u1 * invF1
	pp.uOverF2 = u2 * invF2
	pp.logZ1, pp.logZ2, pp.logX1, pp.logX2 = f1, f2, sx1, sx2
	pp.ok = true
	return pp
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
	uScale := float64(texW*4) / (2 * math.Pi)
	col := g.ensureSkyColBuffer(viewW)
	// Sky column lookup is lightweight and fully cached by size/fov.
	// Keep this serial to avoid worker/scheduling overhead.
	for x := 0; x < viewW; x++ {
		sampleX := skyProjectedSampleIndex(x, viewW, sampleW)
		angle := camAngle + angleOff[sampleX]
		col[x] = wrapIndex(int(math.Floor(angle*uScale)), texW)
	}
	for y := 0; y < viewH; y++ {
		sampleY := skyProjectedSampleIndex(y, viewH, sampleH)
		row[y] = sampleRow[sampleY]
	}
	return col, row
}

func (g *game) ensureSkyColBuffer(viewW int) []int {
	if viewW <= 0 {
		return nil
	}
	if len(g.skyColUCache) != viewW || g.skyColViewW != viewW {
		g.skyColUCache = make([]int, viewW)
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
	off := make([]float64, viewW)
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
	row := make([]int, viewH)
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
		g.skyRowDrawCache = make([]int, drawH)
		g.skyRowDrawH = drawH
	}
	return g.skyRowDrawCache
}

func (g *game) skyProjectionSize() (drawW, drawH, sampleW, sampleH int) {
	drawW = max(g.viewW, 1)
	drawH = max(g.viewH, 1)
	sampleW = drawW
	sampleH = drawH
	if g.opts.SourcePortMode {
		if g.skyOutputW > 0 {
			sampleW = g.skyOutputW
		}
		if g.skyOutputH > 0 {
			sampleH = g.skyOutputH
		}
		if sampleW < 1 {
			sampleW = 1
		}
		if sampleH < 1 {
			sampleH = 1
		}
	}
	return drawW, drawH, sampleW, sampleH
}

func doomSkyIScale(viewW int) float64 {
	if viewW <= 0 {
		return 1
	}
	// Doom sky columns use dc_iscale = pspriteiscale>>detailshift.
	// In standard detail this is roughly SCREENWIDTH/viewwidth (320/viewwidth).
	return 320.0 / float64(viewW)
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

func doomStartMap(lightNum int) int {
	rows := doomShadeRows()
	if rows <= 0 {
		rows = doomNumColorMaps
	}
	return ((doomLightLevels - 1 - lightNum) * 2 * rows) / doomLightLevels
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
	lightNum := doomClampLightNum((int(light) >> doomLightSegShift) + lightBias)
	startMap := doomStartMap(lightNum)
	if depth <= 0 || focal <= 0 {
		return float64(doomClampColorMapRow(startMap))
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
	rows := doomShadeRows()
	if rows <= 0 {
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
	lightNum := doomClampLightNum(int(light) >> doomLightSegShift)
	startMap := doomStartMap(lightNum)
	if depth <= 0 {
		return float64(doomClampColorMapRow(startMap))
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
	rows := doomShadeRows()
	if rows <= 0 {
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
	stats := floorFrameStats{}

	for sec := range g.m.Sectors {
		if sec < 0 || sec >= len(g.mapFloorLoopSets) {
			continue
		}
		set := g.mapFloorLoopSets[sec]
		if len(set.rings) == 0 {
			continue
		}
		// Coarse world-space cull before any per-vertex projection.
		if set.bbox.maxX < viewWB.minX || set.bbox.minX > viewWB.maxX || set.bbox.maxY < viewWB.minY || set.bbox.minY > viewWB.maxY {
			continue
		}

		tex := []byte(nil)
		texOK := false
		if sec >= 0 && sec < len(g.sectorPlaneCache) && len(g.sectorPlaneCache[sec].flatRGBA) == 64*64*4 {
			tex = g.sectorPlaneCache[sec].flatRGBA
			texOK = true
		}
		shadeMul := uint32(g.sectorLightMulCached(sec))
		screenRings := make([][]screenPt, 0, len(set.rings))
		minSX := math.Inf(1)
		minSY := math.Inf(1)
		maxSX := math.Inf(-1)
		maxSY := math.Inf(-1)
		for _, ring := range set.rings {
			sring := make([]screenPt, 0, len(ring))
			for _, p := range ring {
				sx, sy := g.worldToScreen(p.x, p.y)
				sring = append(sring, screenPt{x: sx, y: sy})
				if sx < minSX {
					minSX = sx
				}
				if sy < minSY {
					minSY = sy
				}
				if sx > maxSX {
					maxSX = sx
				}
				if sy > maxSY {
					maxSY = sy
				}
			}
			if len(sring) >= 3 {
				screenRings = append(screenRings, sring)
			}
		}
		if len(screenRings) == 0 || !isFinite(minSX) || !isFinite(minSY) || !isFinite(maxSX) || !isFinite(maxSY) {
			continue
		}
		x0 := max(0, int(math.Floor(minSX)))
		y0 := max(0, int(math.Floor(minSY)))
		x1 := min(w-1, int(math.Ceil(maxSX)))
		y1 := min(h-1, int(math.Ceil(maxSY)))
		if x0 > x1 || y0 > y1 {
			continue
		}

		xHits := make([]float64, 0, 64)
		for py := y0; py <= y1; py++ {
			xHits = xHits[:0]
			row := py * w * 4
			fy := float64(py) + 0.5
			for _, ring := range screenRings {
				for i, j := 0, len(ring)-1; i < len(ring); j, i = i, i+1 {
					a := ring[j]
					b := ring[i]
					if (a.y > fy) == (b.y > fy) {
						continue
					}
					x := a.x + (fy-a.y)*(b.x-a.x)/(b.y-a.y)
					xHits = append(xHits, x)
				}
			}
			if len(xHits) < 2 {
				continue
			}
			sort.Float64s(xHits)
			rowWX0, rowWY0 := g.screenToWorld(0.5, fy)
			rowWX1, rowWY1 := g.screenToWorld(1.5, fy)
			stepWX := rowWX1 - rowWX0
			stepWY := rowWY1 - rowWY0
			for i := 0; i+1 < len(xHits); i += 2 {
				// Fill pixels whose centers lie in [xA, xB) for even-odd winding.
				start := int(math.Ceil(xHits[i] - 0.5))
				end := int(math.Ceil(xHits[i+1]-0.5) - 1)
				if start < x0 {
					start = x0
				}
				if end > x1 {
					end = x1
				}
				if start > end {
					continue
				}
				wx := rowWX0 + float64(start)*stepWX
				wy := rowWY0 + float64(start)*stepWY
				for px := start; px <= end; px++ {
					iPix := row + px*4
					if texOK {
						u := int(math.Floor(wx)) & 63
						v := int(math.Floor(wy)) & 63
						ti := (v*64 + u) * 4
						r, gch, b := shadeRGBByMul(tex[ti+0], tex[ti+1], tex[ti+2], shadeMul)
						pix[iPix+0] = r
						pix[iPix+1] = gch
						pix[iPix+2] = b
						pix[iPix+3] = 255
						stats.markedCols++
					} else {
						r, gch, b := shadeRGBByMul(wallFloorChange.R, wallFloorChange.G, wallFloorChange.B, shadeMul)
						pix[iPix+0] = r
						pix[iPix+1] = gch
						pix[iPix+2] = b
						pix[iPix+3] = 255
						stats.rejectedSpan++
						stats.rejectNoSector++
					}
					wx += stepWX
					wy += stepWY
				}
				stats.emittedSpans++
			}
		}
	}

	g.writePixelsTimed(g.mapFloorLayer, pix)
	screen.DrawImage(g.mapFloorLayer, nil)
	g.mapFloorWorldState = "live-screen"
	g.floorFrame = stats
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
		sort.Slice(verts, func(i, j int) bool {
			ai := math.Atan2(verts[i].y-cy, verts[i].x-cx)
			aj := math.Atan2(verts[j].y-cy, verts[j].x-cx)
			return ai < aj
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
	return chain, ok
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
			sort.Slice(choices, func(i, j int) bool {
				if math.Abs(choices[i].area-choices[j].area) > 1e-9 {
					return choices[i].area < choices[j].area
				}
				return choices[i].dist < choices[j].dist
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
	// Use constrained triangulation when available.
	if cdtTris, ok := triangulateWorldPolygonCDT(indexedWorldPts(verts, idx)); ok && len(cdtTris) > 0 {
		out := make([][3]int, 0, len(cdtTris))
		for _, tri := range cdtTris {
			out = append(out, [3]int{idx[tri[0]], idx[tri[1]], idx[tri[2]]})
		}
		return out, true
	}
	// In CDT-enabled builds, do not fall back to the legacy ear-clipping path.
	if cdtTriangulationAvailable() {
		return nil, false
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
	sort.Slice(order, func(i, j int) bool {
		ai := math.Atan2(verts[order[i]].y-cy, verts[order[i]].x-cx)
		aj := math.Atan2(verts[order[j]].y-cy, verts[order[j]].x-cx)
		return ai < aj
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
		uBase := float64(seg.Offset)
		if frontSideDefIdx >= 0 {
			uBase += float64(g.m.Sidedefs[frontSideDefIdx].TextureOffset)
		}
		frontSectorIdx := g.sectorIndexFromSideNum(ld.SideNum[frontSide])
		backSectorIdx := g.sectorIndexFromSideNum(ld.SideNum[backSide])
		hasTwoSidedMidTex := false
		if ld.SideNum[frontSide] >= 0 && ld.SideNum[backSide] >= 0 &&
			frontSectorIdx >= 0 && backSectorIdx >= 0 &&
			frontSideDefIdx >= 0 && frontSideDefIdx < len(g.m.Sidedefs) {
			mid := normalizeFlatName(g.m.Sidedefs[frontSideDefIdx].Mid)
			hasTwoSidedMidTex = mid != "" && mid != "-"
		}
		g.wallSegStaticCache[si] = wallSegStatic{
			valid:             true,
			ld:                ld,
			frontSide:         frontSide,
			frontSideDefIdx:   frontSideDefIdx,
			frontSectorIdx:    frontSectorIdx,
			backSectorIdx:     backSectorIdx,
			x1w:               x1w,
			y1w:               y1w,
			x2w:               x2w,
			y2w:               y2w,
			segLen:            segLen,
			uBase:             uBase,
			hasTwoSidedMidTex: hasTwoSidedMidTex,
		}
	}
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
	if loopSets != nil && sec >= 0 && sec < len(loopSets) {
		if tris, ok := triangulateSectorLoopsCDT(loopSets[sec]); ok && len(tris) > 0 {
			return tris
		}
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
			tris:      append([]worldTri(nil), g.sectorPlaneTris[sec]...),
			dynamic:   dyn,
			lastFloor: floor,
			lastCeil:  ceil,
			dirty:     false,
			light:     light,
			lightMul:  lightMul,
			lightKind: lightKind,
			texTick:   -1,
		}
	}
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
	g.refreshSectorPlaneCacheLighting()
	g.refreshDynamicSectorPlaneCache()
}

func (g *game) refreshSectorPlaneCacheTextureRefs() {
	if g == nil || g.m == nil || len(g.sectorPlaneCache) != len(g.m.Sectors) {
		return
	}
	animTick := g.textureAnimTick()
	for sec := range g.sectorPlaneCache {
		entry := &g.sectorPlaneCache[sec]
		texID := normalizeFlatName(g.m.Sectors[sec].FloorPic)
		if entry.tex != nil && entry.texID == texID && entry.texTick == animTick {
			continue
		}
		entry.texID = texID
		entry.texTick = animTick
		entry.tex = nil
		entry.flatRGBA = nil
		if rgba, ok := g.flatRGBA(g.m.Sectors[sec].FloorPic); ok {
			entry.flatRGBA = rgba
		}
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
	if g != nil && sec >= 0 && sec < len(g.sectorPlaneCache) {
		return g.sectorPlaneCache[sec].light
	}
	if g != nil && g.m != nil && sec >= 0 && sec < len(g.m.Sectors) {
		return g.m.Sectors[sec].Light
	}
	return 160
}

func (g *game) sectorLightMulCached(sec int) uint32 {
	if g != nil && sec >= 0 && sec < len(g.sectorPlaneCache) {
		return uint32(g.sectorPlaneCache[sec].lightMul)
	}
	return uint32(sectorLightMul(g.sectorLightLevelCached(sec)))
}

func (g *game) sectorLightKindCached(sec int) sectorLightEffectKind {
	if g != nil && sec >= 0 && sec < len(g.sectorPlaneCache) {
		return g.sectorPlaneCache[sec].lightKind
	}
	return sectorLightEffectNone
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
			sort.Slice(pts[:], func(i, j int) bool { return holeQuantLess(pts[i], pts[j]) })
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
		side := pointOnDivlineSide(x, y, dl)
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

	layer := ebiten.NewImage(w, h)
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

		tex, texOK := g.flatRGBA(g.m.Sectors[sec].FloorPic)
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
					ti := (v*64 + u) * 4
					pix[i+0] = tex[ti+0]
					pix[i+1] = tex[ti+1]
					pix[i+2] = tex[ti+2]
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
	sort.Slice(g.orphanRepairQueue, func(i, j int) bool {
		return g.orphanRepairQueue[i].votes > g.orphanRepairQueue[j].votes
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
	if img, ok := g.flatImgCache[key]; ok {
		return img, true
	}
	rgba, ok := g.flatRGBAResolvedKey(key)
	if !ok || len(rgba) != 64*64*4 {
		return nil, false
	}
	img := ebiten.NewImage(64, 64)
	g.writePixelsTimed(img, rgba)
	g.flatImgCache[key] = img
	return img, true
}

func (g *game) flatRGBA(name string) ([]byte, bool) {
	key, _ := g.resolveAnimatedFlatSample(name)
	return g.flatRGBAResolvedKey(key)
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

var wallTextureAnimRefs = buildTextureAnimRefs([][]string{
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

var flatTextureAnimRefs = buildTextureAnimRefs([][]string{
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
				index:  i,
			}
		}
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
	return g.resolveAnimatedTextureSample(name, g.worldTic, wallTextureAnimRefs)
}

func (g *game) resolveAnimatedFlatSample(name string) (string, bool) {
	return g.resolveAnimatedTextureSample(name, g.worldTic, flatTextureAnimRefs)
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

func normalizeTextureAnimCrossfadeFrames(n int, sourcePort bool) int {
	return 0
}

func (g *game) textureAnimTick() int {
	return g.worldTic / textureAnimTics
}

func normalizeFlatName(name string) string {
	out := make([]byte, 0, 8)
	for i := 0; i < len(name) && len(out) < 8; i++ {
		c := name[i]
		if c == 0 {
			break
		}
		if c >= 'a' && c <= 'z' {
			c -= 'a' - 'A'
		}
		out = append(out, c)
	}
	return string(out)
}

func isSkyFlatName(name string) bool {
	n := normalizeFlatName(name)
	if n == "" {
		return false
	}
	return strings.Contains(n, "SKY")
}

func (g *game) capturePrevState() {
	g.prevCamX = g.camX
	g.prevCamY = g.camY
	g.prevPX = g.p.x
	g.prevPY = g.p.y
	g.prevAngle = g.p.angle
}

func (g *game) syncRenderState() {
	g.capturePrevState()
	g.renderCamX = g.camX
	g.renderCamY = g.camY
	g.renderPX = float64(g.p.x) / fracUnit
	g.renderPY = float64(g.p.y) / fracUnit
	g.renderAngle = g.p.angle
	g.renderAlpha = 1
	g.debugAimSS = debugFixedSubsector
	g.lastUpdate = time.Now()
}

func (g *game) prepareRenderState() {
	g.beginSourcePortSpectreFuzzFrame(time.Now())
	alpha := g.interpAlpha()
	if !g.opts.SourcePortMode || g.interpAutoOff {
		alpha = 1
	}
	if g.simTickScale > 1.0 {
		// Multiple sim ticks per frame already advance world state aggressively.
		// Interpolating from prev can make render state lag behind simulation.
		alpha = 1
	}
	g.renderCamX = lerp(g.prevCamX, g.camX, alpha)
	g.renderCamY = lerp(g.prevCamY, g.camY, alpha)
	g.renderPX = lerp(float64(g.prevPX)/fracUnit, float64(g.p.x)/fracUnit, alpha)
	g.renderPY = lerp(float64(g.prevPY)/fracUnit, float64(g.p.y)/fracUnit, alpha)
	g.renderAngle = lerpAngle(g.prevAngle, g.p.angle, alpha)
	g.renderAlpha = alpha
	g.debugAimSS = debugFixedSubsector
}

func (g *game) interpAlpha() float64 {
	if g.lastUpdate.IsZero() {
		return 1
	}
	dt := time.Since(g.lastUpdate).Seconds()
	ticRate := float64(doomTicsPerSecond)
	if g.simTickScale > 0 {
		ticRate *= g.simTickScale
	}
	if ticRate < 1e-6 {
		ticRate = doomTicsPerSecond
	}
	step := 1.0 / ticRate
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

func lerpAngle(a, b uint32, t float64) uint32 {
	d := int64(int32(b - a))
	v := float64(int64(a)) + float64(d)*t
	return uint32(int64(v))
}

func (g *game) linedefDecision(ld mapdata.Linedef) lineDecision {
	front, back := g.lineSectors(ld)
	return parityLineDecision(ld, front, back, g.parity, g.opts.LineColorMode)
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

func (g *game) decisionStyle(d lineDecision) (color.Color, float64) {
	switch d.appearance {
	case lineAppearanceOneSided:
		return wallOneSided, d.width
	case lineAppearanceSecret:
		return wallSecret, d.width
	case lineAppearanceTeleporter:
		return wallTeleporter, d.width
	case lineAppearanceFloorChange:
		return wallFloorChange, d.width
	case lineAppearanceCeilChange:
		return wallCeilChange, d.width
	case lineAppearanceNoHeightDiff:
		return wallNoHeightDiff, d.width
	case lineAppearanceUnrevealed:
		return wallUnrevealed, d.width
	default:
		return wallNoHeightDiff, d.width
	}
}

func (g *game) mapLineStateKey() mapLineCacheKey {
	return mapLineCacheKey{
		camX:          g.renderCamX,
		camY:          g.renderCamY,
		zoom:          g.zoom,
		angle:         g.renderAngle,
		rotateView:    g.rotateView,
		viewW:         g.viewW,
		viewH:         g.viewH,
		reveal:        g.parity.reveal,
		iddt:          g.parity.iddt,
		lineColorMode: g.opts.LineColorMode,
		mappedRev:     g.mapLineRev,
	}
}

func (g *game) rebuildMapLineCache() {
	out := g.mapLineBuf[:0]
	for _, li := range g.visibleLineIndices() {
		pi := g.physForLine[li]
		if pi < 0 || pi >= len(g.lines) {
			continue
		}
		ld := g.m.Linedefs[li]
		d := g.linedefDecision(ld)
		if !d.visible {
			continue
		}
		pl := g.lines[pi]
		x1, y1 := g.worldToScreen(float64(pl.x1)/fracUnit, float64(pl.y1)/fracUnit)
		x2, y2 := g.worldToScreen(float64(pl.x2)/fracUnit, float64(pl.y2)/fracUnit)
		if x1 == x2 && y1 == y2 {
			continue
		}
		c, w := g.decisionStyle(d)
		crgba := color.RGBAModel.Convert(c).(color.RGBA)
		out = append(out, mapLineDraw{
			x1:  float32(x1),
			y1:  float32(y1),
			x2:  float32(x2),
			y2:  float32(y2),
			w:   float32(w),
			clr: crgba,
		})
	}
	g.mapLineBuf = out
}

func (g *game) drawMapLines(screen *ebiten.Image) {
	key := g.mapLineStateKey()
	if !g.mapLineInit || key != g.mapLineKey {
		g.rebuildMapLineCache()
		g.mapLineKey = key
		g.mapLineInit = true
	}
	aa := g.mapVectorAntiAlias()
	for _, ln := range g.mapLineBuf {
		vector.StrokeLine(screen, ln.x1, ln.y1, ln.x2, ln.y2, ln.w, ln.clr, aa)
	}
}

func (g *game) visibleLineIndices() []int {
	margin := 2.0 / g.zoom
	camX := g.renderCamX
	camY := g.renderCamY
	viewHalfW := float64(g.viewW) / (2 * g.zoom)
	viewHalfH := float64(g.viewH) / (2 * g.zoom)
	minXf := camX - viewHalfW - margin
	maxXf := camX + viewHalfW + margin
	minYf := camY - viewHalfH - margin
	maxYf := camY + viewHalfH + margin
	if g.rotateView {
		// Conservative culling when rotating: circumscribed circle around the viewport.
		r := math.Hypot(viewHalfW, viewHalfH) + margin
		minXf = camX - r
		maxXf = camX + r
		minYf = camY - r
		maxYf = camY + r
	}
	minX := floatToFixed(minXf)
	maxX := floatToFixed(maxXf)
	minY := floatToFixed(minYf)
	maxY := floatToFixed(maxYf)

	g.visibleBuf = g.visibleBuf[:0]
	// Robust automap visibility: trust line bboxes directly.
	// Some BLOCKMAP data can omit candidates and cause line pop/disappear at seams.
	for _, pl := range g.lines {
		if !bboxIntersects(pl.bbox, minX, minY, maxX, maxY) {
			continue
		}
		g.visibleBuf = append(g.visibleBuf, pl.idx)
	}
	return g.visibleBuf
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
		if x < sp.l {
			return false
		}
		if x <= sp.r {
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
	return append(out, solidSpan{l: l, r: r})
}

func (g *game) spriteRowVisibleSpansDepthQ(y, x0, x1 int, depthQ uint16, clipSpans, out []solidSpan) []solidSpan {
	out = out[:0]
	if x1 < x0 || g == nil || y < 0 || y >= g.viewH {
		return out
	}
	if !g.billboardClippingEnabled() {
		if len(clipSpans) == 0 {
			return append(out, solidSpan{l: x0, r: x1})
		}
		for _, sp := range clipSpans {
			out = appendClippedSolidSpan(out, sp.l, sp.r, x0, x1)
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
			if x < len(g.wallDepthQCol) {
				wq := g.wallDepthQCol[x]
				if wq != 0xFFFF && depthQ > wq {
					if x < len(g.wallDepthClosedCol) && g.wallDepthClosedCol[x] {
						occluded = true
					} else if x < len(g.wallDepthTopCol) && x < len(g.wallDepthBottomCol) {
						top := g.wallDepthTopCol[x]
						bottom := g.wallDepthBottomCol[x]
						if y >= top && y <= bottom {
							occluded = true
						}
					} else {
						occluded = true
					}
				}
			}
			if !occluded {
				occluded = g.spriteWallClipOccludedAtXYDepth(x, y, depthQ)
			}
			if occluded {
				if runStart >= 0 {
					out = append(out, solidSpan{l: runStart, r: x - 1})
					runStart = -1
				}
				continue
			}
			if runStart < 0 {
				runStart = x
			}
		}
		if runStart >= 0 {
			out = append(out, solidSpan{l: runStart, r: r})
		}
	}

	if len(clipSpans) == 0 {
		appendVisible(x0, x1)
		return out
	}
	for _, sp := range clipSpans {
		l := sp.l
		r := sp.r
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

func (g *game) thingPosFixed(i int, th mapdata.Thing) (int64, int64) {
	if i >= 0 && i < len(g.thingX) && i < len(g.thingY) {
		return g.thingX[i], g.thingY[i]
	}
	return int64(th.X) << fracBits, int64(th.Y) << fracBits
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
}

func (g *game) subsectorFloorCeilAt(x, y int64) (int64, int64, bool) {
	if g == nil {
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

func (g *game) spriteFootprintClipYBounds(x, y, radius int64, viewH int, eyeZ, depth, focal float64) (int, int, bool) {
	if !g.billboardClippingEnabled() {
		return 0, viewH - 1, true
	}
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
		floorZ, ceilZ, ok := g.subsectorFloorCeilAt(x+off[0], y+off[1])
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

func (g *game) lineVisibleInBox(lineIdx int, minX, minY, maxX, maxY int64) bool {
	pi := g.physForLine[lineIdx]
	if pi < 0 || pi >= len(g.lines) {
		return false
	}
	return bboxIntersects(g.lines[pi].bbox, minX, minY, maxX, maxY)
}

func bboxIntersects(lineBBox [4]int64, minX, minY, maxX, maxY int64) bool {
	lineMaxY := lineBBox[0]
	lineMinY := lineBBox[1]
	lineMaxX := lineBBox[2]
	lineMinX := lineBBox[3]
	if lineMaxX < minX || lineMinX > maxX {
		return false
	}
	if lineMaxY < minY || lineMinY > maxY {
		return false
	}
	return true
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
	img := ebiten.NewImage(p.Width, p.Height)
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

func (g *game) drawPauseThermo(screen *ebiten.Image, x, y, width, dot int, sx, sy float64) {
	if g == nil {
		return
	}
	if width < 1 {
		width = 1
	}
	if dot < 0 {
		dot = 0
	}
	if dot > width-1 {
		dot = width - 1
	}
	if !g.drawMenuPatch(screen, "M_THERML", float64(x)*sx, float64(y)*sy, sx, sy) {
		return
	}
	for i := 0; i < width; i++ {
		_ = g.drawMenuPatch(screen, "M_THERMM", float64(x+8+i*8)*sx, float64(y)*sy, sx, sy)
	}
	_ = g.drawMenuPatch(screen, "M_THERMR", float64(x+8+width*8)*sx, float64(y)*sy, sx, sy)
	_ = g.drawMenuPatch(screen, "M_THERMO", float64(x+8+dot*8)*sx, float64(y)*sy, sx, sy)
}

func (g *game) drawPauseOverlay(screen *ebiten.Image) {
	if !g.pauseMenuActive || g.quitPromptActive {
		return
	}
	sx := float64(g.viewW) / 320.0
	sy := float64(g.viewH) / 200.0
	ebitenutil.DrawRect(screen, 0, 0, 320.0*sx, 200.0*sy, color.RGBA{R: 8, G: 8, B: 8, A: 128})
	switch g.pauseMenuMode {
	case pauseMenuModeOptions:
		_ = g.drawMenuPatch(screen, "M_OPTTTL", 108*sx, 15*sy, sx, sy)
		for i, name := range frontendOptionsMenuNames {
			if strings.TrimSpace(name) == "" {
				continue
			}
			_ = g.drawMenuPatch(screen, name, 60*sx, float64(37+i*16)*sy, sx, sy)
		}
		_ = g.drawMenuPatch(screen, frontendMessagesPatch(g.hudMessagesEnabled), float64((60+120))*sx, float64(37+1*16)*sy, sx, sy)
		if g.opts.SourcePortMode {
			g.drawHUTextAt(screen, g.pauseSourcePortDetailLabel(), float64((60+175))*sx, float64(37+2*16+2)*sy, sx*1.6, sy*1.6)
		} else {
			_ = g.drawMenuPatch(screen, frontendDetailPatch(g.lowDetailMode()), float64((60+175))*sx, float64(37+2*16)*sy, sx, sy)
		}
		g.drawPauseThermo(screen, 60, 37+6*16, 10, frontendMouseSensitivityDot(g.opts.MouseLookSpeed), sx, sy)
	case pauseMenuModeSound:
		_ = g.drawMenuPatch(screen, "M_SVOL", 60*sx, 38*sy, sx, sy)
		g.drawPauseThermo(screen, 80, 64+1*16, 16, frontendVolumeDot(g.opts.SFXVolume), sx, sy)
		g.drawPauseThermo(screen, 80, 64+3*16, 16, frontendVolumeDot(g.opts.MusicVolume), sx, sy)
		_ = g.drawMenuPatch(screen, "M_SFXVOL", 80*sx, 64*sy, sx, sy)
		_ = g.drawMenuPatch(screen, "M_MUSVOL", 80*sx, float64(64+2*16)*sy, sx, sy)
	case pauseMenuModeEpisode:
		_ = g.drawMenuPatch(screen, "M_NEWG", 96*sx, 14*sy, sx, sy)
		_ = g.drawMenuPatch(screen, "M_EPISOD", 54*sx, 38*sy, sx, sy)
		episodes := g.availableEpisodeChoices()
		for i, ep := range episodes {
			if name, ok := inGameEpisodeMenuNames[ep]; ok {
				_ = g.drawMenuPatch(screen, name, 48*sx, float64(63+i*16)*sy, sx, sy)
			}
		}
	case pauseMenuModeSkill:
		_ = g.drawMenuPatch(screen, "M_NEWG", 96*sx, 14*sy, sx, sy)
		_ = g.drawMenuPatch(screen, "M_SKILL", 54*sx, 38*sy, sx, sy)
		for i, name := range frontendSkillMenuNames {
			_ = g.drawMenuPatch(screen, name, 48*sx, float64(63+i*16)*sy, sx, sy)
		}
	default:
		px := float64((320 - 68) / 2)
		py := 4.0
		_ = g.drawMenuPatch(screen, "M_PAUSE", px*sx, py*sy, sx, sy)
		_ = g.drawMenuPatch(screen, "M_DOOM", 94*sx, 2*sy, sx, sy)
		for i, name := range inGamePauseMenuNames {
			_ = g.drawMenuPatch(screen, name, 97*sx, float64(64+i*16)*sy, sx, sy)
		}
	}
	skull := "M_SKULL1"
	if g.pauseMenuWhichSkull != 0 {
		skull = "M_SKULL2"
	}
	switch g.pauseMenuMode {
	case pauseMenuModeOptions:
		_ = g.drawMenuPatch(screen, skull, 28*sx, float64(37+g.pauseMenuOptionsOn*16)*sy, sx, sy)
	case pauseMenuModeSound:
		skullY := 64
		if g.pauseMenuSoundOn != 0 {
			skullY += 2 * 16
		}
		_ = g.drawMenuPatch(screen, skull, 48*sx, float64(skullY)*sy, sx, sy)
	case pauseMenuModeEpisode:
		_ = g.drawMenuPatch(screen, skull, 16*sx, float64(63+g.pauseMenuEpisodeOn*16)*sy, sx, sy)
	case pauseMenuModeSkill:
		_ = g.drawMenuPatch(screen, skull, 16*sx, float64(63+g.pauseMenuSkillOn*16)*sy, sx, sy)
	default:
		_ = g.drawMenuPatch(screen, skull, 65*sx, float64(64+g.pauseMenuItemOn*16)*sy, sx, sy)
	}
	if msg := strings.TrimSpace(g.pauseMenuStatus); msg != "" {
		ebitenutil.DebugPrintAt(screen, msg, g.viewW/2-len(msg)*3, int(182*sy))
	}
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
	elapsed := now.Sub(g.fpsStamp)
	if elapsed >= time.Second {
		fps := float64(g.fpsFrames) / elapsed.Seconds()
		g.fpsDisplay = fps
		g.updateInterpolationPerfState(fps)
		if g.fpsFrames > 0 {
			g.renderMSAvg = float64(g.renderAccum) / float64(time.Millisecond) / float64(g.fpsFrames)
		} else {
			g.renderMSAvg = 0
		}
		g.fpsFrames = 0
		g.renderAccum = 0
		g.fpsStamp = now
	}
}

func (g *game) updateInterpolationPerfState(fps float64) {
	if !g.opts.SourcePortMode {
		g.interpAutoOff = false
		return
	}
	const disableAtFPS = float64(doomTicsPerSecond)
	const reenableAtFPS = disableAtFPS + 5.0
	if g.interpAutoOff {
		if fps > reenableAtFPS {
			g.interpAutoOff = false
			// Snap interpolation state when re-enabling to avoid one-frame pops.
			g.syncRenderState()
		}
		return
	}
	if fps <= disableAtFPS {
		g.interpAutoOff = true
		// Snap interpolation state when disabling to avoid one-frame pops.
		g.syncRenderState()
	}
}

func (g *game) writePixelsTimed(img *ebiten.Image, pix []byte) {
	start := time.Now()
	img.WritePixels(pix)
	if g.perfInDraw {
		g.frameUpload += time.Since(start)
	}
}

func (g *game) drawPerfOverlay(screen *ebiten.Image) {
	line1 := fmt.Sprintf("%.2f, %dms", g.fpsDisplay, int(math.Round(g.renderMSAvg)))
	sx, sy, ox, _ := g.hudTransform()
	w := g.huTextWidth(line1)
	maxX := float64(g.viewW)
	if g.opts.SourcePortMode {
		maxX = ox + statusBaseW*sx
	}
	x := int(maxX - float64(w)*sx - 10*sx)
	if x < 4 {
		x = 4
	}
	g.drawHUTextAt(screen, line1, float64(x), 10*sy, sx, sy)
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
	for _, ch := range text {
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
		op.GeoM.Scale(sx, sy)
		op.GeoM.Translate(px-float64(ox)*sx, py-float64(oy)*sy)
		screen.DrawImage(img, op)
		px += float64(w) * sx
	}
}
