import { render, screen, fireEvent } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { Libraries } from '../pages/Libraries'
import { useFetch } from '../hooks/useFetch'

vi.mock('../hooks/useFetch')
const navigate = vi.fn()
vi.mock('react-router-dom', async (orig) => ({
  ...(await orig<typeof import('react-router-dom')>()),
  useNavigate: () => navigate,
}))

test('clicking a library name navigates to its detail page', () => {
  vi.mocked(useFetch).mockImplementation((url: string | null) => {
    if (url?.includes('/api/libraries')) return {
      data: { libraries: [{ id: '1', server_id: 1, server_name: 'Plex', server_type: 'plex',
        name: 'Classics', type: 'movie', item_count: 1, child_count: 0, grandchild_count: 0, total_size: 0 }], errors: [] },
      loading: false, error: null } as never
    return { data: null, loading: false, error: null } as never
  })
  render(<MemoryRouter><Libraries /></MemoryRouter>)
  fireEvent.click(screen.getByText('Classics'))
  expect(navigate).toHaveBeenCalledWith('/library/1/1')
})
