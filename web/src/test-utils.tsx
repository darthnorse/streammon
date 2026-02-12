import { render, RenderOptions } from '@testing-library/react'
import { BrowserRouter } from 'react-router-dom'
import { ReactElement } from 'react'
import { vi } from 'vitest'
import type { Role } from './types'

function Wrapper({ children }: { children: React.ReactNode }) {
  return <BrowserRouter>{children}</BrowserRouter>
}

function renderWithRouter(ui: ReactElement, options?: Omit<RenderOptions, 'wrapper'>) {
  return render(ui, { wrapper: Wrapper, ...options })
}

function makeAuthContext(role: Role = 'admin') {
  return {
    user: { id: role === 'admin' ? 1 : 2, name: role, email: '', role, thumb_url: '', has_password: false, created_at: '', updated_at: '' },
    loading: false,
    setupRequired: false,
    setUser: vi.fn(),
    clearSetupRequired: vi.fn(),
    refreshUser: vi.fn(),
    logout: vi.fn(),
  }
}

function makeMockApiGet(mockApi: { get: { mockImplementation: (fn: never) => void } }) {
  return function mockApiGet(overrides: Record<string, unknown> = {}) {
    const defaults = overrides
    mockApi.get.mockImplementation(((url: string) => {
      for (const [key, val] of Object.entries(defaults)) {
        if (url === key || url.startsWith(key + '?')) return Promise.resolve(val)
      }
      return Promise.resolve(null)
    }) as never)
  }
}

export { renderWithRouter, render, makeAuthContext, makeMockApiGet }
export { screen, waitFor, act } from '@testing-library/react'
