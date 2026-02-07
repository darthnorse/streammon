import { api } from './api'

export type UnitSystem = 'metric' | 'imperial'

const STORAGE_KEY = 'streammon:units'

interface DisplaySettings {
  unit_system: UnitSystem
}

let initialized = false
let initPromise: Promise<void> | null = null

export function getUnitSystem(): UnitSystem {
  const stored = localStorage.getItem(STORAGE_KEY)
  return stored === 'imperial' ? 'imperial' : 'metric'
}

export async function initUnitSystem(): Promise<void> {
  if (initialized) return
  if (initPromise) return initPromise

  initPromise = (async () => {
    try {
      const settings = await api.get<DisplaySettings>('/api/settings/display')
      if (settings.unit_system) {
        localStorage.setItem(STORAGE_KEY, settings.unit_system)
        window.dispatchEvent(new CustomEvent('units-changed', { detail: settings.unit_system }))
      }
    } catch (err) {
      console.warn('Failed to fetch unit preference from server, using localStorage:', err)
    } finally {
      initialized = true
      initPromise = null
    }
  })()

  return initPromise
}

export async function setUnitSystem(system: UnitSystem): Promise<void> {
  localStorage.setItem(STORAGE_KEY, system)
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
