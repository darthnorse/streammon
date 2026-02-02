import { describe, it, expect, vi, afterEach, beforeEach } from 'vitest'
import { screen, waitFor, fireEvent } from '@testing-library/react'
import { renderWithRouter } from '../test-utils'
import { Settings } from '../pages/Settings'
import { baseServer } from './fixtures'

vi.mock('../lib/api', () => ({
  api: {
    get: vi.fn(),
    post: vi.fn(),
    put: vi.fn(),
    del: vi.fn(),
  },
}))

import { api } from '../lib/api'

const mockApi = vi.mocked(api)

beforeEach(() => {
  vi.clearAllMocks()
})

afterEach(() => {
  vi.restoreAllMocks()
})

describe('Settings', () => {
  it('shows loading state initially', () => {
    mockApi.get.mockReturnValue(new Promise(() => {}))
    renderWithRouter(<Settings />)
    expect(screen.getByText(/loading/i)).toBeDefined()
  })

  it('renders server list', async () => {
    const servers = [
      baseServer,
      { ...baseServer, id: 2, name: 'Jellyfin', type: 'jellyfin' as const, enabled: false },
    ]
    mockApi.get.mockResolvedValue(servers)
    renderWithRouter(<Settings />)
    await waitFor(() => {
      expect(screen.getByText('My Plex')).toBeDefined()
      expect(screen.getByText('Jellyfin')).toBeDefined()
    })
  })

  it('shows empty state when no servers', async () => {
    mockApi.get.mockResolvedValue([])
    renderWithRouter(<Settings />)
    await waitFor(() => {
      expect(screen.getByText(/no servers configured/i)).toBeDefined()
    })
  })

  it('shows enabled/disabled status badges', async () => {
    const servers = [
      baseServer,
      { ...baseServer, id: 2, name: 'Disabled One', enabled: false },
    ]
    mockApi.get.mockResolvedValue(servers)
    renderWithRouter(<Settings />)
    await waitFor(() => {
      expect(screen.getByText('Enabled')).toBeDefined()
      expect(screen.getByText('Disabled')).toBeDefined()
    })
  })

  it('shows server type badges', async () => {
    mockApi.get.mockResolvedValue([baseServer])
    renderWithRouter(<Settings />)
    await waitFor(() => {
      expect(screen.getByText('plex')).toBeDefined()
    })
  })

  it('opens add server form when clicking add button', async () => {
    mockApi.get.mockResolvedValue([])
    renderWithRouter(<Settings />)
    await waitFor(() => {
      expect(screen.getByText(/no servers configured/i)).toBeDefined()
    })
    fireEvent.click(screen.getByText(/add server/i))
    expect(screen.getByText(/new server/i)).toBeDefined()
  })

  it('opens edit form when clicking edit on a server', async () => {
    mockApi.get.mockResolvedValue([baseServer])
    renderWithRouter(<Settings />)
    await waitFor(() => {
      expect(screen.getByText('My Plex')).toBeDefined()
    })
    fireEvent.click(screen.getByRole('button', { name: /edit/i }))
    expect(screen.getByText(/edit server/i)).toBeDefined()
  })

  it('deletes a server after confirmation', async () => {
    mockApi.get.mockResolvedValue([baseServer])
    mockApi.del.mockResolvedValue(undefined)
    vi.spyOn(window, 'confirm').mockReturnValue(true)
    renderWithRouter(<Settings />)
    await waitFor(() => {
      expect(screen.getByText('My Plex')).toBeDefined()
    })
    fireEvent.click(screen.getByRole('button', { name: /delete/i }))
    await waitFor(() => {
      expect(mockApi.del).toHaveBeenCalledWith('/api/servers/1')
    })
  })

  it('does not delete when confirm is cancelled', async () => {
    mockApi.get.mockResolvedValue([baseServer])
    vi.spyOn(window, 'confirm').mockReturnValue(false)
    renderWithRouter(<Settings />)
    await waitFor(() => {
      expect(screen.getByText('My Plex')).toBeDefined()
    })
    fireEvent.click(screen.getByRole('button', { name: /delete/i }))
    expect(mockApi.del).not.toHaveBeenCalled()
  })

  it('shows error state on fetch failure', async () => {
    mockApi.get.mockRejectedValue(new Error('Network error'))
    renderWithRouter(<Settings />)
    await waitFor(() => {
      expect(screen.getByText(/failed to load/i)).toBeDefined()
    })
  })

  it('shows error when delete fails', async () => {
    mockApi.get.mockResolvedValue([baseServer])
    renderWithRouter(<Settings />)
    await waitFor(() => {
      expect(screen.getByText('My Plex')).toBeDefined()
    })
    mockApi.del.mockRejectedValue(new Error('Server error'))
    vi.spyOn(window, 'confirm').mockReturnValue(true)
    fireEvent.click(screen.getByRole('button', { name: /delete/i }))
    await waitFor(() => {
      expect(screen.getByText(/failed to delete/i)).toBeDefined()
    })
  })
})
