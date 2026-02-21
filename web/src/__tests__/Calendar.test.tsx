import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { renderWithRouter, makeAuthContext, makeMockApiGet } from '../test-utils'
import { Calendar } from '../pages/Calendar'
import type { SonarrEpisode } from '../types'

vi.mock('../lib/api', () => ({
  api: {
    get: vi.fn(),
    post: vi.fn(),
    put: vi.fn(),
    del: vi.fn(),
  },
}))

vi.mock('../context/AuthContext', () => ({
  useAuth: vi.fn(),
}))

vi.mock('../components/TMDBDetailModal', () => ({
  TMDBDetailModal: ({ mediaType, mediaId, overseerrConfigured, onClose }: { mediaType: string; mediaId: number; overseerrConfigured: boolean; onClose: () => void }) => (
    <div data-testid="tmdb-modal" data-media-type={mediaType} data-media-id={mediaId} data-overseerr-configured={String(overseerrConfigured)}>
      <button onClick={onClose}>Close</button>
    </div>
  ),
}))

vi.mock('../components/PersonModal', () => ({
  PersonModal: () => null,
}))

import { api } from '../lib/api'
import { useAuth } from '../context/AuthContext'

const mockApi = vi.mocked(api)
const mockUseAuth = vi.mocked(useAuth)
const mockApiGet = makeMockApiGet(mockApi)

function makeEpisode(overrides: Partial<SonarrEpisode> = {}): SonarrEpisode {
  return {
    id: 1,
    seriesId: 10,
    seasonNumber: 1,
    episodeNumber: 5,
    title: 'Test Episode',
    airDateUtc: new Date().toISOString(),
    airDate: new Date().toISOString().slice(0, 10),
    hasFile: false,
    monitored: true,
    series: {
      id: 10,
      title: 'Test Series',
      tmdbId: 42,
      year: new Date().getFullYear(),
      network: 'HBO',
    },
    ...overrides,
  }
}

describe('Calendar', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mockUseAuth.mockReturnValue(makeAuthContext('admin'))
  })

  afterEach(() => {
    vi.restoreAllMocks()
  })
  describe('when Sonarr is not configured', () => {
    it('shows not-configured message for admin with settings hint', async () => {
      mockApiGet({ '/api/sonarr/configured': { configured: false } })
      renderWithRouter(<Calendar />)

      await waitFor(() => {
        expect(screen.getByText('Sonarr Not Configured')).toBeDefined()
      })
      expect(screen.getByText(/configure sonarr in settings/i)).toBeDefined()
    })

    it('shows not-configured message for viewer with ask-admin hint', async () => {
      mockUseAuth.mockReturnValue(makeAuthContext('viewer'))
      mockApiGet({ '/api/sonarr/configured': { configured: false } })
      renderWithRouter(<Calendar />)

      await waitFor(() => {
        expect(screen.getByText('Sonarr Not Configured')).toBeDefined()
      })
      expect(screen.getByText(/ask an admin/i)).toBeDefined()
    })

    it('does not fetch overseerr configured status', async () => {
      mockApiGet({ '/api/sonarr/configured': { configured: false } })
      renderWithRouter(<Calendar />)

      await waitFor(() => {
        expect(screen.getByText('Sonarr Not Configured')).toBeDefined()
      })
      const calls = mockApi.get.mock.calls.map(c => c[0] as string)
      expect(calls.some(u => u.includes('/api/overseerr/'))).toBe(false)
    })
  })

  describe('series detail modal', () => {
    it('cards are always clickable even when Seerr is not configured', async () => {
      const ep = makeEpisode()
      mockApiGet({
        '/api/sonarr/configured': { configured: true },
        '/api/overseerr/configured': { configured: false },
        '/api/sonarr/calendar': [ep],
      })
      renderWithRouter(<Calendar />)

      await waitFor(() => {
        expect(screen.getByText('Test Series')).toBeDefined()
      })
      expect(screen.getByRole('button', { name: /view details for test series/i })).toBeDefined()
    })

    it('cards are clickable when Seerr is configured and tmdbId present', async () => {
      const ep = makeEpisode()
      mockApiGet({
        '/api/sonarr/configured': { configured: true },
        '/api/overseerr/configured': { configured: true },
        '/api/sonarr/calendar': [ep],
      })
      renderWithRouter(<Calendar />)

      await waitFor(() => {
        expect(screen.getByRole('button', { name: /view details for test series/i })).toBeDefined()
      })
    })

    it('does not render as clickable when tmdbId is absent', async () => {
      const ep = makeEpisode({ series: { id: 10, title: 'No TMDB', network: 'FOX' } })
      mockApiGet({
        '/api/sonarr/configured': { configured: true },
        '/api/overseerr/configured': { configured: true },
        '/api/sonarr/calendar': [ep],
      })
      renderWithRouter(<Calendar />)

      await waitFor(() => {
        expect(screen.getByText('No TMDB')).toBeDefined()
      })

      expect(screen.queryByRole('button', { name: /view details for no tmdb/i })).toBeNull()
    })

    it('opens TMDBDetailModal with tmdbId as TV', async () => {
      const ep = makeEpisode()
      mockApiGet({
        '/api/sonarr/configured': { configured: true },
        '/api/overseerr/configured': { configured: true },
        '/api/sonarr/calendar': [ep],
      })
      renderWithRouter(<Calendar />)

      await waitFor(() => {
        expect(screen.getByRole('button', { name: /view details for test series/i })).toBeDefined()
      })

      await userEvent.click(screen.getByRole('button', { name: /view details for test series/i }))

      const modal = screen.getByTestId('tmdb-modal')
      expect(modal).toBeDefined()
      expect(modal.getAttribute('data-media-type')).toBe('tv')
      expect(modal.getAttribute('data-media-id')).toBe('42')
      expect(modal.getAttribute('data-overseerr-configured')).toBe('true')
    })

    it('closes modal when onClose is called', async () => {
      const ep = makeEpisode()
      mockApiGet({
        '/api/sonarr/configured': { configured: true },
        '/api/overseerr/configured': { configured: true },
        '/api/sonarr/calendar': [ep],
      })
      renderWithRouter(<Calendar />)

      await waitFor(() => {
        expect(screen.getByRole('button', { name: /view details for test series/i })).toBeDefined()
      })

      await userEvent.click(screen.getByRole('button', { name: /view details for test series/i }))
      expect(screen.getByTestId('tmdb-modal')).toBeDefined()

      await userEvent.click(screen.getByText('Close'))
      expect(screen.queryByTestId('tmdb-modal')).toBeNull()
    })
  })

  describe('view switching', () => {
    it('defaults to week view', async () => {
      localStorage.removeItem('streammon:calendar-view')
      mockApiGet({
        '/api/sonarr/configured': { configured: true },
        '/api/overseerr/configured': { configured: false },
        '/api/sonarr/calendar': [],
      })
      renderWithRouter(<Calendar />)

      await waitFor(() => {
        expect(screen.getByText('No episodes scheduled')).toBeDefined()
      })
      expect(screen.getByText(/nothing airing this week/i)).toBeDefined()
    })

    it('switches to month view', async () => {
      localStorage.removeItem('streammon:calendar-view')
      mockApiGet({
        '/api/sonarr/configured': { configured: true },
        '/api/overseerr/configured': { configured: false },
        '/api/sonarr/calendar': [],
      })
      renderWithRouter(<Calendar />)

      await waitFor(() => {
        expect(screen.getByText('No episodes scheduled')).toBeDefined()
      })

      await userEvent.click(screen.getByText('month'))

      await waitFor(() => {
        expect(screen.getByText(/nothing airing this month/i)).toBeDefined()
      })
      expect(localStorage.getItem('streammon:calendar-view')).toBe('month')
    })

    it('Today button resets offset', async () => {
      mockApiGet({
        '/api/sonarr/configured': { configured: true },
        '/api/overseerr/configured': { configured: false },
        '/api/sonarr/calendar': [],
      })
      renderWithRouter(<Calendar />)

      await waitFor(() => {
        expect(screen.getByText('Today')).toBeDefined()
      })

      const initialLabel = screen.getByRole('heading', { level: 2 }).textContent

      await userEvent.click(screen.getByText('\u2192'))
      await waitFor(() => {
        expect(screen.getByRole('heading', { level: 2 }).textContent).not.toBe(initialLabel)
      })

      await userEvent.click(screen.getByText('Today'))
      await waitFor(() => {
        expect(screen.getByRole('heading', { level: 2 }).textContent).toBe(initialLabel)
      })
    })
  })

  describe('error state', () => {
    it('shows error message when calendar API fails', async () => {
      mockApi.get.mockImplementation(((url: string) => {
        if (url.includes('/api/sonarr/calendar')) return Promise.reject(new Error('network error'))
        if (url.includes('/api/sonarr/configured')) return Promise.resolve({ configured: true })
        if (url.includes('/api/overseerr/configured')) return Promise.resolve({ configured: false })
        return Promise.resolve(null)
      }) as never)
      renderWithRouter(<Calendar />)

      await waitFor(() => {
        expect(screen.getByText('Failed to load calendar')).toBeDefined()
      })
    })
  })
})
