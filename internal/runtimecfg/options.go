package runtimecfg

import (
	"gddoom/internal/demo"
	"gddoom/internal/gameplay"
	"gddoom/internal/mapdata"
	"gddoom/internal/media"
	"gddoom/internal/music"
)

type MusicPlayerTrack struct {
	MapName   mapdata.MapName
	Label     string
	LumpName  string
	MusicName string
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

type LiveTicSource interface {
	PollTic() (demo.Tic, bool, error)
}

type LiveTicSink interface {
	BroadcastTic(demo.Tic) error
}

type Options struct {
	Width                      int
	Height                     int
	StartZoom                  float64
	InitialDetailLevel         int
	AutoDetail                 bool
	InitialGammaLevel          int
	WADHash                    string
	Debug                      bool
	DebugEvents                bool
	PlayerSlot                 int
	SkillLevel                 int
	GameMode                   string
	ShowNoSkillItems           bool
	ShowAllItems               bool
	MouseLook                  bool
	MouseInvert                bool
	SmoothCameraYaw            bool
	MouseLookSpeed             float64
	KeyboardTurnSpeed          float64
	MusicVolume                float64
	MUSPanMax                  float64
	OPLVolume                  float64
	AudioPreEmphasis           bool
	MusicBackend               music.Backend
	OpenMenuOnFrontendStart    bool
	SFXVolume                  float64
	SFXPitchShift              bool
	FastMonsters               bool
	RespawnMonsters            bool
	NoMonsters                 bool
	AlwaysRun                  bool
	AutoWeaponSwitch           bool
	CheatLevel                 int
	Invulnerable               bool
	SourcePortMode             bool
	SourcePortThingRenderMode  string
	SourcePortThingBlendFrames bool
	ZombiemanThinkerBlend      bool
	DebugMonsterThinkerBlend   bool
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
	RendererWorkers            int
	TextureAnimCrossfadeFrames int
	NoVsync                    bool
	NoFPS                      bool
	ShowTPS                    bool
	DisableAspectCorrection    bool
	AllCheats                  bool
	StartInMapMode             bool
	FlatBank                   map[string][]byte
	FlatBankIndexed            map[string][]byte
	WallTexBank                map[string]media.WallTexture
	WallTextureAnimSequences   map[string][]string
	FlatTextureAnimSequences   map[string][]string
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
	DemoExitOnDeath            bool
	DemoStopAfterTics          int
	RecordDemoPath             string
	DemoTracePath              string
	TitleMusicLoader           func() ([]byte, error)
	MapMusicLoader             func(mapName string) ([]byte, error)
	MapMusicInfo               func(mapName string) (levelLabel string, musicName string)
	IntermissionMusicLoader    func(commercial bool) ([]byte, error)
	PlayCheatMusic             func(currentMapName string, code string) (bool, error)
	MusicPlayerCatalog         []MusicPlayerWAD
	MusicPlayerTrackLoader     func(wadKey string, lumpName string) ([]byte, error)
	NewGameLoader              func(mapName string) (*mapdata.Map, error)
	DemoMapLoader              func(demo *demo.Script) (*mapdata.Map, error)
	Episodes                   []int
	LiveTicSource              LiveTicSource
	LiveTicSink                LiveTicSink
	MusicPatchBank             music.PatchBank
	MusicSoundFontPath         string
	MusicSoundFontChoices      []string
	MusicSoundFont             *music.SoundFontBank
	OnRuntimeSettingsChanged   func(gameplay.RuntimeSettings)
}
