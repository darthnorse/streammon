import { describe, it, expect } from 'vitest'
import { visibleNavLinks } from '../lib/constants'
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
    it('includes integration links when status unknown', () => {
      const labels = visibleNavLinks('admin').map(l => l.label)
      expect(labels).toContain('Calendar')
      expect(labels).toContain('Discover')
    })
  })
})
