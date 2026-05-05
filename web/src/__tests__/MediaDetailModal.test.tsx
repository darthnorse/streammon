import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { MediaDetailModal } from '../components/MediaDetailModal'
import { ShowDetail } from '../components/modals/ShowDetail'
import { api } from '../lib/api'
import { MEDIA_STATUS } from '../lib/overseerr'
import type { ItemDetails, TMDBMovieEnvelope, TMDBTVEnvelope } from '../types'

vi.mock('../lib/api', () => ({
  api: {
    get: vi.fn().mockRejectedValue(new Error('not configured')),
    post: vi.fn(),
    put: vi.fn(),
    del: vi.fn(),
  },
}))

vi.mock('../hooks/useTMDBEnrichment', () => ({
  useTMDBEnrichment: () => ({ movie: null, tv: null, loading: false }),
}))

vi.mock('../hooks/useFetch', () => ({
  useFetch: () => ({ data: null, loading: false, error: null }),
}))

vi.mock('../components/PersonModal', () => ({
  PersonModal: () => null,
}))

vi.mock('../hooks/useRequestCount', () => ({
  useRequestCount: vi.fn(() => ({ data: null, loading: false, error: null, refetch: vi.fn() })),
  useRequestChangedListener: vi.fn(),
  dispatchRequestChanged: vi.fn(),
  REQUEST_CHANGED_EVENT: 'overseerr-request-changed',
}))

const mockItem: ItemDetails = {
  id: '12345',
  title: 'Oppenheimer',
  year: 2023,
  summary: 'The story of American scientist J. Robert Oppenheimer.',
  media_type: 'movie',
  thumb_url: 'library/metadata/12345/thumb/1699000000',
  genres: ['Drama', 'History', 'Biography'],
  directors: ['Christopher Nolan'],
  cast: [
    { name: 'Cillian Murphy', role: 'J. Robert Oppenheimer' },
    { name: 'Emily Blunt', role: 'Kitty Oppenheimer' },
  ],
  rating: 8.5,
  content_rating: 'R',
  duration_ms: 10800000,
  studio: 'Universal Pictures',
  server_id: 1,
  server_name: 'Test Plex',
  server_type: 'plex',
}

const mockTMDBMovieResponse: TMDBMovieEnvelope = {
  tmdb: {
    id: 27205,
    title: 'Inception',
    overview: 'A thief who steals corporate secrets through dream-sharing technology.',
    release_date: '2010-07-16',
    runtime: 148,
    vote_average: 8.4,
    backdrop_path: '/s2bT29y0ngXxxu2IA8AOzzXTRhd.jpg',
    poster_path: '/ljsZTbVsrQSqNgrafkv8sJ2RaDs.jpg',
    genres: [{ id: 28, name: 'Action' }, { id: 878, name: 'Science Fiction' }],
    credits: {
      cast: [{ id: 6193, name: 'Leonardo DiCaprio', character: 'Cobb', profile_path: '/wo2hJpn04vbtmh0B9utCFdsQhxM.jpg' }],
      crew: [{ id: 525, name: 'Christopher Nolan', job: 'Director', department: 'Directing', profile_path: '/path.jpg' }],
    },
  },
  library_items: [],
}

const mockTMDBTVResponse: TMDBTVEnvelope = {
  tmdb: {
    id: 1396,
    name: 'Breaking Bad',
    overview: 'A chemistry teacher diagnosed with cancer turns to manufacturing meth.',
    first_air_date: '2008-01-20',
    vote_average: 9.5,
    backdrop_path: '/backdrop.jpg',
    poster_path: '/poster.jpg',
    genres: [{ id: 18, name: 'Drama' }, { id: 80, name: 'Crime' }],
    credits: {
      cast: [{ id: 17419, name: 'Bryan Cranston', character: 'Walter White', profile_path: '/path.jpg' }],
      crew: [{ id: 66633, name: 'Vince Gilligan', job: 'Director', department: 'Directing', profile_path: '/path.jpg' }],
    },
    seasons: [
      { id: 1, season_number: 1, name: 'Season 1', episode_count: 7 },
      { id: 2, season_number: 2, name: 'Season 2', episode_count: 13 },
      { id: 3, season_number: 3, name: 'Season 3', episode_count: 13 },
    ],
  },
  library_items: [],
}

function mockTMDBApi(
  mediaType: 'movie' | 'tv',
  response: TMDBMovieEnvelope | TMDBTVEnvelope,
  overseerrStatus?: number,
) {
  vi.mocked(api.get).mockImplementation((url: string) => {
    if (url.startsWith(`/api/tmdb/${mediaType}/`)) {
      return Promise.resolve(response)
    }
    if (url.startsWith(`/api/overseerr/${mediaType}/`)) {
      return Promise.resolve({ mediaInfo: overseerrStatus != null ? { status: overseerrStatus } : undefined })
    }
    return Promise.reject(new Error('unexpected url'))
  })
}

describe('MediaDetailModal', () => {
  beforeEach(() => {
    vi.mocked(api.get).mockReset().mockRejectedValue(new Error('not configured'))
  })

  describe('library entry (ShowDetail)', () => {
    function renderShowDetail(item: ItemDetails | null, loading: boolean, onClose = () => {}) {
      return render(
        <MemoryRouter>
          <ShowDetail
            item={item}
            loading={loading}
            onClose={onClose}
            pushModal={() => {}}
            active={true}
            overseerrConfigured={false}
          />
        </MemoryRouter>,
      )
    }

    it('shows loading spinner when loading', () => {
      renderShowDetail(null, true)
      expect(document.querySelector('.animate-spin')).toBeInTheDocument()
    })

    it('renders movie details correctly', () => {
      renderShowDetail(mockItem, false)

      expect(screen.getByText('Oppenheimer')).toBeInTheDocument()
      expect(screen.getByText('2023')).toBeInTheDocument()
      expect(screen.getByText('3h 0m')).toBeInTheDocument()
      expect(screen.getByText('R')).toBeInTheDocument()
      expect(screen.getByText('Drama')).toBeInTheDocument()
      expect(screen.getByText('History')).toBeInTheDocument()
      expect(screen.getByText('Biography')).toBeInTheDocument()
      expect(screen.getByText(/Christopher Nolan/)).toBeInTheDocument()
      expect(screen.getByText('Cillian Murphy')).toBeInTheDocument()
      expect(screen.getByText('J. Robert Oppenheimer')).toBeInTheDocument()
      expect(screen.getByText('Universal Pictures')).toBeInTheDocument()
      expect(screen.getByText('Test Plex')).toBeInTheDocument()
    })

    it('calls onClose when backdrop is clicked', () => {
      const onClose = vi.fn()
      renderShowDetail(mockItem, false, onClose)

      const backdrop = document.querySelector('.fixed.inset-0')
      fireEvent.click(backdrop!)
      expect(onClose).toHaveBeenCalled()
    })

    it('calls onClose when close button is clicked', () => {
      const onClose = vi.fn()
      renderShowDetail(mockItem, false, onClose)

      const closeButton = screen.getByLabelText('Close')
      fireEvent.click(closeButton)
      expect(onClose).toHaveBeenCalled()
    })

    it('calls onClose when Escape key is pressed', () => {
      const onClose = vi.fn()
      renderShowDetail(mockItem, false, onClose)

      fireEvent.keyDown(document, { key: 'Escape' })
      expect(onClose).toHaveBeenCalled()
    })

    it('does not close when modal content is clicked', () => {
      const onClose = vi.fn()
      renderShowDetail(mockItem, false, onClose)

      const modalContent = document.querySelector('.max-w-6xl')
      fireEvent.click(modalContent!)
      expect(onClose).not.toHaveBeenCalled()
    })

    it('shows error message when item is null and not loading', () => {
      renderShowDetail(null, false)
      expect(screen.getByText('Failed to load item details')).toBeInTheDocument()
    })
  })

  describe('TMDB movie entry', () => {
    it('shows loading spinner initially', () => {
      mockTMDBApi('movie', mockTMDBMovieResponse)
      render(
        <MediaDetailModal mediaType="movie" mediaId={27205} overseerrConfigured={false} onClose={() => {}} />,
      )
      expect(document.querySelector('.animate-spin')).toBeInTheDocument()
    })

    it('renders TMDB movie details after fetch', async () => {
      mockTMDBApi('movie', mockTMDBMovieResponse)
      render(
        <MediaDetailModal mediaType="movie" mediaId={27205} overseerrConfigured={false} onClose={() => {}} />,
      )

      await waitFor(() => {
        expect(screen.getByText('Inception')).toBeInTheDocument()
      })
      expect(screen.getByText('2010')).toBeInTheDocument()
      expect(screen.getByText('148 min')).toBeInTheDocument()
      expect(screen.getByText('Action')).toBeInTheDocument()
      expect(screen.getByText('Science Fiction')).toBeInTheDocument()
      expect(screen.getByText(/Christopher Nolan/)).toBeInTheDocument()
      expect(screen.getByText('Leonardo DiCaprio')).toBeInTheDocument()
      expect(screen.getByText('Movie')).toBeInTheDocument()
    })

    it('shows Overseerr status badge when available', async () => {
      mockTMDBApi('movie', mockTMDBMovieResponse, MEDIA_STATUS.AVAILABLE)
      render(
        <MediaDetailModal mediaType="movie" mediaId={27205} overseerrConfigured={true} onClose={() => {}} />,
      )

      await waitFor(() => {
        expect(screen.getByText('Inception')).toBeInTheDocument()
      })
      await waitFor(() => {
        expect(screen.getByText('Available')).toBeInTheDocument()
      })
    })

    it('shows request button when not yet requested', async () => {
      mockTMDBApi('movie', mockTMDBMovieResponse, MEDIA_STATUS.UNKNOWN)
      render(
        <MediaDetailModal mediaType="movie" mediaId={27205} overseerrConfigured={true} onClose={() => {}} />,
      )

      await waitFor(() => {
        expect(screen.getByText('Inception')).toBeInTheDocument()
      })
      await waitFor(() => {
        expect(screen.getByText('Request Movie')).toBeInTheDocument()
      })
    })

    it('hides request button when already requested', async () => {
      mockTMDBApi('movie', mockTMDBMovieResponse, MEDIA_STATUS.PENDING)
      render(
        <MediaDetailModal mediaType="movie" mediaId={27205} overseerrConfigured={true} onClose={() => {}} />,
      )

      await waitFor(() => {
        expect(screen.getByText('Inception')).toBeInTheDocument()
      })
      await waitFor(() => {
        expect(screen.getByText('Already Requested')).toBeInTheDocument()
      })
      expect(screen.queryByText('Request Movie')).not.toBeInTheDocument()
    })

    it('shows "Already Requested" indicator for pending movie', async () => {
      mockTMDBApi('movie', mockTMDBMovieResponse, MEDIA_STATUS.PENDING)
      render(
        <MediaDetailModal mediaType="movie" mediaId={27205} overseerrConfigured={true} onClose={() => {}} />,
      )

      await waitFor(() => {
        expect(screen.getByText('Already Requested')).toBeInTheDocument()
      })
      expect(screen.getByText('Pending Approval')).toBeInTheDocument()
    })

    it('shows "Already Requested" indicator for processing movie', async () => {
      mockTMDBApi('movie', mockTMDBMovieResponse, MEDIA_STATUS.PROCESSING)
      render(
        <MediaDetailModal mediaType="movie" mediaId={27205} overseerrConfigured={true} onClose={() => {}} />,
      )

      await waitFor(() => {
        expect(screen.getByText('Already Requested')).toBeInTheDocument()
      })
      // "Processing" appears in both the top status badge and the indicator pill
      expect(screen.getAllByText('Processing').length).toBeGreaterThanOrEqual(1)
    })

    it('does not show "Already Requested" indicator for available movie', async () => {
      mockTMDBApi('movie', mockTMDBMovieResponse, MEDIA_STATUS.AVAILABLE)
      render(
        <MediaDetailModal mediaType="movie" mediaId={27205} overseerrConfigured={true} onClose={() => {}} />,
      )

      await waitFor(() => {
        expect(screen.getByText('Available')).toBeInTheDocument()
      })
      expect(screen.queryByText('Already Requested')).not.toBeInTheDocument()
    })

    it('shows fetch error when API call fails', async () => {
      vi.mocked(api.get).mockRejectedValue(new Error('Network error'))
      render(
        <MediaDetailModal mediaType="movie" mediaId={27205} overseerrConfigured={false} onClose={() => {}} />,
      )

      await waitFor(() => {
        expect(screen.getByText('Network error')).toBeInTheDocument()
      })
    })

    it('hides request button for partially available movie', async () => {
      mockTMDBApi('movie', mockTMDBMovieResponse, MEDIA_STATUS.PARTIALLY_AVAILABLE)
      render(
        <MediaDetailModal mediaType="movie" mediaId={27205} overseerrConfigured={true} onClose={() => {}} />,
      )

      await waitFor(() => {
        expect(screen.getByText('Inception')).toBeInTheDocument()
      })
      await waitFor(() => {
        expect(screen.getByText('Partial')).toBeInTheDocument()
      })
      expect(screen.queryByText('Request Movie')).not.toBeInTheDocument()
      expect(screen.queryByText('Request More')).not.toBeInTheDocument()
    })
  })

  describe('TMDB TV entry', () => {
    it('shows "Request TV Show" button when not yet requested', async () => {
      mockTMDBApi('tv', mockTMDBTVResponse, MEDIA_STATUS.UNKNOWN)
      render(
        <MediaDetailModal mediaType="tv" mediaId={1396} overseerrConfigured={true} onClose={() => {}} />,
      )

      await waitFor(() => {
        expect(screen.getByText('Breaking Bad')).toBeInTheDocument()
      })
      await waitFor(() => {
        expect(screen.getByText('Request TV Show')).toBeInTheDocument()
      })
    })

    it('shows "Request More" button for partially available TV show', async () => {
      mockTMDBApi('tv', mockTMDBTVResponse, MEDIA_STATUS.PARTIALLY_AVAILABLE)
      render(
        <MediaDetailModal mediaType="tv" mediaId={1396} overseerrConfigured={true} onClose={() => {}} />,
      )

      await waitFor(() => {
        expect(screen.getByText('Breaking Bad')).toBeInTheDocument()
      })
      await waitFor(() => {
        expect(screen.getByText('Request More')).toBeInTheDocument()
      })
    })

    it('shows season selector for partially available TV show', async () => {
      mockTMDBApi('tv', mockTMDBTVResponse, MEDIA_STATUS.PARTIALLY_AVAILABLE)
      render(
        <MediaDetailModal mediaType="tv" mediaId={1396} overseerrConfigured={true} onClose={() => {}} />,
      )

      await waitFor(() => {
        expect(screen.getByText('Breaking Bad')).toBeInTheDocument()
      })
      await waitFor(() => {
        expect(screen.getByText('Select Seasons to Request')).toBeInTheDocument()
      })
      expect(screen.getByLabelText('Season 1')).toBeInTheDocument()
      expect(screen.getByLabelText('Season 2')).toBeInTheDocument()
      expect(screen.getByLabelText('Season 3')).toBeInTheDocument()
    })

    it('hides request button for already-requested TV show', async () => {
      mockTMDBApi('tv', mockTMDBTVResponse, MEDIA_STATUS.PENDING)
      render(
        <MediaDetailModal mediaType="tv" mediaId={1396} overseerrConfigured={true} onClose={() => {}} />,
      )

      await waitFor(() => {
        expect(screen.getByText('Breaking Bad')).toBeInTheDocument()
      })
      await waitFor(() => {
        expect(screen.getByText('Already Requested')).toBeInTheDocument()
      })
      expect(screen.queryByText('Request TV Show')).not.toBeInTheDocument()
      expect(screen.queryByText('Request More')).not.toBeInTheDocument()
    })
  })
})
