import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, waitFor, fireEvent } from '@testing-library/react'
import { MapSettings } from '../components/MapSettings'
import { DEFAULT_TILE_URL } from '../lib/mapUtils'
import { api } from '../lib/api'

vi.mock('../lib/api', () => ({
  api: {
    get: vi.fn(),
    post: vi.fn(),
    put: vi.fn(),
    del: vi.fn(),
  },
}))

const mockApi = vi.mocked(api)

describe('MapSettings', () => {
  beforeEach(() => {
    localStorage.clear()
    vi.clearAllMocks()
    mockApi.put.mockResolvedValue(undefined)
    // The hook kicks off a display-settings fetch on mount; let it fail so the
    // cached (localStorage) values stay the single source of truth here.
    mockApi.get.mockRejectedValue(new Error('no server in tests'))
    vi.spyOn(console, 'warn').mockImplementation(() => {})
  })

  it('shows the default OpenStreetMap tile URL as a placeholder, not a value', () => {
    render(<MapSettings />)

    const input = screen.getByLabelText(/tile url/i)
    expect(input).toHaveValue('')
    expect(input).toHaveAttribute('placeholder', DEFAULT_TILE_URL)
  })

  it('saves a custom tile URL', async () => {
    render(<MapSettings />)

    fireEvent.change(screen.getByLabelText(/tile url/i), {
      target: { value: 'https://tiles.example.com/{z}/{x}/{y}.png' },
    })
    fireEvent.click(screen.getByRole('button', { name: /save/i }))

    await waitFor(() => {
      expect(mockApi.put).toHaveBeenCalledWith('/api/settings/display', {
        map_tile_url: 'https://tiles.example.com/{z}/{x}/{y}.png',
        map_tile_attribution: '',
      })
    })
  })

  it('rejects a tile URL missing the {z}/{x}/{y} placeholders without calling the API', async () => {
    render(<MapSettings />)

    fireEvent.change(screen.getByLabelText(/tile url/i), {
      target: { value: 'https://tiles.example.com/tiles.png' },
    })
    fireEvent.click(screen.getByRole('button', { name: /save/i }))

    expect(await screen.findByText(/\{z\}/)).toBeInTheDocument()
    expect(mockApi.put).not.toHaveBeenCalled()
  })

  it('rejects a non-https tile URL without calling the API', async () => {
    render(<MapSettings />)

    fireEvent.change(screen.getByLabelText(/tile url/i), {
      target: { value: 'http://tiles.example.com/{z}/{x}/{y}.png' },
    })
    fireEvent.click(screen.getByRole('button', { name: /save/i }))

    expect(await screen.findByText(/https/i)).toBeInTheDocument()
    expect(mockApi.put).not.toHaveBeenCalled()
  })

  it('clearing the tile URL restores the OpenStreetMap default', async () => {
    localStorage.setItem('streammon:map-tile-url', 'https://tiles.example.com/{z}/{x}/{y}.png')
    render(<MapSettings />)

    expect(screen.getByLabelText(/tile url/i)).toHaveValue('https://tiles.example.com/{z}/{x}/{y}.png')

    fireEvent.change(screen.getByLabelText(/tile url/i), { target: { value: '' } })
    fireEvent.click(screen.getByRole('button', { name: /save/i }))

    await waitFor(() => {
      expect(mockApi.put).toHaveBeenCalledWith('/api/settings/display', {
        map_tile_url: '',
        map_tile_attribution: '',
      })
    })
  })

  it('toggles the dark basemap filter immediately', async () => {
    render(<MapSettings />)

    const toggle = screen.getByRole('switch', { name: /darken/i })
    expect(toggle).toBeChecked()

    fireEvent.click(toggle)

    await waitFor(() => {
      expect(mockApi.put).toHaveBeenCalledWith('/api/settings/display', { map_dark_filter: false })
    })
    expect(toggle).not.toBeChecked()
  })

  it('refuses attribution containing markup', async () => {
    render(<MapSettings />)

    fireEvent.change(screen.getByLabelText(/attribution/i), {
      target: { value: '<b>Example</b>' },
    })
    fireEvent.click(screen.getByRole('button', { name: /save/i }))

    expect(await screen.findByText(/no HTML tags/i)).toBeInTheDocument()
    expect(mockApi.put).not.toHaveBeenCalled()
  })

  it('reports a failed save', async () => {
    mockApi.put.mockRejectedValue(new Error('nope'))
    render(<MapSettings />)

    fireEvent.change(screen.getByLabelText(/tile url/i), {
      target: { value: 'https://tiles.example.com/{z}/{x}/{y}.png' },
    })
    fireEvent.click(screen.getByRole('button', { name: /save/i }))

    expect(await screen.findByText(/nope/i)).toBeInTheDocument()
  })
})
