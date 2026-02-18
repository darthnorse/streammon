/**
 * Format a number with locale-specific thousands separators
 */
export function formatCount(count: number): string {
  return count.toLocaleString()
}

/**
 * Format bytes into human-readable size (B, KB, MB, GB, TB, PB)
 */
export function formatSize(bytes: number): string {
  if (bytes === 0) return '-'
  const units = ['B', 'KB', 'MB', 'GB', 'TB', 'PB']
  const i = Math.floor(Math.log(bytes) / Math.log(1024))
  const value = bytes / Math.pow(1024, i)
  return `${value.toFixed(i > 1 ? 1 : 0)} ${units[i]}`
}

export function formatTimestamp(ms: number): string {
  const totalSec = Math.floor(ms / 1000)
  const h = Math.floor(totalSec / 3600)
  const m = Math.floor((totalSec % 3600) / 60)
  const s = totalSec % 60
  if (h > 0) return `${h}:${String(m).padStart(2, '0')}:${String(s).padStart(2, '0')}`
  return `${m}:${String(s).padStart(2, '0')}`
}

export function formatDuration(ms: number): string {
  const totalMin = Math.floor(ms / 60000)
  const h = Math.floor(totalMin / 60)
  const m = totalMin % 60
  if (h > 0) return `${h}h ${m}m`
  return `${m}m`
}

export function formatBitrate(bps: number): string {
  if (bps <= 0) return ''
  if (bps >= 1_000_000) return `${(bps / 1_000_000).toFixed(1)} Mbps`
  if (bps >= 1_000) return `${Math.round(bps / 1_000)} Kbps`
  return `${bps} bps`
}

export function formatChannels(ch: number): string {
  if (ch === 2) return 'Stereo'
  if (ch === 6) return '5.1'
  if (ch === 8) return '7.1'
  if (ch > 0) return `${ch}ch`
  return ''
}

export function formatDate(iso: string): string {
  return new Date(iso).toLocaleString(undefined, {
    month: 'short',
    day: 'numeric',
    year: 'numeric',
    hour: 'numeric',
    minute: '2-digit',
  })
}

export function formatRelativeTime(isoDate: string): string {
  const date = new Date(isoDate)
  const now = new Date()
  const diffMs = now.getTime() - date.getTime()
  const diffDays = Math.floor(diffMs / (1000 * 60 * 60 * 24))

  if (diffDays === 0) return 'Today'
  if (diffDays === 1) return 'Yesterday'
  if (diffDays < 7) return `${diffDays}d ago`
  if (diffDays < 30) return `${Math.floor(diffDays / 7)}w ago`
  if (diffDays < 365) return `${Math.floor(diffDays / 30)}mo ago`
  return `${Math.floor(diffDays / 365)}y ago`
}

export function formatHours(hours: number): string {
  if (hours === 0) return '0m'
  if (hours < 1) {
    const minutes = Math.round(hours * 60)
    return `${minutes}m`
  }
  if (hours >= 24) {
    const days = hours / 24
    return `${days.toFixed(1)}d`
  }
  return `${hours.toFixed(1)}h`
}

export function formatEpisode(season?: number, episode?: number): string {
  if (season == null && episode == null) return ''
  const s = season != null ? `S${season}` : ''
  const e = episode != null ? `E${episode}` : ''
  if (s && e) return `${s} · ${e}`
  return s || e
}

export function parseSeasonFromTitle(parentTitle: string): number | undefined {
  if (!parentTitle) return undefined
  const match = parentTitle.match(/^(?:Season|Temporada|Staffel|Saison|Stagione)\s+(\d+)$/i)
  if (match) {
    return parseInt(match[1], 10)
  }
  return undefined
}

export function formatAudioCodec(codec?: string, channels?: number): string {
  if (!codec) return ''
  const c = codec.toLowerCase()
  const ch = formatChannels(channels ?? 0)

  if (c.includes('truehd') && c.includes('atmos')) return `Dolby TrueHD Atmos${ch ? ` ${ch}` : ''}`
  if (c.includes('truehd')) return `Dolby TrueHD${ch ? ` ${ch}` : ''}`
  if (c.includes('atmos')) return `Dolby Atmos${ch ? ` ${ch}` : ''}`
  if (c === 'eac3' || c === 'dd+' || c.includes('ddp')) return `Dolby Digital+${ch ? ` ${ch}` : ''}`
  if (c === 'ac3' || c === 'dd') return `Dolby Digital${ch ? ` ${ch}` : ''}`
  if (c === 'dts-hd ma' || c.includes('dts-hd')) return `DTS-HD MA${ch ? ` ${ch}` : ''}`
  if (c.includes('dts:x')) return `DTS:X${ch ? ` ${ch}` : ''}`
  if (c.includes('dts')) return `DTS${ch ? ` ${ch}` : ''}`
  if (c === 'aac') return `AAC${ch ? ` ${ch}` : ''}`
  if (c === 'flac') return `FLAC${ch ? ` ${ch}` : ''}`
  if (c === 'opus') return `Opus${ch ? ` ${ch}` : ''}`

  return ch ? `${codec.toUpperCase()} ${ch}` : codec.toUpperCase()
}

export function formatVideoCodec(codec?: string, resolution?: string): string {
  if (!codec && !resolution) return ''
  const parts: string[] = []
  if (resolution) parts.push(resolution)
  if (codec) {
    const c = codec.toLowerCase()
    if (c === 'hevc' || c === 'h265') parts.push('HEVC')
    else if (c === 'h264' || c === 'avc') parts.push('H.264')
    else if (c === 'av1') parts.push('AV1')
    else if (c === 'vp9') parts.push('VP9')
    else parts.push(codec.toUpperCase())
  }
  return parts.join(' ')
}

interface LocationLike {
  city?: string
  country?: string
}

export function formatLocation(loc: LocationLike, fallback?: string): string
export function formatLocation(city: string | undefined, country: string | undefined, fallback?: string): string
export function formatLocation(
  cityOrLoc: string | undefined | LocationLike,
  countryOrFallback?: string,
  fallback = '—'
): string {
  let city: string | undefined
  let country: string | undefined
  let fb = fallback

  if (typeof cityOrLoc === 'object' && cityOrLoc !== null) {
    city = cityOrLoc.city
    country = cityOrLoc.country
    fb = countryOrFallback ?? fallback
  } else {
    city = cityOrLoc
    country = countryOrFallback
  }

  if (city && city.length > 0 && country && country.length > 0) {
    return `${city}, ${country}`
  }
  return (country && country.length > 0) ? country : fb
}

export function parseYMD(s: string): { year: number; month: number; day: number } {
  const [y, m, d] = s.split('-').map(Number)
  return { year: y, month: m - 1, day: d }
}

export function padDate(d: Date): string {
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`
}

export function localToday(): string {
  return padDate(new Date())
}

export function localDaysAgo(n: number): string {
  const d = new Date()
  d.setDate(d.getDate() - n)
  return padDate(d)
}
