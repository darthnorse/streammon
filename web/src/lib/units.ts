import { api } from './api'
import { DEFAULT_TILE_URL } from './mapUtils'

export type UnitSystem = 'metric' | 'imperial'

const UNITS_KEY = 'streammon:units'
const REGION_KEY = 'streammon:discover-region'
const MAP_TILE_URL_KEY = 'streammon:map-tile-url'
const MAP_TILE_ATTRIBUTION_KEY = 'streammon:map-tile-attribution'
const MAP_DARK_FILTER_KEY = 'streammon:map-dark-filter'

export interface MapSettings {
  tileUrl: string
  attribution: string
  darkFilter: boolean
}

interface DisplaySettings {
  unit_system: UnitSystem
  discover_region: string
  map_tile_url: string
  map_tile_attribution: string
  map_dark_filter: boolean
}

let initialized = false
let initPromise: Promise<void> | null = null

export function getUnitSystem(): UnitSystem {
  const stored = localStorage.getItem(UNITS_KEY)
  return stored === 'imperial' ? 'imperial' : 'metric'
}

export function getDiscoverRegion(): string {
  return localStorage.getItem(REGION_KEY) ?? ''
}

export function getMapSettings(): MapSettings {
  return {
    tileUrl: localStorage.getItem(MAP_TILE_URL_KEY) || DEFAULT_TILE_URL,
    attribution: localStorage.getItem(MAP_TILE_ATTRIBUTION_KEY) ?? '',
    darkFilter: localStorage.getItem(MAP_DARK_FILTER_KEY) !== 'false',
  }
}

// An empty tileUrl means "use the default": the cache and the change event
// always carry a usable URL so consumers never have to fall back themselves.
function cacheMapSettings(settings: MapSettings): void {
  settings = { ...settings, tileUrl: settings.tileUrl || DEFAULT_TILE_URL }
  localStorage.setItem(MAP_TILE_URL_KEY, settings.tileUrl)
  localStorage.setItem(MAP_TILE_ATTRIBUTION_KEY, settings.attribution)
  localStorage.setItem(MAP_DARK_FILTER_KEY, String(settings.darkFilter))
  window.dispatchEvent(new CustomEvent<MapSettings>('map-settings-changed', { detail: settings }))
}

export async function initDisplaySettings(): Promise<void> {
  if (initialized) return
  if (initPromise) return initPromise

  initPromise = (async () => {
    try {
      const settings = await api.get<DisplaySettings>('/api/settings/display')
      if (settings.unit_system) {
        localStorage.setItem(UNITS_KEY, settings.unit_system)
        window.dispatchEvent(new CustomEvent('units-changed', { detail: settings.unit_system }))
      }
      localStorage.setItem(REGION_KEY, settings.discover_region ?? '')
      window.dispatchEvent(new CustomEvent('discover-region-changed', { detail: settings.discover_region ?? '' }))
      cacheMapSettings({
        tileUrl: settings.map_tile_url || DEFAULT_TILE_URL,
        attribution: settings.map_tile_attribution ?? '',
        darkFilter: settings.map_dark_filter ?? true,
      })
    } catch (err) {
      console.warn('Failed to fetch display settings from server, using localStorage:', err)
    } finally {
      initialized = true
      initPromise = null
    }
  })()

  return initPromise
}

export async function setDiscoverRegion(region: string): Promise<void> {
  localStorage.setItem(REGION_KEY, region)
  window.dispatchEvent(new CustomEvent('discover-region-changed', { detail: region }))

  try {
    await api.put('/api/settings/display', { discover_region: region })
  } catch (err) {
    console.warn('Failed to save discover region to server:', err)
  }
}

// Only the changed fields are sent: the endpoint treats an absent field as
// "leave alone", so a partial update can't clobber a sibling setting.
export async function setMapSettings(update: Partial<MapSettings>): Promise<void> {
  cacheMapSettings({ ...getMapSettings(), ...update })

  const body: Record<string, string | boolean> = {}
  if (update.tileUrl !== undefined) body.map_tile_url = update.tileUrl
  if (update.attribution !== undefined) body.map_tile_attribution = update.attribution
  if (update.darkFilter !== undefined) body.map_dark_filter = update.darkFilter

  await api.put('/api/settings/display', body)
}

export async function setUnitSystem(system: UnitSystem): Promise<void> {
  localStorage.setItem(UNITS_KEY, system)
  window.dispatchEvent(new CustomEvent('units-changed', { detail: system }))

  try {
    await api.put<DisplaySettings>('/api/settings/display', { unit_system: system })
  } catch (err) {
    console.warn('Failed to save unit preference to server:', err)
  }
}

const KM_TO_MILES = 0.621371
const KMH_TO_MPH = 0.621371

export function formatDistance(km: number, system?: UnitSystem): string {
  const units = system ?? getUnitSystem()
  if (units === 'imperial') {
    const miles = Math.round(km * KM_TO_MILES)
    return `${miles} mi`
  }
  return `${Math.round(km)} km`
}

export function formatSpeed(kmh: number, system?: UnitSystem): string {
  const units = system ?? getUnitSystem()
  if (units === 'imperial') {
    const mph = Math.round(kmh * KMH_TO_MPH)
    return `${mph} mph`
  }
  return `${Math.round(kmh)} km/h`
}

export function getDistanceUnit(system?: UnitSystem): string {
  const units = system ?? getUnitSystem()
  return units === 'imperial' ? 'mi' : 'km'
}

export function getSpeedUnit(system?: UnitSystem): string {
  const units = system ?? getUnitSystem()
  return units === 'imperial' ? 'mph' : 'km/h'
}

export function toKm(value: number, system?: UnitSystem): number {
  const units = system ?? getUnitSystem()
  if (units === 'imperial') {
    return value / KM_TO_MILES
  }
  return value
}

export function fromKm(km: number, system?: UnitSystem): number {
  const units = system ?? getUnitSystem()
  if (units === 'imperial') {
    return Math.round(km * KM_TO_MILES)
  }
  return km
}

export function toKmh(value: number, system?: UnitSystem): number {
  const units = system ?? getUnitSystem()
  if (units === 'imperial') {
    return value / KMH_TO_MPH
  }
  return value
}

export function fromKmh(kmh: number, system?: UnitSystem): number {
  const units = system ?? getUnitSystem()
  if (units === 'imperial') {
    return Math.round(kmh * KMH_TO_MPH)
  }
  return kmh
}
