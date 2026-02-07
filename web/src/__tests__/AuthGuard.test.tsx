import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import { MemoryRouter, Routes, Route } from 'react-router-dom'
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
})

function TestApp({ initialPath = '/' }: { initialPath?: string }) {
  return (
    <MemoryRouter initialEntries={[initialPath]}>
      <AuthProvider>
        <Routes>
          <Route path="/login" element={<div>Login page</div>} />
          <Route path="/setup" element={<div>Setup page</div>} />
          <Route
            path="/"
            element={
              <AuthGuard>
                <div>Protected content</div>
              </AuthGuard>
            }
          />
        </Routes>
      </AuthProvider>
    </MemoryRouter>
  )
}

describe('AuthGuard + AuthProvider', () => {
  it('shows loading state while checking auth', () => {
    mockApi.get.mockReturnValue(new Promise(() => {}))
    render(<TestApp />)
    expect(screen.getByText('Loading...')).toBeInTheDocument()
  })

  it('renders children when user is authenticated', async () => {
    mockApi.get
      .mockResolvedValueOnce({ setup_required: false, enabled_providers: [] })
      .mockResolvedValueOnce({ id: 1, name: 'test', role: 'admin' })
    render(<TestApp />)
    await waitFor(() => {
      expect(screen.getByText('Protected content')).toBeInTheDocument()
    })
  })

  it('redirects to login on 401', async () => {
    mockApi.get
      .mockResolvedValueOnce({ setup_required: false, enabled_providers: [] })
      .mockRejectedValueOnce(new ApiError(401, 'unauthorized'))
    render(<TestApp />)
    await waitFor(() => {
      expect(screen.getByText('Login page')).toBeInTheDocument()
    })
  })

  it('redirects to setup when setup is required', async () => {
    mockApi.get.mockResolvedValueOnce({ setup_required: true, enabled_providers: [] })
    render(<TestApp />)
    await waitFor(() => {
      expect(screen.getByText('Setup page')).toBeInTheDocument()
    })
  })
})
