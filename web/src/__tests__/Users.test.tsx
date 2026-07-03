import { describe, it, expect, vi, beforeEach } from 'vitest'
import { screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { renderWithRouter } from '../test-utils'
import { Users } from '../pages/Users'
import type { UserSummary } from '../types'

vi.mock('../hooks/useFetch', () => ({
  useFetch: vi.fn(),
}))

vi.mock('../hooks/useMediaDetailModal', () => ({
  useMediaDetailModal: vi.fn(),
}))

import { useFetch } from '../hooks/useFetch'
import { useMediaDetailModal } from '../hooks/useMediaDetailModal'

const mockUseFetch = vi.mocked(useFetch)
const mockUseMediaDetailModal = vi.mocked(useMediaDetailModal)
const handleTitleClick = vi.fn()

const baseUser: UserSummary = {
  name: 'alice',
  thumb_url: '',
  last_streamed_at: null,
  last_ip: '',
  total_plays: 5,
  total_watched_ms: 3600000,
  trust_score: 90,
  last_played_title: 'Inception',
  last_played_grandparent_title: '',
  last_played_media_type: 'movie',
  last_played_server_id: 1,
  last_played_item_id: '100',
  last_played_grandparent_item_id: '',
}

function tableRows() {
  return document.querySelectorAll('tbody tr')
}

describe('Users', () => {
  beforeEach(() => {
    localStorage.clear()
    handleTitleClick.mockClear()
    mockUseMediaDetailModal.mockReturnValue({ handleTitleClick, modal: null })
  })

  it('opens the media detail modal on Enter for the last-played title', async () => {
    const user = userEvent.setup()
    mockUseFetch.mockReturnValue({ data: [baseUser], loading: false, error: null, refetch: vi.fn() })
    renderWithRouter(<Users />)

    const cell = screen.getByRole('button', { name: /inception/i })
    cell.focus()
    await user.keyboard('{Enter}')

    expect(handleTitleClick).toHaveBeenCalledWith(1, '100')
  })

  it('opens the media detail modal on Space for the last-played title', async () => {
    const user = userEvent.setup()
    mockUseFetch.mockReturnValue({ data: [baseUser], loading: false, error: null, refetch: vi.fn() })
    renderWithRouter(<Users />)

    const cell = screen.getByRole('button', { name: /inception/i })
    cell.focus()
    await user.keyboard(' ')

    expect(handleTitleClick).toHaveBeenCalledWith(1, '100')
  })

  it('shows a plain dash (no button) when there is no last-played title', () => {
    mockUseFetch.mockReturnValue({
      data: [{ ...baseUser, last_played_title: '' }],
      loading: false,
      error: null,
      refetch: vi.fn(),
    })
    renderWithRouter(<Users />)

    expect(screen.queryByRole('button', { name: /inception/i })).not.toBeInTheDocument()
    expect(screen.getAllByText('-').length).toBeGreaterThan(0)
  })

  it('sorts via keyboard activation (Enter) on a column header', async () => {
    const user = userEvent.setup()
    const users: UserSummary[] = [
      { ...baseUser, name: 'amy', total_plays: 99 },
      { ...baseUser, name: 'zack', total_plays: 1 },
    ]
    mockUseFetch.mockReturnValue({ data: users, loading: false, error: null, refetch: vi.fn() })
    renderWithRouter(<Users />)

    // Default sort is by name asc, so amy (a < z) is first.
    expect(tableRows()[0].textContent).toContain('amy')

    const header = screen.getByRole('button', { name: /total plays/i })
    header.focus()
    await user.keyboard('{Enter}')

    // Ascending by total_plays puts zack (1 play) first.
    expect(tableRows()[0].textContent).toContain('zack')
  })
})
