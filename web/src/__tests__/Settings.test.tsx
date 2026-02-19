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

  it('shows delete confirmation modal with both options', async () => {
    mockApi.get.mockResolvedValue([baseServer])
    renderWithRouter(<Settings />)
    await waitFor(() => {
      expect(screen.getByText('My Plex')).toBeDefined()
    })
    fireEvent.click(screen.getByRole('button', { name: /delete/i }))
    expect(screen.getByText(/keep watch history/i)).toBeDefined()
    expect(screen.getByText(/delete everything/i)).toBeDefined()
  })

  it('soft deletes when keep history is selected', async () => {
    mockApi.get.mockResolvedValue([baseServer])
    mockApi.del.mockResolvedValue(undefined)
    renderWithRouter(<Settings />)
    await waitFor(() => {
      expect(screen.getByText('My Plex')).toBeDefined()
    })
    fireEvent.click(screen.getByRole('button', { name: /delete/i }))
    // "Keep watch history" is selected by default
    fireEvent.click(screen.getByText('Delete Server'))
    await waitFor(() => {
      expect(mockApi.del).toHaveBeenCalledWith('/api/servers/1?keep_history=true')
    })
  })

  it('hard deletes when delete everything is selected', async () => {
    mockApi.get.mockResolvedValue([baseServer])
    mockApi.del.mockResolvedValue(undefined)
    renderWithRouter(<Settings />)
    await waitFor(() => {
      expect(screen.getByText('My Plex')).toBeDefined()
    })
    fireEvent.click(screen.getByRole('button', { name: /delete/i }))
    // Select "Delete everything"
    fireEvent.click(screen.getByText(/delete everything/i))
    fireEvent.click(screen.getByText('Delete Everything'))
    await waitFor(() => {
      expect(mockApi.del).toHaveBeenCalledWith('/api/servers/1')
    })
  })

  it('cancels delete when modal is dismissed', async () => {
    mockApi.get.mockResolvedValue([baseServer])
    renderWithRouter(<Settings />)
    await waitFor(() => {
      expect(screen.getByText('My Plex')).toBeDefined()
    })
    fireEvent.click(screen.getByRole('button', { name: /delete/i }))
    fireEvent.click(screen.getByText('Cancel'))
    expect(mockApi.del).not.toHaveBeenCalled()
  })

  it('shows error state on fetch failure', async () => {
    mockApi.get.mockRejectedValue(new Error('Network error'))
    renderWithRouter(<Settings />)
    await waitFor(() => {
      expect(screen.getByText(/failed to load/i)).toBeDefined()
    })
  })

  it('shows error in modal when delete fails and keeps modal open', async () => {
    mockApi.get.mockResolvedValue([baseServer])
    renderWithRouter(<Settings />)
    await waitFor(() => {
      expect(screen.getByText('My Plex')).toBeDefined()
    })
    mockApi.del.mockRejectedValue(new Error('Server error'))
    fireEvent.click(screen.getByRole('button', { name: /delete/i }))
    fireEvent.click(screen.getByText('Delete Server'))
    await waitFor(() => {
      expect(screen.getByText(/failed to delete/i)).toBeDefined()
      // Modal should remain open — options still visible
      expect(screen.getByText(/keep watch history/i)).toBeDefined()
    })
  })

  it('shows deleted servers section with restore button', async () => {
    const deletedServer = { ...baseServer, id: 2, name: 'Old Emby', type: 'emby' as const, deleted_at: '2024-06-01T00:00:00Z' }
    mockApi.get.mockResolvedValue([baseServer, deletedServer])
    renderWithRouter(<Settings />)
    await waitFor(() => {
      expect(screen.getByText('My Plex')).toBeDefined()
      expect(screen.getByText('Old Emby')).toBeDefined()
      expect(screen.getByText('Deleted Servers')).toBeDefined()
      expect(screen.getByText('Restore')).toBeDefined()
    })
  })

  it('restores a deleted server', async () => {
    const deletedServer = { ...baseServer, id: 2, name: 'Old Emby', type: 'emby' as const, deleted_at: '2024-06-01T00:00:00Z' }
    mockApi.get.mockResolvedValue([baseServer, deletedServer])
    mockApi.post.mockResolvedValue({ ...deletedServer, deleted_at: undefined })
    renderWithRouter(<Settings />)
    await waitFor(() => {
      expect(screen.getByText('Restore')).toBeDefined()
    })
    fireEvent.click(screen.getByText('Restore'))
    await waitFor(() => {
      expect(mockApi.post).toHaveBeenCalledWith('/api/servers/2/restore', {})
    })
  })
})
