package automap

import "gddoom/internal/music"

type RuntimeSettings struct {
	DetailLevel      int
	GammaLevel       int
	MusicVolume      float64
	MUSPanMax        float64
	OPLVolume        float64
	SFXVolume        float64
	MouseLook        bool
	AlwaysRun        bool
	AutoWeaponSwitch bool
	LineColorMode    string
	CRTEffect        bool
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
	DisableDoomLighting        bool
	KageShader                 bool
	GPUSky                     bool
	CRTEffect                  bool
	DepthBufferView            bool
	DisableDepthOcclusion      bool
	TextureAnimCrossfadeFrames int
	NoVsync                    bool
	NoFPS                      bool
	DisableAspectCorrection    bool
	AllCheats                  bool
	StartInMapMode             bool
	FlatBank                   map[string][]byte
	WallTexBank                map[string]WallTexture
	BootSplash                 WallTexture
	DoomPaletteRGBA            []byte
	DoomColorMap               []byte
	DoomColorMapRows           int
	StatusPatchBank            map[string]WallTexture
	MessageFontBank            map[rune]WallTexture
	SpritePatchBank            map[string]WallTexture
	IntermissionPatchBank      map[string]WallTexture
	SoundBank                  SoundBank
	DemoScript                 *DemoScript
	RecordDemoPath             string
	MapMusicLoader             func(mapName string) ([]byte, error)
	MusicPatchBank             music.PatchBank
	OnRuntimeSettingsChanged   func(RuntimeSettings)
}

type WallTexture struct {
	RGBA     []byte
	RGBA32   []uint32
	ColMajor []uint32
	Width    int
	Height   int
	OffsetX  int
	OffsetY  int
}

type RunResult struct {
	LevelExited bool
	SecretExit  bool
}

type PCMSample struct {
	SampleRate int
	Data       []byte
}

type SoundBank struct {
	DoorOpen            PCMSample
	DoorClose           PCMSample
	BlazeOpen           PCMSample
	BlazeClose          PCMSample
	SwitchOn            PCMSample
	SwitchOff           PCMSample
	NoWay               PCMSample
	ItemUp              PCMSample
	WeaponUp            PCMSample
	PowerUp             PCMSample
	Oof                 PCMSample
	Pain                PCMSample
	ShootPistol         PCMSample
	ShootShotgun        PCMSample
	ShootFireball       PCMSample
	ShootRocket         PCMSample
	ImpactFire          PCMSample
	ImpactRocket        PCMSample
	MonsterPainHumanoid PCMSample
	MonsterPainDemon    PCMSample
	DeathZombie         PCMSample
	DeathShotgunGuy     PCMSample
	DeathImp            PCMSample
	DeathDemon          PCMSample
	DeathCaco           PCMSample
	DeathBaron          PCMSample
	DeathCyber          PCMSample
	DeathSpider         PCMSample
	DeathLostSoul       PCMSample
	MonsterDeath        PCMSample
	PlayerDeath         PCMSample
	InterTick           PCMSample
	InterDone           PCMSample
}

type DemoTic struct {
	Forward int64
	Side    int64
	Turn    int
	TurnRaw int64
	Run     bool
	Use     bool
	Fire    bool
}

type DemoScript struct {
	Path string
	Tics []DemoTic
}
