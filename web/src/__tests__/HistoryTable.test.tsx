import { describe, it, expect } from 'vitest'
import { screen } from '@testing-library/react'
import { renderWithRouter } from '../test-utils'
import { HistoryTable } from '../components/HistoryTable'
import { baseHistoryEntry } from './fixtures'

describe('HistoryTable', () => {
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
})
