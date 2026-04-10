package runtimecfg

import (
	"gddoom/internal/demo"
	"gddoom/internal/gameplay"
	"gddoom/internal/mapdata"
	"gddoom/internal/media"
	"gddoom/internal/music"
	"gddoom/internal/sound"
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

type ChatMessage struct {
	Name string
	Text string
}

type LiveTicBufferedSource interface {
	PendingTics() int
}

type RuntimeKeyframe struct {
	Tic            uint32
	Blob           []byte
	MandatoryApply bool
}

type LiveRuntimeKeyframeSource interface {
	PollRuntimeKeyframe() (RuntimeKeyframe, bool, error)
}

type LiveTicSink interface {
	BroadcastTic(demo.Tic) error
}

type LiveChatSource interface {
	PollRuntimeChat() (ChatMessage, bool, error)
}

type LiveChatSink interface {
	SendRuntimeChat(ChatMessage) error
}

type LiveIntermissionAdvanceSource interface {
	PollIntermissionAdvance() (bool, error)
}

type LiveIntermissionAdvanceSink interface {
	BroadcastIntermissionAdvance() error
}

type NetBandwidthMeter interface {
	BandwidthStats() (uploadBytesPerSec, downloadBytesPerSec float64)
}

// CoopPeerSource is the lockstep input source for a co-op multiplayer session.
// The game loop calls SendLocalTic once per tic for the local player, then
// calls ReadyTics to find how many tics are available across all active peers,
// and finally calls PollPeerTic for each remote player slot to advance them.
type CoopPeerSource interface {
	// LocalPlayerID is this peer's assigned slot (1-4).
	LocalPlayerID() byte

	// ActivePeerIDs returns the current set of remote player IDs (excludes local).
	ActivePeerIDs() []byte

	// SendLocalTic submits the local player's tic for this game tic.
	SendLocalTic(demo.Tic) error

	// ReadyTics returns how many complete tics are available for all active peers.
	// The game must not advance beyond this count.
	ReadyTics() int

	// PollPeerTic removes and returns the next buffered tic for the given remote
	// player ID. Returns (tic, true, nil) if available, (_, false, nil) if not
	// yet ready, or (_, false, err) on stream error.
	PollPeerTic(playerID byte) (demo.Tic, bool, error)

	// PollRosterUpdate returns a pending roster change, if any.
	PollRosterUpdate() (RosterUpdate, bool)

	// PollCheckpoint returns the next checkpoint received from the server, if any.
	// Clients should compare the hash against their local SimChecksum() and call
	// SendDesyncNotify on mismatch.
	PollCheckpoint() (Checkpoint, bool)

	// SendCheckpoint sends this peer's simulation hash at the given tic to the
	// server for relay to other peers. Only the canonical peer (slot 1) calls this.
	SendCheckpoint(tic uint32, hash uint32) error

	// SendDesyncNotify informs the server that the local simulation hash at the
	// given tic does not match the received checkpoint hash. The server will push
	// a mandatory keyframe to resync the client.
	SendDesyncNotify(tic uint32, localHash uint32) error
}

// RosterUpdate describes a change to the active peer roster.
type RosterUpdate struct {
	PlayerIDs []byte
}

// Checkpoint is a periodic hash broadcast by the canonical peer so all other
// peers can verify their simulation has not diverged.
type Checkpoint struct {
	Tic  uint32
	Hash uint32
}

type VoiceSyncMeter interface {
	VoiceSyncOffsetMillis() (millis int, ok bool)
}

type VoiceSettings struct {
	Codec             string
	G726Bits          int
	Bitrate           int
	SampleRate        int
	AGCEnabled        bool
	PushToTalkEnabled bool
	GateEnabled       bool
	GateThreshold     float64
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
	DisableAspectCorrection       bool
	DisableGeometryAspectCorrect  bool
	InputBindings              InputBindings
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
	PCSpeakerBank              map[string][]sound.PCSpeakerTone
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
	CoopPeers                  CoopPeerSource
	WatchStartupBufferTics     int
	NetBandwidthMeter          NetBandwidthMeter
	VoiceBandwidthMeter        NetBandwidthMeter
	VoiceSyncMeter             VoiceSyncMeter
	VoiceCodec                 string
	VoiceG726BitsPerSample     int
	VoiceBitrate               int
	VoiceSampleRate            int
	VoiceAGCEnabled            bool
	VoicePushToTalkEnabled     bool
	VoiceGateEnabled           bool
	VoiceGateThreshold         float64
	VoiceInputDevice           string
	VoiceInputLevel            func() float64
	VoiceInputGateActive       func() bool
	VoiceTransmitActive        func() bool
	OnVoiceSettingsChanged     func(VoiceSettings) error
	MusicPatchBank             music.PatchBank
	MusicSoundFontPath         string
	MusicSoundFontChoices      []string
	MusicSoundFont             *music.SoundFontBank
	OnRuntimeSettingsChanged   func(gameplay.RuntimeSettings)
	OnInputBindingsChanged     func(InputBindings)
}
