import { describe, it, expect, vi, afterEach } from 'vitest'
import { screen } from '@testing-library/react'
import { renderWithRouter } from '../test-utils'
import { Dashboard } from '../pages/Dashboard'
import { baseStream } from './fixtures'

vi.mock('../context/SessionsContext', () => ({
  useSessions: vi.fn(),
}))

import { useSessions } from '../context/SessionsContext'

const mockUseSessions = vi.mocked(useSessions)

afterEach(() => {
  vi.restoreAllMocks()
})

describe('Dashboard', () => {
  it('shows empty state when no sessions', () => {
    mockUseSessions.mockReturnValue({ sessions: [], connected: true })
    renderWithRouter(<Dashboard />)
    expect(screen.getByText(/no active streams/i)).toBeDefined()
  })

  it('renders stream cards for active sessions', () => {
    const stream = { ...baseStream, user_name: 'bob', title: 'The Matrix', year: 1999 }
    mockUseSessions.mockReturnValue({ sessions: [stream], connected: true })
    renderWithRouter(<Dashboard />)
    expect(screen.getByText('bob')).toBeDefined()
    expect(screen.getByText('The Matrix')).toBeDefined()
  })

  it('shows stream count', () => {
    mockUseSessions.mockReturnValue({
      sessions: [
        { ...baseStream, session_id: 's1', user_name: 'a' },
        { ...baseStream, session_id: 's2', user_name: 'b', media_type: 'episode' },
      ],
      connected: true,
    })
    renderWithRouter(<Dashboard />)
    expect(screen.getByText(/2 active streams/)).toBeDefined()
  })
})
