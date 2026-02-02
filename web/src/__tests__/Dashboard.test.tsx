import { describe, it, expect, vi, afterEach } from 'vitest'
import { screen } from '@testing-library/react'
import { renderWithRouter } from '../test-utils'
import { Dashboard } from '../pages/Dashboard'
import type { ActiveStream } from '../types'

vi.mock('../hooks/useSSE', () => ({
  useSSE: vi.fn(),
}))

import { useSSE } from '../hooks/useSSE'

const mockUseSSE = vi.mocked(useSSE)

afterEach(() => {
  vi.restoreAllMocks()
})

describe('Dashboard', () => {
  it('shows empty state when no sessions', () => {
    mockUseSSE.mockReturnValue({ sessions: [], connected: true })
    renderWithRouter(<Dashboard />)
    expect(screen.getByText(/no active streams/i)).toBeDefined()
  })

  it('renders stream cards for active sessions', () => {
    const stream: ActiveStream = {
      session_id: 's1',
      server_id: 1,
      server_name: 'Plex',
      user_name: 'bob',
      media_type: 'movie',
      title: 'The Matrix',
      parent_title: '',
      grandparent_title: '',
      year: 1999,
      duration_ms: 8160000,
      progress_ms: 2000000,
      player: 'Roku',
      platform: 'Roku',
      ip_address: '10.0.0.2',
      started_at: '2024-01-01T00:00:00Z',
    }
    mockUseSSE.mockReturnValue({ sessions: [stream], connected: true })
    renderWithRouter(<Dashboard />)
    expect(screen.getByText('bob')).toBeDefined()
    expect(screen.getByText('The Matrix')).toBeDefined()
  })

  it('shows stream count', () => {
    const streams: ActiveStream[] = [
      {
        session_id: 's1', server_id: 1, server_name: 'Plex', user_name: 'a',
        media_type: 'movie', title: 'A', parent_title: '', grandparent_title: '',
        year: 2024, duration_ms: 1000, progress_ms: 500, player: 'x', platform: 'x',
        ip_address: '1.1.1.1', started_at: '2024-01-01T00:00:00Z',
      },
      {
        session_id: 's2', server_id: 1, server_name: 'Plex', user_name: 'b',
        media_type: 'episode', title: 'B', parent_title: '', grandparent_title: '',
        year: 2024, duration_ms: 1000, progress_ms: 500, player: 'x', platform: 'x',
        ip_address: '2.2.2.2', started_at: '2024-01-01T00:00:00Z',
      },
    ]
    mockUseSSE.mockReturnValue({ sessions: streams, connected: true })
    renderWithRouter(<Dashboard />)
    expect(screen.getByText(/2 active streams/)).toBeDefined()
  })
})
