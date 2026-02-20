import { describe, it, expect } from 'vitest'
import { visibleNavLinks } from '../lib/constants'
import type { IntegrationStatus } from '../lib/constants'

describe('visibleNavLinks', () => {
  const all: IntegrationStatus = { sonarr: true, overseerr: true, discover: true, profile: true }
  const none: IntegrationStatus = { sonarr: false, overseerr: false, discover: false, profile: true }

  describe('admin', () => {
    it('shows Calendar and Discover when integrations configured', () => {
      const labels = visibleNavLinks('admin', all).map(l => l.label)
      expect(labels).toContain('Calendar')
      expect(labels).toContain('Discover')
    })

    it('hides Calendar when Sonarr unconfigured', () => {
      const labels = visibleNavLinks('admin', { sonarr: false, overseerr: true, discover: true, profile: true }).map(l => l.label)
      expect(labels).not.toContain('Calendar')
      expect(labels).toContain('Discover')
    })

    it('hides Discover when discover disabled', () => {
      const labels = visibleNavLinks('admin', { sonarr: true, overseerr: false, discover: false, profile: true }).map(l => l.label)
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

    it('hides Calendar when Sonarr unconfigured', () => {
      const labels = visibleNavLinks('viewer', { sonarr: false, overseerr: true, discover: true, profile: true }).map(l => l.label)
      expect(labels).not.toContain('Calendar')
    })

    it('hides Discover when discover disabled', () => {
      const labels = visibleNavLinks('viewer', { sonarr: true, overseerr: false, discover: false, profile: true }).map(l => l.label)
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
