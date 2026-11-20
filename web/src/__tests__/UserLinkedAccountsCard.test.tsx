import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { UserLinkedAccountsCard } from '../components/UserLinkedAccountsCard'

const get = vi.fn()
const post = vi.fn()
const del = vi.fn()

vi.mock('../lib/api', () => ({
  api: {
    get: (url: string, signal?: AbortSignal) => get(url, signal),
    post: (url: string, body: unknown) => post(url, body),
    del: (url: string) => del(url),
  },
}))

function mockGet(aliases: string[], users: string[] = []) {
  get.mockImplementation((url: string) => {
    if (url === '/api/users/summary') {
      return Promise.resolve(users.map(name => ({ name, thumb_url: '', last_streamed_at: null })))
    }
    return Promise.resolve({ aliases })
  })
}

describe('UserLinkedAccountsCard', () => {
  beforeEach(() => {
    get.mockReset()
    post.mockReset()
    del.mockReset()
    vi.spyOn(window, 'confirm').mockReturnValue(true)
  })

  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('lists linked usernames', async () => {
    mockGet(['patrik', 'pr'])
    render(<UserLinkedAccountsCard userName="patrikr" />)
    expect(await screen.findByText('patrik')).toBeInTheDocument()
    expect(screen.getByText('pr')).toBeInTheDocument()
    expect(get).toHaveBeenCalledWith('/api/users/patrikr/aliases', expect.anything())
  })

  it('shows an empty state', async () => {
    mockGet([])
    render(<UserLinkedAccountsCard userName="patrikr" />)
    expect(await screen.findByText(/no linked accounts/i)).toBeInTheDocument()
  })

  it('links another username after confirmation', async () => {
    mockGet([], ['patrik', 'patrikr'])
    post.mockResolvedValue({ aliases: ['patrik'], watch_history_moved: 426 })
    render(<UserLinkedAccountsCard userName="patrikr" />)

    fireEvent.click(await screen.findByText('+ Link account'))
    fireEvent.change(screen.getByLabelText('Username to link'), { target: { value: ' patrik ' } })
    fireEvent.click(screen.getByRole('button', { name: 'Link' }))

    await waitFor(() =>
      expect(post).toHaveBeenCalledWith('/api/users/patrikr/aliases', { alias: 'patrik' }),
    )
    expect(window.confirm).toHaveBeenCalledWith(expect.stringContaining('"patrik" into "patrikr"'))
    expect(await screen.findByText(/moved 426 plays/i)).toBeInTheDocument()
  })

  it('does not link when the confirmation is cancelled', async () => {
    mockGet([])
    vi.spyOn(window, 'confirm').mockReturnValue(false)
    render(<UserLinkedAccountsCard userName="patrikr" />)

    fireEvent.click(await screen.findByText('+ Link account'))
    fireEvent.change(screen.getByLabelText('Username to link'), { target: { value: 'patrik' } })
    fireEvent.click(screen.getByRole('button', { name: 'Link' }))

    expect(post).not.toHaveBeenCalled()
  })

  it('surfaces a link error', async () => {
    mockGet([])
    post.mockRejectedValue(new Error('cannot remove the last admin'))
    render(<UserLinkedAccountsCard userName="patrikr" />)

    fireEvent.click(await screen.findByText('+ Link account'))
    fireEvent.change(screen.getByLabelText('Username to link'), { target: { value: 'patrik' } })
    fireEvent.click(screen.getByRole('button', { name: 'Link' }))

    expect(await screen.findByText('cannot remove the last admin')).toBeInTheDocument()
  })

  it('unlinks a username after confirmation', async () => {
    mockGet(['patrik r'])
    del.mockResolvedValue(undefined)
    render(<UserLinkedAccountsCard userName="patrikr" />)

    fireEvent.click(await screen.findByRole('button', { name: 'Unlink patrik r' }))

    await waitFor(() => expect(del).toHaveBeenCalledWith('/api/users/patrikr/aliases/patrik%20r'))
    expect(window.confirm).toHaveBeenCalledWith(expect.stringContaining('already merged stay'))
  })

  it('suggests usernames other than this user and its links', async () => {
    mockGet(['pr'], ['patrik', 'patrikr', 'pr'])
    render(<UserLinkedAccountsCard userName="patrikr" />)

    fireEvent.click(await screen.findByText('+ Link account'))
    await waitFor(() => {
      const options = Array.from(document.querySelectorAll('datalist option')).map(o => o.getAttribute('value'))
      expect(options).toEqual(['patrik'])
    })
  })
})
