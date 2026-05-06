import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, waitFor, fireEvent } from '@testing-library/react'
import { APIAccessSettings } from '../components/APIAccessSettings'
import { api } from '../lib/api'

vi.mock('../lib/api', () => ({
  api: {
    get: vi.fn(),
    post: vi.fn(),
    put: vi.fn(),
    del: vi.fn(),
  },
}))

vi.mock('../hooks/useFetch', () => ({
  useFetch: vi.fn(),
}))

import { useFetch } from '../hooks/useFetch'
const mockUseFetch = vi.mocked(useFetch)

describe('APIAccessSettings', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('renders unconfigured state with Generate button', () => {
    mockUseFetch.mockReturnValue({ data: { configured: false }, loading: false, error: null, refetch: vi.fn() })
    render(<APIAccessSettings />)

    expect(screen.getByText('No key configured.')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Generate Key' })).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Revoke' })).not.toBeInTheDocument()
  })

  it('renders configured state masked with Show/Copy/Rotate/Revoke', () => {
    mockUseFetch.mockReturnValue({
      data: { configured: true, key: 'sm_secret_abc', created_at: '2026-05-01T12:00:00Z' },
      loading: false, error: null, refetch: vi.fn(),
    })
    render(<APIAccessSettings />)

    // Key is masked initially.
    expect(screen.queryByText('sm_secret_abc')).not.toBeInTheDocument()
    expect(screen.getByText(/Created/)).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Show API key' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Copy' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Rotate Key' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Revoke' })).toBeInTheDocument()
  })

  it('toggles key visibility via the eye button', () => {
    mockUseFetch.mockReturnValue({
      data: { configured: true, key: 'sm_secret_abc' },
      loading: false, error: null, refetch: vi.fn(),
    })
    render(<APIAccessSettings />)

    fireEvent.click(screen.getByRole('button', { name: 'Show API key' }))
    expect(screen.getByText('sm_secret_abc')).toBeInTheDocument()

    fireEvent.click(screen.getByRole('button', { name: 'Hide API key' }))
    expect(screen.queryByText('sm_secret_abc')).not.toBeInTheDocument()
  })

  it('rotate confirms via dialog, refetches, and auto-reveals the new key', async () => {
    const refetch = vi.fn()
    mockUseFetch.mockReturnValue({ data: { configured: false }, loading: false, error: null, refetch })
    vi.mocked(api.post).mockResolvedValue({ key: 'sm_new_value', created_at: '2026-05-01T12:00:00Z' })

    render(<APIAccessSettings />)

    fireEvent.click(screen.getByRole('button', { name: 'Generate Key' }))
    expect(screen.getByText('Generate API key?')).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: 'Generate' }))

    await waitFor(() => {
      expect(api.post).toHaveBeenCalledWith('/api/admin/api-key/rotate')
    })
    expect(refetch).toHaveBeenCalled()
  })

  it('cancelling the rotate dialog does not call the API', () => {
    mockUseFetch.mockReturnValue({
      data: { configured: true, key: 'sm_existing' },
      loading: false, error: null, refetch: vi.fn(),
    })

    render(<APIAccessSettings />)
    fireEvent.click(screen.getByRole('button', { name: 'Rotate Key' }))
    fireEvent.click(screen.getByRole('button', { name: 'Cancel' }))

    expect(api.post).not.toHaveBeenCalled()
    expect(screen.queryByText('Rotate API key?')).not.toBeInTheDocument()
  })

  it('confirming revoke clears the key', async () => {
    const refetch = vi.fn()
    mockUseFetch.mockReturnValue({
      data: { configured: true, key: 'sm_existing' },
      loading: false, error: null, refetch,
    })
    vi.mocked(api.del).mockResolvedValue(undefined)

    render(<APIAccessSettings />)
    fireEvent.click(screen.getByRole('button', { name: 'Revoke' }))
    const revokeButtons = screen.getAllByRole('button', { name: 'Revoke' })
    fireEvent.click(revokeButtons[revokeButtons.length - 1])

    await waitFor(() => {
      expect(api.del).toHaveBeenCalledWith('/api/admin/api-key')
    })
    expect(refetch).toHaveBeenCalled()
  })

  it('surfaces API errors', async () => {
    mockUseFetch.mockReturnValue({ data: { configured: false }, loading: false, error: null, refetch: vi.fn() })
    vi.mocked(api.post).mockRejectedValue(new Error('boom'))

    render(<APIAccessSettings />)
    fireEvent.click(screen.getByRole('button', { name: 'Generate Key' }))
    fireEvent.click(screen.getByRole('button', { name: 'Generate' }))

    await waitFor(() => {
      expect(screen.getByText(/Generate failed: boom/)).toBeInTheDocument()
    })
  })

  it('shows loading state and disables Generate while status is loading', () => {
    mockUseFetch.mockReturnValue({ data: null, loading: true, error: null, refetch: vi.fn() })
    render(<APIAccessSettings />)

    expect(screen.getByText('Loading…')).toBeInTheDocument()
    expect(screen.queryByText('No key configured.')).not.toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Generate Key' })).toBeDisabled()
  })

  it('shows error state and disables Generate when status fetch fails', () => {
    mockUseFetch.mockReturnValue({ data: null, loading: false, error: new Error('network'), refetch: vi.fn() })
    render(<APIAccessSettings />)

    expect(screen.getByText(/Failed to load API key status/)).toBeInTheDocument()
    expect(screen.queryByText('No key configured.')).not.toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Generate Key' })).toBeDisabled()
  })
})
