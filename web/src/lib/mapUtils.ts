import { MS_PER_DAY } from './constants'
import type { GeoResult } from '../types'

export const COLOR_RECENT = '#f59e0b'
export const COLOR_DEFAULT = '#3b82f6'

export function getLocationColor(loc: GeoResult): string {
  if (!loc.last_seen) return COLOR_DEFAULT
  const elapsed = Date.now() - new Date(loc.last_seen).getTime()
  return elapsed < MS_PER_DAY ? COLOR_RECENT : COLOR_DEFAULT
}
