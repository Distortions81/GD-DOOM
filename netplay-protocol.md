# GDSF Netplay Protocol Reference

This document describes the current binary relay protocol implemented under [`internal/netplay`](/home/dist/github/GD-DOOM/internal/netplay).

`GDSF` stands for `Go Doom Stream Format`.

## Scope

This protocol currently covers relay-backed:

- gameplay broadcast
- gameplay watch/view
- gameplay chat
- audio broadcast
- audio watch/view

The currently implemented wire format is still the relay stream format used for broadcast/watch and voice. The multiplayer gameplay design below is the planning target for the next major protocol stage.

## Transport

- Transport: reliable byte stream
- Native transport: TCP
- Browser transport: not specified here
- Endianness: little-endian for multibyte integers
- Framing: only the initial `hello` carries magic/version; steady-state records do not

## Connections

The implementation uses separate TCP connections for gameplay and audio.

Current roles:

- `1`: `broadcaster`
- `2`: `viewer`
- `3`: `server`
- `4`: `audio_broadcaster`
- `5`: `audio_viewer`
- `6`: `player_peer`

Current protocol version:

- `2`

## Hello

The first record on every connection is a `hello`.

Binary layout:

- `magic[4]`
- `version[1]`
- `role[1]`
- `flags[2]`
- `session_id[8]`
- `payload_len[4]`
- `payload[payload_len]`

Field details:

- `magic`: ASCII `"GDSF"`
- `version`: protocol version, currently `2`
- `role`: one of the role ids above
- `flags`: role-specific feature flags
- `session_id`:
  - gameplay broadcaster: `0` requests new session id assignment
  - viewers/audio peers: existing session id
- `payload`: session metadata encoded by `marshalSessionConfig`

Header size before payload: `20` bytes.

### Hello Flags

Current flags:

- `0x4000`: `gameplay_compact_v1`
- `0x8000`: `audio_compact_v1`

`gameplay_compact_v1` is required on gameplay connections.
`audio_compact_v1` is required on dedicated audio connections. When negotiated, the audio socket uses compact audio records instead of the generic gameplay frame header.

## SessionConfig Payload

The `hello.payload` format is:

- `wad_hash_len[2]`
- `wad_hash[wad_hash_len]`
- `map_name_len[2]`
- `map_name[map_name_len]`
- `game_mode_len[2]`
- `game_mode[game_mode_len]`
- `max_players[2]`
- `player_slot[1]`
- `skill_level[1]`
- `cheat_level[1]`
- `reserved[1]`
- `session_flags[2]`

Current `session_flags` bits:

- bit `0`: `show_no_skill_items`
- bit `1`: `show_all_items`
- bit `2`: `fast_monsters`
- bit `3`: `respawn_monsters`
- bit `4`: `no_monsters`
- bit `5`: `auto_weapon_switch`
- bit `6`: `invulnerable`
- bit `7`: `source_port_mode`

Notes:

- Strings are raw bytes with `uint16` length prefixes.
- `max_players = 0` means unlimited players.
- Current session payload is reused for all roles, including audio roles.
- Audio clients currently send an empty/default session payload.

## Gameplay Connection

`gameplay_compact_v1` is required on gameplay connections.

### Frame Types

Current gameplay frame types:

- `1`: `keyframe`
- `4`: `tic_batch`
- `8`: `intermission_advance`
- `32`: `chat`
- `33`: `peer_tic_batch` (multiplayer peer sessions only)
- `34`: `roster_update` (multiplayer peer sessions only)
- `35`: `checkpoint` (multiplayer peer sessions only)
- `36`: `desync_request` (multiplayer peer sessions only)

### Keyframe

Binary layout:

- `type[1] = 1`
- `flags[1]`
  - bit `0`: `mandatory_apply`
  - bit `1`: `zstd_compressed`
- `length[4]`
- `tic[4]`
- `payload[length]`

Header size before payload: `10` bytes.

Viewer behavior:

- viewers receive the latest retained keyframe on join
- live viewers do not continuously apply keyframes during normal streaming

### Tic Batch

Binary layout:

- `tag[1]`
  - high bits `01`
  - low 6 bits = `count`
- `tics[count * 4]`

Each packed tic is:

- `forward[1]`
- `side[1]`
- `angle_turn_hi[1]`
- `buttons[1]`

Packed tic size: `4` bytes.

Total overhead before tic data: `1` byte.

The `tic` anchor is not transmitted on the wire for compact `tic_batch` records.

Internally, the implementation reconstructs a normalized payload shape of:

- `count[2]`
- `tics[count * 4]`

Those `count[2]` bytes are not transmitted on the wire.

### Intermission Advance

Binary layout:

- `type[1] = 8`

Total size: `1` byte.

### Chat

Binary layout:

- `type[1] = 32`
- `name_len[1]`
- `text_len[2]`
- `reserved[1]`
- `name[name_len]`
- `text[text_len]`

Field limits:

- `name_len <= 128`
- `text_len <= 512`

Current relay behavior:

- gameplay viewers can send chat frames upstream to the relay
- gameplay broadcasters can also send chat frames on the gameplay socket
- the relay forwards chat to all other gameplay participants in the same session

## Peer Gameplay Connection

`player_peer` role uses `gameplay_compact_v1` flag and a bidirectional connection: the peer sends its own tics upstream and receives tagged tics from all other peers.

### Session Join

- `session_id = 0` in the hello creates a new session; the server assigns an ID.
- `session_id != 0` joins an existing session.
- The server ack hello uses `role = server`. `player_slot` in the ack payload is the assigned player ID (1–4).
- If no slot is available, the connection is closed.

On join the server sends:

1. Server hello ack with assigned `player_slot`.
2. Latest retained keyframe (if any).
3. Tic backlog after that keyframe (if any).
4. A `roster_update` frame listing all currently active player IDs.

After all other peers join they also receive an updated `roster_update`.

### Peer Tic Batch

Binary layout:

- `type[1] = 33`
- `player_id[1]`
- `count[2]`
- `tics[count * 4]`

Each packed tic is identical to the single-player `tic_batch` format.

The relay server forwards each `peer_tic_batch` to all *other* peers in the session unchanged. The `player_id` field identifies the originating player. Peers do not receive their own tics back.

### Roster Update

Binary layout:

- `type[1] = 34`
- `count[1]`
- `player_ids[count]`

Sent by the server whenever a peer joins or leaves the session. `player_ids` is the full active roster after the change.

### Checkpoint

Binary layout:

- `type[1] = 35`
- `flags[1] = 0`
- `length[4] = 8`
- `tic[4]`
- `payload[8]`:
  - `tic[4]` — world tic the hash was computed at
  - `hash[4]` — FNV-1a 32-bit hash of key simulation state

The canonical peer (player slot 1) sends a `checkpoint` frame every 175 tics (~5 seconds). The server relays it to all other peers in the session.

Non-canonical peers compare `hash` against their locally computed `SimChecksum()`. On mismatch they send a `desync_request`.

The checkpoint hash covers: world tic, RNG state (both game and cosmetic indices), local and remote player positions/angles/health/armor/ammo/weapon, all sector floor and ceiling heights, and all active monsters (position, angle, HP, AI state).

### Desync Request

Binary layout:

- `type[1] = 36`
- `flags[1] = 0`
- `length[4] = 8`
- `tic[4]`
- `payload[8]`:
  - `tic[4]` — world tic where mismatch was detected
  - `local_hash[4]` — this peer's locally computed hash

Sent by a peer to the server when its local `SimChecksum()` does not match a received `checkpoint` hash. The server responds by pushing the most recent stored keyframe to the requesting peer with `mandatory_apply` set, causing an immediate resync.

### Current API Surface (peer)

- `DialPlayerPeer` — connect as a `player_peer`
- `PlayerPeer.SendTic` / `PlayerPeer.Flush` — send local player's tic upstream
- `PlayerPeer.PollPeerTic` — receive a tagged tic from another peer
- `PlayerPeer.PollRoster` — receive a roster change notification
- `PlayerPeer.PollKeyframe` — receive a mid-join or resync keyframe
- `PlayerPeer.SendChat` / `PlayerPeer.PollChat` — chat
- `PlayerPeer.SendCheckpoint` — emit periodic hash (canonical peer only)
- `PlayerPeer.PollCheckpoint` — receive canonical peer's hash
- `PlayerPeer.SendDesyncRequest` — notify server of detected desync

## Audio Connection

When `audio_compact_v1` is present in `hello.flags`, the dedicated audio socket does not use the generic gameplay frame header.

Instead, it uses compact audio records.

Current audio stream behavior:

- config is sent explicitly
- voiced chunks are sent with compact headers
- fully silent voice frames are not sent at all by the voice sender

### AudioFormat Record

Binary layout:

- `record_type[1] = 0x80`
- `codec[1]`
- `bits_per_sample[1]`
- `sample_rate_choice[1]`
- `sample_rate[4]`
- `channels[2]`
- `packet_duration_ms[2]`
- `packet_samples[2]`
- `bitrate[4]`

Total record size: `18` bytes, including `record_type`.

Current codec ids:

- `2`: `pcm16_mono`
- `3`: `g726_32`
- `4`: `silk_v3`

Current `bits_per_sample` usage:

- `pcm16_mono`: usually `16`
- `g726_32`: normalized to `2..5` in the current implementation
- `silk_v3`: `0`

Current sample rate choice ids:

- `0`: `custom`
- `1`: `16000`
- `2`: `24000`
- `3`: `32000`
- `4`: `48000`

The broadcaster may send a new `AudioFormat` record after the stream has already started. When that happens, subsequent audio chunks use the new format immediately.

### AudioChunk Record

Binary layout:

- `flags[1]`
- `payload_len[2]`, only for codecs with variable payload size
- `payload[n]`

Header size before payload:

- fixed-size codecs: `1` byte
- variable-size codecs: `3` bytes

Current audio chunk flags:

- bit `0`: `silence`

Flag rules:

- `silence`:
  - payload length must be `0`

### Audio Payload Length Rules

Payload length is implied by the current `AudioFormat` and the chunk flags.

For `pcm16_mono`:

- payload bytes = `packet_samples * channels * 2`

For `g726_32`:

- payload bytes = `packet_samples * channels * bits_per_sample / 8`

For `silk_v3`:

- payload length is transmitted explicitly as `payload_len[2]`
- payload must be non-empty for non-silent chunks
- maximum payload length is `65535` bytes

If the payload length does not match the implied length, the record is invalid.

### Current Voice Stream Parameters

Current voice path values:

- capture rate: `48000`
- default encoded rate: `48000`
- channels: `1`
- default SILK packet duration: `20 ms`
- default packet samples after resample: `960`
- default SILK bitrate: `64000`
- compact audio chunk header: `1` byte for fixed-size codecs, `3` bytes for SILK

Current silence policy:

- fully silent voice frames are not transmitted
- silence therefore consumes `0` audio records on the wire

## Server Relay Behavior

### Gameplay

The relay server:

- accepts one gameplay broadcaster per session
- accepts multiple gameplay viewers
- stores:
  - latest keyframe
  - keyframe flags
  - keyframe tic
  - subsequent tic/intermission backlog
- sends to late joiners:
  - session `hello`
  - latest retained keyframe, if any
  - retained backlog after that keyframe
- does not retain chat history for late joiners

### Audio

The relay server:

- accepts one audio broadcaster per session
- accepts multiple audio viewers
- stores:
  - latest `AudioFormat`
- does not retain audio chunks for late joiners
- sends to late audio joiners:
  - session `hello`
  - latest `AudioFormat`, if any

## Current API Surface

Current entry points:

- gameplay broadcaster: `DialRelayBroadcaster`
- gameplay viewer: `DialRelayViewer`
- audio broadcaster: `DialRelayAudioBroadcaster`
- audio viewer: `DialRelayAudioViewer`

Current gameplay send/receive methods:

- `BroadcastTic`
- `BroadcastKeyframe`
- `BroadcastIntermissionAdvance`
- `PollTic`
- `PollKeyframe`
- `PollIntermissionAdvance`

Current audio send/receive methods:

- `BroadcastAudioFormat`
- `BroadcastAudioConfig`
- `BroadcastAudioChunk`
- `PollAudioFormat`
- `PollAudioConfig`
- `PollAudioChunk`

Current gameplay chat methods:

- `SendChat`
- `PollChat`

## Invalid Data Handling

Current parser expectations:

- bad hello magic or version aborts the connection
- unsupported role aborts the connection
- audio chunk before audio format aborts the audio connection
- mismatched implied audio payload size aborts the audio connection
- unexpected gameplay frame type aborts the gameplay connection

## Bandwidth Notes

Current obvious steady-state overhead:

- gameplay `tic_batch` overhead: `1` byte per batch
- gameplay `intermission_advance`: `1` byte
- gameplay `keyframe` header: `10` bytes
- audio chunk header: `1` byte per voiced audio chunk

Current obvious remaining savings opportunities:

- reduce connect-time `hello` cost on dedicated audio sockets if needed

## Compatibility

This document describes the current implemented format, not a future design target.

If the wire format changes:

- bump `version`, or
- negotiate a new feature flag in `hello.flags`

Do not silently change binary layouts for an existing version/flag combination.

## Multiplayer Gameplay Plan

This section describes the intended protocol direction for general DOOM multiplayer gameplay. It is a design target, not an implemented format.

### Core Model

- no gameplay host; all gameplay participants are peers
- every peer runs the full deterministic simulation
- network traffic carries player inputs, membership changes, chat/voice, and join/resume keyframes
- gameplay advances in lockstep by tic for the current active player roster

### Session Lifetime

- sessions are independent of any one player
- a session stays alive while at least one player remains
- when the last player leaves, the session enters an empty grace window
- if no player rejoins before the grace timeout expires, the session closes
- while empty, simulation is paused and the last accepted gameplay keyframe is retained for resume

Suggested session states:

- `forming`
- `active`
- `paused_empty`
- `closed`

Suggested session metadata additions:

- `session_mode`
- `max_players`, where `0` means unlimited
- `empty_timeout_ms`
- membership epoch / roster version
- reserved team fields for later deathmatch and team modes

### Peer Roles

Gameplay should move away from broadcaster/viewer semantics for multiplayer sessions.

Suggested multiplayer roles:

- `player_peer`
- optional later `spectator_peer`

A relay/coordinator may still exist for discovery, transport, and session persistence, but it is not the gameplay authority.

### Lockstep Input Stream

Each active player contributes one command per tic. The base packet should stay close to classic Doom demo tics:

- `forward`
- `side`
- `angle_turn`
- `buttons`

Suggested packet shape:

- `player_id`
- `start_tic`
- `count`
- packed tic commands

Gameplay tics should advance only when commands for that tic are available from all currently active players in the roster epoch.

### Mid-Join

Mid-join is required.

The existing runtime keyframe approach can be reused for multiplayer join and resume:

- active peers periodically retain joinable gameplay keyframes
- a joining peer requests snapshot metadata from current players
- peers advertise snapshot identity before any full blob transfer
- once a canonical snapshot is chosen, one agreeing peer sends the full keyframe blob
- the joining peer loads that keyframe, verifies it, then begins lockstep after a short startup buffer

Suggested snapshot identity fields:

- `membership_epoch`
- `base_tic`
- `world_hash`
- `rng_hash` or RNG position hash
- `roster_hash`
- `blob_hash`

### Keyframe Acceptance

To decide whether a keyframe is good enough for mid-join or resume:

- with `1` active player, that player's keyframe is canonical
- with `2` active players, select a deterministic tie-break peer
- with `3` or more active players, majority matching snapshot identity wins
- if there is no majority, reject the join attempt and mark the session as desynced for recovery purposes

The `2`-player tie-break must be deterministic across all peers. Candidate rules:

- lowest `player_id`
- earliest join sequence in the current session

Peers should compare snapshot identity first and only transfer the full keyframe blob after selecting the canonical candidate.

### Desync Handling

Peer-symmetric gameplay requires routine consistency checks.

Suggested mechanism:

- peers send periodic compact checkpoint hashes
- checkpoints include current tic, world hash, roster hash, and RNG hash
- if hashes diverge, the session enters a desync state
- desynced peers should not participate in keyframe-majority decisions until resynced

### Session Events

Beyond raw movement tics, the gameplay protocol will need deterministic session events tied to a tic:

- player join accepted
- player leave/drop
- player death
- player respawn
- map exit accepted
- map transition start/commit
- pause/resume, if supported

### Initial Implementation Scope

Recommended first multiplayer milestone:

- peer-symmetric co-op only
- relay/coordinator allowed, but not gameplay-authoritative
- lockstep tic exchange for the active roster
- mid-join via retained runtime keyframes
- majority-based snapshot acceptance
- session persists through empty periods until grace timeout

Later modes such as deathmatch and team play should layer on the same session and roster model rather than requiring a separate transport design.
