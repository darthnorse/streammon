import { describe, it, expect } from 'vitest'
import { formatTimestamp, formatDuration, formatDate, formatBitrate, formatChannels } from '../lib/format'

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
