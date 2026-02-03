import { ComposableMap, Geographies, Geography, Marker } from 'react-simple-maps'
import type { GeoResult } from '../../types'

export const GEO_URL = 'https://cdn.jsdelivr.net/npm/world-atlas@2/countries-110m.json'

interface MarkerRenderProps {
  location: GeoResult
  index: number
}

interface WorldMapBaseProps {
  locations: GeoResult[]
  renderMarker: (props: MarkerRenderProps) => React.ReactNode
  markerKey?: (loc: GeoResult, idx: number) => string
}

export function WorldMapBase({ locations, renderMarker, markerKey }: WorldMapBaseProps) {
  const getKey = markerKey ?? ((loc, idx) => `${loc.lat}-${loc.lng}-${idx}`)

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
        <Marker key={getKey(loc, idx)} coordinates={[loc.lng, loc.lat]}>
          {renderMarker({ location: loc, index: idx })}
        </Marker>
      ))}
    </ComposableMap>
  )
}
