import { describe, it, expect, vi, afterEach } from 'vitest'
import { render } from '@testing-library/react'
import { TabTitle } from '../components/TabTitle'
import { baseStream } from './fixtures'

vi.mock('../context/SessionsContext', () => ({
  useSessions: vi.fn(),
}))

import { useSessions } from '../context/SessionsContext'
const mockUseSessions = vi.mocked(useSessions)

afterEach(() => {
  document.title = 'StreamMon'
  vi.restoreAllMocks()
})

describe('TabTitle', () => {
  it('keeps base title when no streams', () => {
    mockUseSessions.mockReturnValue({ sessions: [], connected: true })
    render(<TabTitle />)
    expect(document.title).toBe('StreamMon')
  })

  it('shows singular noun for one stream', () => {
    mockUseSessions.mockReturnValue({
      sessions: [{ ...baseStream, session_id: 's1' }],
      connected: true,
    })
    render(<TabTitle />)
    expect(document.title).toBe('StreamMon | 1 stream')
  })

  it('shows plural noun for multiple streams', () => {
    mockUseSessions.mockReturnValue({
      sessions: [
        { ...baseStream, session_id: 's1' },
        { ...baseStream, session_id: 's2' },
      ],
      connected: true,
    })
    render(<TabTitle />)
    expect(document.title).toBe('StreamMon | 2 streams')
  })

  it('resets title on unmount', () => {
    mockUseSessions.mockReturnValue({
      sessions: [{ ...baseStream, session_id: 's1' }],
      connected: true,
    })
    const { unmount } = render(<TabTitle />)
    expect(document.title).toBe('StreamMon | 1 stream')
    unmount()
    expect(document.title).toBe('StreamMon')
  })
})
