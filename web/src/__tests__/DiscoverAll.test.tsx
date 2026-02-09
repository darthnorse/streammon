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
    { id: 1, mediaType: 'movie', title: 'Trending Movie', posterPath: '/p1.jpg', voteAverage: 7.5, releaseDate: '2024-01-01' },
    { id: 2, mediaType: 'tv', name: 'Trending Show', posterPath: '/p2.jpg', voteAverage: 8.0, firstAirDate: '2024-06-01' },
  ],
  totalPages: 3,
}

const page2Response = {
  results: [
    { id: 3, mediaType: 'movie', title: 'Another Movie', posterPath: '/p3.jpg', voteAverage: 6.5, releaseDate: '2024-03-01' },
  ],
  totalPages: 3,
}

function renderAtRoute(path: string) {
  return render(
    <MemoryRouter initialEntries={[path]}>
      <Routes>
        <Route path="/requests/discover/*" element={<DiscoverAll />} />
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
      url.startsWith('/api/overseerr/discover/trending') ? page1Response : null
    )

    renderAtRoute('/requests/discover/trending')

    await waitFor(() => {
      expect(screen.getByText('Trending Movie')).toBeDefined()
    })
    expect(screen.getByText('Trending')).toBeDefined()
    expect(screen.getByText('Trending Show')).toBeDefined()
  })

  it('shows loading state', () => {
    mockApi.get.mockImplementation((() => new Promise(() => {})) as typeof api.get)

    renderAtRoute('/requests/discover/trending')

    expect(screen.getByText('Loading...')).toBeDefined()
  })

  it('shows empty state when no results', async () => {
    mockGetHandler(url =>
      url.startsWith('/api/overseerr/discover/trending')
        ? { results: [], totalPages: 0 }
        : null
    )

    renderAtRoute('/requests/discover/trending')

    await waitFor(() => {
      expect(screen.getByText('No results')).toBeDefined()
    })
  })

  it('shows error state on API failure', async () => {
    mockGetHandler(url =>
      url.startsWith('/api/overseerr/discover/trending')
        ? new Error('Server error')
        : null
    )

    renderAtRoute('/requests/discover/trending')

    await waitFor(() => {
      expect(screen.getByText('Server error')).toBeDefined()
    })
  })

  it('shows category not found for invalid category', async () => {
    renderAtRoute('/requests/discover/nonexistent')

    expect(screen.getByText('Category not found')).toBeDefined()
    expect(mockApi.get).not.toHaveBeenCalled()
  })

  it('accumulates items across pages on scroll', async () => {
    let callCount = 0
    mockApi.get.mockImplementation((() => {
      callCount++
      return Promise.resolve(callCount === 1 ? page1Response : page2Response)
    }) as typeof api.get)

    renderAtRoute('/requests/discover/trending')

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
      url.startsWith('/api/overseerr/discover/trending') ? page1Response : null
    )

    renderAtRoute('/requests/discover/trending')

    await waitFor(() => {
      expect(screen.getByText('Trending Movie')).toBeDefined()
    })

    expect(mockApi.get).toHaveBeenCalledWith(
      '/api/overseerr/discover/trending?page=1',
      expect.any(AbortSignal),
    )
  })

  it('has back link to /requests', async () => {
    mockApi.get.mockImplementation((() => new Promise(() => {})) as typeof api.get)

    renderAtRoute('/requests/discover/trending')

    const backLink = screen.getByLabelText('Back to Requests') as HTMLAnchorElement
    expect(backLink.getAttribute('href')).toBe('/requests')
  })

  it('renders correct title for nested category paths', async () => {
    mockGetHandler(url =>
      url.startsWith('/api/overseerr/discover/movies/upcoming')
        ? { ...page1Response, results: [page1Response.results[0]] }
        : null
    )

    renderAtRoute('/requests/discover/movies/upcoming')

    await waitFor(() => {
      expect(screen.getByText('Upcoming Movies')).toBeDefined()
    })
  })

  it('filters out person results', async () => {
    mockGetHandler(url =>
      url.startsWith('/api/overseerr/discover/trending')
        ? {
            ...page1Response,
            results: [
              ...page1Response.results,
              { id: 99, mediaType: 'person', name: 'Famous Person' },
            ],
          }
        : null
    )

    renderAtRoute('/requests/discover/trending')

    await waitFor(() => {
      expect(screen.getByText('Trending Movie')).toBeDefined()
    })
    expect(screen.queryByText('Famous Person')).toBeNull()
  })
})
