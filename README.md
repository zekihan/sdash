<h1 align="center" id="title">SDash</h1>

<p id="description">A homelab dashboard: every service on one screen, with live health, host stats and a command palette. One Go binary, configured with two YAML files.</p>

## Features

* **Service grid** grouped your way, from a single `dashboard.yaml` — icons, descriptions, tags, locations, clusters
* **Live health checks** — the server probes every service on an interval: status dots, latency, uptime history and sparklines stream to the browser over SSE
* **Command palette** (`⌘K` or `/`) — fuzzy-search services, run actions, or fall through to web search
* **Widgets** — clock and greeting, service up/down summary, host CPU/memory/disk (Linux), Docker containers, Open-Meteo weather, Kubernetes + Argo CD cluster state, Proxmox VE hosts/guests/storage, TrueNAS pools, Gatus endpoint checks
* **Live service data** — Sonarr/Radarr queues, Jellyfin now-playing, qBittorrent speeds, Immich/Miniflux/Forgejo/Semaphore/Paperless/Syncthing stats and Home Assistant entities with toggles (server-side API keys)
* **Needs-attention feed** — down services, failing pods, Argo drift, expiring certificates, degraded Longhorn volumes, pending node reboots, Gatus failures and failed backups in one list
* **Actions (opt-in)** — Argo sync, rollout restart, Proxmox guest start, Wake-on-LAN and Home Assistant toggles from the dashboard, behind `actions.enabled` and per-integration write credentials
* **Synced preferences** — theme, favorites and layout stored on the server, following you across browsers
* **Favorites & recents** — pin services, jump back to recent ones (stored per browser)
* **Themes** — dark/light/system, seven accents, comfortable/compact density, grid/list layouts, optional background image
* **Hot reload** — edit `dashboard.yaml` and open browsers update live
* **Optional OIDC sign-in** — Authentik, Authelia, Keycloak, any provider; no auth configured means no login wall
* **Hardened icon proxy** for fetching external icons safely (see below)

## Quick start

Edit `config.yaml` (server) and `dashboard.yaml` (your services), then:

```sh
make run          # builds the web UI + binary, serves on http://localhost:33000
```

Or with Docker (binaries first, then the image):

```sh
make build/docker
docker compose up --build
```

## Configuration

### `config.yaml` — the server

Loaded once at startup. Environment variables override YAML with the `SDS__`
prefix (e.g. `SDS__WEB_PORT=8080`, `SDS__HEALTH_INTERVAL=15s`,
`SDS__OIDC_AUTHENTIK_CLIENT_ID=...`).

| Section | What it controls |
|---|---|
| `web` | listen host/port, public `baseUrl`, session lifetime, cookie keys |
| `log` | level, source locations |
| `dashboard.reload` | hot-reload of `dashboard.yaml` (fsnotify or mod-time polling) |
| `health` | service prober: `enabled`, `interval`, `timeout`, samples of `history` |
| `system` | host stats widget (`enabled`) |
| `docker` | containers widget: `enabled`, `host` (socket path, `unix://` or `tcp://`) |
| `weather` | Open-Meteo widget: `enabled`, `latitude`, `longitude`, `label`, `units` |
| `kubernetes` | cluster widget: `enabled`, `kubeconfig` (empty = in-cluster, then `~/.kube/config`), `argocdNamespace` |
| `proxmox` | Proxmox VE widget: `enabled`, `url` (any node), `apiToken` (`user@realm!id=secret`, PVEAuditor), `insecure` |
| `truenas` | TrueNAS widget: `enabled`, `url`, `apiKey`, `insecure` |
| `gatus` | Gatus widget: `enabled`, `url` |
| `pbs` | Proxmox Backup Server widget: `enabled`, `url`, `node`, `apiToken` (Datastore.Audit), `insecure` |
| `crowdsec` | CrowdSec bans widget: `enabled`, `url` (LAPI), `apiKey` (bouncer key) |
| `actions` | write actions master switch: `enabled`, `wakeOnLan` hosts (name/mac/broadcast) |
| `preferences` | server-side preference sync: `enabled`, `path` |
| `integrations` | widget API credentials: `sonarr`/`radarr`/`jellyfin`/`qbittorrent` (username+password)/`immich`/`miniflux`/`forgejo`/`semaphore`/`paperless`/`syncthing`/`homeassistant`, each with `url` + key |
| `oidc` | sign-in providers keyed by name (`clientId`, `clientSecret`, `discoveryEndpoint`, `enabled`, `default`) |

`CONFIG_PATH` / `CONFIG_DIR` select the file. To use the Docker widget, mount
the socket read-only (`/var/run/docker.sock:/var/run/docker.sock:ro`) and set
`docker.enabled: true`.

### `dashboard.yaml` — the dashboard

`DASHBOARD_CONFIG_PATH` / `DASHBOARD_CONFIG_DIR` select the file. Existing
files from SDash 1.x keep working; every new field is optional.

```yaml
settings:
  title: Homelab
  subtitle: All services, one screen
  searchEngine: https://duckduckgo.com/?q=%s   # command-palette web search
  defaultTheme: dark        # dark | light | system
  defaultAccent: lime
  # background: https://example.com/wallpaper.jpg

groups:
  media: { title: Media, order: 1 }

services:
  jellyfin:
    title: Jellyfin
    description: Movies, shows and music
    url: https://jellyfin.lan
    icon: https://cdn.jsdelivr.net/gh/walkxcode/dashboard-icons/svg/jellyfin.svg
    group: media
    tags: [streaming]
    healthUrl: https://jellyfin.lan/health   # probe target (default: url)
    healthKeyword: Healthy                   # required substring in the reply
    healthStatuses: [200]                    # accepted codes (default: 2xx/3xx/401/403)
    healthInsecure: true                     # allow self-signed certs when probing
    # healthDisabled: true                   # never probe this service
    # openInNewTab: false
    widget: {type: jellyfin}                 # sonarr | radarr | jellyfin; key in config.yaml
```

The Kubernetes widget needs read-only RBAC: list/get on nodes, pods,
deployments and namespaces cluster-wide, plus Argo CD `applications` in the
Argo CD namespace. Without Argo CD that section simply hides.

Services without a group land in "Others"; unknown groups are created on the
fly. Ordering: `orderServicesVia` / `orderGroupsVia` accept `title`, `key`,
`order` or `none`.

## HTTP API

JSON, same auth as the UI. `GET /api/ping` and `GET /api/health` are public.

| Endpoint | Purpose |
|---|---|
| `GET /api/me` | auth state + configured providers |
| `GET /api/dashboard` | settings, feature flags, groups and services (health embedded) |
| `GET /api/health/services` | latest probe result + history per service |
| `GET /api/system` | host CPU/memory/disk/load/uptime |
| `GET /api/docker` | container list (or `available: false`) |
| `GET /api/weather` | current conditions + 5-day forecast |
| `GET /api/kubernetes` | nodes, pod/deployment readiness, Argo CD sync + health |
| `GET /api/proxmox` | Proxmox VE nodes, guests, storage |
| `GET /api/truenas` | TrueNAS version, pools, alerts |
| `GET /api/gatus` | Gatus endpoint up/down summary |
| `GET /api/service-widgets` | live per-service widget data (Sonarr/Radarr/Jellyfin/qBittorrent/Immich/Miniflux/Forgejo/Semaphore/Paperless/Syncthing/Home Assistant) |
| `GET /api/pbs` | Proxmox Backup Server datastores + recent backup outcomes |
| `GET /api/crowdsec` | CrowdSec active decision counts |
| `GET/PUT /api/preferences` | server-synced dashboard preferences |
| `POST /api/actions/*` | write actions (`argo-sync`, `restart`, `proxmox-start`, `wol`, `homeassistant`) — 403 unless `actions.enabled` |
| `GET /api/events` | SSE stream: `health` rounds and `config` reloads |
| `GET /api/proxy/icon?url=...` | hardened icon proxy (below) |
| `POST /api/logout` | clear the session cookie |

## Development

```sh
make web        # production frontend build into web/dist (embedded by Go)
make dev        # build web + run the server with the dev config
make web/dev    # Vite dev server on :5173, proxying /api to :33000
make test       # Go tests (incl. -race)
make lint       # golangci-lint + TypeScript check
```

The frontend is React + TypeScript + Tailwind in `web/`; the backend is Go
(stdlib HTTP) in `internal/`. `web/dist` is generated and not committed — a
placeholder keeps `go build` working before the first `make web`.

## Icon proxy

Set `proxyIcon: true` on a service to fetch its icon through `/api/proxy/icon`.
The proxy accepts PNG, JPEG and GIF responses with matching image content types,
and decodes and re-encodes them as PNG. GIF animation is reduced to its first frame.
SVG, HTML, other formats, redirects and non-2xx responses are rejected. If you
currently proxy SVG icons, configure a raster version before upgrading. The sample
dashboard's SVG icons load directly because `proxyIcon` defaults to `false`.

Proxied URLs must use HTTP on port 80 or HTTPS on port 443. Every DNS result must
be a public address; private, loopback, link-local, metadata and special-use
addresses are blocked. Connections use the validated IP directly while preserving
the original hostname for Host and TLS verification. Environment HTTP proxies
are bypassed so they cannot resolve or route around this check. This endpoint
is intended for public icons, and cannot fetch icons hosted on a private network.

Each upstream body and re-encoded PNG is limited to 1 MiB. Images are limited to
2,048 pixels on either axis and 1,048,576 pixels in total before decoding. The
cache retains at most 100 images and 8 MiB of image buffer capacity for ten
minutes. At most four distinct fetch/decode jobs run at once; simultaneous misses
for the same URL share a job. Overload returns 503. A bounded per-peer limiter
allows 120 requests per minute, including cache hits, and returns 429 with
`Retry-After` when exhausted. It uses the connection peer, not forwarded headers,
so callers behind a reverse proxy share that proxy's budget.

The health prober is deliberately different: it only ever requests URLs from
your own `dashboard.yaml`, and private addresses are the normal case there.

The checked-in listener binds to loopback and OIDC is disabled. Deployments that
expose SDash to other users should review their authentication and egress policy.
Local security tests use synthetic DNS answers and mocked/in-memory HTTP peers;
production networking, reverse-proxy budgets and browser acceptance require a
separately authorized deployment check.

## Built with

* Go (stdlib `net/http`, embedded frontend, single binary)
* React + TypeScript + Vite + Tailwind CSS

## License

This project is licensed under the GNU Affero General Public License v3.0

### Authentication lifetime and access logs

`web.sessionTimeout` (or `SDS__WEB_SESSION_TIMEOUT`) sets the authenticated
session expiry and the browser cookie's `Max-Age` and `Expires`. The default is
`24h`. Set a whole number of seconds from `1s` through `720h` (30 days); invalid
values prevent startup. Existing signed sessions retain their original expiry;
the configured lifetime applies to newly issued sessions.

Access logs include method, path, status, response size and duration. They omit
all query values, full URLs, headers and bodies, including callback `code` and
`state` and proxy URLs containing credentials. OIDC failure logs identify the
failed operation without recording provider error bodies, which may echo secrets.
Operators must configure equivalent query omission/redaction in reverse proxies
and centralized log pipelines. Those deployment settings are not managed by SDash.

### Dashboard hot reload

Set `dashboard.reload.enabled` in `config.yaml` to reload `dashboard.yaml`.
With `useFsNotify: true`, SDash watches its parent directory and handles writes,
creation, removal, and renaming of the configured filename. This supports editors
that save by atomic replacement, including repeated replacements and later writes.
Changes to other filenames do not trigger reloads. Connected browsers receive
the new dashboard over the `/api/events` stream without a page refresh.

The default polling mode compares the complete modification timestamp on every
`pollInterval`, including subsecond differences. Any different timestamp reloads,
including an older timestamp on a replacement. Changes that preserve exactly the
same timestamp are not detected by polling; use fsnotify for those saves. Polling
observes the latest file at each interval, not every intermediate edit.
When polling reload is enabled, `pollInterval` must be positive. Zero and negative
intervals (including environment overrides) are rejected at startup. The interval
is unused when reload is disabled or `useFsNotify` is enabled.

With `continueOnError: true`, missing or temporarily invalid dashboard files keep
the last successfully loaded dashboard; later valid changes can reload normally.
With it disabled, a reload error still panics. Polling skips missing files until
they reappear; fsnotify removal events attempt a reload immediately.

A dashboard config must contain a YAML mapping. Empty, whitespace-only,
comments-only, and null documents are reload errors, so a truncate-before-write
save cannot replace the last good dashboard with an accidental empty one when
`continueOnError` is enabled. To intentionally clear the dashboard, save `{}`.
This does not make arbitrary multi-step writes transactional; use atomic
replacement when a save must never expose intermediate valid YAML.

Publishing and manual recovery: [Publishing](docs/publishing.md).
