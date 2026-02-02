import { describe, it, expect, vi, afterEach } from 'vitest'
import { screen } from '@testing-library/react'
import { renderWithRouter } from '../test-utils'
import { History } from '../pages/History'
import { baseHistoryEntry } from './fixtures'
import type { WatchHistoryEntry, PaginatedResult } from '../types'

vi.mock('../hooks/useFetch', () => ({
  useFetch: vi.fn(),
}))

import { useFetch } from '../hooks/useFetch'

const mockUseFetch = vi.mocked(useFetch)

afterEach(() => {
  vi.restoreAllMocks()
})

describe('History', () => {
  it('shows loading state', () => {
    mockUseFetch.mockReturnValue({ data: null, loading: true, error: null })
    renderWithRouter(<History />)
    expect(screen.getByText('History')).toBeDefined()
  })

  it('renders history entries', () => {
    const data: PaginatedResult<WatchHistoryEntry> = {
      items: [baseHistoryEntry],
      total: 1,
      page: 1,
      per_page: 20,
    }
    mockUseFetch.mockReturnValue({ data, loading: false, error: null })
    renderWithRouter(<History />)
    expect(screen.getAllByText('alice').length).toBeGreaterThan(0)
    expect(screen.getAllByText('Inception').length).toBeGreaterThan(0)
  })

  it('shows pagination when multiple pages', () => {
    const data: PaginatedResult<WatchHistoryEntry> = {
      items: [baseHistoryEntry],
      total: 50,
      page: 1,
      per_page: 20,
    }
    mockUseFetch.mockReturnValue({ data, loading: false, error: null })
    renderWithRouter(<History />)
    expect(screen.getByText(/next/i)).toBeDefined()
  })

  it('shows error state', () => {
    mockUseFetch.mockReturnValue({ data: null, loading: false, error: new Error('fail') })
    renderWithRouter(<History />)
    expect(screen.getByText(/error/i)).toBeDefined()
  })
})
