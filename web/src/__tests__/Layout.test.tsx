import { describe, it, expect, vi } from 'vitest'
import { screen } from '@testing-library/react'
import { renderWithRouter } from '../test-utils'
import { Layout } from '../components/Layout'

vi.mock('../context/AuthContext', () => ({
  useAuth: () => ({
    user: {
      id: 1,
      name: 'admin',
      email: 'admin@test.local',
      role: 'admin',
      thumb_url: '',
      has_password: true,
      created_at: '',
      updated_at: '',
    },
    loading: false,
    setupRequired: false,
    setUser: vi.fn(),
    clearSetupRequired: vi.fn(),
    refreshUser: vi.fn(),
    logout: vi.fn(),
  }),
}))

describe('Layout', () => {
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
})
