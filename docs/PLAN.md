# StreamMon — Implementation Plan

## Overview
Simplified Tautulli alternative supporting Plex, Emby, and Jellyfin. Go backend + React/TypeScript frontend, single binary, Docker Compose deployment.

## Tech Stack
- **Backend**: Go 1.22+, Chi router, SQLite (mattn/go-sqlite3)
- **Frontend**: React 18 + TypeScript, Vite, Tailwind CSS (mobile-first), Recharts, Leaflet
- **Auth**: OIDC (go-oidc) — optional, first-class
- **GeoIP**: MaxMind GeoLite2
- **Live updates**: SSE
- **Theme**: System-preference toggle (dark/light)
- **Deploy**: Docker Compose

## Coding Guidelines (enforced via CLAUDE.md)
- **TDD**: Write tests first, then implementation, for all code (Go + React)
- **No `any` types** in TypeScript — everything typed
- **DRY**: Shared functions, minimize duplicate code
- **Simple > Complex**: Simplest solution that works
- **Minimal comments**: Only when logic isn't self-evident
- **CRUD patterns**: Consistent resource handling across all entities

---

## Phase 1: Scaffolding & Build Pipeline ✅
**Goal**: Go binary compiles, Vite dev server runs, Docker builds, "Hello StreamMon" served.

| File | Purpose |
|------|---------|
| `go.mod` | Module init, deps: chi, go-sqlite3, go-oidc, maxminddb-golang |
| `cmd/streammon/main.go` | Entry point: parse env/flags, open DB, run migrations, mount routes, start HTTP |
| `internal/server/server.go` | `Server` struct, `NewServer()`, Chi mux, `GET /api/health` |
| `internal/server/routes.go` | Route registration |
| `internal/server/embed.go` | `//go:embed web/dist`, SPA fallback file server |
| `internal/store/store.go` | `Store` interface, `SQLiteStore` struct, `New(dbPath)` |
| `internal/store/migrate.go` | Sequential migration runner with `schema_migrations` table |
| `migrations/001_init.sql` | Tables: `users`, `servers`, `watch_history`, `sessions`, `settings`, `ip_geo_cache` |
| `web/package.json` | Vite + React + TS + Tailwind + Recharts + react-leaflet + react-router-dom |
| `web/vite.config.ts` | Dev proxy `/api` → `:8080`, output `dist/` |
| `web/tailwind.config.js` | `darkMode: 'class'`, content paths |
| `web/index.html` | HTML shell with viewport meta for mobile |
| `web/src/main.tsx` | React entry |
| `web/src/App.tsx` | Router: `/`, `/history`, `/users/:name`, `/settings` |
| `web/src/index.css` | Tailwind directives |
| `Dockerfile` | Multi-stage: Node build → Go build (CGO_ENABLED=1) → slim Debian |
| `docker-compose.yml` | Service with volume mounts for `data/` and `geoip/` |
| `Makefile` | `dev-backend`, `dev-frontend`, `build`, `docker` |
| `CLAUDE.md` | Coding guidelines: TDD, no `any`, DRY, minimal comments, simple > complex |
| `.gitignore` | Go + Node + data/ + geoip/ |

**Verify**: `go run ./cmd/streammon` → browser shows React page, `curl /api/health` → 200.

---

## Phase 2: Models, Store Layer & Settings API
**Goal**: Server CRUD + history queries work via curl.

| File | Purpose |
|------|---------|
| `internal/models/models.go` | `User`, `Server`, `WatchHistoryEntry`, `ActiveStream`, `MediaType` enum, `Role` enum |
| `internal/store/servers.go` | `ListServers`, `GetServer`, `CreateServer`, `UpdateServer`, `DeleteServer` |
| `internal/store/history.go` | `InsertHistory`, `ListHistory(page, perPage, userFilter)`, `ListHistoryByUser`, `DailyWatchCounts(start, end)` → `[]DayStat` |
| `internal/store/users.go` | `GetOrCreateUser`, `ListUsers`, `GetUser`, `UpdateUserRole` |
| `internal/store/settings.go` | `GetSetting(key)`, `SetSetting(key, value)` |
| `internal/server/api_servers.go` | REST: `GET/POST /api/servers`, `GET/PUT/DELETE /api/servers/{id}`, `POST /api/servers/{id}/test` |
| `internal/server/api_history.go` | `GET /api/history`, `GET /api/history/daily` |
| `internal/server/api_users.go` | `GET /api/users`, `GET /api/users/{name}` |
| `internal/server/middleware.go` | JSON content-type, request logging, CORS (dev) |

| `internal/store/store_test.go` | Tests for all store methods (in-memory SQLite). Written first. |
| `internal/server/api_servers_test.go` | HTTP handler tests using httptest. Written first. |

**TDD flow**: Write store tests → implement store → write handler tests → implement handlers.

**Verify**: All tests pass. Full server CRUD round-trips via curl. History query returns empty paginated result.

---

## Phase 3: Media Server Adapters & Poller
**Goal**: Live sessions from real Plex/Emby/Jellyfin servers, history recorded on stop.

| File | Purpose |
|------|---------|
| `internal/media/interface.go` | `MediaServer` interface: `Name()`, `Type()`, `GetSessions(ctx)`, `GetLibraries(ctx)`, `TestConnection(ctx)` |
| `internal/media/factory.go` | `NewMediaServer(config) (MediaServer, error)` — dispatches on type |
| `internal/media/plex/plex.go` | Plex adapter: `/status/sessions` XML → `[]ActiveStream` |
| `internal/media/emby/emby.go` | Emby adapter: `/Sessions` JSON → `[]ActiveStream` |
| `internal/media/jellyfin/jellyfin.go` | Jellyfin adapter: `/Sessions` JSON (shares base with Emby) |
| `internal/poller/poller.go` | Background ticker (10s). Polls all enabled servers, diffs snapshots, records stopped sessions to DB. Exposes `CurrentSessions()`. Publishes to SSE channel. |
| `internal/server/api_dashboard.go` | `GET /api/dashboard/sessions` |
| `internal/server/sse.go` | `GET /api/dashboard/sse` — fan-out to connected clients, cleanup on disconnect |

### Transcoding & Stream Detail (Tautulli-style)
`ActiveStream` includes transcoding/quality fields: `VideoCodec`, `AudioCodec`, `VideoResolution`, `TranscodeDecision` (direct play/copy/transcode), `TranscodeHWAccel` (bool), `Bitrate`, `Container`, `AudioChannels`, `SubtitleCodec`, `TranscodeProgress`. Plex adapter parses `<TranscodeSession>` + `<Media>/<Part>/<Stream>` elements. Emby/Jellyfin adapter parses `TranscodingInfo` + `MediaSources` from session JSON. Frontend `StreamCard` (Phase 5) renders codec badges, resolution, transcode indicator, and bandwidth.

| `internal/media/plex/plex_test.go` | Tests against canned XML fixtures. Written first. |
| `internal/media/emby/emby_test.go` | Tests against canned JSON fixtures. Written first. |
| `internal/media/jellyfin/jellyfin_test.go` | Tests against canned JSON fixtures. Written first. |
| `internal/poller/poller_test.go` | Tests diff logic with mock MediaServer. Written first. |

**TDD flow**: Write adapter tests with fixture data → implement adapters → write poller tests → implement poller.

**Verify**: All tests pass. Configure a server via API → active sessions appear at `/api/dashboard/sessions`. Stop a stream → history entry created.

---

## Phase 4: GeoIP Resolution
**Goal**: IPs enriched with coordinates, cached.

| File | Purpose |
|------|---------|
| `internal/geoip/resolver.go` | `Resolver` wrapping MaxMind reader. `Lookup(ip) → *GeoResult{Lat,Lng,City,Country}`. Returns nil gracefully if DB missing. |
| `internal/store/geoip_cache.go` | `GetCachedGeo(ip)`, `SetCachedGeo(ip, geo)` — 30-day TTL |
| `internal/server/api_users.go` | Add `GET /api/users/{name}/locations` — distinct IPs → geo-resolved |

| `internal/geoip/resolver_test.go` | Tests with known IPs. Written first. |

**TDD flow**: Write resolver tests → implement resolver → write cache tests → implement cache.

**Verify**: All tests pass. `curl /api/users/somename/locations` → array with lat/lng.

---

## Phase 5: Frontend — Layout, Dashboard, History
**Goal**: Working dashboard and history pages in browser. **Mobile-first**.

> **Design note**: Use the `frontend-design` plugin when implementing this phase. The UI/UX should be modern, cool, and professional — not generic.

| File | Purpose |
|------|---------|
| `web/src/components/Layout.tsx` | **Mobile**: bottom tab bar nav (Dashboard, History, Settings). **Desktop**: sidebar nav. Top bar with theme toggle. `<Outlet/>`. |
| `web/src/components/ThemeToggle.tsx` | System/dark/light, localStorage, sets `dark` class on `<html>` |
| `web/src/components/MobileNav.tsx` | Bottom tab bar, visible only on `sm:` and below |
| `web/src/components/Sidebar.tsx` | Desktop sidebar, hidden on mobile (`hidden lg:block`) |
| `web/src/hooks/useSSE.ts` | Connects `/api/dashboard/sse`, auto-reconnect, returns reactive sessions |
| `web/src/hooks/useFetch.ts` | Generic `useFetch<T>(url, deps)` with loading/error |
| `web/src/lib/api.ts` | Typed fetch wrapper: `api.get<T>()`, `api.post<T>()` |
| `web/src/types.ts` | TS interfaces: `ActiveStream`, `WatchHistoryEntry`, `Server`, `User`, `DayStat`, `GeoLocation` |
| `web/src/pages/Dashboard.tsx` | Stream cards in responsive grid (`grid-cols-1 md:grid-cols-2 xl:grid-cols-3`). Uses `useSSE`. Empty state. |
| `web/src/components/StreamCard.tsx` | Tautulli-style stream card: user, title, media type, player, progress bar, plus transcoding details (video/audio codec, resolution, transcode decision with HW badge, bitrate, subtitles). Codec/quality info shown as compact badges. |
| `web/src/pages/History.tsx` | **Mobile**: card-based list. **Desktop**: full table. Paginated, sortable. User names link to `/users/:name`. |
| `web/src/components/HistoryTable.tsx` | Reusable, accepts `userFilter` prop. Responsive: hides less important columns on mobile via `hidden md:table-cell`. |

**Verify**: Dashboard live-updates. History paginates. Looks good on both mobile and desktop viewports.

---

## Phase 6: Frontend — Charts, User Detail, Map
**Goal**: Line graph, user detail with map.

| File | Purpose |
|------|---------|
| `web/src/pages/UserDetail.tsx` | User header + role badge. Tabs: Watch History, Locations. Responsive layout. |
| `web/src/components/DailyChart.tsx` | Recharts `<ResponsiveContainer>` + `<LineChart>` with Movies/TV/LiveTV series. Date range selector (7/30/90 days). |
| `web/src/components/LocationMap.tsx` | Leaflet map, fetches `/api/users/:name/locations`, markers with popups (city, country, IP, last seen). Full-width on mobile. |
| `web/src/pages/Dashboard.tsx` | Add `<DailyChart/>` below stream cards |

**Verify**: Stats graph renders. User page shows watch history + world map with markers.

---

## Phase 7: Settings UI
**Goal**: Server management in browser.

| File | Purpose |
|------|---------|
| `web/src/pages/Settings.tsx` | Server list with status badges. Add button. Stacked cards on mobile. |
| `web/src/components/ServerForm.tsx` | Modal form: name, type dropdown, URL, API key, enabled toggle. "Test Connection" button. Responsive modal (full-screen on mobile). |

**Verify**: Full server lifecycle from UI. Test connection shows success/failure.

---

## Phase 8: Authentication (OIDC)
**Goal**: Optional OIDC. App works without config. When enabled, login required.

| File | Purpose |
|------|---------|
| `internal/auth/auth.go` | `AuthService`. If `OIDC_ISSUER` + `OIDC_CLIENT_ID` + `OIDC_CLIENT_SECRET` set → initialize provider. Otherwise disabled. |
| `internal/auth/oidc.go` | `/auth/login` → IdP redirect. `/auth/callback` → code exchange, create user+session. `/auth/logout`. |
| `internal/auth/session.go` | Cookie-based sessions stored in `sessions` table. `SessionFromRequest(r) → *User`. |
| `internal/server/middleware_auth.go` | `RequireAuth`: if OIDC enabled, check session; if disabled, inject default admin. `RequireRole(role)` for admin-only routes. |
| `internal/server/api_me.go` | `GET /api/me` → current user |
| `web/src/hooks/useAuth.ts` | Auth context, checks `/api/me`, redirects to login on 401 |
| `web/src/components/AuthGuard.tsx` | Wraps protected routes |

**Verify**: Without OIDC vars → app works as before. With OIDC → login flow works, 401 on unauthenticated API calls.

---

## Phase 9: Polish & Production
**Goal**: Graceful shutdown, error handling, Docker works end-to-end.

| File | Purpose |
|------|---------|
| `internal/server/server.go` | Graceful shutdown via context |
| `internal/poller/poller.go` | `Stop()`, context cancellation |
| `web/src/components/ErrorBoundary.tsx` | React error boundary |
| `web/src/components/EmptyState.tsx` | Reusable empty-state component |
| `.env.example` | All env vars documented |
| `.dockerignore` | node_modules, .git, data/ |
| `README.md` | Setup, env vars, usage |

---

## Mobile Strategy (all phases)
- **Tailwind mobile-first**: base styles are mobile, `md:` / `lg:` for larger screens
- **Navigation**: bottom tab bar on mobile, sidebar on desktop
- **Tables**: card layout on mobile, full table on desktop (or hide non-essential columns)
- **Modals/forms**: full-screen sheet on mobile, centered modal on desktop
- **Charts**: `<ResponsiveContainer>` ensures proper sizing
- **Maps**: full-width on mobile, constrained on desktop
- **Touch targets**: minimum 44px tap targets on all interactive elements

---

## Verification (end-to-end)
1. `docker compose up --build` starts successfully
2. Browse to `http://localhost:8080` on desktop and mobile viewport
3. Add a Plex/Emby/Jellyfin server in Settings, test connection passes
4. Start a stream → appears on Dashboard in real-time
5. Stop stream → appears in History table
6. Click user name → User Detail with watch history and map
7. Daily chart shows data points
8. Toggle dark/light theme
9. (Optional) Configure OIDC env vars, restart → login required
