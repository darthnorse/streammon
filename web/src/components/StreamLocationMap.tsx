import { useEffect, useState, useRef, useMemo, memo } from 'react'
import { LeafletMap } from './shared/LeafletMap'
import { api } from '../lib/api'
import type { ActiveStream, GeoResult, ServerType } from '../types'

interface StreamLocationMapProps {
  sessions: ActiveStream[]
}

interface StreamGeoResult extends GeoResult {
  streams: ActiveStream[]
  serverType?: ServerType
}

const SERVER_COLORS: Record<ServerType, string> = {
  plex: '#ffab00',
  emby: '#4caf50',
  jellyfin: '#aa5cc3',
}

function StreamLocationMapComponent({ sessions }: StreamLocationMapProps) {
  // Geo coordinates are cached per IP (network lookup) and only need
  // refetching when a *new* IP shows up. `geoCacheRef` is the synchronous
  // source of truth used to dedupe fetches; `geoCache` state mirrors it so
  // that a change to the cache re-triggers the `locations` memo below.
  const geoCacheRef = useRef<Map<string, GeoResult>>(new Map())
  const [geoCache, setGeoCache] = useState<Map<string, GeoResult>>(new Map())

  // The 1Hz progress interpolation gives `sessions` a new array reference
  // every tick even when the set of IPs hasn't changed. Derive a stable key
  // from just the distinct IPs so the geo lookup below only re-fires when
  // there's actually something new to resolve.
  const ipKey = useMemo(
    () => [...new Set(sessions.map(s => s.ip_address).filter(Boolean))].sort().join('|'),
    [sessions]
  )

  useEffect(() => {
    const uniqueIPs = ipKey === '' ? [] : ipKey.split('|')

    if (uniqueIPs.length === 0) {
      return
    }

    let ignore = false

    async function fetchGeo() {
      const newEntries = new Map<string, GeoResult>()

      for (const ip of uniqueIPs) {
        if (ignore) break
        // Already cached: nothing to fetch for this IP.
        if (geoCacheRef.current.has(ip)) continue

        try {
          const geo = await api.get<GeoResult>(`/api/geoip/${encodeURIComponent(ip)}`)
          // A newer IP set (or unmount) superseded this run while the
          // request was in flight — stop applying/firing further lookups.
          if (ignore) break
          if (geo && geo.lat && geo.lng) {
            newEntries.set(ip, geo)
          }
        } catch {
          // Skip IPs that fail to resolve
        }
      }

      if (!ignore && newEntries.size > 0) {
        for (const [ip, geo] of newEntries) {
          geoCacheRef.current.set(ip, geo)
        }
        setGeoCache(new Map(geoCacheRef.current))
      }
    }

    fetchGeo()

    return () => {
      ignore = true
    }
    // `sessions` is intentionally omitted: `ipKey` is the stable proxy for
    // "which IPs need geo data," so we only want to re-fetch when it changes.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [ipKey])

  // Stream metadata (who/what is playing at each IP) is cheap and has no
  // network cost, so it's recomputed from the *current* `sessions` prop on
  // every render — unlike the geo lookup above, it must never go stale.
  const locations = useMemo(() => {
    const ipToStreams = new Map<string, ActiveStream[]>()
    for (const session of sessions) {
      if (!session.ip_address) continue
      const existing = ipToStreams.get(session.ip_address) || []
      existing.push(session)
      ipToStreams.set(session.ip_address, existing)
    }

    const results: StreamGeoResult[] = []
    for (const [ip, streams] of ipToStreams) {
      const geo = geoCache.get(ip)
      if (!geo) continue
      // Use the first stream's server type for the marker color
      results.push({
        ...geo,
        ip,
        users: streams.map(s => s.user_name),
        streams,
        serverType: streams[0]?.server_type,
      })
    }
    return results
  }, [sessions, geoCache])

  if (sessions.length === 0) {
    return null
  }

  // Custom popup content for streams
  const locationsWithPopupData = locations.map(loc => ({
    ...loc,
    // Add stream info to users array for popup display
    users: loc.streams.map(s => {
      const title = s.grandparent_title || s.title
      return `${s.user_name}: ${title}`
    }),
  }))

  const getMarkerColor = (loc: GeoResult) => {
    const streamLoc = locations.find(l => l.ip === loc.ip)
    const serverType = streamLoc?.serverType
    return serverType ? SERVER_COLORS[serverType] : SERVER_COLORS.plex
  }

  return (
    <div className="mt-6">
      <h3 className="text-sm font-medium text-muted dark:text-muted-dark mb-3">Stream Locations</h3>
      <div className="rounded-lg overflow-hidden border border-border dark:border-border-dark relative z-0">
        <LeafletMap
          locations={locationsWithPopupData}
          viewMode="markers"
          height="250px"
          markerColor={getMarkerColor}
        />
      </div>
    </div>
  )
}

export const StreamLocationMap = memo(StreamLocationMapComponent)
