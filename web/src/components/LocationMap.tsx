import { ComposableMap, Geographies, Geography, Marker } from 'react-simple-maps'
import { useFetch } from '../hooks/useFetch'
import type { GeoResult } from '../types'

const GEO_URL = 'https://cdn.jsdelivr.net/npm/world-atlas@2/countries-110m.json'

const MS_PER_MINUTE = 60_000
const MS_PER_HOUR = 3_600_000
const MS_PER_DAY = 86_400_000

const COLOR_RECENT = '#f59e0b'
const COLOR_OLD = '#3b82f6'

interface LocationMapProps {
  userName: string
}

function formatLastSeen(isoDate: string): string {
  const date = new Date(isoDate)
  if (isNaN(date.getTime())) return 'Unknown'

  const diffMs = Date.now() - date.getTime()
  const diffMins = Math.floor(diffMs / MS_PER_MINUTE)
  const diffHours = Math.floor(diffMs / MS_PER_HOUR)
  const diffDays = Math.floor(diffMs / MS_PER_DAY)

  if (diffMins < 1) return 'Just now'
  if (diffMins < 60) return `${diffMins}m ago`
  if (diffHours < 24) return `${diffHours}h ago`
  if (diffDays < 7) return `${diffDays}d ago`
  return date.toLocaleDateString()
}

function isRecentLocation(lastSeen: string | undefined): boolean {
  if (!lastSeen) return false
  return Date.now() - new Date(lastSeen).getTime() < MS_PER_DAY
}

function locationKey(loc: GeoResult, idx: number): string {
  return `${loc.ip}-${idx}`
}

function WorldMap({ locations }: { locations: GeoResult[] }) {
  return (
    <ComposableMap
      projection="geoMercator"
      projectionConfig={{ scale: 120, center: [0, 30] }}
      style={{ width: '100%', height: 'auto' }}
    >
      <Geographies geography={GEO_URL}>
        {({ geographies }) =>
          geographies.map((geo) => (
            <Geography
              key={geo.rsmKey}
              geography={geo}
              fill="#e5e7eb"
              stroke="#d1d5db"
              strokeWidth={0.5}
              className="dark:fill-slate-700 dark:stroke-slate-600 outline-none"
              style={{
                default: { outline: 'none' },
                hover: { outline: 'none', fill: '#d1d5db' },
                pressed: { outline: 'none' },
              }}
            />
          ))
        }
      </Geographies>

      {locations.map((loc, idx) => {
        const color = isRecentLocation(loc.last_seen) ? COLOR_RECENT : COLOR_OLD
        return (
          <Marker key={locationKey(loc, idx)} coordinates={[loc.lng, loc.lat]}>
            <circle r={6} fill={color} fillOpacity={0.3} />
            <circle r={3} fill={color} />
          </Marker>
        )
      })}
    </ComposableMap>
  )
}

function LocationTable({ locations }: { locations: GeoResult[] }) {
  return (
    <div className="overflow-x-auto">
      <table className="w-full text-sm">
        <thead>
          <tr className="border-b border-border dark:border-border-dark text-left text-muted dark:text-muted-dark">
            <th className="py-2 pr-4 font-medium">IP Address</th>
            <th className="py-2 pr-4 font-medium">Location</th>
            <th className="py-2 font-medium">Last Seen</th>
          </tr>
        </thead>
        <tbody>
          {locations.map((loc, idx) => (
            <tr key={locationKey(loc, idx)} className="border-b border-border/50 dark:border-border-dark/50">
              <td className="py-2 pr-4 font-mono text-xs">{loc.ip}</td>
              <td className="py-2 pr-4">
                {loc.city}, {loc.country}
              </td>
              <td className="py-2 text-muted dark:text-muted-dark">
                {loc.last_seen ? formatLastSeen(loc.last_seen) : '—'}
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  )
}

const placeholderClass = `h-[200px] rounded-lg bg-panel dark:bg-panel-dark
  border border-border dark:border-border-dark flex items-center justify-center`

export function LocationMap({ userName }: LocationMapProps) {
  const url = `/api/users/${encodeURIComponent(userName)}/locations`
  const { data, loading, error } = useFetch<GeoResult[]>(url)

  if (loading) {
    return (
      <div className={`${placeholderClass} text-muted dark:text-muted-dark text-sm`}>
        Loading locations...
      </div>
    )
  }

  if (error) {
    return (
      <div className={`${placeholderClass} text-red-500 text-sm`}>
        Failed to load locations
      </div>
    )
  }

  if (!data || data.length === 0) {
    return (
      <div className={placeholderClass}>
        <div className="text-center">
          <div className="text-3xl mb-2 opacity-30">◎</div>
          <p className="text-muted dark:text-muted-dark text-sm">No location data available</p>
        </div>
      </div>
    )
  }

  return (
    <div className="space-y-6">
      <div className="rounded-lg overflow-hidden border border-border dark:border-border-dark bg-slate-50 dark:bg-slate-900">
        <WorldMap locations={data} />
      </div>
      <LocationTable locations={data} />
    </div>
  )
}
