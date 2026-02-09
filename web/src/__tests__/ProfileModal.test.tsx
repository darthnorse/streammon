import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { screen } from '@testing-library/react'
import { renderWithRouter } from '../test-utils'
import { ProfileModal } from '../components/ProfileModal'

vi.mock('../lib/api', () => ({
  api: {
    get: vi.fn(),
    post: vi.fn(),
    put: vi.fn(),
    del: vi.fn(),
  },
}))

vi.mock('../context/AuthContext', () => ({
  useAuth: vi.fn(),
}))

import { useAuth } from '../context/AuthContext'

const mockUseAuth = vi.mocked(useAuth)

function mockUser(overrides: Record<string, unknown> = {}) {
  return {
    user: {
      id: 1,
      name: 'testuser',
      email: 'test@example.com',
      role: 'viewer' as const,
      thumb_url: '',
      has_password: true,
      created_at: '',
      updated_at: '',
      ...overrides,
    },
    loading: false,
    setupRequired: false,
    setUser: vi.fn(),
    clearSetupRequired: vi.fn(),
    refreshUser: vi.fn(),
    logout: vi.fn(),
  }
}

beforeEach(() => {
  vi.clearAllMocks()
})

afterEach(() => {
  vi.restoreAllMocks()
})

describe('ProfileModal', () => {
  it('renders email form', () => {
    mockUseAuth.mockReturnValue(mockUser())

    renderWithRouter(<ProfileModal onClose={vi.fn()} />)

    expect(screen.getByLabelText('Email')).toBeInTheDocument()
    expect(screen.getByDisplayValue('test@example.com')).toBeInTheDocument()
  })

  it('renders username', () => {
    mockUseAuth.mockReturnValue(mockUser())

    renderWithRouter(<ProfileModal onClose={vi.fn()} />)

    expect(screen.getByText('testuser')).toBeInTheDocument()
  })

  it('shows password section when has_password is true', () => {
    mockUseAuth.mockReturnValue(mockUser({ has_password: true }))

    renderWithRouter(<ProfileModal onClose={vi.fn()} />)

    expect(screen.getByPlaceholderText('Current password')).toBeInTheDocument()
    expect(screen.getByPlaceholderText('New password (min 8 characters)')).toBeInTheDocument()
    expect(screen.getByPlaceholderText('Confirm new password')).toBeInTheDocument()
  })

  it('hides password section when has_password is false', () => {
    mockUseAuth.mockReturnValue(mockUser({ has_password: false }))

    renderWithRouter(<ProfileModal onClose={vi.fn()} />)

    expect(screen.queryByText('Change Password')).not.toBeInTheDocument()
    expect(screen.queryByPlaceholderText('Current password')).not.toBeInTheDocument()
  })

  it('shows initials when no thumb_url', () => {
    mockUseAuth.mockReturnValue(mockUser({ thumb_url: '' }))

    renderWithRouter(<ProfileModal onClose={vi.fn()} />)

    expect(screen.getByText('TE')).toBeInTheDocument()
  })

  it('shows avatar image when thumb_url present', () => {
    mockUseAuth.mockReturnValue(mockUser({ thumb_url: 'https://example.com/avatar.jpg' }))

    renderWithRouter(<ProfileModal onClose={vi.fn()} />)

    const img = screen.getByAltText('testuser') as HTMLImageElement
    expect(img.src).toBe('https://example.com/avatar.jpg')
  })
})
