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
    vi.spyOn(window, 'confirm').mockReturnValue(true)
  })

  it('renders unconfigured state with Generate button', () => {
    mockUseFetch.mockReturnValue({ data: { configured: false }, loading: false, error: null, refetch: vi.fn() })
    render(<APIAccessSettings />)

    expect(screen.getByText('No key configured.')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Generate Key' })).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Revoke' })).not.toBeInTheDocument()
  })

  it('renders configured state with Rotate and Revoke buttons', () => {
    mockUseFetch.mockReturnValue({
      data: { configured: true, created_at: '2026-05-01T12:00:00Z' },
      loading: false, error: null, refetch: vi.fn(),
    })
    render(<APIAccessSettings />)

    expect(screen.getByText(/Active/)).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Rotate Key' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Revoke' })).toBeInTheDocument()
  })

  it('reveals plaintext key after rotate and hides on dismiss', async () => {
    const refetch = vi.fn()
    mockUseFetch.mockReturnValue({ data: { configured: false }, loading: false, error: null, refetch })
    vi.mocked(api.post).mockResolvedValue({ key: 'sm_abc123', created_at: '2026-05-01T12:00:00Z' })

    render(<APIAccessSettings />)

    fireEvent.click(screen.getByRole('button', { name: 'Generate Key' }))
    await waitFor(() => {
      expect(screen.getByText('sm_abc123')).toBeInTheDocument()
    })
    expect(screen.getByText(/Copy this key now/)).toBeInTheDocument()
    expect(refetch).toHaveBeenCalled()

    fireEvent.click(screen.getByRole('button', { name: 'Done' }))
    expect(screen.queryByText('sm_abc123')).not.toBeInTheDocument()
  })

  it('does not call API when user cancels rotate confirmation', async () => {
    vi.spyOn(window, 'confirm').mockReturnValue(false)
    mockUseFetch.mockReturnValue({ data: { configured: true }, loading: false, error: null, refetch: vi.fn() })

    render(<APIAccessSettings />)
    fireEvent.click(screen.getByRole('button', { name: 'Rotate Key' }))
    expect(api.post).not.toHaveBeenCalled()
  })

  it('calls revoke endpoint and refreshes', async () => {
    const refetch = vi.fn()
    mockUseFetch.mockReturnValue({ data: { configured: true }, loading: false, error: null, refetch })
    vi.mocked(api.del).mockResolvedValue(undefined)

    render(<APIAccessSettings />)
    fireEvent.click(screen.getByRole('button', { name: 'Revoke' }))

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
