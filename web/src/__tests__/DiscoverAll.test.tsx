import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, screen, waitFor, act } from '@testing-library/react'
import { DiscoverAll } from '../pages/DiscoverAll'
import { MemoryRouter, Routes, Route } from 'react-router-dom'
import { setupIntersectionObserver } from './helpers/mockIntersectionObserver'

vi.mock('../lib/api', () => ({
  api: { get: vi.fn() },
}))

import { api } from '../lib/api'
const mockApi = vi.mocked(api)

let triggerIntersection: () => void

const page1Response = {
  results: [
    { id: 1, media_type: 'movie', title: 'Trending Movie', poster_path: '/p1.jpg', vote_average: 7.5, release_date: '2024-01-01' },
    { id: 2, media_type: 'tv', name: 'Trending Show', poster_path: '/p2.jpg', vote_average: 8.0, first_air_date: '2024-06-01' },
  ],
  total_pages: 3,
}

const page2Response = {
  results: [
    { id: 3, media_type: 'movie', title: 'Another Movie', poster_path: '/p3.jpg', vote_average: 6.5, release_date: '2024-03-01' },
  ],
  total_pages: 3,
}

function renderAtRoute(path: string) {
  return render(
    <MemoryRouter initialEntries={[path]}>
      <Routes>
        <Route path="/discover/*" element={<DiscoverAll />} />
      </Routes>
    </MemoryRouter>
  )
}

function mockGetHandler(handler: (url: string) => unknown) {
  mockApi.get.mockImplementation(((url: string) => {
    const result = handler(url)
    return result instanceof Error ? Promise.reject(result) : Promise.resolve(result)
  }) as typeof api.get)
}

describe('DiscoverAll', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    const observer = setupIntersectionObserver()
    triggerIntersection = observer.triggerIntersection
  })

  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('renders category title and results', async () => {
    mockGetHandler(url =>
      url.startsWith('/api/tmdb/discover/trending') ? page1Response : null
    )

    renderAtRoute('/discover/trending')

    await waitFor(() => {
      expect(screen.getByText('Trending Movie')).toBeDefined()
    })
    expect(screen.getByText('Trending')).toBeDefined()
    expect(screen.getByText('Trending Show')).toBeDefined()
  })

  it('shows loading state', () => {
    mockApi.get.mockImplementation(((url: string) =>
      url.includes('/api/overseerr/configured')
        ? Promise.resolve({ configured: false })
        : new Promise(() => {})
    ) as typeof api.get)

    renderAtRoute('/discover/trending')

    expect(screen.getByText('Loading...')).toBeDefined()
  })

  it('shows empty state when no results', async () => {
    mockGetHandler(url =>
      url.startsWith('/api/tmdb/discover/trending')
        ? { results: [], total_pages: 0 }
        : null
    )

    renderAtRoute('/discover/trending')

    await waitFor(() => {
      expect(screen.getByText('No results')).toBeDefined()
    })
  })

  it('shows error state on API failure', async () => {
    mockGetHandler(url =>
      url.startsWith('/api/tmdb/discover/trending')
        ? new Error('Server error')
        : null
    )

    renderAtRoute('/discover/trending')

    await waitFor(() => {
      expect(screen.getByText('Server error')).toBeDefined()
    })
  })

  it('shows category not found for invalid category', async () => {
    mockGetHandler(() => ({ configured: false }))

    renderAtRoute('/discover/nonexistent')

    expect(screen.getByText('Category not found')).toBeDefined()
    // Only overseerr/configured should be fetched, not any discover endpoint
    const discoverCalls = mockApi.get.mock.calls.filter(
      ([url]) => typeof url === 'string' && url.includes('/api/tmdb/discover/')
    )
    expect(discoverCalls).toHaveLength(0)
  })

  it('accumulates items across pages on scroll', async () => {
    let discoverCallCount = 0
    mockApi.get.mockImplementation(((url: string) => {
      if (url.includes('/api/overseerr/configured')) return Promise.resolve({ configured: false })
      discoverCallCount++
      return Promise.resolve(discoverCallCount === 1 ? page1Response : page2Response)
    }) as typeof api.get)

    renderAtRoute('/discover/trending')

    await waitFor(() => {
      expect(screen.getByText('Trending Movie')).toBeDefined()
    })
    expect(screen.getByText('Trending Show')).toBeDefined()

    act(() => triggerIntersection())

    await waitFor(() => {
      expect(screen.getByText('Another Movie')).toBeDefined()
    })

    expect(screen.getByText('Trending Movie')).toBeDefined()
    expect(screen.getByText('Trending Show')).toBeDefined()
  })

  it('uses page=1 for initial fetch', async () => {
    mockGetHandler(url =>
      url.startsWith('/api/tmdb/discover/trending') ? page1Response : null
    )

    renderAtRoute('/discover/trending')

    await waitFor(() => {
      expect(screen.getByText('Trending Movie')).toBeDefined()
    })

    expect(mockApi.get).toHaveBeenCalledWith(
      '/api/tmdb/discover/trending?page=1',
      expect.any(AbortSignal),
    )
  })

  it('has back link to /discover', async () => {
    mockApi.get.mockImplementation(((url: string) =>
      url.includes('/api/overseerr/configured')
        ? Promise.resolve({ configured: false })
        : new Promise(() => {})
    ) as typeof api.get)

    renderAtRoute('/discover/trending')

    const backLink = screen.getByLabelText('Back to Discover') as HTMLAnchorElement
    expect(backLink.getAttribute('href')).toBe('/discover')
  })

  it('renders correct title for nested category paths', async () => {
    mockGetHandler(url =>
      url.startsWith('/api/tmdb/discover/movies/upcoming')
        ? { ...page1Response, results: [page1Response.results[0]] }
        : null
    )

    renderAtRoute('/discover/movies/upcoming')

    await waitFor(() => {
      expect(screen.getByText('Upcoming Movies')).toBeDefined()
    })
  })

  it('filters out person results', async () => {
    mockGetHandler(url =>
      url.startsWith('/api/tmdb/discover/trending')
        ? {
            ...page1Response,
            results: [
              ...page1Response.results,
              { id: 99, media_type: 'person', name: 'Famous Person' },
            ],
          }
        : null
    )

    renderAtRoute('/discover/trending')

    await waitFor(() => {
      expect(screen.getByText('Trending Movie')).toBeDefined()
    })
    expect(screen.queryByText('Famous Person')).toBeNull()
  })
})
