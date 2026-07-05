import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, waitFor, act } from '@testing-library/react'
import { StreamLocationMap } from '../components/StreamLocationMap'
import { baseStream } from './fixtures'
import type { GeoResult } from '../types'

vi.mock('../lib/api', () => ({
  api: { get: vi.fn() },
}))

interface RenderedLocation extends GeoResult {
  users?: string[]
}

vi.mock('../components/shared/LeafletMap', () => ({
  LeafletMap: ({ locations }: { locations: RenderedLocation[] }) => (
    <div data-testid="map">
      {locations.map(loc => (
        <div key={loc.ip} data-testid="location">
          <span data-testid="location-ip">{loc.ip}</span>
          <span data-testid="location-users">{(loc.users ?? []).join(', ')}</span>
        </div>
      ))}
    </div>
  ),
}))

import { api } from '../lib/api'
const mockGet = vi.mocked(api.get)

function geo(ip: string): GeoResult {
  return { ip, lat: 1, lng: 2, city: 'City', country: 'US' }
}

describe('StreamLocationMap', () => {
  beforeEach(() => {
    mockGet.mockReset()
  })

  it('renders nothing when there are no sessions', () => {
    const { container } = render(<StreamLocationMap sessions={[]} />)
    expect(container).toBeEmptyDOMElement()
    expect(mockGet).not.toHaveBeenCalled()
  })

  it('fetches geo data for each unique session IP and renders a marker', async () => {
    mockGet.mockResolvedValue(geo('10.0.0.1'))
    render(<StreamLocationMap sessions={[baseStream]} />)

    await waitFor(() => expect(screen.getByTestId('location')).toHaveTextContent('10.0.0.1'))
    expect(mockGet).toHaveBeenCalledTimes(1)
    expect(mockGet).toHaveBeenCalledWith('/api/geoip/10.0.0.1')
  })

  it('does not refetch when the sessions array is replaced but the IP set is unchanged', async () => {
    mockGet.mockResolvedValue(geo('10.0.0.1'))
    const { rerender } = render(<StreamLocationMap sessions={[baseStream]} />)
    await waitFor(() => expect(mockGet).toHaveBeenCalledTimes(1))

    // Simulate a 1Hz interpolation tick: new array + new session object, same IP
    const ticked = { ...baseStream, progress_ms: baseStream.progress_ms + 1000 }
    rerender(<StreamLocationMap sessions={[ticked]} />)

    await waitFor(() => expect(screen.getByTestId('location')).toBeInTheDocument())
    expect(mockGet).toHaveBeenCalledTimes(1)
  })

  it('refreshes popup metadata immediately when session data changes at a known IP, without refetching geo', async () => {
    mockGet.mockResolvedValue(geo('10.0.0.1'))
    const { rerender } = render(<StreamLocationMap sessions={[baseStream]} />)
    await waitFor(() =>
      expect(screen.getByTestId('location-users')).toHaveTextContent('alice: Inception')
    )
    expect(mockGet).toHaveBeenCalledTimes(1)

    // Same IP set (10.0.0.1), but a different user/title now streaming from it —
    // e.g. a new viewer took over the session, or the title changed.
    const updated = { ...baseStream, user_name: 'bob', title: 'The Matrix', grandparent_title: '' }
    rerender(<StreamLocationMap sessions={[updated]} />)

    await waitFor(() =>
      expect(screen.getByTestId('location-users')).toHaveTextContent('bob: The Matrix')
    )
    expect(screen.queryByText(/alice: Inception/)).not.toBeInTheDocument()
    // Geo lookup is still cached per IP — no new network call for the same IP.
    expect(mockGet).toHaveBeenCalledTimes(1)
  })

  it('reflects a second viewer joining an already-known IP without refetching geo', async () => {
    mockGet.mockResolvedValue(geo('10.0.0.1'))
    const { rerender } = render(<StreamLocationMap sessions={[baseStream]} />)
    await waitFor(() =>
      expect(screen.getByTestId('location-users')).toHaveTextContent('alice: Inception')
    )
    expect(mockGet).toHaveBeenCalledTimes(1)

    const second = {
      ...baseStream,
      session_id: 's2',
      user_name: 'carol',
      title: 'The Matrix',
      grandparent_title: '',
    }
    rerender(<StreamLocationMap sessions={[baseStream, second]} />)

    await waitFor(() =>
      expect(screen.getByTestId('location-users')).toHaveTextContent('alice: Inception, carol: The Matrix')
    )
    expect(screen.getAllByTestId('location')).toHaveLength(1)
    expect(mockGet).toHaveBeenCalledTimes(1)
  })

  it('fetches again when a new IP appears', async () => {
    mockGet.mockImplementation((url: string) => Promise.resolve(geo(url.split('/').pop() as string)))
    const { rerender } = render(<StreamLocationMap sessions={[baseStream]} />)
    await waitFor(() => expect(mockGet).toHaveBeenCalledTimes(1))

    const other = { ...baseStream, session_id: 's2', ip_address: '10.0.0.2' }
    rerender(<StreamLocationMap sessions={[baseStream, other]} />)

    await waitFor(() => expect(mockGet).toHaveBeenCalledTimes(2))
    expect(mockGet).toHaveBeenCalledWith('/api/geoip/10.0.0.2')
  })

  it('ignores a stale response after the IP set has changed before it resolves', async () => {
    let resolveFirst: (v: GeoResult) => void = () => {}
    const firstPromise = new Promise<GeoResult>(res => { resolveFirst = res })
    mockGet.mockReturnValueOnce(firstPromise)

    const { rerender } = render(<StreamLocationMap sessions={[baseStream]} />)
    await waitFor(() => expect(mockGet).toHaveBeenCalledTimes(1))

    // Before the first (slow) request resolves, the IP set changes entirely.
    mockGet.mockResolvedValueOnce(geo('10.0.0.2'))
    const other = { ...baseStream, session_id: 's2', ip_address: '10.0.0.2' }
    rerender(<StreamLocationMap sessions={[other]} />)

    await waitFor(() => expect(screen.getByTestId('location')).toHaveTextContent('10.0.0.2'))

    // Now let the superseded first request resolve out of order.
    await act(async () => {
      resolveFirst(geo('10.0.0.1'))
      await firstPromise
    })

    expect(screen.queryByText('10.0.0.1')).not.toBeInTheDocument()
    expect(screen.getByTestId('location')).toHaveTextContent('10.0.0.2')
  })

  it('does not throw when a pending fetch resolves after unmount', async () => {
    let resolveFn: (v: GeoResult) => void = () => {}
    const pending = new Promise<GeoResult>(res => { resolveFn = res })
    mockGet.mockReturnValue(pending)

    const { unmount } = render(<StreamLocationMap sessions={[baseStream]} />)
    unmount()

    await expect(
      act(async () => {
        resolveFn(geo('10.0.0.1'))
        await pending
      })
    ).resolves.not.toThrow()
  })
})
