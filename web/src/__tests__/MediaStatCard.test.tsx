import { describe, it, expect } from 'vitest'
import { render, screen } from '@testing-library/react'
import { MediaStatCard } from '../components/stats/MediaStatCard'

describe('MediaStatCard', () => {
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
    expect(screen.getByText('10 plays · 5.0h')).toBeInTheDocument()
    const img = container.querySelector('img')
    expect(img).toHaveAttribute('src', '/api/servers/1/thumb/thumb1.jpg')
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
})
