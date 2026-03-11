package automap

import (
	"gddoom/internal/demo"
	"gddoom/internal/gameplay"
	"gddoom/internal/mapdata"
	"gddoom/internal/media"
	"gddoom/internal/music"
)

type RuntimeSettings = gameplay.RuntimeSettings

type Options struct {
	Width                      int
	Height                     int
	StartZoom                  float64
	InitialDetailLevel         int
	InitialGammaLevel          int
	WADHash                    string
	Debug                      bool
	PlayerSlot                 int
	SkillLevel                 int
	GameMode                   string
	ShowNoSkillItems           bool
	ShowAllItems               bool
	MouseLook                  bool
	MouseLookSpeed             float64
	KeyboardTurnSpeed          float64
	MusicVolume                float64
	MUSPanMax                  float64
	OPLVolume                  float64
	SFXVolume                  float64
	FastMonsters               bool
	AlwaysRun                  bool
	AutoWeaponSwitch           bool
	CheatLevel                 int
	Invulnerable               bool
	LineColorMode              string
	SourcePortMode             bool
	SourcePortThingRenderMode  string
	SourcePortThingBlendFrames bool
	SourcePortSectorLighting   bool
	DisableDoomLighting        bool
	KageShader                 bool
	GPUSky                     bool
	SkyUpscaleMode             string
	CRTEffect                  bool
	DisableWallOcclusion       bool
	DisableWallSpanReject      bool
	DisableWallSpanClip        bool
	DisableWallSliceOcclusion  bool
	DisableBillboardClipping   bool
	TextureAnimCrossfadeFrames int
	NoVsync                    bool
	NoFPS                      bool
	DisableAspectCorrection    bool
	AllCheats                  bool
	StartInMapMode             bool
	FlatBank                   map[string][]byte
	FlatBankIndexed            map[string][]byte
	WallTexBank                map[string]WallTexture
	BootSplash                 WallTexture
	DoomPaletteRGBA            []byte
	DoomColorMap               []byte
	DoomColorMapRows           int
	MenuPatchBank              map[string]WallTexture
	StatusPatchBank            map[string]WallTexture
	MessageFontBank            map[rune]WallTexture
	SpritePatchBank            map[string]WallTexture
	IntermissionPatchBank      map[string]WallTexture
	SoundBank                  SoundBank
	DemoScript                 *DemoScript
	AttractDemos               []*DemoScript
	DemoQuitOnComplete         bool
	RecordDemoPath             string
	DemoTracePath              string
	TitleMusicLoader           func() ([]byte, error)
	MapMusicLoader             func(mapName string) ([]byte, error)
	NewGameLoader              func(mapName string) (*mapdata.Map, error)
	DemoMapLoader              func(demo *DemoScript) (*mapdata.Map, error)
	Episodes                   []int
	MusicPatchBank             music.PatchBank
	OnRuntimeSettingsChanged   func(RuntimeSettings)
}

type WallTexture = media.WallTexture

type RunResult struct {
	LevelExited bool
	SecretExit  bool
}

type PCMSample = media.PCMSample

type SoundBank = media.SoundBank

type DemoTic = demo.Tic

type DemoHeader = demo.Header

type DemoScript = demo.Script
