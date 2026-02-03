import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import { AuthProvider } from '../context/AuthContext'
import { AuthGuard } from '../components/AuthGuard'

vi.mock('../lib/api', () => ({
  api: {
    get: vi.fn(),
    post: vi.fn(),
  },
  ApiError: class extends Error {
    status: number
    constructor(status: number, message: string) {
      super(message)
      this.status = status
      this.name = 'ApiError'
    }
  },
}))

import { api, ApiError } from '../lib/api'

const mockApi = vi.mocked(api)

beforeEach(() => {
  vi.clearAllMocks()
  Object.defineProperty(window, 'location', {
    value: { href: '/' },
    writable: true,
  })
})

function TestApp() {
  return (
    <AuthProvider>
      <AuthGuard>
        <div>Protected content</div>
      </AuthGuard>
    </AuthProvider>
  )
}

describe('AuthGuard + AuthProvider', () => {
  it('shows loading state while checking auth', () => {
    mockApi.get.mockReturnValue(new Promise(() => {}))
    render(<TestApp />)
    expect(screen.getByText('Loading...')).toBeInTheDocument()
  })

  it('renders children when user is authenticated', async () => {
    mockApi.get.mockResolvedValue({ id: 1, name: 'test', role: 'admin' })
    render(<TestApp />)
    await waitFor(() => {
      expect(screen.getByText('Protected content')).toBeInTheDocument()
    })
  })

  it('redirects to login on 401', async () => {
    mockApi.get.mockRejectedValue(new ApiError(401, 'unauthorized'))
    render(<TestApp />)
    await waitFor(() => {
      expect(window.location.href).toBe('/auth/login')
    })
  })
})
