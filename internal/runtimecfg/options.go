package runtimecfg

import (
	"gddoom/internal/demo"
	"gddoom/internal/gameplay"
	"gddoom/internal/mapdata"
	"gddoom/internal/media"
	"gddoom/internal/music"
)

type MusicPlayerTrack struct {
	MapName  mapdata.MapName
	Label    string
	LumpName string
}

type MusicPlayerEpisode struct {
	Label  string
	Tracks []MusicPlayerTrack
}

type MusicPlayerWAD struct {
	Key      string
	Label    string
	Episodes []MusicPlayerEpisode
}

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
	WallTexBank                map[string]media.WallTexture
	BootSplash                 media.WallTexture
	DoomPaletteRGBA            []byte
	DoomColorMap               []byte
	DoomColorMapRows           int
	MenuPatchBank              map[string]media.WallTexture
	StatusPatchBank            map[string]media.WallTexture
	MessageFontBank            map[rune]media.WallTexture
	SpritePatchBank            map[string]media.WallTexture
	IntermissionPatchBank      map[string]media.WallTexture
	SoundBank                  media.SoundBank
	DemoScript                 *demo.Script
	AttractDemos               []*demo.Script
	DemoQuitOnComplete         bool
	RecordDemoPath             string
	DemoTracePath              string
	TitleMusicLoader           func() ([]byte, error)
	MapMusicLoader             func(mapName string) ([]byte, error)
	MusicPlayerCatalog         []MusicPlayerWAD
	MusicPlayerTrackLoader     func(wadKey string, mapName string) ([]byte, error)
	NewGameLoader              func(mapName string) (*mapdata.Map, error)
	DemoMapLoader              func(demo *demo.Script) (*mapdata.Map, error)
	Episodes                   []int
	MusicPatchBank             music.PatchBank
	OnRuntimeSettingsChanged   func(gameplay.RuntimeSettings)
}
