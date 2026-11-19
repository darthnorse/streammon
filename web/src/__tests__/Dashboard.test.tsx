import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { screen } from '@testing-library/react'
import { renderWithRouter, makeAuthContext } from '../test-utils'
import { Dashboard } from '../pages/Dashboard'
import { baseStream } from './fixtures'

vi.mock('../context/SessionsContext', () => ({
  useSessions: vi.fn(),
}))

vi.mock('../components/ServerHealthBanner', () => ({
  ServerHealthBanner: () => <div data-testid="server-health-banner" />,
}))

vi.mock('../context/AuthContext', async (importOriginal) => ({
  ...(await importOriginal<typeof import('../context/AuthContext')>()),
  useAuth: vi.fn(),
}))

import { useSessions } from '../context/SessionsContext'
import { useAuth } from '../context/AuthContext'

const mockUseSessions = vi.mocked(useSessions)
const mockUseAuth = vi.mocked(useAuth)

beforeEach(() => {
  mockUseAuth.mockReturnValue(makeAuthContext('viewer'))
})

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

  it('shows the server health banner to admins only', () => {
    mockUseSessions.mockReturnValue({ sessions: [], connected: true })
    mockUseAuth.mockReturnValue(makeAuthContext('admin'))
    const { unmount } = renderWithRouter(<Dashboard />)
    expect(screen.getByTestId('server-health-banner')).toBeInTheDocument()
    unmount()

    mockUseAuth.mockReturnValue(makeAuthContext('viewer'))
    renderWithRouter(<Dashboard />)
    expect(screen.queryByTestId('server-health-banner')).not.toBeInTheDocument()
  })
})
