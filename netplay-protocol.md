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

It is not a multiplayer gameplay protocol.

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

Current protocol version:

- `1`

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
- `version`: protocol version, currently `1`
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
