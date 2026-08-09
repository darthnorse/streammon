import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, screen, waitFor, act } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { DiscoverAll } from '../pages/DiscoverAll'
import { MemoryRouter, Routes, Route } from 'react-router-dom'
import { setupIntersectionObserver } from './helpers/mockIntersectionObserver'

vi.mock('../lib/api', () => ({
  api: { get: vi.fn() },
}))

import { api } from '../lib/api'
const mockApi = vi.mocked(api)

let triggerIntersection: () => void
let setIntersecting: (value: boolean) => void

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
    if (url === '/api/library/tmdb-ids') return Promise.resolve({ ids: [] })
    if (url.startsWith('/api/tmdb/genres/')) return Promise.resolve({ genres: [] })
    const result = handler(url)
    return result instanceof Error ? Promise.reject(result) : Promise.resolve(result)
  }) as typeof api.get)
}

describe('DiscoverAll', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    const observer = setupIntersectionObserver()
    triggerIntersection = observer.triggerIntersection
    setIntersecting = observer.setIntersecting
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
        : url === '/api/library/tmdb-ids'
          ? Promise.resolve({ ids: [] })
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
      if (url === '/api/library/tmdb-ids') return Promise.resolve({ ids: [] })
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
        : url === '/api/library/tmdb-ids'
          ? Promise.resolve({ ids: [] })
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

  it('sends filter params to the API', async () => {
    mockGetHandler(url => (url.startsWith('/api/tmdb/discover/tv') ? page1Response : null))

    renderAtRoute('/discover/tv?year=2024&genres=80,18')

    await waitFor(() => {
      expect(mockApi.get).toHaveBeenCalledWith(
        '/api/tmdb/discover/tv?year=2024&genres=80%2C18&page=1',
        expect.any(AbortSignal),
      )
    })
  })

  it('omits filter params when no filter is set', async () => {
    mockGetHandler(url => (url.startsWith('/api/tmdb/discover/tv') ? page1Response : null))

    renderAtRoute('/discover/tv')

    await waitFor(() => {
      expect(mockApi.get).toHaveBeenCalledWith('/api/tmdb/discover/tv?page=1', expect.any(AbortSignal))
    })
  })

  // Guards the whole bar -> hook -> URL -> request chain, which the component
  // and hook tests each only cover one link of.
  it('refetches page 1 when a filter is changed in the UI', async () => {
    const user = userEvent.setup()
    mockApi.get.mockImplementation(((url: string) => {
      if (url.includes('/api/overseerr/configured')) return Promise.resolve({ configured: false })
      if (url === '/api/library/tmdb-ids') return Promise.resolve({ ids: [] })
      if (url.startsWith('/api/tmdb/genres/')) return Promise.resolve({ genres: [{ id: 80, name: 'Crime' }] })
      return Promise.resolve(page1Response)
    }) as typeof api.get)

    renderAtRoute('/discover/tv')
    await waitFor(() => expect(screen.getByText('Trending Movie')).toBeDefined())

    await user.click(screen.getByLabelText('Release year'))
    await user.click(screen.getByText('2024 or newer'))

    await waitFor(() => {
      expect(mockApi.get).toHaveBeenCalledWith(
        '/api/tmdb/discover/tv?year=2024&page=1',
        expect.any(AbortSignal),
      )
    })
  })

  it('does not send hide_owned to the API', async () => {
    mockGetHandler(url => (url.startsWith('/api/tmdb/discover/tv') ? page1Response : null))

    renderAtRoute('/discover/tv?hide_owned=1')

    await waitFor(() => expect(screen.getByText('Trending Show')).toBeDefined())

    const discoverCalls = mockApi.get.mock.calls.filter(
      ([url]) => typeof url === 'string' && url.includes('/api/tmdb/discover/'),
    )
    expect(discoverCalls.every(([url]) => !String(url).includes('hide_owned'))).toBe(true)
  })

  it('hides owned items when hide_owned is set', async () => {
    mockApi.get.mockImplementation(((url: string) => {
      if (url.includes('/api/overseerr/configured')) return Promise.resolve({ configured: false })
      if (url === '/api/library/tmdb-ids') return Promise.resolve({ ids: ['1'] })
      if (url.startsWith('/api/tmdb/genres/')) return Promise.resolve({ genres: [] })
      return Promise.resolve(page1Response)
    }) as typeof api.get)

    renderAtRoute('/discover/tv?hide_owned=1')

    await waitFor(() => expect(screen.getByText('Trending Show')).toBeDefined())
    expect(screen.queryByText('Trending Movie')).toBeNull()
  })

  // An entirely-owned first page must keep paginating rather than report "no results".
  it('keeps loading when hide_owned empties the first page', async () => {
    let discoverCalls = 0
    mockApi.get.mockImplementation(((url: string) => {
      if (url.includes('/api/overseerr/configured')) return Promise.resolve({ configured: false })
      if (url === '/api/library/tmdb-ids') return Promise.resolve({ ids: ['1', '2'] })
      if (url.startsWith('/api/tmdb/genres/')) return Promise.resolve({ genres: [] })
      discoverCalls++
      return Promise.resolve(discoverCalls === 1 ? page1Response : page2Response)
    }) as typeof api.get)

    renderAtRoute('/discover/tv?hide_owned=1')

    await waitFor(() => expect(discoverCalls).toBeGreaterThan(0))
    expect(screen.queryByText('No results match your filters')).toBeNull()

    act(() => triggerIntersection())

    await waitFor(() => expect(screen.getByText('Another Movie')).toBeDefined())
  })

  // The `capped` disjunct of `exhausted`. A sentinel that stays in view (nothing
  // is ever visible to scroll past) must give up after MAX_AUTO_FILL pages and
  // hand over a Load more button, not keep fetching or sit blank forever.
  it('stops auto-filling and offers Load more when every page is owned', async () => {
    let discoverCalls = 0
    mockApi.get.mockImplementation(((url: string) => {
      if (url.includes('/api/overseerr/configured')) return Promise.resolve({ configured: false })
      if (url === '/api/library/tmdb-ids') return Promise.resolve({ ids: ['1', '2'] })
      if (url.startsWith('/api/tmdb/genres/')) return Promise.resolve({ genres: [] })
      discoverCalls++
      return Promise.resolve({ ...page1Response, total_pages: 50 })
    }) as typeof api.get)

    renderAtRoute('/discover/tv?hide_owned=1')

    await waitFor(() => expect(discoverCalls).toBe(1))

    act(() => setIntersecting(true))

    await waitFor(() => expect(screen.getByText('No results match your filters')).toBeDefined())
    expect(screen.getByRole('button', { name: 'Load more' })).toBeDefined()
    // 1 initial page + MAX_AUTO_FILL (10) auto-filled pages, then auto-fill gives up.
    expect(discoverCalls).toBe(11)
  })

  it('shows the filtered empty state with a clear action', async () => {
    mockGetHandler(url =>
      url.startsWith('/api/tmdb/discover/tv') ? { results: [], total_pages: 0 } : null,
    )

    renderAtRoute('/discover/tv?year=2024')

    await waitFor(() => expect(screen.getByText('No results match your filters')).toBeDefined())
    expect(screen.getByRole('button', { name: 'Clear all filters' })).toBeDefined()
  })

  it('does not render the filter bar for an unknown category', async () => {
    mockGetHandler(() => ({ configured: false }))

    renderAtRoute('/discover/nonexistent')

    expect(screen.getByText('Category not found')).toBeDefined()
    expect(screen.queryByLabelText('Genres')).toBeNull()
    // Flush useDiscoverData's in-flight fetches inside act, so their state
    // updates do not land after the test ends.
    await act(async () => {})
  })
})
