import { describe, it, expect, beforeEach } from 'vitest'
import { screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { renderWithRouter } from '../test-utils'
import { HistoryTable } from '../components/HistoryTable'
import { baseHistoryEntry } from './fixtures'

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
})
