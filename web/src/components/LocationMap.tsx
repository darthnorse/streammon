import { useFetch } from '../hooks/useFetch'
import type { GeoResult } from '../types'
import { LeafletMap } from './shared/LeafletMap'
import { LocationTable, type LocationColumn } from './shared/LocationTable'
import { formatLocation } from '../lib/format'
import { MS_PER_MINUTE, MS_PER_HOUR, MS_PER_DAY } from '../lib/constants'

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

function getLocationColor(loc: GeoResult): string {
  return isRecentLocation(loc.last_seen) ? COLOR_RECENT : COLOR_OLD
}

const columns: LocationColumn[] = [
  {
    header: 'IP Address',
    accessor: (loc) => loc.ip,
    className: 'font-mono text-xs',
  },
  {
    header: 'Location',
    accessor: (loc) => formatLocation(loc, '—'),
  },
  {
    header: 'Last Seen',
    accessor: (loc) => loc.last_seen ? formatLastSeen(loc.last_seen) : '—',
    className: 'text-muted dark:text-muted-dark',
  },
]

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
          <div className="text-3xl mb-2 opacity-30">&#9678;</div>
          <p className="text-muted dark:text-muted-dark text-sm">No location data available</p>
        </div>
      </div>
    )
  }

  return (
    <div className="space-y-6">
      <div className="rounded-lg overflow-hidden border border-border dark:border-border-dark relative z-0">
        <LeafletMap
          locations={data}
          viewMode="markers"
          height="250px"
          markerColor={getLocationColor}
        />
      </div>
      <LocationTable
        locations={data}
        columns={columns}
        rowKey={(loc, idx) => `${loc.ip}-${idx}`}
      />
    </div>
  )
}
