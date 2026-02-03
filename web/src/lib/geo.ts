import type { GeoResult } from '../types'

export function formatLocation(loc: GeoResult, fallback = 'Unknown'): string {
  if (loc.city && loc.country) return `${loc.city}, ${loc.country}`
  return loc.country || fallback
}
