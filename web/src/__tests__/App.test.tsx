import { type ReactNode } from 'react'
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import App from '../App'

vi.mock('../hooks/useSSE', () => ({
  useSSE: () => ({ sessions: [], connected: false }),
}))

vi.mock('../hooks/useFetch', () => ({
  useFetch: () => ({ data: null, loading: false, error: null, refetch: () => {} }),
}))

vi.mock('../context/AuthContext', () => ({
  AuthProvider: ({ children }: { children: ReactNode }) => <>{children}</>,
  useAuth: vi.fn(),
}))

vi.mock('../components/AuthGuard', () => ({
  AuthGuard: ({ children }: { children: ReactNode }) => <>{children}</>,
}))

import { useAuth } from '../context/AuthContext'

const mockUseAuth = vi.mocked(useAuth)

describe('App routes', () => {
  beforeEach(() => {
    mockUseAuth.mockReturnValue({ user: { name: 'test', role: 'admin' }, loading: false } as ReturnType<typeof useAuth>)
  })

  it('renders dashboard at /', () => {
    render(
      <MemoryRouter initialEntries={['/']}>
        <App />
      </MemoryRouter>
    )
    expect(screen.getByText(/no active streams/i)).toBeInTheDocument()
  })

  it('renders history at /history', () => {
    render(
      <MemoryRouter initialEntries={['/history']}>
        <App />
      </MemoryRouter>
    )
    expect(screen.getByRole('heading', { name: 'History' })).toBeInTheDocument()
  })

  it('renders my-stats route with current user name', () => {
    render(
      <MemoryRouter initialEntries={['/my-stats']}>
        <App />
      </MemoryRouter>
    )
    // MyStats renders UserDetail with user.name from auth context ('test')
    expect(screen.getByRole('heading', { name: 'test' })).toBeInTheDocument()
  })

  it('renders user detail at /users/:name for an admin', () => {
    render(
      <MemoryRouter initialEntries={['/users/someone']}>
        <App />
      </MemoryRouter>
    )
    expect(screen.getByRole('heading', { name: 'someone' })).toBeInTheDocument()
  })

  it('redirects a non-admin away from /users/:name', () => {
    mockUseAuth.mockReturnValue({ user: { name: 'viewer1', role: 'viewer' }, loading: false } as ReturnType<typeof useAuth>)
    render(
      <MemoryRouter initialEntries={['/users/someone']}>
        <App />
      </MemoryRouter>
    )
    // AdminRoute redirects non-admins to /discover instead of rendering UserDetail
    expect(screen.queryByRole('heading', { name: 'someone' })).not.toBeInTheDocument()
    expect(screen.getByRole('heading', { name: 'Discover' })).toBeInTheDocument()
  })

  it('lets a non-admin reach their own stats via /my-stats', () => {
    mockUseAuth.mockReturnValue({ user: { name: 'viewer1', role: 'viewer' }, loading: false } as ReturnType<typeof useAuth>)
    render(
      <MemoryRouter initialEntries={['/my-stats']}>
        <App />
      </MemoryRouter>
    )
    expect(screen.getByRole('heading', { name: 'viewer1' })).toBeInTheDocument()
  })

  it('renders 404 for unknown routes', () => {
    render(
      <MemoryRouter initialEntries={['/nonexistent']}>
        <App />
      </MemoryRouter>
    )
    expect(screen.getByText('Page not found')).toBeInTheDocument()
  })
})
