import { describe, it, expect, vi, afterEach, beforeAll } from 'vitest'
import { screen, waitFor } from '@testing-library/react'
import { renderWithRouter } from '../test-utils'
import { DailyChart } from '../components/DailyChart'
import { emptyDayStat } from './fixtures'

beforeAll(() => {
  window.ResizeObserver = class {
    observe() {}
    unobserve() {}
    disconnect() {}
  } as unknown as typeof ResizeObserver
})

vi.mock('../hooks/useFetch', () => ({
  useFetch: vi.fn(),
}))

import { useFetch } from '../hooks/useFetch'

const mockUseFetch = vi.mocked(useFetch)

afterEach(() => {
  vi.restoreAllMocks()
})

const activeDays = [
  { ...emptyDayStat, date: '2024-06-14', movies: 3, tv: 5 },
  { ...emptyDayStat, date: '2024-06-15', movies: 1, tv: 2, music: 1 },
]

describe('DailyChart', () => {
  it('shows loading state', () => {
    mockUseFetch.mockReturnValue({ data: null, loading: true, error: null, refetch: vi.fn() })
    renderWithRouter(<DailyChart />)
    expect(screen.getByText(/loading chart data/i)).toBeDefined()
  })

  it('shows error state', () => {
    mockUseFetch.mockReturnValue({ data: null, loading: false, error: new Error('fail'), refetch: vi.fn() })
    renderWithRouter(<DailyChart />)
    expect(screen.getByText(/failed to load chart data/i)).toBeDefined()
  })

  it('shows empty data message when all counts are zero', () => {
    mockUseFetch.mockReturnValue({ data: [emptyDayStat], loading: false, error: null, refetch: vi.fn() })
    renderWithRouter(<DailyChart />)
    expect(screen.getByText(/no play data/i)).toBeDefined()
  })

  it('renders chart when data has values', async () => {
    mockUseFetch.mockReturnValue({ data: activeDays, loading: false, error: null, refetch: vi.fn() })
    renderWithRouter(<DailyChart />)
    await waitFor(() => {
      expect(screen.getByText('Daily Plays')).toBeDefined()
    })
    expect(screen.queryByText(/no play data/i)).toBeNull()
    expect(screen.queryByText(/loading/i)).toBeNull()
  })

  it('renders range selector buttons', () => {
    mockUseFetch.mockReturnValue({ data: null, loading: true, error: null, refetch: vi.fn() })
    renderWithRouter(<DailyChart />)
    expect(screen.getByText('7d')).toBeDefined()
    expect(screen.getByText('30d')).toBeDefined()
    expect(screen.getByText('90d')).toBeDefined()
  })

  it('passes correct URL with date range params', () => {
    mockUseFetch.mockReturnValue({ data: null, loading: true, error: null, refetch: vi.fn() })
    renderWithRouter(<DailyChart />)
    const url = mockUseFetch.mock.calls[0][0] as string
    expect(url).toMatch(/^\/api\/history\/daily\?start=\d{4}-\d{2}-\d{2}&end=\d{4}-\d{2}-\d{2}$/)
  })

  it('sets end to tomorrow so today is included', () => {
    mockUseFetch.mockReturnValue({ data: null, loading: true, error: null, refetch: vi.fn() })
    renderWithRouter(<DailyChart />)
    const url = mockUseFetch.mock.calls[0][0] as string
    const params = new URLSearchParams(url.split('?')[1])
    const today = new Date().toISOString().slice(0, 10)
    const tomorrow = new Date(Date.now() + 86_400_000).toISOString().slice(0, 10)
    expect(params.get('end')).toBe(tomorrow)
    expect(params.get('start')! <= today).toBe(true)
  })
})
