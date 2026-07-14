import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { UserNotesCard } from '../components/UserNotesCard'

const get = vi.fn()
const put = vi.fn()

vi.mock('../lib/api', () => ({
  api: {
    get: (url: string, signal?: AbortSignal) => get(url, signal),
    put: (url: string, body: unknown) => put(url, body),
  },
  updateUserNotes: (name: string, notes: string) =>
    put(`/api/users/${encodeURIComponent(name)}/notes`, { notes }),
}))

describe('UserNotesCard', () => {
  beforeEach(() => {
    get.mockReset()
    put.mockReset()
  })

  it('renders a saved note', async () => {
    get.mockResolvedValue({ notes: 'brother of patrik' })
    render(<UserNotesCard userName="alice" />)
    expect(await screen.findByText('brother of patrik')).toBeInTheDocument()
    expect(screen.getByText('Edit')).toBeInTheDocument()
  })

  it('shows the add affordance when empty', async () => {
    get.mockResolvedValue({ notes: '' })
    render(<UserNotesCard userName="alice" />)
    expect(await screen.findByText('+ Add a note')).toBeInTheDocument()
  })

  it('saves an edited note', async () => {
    get.mockResolvedValue({ notes: '' })
    put.mockResolvedValue({ notes: 'new note' })
    render(<UserNotesCard userName="alice" />)

    fireEvent.click(await screen.findByText('+ Add a note'))
    fireEvent.change(screen.getByRole('textbox'), { target: { value: 'new note' } })
    fireEvent.click(screen.getByText('Save'))

    await waitFor(() =>
      expect(put).toHaveBeenCalledWith('/api/users/alice/notes', { notes: 'new note' }),
    )
  })
})
