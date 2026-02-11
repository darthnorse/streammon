# StreamMon

StreamMon is a self-hosted media server management and monitoring platform for Plex, Emby, and Jellyfin. It provides real-time stream monitoring, watch history analytics, account sharing detection, library maintenance automation, and Overseerr integration -- all packaged as a single Go binary with an embedded React frontend, deployed via Docker Compose.

Typical memory footprint is around 15 MB. No runtime dependencies, no separate database process -- just a compiled Go binary with embedded SQLite and static frontend assets.

## Table of Contents

- [Quick Start](#quick-start)
- [Features](#features)
- [Configuration](#configuration)
- [Media Server Setup](#media-server-setup)
- [Authentication](#authentication)
- [Overseerr Integration](#overseerr-integration)
- [GeoIP Geolocation](#geoip-geolocation)
- [Sharing Detection Rules](#sharing-detection-rules)
- [Notifications](#notifications)
- [Library Maintenance](#library-maintenance)
- [Tautulli Import](#tautulli-import)
- [Reverse Proxy](#reverse-proxy)
- [Development](#development)
- [Tech Stack](#tech-stack)

## Quick Start

### 1. Create a `docker-compose.yml`

```yaml
services:
  streammon:
    image: ghcr.io/darthnorse/streammon:latest
    container_name: streammon
    ports:
      - "7935:7935"
    volumes:
      - ./data:/app/data
      - ./geoip:/app/geoip
    environment:
      - TOKEN_ENCRYPTION_KEY=${TOKEN_ENCRYPTION_KEY}
    restart: unless-stopped
```

### 2. Generate an encryption key

This key is used to encrypt stored Plex tokens for [Overseerr user attribution](#overseerr-integration). Run this command and copy the output:

```bash
openssl rand -base64 32
```

### 3. Create a `.env` file

Create a `.env` file in the same directory as your `docker-compose.yml` and paste the key from step 2:

```
TOKEN_ENCRYPTION_KEY=your-generated-key-here
```

### 4. Start the container

```bash
docker compose up -d
```

Open `http://localhost:7935` in your browser. On first launch you will be prompted to create an admin account.

## Features

**Real-Time Monitoring**
- Live stream dashboard with active session details from all configured servers in one view
- Per-stream bandwidth, transcoding status, player, and quality info
- Geographic stream map with IP geolocation

**Watch History and Analytics**
- Full watch history with search and filtering
- Daily, weekly, and hourly activity charts
- Per-user statistics: total watch time, stream count, devices, ISPs, locations
- Top movies, TV shows, and users
- Platform and player distribution breakdowns
- Concurrent stream tracking over time

**Account Sharing Detection**
- Eight configurable rule types for detecting shared accounts (see [Sharing Detection Rules](#sharing-detection-rules))
- Per-user trust scores that decrement on violations
- Real-time rule evaluation on every poll cycle
- Violation history with filtering and pagination
- Notifications via Discord, webhooks, Pushover, and Ntfy

**Library Maintenance**
- Automated cleanup of unwatched, low-resolution, or oversized media
- Five criterion types with configurable thresholds
- Daily evaluation with candidate review before deletion
- Per-item exclusion lists

**Overseerr Integration**
- Search and discover movies and TV shows
- Submit and manage media requests with per-user attribution
- Admin approval and rejection workflow

**Multi-Server Support**
- Plex, Emby, and Jellyfin from a single interface
- Per-server enable/disable toggle
- Concurrent polling across all servers

**Authentication**
- Local accounts, Plex, Emby, Jellyfin, and OIDC (Authentik, Authelia, Keycloak, etc.)
- Role-based access control (Admin and Viewer roles)
- Multi-provider account linking
- Optional guest access

**User Interface**
- Dark and light themes
- Mobile-first responsive design
- Non-admin users see only Requests and My Stats pages

## Configuration

StreamMon stores all data under two directories inside the container: `/app/data` (database) and `/app/geoip` (geolocation database). These are mapped to your host filesystem via bind mounts in `docker-compose.yml` so you can easily back up and inspect the data.

Most settings (media servers, auth providers, GeoIP license key, integrations) are configured through the web UI. The following environment variables are available for deployment-level configuration:

| Variable | Default | Description |
|----------|---------|-------------|
| `LISTEN_ADDR` | `:7935` | HTTP listen address |
| `POLL_INTERVAL` | `5s` | Media server polling interval (minimum `2s`) |
| `TOKEN_ENCRYPTION_KEY` | | Base64-encoded 32-byte key for encrypting stored Plex tokens (see [Overseerr Integration](#overseerr-integration)) |
| `CORS_ORIGIN` | | Allowed CORS origin for reverse proxy setups (see [Reverse Proxy](#reverse-proxy)) |
| `HOUSEHOLD_AUTOLEARN_MIN_SESSIONS` | `10` | Minimum sessions from an IP before auto-detecting it as a household location (set to `0` to disable) |

## Media Server Setup

Add your media servers in **Settings > Servers**. Each server needs:

- **Name**: Display name
- **Type**: Plex, Emby, or Jellyfin
- **URL**: Server address (e.g. `http://192.168.1.100:32400` for Plex)
- **API Key / Token**: Authentication token for the server API

Servers can be individually enabled or disabled. StreamMon polls all enabled servers at the configured interval.

## Authentication

StreamMon supports multiple authentication providers, all configured from the Settings UI.

**Local Accounts** -- Username and password authentication. Always available.

**Plex** -- Sign in with a Plex account. Users are matched by Plex username or email.

**Emby / Jellyfin** -- Sign in with media server credentials. Requires at least one Emby or Jellyfin server to be configured.

**OIDC** -- Any OpenID Connect provider (Authentik, Authelia, Keycloak, Google, etc.). Configure in Settings > Authentication with your provider's issuer URL, client ID, and client secret.

Accounts from different providers can be linked to a single StreamMon user. Guest access can be toggled in Settings > Users to allow unauthenticated browsing of the Requests page.

## Overseerr Integration

StreamMon proxies media requests through Overseerr and attributes them to the correct user.

### Basic Setup

1. In **Settings > Integrations**, add your Overseerr URL and API key
2. Requests are attributed to users by matching their email address against Overseerr accounts

### Plex Token Attribution (Optional)

For more reliable user attribution, StreamMon can store each user's Plex authentication token (encrypted at rest) and use it to authenticate directly with Overseerr's Plex auth endpoint. This ensures requests are attributed to the correct Overseerr user even when email addresses don't match.

1. Generate a 32-byte encryption key:
   ```bash
   openssl rand -base64 32
   ```

2. Add it to your `.env` file or `docker-compose.yml` environment:
   ```yaml
   environment:
     - TOKEN_ENCRYPTION_KEY=your-generated-key-here
   ```

3. Restart the container:
   ```bash
   docker compose up -d --build
   ```

4. Enable **Store Plex tokens for Overseerr** in Settings > Users
5. Users must log out and log back in via Plex for their token to be stored

**Security notes:**
- Tokens are encrypted with AES-256-GCM before storage
- Without `TOKEN_ENCRYPTION_KEY`, the feature is unavailable and the toggle is hidden
- Plex token auth requires HTTPS (or localhost) for the Overseerr URL to avoid sending tokens over unencrypted connections; plain HTTP falls back to email matching
- If using HTTP, import your Plex users into Overseerr (Users > Import Plex Users) to ensure email matching works
- Disabling the toggle deletes all stored tokens immediately

**Known issue:** A [bug in Overseerr](https://github.com/sct/overseerr/issues/4306) causes per-user tag creation to fail with newer versions of Radarr and Sonarr. If you use per-user tagging, disable "Tag Requests" in Overseerr Settings > Services > Radarr/Sonarr until the fix is available.

## GeoIP Geolocation

IP geolocation powers the dashboard stream map, user location tracking, and geographic sharing detection rules.

**Automatic download:** Add your MaxMind license key in **Settings > GeoIP**. StreamMon will download and update the GeoLite2-City database automatically. Create a free account at [MaxMind](https://dev.maxmind.com/geoip/geolite2-free-geolocation-data) to get a license key. The database is stored in the `./geoip` bind mount so it persists across container restarts.

**Manual setup:** Download the GeoLite2-City.mmdb file from MaxMind and place it in your `./geoip` directory on the host.

Lookups are cached in SQLite with a 30-day TTL. Historical watch data can be backfilled from Settings > GeoIP.

## Sharing Detection Rules

StreamMon evaluates eight rule types against active streams in real time. Rules are managed in the **Rules** page.

| Rule Type | Description |
|-----------|-------------|
| **Concurrent Streams** | Triggers when a user exceeds a configurable number of simultaneous streams. Optional household exemption. |
| **Geo-Restriction** | Whitelist or blacklist streaming by country. |
| **Impossible Travel** | Detects physically impossible movement between stream locations (configurable speed threshold, default 800 km/h). |
| **Simultaneous Locations** | Detects concurrent streams from geographically distant locations (configurable distance threshold). |
| **Device Velocity** | Triggers when a user connects from too many new devices within a time window. |
| **ISP Velocity** | Triggers when a user's ISP changes too frequently within a time window. |
| **New Device** | Informational alert when a user connects from a previously unseen device. |
| **New Location** | Informational alert when a user streams from a new geographic location. |

Each rule has configurable severity (Critical, Warning, Info) which determines the trust score impact. Rules can be linked to notification channels for automatic alerts.

**Household Locations:** StreamMon auto-detects household locations based on IP frequency (configurable via `HOUSEHOLD_AUTOLEARN_MIN_SESSIONS`). Household locations can also be managed manually. Streams from household IPs are exempt from geographic rules.

**Trust Scores:** Each user has a trust score (0-100) that decrements on violations. The decrement amount depends on severity: -20 for critical, -10 for warning, -5 for info. Trust score visibility for non-admin users is configurable in Settings > Users.

## Notifications

Rule violations can trigger notifications on multiple channels simultaneously. Channels are managed in Rules > Notifications.

| Channel | Configuration |
|---------|---------------|
| **Discord** | Webhook URL. Sends rich embeds with color-coded severity. |
| **Webhook** | Custom URL, HTTP method, and optional headers. Sends full violation details as JSON. |
| **Pushover** | API token and user key. Maps severity to Pushover priority levels. |
| **Ntfy** | Server URL, topic, and optional bearer token. Maps severity to ntfy priority levels. |

Each notification channel can be linked to specific rules and individually enabled or disabled. A test function is available to verify delivery.

## Library Maintenance

Automate cleanup of unwatched or low-quality media. Maintenance rules are managed per-library from the **Libraries** page.

| Criterion | Description |
|-----------|-------------|
| **Unwatched Movies** | Movies added more than N days ago that have never been watched. |
| **Zero Episodes Watched** | TV shows with no episodes watched in more than N days. |
| **Low Watch Percentage** | TV shows with less than X% of episodes watched after N days. |
| **Low Resolution** | Items at or below a specified resolution (SD, 720p, 1080p, 4K). |
| **Large Files** | Items exceeding a specified file size in GB. |

Maintenance rules are evaluated daily at 3 AM (or manually on demand). Matched items appear as candidates for review before deletion. Individual items can be excluded from future evaluation.

## Tautulli Import

StreamMon can import historical watch data from Tautulli. Configure the Tautulli URL and API key in **Settings > Import** and run the import. Data is mapped to the appropriate StreamMon media server.

## Reverse Proxy

StreamMon works behind a reverse proxy. Set `CORS_ORIGIN` to your public domain if the frontend is served from a different origin:

```yaml
environment:
  - CORS_ORIGIN=https://streammon.example.com
```

StreamMon respects `X-Forwarded-For` headers for accurate IP geolocation behind a proxy.

## Development

**Prerequisites:** Go 1.24+, Node.js 20+, SQLite3

```bash
make dev-backend    # Run Go backend
make dev-frontend   # Run Vite dev server with HMR
make build          # Production build
make test           # Run all Go and frontend tests
make docker         # Build Docker image
```

The frontend dev server proxies API requests to the backend on port 7935.

## Tech Stack

- **Backend:** Go, Chi router, SQLite (WAL mode), SSE
- **Frontend:** React 18, TypeScript, Vite, Tailwind CSS, Recharts, Leaflet
- **Auth:** Local, Plex, Emby, Jellyfin, OIDC
- **Deployment:** Docker Compose, multi-stage build (Node > Go > Alpine)

## Acknowledgments

This project has been developed with vibe coding and AI assistance using [Claude Code](https://claude.ai/claude-code). The codebase includes clean, well-documented code with proper error handling, comprehensive testing considerations, modern async/await patterns, robust database design, and production-ready deployment configurations.

Thanks to [Tautulli](https://tautulli.com/), [Overseerr](https://overseerr.dev/), and [Tracearr](https://www.tracearr.com/) for their amazing work and inspiration.
