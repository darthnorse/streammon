import { describe, it, expect, vi, beforeEach } from 'vitest'
import { screen, waitFor, fireEvent } from '@testing-library/react'
import { renderWithRouter } from '../test-utils'
import { MediaServerSignIn } from '../components/MediaServerSignIn'
import type { User } from '../types'

vi.mock('../lib/api', () => ({
  api: {
    get: vi.fn(),
    post: vi.fn(),
  },
}))

import { api } from '../lib/api'

const mockGet = vi.mocked(api.get)
const mockPost = vi.mocked(api.post)

beforeEach(() => {
  vi.clearAllMocks()
})

const defaultProps = {
  serverType: 'emby' as const,
  loginEndpoint: '/auth/emby/login',
  serversEndpoint: '/auth/emby/servers',
  onSuccess: vi.fn(),
}

describe('MediaServerSignIn', () => {
  it('shows loading state while fetching servers', () => {
    mockGet.mockReturnValue(new Promise(() => {}))
    renderWithRouter(<MediaServerSignIn {...defaultProps} />)
    expect(screen.getByText(/loading emby servers/i)).toBeDefined()
  })

  it('shows message when no servers configured', async () => {
    mockGet.mockResolvedValue([])
    renderWithRouter(<MediaServerSignIn {...defaultProps} />)
    await waitFor(() => {
      expect(screen.getByText(/no emby servers configured/i)).toBeDefined()
    })
  })

  it('shows Jellyfin label for jellyfin server type', async () => {
    mockGet.mockResolvedValue([])
    renderWithRouter(
      <MediaServerSignIn
        {...defaultProps}
        serverType="jellyfin"
        serversEndpoint="/auth/jellyfin/servers"
      />,
    )
    await waitFor(() => {
      expect(screen.getByText(/no jellyfin servers configured/i)).toBeDefined()
    })
  })

  it('renders login form with single server auto-selected', async () => {
    mockGet.mockResolvedValue([{ id: 1, name: 'My Emby' }])
    renderWithRouter(<MediaServerSignIn {...defaultProps} />)

    await waitFor(() => {
      expect(screen.getByText(/emby username/i)).toBeDefined()
      expect(screen.getByText(/emby password/i)).toBeDefined()
      expect(screen.getByRole('button', { name: /sign in with emby/i })).toBeDefined()
    })

    expect(screen.queryByText(/emby server/i)).toBeNull()
  })

  it('shows server selector when multiple servers available', async () => {
    mockGet.mockResolvedValue([
      { id: 1, name: 'Emby One' },
      { id: 2, name: 'Emby Two' },
    ])
    renderWithRouter(<MediaServerSignIn {...defaultProps} />)

    await waitFor(() => {
      expect(screen.getByText(/emby server/i)).toBeDefined()
    })

    const options = screen.getAllByRole('option')
    expect(options).toHaveLength(3) // "Select a server..." + 2 servers
    expect(options[1].textContent).toBe('Emby One')
    expect(options[2].textContent).toBe('Emby Two')
  })

  it('submits credentials and calls onSuccess', async () => {
    const mockUser: User = { id: 1, name: 'alice', email: '', role: 'viewer', thumb_url: '', has_password: false, created_at: '', updated_at: '' }
    mockGet.mockResolvedValue([{ id: 5, name: 'My Emby' }])
    mockPost.mockResolvedValue(mockUser)

    const onSuccess = vi.fn()
    renderWithRouter(<MediaServerSignIn {...defaultProps} onSuccess={onSuccess} />)

    await waitFor(() => {
      expect(screen.getByRole('textbox')).toBeDefined()
    })

    fireEvent.change(screen.getByRole('textbox'), { target: { value: 'alice' } })
    fireEvent.change(document.querySelector('input[type="password"]')!, { target: { value: 'secret' } })
    fireEvent.click(screen.getByRole('button', { name: /sign in with emby/i }))

    await waitFor(() => {
      expect(mockPost).toHaveBeenCalledWith('/auth/emby/login', {
        server_id: 5,
        username: 'alice',
        password: 'secret',
      })
      expect(onSuccess).toHaveBeenCalledWith(mockUser)
    })
  })

  it('shows error on failed login', async () => {
    mockGet.mockResolvedValue([{ id: 1, name: 'My Emby' }])
    mockPost.mockRejectedValue(new Error('invalid credentials'))

    renderWithRouter(<MediaServerSignIn {...defaultProps} />)

    await waitFor(() => {
      expect(screen.getByRole('textbox')).toBeDefined()
    })

    fireEvent.change(screen.getByRole('textbox'), { target: { value: 'alice' } })
    fireEvent.change(document.querySelector('input[type="password"]')!, { target: { value: 'wrong' } })
    fireEvent.click(screen.getByRole('button', { name: /sign in with emby/i }))

    await waitFor(() => {
      expect(screen.getByText(/invalid credentials/i)).toBeDefined()
    })
  })

  it('disables submit button when no server selected', async () => {
    mockGet.mockResolvedValue([
      { id: 1, name: 'Emby One' },
      { id: 2, name: 'Emby Two' },
    ])
    renderWithRouter(<MediaServerSignIn {...defaultProps} />)

    await waitFor(() => {
      expect(screen.getByRole('button', { name: /sign in with emby/i })).toBeDefined()
    })

    const button = screen.getByRole('button', { name: /sign in with emby/i })
    expect(button).toHaveProperty('disabled', true)
  })
})
