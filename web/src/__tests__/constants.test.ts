import { describe, it, expect } from 'vitest'
import { visibleNavLinks } from '../lib/constants'
import type { IntegrationStatus } from '../lib/constants'

describe('visibleNavLinks', () => {
  const both: IntegrationStatus = { sonarr: true, overseerr: true }
  const none: IntegrationStatus = { sonarr: false, overseerr: false }

  describe('admin', () => {
    it('shows Calendar and Requests when integrations configured', () => {
      const labels = visibleNavLinks('admin', both).map(l => l.label)
      expect(labels).toContain('Calendar')
      expect(labels).toContain('Requests')
    })

    it('hides Calendar when Sonarr unconfigured', () => {
      const labels = visibleNavLinks('admin', { sonarr: false, overseerr: true }).map(l => l.label)
      expect(labels).not.toContain('Calendar')
      expect(labels).toContain('Requests')
    })

    it('hides Requests when Seerr unconfigured', () => {
      const labels = visibleNavLinks('admin', { sonarr: true, overseerr: false }).map(l => l.label)
      expect(labels).toContain('Calendar')
      expect(labels).not.toContain('Requests')
    })

    it('hides both when neither configured', () => {
      const labels = visibleNavLinks('admin', none).map(l => l.label)
      expect(labels).not.toContain('Calendar')
      expect(labels).not.toContain('Requests')
    })
  })

  describe('viewer', () => {
    it('shows Calendar and Requests when integrations configured', () => {
      const labels = visibleNavLinks('viewer', both).map(l => l.label)
      expect(labels).toContain('Calendar')
      expect(labels).toContain('Requests')
    })

    it('hides Calendar when Sonarr unconfigured', () => {
      const labels = visibleNavLinks('viewer', { sonarr: false, overseerr: true }).map(l => l.label)
      expect(labels).not.toContain('Calendar')
    })

    it('hides Requests when Seerr unconfigured', () => {
      const labels = visibleNavLinks('viewer', { sonarr: true, overseerr: false }).map(l => l.label)
      expect(labels).not.toContain('Requests')
    })
  })

  describe('no integrations provided', () => {
    it('includes integration links when status unknown', () => {
      const labels = visibleNavLinks('admin').map(l => l.label)
      expect(labels).toContain('Calendar')
      expect(labels).toContain('Requests')
    })
  })
})
