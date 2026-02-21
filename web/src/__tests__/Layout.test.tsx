import { describe, it, expect, vi, beforeEach } from 'vitest'
import { screen } from '@testing-library/react'
import { renderWithRouter } from '../test-utils'
import { Layout } from '../components/Layout'

vi.mock('../context/AuthContext', () => ({
  useAuth: vi.fn(),
}))

vi.mock('../hooks/useFetch', () => ({
  useFetch: vi.fn((url: string | null) => {
    if (url === '/api/sonarr/configured') return { data: { configured: false }, loading: false, error: null, refetch: vi.fn() }
    if (url === '/api/overseerr/configured') return { data: { configured: false }, loading: false, error: null, refetch: vi.fn() }
    if (url === '/api/settings/guest') return { data: { settings: {}, plex_tokens_available: false }, loading: false, error: null, refetch: vi.fn() }
    return { data: null, loading: false, error: null, refetch: vi.fn() }
  }),
}))

import { useAuth } from '../context/AuthContext'

const mockUseAuth = vi.mocked(useAuth)

const baseAuth = {
  loading: false,
  setupRequired: false,
  setUser: vi.fn(),
  clearSetupRequired: vi.fn(),
  refreshUser: vi.fn(),
  logout: vi.fn(),
}

const adminUser = {
  id: 1, name: 'admin', email: 'admin@test.local', role: 'admin' as const,
  thumb_url: '', has_password: true, created_at: '', updated_at: '',
}

const viewerUser = {
  ...adminUser, id: 2, name: 'viewer', role: 'viewer' as const,
}

describe('Layout', () => {
  beforeEach(() => {
    mockUseAuth.mockReturnValue({ ...baseAuth, user: adminUser })
  })

  it('renders sidebar nav links', () => {
    renderWithRouter(<Layout />)
    expect(screen.getAllByText('Dashboard').length).toBeGreaterThanOrEqual(1)
    expect(screen.getAllByText('History').length).toBeGreaterThanOrEqual(1)
    expect(screen.getAllByText('Settings').length).toBeGreaterThanOrEqual(1)
  })

  it('renders theme toggle', () => {
    renderWithRouter(<Layout />)
    expect(screen.getAllByRole('button', { name: /theme/i }).length).toBeGreaterThanOrEqual(1)
  })

  it('renders mobile nav', () => {
    renderWithRouter(<Layout />)
    const dashLinks = screen.getAllByText('Dashboard')
    expect(dashLinks.length).toBeGreaterThanOrEqual(2)
  })

  it('hides My Stats for admin users', () => {
    renderWithRouter(<Layout />)
    expect(screen.queryByText('My Stats')).toBeNull()
  })

  it('shows My Stats for viewer users', () => {
    mockUseAuth.mockReturnValue({ ...baseAuth, user: viewerUser })
    renderWithRouter(<Layout />)
    expect(screen.getAllByText('My Stats').length).toBeGreaterThanOrEqual(1)
  })
})
