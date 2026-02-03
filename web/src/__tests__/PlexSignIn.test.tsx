import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { screen, waitFor, fireEvent } from '@testing-library/react'
import { renderWithRouter } from '../test-utils'
import { PlexSignIn } from '../components/PlexSignIn'

vi.mock('../lib/plexOAuth', () => ({
  getClientId: vi.fn(() => 'test-client-id'),
  requestPin: vi.fn(),
  checkPin: vi.fn(),
  getAuthUrl: vi.fn(() => 'https://app.plex.tv/auth/#!?test=1'),
  fetchResources: vi.fn(),
}))

vi.mock('../lib/api', () => ({
  api: {
    post: vi.fn(),
  },
}))

import { requestPin, checkPin, fetchResources } from '../lib/plexOAuth'
import { api } from '../lib/api'

const mockRequestPin = vi.mocked(requestPin)
const mockCheckPin = vi.mocked(checkPin)
const mockFetchResources = vi.mocked(fetchResources)
const mockApiPost = vi.mocked(api.post)

beforeEach(() => {
  vi.clearAllMocks()
  vi.useFakeTimers({ shouldAdvanceTime: true })
})

afterEach(() => {
  vi.useRealTimers()
  vi.restoreAllMocks()
})

describe('PlexSignIn', () => {
  it('renders sign-in button', () => {
    renderWithRouter(<PlexSignIn onServersAdded={vi.fn()} />)
    expect(screen.getByRole('button', { name: /sign in to plex/i })).toBeDefined()
  })

  it('requests PIN and opens popup on click', async () => {
    mockRequestPin.mockResolvedValue({ id: 1, code: 'ABCD', authToken: null })
    mockCheckPin.mockResolvedValue({ id: 1, code: 'ABCD', authToken: null })

    const openSpy = vi.spyOn(window, 'open').mockReturnValue({ closed: false, close: vi.fn() } as unknown as Window)

    renderWithRouter(<PlexSignIn onServersAdded={vi.fn()} />)
    fireEvent.click(screen.getByRole('button', { name: /sign in to plex/i }))

    await waitFor(() => {
      expect(mockRequestPin).toHaveBeenCalled()
      expect(openSpy).toHaveBeenCalled()
    })
  })

  it('shows server list after successful auth', async () => {
    mockRequestPin.mockResolvedValue({ id: 1, code: 'ABCD', authToken: null })
    mockCheckPin
      .mockResolvedValueOnce({ id: 1, code: 'ABCD', authToken: null })
      .mockResolvedValueOnce({ id: 1, code: 'ABCD', authToken: 'my-token' })
    mockFetchResources.mockResolvedValue([
      {
        name: 'Home Server',
        clientIdentifier: 'abc',
        accessToken: 'srv-token',
        provides: 'server',
        connections: [{ uri: 'https://192.168.1.100:32400', local: false, relay: false, protocol: 'https' }],
      },
    ])

    const popup = { closed: false, close: vi.fn() } as unknown as Window
    vi.spyOn(window, 'open').mockReturnValue(popup)

    renderWithRouter(<PlexSignIn onServersAdded={vi.fn()} />)
    fireEvent.click(screen.getByRole('button', { name: /sign in to plex/i }))

    await waitFor(() => expect(mockRequestPin).toHaveBeenCalled())

    // Advance timer to trigger first poll (null token)
    await vi.advanceTimersByTimeAsync(1500)
    // Advance timer to trigger second poll (token received)
    await vi.advanceTimersByTimeAsync(1500)

    await waitFor(() => {
      expect(screen.getByText('Home Server')).toBeDefined()
    })
  })

  it('submits selected servers via API', async () => {
    mockRequestPin.mockResolvedValue({ id: 1, code: 'ABCD', authToken: null })
    mockCheckPin.mockResolvedValue({ id: 1, code: 'ABCD', authToken: 'my-token' })
    mockFetchResources.mockResolvedValue([
      {
        name: 'Home Server',
        clientIdentifier: 'abc',
        accessToken: 'srv-token',
        provides: 'server',
        connections: [{ uri: 'https://192.168.1.100:32400', local: false, relay: false, protocol: 'https' }],
      },
    ])
    mockApiPost.mockResolvedValue({})

    const onServersAdded = vi.fn()
    const popup = { closed: false, close: vi.fn() } as unknown as Window
    vi.spyOn(window, 'open').mockReturnValue(popup)

    renderWithRouter(<PlexSignIn onServersAdded={onServersAdded} />)
    fireEvent.click(screen.getByRole('button', { name: /sign in to plex/i }))

    await waitFor(() => expect(mockRequestPin).toHaveBeenCalled())
    await vi.advanceTimersByTimeAsync(1500)

    await waitFor(() => {
      expect(screen.getByText('Home Server')).toBeDefined()
    })

    // Server should be checked by default
    fireEvent.click(screen.getByRole('button', { name: /add selected/i }))

    await waitFor(() => {
      expect(mockApiPost).toHaveBeenCalledWith('/api/servers', expect.objectContaining({
        name: 'Home Server',
        type: 'plex',
        url: 'https://192.168.1.100:32400',
        api_key: 'my-token',
        enabled: true,
      }))
      expect(onServersAdded).toHaveBeenCalled()
    })
  })

  it('shows empty message when no servers found', async () => {
    mockRequestPin.mockResolvedValue({ id: 1, code: 'ABCD', authToken: null })
    mockCheckPin.mockResolvedValue({ id: 1, code: 'ABCD', authToken: 'my-token' })
    mockFetchResources.mockResolvedValue([])

    const popup = { closed: false, close: vi.fn() } as unknown as Window
    vi.spyOn(window, 'open').mockReturnValue(popup)

    renderWithRouter(<PlexSignIn onServersAdded={vi.fn()} />)
    fireEvent.click(screen.getByRole('button', { name: /sign in to plex/i }))

    await waitFor(() => expect(mockRequestPin).toHaveBeenCalled())
    await vi.advanceTimersByTimeAsync(1500)

    await waitFor(() => {
      expect(screen.getByText(/no servers found/i)).toBeDefined()
    })
  })

  it('shows error when PIN request fails', async () => {
    mockRequestPin.mockRejectedValue(new Error('Network error'))

    renderWithRouter(<PlexSignIn onServersAdded={vi.fn()} />)
    fireEvent.click(screen.getByRole('button', { name: /sign in to plex/i }))

    await waitFor(() => {
      expect(screen.getByText(/failed to start/i)).toBeDefined()
    })
  })
})
