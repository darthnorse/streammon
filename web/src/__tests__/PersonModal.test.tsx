import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import { PersonModal } from '../components/PersonModal'
import { api } from '../lib/api'
import { MEDIA_STATUS } from '../lib/overseerr'
import type { TMDBPersonDetails } from '../types'

vi.mock('../lib/api', () => ({
  api: {
    get: vi.fn(),
    post: vi.fn(),
    put: vi.fn(),
    del: vi.fn(),
  },
}))

const mockPerson: TMDBPersonDetails = {
  id: 1,
  name: 'Cillian Murphy',
  biography: 'Irish actor.',
  combined_credits: {
    cast: [
      {
        id: 100,
        media_type: 'movie',
        title: 'Oppenheimer',
        poster_path: '/a.jpg',
        release_date: '2023-07-19',
        vote_average: 8.5,
      },
      {
        id: 200,
        media_type: 'movie',
        title: 'Inception',
        poster_path: '/b.jpg',
        release_date: '2010-07-15',
        vote_average: 8.4,
      },
    ],
  },
}

describe('PersonModal', () => {
  beforeEach(() => {
    vi.mocked(api.get).mockReset().mockResolvedValue(mockPerson)
  })

  it('renders cast credits', async () => {
    render(<PersonModal personId={1} onClose={() => {}} />)
    await waitFor(() => {
      expect(screen.getByText('Oppenheimer')).toBeInTheDocument()
    })
    expect(screen.getByText('Inception')).toBeInTheDocument()
  })

  it('shows Overseerr status badge on cast credits when mediaStatuses provided', async () => {
    const statuses = new Map<string, number>([
      ['movie:100', MEDIA_STATUS.PENDING],
      ['movie:200', MEDIA_STATUS.AVAILABLE],
    ])
    render(<PersonModal personId={1} onClose={() => {}} mediaStatuses={statuses} />)
    await waitFor(() => {
      expect(screen.getByText('Oppenheimer')).toBeInTheDocument()
    })
    expect(screen.getByText('Pending')).toBeInTheDocument()
    expect(screen.getByText('Available')).toBeInTheDocument()
  })

  it('does not show status badges when mediaStatuses not provided', async () => {
    render(<PersonModal personId={1} onClose={() => {}} />)
    await waitFor(() => {
      expect(screen.getByText('Oppenheimer')).toBeInTheDocument()
    })
    expect(screen.queryByText('Pending')).not.toBeInTheDocument()
    expect(screen.queryByText('Available')).not.toBeInTheDocument()
  })
})
