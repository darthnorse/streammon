import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { screen, waitFor } from '@testing-library/react'
import { renderWithRouter } from '../test-utils'
import { Requests } from '../pages/Requests'

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

import { api } from '../lib/api'
import { useAuth } from '../context/AuthContext'

const mockApi = vi.mocked(api)
const mockUseAuth = vi.mocked(useAuth)

function mockApiGet(overrides: Record<string, unknown> = {}) {
  const defaults: Record<string, unknown> = {
    '/api/overseerr/configured': { configured: false },
    ...overrides,
  }
  mockApi.get.mockImplementation(((url: string) => {
    const data = defaults[url]
    if (data !== undefined) return Promise.resolve(data)
    return Promise.resolve(null)
  }) as typeof api.get)
}

beforeEach(() => {
  vi.clearAllMocks()
})

afterEach(() => {
  vi.restoreAllMocks()
})

describe('Requests', () => {
  describe('when Overseerr is not configured', () => {
    it('shows not-configured message for admin with settings hint', async () => {
      mockUseAuth.mockReturnValue({
        user: { id: 1, name: 'admin', email: '', role: 'admin', thumb_url: '', created_at: '', updated_at: '' },
        loading: false,
        setupRequired: false,
        setUser: vi.fn(),
        clearSetupRequired: vi.fn(),
        logout: vi.fn(),
      })
      mockApiGet({ '/api/overseerr/configured': { configured: false } })

      renderWithRouter(<Requests />)

      await waitFor(() => {
        expect(screen.getByText('Overseerr Not Configured')).toBeDefined()
      })
      expect(screen.getByText(/configure overseerr in settings/i)).toBeDefined()
    })

    it('shows not-configured message for viewer with ask-admin hint', async () => {
      mockUseAuth.mockReturnValue({
        user: { id: 2, name: 'viewer', email: '', role: 'viewer', thumb_url: '', created_at: '', updated_at: '' },
        loading: false,
        setupRequired: false,
        setUser: vi.fn(),
        clearSetupRequired: vi.fn(),
        logout: vi.fn(),
      })
      mockApiGet({ '/api/overseerr/configured': { configured: false } })

      renderWithRouter(<Requests />)

      await waitFor(() => {
        expect(screen.getByText('Overseerr Not Configured')).toBeDefined()
      })
      expect(screen.getByText(/ask an admin/i)).toBeDefined()
    })
  })

  describe('when Overseerr is configured', () => {
    const trendingResponse = {
      page: 1,
      totalPages: 1,
      totalResults: 2,
      results: [
        { id: 1, mediaType: 'movie', title: 'Test Movie', posterPath: '/poster1.jpg', voteAverage: 8.5, releaseDate: '2024-01-01' },
        { id: 2, mediaType: 'tv', name: 'Test TV Show', posterPath: '/poster2.jpg', voteAverage: 7.2, firstAirDate: '2024-06-01' },
      ],
    }

    it('shows discover tab with search input for admin', async () => {
      mockUseAuth.mockReturnValue({
        user: { id: 1, name: 'admin', email: '', role: 'admin', thumb_url: '', created_at: '', updated_at: '' },
        loading: false,
        setupRequired: false,
        setUser: vi.fn(),
        clearSetupRequired: vi.fn(),
        logout: vi.fn(),
      })
      mockApiGet({
        '/api/overseerr/configured': { configured: true },
        '/api/overseerr/requests/count': { total: 0, pending: 0, approved: 0, processing: 0, available: 0 },
      })

      renderWithRouter(<Requests />)

      await waitFor(() => {
        expect(screen.getByPlaceholderText(/search movies/i)).toBeDefined()
      })
      expect(screen.queryByText('Overseerr Not Configured')).toBeNull()
    })

    it('shows discover tab with search input for viewer', async () => {
      mockUseAuth.mockReturnValue({
        user: { id: 2, name: 'viewer', email: '', role: 'viewer', thumb_url: '', created_at: '', updated_at: '' },
        loading: false,
        setupRequired: false,
        setUser: vi.fn(),
        clearSetupRequired: vi.fn(),
        logout: vi.fn(),
      })
      mockApiGet({
        '/api/overseerr/configured': { configured: true },
        '/api/overseerr/requests/count': { total: 0, pending: 0, approved: 0, processing: 0, available: 0 },
      })

      renderWithRouter(<Requests />)

      await waitFor(() => {
        expect(screen.getByPlaceholderText(/search movies/i)).toBeDefined()
      })
      expect(screen.queryByText('Overseerr Not Configured')).toBeNull()
    })

    it('shows trending results when configured', async () => {
      mockUseAuth.mockReturnValue({
        user: { id: 1, name: 'admin', email: '', role: 'admin', thumb_url: '', created_at: '', updated_at: '' },
        loading: false,
        setupRequired: false,
        setUser: vi.fn(),
        clearSetupRequired: vi.fn(),
        logout: vi.fn(),
      })
      mockApiGet({
        '/api/overseerr/configured': { configured: true },
        '/api/overseerr/discover/trending': trendingResponse,
        '/api/overseerr/requests/count': { total: 0, pending: 0, approved: 0, processing: 0, available: 0 },
      })

      renderWithRouter(<Requests />)

      await waitFor(() => {
        expect(screen.getByText('Trending')).toBeDefined()
      })
      expect(screen.getByText('Test Movie')).toBeDefined()
      expect(screen.getByText('Test TV Show')).toBeDefined()
    })

    it('shows pending badge for admin when pending requests exist', async () => {
      mockUseAuth.mockReturnValue({
        user: { id: 1, name: 'admin', email: '', role: 'admin', thumb_url: '', created_at: '', updated_at: '' },
        loading: false,
        setupRequired: false,
        setUser: vi.fn(),
        clearSetupRequired: vi.fn(),
        logout: vi.fn(),
      })
      mockApiGet({
        '/api/overseerr/configured': { configured: true },
        '/api/overseerr/requests/count': { total: 5, pending: 3, approved: 1, processing: 1, available: 0 },
      })

      renderWithRouter(<Requests />)

      await waitFor(() => {
        expect(screen.getByText('3 pending')).toBeDefined()
      })
    })

    it('does not show pending badge for viewer', async () => {
      mockUseAuth.mockReturnValue({
        user: { id: 2, name: 'viewer', email: '', role: 'viewer', thumb_url: '', created_at: '', updated_at: '' },
        loading: false,
        setupRequired: false,
        setUser: vi.fn(),
        clearSetupRequired: vi.fn(),
        logout: vi.fn(),
      })
      mockApiGet({
        '/api/overseerr/configured': { configured: true },
        '/api/overseerr/requests/count': { total: 5, pending: 3, approved: 1, processing: 1, available: 0 },
      })

      renderWithRouter(<Requests />)

      await waitFor(() => {
        expect(screen.getByPlaceholderText(/search movies/i)).toBeDefined()
      })
      expect(screen.queryByText('3 pending')).toBeNull()
    })
  })
})
