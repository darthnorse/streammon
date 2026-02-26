import { describe, it, expect } from 'vitest'
import { visibleNavLinks, getMediaLabel } from '../lib/constants'
import type { IntegrationStatus } from '../lib/constants'

describe('visibleNavLinks', () => {
  const all: IntegrationStatus = { sonarr: true, overseerr: true, discover: true, profile: true, calendar: true }
  const none: IntegrationStatus = { sonarr: false, overseerr: false, discover: false, profile: true, calendar: false }

  describe('admin', () => {
    it('shows Calendar and Discover when integrations configured', () => {
      const labels = visibleNavLinks('admin', all).map(l => l.label)
      expect(labels).toContain('Calendar')
      expect(labels).toContain('Discover')
    })

    it('hides Calendar when calendar disabled', () => {
      const labels = visibleNavLinks('admin', { ...all, calendar: false }).map(l => l.label)
      expect(labels).not.toContain('Calendar')
      expect(labels).toContain('Discover')
    })

    it('hides Discover when discover disabled', () => {
      const labels = visibleNavLinks('admin', { ...all, discover: false }).map(l => l.label)
      expect(labels).toContain('Calendar')
      expect(labels).not.toContain('Discover')
    })

    it('hides both when neither configured', () => {
      const labels = visibleNavLinks('admin', none).map(l => l.label)
      expect(labels).not.toContain('Calendar')
      expect(labels).not.toContain('Discover')
    })
  })

  describe('viewer', () => {
    it('shows Calendar, Discover, and My Stats when integrations configured', () => {
      const labels = visibleNavLinks('viewer', all).map(l => l.label)
      expect(labels).toContain('Calendar')
      expect(labels).toContain('Discover')
      expect(labels).toContain('My Stats')
    })

    it('hides My Stats when profile disabled', () => {
      const labels = visibleNavLinks('viewer', { ...all, profile: false }).map(l => l.label)
      expect(labels).not.toContain('My Stats')
    })

    it('hides Calendar when calendar disabled', () => {
      const labels = visibleNavLinks('viewer', { ...all, calendar: false }).map(l => l.label)
      expect(labels).not.toContain('Calendar')
    })

    it('hides Discover when discover disabled', () => {
      const labels = visibleNavLinks('viewer', { ...all, discover: false }).map(l => l.label)
      expect(labels).not.toContain('Discover')
    })
  })

  describe('no integrations provided', () => {
    it('hides integration links when status unknown', () => {
      const labels = visibleNavLinks('admin').map(l => l.label)
      expect(labels).not.toContain('Calendar')
      expect(labels).not.toContain('Discover')
    })
  })
})

describe('getMediaLabel', () => {
  it('returns extra type label when extra_type is set', () => {
    expect(getMediaLabel('movie', 'trailer')).toBe('Trailer')
    expect(getMediaLabel('movie', 'behind_the_scenes')).toBe('Behind The Scenes')
    expect(getMediaLabel('movie', 'deleted_scene')).toBe('Deleted Scene')
    expect(getMediaLabel('movie', 'featurette')).toBe('Featurette')
    expect(getMediaLabel('movie', 'interview')).toBe('Interview')
    expect(getMediaLabel('movie', 'short')).toBe('Short')
  })

  it('falls back to media type label without extra_type', () => {
    expect(getMediaLabel('movie')).toBe('Movie')
    expect(getMediaLabel('episode')).toBe('TV')
    expect(getMediaLabel('track')).toBe('Music')
  })

  it('falls back to media type when extra_type is undefined', () => {
    expect(getMediaLabel('movie', undefined)).toBe('Movie')
  })
})
