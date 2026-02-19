import { describe, it, expect, beforeEach, vi } from 'vitest'
import { screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { renderWithRouter } from '../test-utils'
import { HistoryTable } from '../components/HistoryTable'
import { baseHistoryEntry } from './fixtures'
import { api } from '../lib/api'

describe('HistoryTable', () => {
  beforeEach(() => {
    localStorage.clear()
  })

  it('renders entries with user name and title', () => {
    renderWithRouter(<HistoryTable entries={[baseHistoryEntry]} />)
    expect(screen.getAllByText('alice').length).toBeGreaterThan(0)
    expect(screen.getAllByText('Inception').length).toBeGreaterThan(0)
  })

  it('shows empty state when no entries', () => {
    renderWithRouter(<HistoryTable entries={[]} />)
    expect(screen.getByText(/no history/i)).toBeDefined()
  })

  it('renders TV episode with show name', () => {
    renderWithRouter(
      <HistoryTable entries={[{
        ...baseHistoryEntry,
        media_type: 'episode',
        title: 'Ozymandias',
        parent_title: 'Season 5',
        grandparent_title: 'Breaking Bad',
      }]} />
    )
    expect(screen.getAllByText('Breaking Bad').length).toBeGreaterThan(0)
  })

  it('renders user name as link', () => {
    renderWithRouter(<HistoryTable entries={[baseHistoryEntry]} />)
    const links = screen.getAllByRole('link', { name: 'alice' })
    expect(links.length).toBeGreaterThan(0)
    expect(links[0].getAttribute('href')).toBe('/users/alice')
  })

  it('renders dates in local timezone', () => {
    renderWithRouter(<HistoryTable entries={[baseHistoryEntry]} />)
    const rows = screen.getAllByTestId('history-row')
    expect(rows[0].textContent).toContain('2024')
  })

  it('hides user column when hideUser is true', () => {
    renderWithRouter(<HistoryTable entries={[baseHistoryEntry]} hideUser />)
    const links = screen.queryAllByRole('link', { name: 'alice' })
    expect(links.length).toBe(0)
  })

  it('renders column settings button', () => {
    renderWithRouter(<HistoryTable entries={[baseHistoryEntry]} />)
    expect(screen.getByRole('button', { name: /column settings/i })).toBeDefined()
  })

  it('opens column settings popover on click', async () => {
    const user = userEvent.setup()
    renderWithRouter(<HistoryTable entries={[baseHistoryEntry]} />)

    await user.click(screen.getByRole('button', { name: /column settings/i }))

    expect(screen.getByText('Show columns')).toBeDefined()
    expect(screen.getByLabelText('Title')).toBeDefined()
    expect(screen.getByLabelText('Type')).toBeDefined()
  })

  it('toggles column visibility', async () => {
    const user = userEvent.setup()
    renderWithRouter(<HistoryTable entries={[baseHistoryEntry]} />)

    const table = document.querySelector('table')
    expect(table?.textContent).toContain('Chrome')

    await user.click(screen.getByRole('button', { name: /column settings/i }))
    await user.click(screen.getByLabelText('Player'))

    expect(table?.textContent).not.toContain('Chrome')
  })

  it('persists column settings to localStorage', async () => {
    const user = userEvent.setup()
    renderWithRouter(<HistoryTable entries={[baseHistoryEntry]} />)

    await user.click(screen.getByRole('button', { name: /column settings/i }))
    await user.click(screen.getByLabelText('Player'))

    const stored = JSON.parse(localStorage.getItem('history-columns')!)
    expect(stored).not.toContain('player')
  })

  it('resets columns to defaults', async () => {
    const user = userEvent.setup()
    renderWithRouter(<HistoryTable entries={[baseHistoryEntry]} />)

    const table = document.querySelector('table')

    await user.click(screen.getByRole('button', { name: /column settings/i }))
    await user.click(screen.getByLabelText('Player'))
    await user.click(screen.getByLabelText('Platform'))

    expect(table?.textContent).not.toContain('Chrome')

    await user.click(screen.getByText('Reset to defaults'))

    const stored = JSON.parse(localStorage.getItem('history-columns')!)
    expect(stored).toContain('player')
    expect(stored).toContain('platform')
  })

  it('displays entry count in header', () => {
    renderWithRouter(<HistoryTable entries={[baseHistoryEntry]} />)
    expect(screen.getByText('1 entry')).toBeDefined()
  })

  it('pluralizes entry count correctly', () => {
    renderWithRouter(<HistoryTable entries={[baseHistoryEntry, { ...baseHistoryEntry, id: 2 }]} />)
    expect(screen.getByText('2 entries')).toBeDefined()
  })

  it('sorts entries when column header is clicked', async () => {
    const user = userEvent.setup()
    const entries = [
      { ...baseHistoryEntry, id: 1, user_name: 'bob' },
      { ...baseHistoryEntry, id: 2, user_name: 'alice' },
    ]
    renderWithRouter(<HistoryTable entries={entries} />)

    const table = document.querySelector('table')!
    const getTableRows = () => table.querySelectorAll('tbody tr')

    expect(getTableRows()[0].textContent).toContain('bob')

    await user.click(screen.getByRole('columnheader', { name: /user/i }))

    expect(getTableRows()[0].textContent).toContain('alice')
    expect(getTableRows()[1].textContent).toContain('bob')
  })

  it('toggles sort direction on repeated clicks', async () => {
    const user = userEvent.setup()
    const entries = [
      { ...baseHistoryEntry, id: 1, user_name: 'alice' },
      { ...baseHistoryEntry, id: 2, user_name: 'bob' },
    ]
    renderWithRouter(<HistoryTable entries={entries} />)

    const table = document.querySelector('table')!
    const getTableRows = () => table.querySelectorAll('tbody tr')

    const userHeader = screen.getByRole('columnheader', { name: /user/i })
    await user.click(userHeader) // asc
    expect(getTableRows()[0].textContent).toContain('alice')

    await user.click(userHeader) // desc
    expect(getTableRows()[0].textContent).toContain('bob')

    await user.click(userHeader) // clear - back to original order
    expect(getTableRows()[0].textContent).toContain('alice')
  })

  it('shows chevron for multi-session entries', () => {
    const multiEntry = { ...baseHistoryEntry, session_count: 3 }
    renderWithRouter(<HistoryTable entries={[multiEntry]} />)
    expect(screen.getAllByTestId('session-chevron').length).toBeGreaterThan(0)
  })

  it('hides chevron for single-session entries', () => {
    const singleEntry = { ...baseHistoryEntry, session_count: 1 }
    renderWithRouter(<HistoryTable entries={[singleEntry]} />)
    expect(screen.queryAllByTestId('session-chevron').length).toBe(0)
  })

  it('expands sessions on chevron click', async () => {
    const user = userEvent.setup()
    const multiEntry = { ...baseHistoryEntry, id: 42, session_count: 2 }

    const mockSessions = [
      {
        id: 1, history_id: 42, duration_ms: 50000, watched_ms: 25000, paused_ms: 0,
        player: 'Chrome', platform: 'Web', ip_address: '1.1.1.1',
        started_at: '2024-06-15T12:00:00Z', stopped_at: '2024-06-15T13:00:00Z',
        created_at: '2024-06-15T12:00:00Z',
      },
      {
        id: 2, history_id: 42, duration_ms: 50000, watched_ms: 25000, paused_ms: 0,
        player: 'iOS App', platform: 'iPhone', ip_address: '2.2.2.2',
        started_at: '2024-06-15T13:10:00Z', stopped_at: '2024-06-15T14:00:00Z',
        created_at: '2024-06-15T13:10:00Z',
      },
    ]

    vi.spyOn(api, 'get').mockResolvedValueOnce(mockSessions)

    renderWithRouter(<HistoryTable entries={[multiEntry]} />)

    const chevrons = screen.getAllByTestId('session-chevron')
    await user.click(chevrons[0])

    await waitFor(() => {
      expect(screen.getAllByTestId('session-row').length).toBe(2)
    })
    expect(api.get).toHaveBeenCalledWith('/api/history/42/sessions')

    vi.restoreAllMocks()
  })
})
