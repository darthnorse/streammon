import { useEffect } from 'react'
import { MapContainer, TileLayer, Marker, Popup, useMap } from 'react-leaflet'
import L from 'leaflet'
import 'leaflet/dist/leaflet.css'
import markerIcon2x from 'leaflet/dist/images/marker-icon-2x.png'
import markerIcon from 'leaflet/dist/images/marker-icon.png'
import markerShadow from 'leaflet/dist/images/marker-shadow.png'
import { useFetch } from '../hooks/useFetch'
import { useIsDark } from '../hooks/useIsDark'
import type { GeoResult } from '../types'

const icon = new L.Icon({
  iconUrl: markerIcon,
  iconRetinaUrl: markerIcon2x,
  shadowUrl: markerShadow,
  iconSize: [25, 41],
  iconAnchor: [12, 41],
  popupAnchor: [1, -34],
  shadowSize: [41, 41],
})

const TILES = {
  dark: 'https://{s}.basemaps.cartocdn.com/dark_all/{z}/{x}/{y}{r}.png',
  light: 'https://{s}.basemaps.cartocdn.com/light_all/{z}/{x}/{y}{r}.png',
}

function FitBounds({ locations }: { locations: GeoResult[] }) {
  const map = useMap()
  useEffect(() => {
    if (locations.length === 0) return
    if (locations.length === 1) {
      map.setView([locations[0].lat, locations[0].lng], 6)
      return
    }
    const bounds = L.latLngBounds(locations.map(l => [l.lat, l.lng]))
    map.fitBounds(bounds, { padding: [40, 40], maxZoom: 10 })
  }, [map, locations])
  return null
}

const placeholderClass = `h-[300px] md:h-[400px] rounded-lg bg-panel dark:bg-panel-dark
  border border-border dark:border-border-dark flex items-center justify-center`

interface LocationMapProps {
  userName: string
}

export function LocationMap({ userName }: LocationMapProps) {
  const isDark = useIsDark()
  const url = `/api/users/${encodeURIComponent(userName)}/locations`
  const { data, loading, error } = useFetch<GeoResult[]>(url)

  if (loading) {
    return (
      <div className={`${placeholderClass} text-muted dark:text-muted-dark text-sm`}>
        Loading map...
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
    <div className="h-[300px] md:h-[400px] rounded-lg overflow-hidden border border-border dark:border-border-dark">
      <MapContainer
        center={[20, 0]}
        zoom={2}
        className="h-full w-full"
        scrollWheelZoom={true}
      >
        <TileLayer
          key={isDark ? 'dark' : 'light'}
          attribution='&copy; <a href="https://www.openstreetmap.org/copyright">OSM</a>'
          url={isDark ? TILES.dark : TILES.light}
        />
        <FitBounds locations={data} />
        {data.map((loc, idx) => (
          <Marker key={`${loc.ip}-${idx}`} position={[loc.lat, loc.lng]} icon={icon}>
            <Popup>
              <div className="text-sm font-sans">
                <div className="font-semibold">{loc.city}, {loc.country}</div>
                <div className="font-mono text-xs text-gray-500 mt-0.5">{loc.ip}</div>
              </div>
            </Popup>
          </Marker>
        ))}
      </MapContainer>
    </div>
  )
}
