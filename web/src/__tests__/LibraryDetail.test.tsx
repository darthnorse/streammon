import { render, screen, waitFor } from '@testing-library/react'
import { MemoryRouter, Routes, Route } from 'react-router-dom'
import { LibraryDetail } from '../pages/LibraryDetail'
import { useFetch } from '../hooks/useFetch'

vi.mock('../hooks/useFetch')

const itemsResp = {
  items: [{ id: 1, item_id: 'a', server_id: 1, title: 'Alpha',
    media_type: 'movie', added_at: '2026-01-01T00:00:00Z', plays: 2, total_hours: 1,
    unique_viewers: 1, file_size: 10, flagged_for_deletion: false, protected: false }],
  total: 1, page: 1, per_page: 20,
}
const summaryResp = { total_titles: 1, total_size: 10, watched_titles: 1, never_played: 0, reclaimable_size: 0 }

test('renders summary and item rows', async () => {
  vi.mocked(useFetch).mockImplementation((url: string | null) => {
    if (url?.includes('/summary')) return { data: summaryResp, loading: false, error: null } as never
    if (url?.includes('/items')) return { data: itemsResp, loading: false, error: null } as never
    return { data: null, loading: false, error: null } as never
  })
  render(
    <MemoryRouter initialEntries={['/library/1/1']}>
      <Routes><Route path="/library/:serverId/:libraryId" element={<LibraryDetail />} /></Routes>
    </MemoryRouter>,
  )
  await waitFor(() => expect(screen.getByText('Alpha')).toBeInTheDocument())
  expect(screen.getByText('Total titles')).toBeInTheDocument() // summary card rendered
})
