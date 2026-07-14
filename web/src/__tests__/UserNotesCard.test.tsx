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

  it('shows an error state instead of the empty state when the fetch fails', async () => {
    get.mockRejectedValue(new Error('boom'))
    render(<UserNotesCard userName="alice" />)

    expect(await screen.findByText(/failed to load/i)).toBeInTheDocument()
    expect(screen.queryByText('+ Add a note')).not.toBeInTheDocument()
    expect(screen.queryByText('Edit')).not.toBeInTheDocument()
  })

  it('surfaces a save error and keeps the editor open when the PUT fails', async () => {
    get.mockResolvedValue({ notes: '' })
    put.mockRejectedValue(new Error('boom'))
    render(<UserNotesCard userName="alice" />)

    fireEvent.click(await screen.findByText('+ Add a note'))
    fireEvent.change(screen.getByRole('textbox'), { target: { value: 'oops' } })
    fireEvent.click(screen.getByText('Save'))

    expect(await screen.findByText(/failed to save/i)).toBeInTheDocument()
    // editor stays open with the unsaved draft so the admin can retry
    expect(screen.getByRole('textbox')).toHaveValue('oops')
  })
})
