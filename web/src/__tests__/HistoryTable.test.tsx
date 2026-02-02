import { describe, it, expect } from 'vitest'
import { screen } from '@testing-library/react'
import { renderWithRouter } from '../test-utils'
import { HistoryTable } from '../components/HistoryTable'
import type { WatchHistoryEntry } from '../types'

const entry: WatchHistoryEntry = {
  id: 1,
  server_id: 1,
  user_name: 'alice',
  media_type: 'movie',
  title: 'Inception',
  parent_title: '',
  grandparent_title: '',
  year: 2010,
  duration_ms: 8880000,
  watched_ms: 8880000,
  player: 'Chrome',
  platform: 'Web',
  ip_address: '10.0.0.1',
  started_at: '2024-06-15T12:00:00Z',
  stopped_at: '2024-06-15T14:28:00Z',
  created_at: '2024-06-15T12:00:00Z',
}

describe('HistoryTable', () => {
  it('renders entries with user name and title', () => {
    renderWithRouter(<HistoryTable entries={[entry]} />)
    expect(screen.getAllByText('alice').length).toBeGreaterThan(0)
    expect(screen.getAllByText('Inception').length).toBeGreaterThan(0)
  })

  it('shows empty state when no entries', () => {
    renderWithRouter(<HistoryTable entries={[]} />)
    expect(screen.getByText(/no history/i)).toBeDefined()
  })

  it('renders TV episode with show name', () => {
    const tvEntry: WatchHistoryEntry = {
      ...entry,
      media_type: 'episode',
      title: 'Ozymandias',
      parent_title: 'Season 5',
      grandparent_title: 'Breaking Bad',
    }
    renderWithRouter(<HistoryTable entries={[tvEntry]} />)
    expect(screen.getAllByText('Breaking Bad').length).toBeGreaterThan(0)
  })

  it('renders user name as link', () => {
    renderWithRouter(<HistoryTable entries={[entry]} />)
    const links = screen.getAllByRole('link', { name: 'alice' })
    expect(links.length).toBeGreaterThan(0)
    expect(links[0].getAttribute('href')).toBe('/users/alice')
  })

  it('renders dates in local timezone', () => {
    renderWithRouter(<HistoryTable entries={[entry]} />)
    const rows = screen.getAllByTestId('history-row')
    expect(rows[0].textContent).toContain('2024')
  })
})
