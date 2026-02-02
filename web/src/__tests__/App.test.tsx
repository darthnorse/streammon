import { describe, it, expect, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import App from '../App'

vi.mock('../hooks/useSSE', () => ({
  useSSE: () => ({ sessions: [], connected: false }),
}))

vi.mock('../hooks/useFetch', () => ({
  useFetch: () => ({ data: null, loading: false, error: null }),
}))

describe('App routes', () => {
  it('renders dashboard at /', () => {
    render(
      <MemoryRouter initialEntries={['/']}>
        <App />
      </MemoryRouter>
    )
    expect(screen.getByRole('heading', { name: 'Dashboard' })).toBeDefined()
  })

  it('renders history at /history', () => {
    render(
      <MemoryRouter initialEntries={['/history']}>
        <App />
      </MemoryRouter>
    )
    expect(screen.getByRole('heading', { name: 'History' })).toBeDefined()
  })

  it('renders 404 for unknown routes', () => {
    render(
      <MemoryRouter initialEntries={['/nonexistent']}>
        <App />
      </MemoryRouter>
    )
    expect(screen.getByText('Page not found')).toBeDefined()
  })
})
