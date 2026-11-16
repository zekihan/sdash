# AGENTS.md

This file provides guidance to AI Agents when working with code in this repository.

## Commands

```sh
# Build the frontend into web/dist (embedded by the Go binary)
make web

# Build binary for local platform (web + goreleaser snapshot)
make build

# Run the server with the checked-in dev config on :33000
make dev

# Vite dev server on :5173 (proxies /api to :33000; run `make dev` alongside)
make web/dev

# Run all tests (scripts/go_test.sh sets CONFIG_DIR and DASHBOARD_CONFIG_DIR)
make test

# Run a single test package
./scripts/go_test.sh -v -run TestName ./config/dashboard/...

# Format + typecheck, linter, built binary
make tidy
make lint
make run
```

`web/dist` is generated and gitignored except a `.gitkeep` placeholder, so
`go build`/`go test` work before the first `make web`; without a frontend
build the server answers `/` with a "run make web" notice while the API works.

## Architecture

SDash is a single Go binary serving a homelab dashboard: JSON API + SSE live
updates + an embedded React/TypeScript SPA (Vite + Tailwind, in `web/`).

### Two-config system

**`config.yaml`** (`config/config.go`) — app config: web server, logging, OIDC
providers, dashboard reload, and the `health`/`system`/`docker`/`weather`/
`kubernetes`/`proxmox`/`truenas`/`gatus`/`integrations` widget sections. Loaded once at startup as a singleton. Env overrides use the
`SDS__` prefix (e.g. `SDS__WEB_PORT`, `SDS__HEALTH_INTERVAL`); OIDC providers
are keyed by name (`SDS__OIDC_<NAME>_<FIELD>`).

**`dashboard.yaml`** (`config/dashboard/`) — services, groups and `settings`
(title, search engine, default theme/accent, background). Its own singleton
behind an RWMutex with hot-reload (fsnotify or mod-time polling, from
`dashboard.reload`). `OnReload` listeners (SSE push, prober targets) fire after
each successful reload. Env: `DASHBOARD_CONFIG_PATH` or `DASHBOARD_CONFIG_DIR`.
Schema is backward compatible with 1.x files; `postProcessConfig()` assigns
unkeyed services to `others`, creates missing groups, sorts per
`orderServicesVia`/`orderGroupsVia`, and fills settings defaults.

### Backend packages

- `internal/server` — routes, DTOs, auth gate (401 JSON for APIs), SSE hub,
  SPA/static serving, plus the OIDC login/callback handlers.
- `internal/health` — background prober for every service (`healthUrl`,
  `healthKeyword`, `healthStatuses`, `healthInsecure`, `healthDisabled`);
  bounded per-service history feeds status/uptime/sparklines.
- `internal/systeminfo` — host CPU/mem/disk/load from `/proc` (Linux; degrades
  to `available: false` elsewhere).
- `internal/dockerinfo` — read-only Docker Engine API client (unix socket or
  TCP), no SDK dependency.
- `internal/weather` — Open-Meteo client with a 15-minute server-side cache.
- `internal/kubeinfo` — dependency-free Kubernetes REST summary (nodes,
  pods, deployments, Argo CD applications); in-cluster SA or kubeconfig
  (token or client-cert; exec plugins unsupported).
- `internal/gatusinfo` — Gatus `/api/v1/endpoints/statuses` reducer.
- `internal/proxmoxinfo` — Proxmox VE `/cluster/resources` summary (nodes,
  guests, shared storage deduped) with a PVEAuditor API token.
- `internal/truenasinfo` — TrueNAS REST v2 summary (system info, pools,
  undismissed alerts) with a Bearer API key.
- `internal/appwidgets` — Sonarr/Radarr/Jellyfin live card data; keys from
  `integrations` config only, results cached 30s and shared per widget type.
- `internal/icon` — hardened icon proxy (public-IP-only, re-encodes to PNG,
  bounded cache/concurrency/rate). Kept from 1.x; see README before touching.
- `session/auth` — OIDC provider init; auth is on only when ≥1 provider is
  enabled. Sessions are go-commons secure cookies.
- `requestlog` — credential-safe access logs (no query strings/headers).

### API surface

`/api/me`, `/api/dashboard`, `/api/health/services`, `/api/system`,
`/api/docker`, `/api/weather`, `/api/kubernetes`, `/api/proxmox`,
`/api/truenas`, `/api/gatus`,
`/api/service-widgets`, `/api/events` (SSE), `/api/proxy/icon`,
`POST /api/logout`; public `/api/ping`, `/api/health`; OIDC at
`/login`, `/login/{oidc}`, `/user/oauth2/{oidc}/callback`. Everything else
serves the SPA (`web/embed.go` embeds `web/dist`).

### Frontend

`web/src`: `App.tsx` orchestrates loading/SSE/polling/prefs; components in
`web/src/components`, helpers in `web/src/lib`. Client prefs (theme, accent,
density, layout, pins, recents) live in localStorage (`sdash:prefs:v1`) —
save failures must surface as a toast, never silently drop. Search is
client-side fuzzy matching over the `/api/dashboard` payload. Theming is CSS
variables (`data-theme`/`data-accent` on `<html>`, tokens in `index.css`)
mapped into Tailwind via `@theme inline`.

### Build

Goreleaser builds binaries; its before-hook runs `npm --prefix web run build`
(deps installed via `.github/scripts/install_build_deps.sh` = `npm ci`, the
only approved install script is esbuild's). The Dockerfile packages prepared
`dist/` binaries into distroless as user 25000 — it runs no toolchain itself.
