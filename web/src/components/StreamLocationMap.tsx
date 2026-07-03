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
  const [locations, setLocations] = useState<StreamGeoResult[]>([])
  const cacheRef = useRef<Map<string, GeoResult>>(new Map())

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
      setLocations([])
      return
    }

    let ignore = false

    async function fetchLocations() {
      const results: StreamGeoResult[] = []
      const ipToStreams = new Map<string, ActiveStream[]>()

      // Group streams by IP
      for (const session of sessions) {
        if (!session.ip_address) continue
        const existing = ipToStreams.get(session.ip_address) || []
        existing.push(session)
        ipToStreams.set(session.ip_address, existing)
      }

      // Fetch geo data for each unique IP
      for (const ip of uniqueIPs) {
        // Check cache first
        let geo = cacheRef.current.get(ip)

        if (!geo) {
          try {
            geo = await api.get<GeoResult>(`/api/geoip/${encodeURIComponent(ip)}`)
            if (geo && geo.lat && geo.lng) {
              cacheRef.current.set(ip, geo)
            }
          } catch {
            // Skip IPs that fail to resolve
            continue
          }
        }

        if (geo && geo.lat && geo.lng) {
          const streams = ipToStreams.get(ip) || []
          // Use the first stream's server type for the marker color
          const serverType = streams[0]?.server_type
          results.push({
            ...geo,
            ip,
            users: streams.map(s => s.user_name),
            streams,
            serverType,
          })
        }
      }

      // Skip applying results from a run that's been superseded by a newer
      // IP set (or the component unmounting) to avoid an out-of-order write.
      if (!ignore) {
        setLocations(results)
      }
    }

    fetchLocations()

    return () => {
      ignore = true
    }
    // `sessions` is intentionally omitted: `ipKey` is the stable proxy for
    // "which IPs need geo data," so we only want to re-fetch when it changes.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [ipKey])

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
