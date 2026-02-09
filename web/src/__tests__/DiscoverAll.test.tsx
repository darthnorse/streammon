import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import { DiscoverAll } from '../pages/DiscoverAll'
import { MemoryRouter, Routes, Route } from 'react-router-dom'

vi.mock('../lib/api', () => ({
  api: {
    get: vi.fn(),
    post: vi.fn(),
    put: vi.fn(),
    del: vi.fn(),
  },
}))

import { api } from '../lib/api'
const mockApi = vi.mocked(api)

const trendingResponse = {
  page: 1,
  totalPages: 3,
  totalResults: 60,
  results: [
    { id: 1, mediaType: 'movie', title: 'Trending Movie', posterPath: '/p1.jpg', voteAverage: 7.5, releaseDate: '2024-01-01' },
    { id: 2, mediaType: 'tv', name: 'Trending Show', posterPath: '/p2.jpg', voteAverage: 8.0, firstAirDate: '2024-06-01' },
  ],
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

function mockGet(handler: (url: string) => unknown) {
  mockApi.get.mockImplementation(((url: string) => {
    const result = handler(url)
    return result instanceof Error ? Promise.reject(result) : Promise.resolve(result)
  }) as typeof api.get)
}

describe('DiscoverAll', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    window.scrollTo = vi.fn()
  })

  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('renders category title and results', async () => {
    mockGet(url =>
      url.startsWith('/api/overseerr/discover/trending') ? trendingResponse : null
    )

    renderAtRoute('/requests/discover/trending')

    await waitFor(() => {
      expect(screen.getByText('Trending Movie')).toBeDefined()
    })
    expect(screen.getByText('Trending')).toBeDefined()
    expect(screen.getByText('Trending Show')).toBeDefined()
    expect(screen.getByText('60 titles')).toBeDefined()
  })

  it('shows loading state', () => {
    mockApi.get.mockImplementation((() => new Promise(() => {})) as typeof api.get)

    renderAtRoute('/requests/discover/trending')

    expect(screen.getByText('Loading...')).toBeDefined()
  })

  it('shows empty state when no results', async () => {
    mockGet(url =>
      url.startsWith('/api/overseerr/discover/trending')
        ? { page: 1, totalPages: 0, totalResults: 0, results: [] }
        : null
    )

    renderAtRoute('/requests/discover/trending')

    await waitFor(() => {
      expect(screen.getByText('No results')).toBeDefined()
    })
  })

  it('shows error state on API failure', async () => {
    mockGet(url =>
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

  it('shows pagination when totalPages > 1', async () => {
    mockGet(url =>
      url.startsWith('/api/overseerr/discover/trending') ? trendingResponse : null
    )

    renderAtRoute('/requests/discover/trending')

    await waitFor(() => {
      expect(screen.getByText('1 / 3')).toBeDefined()
    })
    expect(screen.getByText('Previous')).toBeDefined()
    expect(screen.getByText('Next')).toBeDefined()
  })

  it('has back link to /requests', async () => {
    mockApi.get.mockImplementation((() => new Promise(() => {})) as typeof api.get)

    renderAtRoute('/requests/discover/trending')

    const backLink = screen.getByLabelText('Back to Requests') as HTMLAnchorElement
    expect(backLink.getAttribute('href')).toBe('/requests')
  })

  it('renders correct title for nested category paths', async () => {
    mockGet(url =>
      url.startsWith('/api/overseerr/discover/movies/upcoming')
        ? { ...trendingResponse, results: [trendingResponse.results[0]] }
        : null
    )

    renderAtRoute('/requests/discover/movies/upcoming')

    await waitFor(() => {
      expect(screen.getByText('Upcoming Movies')).toBeDefined()
    })
  })

  it('filters out person results', async () => {
    mockGet(url =>
      url.startsWith('/api/overseerr/discover/trending')
        ? {
            ...trendingResponse,
            results: [
              ...trendingResponse.results,
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
