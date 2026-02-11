import { useState } from 'react'
import type { GeoResult, ViewMode } from '../../types'
import { LeafletMap } from '../shared/LeafletMap'
import { ViewModeToggle } from '../shared/ViewModeToggle'
import { LocationTable, type LocationColumn } from '../shared/LocationTable'
import { formatLocation } from '../../lib/format'

interface LocationsCardProps {
  locations: GeoResult[]
}

const columns: LocationColumn[] = [
  {
    header: 'Location',
    accessor: (loc) => formatLocation(loc, '—'),
  },
  {
    header: 'ISP',
    accessor: (loc) => loc.isp || '—',
    className: 'text-gray-600 dark:text-gray-400',
  },
  {
    header: 'Users',
    accessor: (loc) => loc.users?.join(', ') || '—',
    className: 'text-gray-600 dark:text-gray-400',
  },
]

export function LocationsCard({ locations }: LocationsCardProps) {
  const [viewMode, setViewMode] = useState<ViewMode>('heatmap')

  return (
    <div className="card p-4">
      <div className="flex items-center justify-between mb-4">
        <h2 className="text-lg font-semibold flex items-center gap-2">
          <span className="opacity-50">&#9678;</span>
          Watch Locations
          {locations.length > 0 && (
            <span className="text-sm font-normal text-muted dark:text-muted-dark">
              ({locations.length} locations)
            </span>
          )}
        </h2>
        {locations.length > 0 && (
          <ViewModeToggle viewMode={viewMode} onChange={setViewMode} />
        )}
      </div>

      {locations.length === 0 ? (
        <div className="text-center py-8 text-muted dark:text-muted-dark">
          No location data available
        </div>
      ) : (
        <div className="space-y-4">
          <div className="rounded-lg overflow-hidden border border-border dark:border-border-dark relative z-0">
            <LeafletMap locations={locations} viewMode={viewMode} height="350px" />
          </div>
          <LocationTable
            locations={locations}
            columns={columns}
            rowKey={(loc, idx) => `${loc.lat}-${loc.lng}-${idx}`}
            className="mt-4"
          />
        </div>
      )}
    </div>
  )
}
