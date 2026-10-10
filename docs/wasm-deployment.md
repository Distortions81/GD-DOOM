# WASM, co-op and deathmatch deployment

## Current deployment

The public browser site is <https://m45sci.xyz/u/dist/GD-DOOM/>. Nginx maps it to
`/home/dist/www/public_html/GD-DOOM` on `m45sci.xyz`. Use
`ssh -l dist m45sci.xyz`; the existing SSH configuration supplies port 5313 and
the key. No private-key contents need to be copied or printed.

The current release ID, source commit, and asset hashes are recorded in the
site's `wasm-manifest.json`. Build and deploy the browser and server from the
same clean commit. Browser launches start AUTO detail at full resolution.
Esc → Multiplayer opens the live lobby, with **Create Game**, custom WAD
uploads, and automatic loading of approved downloadable content. **Direct
Servers** retains **GD-DOOM Co-op**, **GD-DOOM Deathmatch**, and saved custom
servers. Select a room and press Enter to join; the connected menu offers
Return to Game and Leave Match. The client runs a continuous input clock,
accounts for command lead, and smooths small prediction corrections.

The lobby base URL is `https://m45sci.xyz:6672`. Its user service is
`gd-doom-lobby.service`, listening only on `127.0.0.1:6675`. The existing co-op
HTTPS listener forwards `/api/v1/` and `/rooms/` to it; `/netplay`, `/deathmatch`
and the co-op WebTransport listener retain their existing endpoints. This
requires no additional public ports or nginx changes.

The default authoritative co-op server was installed on 2026-10-10 UTC:

| Item | Value |
| --- | --- |
| Intended public endpoint | `https://m45sci.xyz:6672/netplay` |
| WSS endpoint | `wss://m45sci.xyz:6672/netplay` |
| User service | `gd-doom-coop.service` |
| Binary | `/home/dist/.local/bin/gdserver` |
| Working directory | `/home/dist/.local/share/gd-doom` |
| Deployment protocol probe | `/home/dist/.local/share/gd-doom/gdserver-deployment-probe` |
| Unit | `/home/dist/.config/systemd/user/gd-doom-coop.service` |
| Mode | Four-player co-op, E1M1, skill 3, monsters enabled, friendly fire disabled |
| Native TLS listener | `127.0.0.1:6671`, not exposed publicly |
| Web listeners | TCP 6672 for HTTPS/WSS; UDP 6672 for WebTransport |
| Allowed browser origin | `https://m45sci.xyz` |

Both services are enabled and running, and the `dist` user manager has lingering
enabled. A clean campaign completion restarts the service with a fresh E1M1.
An empty match does not advance the world.

Host-local probes passed real discovery, WAD/rules validation, joining, snapshot
decoding and explicit leaving over TLS, WSS and WebTransport. External WSS and
WebTransport probes also passed on 2026-10-10 UTC. The initial deployment's UFW
block on port 6672 is no longer preventing these connections. The deployment
account does not have permission to manage the firewall or nginx routes.

If firewall rules are recreated, an administrator can permit just the required
service ports:

```bash
sudo ufw allow 6672/tcp comment 'GD-DOOM WSS'
sudo ufw allow 6672/udp comment 'GD-DOOM WebTransport'
sudo ufw status numbered
```

If an upstream firewall also filters the host, permit the same two ports there.
No change to the existing SSH, HTTPS or other application rules is needed.

## Build and update the backend

Build both `./cmd/gdserver` and `./cmd/gdlobby` from the browser's clean commit.
The first lobby installation and gateway unit are described below. For later
updates, stage and hash-check both binaries, back up the current binaries and
units, atomically replace the binaries, then restart the lobby and direct
servers. Restarting the lobby ends its dynamic rooms; the configured uploaded
WAD storage persists. Publish the browser assets only after backend probes pass.

From the repository root, build the same working tree as the browser client:

```bash
CGO_ENABLED=0 GOCACHE=/tmp/gd-doom-go-cache \
  go build -trimpath -ldflags='-s -w' -o /tmp/gdserver-m45sci ./cmd/gdserver
sha256sum /tmp/gdserver-m45sci DOOM1.WAD

rsync -av --chmod=F755 /tmp/gdserver-m45sci \
  dist@m45sci.xyz:/home/dist/.local/bin/gdserver.next
rsync -av DOOM1.WAD dist@m45sci.xyz:/home/dist/.local/share/gd-doom/
ssh -l dist m45sci.xyz 'set -e
  cp /home/dist/.local/bin/gdserver /home/dist/.local/bin/gdserver.previous
  mv /home/dist/.local/bin/gdserver.next /home/dist/.local/bin/gdserver
  systemctl --user restart gd-doom-deathmatch.service gd-doom-coop.service
  systemctl --user status gd-doom-coop.service gd-doom-deathmatch.service --no-pager'
```

The shareware WAD SHA-256 is
`1d7d43be501e67d927e415e0b8f3e29c3bf33075e859721816f652a526cac771`.
Back up the existing binary and service units before updating either service.

The installed service unit is:

```ini
[Unit]
Description=GD-DOOM default authoritative co-op server
After=network-online.target

[Service]
Type=simple
WorkingDirectory=%h/.local/share/gd-doom
ExecStart=%h/.local/bin/gdserver -wad DOOM1.WAD -map E1M1 -mode coop -skill 3 -players 4 -listen 127.0.0.1:6671 -web-listen :6672 -udp-listen :6672 -tls-cert /etc/letsencrypt/live/m45sci.xyz-0003/fullchain.pem -tls-key /etc/letsencrypt/live/m45sci.xyz-0003/privkey.pem -web-origins https://m45sci.xyz -web-proxy /deathmatch=http://127.0.0.1:6674/netplay -web-proxy-prefix /api/v1/=http://127.0.0.1:6675/api/v1/ -web-proxy-prefix /rooms/=http://127.0.0.1:6675/rooms/
Restart=always
RestartSec=3
TimeoutStopSec=10
NoNewPrivileges=true
PrivateTmp=true
UMask=0077

[Install]
WantedBy=default.target
```

The separate deathmatch unit is
`/home/dist/.config/systemd/user/gd-doom-deathmatch.service`:

```ini
[Unit]
Description=GD-DOOM default authoritative deathmatch server
After=network-online.target

[Service]
Type=simple
WorkingDirectory=%h/.local/share/gd-doom
ExecStart=%h/.local/bin/gdserver -wad DOOM1.WAD -map E1M1 -mode deathmatch -skill 3 -players 4 -no-monsters -frag-limit 20 -time-limit 600 -rotation E1M1,E1M2 -listen 127.0.0.1:6673 -web-listen 127.0.0.1:6674 -web-origins https://m45sci.xyz
Restart=always
RestartSec=3
TimeoutStopSec=10
NoNewPrivileges=true
PrivateTmp=true
UMask=0077

[Install]
WantedBy=default.target
```

Run `systemctl --user daemon-reload` after changing units and
`systemctl --user enable --now gd-doom-deathmatch.service` for first install.
The public deathmatch address is `wss://m45sci.xyz:6672/deathmatch`. The co-op
process terminates TLS and forwards this exact WebSocket route to the isolated
loopback deathmatch process. The upstream retains the browser Origin check.
Both loopback deathmatch listeners use plaintext only on the host; no additional
public ports or certificate copies are required. Co-op retains WebTransport and
WSS; the deathmatch route currently provides WSS only. Restarting co-op briefly
interrupts both public routes; deathmatch retains its process and reconnect grace.

The `-web-proxy` option is repeatable, requires `-web-listen`, and accepts only
literal loopback HTTP backends. It does not add another Authority to the co-op
process, because the Doom RNG and compatibility state are process-global.

`-web-proxy-prefix` also forwards ordinary HTTP and dynamic WebSocket paths
below an operator-selected prefix. It preserves the suffix, browser Origin and
subprotocol, rejects ambiguous paths, and allows enough upstream response time
for bounded WAD uploads and room startup. Its destination is fixed to literal
loopback HTTP. It replaces forwarded client identity from the actual socket;
the lobby trusts that identity only from its explicitly configured proxy IP.

Deathmatch starts on E1M1 with four slots, no monsters, 20 frags or 10 minutes,
and E1M1/E1M2 rotation. Map exits can rotate early. Empty matches pause the timer;
a single player starts it. Press Use after the one-second death delay to respawn.
Scores and loadouts reset at each map change.

The existing Let's Encrypt certificate covers `m45sci.xyz`. At deployment it
expires on November 19, 2026. The service references the existing certificate
and key in place, without copying them. It loads them at startup, so restart it
after certificate renewal. Service management and logs:

```bash
ssh -l dist m45sci.xyz 'systemctl --user status gd-doom-coop.service --no-pager'
ssh -l dist m45sci.xyz 'journalctl --user -u gd-doom-coop.service -n 50 --no-pager'
ssh -l dist m45sci.xyz 'systemctl --user restart gd-doom-coop.service'
ssh -l dist m45sci.xyz '/home/dist/.local/share/gd-doom/gdserver-deployment-probe -transport tls -address 127.0.0.1:6671'
```

For each public room, the deployment probe uses certificate verification, queries live slot counts
and the real manifest, joins a temporary player, decodes a snapshot and sends
explicit Leave. It also
accepts `-transport wss -address wss://m45sci.xyz:6672/netplay` or
`-transport wt -address https://m45sci.xyz:6672/netplay`. Run these from a different
machine to prove public reachability. For deathmatch use the WSS `/deathmatch`
route and expect its deathmatch manifest; successful host-local tests do not prove
the firewall permits internet clients.

## Install the lobby

Install `gdlobby` at `/home/dist/.local/bin/gdlobby`. Create
`/home/dist/.local/share/gd-doom/lobby` and its `uploads` subdirectory with mode
0700. Place this catalog at `lobby/catalog.json`:

```json
{
  "packs": [
    {"id": "doom-shareware", "name": "DOOM Shareware", "wads": ["../DOOM1.WAD"]}
  ],
  "redistribution": [
    {
      "sha256": "1d7d43be501e67d927e415e0b8f3e29c3bf33075e859721816f652a526cac771",
      "name": "DOOM1.WAD",
      "allow": true,
      "license": "Original shareware redistribution terms",
      "source": "Bundled GD-DOOM shareware data"
    }
  ]
}
```

Only this already bundled shareware file is initially approved for download.
Custom uploads stay private unless the operator adds an exact hash approval
and restarts the lobby. Commercial WADs are not installed or approved by this
deployment. See [content policy and limits](authoritative-multiplayer.md#multi-room-lobby-and-custom-wads)
before adding other files.

Install `/home/dist/.config/systemd/user/gd-doom-lobby.service`:

```ini
[Unit]
Description=GD-DOOM multiplayer lobby and room supervisor
After=network-online.target

[Service]
Type=simple
WorkingDirectory=%h/.local/share/gd-doom
ExecStart=%h/.local/bin/gdlobby -listen 127.0.0.1:6675 -public-url https://m45sci.xyz:6672 -worker %h/.local/bin/gdserver -catalog %h/.local/share/gd-doom/lobby/catalog.json -upload-dir %h/.local/share/gd-doom/lobby/uploads -web-origins https://m45sci.xyz -trusted-proxies 127.0.0.1 -max-rooms 8 -idle-timeout 10m -upload-quota 536870912 -download-quota 536870912
Restart=on-failure
RestartSec=3
TimeoutStopSec=15
KillMode=control-group
NoNewPrivileges=true
PrivateTmp=true
UMask=0077

[Install]
WantedBy=default.target
```

Run `systemctl --user daemon-reload`, enable/start `gd-doom-lobby.service`,
and restart the co-op gateway after installing its prefix routes. Verify the
public `/api/v1/lobby`, content hash route, and an actual created room's WSS
handshake. Test both existing direct servers as well. Browser requests from
`https://m45sci.xyz` must pass the origin check; unapproved origins must fail.

Dynamic rooms expire after ten minutes without players, spectators or reconnect
reservations. Any rooms seeded during deployment follow the same lifecycle and
are not permanent default servers. The direct co-op and deathmatch services
remain available under **Direct Servers** even when the lobby is empty.

## Publish the browser assets

Build from the committed source with a traceable release ID:

```bash
GOCACHE=/tmp/gd-doom-wasm-cache \
BUILD_ID="experiments-$(git rev-parse --short=12 HEAD)" \
MULTIPLAYER_SERVER=https://m45sci.xyz:6672/netplay \
MULTIPLAYER_LOBBY=https://m45sci.xyz:6672 \
  ./scripts/build_wasm.sh /tmp/gd-doom-wasm-release
```

The output includes the shareware WAD and General MIDI SoundFont embedded in
`gddoom.wasm`. Publish these eight assets and an updated `wasm-manifest.json`:
`index.html`, `player.html`, `launch.js`, `build-id.js`, `wasm_exec.js`,
`gddoom.wasm`, `gddoom.wasm.gz`, and `font-notice.txt`. The font notice retains
the credits and separate game-artwork terms for the embedded menu font.
Use a staged upload, verify the hashes, and
publish the manifest last. For direct updates, `rsync --delay-updates` delays
replacement until the transfer has completed.

Never use `rsync --delete` against the existing site directory. It also contains
`SC55-HQ.sf2`, `SGM-HQ.sf2` and historical manifests that are not build outputs.
The existing nginx configuration already sends `application/wasm` and
`Cache-Control: no-cache`.

Preserve the existing manifest's `branch`, `commit`, `build_id` and `files`
fields. Add `schema_version: 1`, `source_dirty` and `built_at` (UTC RFC3339).
`files` maps each of the eight asset names to its SHA-256; it excludes the
manifest itself. `build_id` must equal the value in `build-id.js`.

## Alternative: WSS through the existing HTTPS port

If a dedicated public port cannot be opened, an administrator can add this
exact location inside the existing HTTPS `server` block for `m45sci.xyz` in
`/etc/nginx/sites-enabled/m45sci`:

```nginx
location = /gd-doom/netplay {
    proxy_pass https://127.0.0.1:6672/netplay;
    proxy_http_version 1.1;
    proxy_set_header Upgrade $http_upgrade;
    proxy_set_header Connection "upgrade";
    proxy_set_header Host m45sci.xyz:6672;
    proxy_ssl_server_name on;
    proxy_ssl_name m45sci.xyz;
    proxy_ssl_verify on;
    proxy_ssl_trusted_certificate /etc/ssl/certs/ca-certificates.crt;
    proxy_read_timeout 30s;
    proxy_send_timeout 30s;
    proxy_buffering off;
}
```

After `sudo nginx -t && sudo systemctl reload nginx`, the client endpoint is
`wss://m45sci.xyz/gd-doom/netplay`. This exact route leaves the static site and
other services unchanged. This proxy provides WSS only; WebTransport needs the
direct authenticated UDP listener. The alternative has been prepared but has
not been applied or tested on the host.
