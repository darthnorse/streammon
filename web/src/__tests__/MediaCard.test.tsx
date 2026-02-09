import { describe, it, expect, vi } from 'vitest'
import { screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { renderWithRouter } from '../test-utils'
import { MediaCard } from '../components/MediaCard'
import type { OverseerrMediaResult } from '../types'

function makeItem(overrides: Partial<OverseerrMediaResult> = {}): OverseerrMediaResult {
  return {
    id: 1,
    mediaType: 'movie',
    title: 'Test Movie',
    posterPath: '/poster.jpg',
    releaseDate: '2024-03-15',
    voteAverage: 8.5,
    ...overrides,
  }
}

describe('MediaCard', () => {
  it('renders movie title, year, and rating', () => {
    renderWithRouter(<MediaCard item={makeItem()} onClick={() => {}} />)

    expect(screen.getByText('Test Movie')).toBeDefined()
    expect(screen.getByText('2024')).toBeDefined()
    expect(screen.getByText('★ 8.5')).toBeDefined()
    expect(screen.getByText('Movie')).toBeDefined()
  })

  it('renders TV show with name and firstAirDate', () => {
    const item = makeItem({
      mediaType: 'tv',
      title: undefined,
      name: 'Test Show',
      releaseDate: undefined,
      firstAirDate: '2023-11-01',
    })
    renderWithRouter(<MediaCard item={item} onClick={() => {}} />)

    expect(screen.getByText('Test Show')).toBeDefined()
    expect(screen.getByText('2023')).toBeDefined()
    expect(screen.getByText('TV')).toBeDefined()
  })

  it('renders poster image when posterPath is set', () => {
    renderWithRouter(<MediaCard item={makeItem()} onClick={() => {}} />)

    const img = screen.getByAltText('Test Movie') as HTMLImageElement
    expect(img.src).toContain('/w185/poster.jpg')
    expect(img.getAttribute('loading')).toBe('lazy')
  })

  it('renders placeholder when no posterPath', () => {
    const item = makeItem({ posterPath: undefined })
    renderWithRouter(<MediaCard item={item} onClick={() => {}} />)

    expect(screen.queryByRole('img')).toBeNull()
  })

  it('shows "Unknown" when no title or name', () => {
    const item = makeItem({ title: undefined, name: undefined })
    renderWithRouter(<MediaCard item={item} onClick={() => {}} />)

    expect(screen.getByText('Unknown')).toBeDefined()
  })

  it('hides year when no releaseDate or firstAirDate', () => {
    const item = makeItem({ releaseDate: undefined, firstAirDate: undefined })
    renderWithRouter(<MediaCard item={item} onClick={() => {}} />)

    expect(screen.queryByText('2024')).toBeNull()
  })

  it('hides rating when voteAverage is 0', () => {
    const item = makeItem({ voteAverage: 0 })
    renderWithRouter(<MediaCard item={item} onClick={() => {}} />)

    expect(screen.queryByText(/★/)).toBeNull()
  })

  it('hides rating when voteAverage is undefined', () => {
    const item = makeItem({ voteAverage: undefined })
    renderWithRouter(<MediaCard item={item} onClick={() => {}} />)

    expect(screen.queryByText(/★/)).toBeNull()
  })

  it('calls onClick when clicked', async () => {
    const onClick = vi.fn()
    renderWithRouter(<MediaCard item={makeItem()} onClick={onClick} />)

    await userEvent.click(screen.getByRole('button'))
    expect(onClick).toHaveBeenCalledOnce()
  })

  it('applies custom className', () => {
    renderWithRouter(<MediaCard item={makeItem()} onClick={() => {}} className="custom-class" />)

    expect(screen.getByRole('button').className).toContain('custom-class')
  })

  it('shows media status badge when status > 1', () => {
    const item = makeItem({ mediaInfo: { id: 1, tmdbId: 1, status: 5, requests: [] } })
    renderWithRouter(<MediaCard item={item} onClick={() => {}} />)

    expect(screen.getByText('Available')).toBeDefined()
  })
})
