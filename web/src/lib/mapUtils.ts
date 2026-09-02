import { MS_PER_DAY } from './constants'
import type { GeoResult } from '../types'

export const COLOR_RECENT = '#f59e0b'
export const COLOR_DEFAULT = '#3b82f6'

export function getLocationColor(loc: GeoResult): string {
  if (!loc.last_seen) return COLOR_DEFAULT
  const elapsed = Date.now() - new Date(loc.last_seen).getTime()
  return elapsed < MS_PER_DAY ? COLOR_RECENT : COLOR_DEFAULT
}

// The OSM tile usage policy asks for the plain host — no {s} subdomain
// sharding — and for visible attribution including a way to report map issues.
export const DEFAULT_TILE_URL = 'https://tile.openstreetmap.org/{z}/{x}/{y}.png'

export const DEFAULT_TILE_ATTRIBUTION =
  '&copy; <a href="https://www.openstreetmap.org/copyright" target="_blank" rel="noreferrer">OpenStreetMap</a> contributors' +
  ' &middot; <a href="https://www.openstreetmap.org/fixthemap" target="_blank" rel="noreferrer">Report a map issue</a>'

// Order matters: greying out before the invert is what stops water turning
// teal and motorways salmon, which is what invert + hue-rotate leaves behind.
export const DARK_TILE_FILTER = 'grayscale(1) invert(1) brightness(0.8) contrast(1.1)'

const HTML_ESCAPES: Record<string, string> = {
  '&': '&amp;',
  '<': '&lt;',
  '>': '&gt;',
  '"': '&quot;',
  "'": '&#39;',
}

// Leaflet writes the attribution string into the DOM as HTML. The default
// credit is our own markup; anything an admin configures is plain text and
// gets escaped, so a stored attribution can never run script in a viewer's
// session (the API refuses angle brackets too).
export function tileAttributionHtml(custom: string): string {
  if (!custom) return DEFAULT_TILE_ATTRIBUTION
  return custom.replace(/[&<>"']/g, (ch) => HTML_ESCAPES[ch])
}

// Mirrors store.IsValidTileURL so the Settings form can explain what's wrong
// before the request, rather than surfacing a bare 400.
export function validateTileUrl(u: string): string | null {
  if (!u) return null
  let parsed: URL
  try {
    // {s} shards sit in the host and braces aren't legal there.
    parsed = new URL(u.replace(/\{s\}/g, 'a'))
  } catch {
    return 'Enter a full tile URL, e.g. https://tile.openstreetmap.org/{z}/{x}/{y}.png'
  }
  if (parsed.protocol !== 'https:') {
    return 'Tile URL must use https'
  }
  const missing = ['{z}', '{x}', '{y}'].filter((p) => !u.includes(p))
  if (missing.length > 0) {
    return `Tile URL must contain ${missing.join(', ')}`
  }
  return null
}

export function validateTileAttribution(a: string): string | null {
  if (/[<>]/.test(a)) return 'Attribution must be plain text — no HTML tags'
  return null
}
