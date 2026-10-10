# Authoritative multiplayer

Status: implementation integrated and locally verified; extended release playtesting remains. This is the current multiplayer design,
superseding the peer-symmetric lockstep target in `netplay-protocol.md`.
Existing broadcast/watch sessions remain a separate supported protocol.

## Goal and release scope

Deliver 2–4 player co-op and casual deathmatch with desktop/browser cross-play.
A dedicated Go server owns the Doom simulation. Clients predict their own
movement and reconcile to server snapshots. Losing a player's packets must not
stop the other players, grant extra simulation time, duplicate an action, or
corrupt the shared world.

The release includes create/join, exact engine/content/rules compatibility,
late join, reconnect, individual death/respawn, co-op map progression, deathmatch
spawns/scoring/limits/rotation, and spectator viewing. Keep existing chat usable.
Voice transport redesign and competitive hitscan lag compensation are later
enhancements, not requirements for this release.

## Authority and simulation

* One owner advances a match in fixed 35 Hz tics. Rendering and network delivery
  have independent clocks. Never advance the world once per player.
* All players use the same player simulation, in ascending stable player ID
  order. Inventory, health, weapon timing, use/attack latches, movement and
  statistics belong to each player. Monsters, projectiles, sectors and RNG
  belong to the world. Target identity and projectile ownership are explicit.
* Clients send intent only. The server determines movement, collisions, damage,
  ammo, pickups, deaths, respawns, score and map transitions.
* Network readers enqueue bounded, validated messages. No socket operation may
  block the simulation owner. Slow receivers have bounded queues and are dropped
  or resynchronized instead of accumulating unlimited stale state.
* Commands cannot grant additional elapsed time. Consume at most one command
  for a player per server tic; reject duplicates, expired inputs and inputs
  beyond a bounded future window. Use session/map epochs to reject old traffic.
* Missing inputs briefly preserve forward/side movement, with turn and buttons
  cleared. Then neutralize movement. Do not repeat fire, use or weapon changes.
  Finalized tics never accept late commands. Timeout/removal does not stall peers.
* If the server is overloaded, bound catch-up work and expose overload. Do not
  skip physics tics. If a client is behind, replace its obsolete state with a
  fresh authoritative snapshot rather than replaying an unbounded backlog.

## Headless boundary

The server must start and simulate without a display, window, GPU or audio
device. Reuse existing Doom physics; do not introduce a second simplified game.
Expose map initialization, player lifecycle, fixed-tic stepping, snapshots and
restore independently of input polling, drawing and sound playback. Keep the
single-player/demo ordering intact and retain its parity regression tests.
Package-global gameplay RNG must not be shared unsafely between concurrent
matches; one match per server process keeps independent rooms isolated.

## Inputs, snapshots and prediction

Every command carries a sequence number and intended server tic. A connection
is bound to its server-assigned player ID; never trust a payload to choose the
player it controls. Bounded clock/lead negotiation maps client sampling to the
server's timeline. Reconnect establishes a fresh connection binding and input history. Map changes
establish a new match epoch; packets from either superseded identity are rejected.

Snapshots contain a monotonically ordered server tic, snapshot ID, baseline ID,
per-client finalized input acknowledgment, authoritative local player state,
entity states and lifecycle changes, relevant dynamic collision state, and
match/score state. Entity IDs include a generation. Delta snapshots reference
only a client-acknowledged baseline. Missing baselines trigger a reliable full
snapshot. Compressed snapshots are reconstructed and verified before the game
loop applies them; measured payload sizes are recorded below.

The client keeps a bounded command history, predicts local movement using the
shared movement code, restores the server player state, discards finalized
commands (including expired ones), and replays only valid pending commands.
Smooth small visual corrections and snap large corrections, teleports and
respawns. Bound extrapolation during prolonged silence, then show disconnection.
Interpolate remote entities from snapshots. Dynamic doors, lifts and player
collisions require replicated collision state and can still cause corrections.

Predict local weapon presentation where practical, but server results confirm
hits and pickups. Stable event IDs deduplicate sounds/effects during replay.
Cosmetic effects must not consume authoritative gameplay RNG.

## Transport and protocol

Define a new versioned game protocol; do not reinterpret GDSF v2 stream records
as datagrams. Separate codec, session rules, simulation, and transport adapters.

| Traffic | Delivery |
| --- | --- |
| Player input | Datagram with recent redundant commands and bounded history |
| World snapshots | Sequenced datagram; newer usable state supersedes older state |
| Setup, rules, map changes, required events | Reliable bounded control channel |
| Join/resync baseline | Reliable bounded bulk transfer, isolated from live input |

Use authenticated QUIC over UDP for native gameplay, with WebTransport over
HTTP/3 providing the same datagrams and reliable streams to browsers. Reuse
quic-go/webtransport-go for TLS, congestion control, replay protection and
connection establishment. TCP/TLS remains a native stream option. WSS is the compatibility
path for both gameplay and control and also supports initial bring-up. Browser
WebTransport capability and deployment must be verified, with WSS fallback.
WebRTC/mesh peer connections are not part of this architecture.

Datagram sessions need authenticated connection binding, replay protection,
size bounds, rate/congestion limits and no unauthenticated amplification.
Use established security mechanisms rather than inventing cryptography.
All decoders reject unsupported versions, invalid lengths and invalid values.
WSS uses binary messages and bounded queues. Multiple logical channels on one
WebSocket do not remove TCP head-of-line blocking; bulk transfer needs a separate
connection or scheduling that prevents unbounded gameplay delay.

Current wire limits: GDMP version 3; 8 inputs per batch; 8 MiB maximum decoded
snapshot; 1,100 bytes maximum game datagram. Input packets and small compressed
updates use datagrams on WebTransport. Independent baselines, larger updates,
discovery, map changes, chat and follow controls use reliable streams. The
datagram receive queue holds at most 32 messages. This is authenticated QUIC,
not a separate raw-UDP socket protocol.

Discovery uses a short-lived Query/ServerInfo exchange without reserving a slot.
Its manifest is bounded to 8 KiB and 64 ordered WAD hashes. The client compares
its own content and engine hashes before joining; discovery never supplies local
filesystem paths. Map transitions validate the next map's compatibility key.

## Sessions, spectators and chat

The dedicated server grants players a 30-second reconnect grace period. Their
bodies remain in the world, receive neutral controls and can still take damage.
A valid 256-bit resume token preserves the body, inventory and score while
replacing the connection identity, input history and snapshot acknowledgments.
Tokens rotate; the previous token remains valid only until the new connection
confirms activity, allowing recovery when a Welcome is lost. Grace expires even
when every player disconnects. Explicit Leave releases the body without grace;
the client gives that reliable message up to 500 ms before closing. A server
process restart does not preserve these in-memory sessions.
Five seconds without any server record triggers reconnect, even if client writes
continue to succeed on a blackholed connection. Pongs count as server activity,
so an observer waiting in an empty lobby remains connected.

Up to 16 spectators join separately from the four gameplay slots. Spectators
send acknowledgments and follow controls, never movement or weapon commands.
They initially follow the lowest present player; F12 cycles the camera. If the
selected player leaves, the camera follows another present player. An empty
server keeps spectators waiting without advancing an empty world. Spectators
currently rejoin as new observers after loss; they receive no player resume token.

The existing T chat binding works for players and spectators. The server derives
the sender name and slot from the admitted connection. Text is limited to 160
UTF-8 runes, with a four-message burst and a two-message-per-second refill. Chat
has monotonically increasing event IDs across maps, bounded reliable queues and
client deduplication. Menus keep receiving chat and snapshots while gameplay
controls are neutral; opening chat never pauses the match.
Hold F6 to view player names, ping, frags and deaths. **Multiplayer → Players**
opens a scrollable list including spectators and reconnecting players. Join,
leave, connection-loss and reconnect notices appear in an on-screen feed. The
gameplay connection indicator shows your ping and player count, with a warning
when confirmed server updates stall.

The server publishes a small binary roster control message on membership
changes and at most once per second for latency updates. Ping uses nonce-bound
round trips measured by the server; probes do not keep an inactive player alive
or affect simulation timing. Roster identities survive reconnects and map changes;
no addresses or reconnect credentials are included. Client and server must both
use GDMP version 3; there is no older-protocol fallback.

The desktop and WASM title/pause menus place **Multiplayer** directly below
**New Game**. The multiplayer home separates **Find Game**, **Create Game**,
**Player Setup** and **Servers**. Find Game shows the live room list with
only refresh and back actions. Player Setup contains name and player/spectator
preference. Servers opens the saved server list. Without a configured
lobby, the home offers **Saved Servers** and Player Setup.
Select a room or server and press Enter or tap the touch **Use** button to join.
Details show map, mode, occupancy and whether the loaded game files match.
Servers selects the configured default server, so joining does not
require entering an address or choosing a transport.
The built-in list includes **GD-DOOM Co-op** at
`https://m45sci.xyz:6672/netplay` and **GD-DOOM Deathmatch** at
`wss://m45sci.xyz:6672/deathmatch`. A configured custom address remains preferred.
The hosted deathmatch room has four slots, no monsters, a 20-frag/10-minute
limit, and E1M1/E1M2 rotation. Map exits can rotate early. Press Use after the
one-second respawn delay; scores and loadouts reset on each map.
Co-op uses WebTransport with WSS fallback. The hosted deathmatch endpoint uses
WSS through the same public port, forwarded to an isolated loopback match
process; gameplay RNG is never shared between the two matches.

**Player Setup → Join As** selects spectator mode; the game list then offers
**Watch**. **Manage Servers** under Servers contains custom addresses.
**Add Server** and **Edit Selected Server**
accept an HTTPS/WebTransport or WS/WSS URL, or a native `host:port` TCP address.
This page also shows the selected address and response time. Entries persist
in native configuration's `multiplayer_servers` field, or the current browser
origin's local storage. The list contains at most 32 configured/saved addresses;
there is no public master registry or automatic network scanning. Background
status queries reserve no gameplay slots, and older servers can omit counts.

Joining has a progress screen with Escape/Back to cancel; errors remain on the
server list for retry. The loaded WAD stack is retained and verified. While
connected, the menu defaults to **Return to Game**, which closes the menu and
resumes the current match. **Leave Match** releases the player and returns to the
title menu without exiting the program or reloading the browser. Escape returns
to the parent menu. Leaving restores local rules and keeps current audio,
rendering and input preferences.
The native `-multiplayer-server` option selects the default list entry;
`-connect` joins immediately. `MULTIPLAYER_SERVER` supplies the default in-game
server address when running `scripts/build_wasm.sh`. Browser builds always
enter game setup before joining through the in-game Multiplayer menu.

## Multi-room lobby and custom WADs

`cmd/gdlobby` is the HTTP control plane and WebSocket gateway for user-created
games. It launches one `gdserver` child process for each room, with private
loopback listeners. Each process owns its world, RNG, clocks, players and rules.
A room appears as ready only after the supervisor queries the running worker
and verifies its actual content/rules manifest. Room creation has a stable
request ID: a retry after a lost HTTP response does not start a second match.

The default capacity is eight rooms (maximum 32). Empty rooms expire after ten
minutes; reconnect reservations and spectators keep a room occupied. Failed
children are reaped, and stopping the lobby stops its children and upgraded
WebSocket connections. Startup, logs, creation rate and retained room records
are bounded. Rooms use WSS through `/rooms/<id>/netplay` on the public gateway;
the existing direct servers can still use WebTransport or native TCP.

Create an operator-owned catalog, with paths relative to the JSON file:

```json
{
  "packs": [
    {"id": "doom-shareware", "name": "DOOM Shareware", "wads": ["DOOM1.WAD"]},
    {"id": "custom-mapset", "name": "Custom mapset", "wads": ["DOOM2.WAD", "maps.wad"]}
  ],
  "redistribution": [
    {
      "sha256": "1d7d43be501e67d927e415e0b8f3e29c3bf33075e859721816f652a526cac771",
      "name": "DOOM1.WAD",
      "allow": true,
      "license": "Original shareware redistribution terms",
      "source": "Repository DOOM1.WAD"
    }
  ]
}
```

Only list files actually installed on the host. Overlay order matters. The
lobby hashes the files and validates available maps before advertising them.
The example approval identifies the repository's exact `DOOM1.WAD`; verify
your own file's digest and redistribution terms before adding an approval.
`redistribution` is optional. Each entry is an explicit operator permission for
one SHA-256, with a display filename and optional license/source information.
The host never infers permission from a filename, shareware label, uploader
claim, or whether a WAD is described as noncommercial. Unknown files and
`allow: false` entries stay private, including files in custom uploaded stacks.
An uploader cannot opt a file into public sharing.

An approval applies to that exact file wherever it appears: in an installed
pack, a later upload, or a previously stored upload after restart. Add the
verified overlay hash to approve a mapset while keeping its commercial base
private. Restart the lobby after changing policy; removing an approval stops
new origin downloads but cannot retract copies already downloaded or cached.
For a local browser preview, build the two executables and run:

```bash
CGO_ENABLED=0 go build -o /tmp/gdserver ./cmd/gdserver
CGO_ENABLED=0 go build -o /tmp/gdlobby ./cmd/gdlobby
/tmp/gdlobby -listen 127.0.0.1:6670 -public-url http://127.0.0.1:6670 \
  -worker /tmp/gdserver -catalog /path/to/catalog.json \
  -web-origins http://127.0.0.1:8080 -upload-dir /path/to/lobby-uploads
MULTIPLAYER_LOBBY=http://127.0.0.1:6670 ./scripts/build_wasm.sh
go run ./cmd/wasmserve
```

For public hosting, use an HTTPS public URL and TLS termination that forwards
WebSocket upgrades and the HTTP API to the lobby. Allow the frontend's exact
origin with `-web-origins`. Configure the proxy's upload body limit and timeout
to accommodate WAD uploads. Worker ports stay private. Browser builds embed the
lobby URL from `MULTIPLAYER_LOBBY`; native clients accept
`-multiplayer-lobby=<URL>`. **Servers** retains the saved server browser.
The room list is transient and is not added to saved favorites.

**Create Game** selects name, mode and difficulty. Its **Game Files** page
selects the WAD and starting level; **Rules** contains player count, monster
behavior, friendly fire, frag and time limits. Each page returns to its parent
with Back or Esc, preserving the current choices.
With `-upload-dir` configured, **Game Files → Upload Loaded WADs** uploads the entire ordered
stack currently loaded by the client, including browser-local files selected
in the launcher. **Load WAD Files** lets browser players choose their base IWAD,
enable custom PWADs and reorder overlays before starting. Uploads are disabled
when this flag is omitted. The upload
API accepts at most 16 files, 64 MiB per file and 128 MiB per stack, with a
default storage quota of 512 MiB. The server checks lengths, SHA-256 identities,
WAD structure and playable maps before publishing immutable content. Repeating
an identical stack reuses its pack; original filenames never choose host paths.
Uploaded maps also have geometry and expanded allocation limits, including
8,192 sectors and 65,536 BLOCKMAP cells per map. This bounds validation before
the ordinary loader can synthesize a large REJECT table or expand repeated
BLOCKMAP lists. Exceptionally large custom maps can be rejected even when their
WAD file fits the byte limit. Only one lobby may own an upload directory.

On join, the client assembles the room's exact ordered WAD stack. It reuses
matching files already loaded locally and downloads only missing files marked
as approved by the host. A player with a locally loaded commercial base can
therefore join a room using an approved custom overlay without downloading the
base. Missing private files still require local loading; direct servers without
lobby content metadata also require the matching local stack.

Each download comes from the lobby's fixed same-origin
`/api/v1/content/<sha256>` endpoint. The client verifies the declared byte count
and SHA-256 before rebuilding the map, textures, sprites, palettes and audio,
then the gameplay handshake verifies the resulting content manifest again.
Automatic preparation accepts at most 16 files, 64 MiB per file and 128 MiB for
the complete target stack. Downloads use a two-minute timeout and do not follow
redirects to another source. There is no persistent client download cache:
prepared content remains in memory, and leaving a match keeps that WAD stack
loaded for local play and later joins.

The supervisor copies approved files into a private, immutable download cache
and verifies their lengths and hashes before exposing an endpoint. Workers use
the same copied bytes, so editing an original file cannot change an active
download or approved worker source. The cache defaults to 512 MiB, controlled
by `-download-quota` in bytes; repeated hashes share one copy. Four transfers
can run concurrently, each with a two-minute deadline, and shutdown interrupts
stalled readers and removes the cache. Even a correctly guessed private hash
has no download route. Browser requests use the same origin allowlist as the
lobby API.

Uploaded packs persist in `-upload-dir`, independently of the temporary download
cache. There is no automatic uploaded-content eviction in this version, so a
full upload quota returns an error. Room processes still expire independently
of stored content.

Control API: `GET /api/v1/lobby` returns packs, rooms and capacity;
`POST /api/v1/rooms` creates a room and waits for verified readiness;
`POST /api/v1/packs` accepts an ordered multipart upload and returns its pack;
`GET` or `HEAD /api/v1/content/<sha256>` serves only approved immutable files.
Pack metadata includes ordered filenames, sizes, SHA-256 hashes and download
permissions, plus operator-supplied license/source information when approved.
Uploading content and creating a room are separate requests from gameplay, so
an upload does not occupy a player's game transport.

## Multiplayer controls and movement timing

Mouse look turns the player left and right with the same sensitivity and
inversion settings as local play; vertical pitch is not implemented. In source
port mode, Backslash toggles mouse look. Browser players click the game to
capture the pointer. Relative motion accumulates between the 35 Hz commands,
is predicted locally once, and is reconciled to the authoritative result.
Menus, chat and reconnecting clear pending motion and send neutral controls or
no gameplay input as appropriate. Regression tests cover startup suppression,
repeated host samples, catch-up commands, sensitivity, inversion and correction
without duplicate turning.

Multiplayer input and snapshots are pumped on every 140 Hz host update; the
classic 35 Hz session/menu gate must not gate the network movement clock again.
The server remains fixed at 35 Hz. The client's command clock adjusts by at most
5% to recover latency lead, with contiguous input tics during ordinary play.
It preserves fractional interpolation phase when the rate changes, and delayed
snapshots cannot rewind its server-clock estimate. A new baseline can re-anchor
the schedule after a stall overtakes prediction.

Client rendering keeps its movement clock and interpolation endpoints across
matching snapshots. Receiving a baseline does not restart camera interpolation;
host updates retain the elapsed fraction of the current tic. Small position,
eye-height and yaw corrections use a presentation-only offset that fades over
100 ms, while new input and collision take effect immediately. Teleports,
respawns, death transitions and large corrections reset that offset. Regression
tests exercise the outer host gate as well as a moving server with repeated
latency changes, uneven snapshot delivery and the server's held-input policy.

Remote players, monsters, projectiles and spectator camera motion interpolate
raw snapshots on a monotonic server-tic timeline rather than starting another
blend whenever a packet arrives. The buffer normally adds three tics (about
86 ms) of visual delay, retains at most eight frames, and bounds recovery delay
to seven tics. A stalled stream holds the newest known pose without
extrapolating. Player incarnation/teleport changes cut immediately and clear
affected pose history; confirmed gameplay and collision always use the newest
server state. Deterministic tests exercise 120 Hz rendering with uneven snapshot
delivery, corrections, stalls and discontinuities.

Doors, lifts and other sector planes share that snapshot timeline. Their
rendered heights retain fractional map units across wall edges, floor/ceiling
planes and visibility checks; they never extrapolate from the local input
clock. A grounded lift rider's camera follows the displayed support floor,
without applying a second reconciliation blend to the same floor displacement.
Entering or leaving a moving support uses a bounded 100 ms camera transition,
including spectator views, without restarting other correction smoothing.
Collision and input replay still use the newest confirmed heights. Tests cover
opening, stopping, reversing, packet loss, burst delivery, stalled streams and
riding ascending/descending floors.

Menu verification: real browser joining, movement, leaving to title, and
rejoining as a spectator passed without a page reload. The public WASM build
also passed the WAD-picker-to-Multiplayer-menu check. All 36 repository test
packages passed, including new join/leave/cancel/retry tests; focused app and
menu race checks passed. See [deployment status](wasm-deployment.md) for the
hosted server's remaining firewall requirement.

### Browser audio and snapshot work

Each host update reconciles at most one snapshot. Network delivery still
validates and retains transport baselines before coalescing to the newest one.
Static map validation reuses the hash of the owned, immutable restart template;
replacing the template invalidates it, and mutable fallback maps always rehash.
Replicated browser sound bursts use the local sound coalescing and priority
budgets while preserving the selected event's pitch and deduplication cursor.

Music refills run before session work and yield between synth chunks after a
four-millisecond work budget. The prepared reserve grows after late refills and
recovers gradually. An empty active music reader yields without adding silence:
it must not wait inside Oto's serial source-refill loop and stall other sounds.
The shared browser AudioWorklet separately adapts its output reserve from about
60 ms up to 200 ms. This protects both music and effects at the cost of extra
sound latency under load. See `third_party/oto/GDDOOM.md` for the pinned patch
and deterministic worklet tests.

Frequent world snapshots now use a generated, typed binary codec. Integer
fields use checked zigzag/unsigned varints, repeated texture names share a bounded
string table, and maps use sorted keys. A binary version plus schema fingerprint
prevents incompatible layouts from being read.
The outer BLAKE3 checksum, loaded-map/WAD checks, full world validation and
acknowledged-baseline recovery remain in place. Decoding bounds lengths and
allocations before constructing arrays/maps, and rejects malformed fields or
trailing data. Save files retain their existing format.

A broadcast captures world arrays and the player roster once, then applies each
viewer's player state and sound audience. Spectators sharing a viewer reuse the
same encoded state; input acknowledgments and compression remain per connection.
Applying a correction reuses live world array storage while copying the decoded
snapshot, so retained baselines and interpolation endpoints remain independent.

Gameplay packet framing now writes and reads fixed layouts directly. Compression
tries the acknowledged dictionary first and skips a second full compression pass
when the delta is at most 2 KiB and at most 1/32 of the raw state. In that case
it can trade at most 2 KiB for avoiding duplicate work. Compression output
capacity follows recent sizes, cached dictionaries are invalidated on eviction or
epoch changes, and equal-sized evicted history buffers can be reused. Packet
ownership, eight-baseline/16-MiB history bounds and recovery semantics are retained.

This changes the simulation compatibility identifier to
`gd-doom-authority-dev-2`: deploy the server and browser client together. Older
clients/servers are rejected during the compatibility handshake.

`BenchmarkAuthorityReplicaRenderCaches` separates binary/JSON encoding and
decoding, validation, application and lazy automap rebuilding. `ValidateRehash`
compares the former repeated-map-hash cost, while
`BenchmarkAuthoritySnapshotBroadcast` compares four separate captures with a
shared capture. The JSON cases remain test-only references for the former format.
Regenerate the typed codec with:

```sh
AUTHORITY_BINARY_GENERATE=1 go test ./internal/doomruntime \
  -run '^TestGenerateAuthorityBinaryCodec$'
```

Use Xvfb on headless Linux. A schema drift test catches state fields added without
regeneration. Changes to the schema or binary grammar also require a simulation
compatibility version bump.

Measured after this pass (Go 1.26.6, Node 22 WASM CPU harness with only Ebitengine
window initialization skipped; these are CPU measurements, not rendered FPS):

| Snapshot operation | E1M1 | E1M3 |
| --- | ---: | ---: |
| Strict JSON decode reference | 7.07 ms | 15.58 ms |
| Binary decode with checksum | 0.47 ms | 0.99 ms |
| JSON / binary uncompressed bytes | 126,631 / 19,325 | 285,633 / 48,074 |
| JSON / binary decode allocations | 586 / 112 KB | 1,667 / 236 KB |
| Cached validation | 0.005 ms, no allocations | 0.010 ms, no allocations |
| Apply with reused world buffers | 0.076 ms, 9 KB | 0.156 ms, 18 KB |

The native four-viewer broadcast benchmark reduced allocated bytes from
582 to 265 KB on E1M1 and 1,267 to 583 KB on E1M3 (about 54%), with 12–14%
less CPU time compared with four separate captures using the same binary codec.
Transport stream benchmarks with changing acknowledgments, compression, framing,
decoding and history commits used 32–35% less WASM CPU and 36–38% less native CPU
on structured 128/512-KiB fixtures. Results depend on map size and activity.

Verification includes the complete native suite, executed WASM authority/audio
tests, the actual WASM WebSocket bridge, network race checks, malformed/ownership/
epoch recovery tests, and a 150,494-case binary decoder fuzz run. The production
WASM build uses the normal Ebitengine dependency; the CPU benchmark harness is
not included in the shipped game.

## Rules

Co-op: independent health/ammo/weapons, cooperative map completion, per-player
respawn, explicit shared-key and pickup rules, configurable friendly fire.
Collected keys are shared and survive individual respawn. Living players carry
health, armor, ammunition and weapons to the next map; keys and temporary powers
reset for that map. A dead player starts the next map with a fresh loadout.
Deathmatch: map deathmatch starts, player-to-player hitscan/projectile damage,
suicide/frag scoring, respawn, frag/time limits and rotation. Original map pickups
return after 30 seconds, including weapons; there is no weapons-stay mode.
Invulnerability, invisibility, dropped items and runtime-spawned items do not
respawn. Co-op retains consumed world pickups. With co-op friendly fire disabled,
a teleporter occupied by a living teammate blocks the teleport. Deathmatch
telefrags bypass invulnerability and credit the teleporting player.
The server announces these rules in the compatibility handshake. Late joins and
disconnects are committed at server tic boundaries. A client menu never pauses
the match. A lost server ends the session cleanly; player host migration is not
required because the dedicated server is the authority.

## Implementation checklist

- [x] Record design and release acceptance criteria.
- [x] Separate player stepping from world stepping; baseline runtime regression suite passed.
- [x] Implement canonical per-player state, targeting, collision and damage ownership.
- [x] Establish a display/audio-free simulation and server executable.
- [x] Implement bounded input scheduling, missing-input policy and acknowledgments.
- [x] Implement versioned messages and strict codec bounds.
- [x] Implement validated client baselines, player incarnations and acknowledged snapshot compression.
- [x] Add the desktop connection path and verify TCP clients against a real map.
- [x] Implement and unit-test prediction, reconciliation, interpolation and event deduplication.
- [x] Implement co-op/deathmatch rules and epoch transitions; full play-through acceptance remains below.
- [x] Implement WS/WSS cross-play; TCP/WS integration and actual browser WS movement/firing verified.
- [x] Implement authenticated QUIC/WebTransport and HTTPS-to-WSS fallback; real browser QUIC and native fallback verified.
- [x] Implement late join, reconnect grace, explicit leave, timeout, spectator cameras and status UI.
- [x] Connect the existing chat UI to bounded server-owned identity, rate limits and reliable delivery.
- [x] Run fault-injection, integration, race, regression and the local browser checks recorded below.
- [x] Document supported launch/build commands and measured transport limits.

## Acceptance evidence

Completion requires all of the following, not merely package-level codec tests:

1. A server starts with DISPLAY unset and no audio device and loads a real WAD.
2. Two independently controlled clients complete a co-op map and a deathmatch.
   Health, ammo, monsters, damage, pickups, respawns and scores remain correct.
3. Desktop/browser clients share a session with content mismatch rejection.
4. One client loses traffic briefly while other players and the world continue.
   Recovery yields correct server state without duplicated shots or extra motion.
5. Tests exercise loss, jitter, reorder, duplication, stale epochs, future-input
   flooding, disconnect, late join, missing delta baseline and slow receivers.
6. Prediction converges after collision/correction; teleports/respawns clear old
   history; replay does not duplicate effects or mutate world RNG.
7. Native UDP and browser datagrams are verified against the same authority;
   WSS fallback works when datagrams are unavailable.
8. Server simulation never waits on network writes, memory is bounded, and
   concurrency checks pass. Existing single-player/demo and broadcast checks pass.

## Progress and known gaps

2026-10-09 implementation evidence:

* A real E1M1 test runs two TCP clients while one stops sending input, then resumes.
  The world continues; acknowledgments finalize missing tics without inventing
  received sequence numbers.
* The server command test runs a TCP player and WebSocket player on E1M1, ends a
  timed deathmatch, and retains both connections on E1M2 with a new epoch.
* Two actual browser/WASM co-op clients moved and fired independently on E1M1,
  rendered remote player sprites, and retained 100 health with friendly fire
  disabled. Chat reached a spectator, F12 changed its camera, and automap
  exploration updated. Two deathmatch players remained connected during live
  E1M1/E1M2 rotation. Explicit quit displayed the session-ended page.
  Transport-level WASM tests also execute the browser WebSocket API bridge.
* Two production clients, framed messages, the real input scheduler and canonical
  snapshots complete E1M2 co-op through its real exit switch. Scheduled movement
  collects actual ammo, health and the shared red key. Both clients retain health
  and ammunition on real E1M3; map-specific keys reset. A separate E1M1 test fires
  the actual pistol, confirms the first frag on both clients, independently
  respawns the victim using its own input, fires the second kill to reach the
  frag limit, then rotates to real E1M2 with fresh scores/loadouts. These focused
  fixtures place players beside pickups/switches or in a clear firing lane and
  set low target health; they do not stand in for walking a full campaign.
* Thirty-six moving/firing two-player E1M1 binary snapshots measured about
  19,896 bytes raw each, 5,278 bytes for the first compressed full state and
  295 bytes on average for subsequent updates. At 17.5 snapshots/second that
  is roughly 5.0 KiB/s per viewer, excluding transport overhead. The earlier
  JSON implementation measured about 10.5 KiB/s in this fixture. This is one
  measured scenario, not a bound for all WADs or combat loads.
* Compression uses zstd with an acknowledged reconstructed snapshot as a raw
  dictionary. Each peer retains at most eight baselines/16 MiB. Exact decoded
  length, BLAKE3 digest, zstd checksum and allocation/window bounds are checked.
  Independent compressed full states repair missing or evicted baselines.
* Input timing incorporates a measured, bounded RTT. Client simulation work is
  capped at four tics per update and a 35-tic prediction horizon. A prolonged
  gap stops further prediction instead of granting extra elapsed time.
* Race checks pass for the network package; a snapshot codec fuzz run processed
  329,061 inputs. Display-free runtime tests cover correction, menus, teleports,
  damage attribution, respawn, map changes, sounds and prediction RNG isolation.
* Native WebTransport tests run real QUIC with certificate verification, origin
  rejection, discovery close/drain, input consumption, compressed snapshots,
  final-state delivery and cancellation. These tests pass with the race detector.
  Transport fault tests cover lost baselines, reordering, stale epochs, exact
  reconstruction and bounded datagram queues. A real-browser WebTransport
  fixture also passes HTTP/3 with a TLS certificate pin, discovery, reliable
  initial baseline, input datagram and compressed correction. The browser API
  bridge has separate Node mock coverage.
* Native HTTPS-to-WSS fallback passes against an actual UDP blackhole and a
  fixture certificate trusted through the platform CA path. Discovery falls
  back successfully; joining reuses WSS without another QUIC attempt. A separate
  untrusted-certificate case confirms certificate verification remains enabled.
* Real TCP/WebSocket spectator tests cover an empty lobby, later player joins,
  camera cycling, immediate explicit player leave, camera fallback and map
  transitions. Both transport permutations pass with the race detector.
* Reconnect tests cover preserved bodies, stale connection/input rejection,
  lost-Welcome retry, token retirement, grace expiry with no connected players
  and map changes. Blackholed-connection tests prove five-second silent-read
  detection initiates resume within the grace period; Pong-only liveness passes.
  Chat tests cover identity assignment, UTF-8 bounds, rate
  limits, reliable echo to both clients, deduplication and background UI polling.
* The E1M1 datagram fault test connects the real client, codec, match and prediction
  through a deterministic transport emulator for 160 server tics. It injects 65
  drops, 40 duplicates and 17 reorders, including burst outages. Pending history
  peaks at 23 commands; one attack consumes exactly one bullet; final pose and
  ammunition reconcile exactly. Socket/QUIC authentication is tested separately.
* The final `go test ./... -count=1` run passes, including existing single-player,
  demo, broadcast and voice packages. The network, app and server suites and
  authoritative runtime/prediction tests pass with `-race`. Native display-free
  server and JS/WASM builds pass. Scoped vet
  passes for the network, app and server; runtime-wide vet reports three existing
  unkeyed `WorldBBox` literals in `map_floor_boundaries_test.go`.

Still required before release: long co-op campaigns and deathmatch playtests,
WAN/mobile-network testing, and the full supported browser/native platform matrix. The local checks above
cover specific deterministic and interactive scenarios, not every WAD or network
condition. Deathmatch items currently reappear without a dedicated
respawn fog/sound effect. A client baseline is not a server rollback/save
checkpoint; server-only item timers are intentionally absent. One authority
process hosts one match because the gameplay RNG remains process-global.

## Current launch paths

Experimental local co-op (matching WADs in the same order on every machine):

```bash
go run ./cmd/gdserver -wad DOOM1.WAD -listen 127.0.0.1:6671
go run . -wad DOOM1.WAD -connect 127.0.0.1:6671 -player-name Player
```

The client queries the server's map/rules and checks its own engine/WAD hashes
before joining. Use `-mode deathmatch -frag-limit 20 -rotation E1M1,E1M2` on the
server for rotating deathmatch. Without `-rotation`, deathmatch follows normal
and secret map exits, including games created through the lobby. An explicit
single-map rotation repeats that arena. Reaching the episode finale or exhausting
a custom map pack ends an unrotated session; multiplayer transitions currently
load the next map directly without the original deathmatch frag intermission.
`-time-limit` is in seconds; zero disables it.
The server's `-no-monsters`, `-skill`, and `-friendly-fire` settings are authoritative.

WebSocket testing adds `-web-listen 127.0.0.1:6672` and, when the browser page is
served from another origin, an explicit allowlist such as
`-web-origins http://localhost:8000`. Join with
`-connect ws://127.0.0.1:6672/netplay` on native clients. In the browser, complete
the graphics and audio setup, then add the address under **Multiplayer → Servers
→ Manage Servers**. The browser launcher accepts only `wad` and `file` content
parameters; URLs cannot join a server or set multiplayer options.

Add `-spectate` to a native client command, or select spectator under the
in-game **Player Setup → Join As**, to join as an observer. Use F12 to switch
followed player and T to chat; hold F6 for the roster and score.

For TLS TCP and WSS listeners supply `-tls-cert` and `-tls-key`; native clients
use `tls://host:port`, while browser clients use `wss://host:port/netplay`.
Certificate verification uses the platform trust store. Plain TCP/WS examples
are local testing paths. WSS still has TCP head-of-line blocking: compression
and bounded queues limit backlog but cannot remove that transport behavior.

To serve authenticated datagrams and WSS fallback at the same host/port:

```bash
go run ./cmd/gdserver -wad DOOM1.WAD \
  -listen 0.0.0.0:6671 \
  -udp-listen 0.0.0.0:6672 -web-listen 0.0.0.0:6672 \
  -tls-cert server.crt -tls-key server.key \
  -web-origins https://play.example.com
go run . -wad DOOM1.WAD -connect https://game.example.com:6672/netplay
```

The certificate must be trusted and valid for the server hostname. An `https://`
game URL tries WebTransport first, then the same URL over WSS; a successful
fallback is reused for discovery, joining and reconnect. `wt://` explicitly
selects WebTransport without fallback. Both native and browser adapters share
the same authority, and protocol controls remain reliable on either path.

Build the dedicated server without display/audio dependencies and build the
browser launcher from the repository root:

```bash
CGO_ENABLED=0 go build -o gdserver ./cmd/gdserver
./scripts/build_wasm.sh
go run ./cmd/wasmserve
```

For local browser testing, point the served launcher's `connect` setting at the
WS endpoint above and allow that page's exact origin with `-web-origins`.

## References

* [Valve: latency compensation and shared prediction code](https://developer.valvesoftware.com/w/index.php?title=Latency_Compensating_Methods_in_Client%2FServer_In-game_Protocol_Design_and_Optimization)
* [WebTransport streams and datagrams](https://developer.chrome.com/docs/capabilities/web-apis/webtransport)
* [WebSocket protocol](https://www.rfc-editor.org/info/rfc6455/)

* [quic-go WebTransport server and origin validation](https://quic-go.net/docs/webtransport/server/)
* [WebTransport Go releases and supported protocol revisions](https://github.com/quic-go/webtransport-go/releases)
