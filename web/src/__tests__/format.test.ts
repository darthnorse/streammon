import { describe, it, expect } from 'vitest'
import { formatTimestamp, formatDuration, formatDate } from '../lib/format'

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

describe('formatDate', () => {
  it('returns a formatted date string containing the year', () => {
    const result = formatDate('2024-06-15T12:00:00Z')
    expect(result).toContain('2024')
  })
})
