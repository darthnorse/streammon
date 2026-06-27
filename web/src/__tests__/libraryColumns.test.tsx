import { render, screen, fireEvent } from '@testing-library/react'
import { getLibraryColumns } from '../lib/libraryColumns'
import type { LibraryItemDetail } from '../types'

const row: LibraryItemDetail = {
  id: 1, item_id: 'm1', server_id: 1, title: 'Dune',
  media_type: 'movie', added_at: '2026-01-01T00:00:00Z', plays: 3, total_hours: 2,
  unique_viewers: 2, file_size: 5000, flagged_for_deletion: false, protected: false,
}

test('title column calls onTitleClick with server + item id', () => {
  const onClick = vi.fn()
  const cols = getLibraryColumns(onClick, 'movie')
  const title = cols.find(c => c.id === 'title')!
  render(<table><tbody><tr><td>{title.render(row)}</td></tr></tbody></table>)
  fireEvent.click(screen.getByText('Dune'))
  expect(onClick).toHaveBeenCalledWith(1, 'm1')
})

test('episodes column is excluded for movie libraries', () => {
  const cols = getLibraryColumns(undefined, 'movie')
  expect(cols.find(c => c.id === 'episodes')).toBeUndefined()
})

test('episodes column is present for show libraries', () => {
  const cols = getLibraryColumns(undefined, 'show')
  expect(cols.find(c => c.id === 'episodes')).toBeDefined()
})
