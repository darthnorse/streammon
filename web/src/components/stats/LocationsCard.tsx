import { ComposableMap, Geographies, Geography, Marker } from 'react-simple-maps'
import type { GeoResult } from '../../types'

const GEO_URL = 'https://cdn.jsdelivr.net/npm/world-atlas@2/countries-110m.json'

interface LocationsCardProps {
  locations: GeoResult[]
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

      {locations.map((loc, idx) => (
        <Marker key={`${loc.ip}-${idx}`} coordinates={[loc.lng, loc.lat]}>
          <circle r={6} fill="#3b82f6" fillOpacity={0.3} />
          <circle r={3} fill="#3b82f6" />
        </Marker>
      ))}
    </ComposableMap>
  )
}

function LocationTable({ locations }: { locations: GeoResult[] }) {
  return (
    <div className="overflow-x-auto mt-4">
      <table className="w-full text-sm">
        <thead>
          <tr className="border-b border-border dark:border-border-dark text-left text-muted dark:text-muted-dark">
            <th className="py-2 pr-4 font-medium">IP Address</th>
            <th className="py-2 font-medium">Location</th>
          </tr>
        </thead>
        <tbody>
          {locations.map((loc, idx) => (
            <tr key={`${loc.ip}-${idx}`} className="border-b border-border/50 dark:border-border-dark/50">
              <td className="py-2 pr-4 font-mono text-xs">{loc.ip}</td>
              <td className="py-2">
                {loc.city && loc.country ? `${loc.city}, ${loc.country}` : loc.country || '—'}
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
