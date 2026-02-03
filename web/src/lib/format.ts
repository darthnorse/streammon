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
