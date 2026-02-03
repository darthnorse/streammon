# StreamMon

A simplified media server monitoring tool supporting Plex, Emby, and Jellyfin. Go backend + React/TypeScript frontend, single binary, Docker Compose deployment.

## Features

- Real-time stream monitoring via SSE
- Watch history tracking with pagination
- Per-user detail pages with location maps (GeoIP)
- Daily watch statistics charts
- Multi-server support (Plex, Emby, Jellyfin)
- Optional OIDC authentication
- Dark/light theme
- Mobile-first responsive design

## Quick Start

```bash
cp .env.example .env
docker compose up --build
```

Browse to `http://localhost:7935`.

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `DB_PATH` | `./data/streammon.db` | SQLite database path |
| `GEOIP_PATH` | `./geoip/GeoLite2-City.mmdb` | MaxMind GeoLite2 database path |
| `LISTEN_ADDR` | `:7935` | HTTP listen address |

### Optional OIDC

OIDC authentication is configured from the Settings UI at runtime. Alternatively, you can set these environment variables for initial bootstrap:

| Variable | Description |
|----------|-------------|
| `OIDC_ISSUER` | OIDC provider URL (e.g. `https://accounts.google.com`) |
| `OIDC_CLIENT_ID` | OAuth client ID |
| `OIDC_CLIENT_SECRET` | OAuth client secret |
| `OIDC_REDIRECT_URL` | Callback URL (e.g. `http://localhost:7935/auth/callback`) |

Settings UI values take precedence over environment variables.

## Development

```bash
# Backend
make dev-backend

# Frontend
make dev-frontend
```

## GeoIP

Download [MaxMind GeoLite2-City](https://dev.maxmind.com/geoip/geolite2-free-geolocation-data) database and place at the `GEOIP_PATH` location.

## Tech Stack

- **Backend**: Go, Chi router, SQLite
- **Frontend**: React 18, TypeScript, Vite, Tailwind CSS, Recharts, Leaflet
- **Auth**: OIDC (optional)
- **Deploy**: Docker Compose
