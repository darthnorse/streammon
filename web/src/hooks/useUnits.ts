import { useState, useEffect, useCallback } from 'react'
import {
  type UnitSystem,
  getUnitSystem,
  setUnitSystem as setUnitSystemAsync,
  initUnitSystem,
  formatDistance,
  formatSpeed,
  getDistanceUnit,
  getSpeedUnit,
  toKm,
  fromKm,
  toKmh,
  fromKmh,
} from '../lib/units'

export function useUnits() {
  const [system, setSystem] = useState<UnitSystem>(getUnitSystem)

  useEffect(() => {
    initUnitSystem()

    const handleChange = (e: Event) => {
      const customEvent = e as CustomEvent<UnitSystem>
      setSystem(customEvent.detail)
    }
    window.addEventListener('units-changed', handleChange)
    return () => window.removeEventListener('units-changed', handleChange)
  }, [])

  const updateSystem = useCallback((newSystem: UnitSystem) => {
    setSystem(newSystem)
    setUnitSystemAsync(newSystem)
  }, [])

  return {
    system,
    setSystem: updateSystem,
    isMetric: system === 'metric',
    isImperial: system === 'imperial',
    formatDistance: (km: number) => formatDistance(km, system),
    formatSpeed: (kmh: number) => formatSpeed(kmh, system),
    distanceUnit: getDistanceUnit(system),
    speedUnit: getSpeedUnit(system),
    toKm: (value: number) => toKm(value, system),
    fromKm: (km: number) => fromKm(km, system),
    toKmh: (value: number) => toKmh(value, system),
    fromKmh: (kmh: number) => fromKmh(kmh, system),
  }
}
