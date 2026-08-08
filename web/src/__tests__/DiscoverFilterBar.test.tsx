import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, waitFor, fireEvent } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { DiscoverFilterBar } from '../components/DiscoverFilterBar'
import { EMPTY_FILTERS, MAX_GENRES, categoryCaps, yearOptions, type DiscoverFilters } from '../lib/discoverFilters'

vi.mock('../lib/api', () => ({
  api: { get: vi.fn() },
}))

import { api } from '../lib/api'
const mockApi = vi.mocked(api)

const tvCaps = categoryCaps('tv')!
const upcomingCaps = categoryCaps('movies/upcoming')!
const trendingCaps = categoryCaps('trending')!

const GENRES = [
  { id: 80, name: 'Crime' },
  { id: 18, name: 'Drama' },
  { id: 35, name: 'Comedy' },
  { id: 28, name: 'Action' },
  { id: 12, name: 'Adventure' },
  { id: 99, name: 'Documentary' },
]

function renderBar(caps = tvCaps, filters: DiscoverFilters = EMPTY_FILTERS, activeCount = 0) {
  const onChange = vi.fn()
  const onClear = vi.fn()
  const view = render(
    <DiscoverFilterBar caps={caps} filters={filters} onChange={onChange} onClear={onClear} activeCount={activeCount} />,
  )
  return { onChange, onClear, ...view }
}

describe('DiscoverFilterBar', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mockApi.get.mockResolvedValue({ genres: GENRES })
  })

  it('renders the year control on categories that support it', async () => {
    renderBar(tvCaps)
    await waitFor(() => expect(screen.getByLabelText('Release year')).toBeDefined())
  })

  it('omits the year control on upcoming categories', async () => {
    renderBar(upcomingCaps)
    await waitFor(() => expect(screen.getByLabelText('Genres')).toBeDefined())
    expect(screen.queryByLabelText('Release year')).toBeNull()
  })

  it('emits a year change', async () => {
    const user = userEvent.setup()
    const { onChange } = renderBar(tvCaps)

    const year = yearOptions(new Date())[2]

    await user.click(screen.getByLabelText('Release year'))
    await user.click(screen.getByText(`${year} or newer`))

    expect(onChange).toHaveBeenCalledWith({ ...EMPTY_FILTERS, year })
  })

  it('emits a multi-genre selection', async () => {
    const user = userEvent.setup()
    const { onChange } = renderBar(tvCaps, { ...EMPTY_FILTERS, genres: [80] })

    await user.click(screen.getByLabelText('Genres'))
    await waitFor(() => expect(screen.getByText('Drama')).toBeDefined())
    await user.click(screen.getByText('Drama'))

    expect(onChange).toHaveBeenCalledWith({ ...EMPTY_FILTERS, genres: [80, 18] })
  })

  it('renders selected genres as removable chips', async () => {
    const user = userEvent.setup()
    const { onChange } = renderBar(tvCaps, { ...EMPTY_FILTERS, genres: [80, 18] })

    await waitFor(() => expect(screen.getByRole('button', { name: 'Remove Crime' })).toBeDefined())
    await user.click(screen.getByRole('button', { name: 'Remove Crime' }))

    expect(onChange).toHaveBeenCalledWith({ ...EMPTY_FILTERS, genres: [18] })
  })

  it('stops further genre selection at the cap', async () => {
    const user = userEvent.setup()
    const atCap = GENRES.slice(0, MAX_GENRES).map(g => g.id)
    renderBar(tvCaps, { ...EMPTY_FILTERS, genres: atCap })

    await user.click(screen.getByLabelText('Genres'))
    const unselected = await screen.findByRole('checkbox', { name: 'Documentary' })
    expect((unselected as HTMLInputElement).disabled).toBe(true)

    const selected = screen.getByRole('checkbox', { name: 'Crime' })
    expect((selected as HTMLInputElement).disabled).toBe(false)
  })

  it('renders trending type as a segmented control', async () => {
    renderBar(trendingCaps)
    await waitFor(() => expect(screen.getByRole('radio', { name: 'All' })).toBeDefined())
    expect(screen.getByRole('radio', { name: 'Movies' })).toBeDefined()
    expect(screen.getByRole('radio', { name: 'TV' })).toBeDefined()
  })

  it('disables every filter on trending until a media type is chosen', async () => {
    renderBar(trendingCaps)
    await waitFor(() => expect(screen.getByLabelText('Genres')).toBeDefined())

    expect(screen.getByLabelText('Genres').hasAttribute('disabled')).toBe(true)
    expect(screen.getByLabelText('Release year').hasAttribute('disabled')).toBe(true)
    expect(screen.getByLabelText('Sort by').hasAttribute('disabled')).toBe(true)
    expect(screen.getByLabelText('Minimum rating').hasAttribute('disabled')).toBe(true)
    expect(screen.getByRole('switch').hasAttribute('disabled')).toBe(true)
    expect(screen.getByText('Choose Movies or TV to filter')).toBeDefined()
  })

  it('enables filters on trending once a media type is chosen', async () => {
    renderBar(trendingCaps, { ...EMPTY_FILTERS, type: 'movie' })
    await waitFor(() => expect(screen.getByLabelText('Genres')).toBeDefined())

    expect(screen.getByLabelText('Genres').hasAttribute('disabled')).toBe(false)
    expect(screen.queryByText('Choose Movies or TV to filter')).toBeNull()
  })

  it('fetches genres for the chosen trending type', async () => {
    renderBar(trendingCaps, { ...EMPTY_FILTERS, type: 'movie' })
    await waitFor(() =>
      expect(mockApi.get).toHaveBeenCalledWith('/api/tmdb/genres/movie', expect.any(AbortSignal)),
    )
  })

  it('drops genres when the trending type changes', async () => {
    const user = userEvent.setup()
    const { onChange } = renderBar(trendingCaps, { ...EMPTY_FILTERS, type: 'movie', genres: [878] })

    await user.click(screen.getByRole('radio', { name: 'TV' }))

    expect(onChange).toHaveBeenCalledWith({ ...EMPTY_FILTERS, type: 'tv', genres: [] })
  })

  it('clears server filters when the trending type returns to All', async () => {
    const user = userEvent.setup()
    const { onChange } = renderBar(trendingCaps, { ...EMPTY_FILTERS, type: 'movie', genres: [878], year: 2024 })

    await user.click(screen.getByRole('radio', { name: 'All' }))

    expect(onChange).toHaveBeenCalledWith(EMPTY_FILTERS)
  })

  it('collapses behind a Filters button that reports the active count', async () => {
    const user = userEvent.setup()
    renderBar(tvCaps, { ...EMPTY_FILTERS, year: 2024 }, 1)

    const toggle = screen.getByRole('button', { name: 'Filters (1)' })
    expect(toggle.getAttribute('aria-expanded')).toBe('false')

    await user.click(toggle)
    expect(toggle.getAttribute('aria-expanded')).toBe('true')
  })

  it('names every control without nesting labels', async () => {
    const user = userEvent.setup()
    const { container } = renderBar(tvCaps)

    await user.click(screen.getByLabelText('Genres'))
    await screen.findByRole('checkbox', { name: 'Drama' })

    expect(container.querySelectorAll('label label')).toHaveLength(0)
    expect(screen.getByRole('button', { name: 'Genres' })).toBeDefined()
    expect(screen.getByRole('button', { name: 'Release year' })).toBeDefined()
    expect(screen.getByRole('button', { name: 'Sort by' })).toBeDefined()
    expect(screen.getByRole('button', { name: 'Minimum rating' })).toBeDefined()
    expect(screen.getByRole('switch', { name: 'Hide items in my library' })).toBeDefined()
  })

  it('refreshes the year options when the calendar year rolls over', () => {
    vi.useFakeTimers()
    try {
      vi.setSystemTime(new Date('2026-06-01T00:00:00Z'))
      // A fresh element each time: React bails out of re-rendering a component
      // whose element is referentially identical to the previous one.
      const bar = () => (
        <DiscoverFilterBar caps={tvCaps} filters={EMPTY_FILTERS} onChange={() => {}} onClear={() => {}} activeCount={0} />
      )
      const { rerender } = render(bar())

      fireEvent.click(screen.getByLabelText('Release year'))
      expect(screen.queryByText('2027 or newer')).toBeNull()
      fireEvent.click(screen.getByLabelText('Release year'))

      vi.setSystemTime(new Date('2027-06-01T00:00:00Z'))
      rerender(bar())

      fireEvent.click(screen.getByLabelText('Release year'))
      expect(screen.getByText('2027 or newer')).toBeDefined()
    } finally {
      vi.useRealTimers()
    }
  })

  it('shows a clear button only when filters are active', async () => {
    const onChange = vi.fn()
    const onClear = vi.fn()
    const { rerender } = render(
      <DiscoverFilterBar caps={tvCaps} filters={EMPTY_FILTERS} onChange={onChange} onClear={onClear} activeCount={0} />,
    )
    expect(screen.queryByRole('button', { name: 'Clear filters' })).toBeNull()

    rerender(
      <DiscoverFilterBar caps={tvCaps} filters={{ ...EMPTY_FILTERS, year: 2024 }} onChange={onChange} onClear={onClear} activeCount={1} />,
    )
    await userEvent.click(await screen.findByRole('button', { name: 'Clear filters' }))
    expect(onClear).toHaveBeenCalled()
  })
})
