import { render, screen, fireEvent } from '@testing-library/react'
import { LibraryItemsTable } from '../components/LibraryItemsTable'
import { getLibraryColumns } from '../lib/libraryColumns'
import type { LibraryItemDetail } from '../types'

const items: LibraryItemDetail[] = [
  { id: 1, item_id: 'a', server_id: 1, title: 'Alpha', media_type: 'movie',
    added_at: '2026-01-01T00:00:00Z', plays: 0, total_hours: 0, unique_viewers: 0,
    file_size: 1, flagged_for_deletion: false, protected: false },
]

test('renders rows and toggles sort on a sortable header', () => {
  const onSort = vi.fn()
  const cols = getLibraryColumns(undefined, 'movie').filter(c => ['added', 'title', 'plays'].includes(c.id))
  render(<LibraryItemsTable items={items} columns={cols} sort={null} onSort={onSort} />)
  expect(screen.getByText('Alpha')).toBeInTheDocument()
  fireEvent.click(screen.getByText('Plays'))
  expect(onSort).toHaveBeenCalledWith({ columnId: 'plays', direction: 'desc' })
})

test('highlights never-played rows', () => {
  const cols = getLibraryColumns(undefined, 'movie').filter(c => c.id === 'title')
  render(<LibraryItemsTable items={items} columns={cols} sort={null} onSort={() => {}} />)
  expect(screen.getByTestId('library-row-1').className).toMatch(/border-l/)
})
