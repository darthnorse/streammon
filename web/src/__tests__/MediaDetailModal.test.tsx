import { describe, it, expect, vi } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/react'
import { MediaDetailModal } from '../components/MediaDetailModal'
import type { ItemDetails } from '../types'

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

vi.mock('../components/TMDBDetailModal', () => ({
  TMDBDetailModal: () => null,
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

describe('MediaDetailModal', () => {
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
