import { useState } from 'react'
import type { GeoResult } from '../../types'
import { LeafletMap, type ViewMode } from '../shared/LeafletMap'
import { LocationTable, type LocationColumn } from '../shared/LocationTable'
import { formatLocation } from '../../lib/format'

interface LocationsCardProps {
  locations: GeoResult[]
}

function ViewModeToggle({ viewMode, onChange }: { viewMode: ViewMode; onChange: (mode: ViewMode) => void }) {
  return (
    <div className="flex gap-1 bg-gray-100 dark:bg-gray-800 rounded-md p-0.5" role="group" aria-label="Map view mode">
      <button
        onClick={() => onChange('heatmap')}
        aria-pressed={viewMode === 'heatmap'}
        className={`px-2 py-1 text-xs rounded transition-colors ${
          viewMode === 'heatmap'
            ? 'bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100 shadow-sm'
            : 'text-gray-600 dark:text-gray-400 hover:text-gray-900 dark:hover:text-gray-200'
        }`}
      >
        Heatmap
      </button>
      <button
        onClick={() => onChange('markers')}
        aria-pressed={viewMode === 'markers'}
        className={`px-2 py-1 text-xs rounded transition-colors ${
          viewMode === 'markers'
            ? 'bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100 shadow-sm'
            : 'text-gray-600 dark:text-gray-400 hover:text-gray-900 dark:hover:text-gray-200'
        }`}
      >
        Markers
      </button>
    </div>
  )
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
          <div className="rounded-lg overflow-hidden border border-border dark:border-border-dark">
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
