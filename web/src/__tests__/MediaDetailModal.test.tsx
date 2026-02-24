import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { MediaDetailModal } from '../components/MediaDetailModal'
import { api } from '../lib/api'
import type { ItemDetails, TMDBMovieEnvelope } from '../types'

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
  dispatchRequestChanged: vi.fn(),
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

const mockEpisode: ItemDetails = {
  id: '67890',
  title: 'Ozymandias',
  year: 2013,
  summary: 'Everyone copes with radically changed circumstances.',
  media_type: 'episode',
  genres: ['Crime', 'Drama'],
  directors: ['Rian Johnson'],
  cast: [{ name: 'Bryan Cranston', role: 'Walter White' }],
  rating: 10.0,
  content_rating: 'TV-MA',
  duration_ms: 2880000,
  series_title: 'Breaking Bad',
  season_number: 5,
  episode_number: 14,
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

describe('MediaDetailModal', () => {
  beforeEach(() => {
    vi.mocked(api.get).mockReset().mockRejectedValue(new Error('not configured'))
  })

  describe('library entry', () => {
    it('shows loading spinner when loading', () => {
      render(<MediaDetailModal item={null} loading={true} onClose={() => {}} />)
      expect(document.querySelector('.animate-spin')).toBeInTheDocument()
    })

    it('renders movie details correctly', () => {
      render(<MediaDetailModal item={mockItem} loading={false} onClose={() => {}} />)

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

    it('renders episode details with series info', () => {
      render(<MediaDetailModal item={mockEpisode} loading={false} onClose={() => {}} />)

      expect(screen.getByText('Ozymandias')).toBeInTheDocument()
      expect(screen.getByText(/Breaking Bad/)).toBeInTheDocument()
      expect(screen.getByText(/S5E14/)).toBeInTheDocument()
    })

    it('calls onClose when backdrop is clicked', () => {
      const onClose = vi.fn()
      render(<MediaDetailModal item={mockItem} loading={false} onClose={onClose} />)

      const backdrop = document.querySelector('.fixed.inset-0')
      fireEvent.click(backdrop!)
      expect(onClose).toHaveBeenCalled()
    })

    it('calls onClose when close button is clicked', () => {
      const onClose = vi.fn()
      render(<MediaDetailModal item={mockItem} loading={false} onClose={onClose} />)

      const closeButton = screen.getByLabelText('Close')
      fireEvent.click(closeButton)
      expect(onClose).toHaveBeenCalled()
    })

    it('calls onClose when Escape key is pressed', () => {
      const onClose = vi.fn()
      render(<MediaDetailModal item={mockItem} loading={false} onClose={onClose} />)

      fireEvent.keyDown(document, { key: 'Escape' })
      expect(onClose).toHaveBeenCalled()
    })

    it('does not close when modal content is clicked', () => {
      const onClose = vi.fn()
      render(<MediaDetailModal item={mockItem} loading={false} onClose={onClose} />)

      const modalContent = document.querySelector('.max-w-6xl')
      fireEvent.click(modalContent!)
      expect(onClose).not.toHaveBeenCalled()
    })

    it('shows error message when item is null and not loading', () => {
      render(<MediaDetailModal item={null} loading={false} onClose={() => {}} />)
      expect(screen.getByText('Failed to load item details')).toBeInTheDocument()
    })
  })

  describe('TMDB entry', () => {
    function mockApiForTMDB(overseerrStatus?: number) {
      vi.mocked(api.get).mockImplementation((url: string) => {
        if (url.startsWith('/api/tmdb/movie/')) {
          return Promise.resolve(mockTMDBMovieResponse)
        }
        if (url.startsWith('/api/overseerr/movie/')) {
          return Promise.resolve({ mediaInfo: overseerrStatus != null ? { status: overseerrStatus } : undefined })
        }
        return Promise.reject(new Error('unexpected url'))
      })
    }

    it('shows loading spinner initially', () => {
      mockApiForTMDB()
      render(
        <MediaDetailModal mediaType="movie" mediaId={27205} overseerrConfigured={false} onClose={() => {}} />,
      )
      expect(document.querySelector('.animate-spin')).toBeInTheDocument()
    })

    it('renders TMDB movie details after fetch', async () => {
      mockApiForTMDB()
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
      mockApiForTMDB(5)
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
      mockApiForTMDB(1)
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
      mockApiForTMDB(2)
      render(
        <MediaDetailModal mediaType="movie" mediaId={27205} overseerrConfigured={true} onClose={() => {}} />,
      )

      await waitFor(() => {
        expect(screen.getByText('Inception')).toBeInTheDocument()
      })
      await waitFor(() => {
        expect(screen.getByText('Pending')).toBeInTheDocument()
      })
      expect(screen.queryByText('Request Movie')).not.toBeInTheDocument()
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
  })
})
