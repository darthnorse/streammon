import { useState } from 'react'
import type { GeoResult } from '../../types'
import { WorldMapBase } from '../shared/WorldMapBase'
import { formatLocation } from '../../lib/geo'

const MAX_TOOLTIP_USERS = 5
const TOOLTIP_OFFSET_PX = 8

interface LocationsCardProps {
  locations: GeoResult[]
}

interface TooltipState {
  x: number
  y: number
  location: GeoResult
}

function WorldMap({ locations }: { locations: GeoResult[] }) {
  const [tooltip, setTooltip] = useState<TooltipState | null>(null)

  return (
    <div className="relative">
      <WorldMapBase
        locations={locations}
        renderMarker={({ location: loc }) => (
          <>
            <circle
              r={6}
              fill="#3b82f6"
              fillOpacity={0.3}
              style={{ cursor: 'pointer' }}
              onMouseEnter={(e) => {
                const rect = (e.target as SVGCircleElement).ownerSVGElement?.getBoundingClientRect()
                const circle = (e.target as SVGCircleElement).getBoundingClientRect()
                if (rect) {
                  setTooltip({
                    x: circle.left - rect.left + circle.width / 2,
                    y: circle.top - rect.top,
                    location: loc,
                  })
                }
              }}
              onMouseLeave={() => setTooltip(null)}
            />
            <circle r={3} fill="#3b82f6" style={{ pointerEvents: 'none' }} />
          </>
        )}
      />

      {tooltip && (
        <div
          className="absolute z-10 px-3 py-2 text-xs bg-gray-900 dark:bg-gray-800 text-white rounded-lg shadow-lg pointer-events-none transform -translate-x-1/2 -translate-y-full"
          style={{ left: tooltip.x, top: tooltip.y - TOOLTIP_OFFSET_PX }}
        >
          <div className="font-medium">{formatLocation(tooltip.location)}</div>
          {tooltip.location.isp && (
            <div className="text-gray-400">{tooltip.location.isp}</div>
          )}
          {tooltip.location.users && tooltip.location.users.length > 0 && (
            <div className="mt-1 text-gray-300">
              {tooltip.location.users.slice(0, MAX_TOOLTIP_USERS).join(', ')}
              {tooltip.location.users.length > MAX_TOOLTIP_USERS && ` +${tooltip.location.users.length - MAX_TOOLTIP_USERS} more`}
            </div>
          )}
          <div
            className="absolute left-1/2 -translate-x-1/2 top-full w-0 h-0 border-l-4 border-r-4 border-t-4 border-transparent border-t-gray-900 dark:border-t-gray-800"
          />
        </div>
      )}
    </div>
  )
}

function LocationTable({ locations }: { locations: GeoResult[] }) {
  return (
    <div className="overflow-x-auto mt-4">
      <table className="w-full text-sm">
        <thead>
          <tr className="border-b border-border dark:border-border-dark text-left text-muted dark:text-muted-dark">
            <th className="py-2 pr-4 font-medium">Location</th>
            <th className="py-2 pr-4 font-medium">ISP</th>
            <th className="py-2 font-medium">Users</th>
          </tr>
        </thead>
        <tbody>
          {locations.map((loc, idx) => (
            <tr key={`${loc.lat}-${loc.lng}-${idx}`} className="border-b border-border/50 dark:border-border-dark/50">
              <td className="py-2 pr-4">{formatLocation(loc, '—')}</td>
              <td className="py-2 pr-4 text-gray-600 dark:text-gray-400">{loc.isp || '—'}</td>
              <td className="py-2 text-gray-600 dark:text-gray-400">
                {loc.users?.join(', ') || '—'}
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  )
}

export function LocationsCard({ locations }: LocationsCardProps) {
  return (
    <div className="card p-4">
      <h2 className="text-lg font-semibold mb-4 flex items-center gap-2">
        <span className="opacity-50">◎</span>
        Watch Locations
        {locations.length > 0 && (
          <span className="text-sm font-normal text-muted dark:text-muted-dark">
            ({locations.length} locations)
          </span>
        )}
      </h2>

      {locations.length === 0 ? (
        <div className="text-center py-8 text-muted dark:text-muted-dark">
          No location data available
        </div>
      ) : (
        <div className="space-y-4">
          <div className="rounded-lg overflow-hidden border border-border dark:border-border-dark bg-slate-50 dark:bg-slate-900">
            <WorldMap locations={locations} />
          </div>
          <LocationTable locations={locations} />
        </div>
      )}
    </div>
  )
}
