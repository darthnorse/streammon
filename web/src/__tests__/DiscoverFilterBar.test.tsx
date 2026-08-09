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

// Triggers are named "<field>: <current value>"; the exact pairing is pinned by
// the naming test, so the rest match on the field alone.
function trigger(field: string) {
  return screen.getByRole('button', { name: new RegExp(`^${field}: `) })
}

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
    await waitFor(() => expect(trigger('Release year')).toBeDefined())
  })

  it('omits the year control on upcoming categories', async () => {
    renderBar(upcomingCaps)
    await waitFor(() => expect(trigger('Genres')).toBeDefined())
    expect(screen.queryByRole('button', { name: /^Release year: / })).toBeNull()
  })

  it('emits a year change', async () => {
    const user = userEvent.setup()
    const { onChange } = renderBar(tvCaps)

    const year = yearOptions(new Date())[2]

    await user.click(trigger('Release year'))
    await user.click(screen.getByText(`${year} or newer`))

    expect(onChange).toHaveBeenCalledWith({ ...EMPTY_FILTERS, year })
  })

  it('emits a multi-genre selection', async () => {
    const user = userEvent.setup()
    const { onChange } = renderBar(tvCaps, { ...EMPTY_FILTERS, genres: [80] })

    await user.click(trigger('Genres'))
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

    await user.click(trigger('Genres'))
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

  it('disables the server-side filters on trending until a media type is chosen', async () => {
    renderBar(trendingCaps)
    await waitFor(() => expect(trigger('Genres')).toBeDefined())

    expect(trigger('Genres').hasAttribute('disabled')).toBe(true)
    expect(trigger('Release year').hasAttribute('disabled')).toBe(true)
    expect(trigger('Sort by').hasAttribute('disabled')).toBe(true)
    expect(trigger('Minimum rating').hasAttribute('disabled')).toBe(true)
    expect(screen.getByText('Choose Movies or TV to filter')).toBeDefined()
  })

  // hide-owned is applied client-side and never reaches the backend, so it stays
  // usable on a mixed category with no media type.
  it('keeps hide-owned live on trending without a media type', async () => {
    const user = userEvent.setup()
    const { onChange } = renderBar(trendingCaps)
    await waitFor(() => expect(trigger('Genres')).toBeDefined())

    const toggle = screen.getByRole('switch')
    expect(toggle.hasAttribute('disabled')).toBe(false)

    await user.click(toggle)
    expect(onChange).toHaveBeenCalledWith({ ...EMPTY_FILTERS, hideOwned: true })
  })

  it('enables filters on trending once a media type is chosen', async () => {
    renderBar(trendingCaps, { ...EMPTY_FILTERS, type: 'movie' })
    await waitFor(() => expect(trigger('Genres')).toBeDefined())

    expect(trigger('Genres').hasAttribute('disabled')).toBe(false)
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

  it('keeps only the selected type segment in the tab order', async () => {
    renderBar(trendingCaps, { ...EMPTY_FILTERS, type: 'tv' })

    await waitFor(() => expect(screen.getByRole('radio', { name: 'TV' })).toBeDefined())
    expect(screen.getByRole('radio', { name: 'TV' }).getAttribute('tabindex')).toBe('0')
    expect(screen.getByRole('radio', { name: 'All' }).getAttribute('tabindex')).toBe('-1')
    expect(screen.getByRole('radio', { name: 'Movies' }).getAttribute('tabindex')).toBe('-1')
  })

  it('moves the type selection and focus with the arrow keys', async () => {
    const { onChange } = renderBar(trendingCaps, { ...EMPTY_FILTERS, type: 'movie', genres: [878] })

    const movies = screen.getByRole('radio', { name: 'Movies' })
    movies.focus()
    fireEvent.keyDown(movies, { key: 'ArrowRight' })

    expect(onChange).toHaveBeenCalledWith({ ...EMPTY_FILTERS, type: 'tv', genres: [] })
    expect(document.activeElement).toBe(screen.getByRole('radio', { name: 'TV' }))
  })

  it('moves the type selection backwards with ArrowUp', async () => {
    const { onChange } = renderBar(trendingCaps, { ...EMPTY_FILTERS, type: 'movie' })

    const movies = screen.getByRole('radio', { name: 'Movies' })
    movies.focus()
    fireEvent.keyDown(movies, { key: 'ArrowUp' })

    expect(onChange).toHaveBeenCalledWith(EMPTY_FILTERS)
    expect(document.activeElement).toBe(screen.getByRole('radio', { name: 'All' }))
  })

  it('wraps arrow navigation at the ends of the type segments', async () => {
    const { onChange } = renderBar(trendingCaps)

    const all = screen.getByRole('radio', { name: 'All' })
    all.focus()
    fireEvent.keyDown(all, { key: 'ArrowLeft' })

    expect(onChange).toHaveBeenCalledWith({ ...EMPTY_FILTERS, type: 'tv', genres: [] })
    expect(document.activeElement).toBe(screen.getByRole('radio', { name: 'TV' }))
  })

  it('gives the clickable filter text the shared hover treatment', async () => {
    renderBar(trendingCaps, { ...EMPTY_FILTERS, type: 'movie' }, 1)

    const collapse = screen.getByRole('button', { name: 'Filters (1)' })
    expect(collapse).toHaveClass('hover:text-accent', 'hover:underline')

    const unselected = screen.getByRole('radio', { name: 'TV' })
    expect(unselected).toHaveClass('hover:text-accent', 'hover:underline')
    expect(screen.getByRole('radio', { name: 'Movies' })).toHaveClass('hover:underline')
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

    await user.click(trigger('Genres'))
    await screen.findByRole('checkbox', { name: 'Drama' })

    expect(container.querySelectorAll('label label')).toHaveLength(0)
    expect(screen.getByRole('button', { name: 'Genres: Any genre' })).toBeDefined()
    expect(screen.getByRole('button', { name: 'Release year: Any year' })).toBeDefined()
    expect(screen.getByRole('button', { name: 'Sort by: Popularity' })).toBeDefined()
    expect(screen.getByRole('button', { name: 'Minimum rating: Any rating' })).toBeDefined()
    expect(screen.getByRole('switch', { name: 'Hide items in my library' })).toBeDefined()
  })

  it('announces the current value of a control that has one set', async () => {
    renderBar(tvCaps, { ...EMPTY_FILTERS, year: 2024, sort: 'rating', rating: 7, genres: [80, 18] })

    await waitFor(() => expect(screen.getByRole('button', { name: 'Genres: 2 selected' })).toBeDefined())
    expect(screen.getByRole('button', { name: 'Release year: 2024 or newer' })).toBeDefined()
    expect(screen.getByRole('button', { name: 'Sort by: Rating' })).toBeDefined()
    expect(screen.getByRole('button', { name: 'Minimum rating: 7+ rating' })).toBeDefined()
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

      fireEvent.click(trigger('Release year'))
      expect(screen.queryByText('2027 or newer')).toBeNull()
      fireEvent.click(trigger('Release year'))

      vi.setSystemTime(new Date('2027-06-01T00:00:00Z'))
      rerender(bar())

      fireEvent.click(trigger('Release year'))
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
