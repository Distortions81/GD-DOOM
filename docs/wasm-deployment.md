# WASM and default co-op deployment

## Current deployment

The public browser site is <https://m45sci.xyz/u/dist/GD-DOOM/>. Nginx maps it to
`/home/dist/www/public_html/GD-DOOM` on `m45sci.xyz`. Use
`ssh -l dist m45sci.xyz`; the existing SSH configuration supplies port 5313 and
the key. No private-key contents need to be copied or printed.

The initial multiplayer browser build `multiplayer-20261010T000307Z` was published and verified in a
browser on 2026-10-10 UTC. Both the launcher and in-game Multiplayer menu prefill
the intended server endpoint below. All seven published asset hashes match the
manifest. The previous browser assets are retained outside the webroot at
`/home/dist/.local/share/gd-doom/wasm-releases/before-multiplayer-20261010T000307Z`.
The current release ID, source commit, and asset hashes are recorded in the
site's `wasm-manifest.json`. New browser launches now start AUTO detail at full
resolution, including launches with a previously saved automatic reduction.
Esc → Multiplayer opens the saved-server browser, with the default co-op server,
custom entries, live capacity/map/WAD checks, and join/leave controls. The client
also retains mouse turning between the authoritative server's input ticks.

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

The service is enabled and running, and the `dist` user manager has lingering
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
  systemctl --user restart gd-doom-coop.service
  systemctl --user status gd-doom-coop.service --no-pager'
```

The initial installed binary SHA-256 was
`53cf567d2b1985653c454f4f12aaeb9c9378f32c24489abaeb9e0289ae07f8d3`.
The shareware WAD SHA-256 is
`1d7d43be501e67d927e415e0b8f3e29c3bf33075e859721816f652a526cac771`.
The initial build came from branch `experiments`, HEAD
`f16e8457f2e545d9e1779022028c2f7027a796c5`, with uncommitted multiplayer changes;
that commit alone does not reproduce the deployed binary.

The installed service unit is:

```ini
[Unit]
Description=GD-DOOM default authoritative co-op server
After=network-online.target

[Service]
Type=simple
WorkingDirectory=%h/.local/share/gd-doom
ExecStart=%h/.local/bin/gdserver -wad DOOM1.WAD -map E1M1 -mode coop -skill 3 -players 4 -listen 127.0.0.1:6671 -web-listen :6672 -udp-listen :6672 -tls-cert /etc/letsencrypt/live/m45sci.xyz-0003/fullchain.pem -tls-key /etc/letsencrypt/live/m45sci.xyz-0003/privkey.pem -web-origins https://m45sci.xyz
Restart=always
RestartSec=3
TimeoutStopSec=10
NoNewPrivileges=true
PrivateTmp=true
UMask=0077

[Install]
WantedBy=default.target
```

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

The deployment probe uses certificate verification, queries live slot counts
and the real manifest, joins a temporary player, decodes a snapshot and sends
explicit Leave. It also
accepts `-transport wss -address wss://m45sci.xyz:6672/netplay` or
`-transport wt -address https://m45sci.xyz:6672/netplay`. Run these from a different
machine to prove public reachability; successful host-local tests do not prove
the firewall permits internet clients.

## Publish the browser assets

Build with a unique ID for each deployment, including uncommitted builds:

```bash
GOCACHE=/tmp/gd-doom-wasm-cache \
BUILD_ID="multiplayer-$(date -u +%Y%m%dT%H%M%SZ)" \
MULTIPLAYER_SERVER=https://m45sci.xyz:6672/netplay \
  ./scripts/build_wasm.sh /tmp/gd-doom-wasm-release
```

The output includes the shareware WAD and General MIDI SoundFont embedded in
`gddoom.wasm`. Publish these seven assets and an updated `wasm-manifest.json`:
`index.html`, `player.html`, `launch.js`, `build-id.js`, `wasm_exec.js`,
`gddoom.wasm`, and `gddoom.wasm.gz`. Use a staged upload, verify the hashes, and
publish the manifest last. For direct updates, `rsync --delay-updates` delays
replacement until the transfer has completed.

Never use `rsync --delete` against the existing site directory. It also contains
`SC55-HQ.sf2`, `SGM-HQ.sf2` and historical manifests that are not build outputs.
The existing nginx configuration already sends `application/wasm` and
`Cache-Control: no-cache`.

Preserve the existing manifest's `branch`, `commit`, `build_id` and `files`
fields. Add `schema_version: 1`, `source_dirty` and `built_at` (UTC RFC3339).
`files` maps each of the seven asset names to its SHA-256; it excludes the
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
