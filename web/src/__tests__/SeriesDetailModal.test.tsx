import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { renderWithRouter } from '../test-utils'
import { SeriesDetailModal } from '../components/SeriesDetailModal'
import type { OverseerrTVDetails, SonarrSeriesDetails } from '../types'

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

const overseerrTVData: OverseerrTVDetails = {
  id: 42,
  name: 'Test Show',
  overview: 'An overseerr overview',
  posterPath: '/poster.jpg',
  backdropPath: '/backdrop.jpg',
  firstAirDate: '2024-01-15',
  voteAverage: 8.2,
  genres: [{ id: 1, name: 'Drama' }, { id: 2, name: 'Sci-Fi' }],
  status: 'Returning Series',
  tagline: 'A great tagline',
  numberOfEpisodes: 20,
  seasons: [
    { id: 100, seasonNumber: 0, name: 'Specials', episodeCount: 2 },
    { id: 101, seasonNumber: 1, name: 'Season 1', episodeCount: 10 },
    { id: 102, seasonNumber: 2, name: 'Season 2', episodeCount: 10 },
  ],
  credits: {
    cast: [
      { id: 1, name: 'Actor One', character: 'Hero', profilePath: '/a1.jpg' },
      { id: 2, name: 'Actor Two', character: 'Villain' },
    ],
    crew: [{ id: 10, name: 'Director One', job: 'Director' }],
  },
  networks: [{ id: 1, name: 'HBO' }],
  mediaInfo: { status: 1 },
}

const sonarrSeriesData: SonarrSeriesDetails = {
  id: 10,
  title: 'Sonarr Show',
  overview: 'A sonarr overview',
  year: 2024,
  network: 'Netflix',
  status: 'continuing',
  genres: ['Drama', 'Thriller'],
  ratings: { value: 7.5 },
  seasons: [
    { seasonNumber: 1, statistics: { episodeCount: 8, totalEpisodeCount: 10 } },
    { seasonNumber: 2, statistics: { episodeCount: 5, totalEpisodeCount: 10 } },
  ],
  statistics: { episodeCount: 13, seasonCount: 2 },
}

beforeEach(() => {
  vi.clearAllMocks()
})

afterEach(() => {
  vi.restoreAllMocks()
  document.body.style.overflow = ''
})

function mockOverseerrSuccess() {
  mockApi.get.mockImplementation((url: string) => {
    if (url.includes('/api/overseerr/tv/')) return Promise.resolve(overseerrTVData)
    return Promise.reject(new Error('unexpected'))
  })
}

function mockSonarrOnly() {
  mockApi.get.mockImplementation((url: string) => {
    if (url.includes('/api/sonarr/series/')) return Promise.resolve(sonarrSeriesData)
    return Promise.reject(new Error('unexpected'))
  })
}

describe('SeriesDetailModal', () => {
  it('shows Overseerr data with backdrop, cast, and request button on success', async () => {
    mockOverseerrSuccess()

    renderWithRouter(
      <SeriesDetailModal
        tmdbId={42}
        sonarrSeriesId={10}
        overseerrAvailable={true}
        onClose={vi.fn()}
      />
    )

    await waitFor(() => {
      expect(screen.getByText('Test Show')).toBeDefined()
    })
    expect(screen.getByText('An overseerr overview')).toBeDefined()
    expect(screen.getByText('Actor One')).toBeDefined()
    expect(screen.getByText('Actor Two')).toBeDefined()
    expect(screen.getByText('Directed by')).toBeDefined()
    expect(screen.getByText('Request TV Show')).toBeDefined()
    expect(screen.getByText('Drama')).toBeDefined()
    expect(screen.getByText('Sci-Fi')).toBeDefined()
  })

  it('falls back to Sonarr when Overseerr fails', async () => {
    mockApi.get.mockImplementation((url: string) => {
      if (url.includes('/api/overseerr/tv/')) return Promise.reject(new Error('upstream error'))
      if (url.includes('/api/sonarr/series/')) return Promise.resolve(sonarrSeriesData)
      return Promise.reject(new Error('unexpected'))
    })

    renderWithRouter(
      <SeriesDetailModal
        tmdbId={42}
        sonarrSeriesId={10}
        overseerrAvailable={true}
        onClose={vi.fn()}
      />
    )

    await waitFor(() => {
      expect(screen.getByText('Sonarr Show')).toBeDefined()
    })
    expect(screen.getByText('A sonarr overview')).toBeDefined()
    expect(screen.getByText('Drama')).toBeDefined()
    expect(screen.getByText('Thriller')).toBeDefined()
    // No cast, backdrop, or request button from Sonarr
    expect(screen.queryByText('Actor One')).toBeNull()
    expect(screen.queryByText('Request TV Show')).toBeNull()
  })

  it('goes straight to Sonarr when Overseerr is unavailable', async () => {
    mockSonarrOnly()

    renderWithRouter(
      <SeriesDetailModal
        tmdbId={42}
        sonarrSeriesId={10}
        overseerrAvailable={false}
        onClose={vi.fn()}
      />
    )

    await waitFor(() => {
      expect(screen.getByText('Sonarr Show')).toBeDefined()
    })
    // Should not have attempted Overseerr
    const calls = mockApi.get.mock.calls.map(c => c[0] as string)
    expect(calls.some(u => u.includes('/api/overseerr/'))).toBe(false)
  })

  it('goes straight to Sonarr when tmdbId is null even if overseerrAvailable', async () => {
    mockSonarrOnly()

    renderWithRouter(
      <SeriesDetailModal
        tmdbId={null}
        sonarrSeriesId={10}
        overseerrAvailable={true}
        onClose={vi.fn()}
      />
    )

    await waitFor(() => {
      expect(screen.getByText('Sonarr Show')).toBeDefined()
    })
    const calls = mockApi.get.mock.calls.map(c => c[0] as string)
    expect(calls.some(u => u.includes('/api/overseerr/'))).toBe(false)
  })

  it('shows error when both Overseerr and Sonarr fail', async () => {
    mockApi.get.mockRejectedValue(new Error('network error'))

    renderWithRouter(
      <SeriesDetailModal
        tmdbId={42}
        sonarrSeriesId={10}
        overseerrAvailable={true}
        onClose={vi.fn()}
      />
    )

    await waitFor(() => {
      expect(screen.getByText('Failed to load series details')).toBeDefined()
    })
  })

  it('submits request with correct seasons', async () => {
    mockOverseerrSuccess()
    mockApi.post.mockResolvedValue({})

    renderWithRouter(
      <SeriesDetailModal
        tmdbId={42}
        sonarrSeriesId={10}
        overseerrAvailable={true}
        onClose={vi.fn()}
      />
    )

    await waitFor(() => {
      expect(screen.getByText('Request TV Show')).toBeDefined()
    })

    await userEvent.click(screen.getByText('Request TV Show'))

    await waitFor(() => {
      expect(screen.getByText('Request submitted successfully!')).toBeDefined()
    })
    expect(mockApi.post).toHaveBeenCalledWith('/api/overseerr/requests', {
      mediaType: 'tv',
      mediaId: 42,
      seasons: [1, 2],
    })
  })

  it('shows request error on failure', async () => {
    mockOverseerrSuccess()
    mockApi.post.mockRejectedValue(new Error('quota exceeded'))

    renderWithRouter(
      <SeriesDetailModal
        tmdbId={42}
        sonarrSeriesId={10}
        overseerrAvailable={true}
        onClose={vi.fn()}
      />
    )

    await waitFor(() => {
      expect(screen.getByText('Request TV Show')).toBeDefined()
    })

    await userEvent.click(screen.getByText('Request TV Show'))

    await waitFor(() => {
      expect(screen.getByText('quota exceeded')).toBeDefined()
    })
  })

  it('does not show request button with Sonarr data source', async () => {
    mockSonarrOnly()

    renderWithRouter(
      <SeriesDetailModal
        tmdbId={null}
        sonarrSeriesId={10}
        overseerrAvailable={false}
        onClose={vi.fn()}
      />
    )

    await waitFor(() => {
      expect(screen.getByText('Sonarr Show')).toBeDefined()
    })
    expect(screen.queryByText('Request TV Show')).toBeNull()
  })

  it('closes on Escape key', async () => {
    mockSonarrOnly()

    const onClose = vi.fn()
    renderWithRouter(
      <SeriesDetailModal
        tmdbId={null}
        sonarrSeriesId={10}
        overseerrAvailable={false}
        onClose={onClose}
      />
    )

    await waitFor(() => {
      expect(screen.getByText('Sonarr Show')).toBeDefined()
    })

    await userEvent.keyboard('{Escape}')
    expect(onClose).toHaveBeenCalled()
  })
})
