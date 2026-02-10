# StreamMon

A media server monitoring tool supporting Plex, Emby, and Jellyfin. Go backend + React/TypeScript frontend, single binary, Docker Compose deployment.

## Features

- Real-time stream monitoring via SSE
- Watch history tracking with pagination
- Per-user detail pages with location maps (GeoIP)
- Daily watch statistics and charts
- Multi-server support (Plex, Emby, Jellyfin)
- Overseerr integration with per-user request attribution
- Trust score and rule-based violation detection
- Authentication via local accounts, Plex, Emby, Jellyfin, or OIDC
- Notifications via Discord, Webhook, Pushover, Ntfy
- Dark/light theme
- Mobile-first responsive design

## Quick Start

```bash
cp docker-compose.example.yml docker-compose.yml
docker compose up -d
```

Browse to `http://localhost:7935` and create your admin account.

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `DB_PATH` | `./data/streammon.db` | SQLite database path |
| `LISTEN_ADDR` | `:7935` | HTTP listen address |
| `POLL_INTERVAL` | `5s` | How often to poll media servers for active streams |
| `GEOIP_DB` | `./geoip/GeoLite2-City.mmdb` | MaxMind GeoLite2 database path |
| `MAXMIND_LICENSE_KEY` | | MaxMind license key (auto-downloads GeoIP DB if set) |
| `TOKEN_ENCRYPTION_KEY` | | Base64-encoded 32-byte key for encrypting stored tokens (see below) |

## Overseerr Integration

StreamMon can proxy media requests through Overseerr and attribute them to the correct user.

**Basic setup** (configured in Settings > Integrations):
- Add your Overseerr URL and API key
- Requests are attributed to users by matching their email address against Overseerr accounts

**Plex token attribution** (optional, more accurate):

When enabled, StreamMon stores each user's Plex authentication token (encrypted at rest) and uses it to authenticate directly with Overseerr's Plex auth endpoint. This ensures requests are always attributed to the correct Overseerr user, even if their email doesn't match.

To enable:

1. Generate a 32-byte encryption key:
   ```bash
   openssl rand -base64 32
   ```

2. Add it to your `docker-compose.yml` environment:
   ```yaml
   environment:
     - TOKEN_ENCRYPTION_KEY=your-generated-key-here
   ```

3. Restart the container:
   ```bash
   docker compose up -d
   ```

4. Enable "Store Plex tokens for Overseerr" in Settings > Users

**Security notes:**
- Tokens are encrypted with AES-256-GCM before being stored in the database
- Without `TOKEN_ENCRYPTION_KEY`, the feature is unavailable and the toggle is hidden
- Plex token auth requires HTTPS (or localhost) for the Overseerr URL; plain HTTP falls back to email matching
- Disabling the toggle deletes all stored tokens immediately

## Authentication

StreamMon supports multiple authentication providers, all configured from the Settings UI:

- **Local accounts** - Username/password
- **Plex** - Sign in with Plex account
- **Emby/Jellyfin** - Sign in with media server credentials
- **OIDC** - Any OpenID Connect provider (Authentik, Authelia, Keycloak, etc.)

## GeoIP

For IP geolocation on the dashboard and user detail pages:

- **Auto-download**: Set `MAXMIND_LICENSE_KEY` in your environment (free account at [MaxMind](https://dev.maxmind.com/geoip/geolite2-free-geolocation-data))
- **Manual**: Download GeoLite2-City.mmdb and mount it at the `GEOIP_DB` path

## Development

```bash
make dev-backend    # Go backend with hot reload
make dev-frontend   # Vite dev server
make test           # Run all tests
make build          # Production build
```

## Tech Stack

- **Backend**: Go, Chi router, SQLite (WAL mode)
- **Frontend**: React 18, TypeScript, Vite, Tailwind CSS, Recharts, Leaflet
- **Auth**: Local, Plex, Emby, Jellyfin, OIDC
- **Deploy**: Docker Compose
