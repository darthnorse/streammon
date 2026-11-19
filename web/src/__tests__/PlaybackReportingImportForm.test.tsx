import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { PlaybackReportingImportForm } from '../components/PlaybackReportingImportForm'
import type { Server } from '../types'

vi.mock('../lib/api', () => ({
  api: { uploadSSE: vi.fn() },
}))

vi.mock('../hooks/useFetch', () => ({
  useFetch: vi.fn(),
}))

vi.mock('../hooks/useSSEImport', () => ({
  useSSEImport: () => ({
    importing: false, importProgress: null, importResult: null, error: null,
    setError: vi.fn(), clearImport: vi.fn(), runSSEImport: vi.fn(),
  }),
}))

import { api } from '../lib/api'
import { useFetch } from '../hooks/useFetch'

const mockUploadSSE = vi.mocked(api.uploadSSE)
const mockUseFetch = vi.mocked(useFetch)

const server = (id: number, type: Server['type']): Server =>
  ({ id, name: `${type}-${id}`, type, url: 'http://x', enabled: true }) as Server

function renderWith(servers: Server[]) {
  mockUseFetch.mockReturnValue({ data: servers, loading: false, error: null, refetch: vi.fn() })
  return render(<PlaybackReportingImportForm onClose={vi.fn()} />)
}

describe('PlaybackReportingImportForm timezone', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mockUploadSSE.mockResolvedValue(new Response())
  })

  it("defaults the timezone to the browser's", () => {
    renderWith([server(3, 'jellyfin')])
    const browserTz = Intl.DateTimeFormat().resolvedOptions().timeZone
    expect(screen.getByLabelText(/server timezone/i)).toHaveValue(browserTz)
  })

  it('sends the selected timezone with the upload', async () => {
    renderWith([server(3, 'jellyfin')])
    fireEvent.change(screen.getByLabelText(/server timezone/i), { target: { value: 'America/Denver' } })
    fireEvent.change(screen.getByLabelText(/tsv file/i), {
      target: { files: [new File(['x'], 'pbr.tsv')] },
    })
    fireEvent.click(screen.getByRole('button', { name: 'Import' }))

    await waitFor(() => expect(mockUploadSSE).toHaveBeenCalled())
    const form = mockUploadSSE.mock.calls[0][1]
    expect(form.get('timezone')).toBe('America/Denver')
    expect(form.get('server_id')).toBe('3')
  })

  it('offers UTC even when the runtime list omits it', () => {
    renderWith([server(3, 'jellyfin')])
    expect(screen.getByRole('option', { name: 'UTC' })).toBeInTheDocument()
  })

  it('hides the timezone picker for Emby, whose exports carry their own offset', () => {
    renderWith([server(1, 'emby')])
    expect(screen.queryByLabelText(/server timezone/i)).not.toBeInTheDocument()
  })

  it('still renders when the browser lacks Intl.supportedValuesOf', async () => {
    const original = Intl.supportedValuesOf
    // @ts-expect-error simulating a browser without the API
    delete Intl.supportedValuesOf
    try {
      vi.resetModules()
      const { PlaybackReportingImportForm: Fresh } = await import('../components/PlaybackReportingImportForm')
      const { useFetch: freshUseFetch } = await import('../hooks/useFetch')
      vi.mocked(freshUseFetch).mockReturnValue({ data: [server(3, 'jellyfin')], loading: false, error: null, refetch: vi.fn() })
      render(<Fresh onClose={vi.fn()} />)
      expect(screen.getByRole('option', { name: 'UTC' })).toBeInTheDocument()
    } finally {
      Intl.supportedValuesOf = original
    }
  })
})
