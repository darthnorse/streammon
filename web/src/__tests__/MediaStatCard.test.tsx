import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { MediaStatCard } from '../components/stats/MediaStatCard'

// Mock useItemDetails hook
vi.mock('../hooks/useItemDetails', () => ({
  useItemDetails: vi.fn(() => ({
    data: null,
    loading: false,
    error: null,
  })),
}))

import { useItemDetails } from '../hooks/useItemDetails'
const mockUseItemDetails = vi.mocked(useItemDetails)

describe('MediaStatCard', () => {
  beforeEach(() => {
    mockUseItemDetails.mockReturnValue({
      data: null,
      loading: false,
      error: null,
    })
  })

  it('renders title', () => {
    render(<MediaStatCard title="Test Title" items={[]} />)
    expect(screen.getByText('Test Title')).toBeInTheDocument()
  })

  it('shows empty state when no items', () => {
    render(<MediaStatCard title="Test" items={[]} />)
    expect(screen.getByText('No data available')).toBeInTheDocument()
  })

  it('renders items with poster thumbnails', () => {
    const items = [
      { title: 'Movie 1', year: 2024, play_count: 10, total_hours: 5, thumb_url: 'thumb1.jpg', server_id: 1 },
    ]
    const { container } = render(<MediaStatCard title="Movies" items={items} />)
    expect(screen.getByText('Movie 1')).toBeInTheDocument()
    expect(screen.getByText('(2024)')).toBeInTheDocument()
    expect(screen.getByText('10 plays')).toBeInTheDocument()
    const img = container.querySelector('img')
    expect(img).toHaveAttribute('src', '/api/servers/1/thumb/thumb1.jpg')
  })

  it('builds correct img src from thumb_url with leading slash', () => {
    const items = [
      { title: 'Movie 1', play_count: 5, total_hours: 2, thumb_url: '/library/metadata/123/thumb/456', server_id: 2 },
    ]
    const { container } = render(<MediaStatCard title="Movies" items={items} />)
    const img = container.querySelector('img')
    expect(img).toHaveAttribute('src', '/api/servers/2/thumb/library/metadata/123/thumb/456')
  })

  it('shows rank number when no thumbnail', () => {
    const items = [{ title: 'Movie 1', play_count: 10, total_hours: 5 }]
    render(<MediaStatCard title="Movies" items={items} />)
    expect(screen.getByText('1')).toBeInTheDocument()
  })

  it('limits to 5 items', () => {
    const items = Array.from({ length: 10 }, (_, i) => ({
      title: `Movie ${i + 1}`,
      play_count: 10 - i,
      total_hours: 5,
    }))
    render(<MediaStatCard title="Movies" items={items} />)
    expect(screen.getByText('Movie 1')).toBeInTheDocument()
    expect(screen.getByText('Movie 5')).toBeInTheDocument()
    expect(screen.queryByText('Movie 6')).not.toBeInTheDocument()
  })

  it('opens modal when clicking item with item_id and server_id', async () => {
    mockUseItemDetails.mockReturnValue({
      data: {
        id: '123',
        title: 'Movie 1',
        media_type: 'movie',
        server_id: 1,
        server_type: 'plex',
        server_name: 'Test Server',
      },
      loading: false,
      error: null,
    })

    const items = [
      { title: 'Movie 1', year: 2024, play_count: 10, total_hours: 5, item_id: '123', server_id: 1 },
    ]
    render(<MediaStatCard title="Movies" items={items} />)

    const itemRow = screen.getByText('Movie 1').closest('div[class*="cursor-pointer"]')
    expect(itemRow).toBeInTheDocument()

    fireEvent.click(itemRow!)

    await waitFor(() => {
      expect(screen.getByRole('dialog')).toBeInTheDocument()
    })
  })

  it('does not show cursor-pointer when item has no item_id', () => {
    const items = [
      { title: 'Movie 1', year: 2024, play_count: 10, total_hours: 5, server_id: 1 },
    ]
    const { container } = render(<MediaStatCard title="Movies" items={items} />)

    // Item row should not have cursor-pointer class when no item_id
    const itemRows = container.querySelectorAll('[class*="flex items-center gap-2"]')
    const itemRow = Array.from(itemRows).find(el => el.textContent?.includes('Movie 1'))
    expect(itemRow?.className).not.toContain('cursor-pointer')
  })

  it('does not open modal when clicking item without item_id', async () => {
    const items = [
      { title: 'Movie 1', year: 2024, play_count: 10, total_hours: 5, server_id: 1 },
    ]
    render(<MediaStatCard title="Movies" items={items} />)

    const movieText = screen.getByText('Movie 1')
    fireEvent.click(movieText)

    // Modal should not appear
    await waitFor(() => {
      expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
    })
  })

  it('shows loading state in modal while fetching details', async () => {
    mockUseItemDetails.mockReturnValue({
      data: null,
      loading: true,
      error: null,
    })

    const items = [
      { title: 'Movie 1', year: 2024, play_count: 10, total_hours: 5, item_id: '123', server_id: 1 },
    ]
    render(<MediaStatCard title="Movies" items={items} />)

    const itemRow = screen.getByText('Movie 1').closest('div[class*="cursor-pointer"]')
    fireEvent.click(itemRow!)

    await waitFor(() => {
      expect(screen.getByRole('dialog')).toBeInTheDocument()
    })

    // Loading spinner should be visible
    const spinner = document.querySelector('.animate-spin')
    expect(spinner).toBeInTheDocument()
  })

  it('shows error state in modal when fetch fails', async () => {
    mockUseItemDetails.mockReturnValue({
      data: null,
      loading: false,
      error: new Error('Failed to fetch'),
    })

    const items = [
      { title: 'Movie 1', year: 2024, play_count: 10, total_hours: 5, item_id: '123', server_id: 1 },
    ]
    render(<MediaStatCard title="Movies" items={items} />)

    const itemRow = screen.getByText('Movie 1').closest('div[class*="cursor-pointer"]')
    fireEvent.click(itemRow!)

    await waitFor(() => {
      expect(screen.getByRole('dialog')).toBeInTheDocument()
    })

    expect(screen.getByText('Failed to load item details')).toBeInTheDocument()
  })
})
