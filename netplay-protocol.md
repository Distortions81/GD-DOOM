# GDSF Relay Protocol Draft

This document describes the intended binary protocol for relay-backed `broadcast` / `watch`.

`GDSF` stands for `Go Doom Stream Format`.

## Preface

Current state:

- `broadcast` and `watch` exist as a direct TCP host-to-viewer path
- transport is currently implementation-first rather than final-form
- the feature works as an early watch-only slice, not yet as a relay-backed public system

This document describes the intended direction from that starting point.

This protocol is specifically for streaming, relay, watch, and spectator use cases.

Full multiplayer should use a separate protocol family with:

- its own magic
- its own default port
- its own packet semantics

That separation keeps the stream/watch protocol optimized for viewing and retention while leaving the multiplayer protocol free to optimize for interactive play.

The next architectural step is to split the system into three explicit roles:

- broadcaster
- server
- viewer

In that model:

- the broadcaster publishes gameplay state to the server
- the server retains and relays session data
- viewers connect to the server rather than directly to the broadcaster

The first practical steps toward that model are:

1. replace the current direct stream format with the intended binary protocol
2. preserve demo-style compact tic transport
3. introduce a relay server that can accept a single broadcaster and one or more viewers
4. move viewers onto server-mediated session lookup and streaming

The initial relay rollout does not need to solve multiplayer immediately. The first goal is a solid watch pipeline:

- broadcaster -> server -> viewer

Once that path is stable, the same architecture can expand to:

- browser viewers
- late join via keyframes
- seek/scrub
- spectators

The design goals are:

- demo-style compact tic transport
- fast late join and scrub
- server-side rolling retention
- public broadcast discovery
- browser-based viewing via WASM
- clean evolution path to richer spectator systems
- spectator support
- spectator camera flexibility
- optional audio and asset side channels
- exact content identification at session start

## Overview

The broadcaster connects to a server and publishes:

- session identity and compatibility metadata
- compact demo-style tics
- periodic keyframes for random access
- optional music/audio/assets

Viewers connect to the server and can:

- watch live
- late join
- seek within the retained timeline
- optionally receive assets and compressed audio

Public viewers should also be able to watch from a browser via a WASM client over WebSocket.

The protocol should also leave room for future richer spectator systems built on the same watch/relay foundation.

The server stores:

- a rolling tic buffer for the last `X` seconds
- keyframes, typically one per second
- optional cached assets keyed by hash
- public session metadata for discovery pages
- viewer counts and session duration
- active spectator role information

## Transport Model

Use binary framing over a reliable byte stream.

Only the initial `hello` packet carries protocol magic. Normal frames do not.

Why:

- cheap steady-state framing
- simple parser
- explicit session bootstrap
- no JSON overhead

Native clients can use raw TCP. Browser clients should use WebSocket with the same logical binary frames.

## Hello Packet

The first packet on a connection is `hello`.

It should contain:

- `magic[4] = "GDSF"`
- `version[1]`
- `role[1]`
  - broadcaster
  - spectator
  - browser spectator
  - viewer
  - server
- `flags[2]`
- `session_id[8]`

It should also contain initial session metadata:

- game/session flags
- skill
- player slot
- current map name
- IWAD name + hash
- PWAD names + hashes in load order
- music backend
- soundfont name + hash + size
- OPL bank name + hash + size
- current song identity
- capability flags for keyframes, audio, and asset relay
- capability flags for spectator camera control

This packet answers:

- can the viewer join this stream?
- does the viewer already have matching content?
- what optional media/assets are available?

It should also provide enough metadata for the public broadcast directory and browser watch page.

## Normal Frame Header

After `hello`, use a lightweight frame header for all subsequent packets.

Suggested layout:

- `type[1]`
- `flags[1]`
- `reserved[2]`
- `length[4]`
- `tic[4]`

Header size: `12` bytes.

`tic` semantics depend on frame type:

- `tic_batch`: first tic in the batch
- `keyframe`: tic captured by the keyframe
- `seek`: requested target tic
- `stream_state`: live tail tic or relevant anchor tic

## Frame Types

Suggested frame type ids:

- `1` = `session_info`
- `2` = `stream_state`
- `3` = `keyframe`
- `4` = `tic_batch`
- `5` = `seek`
- `6` = `close`
- `7` = `error`
- `16` = `audio_config`
- `17` = `audio_chunk`
- `32` = `asset_offer`
- `33` = `asset_chunk`
- `34` = `asset_complete`

Future stream/spectator control frames can extend this set, for example:

- `50` = `spectator_view`

Exact ids are flexible, but gameplay, audio, and asset families should remain clearly separated.

## Tic Format

Tics should use the same compact payload shape as demo input.

One tic:

- `forward[1]`
- `side[1]`
- `angle_turn_hi[1]`
- `buttons[1]`

Size: `4` bytes per tic.

This matches demo-style transport and keeps bandwidth very low.

## Tic Batch Payload

Do not send one frame per tic. Send batches.

Suggested `tic_batch` payload:

- `count[2]`
- `reserved[2]`
- `tics[count * 4]`

The frame header `tic` field is the first tic number in the batch.

Example:

- header `tic = 1050`
- payload `count = 35`

This batch covers tics `1050..1084`.

## Keyframe Strategy

The server should retain keyframes, typically one every second.

At `35` tics per second, that means a keyframe every `35` tics.

Keyframes are not sent continuously to viewers. They are sent on:

- initial watch start
- late join
- seek/scrub

This keeps live viewer bandwidth low while preserving fast random access.

## Keyframe Payload

Keyframes should contain dynamic runtime state only.

Do not include:

- full map geometry
- static WAD-derived content
- textures
- other immutable content

The viewer should resolve static content locally using the IWAD/PWAD identities from `hello`.

Suggested `keyframe` payload:

- `keyframe_version[2]`
- `codec[2]`
  - raw
  - compressed
- `blob_len[4]`
- `blob[blob_len]`

The blob should include:

- world tic
- player state
- inventory/ammo/weapons
- thinker state
- monster state
- projectiles/puffs/impacts
- doors/floors/plats/ceilings
- sector dynamic state
- RNG state
- delayed switch reverts
- any other mutable state needed for deterministic resume

The existing savegame serializer is a useful starting point, but the net keyframe format should avoid repeatedly embedding static map/template data.

## Seek Flow

Viewer sends `seek`:

- target tic
- mode
  - nearest exact replay
  - live tail

Server responds with:

1. nearest keyframe at or before target tic
2. tic batches from keyframe tic forward to target
3. live tic batches if the viewer requested tail/live mode

## Stream State

`stream_state` should expose the retained timeline and session state needed by the UI.

Suggested fields:

- head tic
- tail tic
- live tic
- keyframe interval
- paused/intermission/finale/menu flags
- current map
- current song
- active player slots
- spectator count
- currently followed player when applicable
- whether free-roam spectator mode is allowed

This lets viewers render scrub bars and display current session context.

## Content Identity

The protocol should identify content precisely at session start.

Include in `hello`:

- IWAD name + hash
- PWAD names + hashes in load order
- current map name
- gameplay-affecting flags

This ensures the viewer can validate exact compatibility before accepting keyframes or tic batches.

The server should also classify the IWAD into a display-friendly category for public listing, for example:

- `doom-shareware`
- `doom-registered`
- `ultimate-doom`
- `doom2`
- `tnt`
- `plutonia`
- `unknown`

## Metadata

Session metadata should distinguish between:

- compatibility-critical fields
- presentation fields

Compatibility-critical:

- WAD identities
- map
- skill
- gameplay flags
- source-port mode

Presentation fields:

- current song
- music backend
- soundfont identity
- OPL bank identity

Presentation metadata should be visible to viewers, but should not block watching if gameplay compatibility is otherwise valid.

Useful public-directory metadata includes:

- session id
- public/private visibility
- current map
- IWAD class
- PWAD list
- soundfont / OPL bank identity
- music backend
- current song
- viewer count
- spectator count
- active player count
- duration

## Audio Side Channel

Gameplay sync should not depend on audio delivery.

If compressed audio is supported, keep it on a separate side channel within the protocol:

- `audio_config`
- `audio_chunk`

The planned codec implementation should use [`pion/opus`](https://github.com/pion/opus).

This should be the default path for:

- broadcaster music/audio relay
- broadcaster voice if added
- multiplayer voice channels later

Suggested `audio_config` fields:

- codec
- sample rate
- channels
- frame duration
- stream id

Suggested codec values:

- `opus`

Suggested stream classes:

- broadcast music
- broadcast voice
- multiplayer voice

Suggested `audio_chunk` fields:

- stream id
- timestamp or tic anchor
- compressed audio bytes

Audio should be tied to a timing base that is compatible with both:

- passive browser/native viewing
- future multiplayer voice playback

Viewers should be able to ignore audio and still watch normally.

## Opus Plan

The current plan is to standardize on `pion/opus` for compressed realtime audio.

Reasons:

- suitable for speech and music
- compact enough for relay fanout
- works well for browser/native streaming scenarios
- provides a unified path for broadcast audio and multiplayer voice

The protocol should treat Opus streams as optional side channels and should not mix them into gameplay/keyframe transport.

## Audio Stream Roles

Audio streams should be role-aware.

At minimum the design should allow:

- broadcaster music stream
- broadcaster voice stream
- future participant voice streams in richer session types

Suggested metadata for an audio stream:

- stream id
- stream role
- owner role
  - broadcaster
  - player
  - camera man
  - director
- owner id
- codec
- sample rate
- channel count
- frame duration
- human-readable label

This allows the same side-channel mechanism to support both one-to-many broadcast audio and future richer-session voice use cases.

## Asset Relay

Music-related assets should be transferred separately from gameplay state.

This includes:

- soundfonts (`.sf2`)
- OPL banks
- optionally non-commercial music assets

Suggested asset frames:

- `asset_offer`
- `asset_chunk`
- `asset_complete`

Asset metadata should include:

- asset type
  - soundfont
  - OPL bank
  - music lump/blob
- asset name
- asset hash
- asset size
- codec/compression mode if applicable

The server should cache assets by hash so the broadcaster uploads them once and viewers fetch them on demand.

Assets must not block watch startup. A viewer should be able to:

- watch immediately with fallback/no music
- switch to exact audio once the asset transfer completes

## Music and Soundfont Metadata

The protocol should provision for the broadcaster’s music presentation settings:

- music backend
- soundfont name/hash/size
- OPL bank name/hash/size

For exact music reproduction, the `.sf2` should be transferable as an optional asset.

## Public Broadcast Directory

The relay should expose public broadcasts to a web UI.

The intended web stack is:

- Go HTTP server
- `html/template` for server-rendered pages
- plain HTML/CSS/JS for the browser shell
- WASM client for actual viewing

The directory page should show:

- active broadcasts
- viewer counts
- spectator counts
- duration
- current map
- IWAD class
- PWAD summary
- music backend
- soundfont / OPL bank summary

For richer spectator sessions it may also show:

- whether camera men are present
- whether a director feed is active

The directory should distinguish between public and private sessions.

Broadcasters should automatically publish to the server when `broadcast` starts, without requiring manual direct peer setup.

## Browser Viewing

Public viewing should use the WASM build and connect to the relay over WebSocket.

The intended flow is:

- browsing happens in normal server-rendered HTML
- choosing a broadcast opens a dedicated watch page
- the watch page launches the WASM app
- the WASM app connects to the relay over WebSocket

Suggested user flow:

1. user opens the public broadcasts page
2. user clicks `Watch`
3. browser opens a watch page
4. watch page launches the WASM client
5. WASM client connects to the relay over WebSocket
6. relay sends:
   - session metadata
   - keyframe for start/seek
   - tic batches for catch-up
   - live tic batches for tail mode

Suggested URL shapes:

- `/watch/<session-id>`
- `/watch?id=<session-id>`

Optional query params:

- `t=<tic>` for seek target
- `live=1`
- `mute=1`

Suggested WebSocket endpoint:

- `/ws/watch/<session-id>`

WebSocket frames should carry the same logical binary protocol as native clients so the system remains coherent across native and browser viewers.

Suggested page split:

- `/broadcasts`
  - public broadcast list
  - server-rendered via `html/template`
  - includes watch links/buttons
- `/watch/<session-id>`
  - server-rendered shell page
  - shows metadata and loading state
  - loads the WASM viewer

The non-WASM page should be responsible for:

- rendering metadata
- rendering loading/error UI
- loading the JS/WASM bootstrap
- passing session id, mode, and endpoint information into the viewer

The WASM app should be responsible for:

- opening the WebSocket
- decoding binary frames
- applying keyframes
- consuming tic batches
- rendering the watch client

For spectator mode, the client should also support:

- following a selected player
- switching viewed players
- free-roam camera mode
- following camera men
- following a director feed

## Broadcaster Publish Flow

When broadcast starts:

1. broadcaster auto-connects to the relay
2. broadcaster sends `hello`
3. broadcaster publishes session metadata
4. broadcaster streams tic batches
5. broadcaster uploads periodic keyframes
6. broadcaster updates live state such as map/song/viewer-visible metadata

This replaces direct host-to-viewer discovery for public sessions.

For future richer spectator sessions:

- the broadcaster remains the authoritative publisher
- spectators connect as read-only viewers
- camera men and directors are special spectator roles in this protocol family

## Web/API Surface

The server should expose both relay endpoints and HTTP endpoints.

Useful HTTP endpoints:

- `GET /broadcasts`
- `GET /watch/:id`
- `GET /api/broadcasts`
- `GET /api/broadcasts/:id`
- `GET /api/assets/:hash`

Useful streaming endpoints:

- `WS /ws/watch/:id`

The HTTP API should be sufficient for the public listing page to show:

- active public sessions
- current viewer count
- current spectator count
- duration
- IWAD class
- PWADs
- soundfont / backend information
- camera-man / director presence when applicable

This should remain framework-light. The preferred implementation is:

- Go handlers
- `html/template`
- plain JS for page bootstrap and lightweight interactions
- no large frontend framework unless later requirements justify it

## Retention Model

Server retention should be based on:

- rolling time window
- optional total byte cap

Server stores:

- tics continuously
- keyframes periodically
- optional assets by hash

Keyframes should exist server-side for fast seek/join, but should only be sent to viewers when needed.

This same mechanism should support spectator late-join in richer stream/session formats.

## Spectator Evolution

The current protocol is centered on broadcaster-to-viewer streaming, but it should evolve cleanly into richer spectator systems.

Long-term session roles:

- broadcaster / host
- spectator

Spectator-capable sessions should also leave room for spectator sub-roles:

- camera man
- director

Spectators should use the same core sync path as `watch`:

- keyframe on join or seek
- tic catch-up
- live tail thereafter

This keeps spectator mode aligned with the existing watch design.

## Spectator Camera Modes

Spectators should support at least two camera modes:

1. player follow
   - spectator watches a chosen player
   - spectator can switch active target between player slots

2. free roam
   - no in-world body
   - camera is detached from player entities
   - view-only, no gameplay interaction

Suggested spectator state fields:

- camera mode
  - follow-player
  - free-roam
- target player slot
- free-roam position
- free-roam angle
- free-roam pitch if applicable

Spectator camera changes are read-only operations and must not affect gameplay simulation.

## Spectator Roles

Not all spectators need to be equivalent. The protocol should leave room for at least two authored-view roles:

1. camera man
   - a spectator with a controllable free-roam camera
   - produces a view that other spectators may choose to follow

2. director
   - a spectator with authority to choose the preferred public view
   - can switch between:
     - player views
     - camera man views
     - potentially fixed/automated views later

This allows the viewing experience to work more like a live production pipeline rather than a purely individual spectator system.

## View Targets

A spectator should be able to watch:

- a player
- a camera man
- the director program feed
- their own local free-roam view

Suggested target model:

- `target_kind`
  - player
  - cameraman
  - director-feed
  - self-free-roam
- `target_id`

Where:

- `player` targets a player slot
- `cameraman` targets a named or indexed spectator camera source
- `director-feed` follows the current director-selected source
- `self-free-roam` means the spectator is operating locally and not following another source

## Director Feed

The protocol should provision for a single logical "program feed" selected by a director.

The director feed is not separate gameplay state. It is metadata describing which existing view target should be considered the preferred broadcast view.

Useful director operations:

- cut to player N
- cut to camera man N
- switch back to local follow mode
- optionally mark named shots or presets later

Spectators can then choose one of two high-level behaviors:

- follow the director feed
- ignore the director and choose their own target

## Camera Man State

A camera man should have normal spectator free-roam state, plus an identity that other spectators can target.

Suggested camera man metadata:

- camera id
- display name
- current free-roam transform
- active/public flag

This lets the server expose a list of available camera feeds.

## Directory / Session Metadata

For multiplayer/spectator sessions, useful public metadata may include:

- active player count
- spectator count
- camera man count
- whether a director is active
- current director target kind/id

This is useful both for the public web directory and for in-viewer camera selection UI.

## Rate Limiting

The server should enforce broadcaster input limits:

- retain up to `X` seconds of tics
- cap average publish rate to `X` tics/sec

This protects the relay from malformed or abusive publishers and keeps timeline math sane.

## First Implementation Slice

The first useful implementation should be:

1. binary `hello`
2. binary `tic_batch` using demo-style tic payloads
3. server-side rolling tic buffer
4. public session registry and metadata
5. browser watch path over WebSocket
6. binary `keyframe` frame type reserved, even if not fully wired yet

The next slice should add:

1. 1 Hz server-retained keyframes
2. `seek`
3. late join from keyframe + tic replay
4. public broadcasts page and watch page

After that:

1. asset relay
2. optional audio side channel
3. spectator camera selection
4. multiplayer player roles

## Development Stages

The project should be built in deliberate stages so each milestone is usable on its own.

### Stage 1: Binary Broadcast Baseline

Goals:

- replace JSON tic transport with binary framing
- keep demo-style compact tic payloads
- preserve current host-to-viewer watch behavior

Features:

- binary `hello`
- binary `tic_batch`
- protocol versioning
- content identity in handshake
- direct broadcaster-to-viewer watch path

Success criteria:

- current `broadcast` / `watch` flow works with binary transport
- tic bandwidth drops to demo-like levels

### Stage 2: Relay Server Core

Goals:

- move from direct connections to a central relay
- make broadcasters publish automatically to the server
- make sessions discoverable by id

Features:

- relay server process
- session registry
- broadcaster auto-publish
- viewer connect by session id
- rolling tic buffer
- basic rate limiting

Success criteria:

- broadcaster can publish to server
- viewer can connect through server and watch live

### Stage 3: Public Directory and Browser Viewing

Goals:

- expose public sessions on the web
- make one-click browser viewing work

Features:

- Go web server handlers
- `html/template` broadcast list
- `html/template` watch page
- WASM viewer bootstrap
- WebSocket watch transport
- session metadata listing:
  - viewers
  - duration
  - IWAD class
  - PWAD summary
  - soundfont/backend

Success criteria:

- user can open `/broadcasts`
- user can click `Watch`
- browser launches the WASM viewer and joins the live session

### Stage 4: Keyframes and Late Join

Goals:

- allow late join without requiring map-start alignment
- support initial seek/scrub foundation

Features:

- server-retained keyframes
- dynamic-state-only keyframe blobs
- keyframe cadence, e.g. 1 Hz
- join from nearest keyframe plus tic catch-up

Success criteria:

- a viewer can join mid-session and catch up to live
- start time is fast enough for public viewing

### Stage 5: Seek and Scrub

Goals:

- make retained sessions navigable
- support replay-like viewing within the retention window

Features:

- `seek` frame
- `stream_state` retained range reporting
- keyframe lookup by tic
- tic replay from keyframe to target
- watch page scrub UI

Success criteria:

- viewer can seek to a tic in the retained window
- viewer can return to live tail cleanly

### Stage 6: Asset Relay

Goals:

- reproduce broadcaster presentation more faithfully
- avoid repeated manual asset setup on viewers

Features:

- asset offer/chunk/complete frames
- server-side asset cache by hash
- soundfont (`.sf2`) transfer
- OPL bank transfer
- optional non-commercial music asset transfer

Success criteria:

- viewer can receive missing soundfont/bank assets from the relay
- watch startup still works even before assets finish downloading

### Stage 7: Audio Side Channel

Goals:

- add optional synchronized audio alongside gameplay
- support broadcaster music and voice

Features:

- `pion/opus` integration
- `audio_config`
- `audio_chunk`
- broadcaster music stream
- broadcaster voice stream
- browser/native playback support

Success criteria:

- a viewer can optionally hear broadcaster audio while watching
- gameplay sync remains independent of audio delivery

### Stage 8: Spectator Camera System

Goals:

- make spectator viewing more flexible and useful
- support authored shots and detached camera work

Features:

- player-follow spectator mode
- free-roam spectator mode
- switch viewed player
- camera man role
- director role
- director-selected program feed

Success criteria:

- a spectator can follow players or roam freely
- spectators can choose to follow a director feed

### Stage 9: Multiplayer Foundation

Goals:

- define a separate multiplayer protocol family
- keep `GDSF` focused on stream/watch/spectator use cases

Features:

- separate multiplayer magic
- separate multiplayer default port
- separate multiplayer packet semantics
- documented boundary between stream protocol and multiplayer protocol

Success criteria:

- stream/watch protocol remains cleanly scoped
- multiplayer can be developed without distorting `GDSF`

### Stage 10: Multiplayer Voice and Production Features

Goals:

- round out adjacent protocol families and produced spectator experiences

Features:

- multiplayer voice via the separate multiplayer protocol family
- camera man audio if desired
- director production metadata
- improved browser session UI
- optional session archive/export tooling

Success criteria:

- adjacent protocol families have voice support where appropriate
- broadcast-style produced spectator experiences are possible

### Stage 11: Joinable Broadcasts

Goals:

- support broadcasts that remain publicly viewable while also allowing invited or permitted players to join live
- bridge the stream/watch world and the future multiplayer protocol world without collapsing them into one protocol

Features:

- broadcaster-controlled joinable-broadcast mode
- `GDSF` session metadata indicating that live player joining is allowed
- link from a public broadcast session to an associated multiplayer session
- viewers continue to use `GDSF`
- active players use the separate multiplayer protocol

Success criteria:

- a session can be publicly broadcast and watched through `GDSF`
- selected players can join the same event through the separate multiplayer protocol
- stream/watch protocol remains cleanly scoped even in hybrid sessions

## Summary

The protocol should be:

- binary
- demo-tic-based for live control data
- keyframe-assisted for random access
- metadata-rich at session start
- explicit about content identity
- extensible for music/audio/assets

The core principle is:

- gameplay state stays compact and deterministic
- keyframes enable join/seek
- media and assets are optional side channels
