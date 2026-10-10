package runtimecfg

import (
	"context"
	"time"

	"gddoom/internal/audiofx"
	"gddoom/internal/demo"
	"gddoom/internal/gameplay"
	"gddoom/internal/mapdata"
	"gddoom/internal/media"
	"gddoom/internal/music"
	"gddoom/internal/netgame"
	"gddoom/internal/sound"
)

// DefaultAlwaysRun is shared by native and Ebiten launchers. The run modifier
// inverts it while held, so Shift walks with the default setting.
const DefaultAlwaysRun = true

// Common launcher defaults keep the native and Ebiten controls and audio aligned.
const (
	DefaultMouseLookSpeed    = 0.5
	DefaultKeyboardTurnSpeed = 1.0
	DefaultSmoothCameraYaw   = true
	DefaultMusicVolume       = 1.0
	DefaultSFXVolume         = 0.5
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

type WADSource struct {
	Name string
	Hash string
}

type LiveTicSource interface {
	PollTic() (demo.Tic, bool, error)
}

// AuthorityClient supplies authenticated full baselines and sends intent to the
// server. Implementations must bound their queues and keep these calls
// nonblocking so network I/O cannot stall the game/render goroutine.
type AuthorityClient interface {
	Welcome() netgame.Welcome
	PollTransition() (netgame.MapChange, bool, error)
	PollSnapshot() (netgame.Snapshot, bool, error)
	SendInputs(netgame.InputBatch) error
}

type AuthorityChatClient interface {
	SendChat(text string) error
	PollChat() (netgame.ChatEvent, bool, error)
}

// AuthorityJoinRequest joins a match using the WADs already loaded by the game.
type AuthorityJoinRequest struct {
	Address, Name string
	Spectator     bool
}

type AuthorityServerEntry struct {
	Label   string `json:"label" toml:"label"`
	Address string `json:"address" toml:"address"`
}

// Counts are negative when an older server does not advertise capacity.
type AuthorityServerInfo struct {
	Manifest           netgame.CompatibilityManifest
	Ping               time.Duration
	Players            int
	PlayerLimit        int
	Spectators         int
	Compatible         bool
	CompatibilityError string
}

// AuthorityJoinResult is prepared off the game thread. The caller owns Client
// and the supplied context for the entire session, including reconnection.
type AuthorityJoinResult struct {
	Client    AuthorityClient
	Map       *mapdata.Map
	Manifest  netgame.CompatibilityManifest
	MapLoader func(netgame.MapChange) (*mapdata.Map, error)
}

// AuthorityLocalRules preserves local game rules across a network session
// without overwriting rendering, sound, or input preferences changed in menus.
type AuthorityLocalRules struct {
	GameMode                                  string
	SkillLevel, PlayerSlot                    int
	NoMonsters, FastMonsters, RespawnMonsters bool
	WADHash                                   string
	AllCheats                                 bool
	CheatLevel                                int
	Invulnerable                              bool
	ShowAllItems, ShowNoSkillItems            bool
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

	// SendKeyframe uploads a full game state snapshot to the relay server.
	// The server stores it for late-joiners and desync recovery. Only the
	// canonical peer (slot 1) calls this periodically.
	SendKeyframe(tic uint32, blob []byte) error

	// PollKeyframe returns the next keyframe received from the server, if any.
	// mandatory is true when the frame must be applied immediately (desync recovery).
	PollKeyframe() (blob []byte, mandatory bool, ok bool)
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
	// Headless suppresses device-backed audio and render asset preparation for
	// callers that drive the simulation directly without an Ebitengine game loop.
	Headless                     bool
	Width                        int
	Height                       int
	StartZoom                    float64
	InitialDetailLevel           int
	AutoDetail                   bool
	InitialGammaLevel            int
	WADHash                      string
	WADSources                   []WADSource
	Debug                        bool
	DebugEvents                  bool
	PlayerSlot                   int
	SkillLevel                   int
	GameMode                     string
	ShowNoSkillItems             bool
	ShowAllItems                 bool
	MouseLook                    bool
	MouseInvert                  bool
	SmoothCameraYaw              bool
	MouseLookSpeed               float64
	KeyboardTurnSpeed            float64
	MusicVolume                  float64
	MUSPanMax                    float64
	MUSVolumeCompression         float64
	OPLVolume                    float64
	AudioPreEmphasis             bool
	MusicBackend                 music.Backend
	OpenMenuOnFrontendStart      bool
	SFXVolume                    float64
	PCSpeakerVolume              float64
	SFXPitchShift                bool
	PCSpeakerVariant             audiofx.PCSpeakerVariant
	SharedPCSpeaker              audiofx.PCSpeaker
	FastMonsters                 bool
	RespawnMonsters              bool
	NoMonsters                   bool
	AlwaysRun                    bool
	AutoWeaponSwitch             bool
	CheatLevel                   int
	Invulnerable                 bool
	SourcePortMode               bool
	SourcePortThingRenderMode    string
	SourcePortThingBlendFrames   bool
	ZombiemanThinkerBlend        bool
	DebugMonsterThinkerBlend     bool
	SourcePortSectorLighting     bool
	DisableDoomLighting          bool
	KageShader                   bool
	CRTEffect                    bool
	DisableWallOcclusion         bool
	DisableWallSpanReject        bool
	DisableWallSpanClip          bool
	DisableWallSliceOcclusion    bool
	DisableBillboardClipping     bool
	DisableMaskedMidFastPaths    bool
	GPURenderer                  bool
	MeshRenderer                 string // Empty disables the experimental triangle renderer.
	RendererWorkers              int
	TextureAnimCrossfadeFrames   int
	NoVsync                      bool
	NoFPS                        bool
	ShowTPS                      bool
	DisableAspectCorrection      bool
	DisableGeometryAspectCorrect bool
	InputBindings                InputBindings
	AllCheats                    bool
	StartInMapMode               bool
	FlatBank                     map[string][]byte
	FlatBankIndexed              map[string][]byte
	WallTexBank                  map[string]media.WallTexture
	WallTextureHeights           map[string]int
	Shareware                    bool
	WallTextureAnimSequences     map[string][]string
	FlatTextureAnimSequences     map[string][]string
	BootSplash                   media.WallTexture
	DoomPaletteRGBA              []byte
	DoomColorMap                 []byte
	DoomColorMapRows             int
	MenuPatchBank                map[string]media.WallTexture
	StatusPatchBank              map[string]media.WallTexture
	MessageFontBank              map[rune]media.WallTexture
	SpritePatchBank              map[string]media.WallTexture
	IntermissionPatchBank        map[string]media.WallTexture
	SoundBank                    media.SoundBank
	PCSpeakerBank                map[string][]sound.PCSpeakerTone
	DemoScript                   *demo.Script
	AttractDemos                 []*demo.Script
	DemoQuitOnComplete           bool
	DemoExitOnDeath              bool
	DemoStopAfterTics            int
	RecordDemoPath               string
	DemoTracePath                string
	TitleMusicLoader             func() (*music.ParsedMUS, error)
	MapMusicLoader               func(mapName string) (*music.ParsedMUS, error)
	MapMusicInfo                 func(mapName string) (levelLabel string, musicName string)
	IntermissionMusicLoader      func(commercial bool) (*music.ParsedMUS, error)
	FinaleMusicLoader            func(mapName string, secret bool) (*music.ParsedMUS, error)
	PlayCheatMusic               func(currentMapName string, code string) (bool, error)
	MusicPlayerCatalog           []MusicPlayerWAD
	MusicPlayerTrackLoader       func(wadKey string, lumpName string) ([]byte, error)
	NewGameLoader                func(mapName string) (*mapdata.Map, error)
	DemoMapLoader                func(demo *demo.Script) (*mapdata.Map, error)
	Episodes                     []int
	LiveTicSource                LiveTicSource
	AuthorityClient              AuthorityClient
	AuthorityMapLoader           func(netgame.MapChange) (*mapdata.Map, error)
	AuthorityJoin                func(context.Context, AuthorityJoinRequest) (AuthorityJoinResult, error)
	AuthorityJoinDefaults        AuthorityJoinRequest
	AuthorityServers             []AuthorityServerEntry
	AuthorityDiscover            func(context.Context, string) (AuthorityServerInfo, error)
	OnAuthorityServersChanged    func([]AuthorityServerEntry) error
	AuthorityLocalRules          *AuthorityLocalRules
	LiveTicSink                  LiveTicSink
	CoopPeers                    CoopPeerSource
	CaptureKeyframe              func() ([]byte, error)
	LoadKeyframe                 func([]byte) error
	WatchStartupBufferTics       int
	NetBandwidthMeter            NetBandwidthMeter
	VoiceBandwidthMeter          NetBandwidthMeter
	VoiceSyncMeter               VoiceSyncMeter
	VoiceCodec                   string
	VoiceG726BitsPerSample       int
	VoiceBitrate                 int
	VoiceSampleRate              int
	VoiceAGCEnabled              bool
	VoicePushToTalkEnabled       bool
	VoiceGateEnabled             bool
	VoiceGateThreshold           float64
	VoiceInputDevice             string
	VoiceInputLevel              func() float64
	VoiceInputGateActive         func() bool
	VoiceTransmitActive          func() bool
	OnVoiceSettingsChanged       func(VoiceSettings) error
	MusicPatchBank               music.PatchBank
	MusicSoundFontPath           string
	MusicSoundFontChoices        []string
	MusicSoundFont               *music.SoundFontBank
	OnRuntimeSettingsChanged     func(gameplay.RuntimeSettings)
	OnInputBindingsChanged       func(InputBindings)
}
