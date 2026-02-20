import { describe, it, expect } from 'vitest'
import { formatTimestamp, formatDuration, formatDate, formatShortDate, formatBitrate, formatChannels, formatEpisode, parseSeasonFromTitle, formatAudioCodec, formatVideoCodec, formatLocation, thumbUrl } from '../lib/format'

describe('formatTimestamp', () => {
  it('formats seconds only', () => {
    expect(formatTimestamp(45000)).toBe('0:45')
  })

  it('formats minutes and seconds', () => {
    expect(formatTimestamp(125000)).toBe('2:05')
  })

  it('formats hours, minutes, and seconds', () => {
    expect(formatTimestamp(3661000)).toBe('1:01:01')
  })

  it('handles zero', () => {
    expect(formatTimestamp(0)).toBe('0:00')
  })
})

describe('formatDuration', () => {
  it('formats minutes only', () => {
    expect(formatDuration(1500000)).toBe('25m')
  })

  it('formats hours and minutes', () => {
    expect(formatDuration(5400000)).toBe('1h 30m')
  })

  it('handles zero', () => {
    expect(formatDuration(0)).toBe('0m')
  })
})

describe('formatBitrate', () => {
  it('formats megabits', () => expect(formatBitrate(6_000_000)).toBe('6.0 Mbps'))
  it('formats fractional megabits', () => expect(formatBitrate(2_500_000)).toBe('2.5 Mbps'))
  it('formats kilobits', () => expect(formatBitrate(500_000)).toBe('500 Kbps'))
  it('returns empty for zero', () => expect(formatBitrate(0)).toBe(''))
})

describe('formatChannels', () => {
  it('formats stereo', () => expect(formatChannels(2)).toBe('Stereo'))
  it('formats 5.1', () => expect(formatChannels(6)).toBe('5.1'))
  it('formats 7.1', () => expect(formatChannels(8)).toBe('7.1'))
  it('formats other channels', () => expect(formatChannels(4)).toBe('4ch'))
  it('returns empty for zero', () => expect(formatChannels(0)).toBe(''))
})

describe('formatDate', () => {
  it('returns a formatted date string containing the year', () => {
    const result = formatDate('2024-06-15T12:00:00Z')
    expect(result).toContain('2024')
  })
})

describe('formatShortDate', () => {
  it('formats imperial MM/DD/YYYY', () => {
    expect(formatShortDate('2024-06-05T12:00:00Z', false)).toBe('06/05/2024')
  })
  it('formats metric DD/MM/YYYY', () => {
    expect(formatShortDate('2024-06-05T12:00:00Z', true)).toBe('05/06/2024')
  })
  it('pads single-digit day and month', () => {
    expect(formatShortDate('2024-01-03T12:00:00Z', false)).toBe('01/03/2024')
    expect(formatShortDate('2024-01-03T12:00:00Z', true)).toBe('03/01/2024')
  })
})

describe('formatEpisode', () => {
  it('formats season and episode', () => {
    expect(formatEpisode(5, 14)).toBe('S5 · E14')
  })
  it('formats season only', () => {
    expect(formatEpisode(3, undefined)).toBe('S3')
  })
  it('formats episode only', () => {
    expect(formatEpisode(undefined, 7)).toBe('E7')
  })
  it('returns empty for no values', () => {
    expect(formatEpisode(undefined, undefined)).toBe('')
  })
  it('handles season 0 (specials)', () => {
    expect(formatEpisode(0, 1)).toBe('S0 · E1')
  })
  it('handles episode 0', () => {
    expect(formatEpisode(1, 0)).toBe('S1 · E0')
  })
})

describe('parseSeasonFromTitle', () => {
  it('parses "Season N" format', () => {
    expect(parseSeasonFromTitle('Season 5')).toBe(5)
  })
  it('parses case-insensitive', () => {
    expect(parseSeasonFromTitle('SEASON 3')).toBe(3)
  })
  it('parses "Temporada N" (Spanish)', () => {
    expect(parseSeasonFromTitle('Temporada 2')).toBe(2)
  })
  it('parses "Staffel N" (German)', () => {
    expect(parseSeasonFromTitle('Staffel 4')).toBe(4)
  })
  it('parses "Saison N" (French)', () => {
    expect(parseSeasonFromTitle('Saison 1')).toBe(1)
  })
  it('parses "Stagione N" (Italian)', () => {
    expect(parseSeasonFromTitle('Stagione 6')).toBe(6)
  })
  it('returns undefined for empty string', () => {
    expect(parseSeasonFromTitle('')).toBeUndefined()
  })
  it('returns undefined for non-matching format', () => {
    expect(parseSeasonFromTitle('S5')).toBeUndefined()
  })
})

describe('formatAudioCodec', () => {
  it('formats Dolby TrueHD Atmos', () => {
    expect(formatAudioCodec('truehd atmos', 8)).toBe('Dolby TrueHD Atmos 7.1')
  })
  it('formats Dolby TrueHD', () => {
    expect(formatAudioCodec('truehd', 8)).toBe('Dolby TrueHD 7.1')
  })
  it('formats Dolby Digital Plus', () => {
    expect(formatAudioCodec('eac3', 6)).toBe('Dolby Digital+ 5.1')
  })
  it('formats Dolby Digital', () => {
    expect(formatAudioCodec('ac3', 6)).toBe('Dolby Digital 5.1')
  })
  it('formats DTS-HD MA', () => {
    expect(formatAudioCodec('dts-hd ma', 8)).toBe('DTS-HD MA 7.1')
  })
  it('formats DTS', () => {
    expect(formatAudioCodec('dts', 6)).toBe('DTS 5.1')
  })
  it('formats AAC', () => {
    expect(formatAudioCodec('aac', 2)).toBe('AAC Stereo')
  })
  it('returns empty for no codec', () => {
    expect(formatAudioCodec(undefined, 6)).toBe('')
  })
})

describe('formatVideoCodec', () => {
  it('formats HEVC with resolution', () => {
    expect(formatVideoCodec('hevc', '2160p')).toBe('2160p HEVC')
  })
  it('formats H.264 with resolution', () => {
    expect(formatVideoCodec('h264', '1080p')).toBe('1080p H.264')
  })
  it('formats resolution only', () => {
    expect(formatVideoCodec(undefined, '720p')).toBe('720p')
  })
  it('formats codec only', () => {
    expect(formatVideoCodec('av1', undefined)).toBe('AV1')
  })
  it('returns empty for no data', () => {
    expect(formatVideoCodec(undefined, undefined)).toBe('')
  })
})

describe('thumbUrl', () => {
  it('builds url from server id and path', () => {
    expect(thumbUrl(1, 'library/metadata/123/thumb/456')).toBe('/api/servers/1/thumb/library/metadata/123/thumb/456')
  })
  it('strips single leading slash', () => {
    expect(thumbUrl(2, '/library/metadata/123/thumb/456')).toBe('/api/servers/2/thumb/library/metadata/123/thumb/456')
  })
  it('strips multiple leading slashes', () => {
    expect(thumbUrl(3, '///foo')).toBe('/api/servers/3/thumb/foo')
  })
  it('handles numeric rating key', () => {
    expect(thumbUrl(1, '55555')).toBe('/api/servers/1/thumb/55555')
  })
  it('passes through http URLs directly', () => {
    expect(thumbUrl(1, 'http://image.tmdb.org/t/p/original/actor.jpg')).toBe('http://image.tmdb.org/t/p/original/actor.jpg')
  })
  it('passes through https URLs directly', () => {
    expect(thumbUrl(1, 'https://metadata-static.plex.tv/people/5d776/thumb.jpg')).toBe('https://metadata-static.plex.tv/people/5d776/thumb.jpg')
  })
})

describe('formatLocation', () => {
  it('formats city and country', () => {
    expect(formatLocation('New York', 'USA')).toBe('New York, USA')
  })
  it('returns country only when no city', () => {
    expect(formatLocation(undefined, 'USA')).toBe('USA')
  })
  it('returns default fallback when no data', () => {
    expect(formatLocation(undefined, undefined)).toBe('—')
  })
  it('returns custom fallback when provided', () => {
    expect(formatLocation(undefined, undefined, 'Unknown')).toBe('Unknown')
  })
  it('returns country when city is empty string', () => {
    expect(formatLocation('', 'Canada')).toBe('Canada')
  })
})
