import { useEffect, useState, useRef } from 'react'
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

export function StreamLocationMap({ sessions }: StreamLocationMapProps) {
  const [locations, setLocations] = useState<StreamGeoResult[]>([])
  const cacheRef = useRef<Map<string, GeoResult>>(new Map())

  useEffect(() => {
    if (sessions.length === 0) {
      setLocations([])
      return
    }

    const uniqueIPs = [...new Set(sessions.map(s => s.ip_address).filter(Boolean))]

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

      setLocations(results)
    }

    fetchLocations()
  }, [sessions])

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
