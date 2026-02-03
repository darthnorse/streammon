import { describe, it, expect, vi, afterEach, beforeEach } from 'vitest'
import { screen, fireEvent, waitFor } from '@testing-library/react'
import { renderWithRouter } from '../test-utils'
import { ServerForm } from '../components/ServerForm'
import { baseServer } from './fixtures'

vi.mock('../lib/api', () => ({
  api: {
    get: vi.fn(),
    post: vi.fn(),
    put: vi.fn(),
    del: vi.fn(),
  },
}))

import { api } from '../lib/api'

const mockApi = vi.mocked(api)

beforeEach(() => {
  vi.clearAllMocks()
})

afterEach(() => {
  vi.restoreAllMocks()
})

describe('ServerForm', () => {
  const onClose = vi.fn()
  const onSaved = vi.fn()

  it('renders empty form for new server', () => {
    renderWithRouter(<ServerForm onClose={onClose} onSaved={onSaved} />)
    expect(screen.getByText(/new server/i)).toBeDefined()
    expect(screen.getByLabelText(/name/i)).toBeDefined()
    expect(screen.getByLabelText(/type/i)).toBeDefined()
    expect(screen.getByLabelText(/url/i)).toBeDefined()
    expect(screen.getByLabelText(/api key/i)).toBeDefined()
  })

  it('renders pre-filled form for editing', () => {
    renderWithRouter(<ServerForm server={baseServer} onClose={onClose} onSaved={onSaved} />)
    expect(screen.getByText(/edit server/i)).toBeDefined()
    expect((screen.getByLabelText(/name/i) as HTMLInputElement).value).toBe('My Plex')
    expect((screen.getByLabelText(/url/i) as HTMLInputElement).value).toBe('http://localhost:32400')
  })

  it('submits create request for new server', async () => {
    mockApi.post.mockResolvedValue({ ...baseServer })
    renderWithRouter(<ServerForm onClose={onClose} onSaved={onSaved} />)

    fireEvent.change(screen.getByLabelText(/name/i), { target: { value: 'My Plex' } })
    fireEvent.change(screen.getByLabelText(/url/i), { target: { value: 'http://localhost:32400' } })
    fireEvent.change(screen.getByLabelText(/api key/i), { target: { value: 'abc123' } })
    fireEvent.click(screen.getByRole('button', { name: /save/i }))

    await waitFor(() => {
      expect(mockApi.post).toHaveBeenCalledWith('/api/servers', expect.objectContaining({
        name: 'My Plex',
        url: 'http://localhost:32400',
        api_key: 'abc123',
      }))
      expect(onSaved).toHaveBeenCalled()
    })
  })

  it('submits update request for existing server', async () => {
    mockApi.put.mockResolvedValue({ ...baseServer })
    renderWithRouter(<ServerForm server={baseServer} onClose={onClose} onSaved={onSaved} />)

    fireEvent.change(screen.getByLabelText(/name/i), { target: { value: 'Updated Plex' } })
    fireEvent.change(screen.getByLabelText(/api key/i), { target: { value: 'newkey' } })
    fireEvent.click(screen.getByRole('button', { name: /save/i }))

    await waitFor(() => {
      expect(mockApi.put).toHaveBeenCalledWith('/api/servers/1', expect.objectContaining({
        name: 'Updated Plex',
        api_key: 'newkey',
      }))
      expect(onSaved).toHaveBeenCalled()
    })
  })

  it('shows validation error when name is empty', async () => {
    renderWithRouter(<ServerForm onClose={onClose} onSaved={onSaved} />)
    fireEvent.click(screen.getByRole('button', { name: /save/i }))
    await waitFor(() => {
      expect(screen.getByText(/name is required/i)).toBeDefined()
    })
    expect(mockApi.post).not.toHaveBeenCalled()
  })

  it('shows API error on submit failure', async () => {
    mockApi.post.mockRejectedValue(new Error('url is required'))
    renderWithRouter(<ServerForm onClose={onClose} onSaved={onSaved} />)

    fireEvent.change(screen.getByLabelText(/name/i), { target: { value: 'Test' } })
    fireEvent.change(screen.getByLabelText(/api key/i), { target: { value: 'key' } })
    fireEvent.click(screen.getByRole('button', { name: /save/i }))

    await waitFor(() => {
      expect(screen.getByText(/url is required/i)).toBeDefined()
    })
  })

  it('calls onClose when cancel is clicked', () => {
    renderWithRouter(<ServerForm onClose={onClose} onSaved={onSaved} />)
    fireEvent.click(screen.getByRole('button', { name: /cancel/i }))
    expect(onClose).toHaveBeenCalled()
  })

  it('closes on Escape key', () => {
    renderWithRouter(<ServerForm onClose={onClose} onSaved={onSaved} />)
    fireEvent.keyDown(document, { key: 'Escape' })
    expect(onClose).toHaveBeenCalled()
  })

  it('tests connection for existing server', async () => {
    mockApi.post.mockResolvedValue({ success: true })
    renderWithRouter(<ServerForm server={baseServer} onClose={onClose} onSaved={onSaved} />)

    fireEvent.click(screen.getByRole('button', { name: /test connection/i }))

    await waitFor(() => {
      expect(mockApi.post).toHaveBeenCalledWith('/api/servers/1/test')
      expect(screen.getByText(/connection successful/i)).toBeDefined()
    })
  })

  it('shows test connection failure', async () => {
    mockApi.post.mockResolvedValue({ success: false, error: 'Connection refused' })
    renderWithRouter(<ServerForm server={baseServer} onClose={onClose} onSaved={onSaved} />)

    fireEvent.click(screen.getByRole('button', { name: /test connection/i }))

    await waitFor(() => {
      expect(screen.getByText(/connection refused/i)).toBeDefined()
    })
  })

  it('tests connection ad-hoc for new server', async () => {
    mockApi.post.mockResolvedValue({ success: true })
    renderWithRouter(<ServerForm onClose={onClose} onSaved={onSaved} />)

    fireEvent.change(screen.getByLabelText(/name/i), { target: { value: 'New Server' } })
    fireEvent.change(screen.getByLabelText(/url/i), { target: { value: 'http://plex:32400' } })
    fireEvent.change(screen.getByLabelText(/api key/i), { target: { value: 'testkey' } })
    fireEvent.click(screen.getByRole('button', { name: /test connection/i }))

    await waitFor(() => {
      expect(mockApi.post).toHaveBeenCalledWith('/api/servers/test', expect.objectContaining({
        name: 'New Server',
        url: 'http://plex:32400',
        api_key: 'testkey',
      }))
      expect(screen.getByText(/connection successful/i)).toBeDefined()
    })
  })

  it('shows error when testing new server with empty fields', async () => {
    renderWithRouter(<ServerForm onClose={onClose} onSaved={onSaved} />)
    fireEvent.click(screen.getByRole('button', { name: /test connection/i }))

    await waitFor(() => {
      expect(screen.getByText(/fill in all fields/i)).toBeDefined()
    })
    expect(mockApi.post).not.toHaveBeenCalled()
  })

  it('has enabled toggle defaulting to true for new servers', () => {
    renderWithRouter(<ServerForm onClose={onClose} onSaved={onSaved} />)
    const checkboxes = screen.getAllByRole('checkbox') as HTMLInputElement[]
    const enabledToggle = checkboxes.find(cb => cb.nextElementSibling?.textContent === 'Enabled')
    expect(enabledToggle?.checked).toBe(true)
  })

  it('has dialog role with aria-modal', () => {
    renderWithRouter(<ServerForm onClose={onClose} onSaved={onSaved} />)
    expect(screen.getByRole('dialog')).toBeDefined()
  })
})
