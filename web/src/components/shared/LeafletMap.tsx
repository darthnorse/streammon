import { Component, useEffect, useMemo } from 'react'
import { MapContainer, TileLayer, CircleMarker, Popup, useMap } from 'react-leaflet'
import { HeatmapLayer } from 'react-leaflet-heatmap-layer-v3'
import type { LatLngBoundsExpression } from 'leaflet'
import { useIsDark } from '../../hooks/useIsDark'
import { formatLocation } from '../../lib/format'
import type { GeoResult } from '../../types'

const TILE_URLS = {
  dark: 'https://{s}.basemaps.cartocdn.com/dark_all/{z}/{x}/{y}{r}.png',
  light: 'https://{s}.basemaps.cartocdn.com/light_all/{z}/{x}/{y}{r}.png',
}

const TILE_ATTRIBUTION = '&copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a> contributors &copy; <a href="https://carto.com/attributions">CARTO</a>'

const HEATMAP_CONFIG = {
  radius: 30,
  blur: 20,
  minOpacity: 0.4,
  maxZoom: 12,
}

const DEFAULT_CENTER: [number, number] = [30, 0]
const DEFAULT_ZOOM = 2
const MIN_ZOOM = 2
const MAX_ZOOM = 18

const MAX_POPUP_USERS = 5

export type ViewMode = 'heatmap' | 'markers'

interface LeafletMapProps {
  locations: GeoResult[]
  viewMode?: ViewMode
  height?: string
  markerColor?: string | ((loc: GeoResult) => string)
}

interface MapErrorBoundaryProps {
  children: React.ReactNode
  height: string
}

interface MapErrorBoundaryState {
  hasError: boolean
}

class MapErrorBoundary extends Component<MapErrorBoundaryProps, MapErrorBoundaryState> {
  state: MapErrorBoundaryState = { hasError: false }

  static getDerivedStateFromError(): MapErrorBoundaryState {
    return { hasError: true }
  }

  componentDidCatch(error: Error, info: React.ErrorInfo) {
    console.error('Map error:', error, info.componentStack)
  }

  render() {
    if (this.state.hasError) {
      return (
        <div
          className="flex items-center justify-center bg-gray-100 dark:bg-gray-800 text-muted dark:text-muted-dark"
          style={{ height: this.props.height }}
        >
          <div className="text-center">
            <div className="text-2xl mb-2 opacity-30">&#9678;</div>
            <p className="text-sm">Map failed to load</p>
          </div>
        </div>
      )
    }
    return this.props.children
  }
}

function MapBoundsUpdater({ locations }: { locations: GeoResult[] }) {
  const map = useMap()

  useEffect(() => {
    if (locations.length === 0) return

    if (locations.length === 1) {
      map.setView([locations[0].lat, locations[0].lng], 6)
      return
    }

    const bounds: LatLngBoundsExpression = locations.map((loc) => [loc.lat, loc.lng] as [number, number])
    map.fitBounds(bounds, { padding: [50, 50], maxZoom: 10 })
  }, [locations, map])

  return null
}

function LocationPopup({ location }: { location: GeoResult }) {
  return (
    <div className="text-xs min-w-[150px]">
      <div className="font-medium text-gray-900 dark:text-gray-100">
        {formatLocation(location)}
      </div>
      {location.isp && (
        <div className="text-gray-600 dark:text-gray-400 mt-1">
          {location.isp}
        </div>
      )}
      {location.ip && (
        <div className="text-gray-500 dark:text-gray-500 font-mono mt-1">
          {location.ip}
        </div>
      )}
      {location.users && location.users.length > 0 && (
        <div className="mt-2 pt-2 border-t border-gray-200 dark:border-gray-600">
          <div className="text-gray-500 dark:text-gray-400 mb-1">Users:</div>
          <div className="text-gray-700 dark:text-gray-300">
            {location.users.slice(0, MAX_POPUP_USERS).join(', ')}
            {location.users.length > MAX_POPUP_USERS && (
              <span className="text-gray-500"> +{location.users.length - MAX_POPUP_USERS} more</span>
            )}
          </div>
        </div>
      )}
    </div>
  )
}

function LeafletMapInner({
  locations,
  viewMode = 'heatmap',
  height = '300px',
  markerColor = '#3b82f6',
}: LeafletMapProps) {
  const isDark = useIsDark()
  const tileUrl = isDark ? TILE_URLS.dark : TILE_URLS.light

  const heatmapGradient = useMemo(() => ({
    0.2: isDark ? '#1e40af' : '#93c5fd',
    0.4: isDark ? '#3b82f6' : '#60a5fa',
    0.6: isDark ? '#f59e0b' : '#fbbf24',
    0.8: isDark ? '#f97316' : '#fb923c',
    1.0: isDark ? '#ef4444' : '#f87171',
  }), [isDark])

  const getMarkerColor = (loc: GeoResult) => {
    if (typeof markerColor === 'function') return markerColor(loc)
    return markerColor
  }

  return (
    <MapContainer
      center={DEFAULT_CENTER}
      zoom={DEFAULT_ZOOM}
      minZoom={MIN_ZOOM}
      maxZoom={MAX_ZOOM}
      style={{ height, width: '100%' }}
      scrollWheelZoom={true}
    >
      <TileLayer url={tileUrl} attribution={TILE_ATTRIBUTION} />
      <MapBoundsUpdater locations={locations} />

      {viewMode === 'heatmap' && locations.length > 0 && (
        <HeatmapLayer
          points={locations}
          latitudeExtractor={(loc: GeoResult) => loc.lat}
          longitudeExtractor={(loc: GeoResult) => loc.lng}
          intensityExtractor={(loc: GeoResult) => Math.log10((loc.users?.length || 1) + 1)}
          gradient={heatmapGradient}
          radius={HEATMAP_CONFIG.radius}
          blur={HEATMAP_CONFIG.blur}
          minOpacity={HEATMAP_CONFIG.minOpacity}
          maxZoom={HEATMAP_CONFIG.maxZoom}
        />
      )}

      {viewMode === 'markers' && locations.map((loc, idx) => {
        const color = getMarkerColor(loc)
        return (
          <CircleMarker
            key={`${loc.lat}-${loc.lng}-${loc.ip || idx}`}
            center={[loc.lat, loc.lng]}
            radius={8}
            pathOptions={{
              color: color,
              fillColor: color,
              fillOpacity: 0.6,
              weight: 2,
            }}
          >
            <Popup>
              <LocationPopup location={loc} />
            </Popup>
          </CircleMarker>
        )
      })}
    </MapContainer>
  )
}

export function LeafletMap(props: LeafletMapProps) {
  return (
    <MapErrorBoundary height={props.height || '300px'}>
      <LeafletMapInner {...props} />
    </MapErrorBoundary>
  )
}
