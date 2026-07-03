import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { renderHook, act } from '@testing-library/react'
import { useSSE } from '../hooks/useSSE'
import { baseStream } from './fixtures'

class MockEventSource {
  static instances: MockEventSource[] = []
  onmessage: ((event: MessageEvent) => void) | null = null
  onerror: (() => void) | null = null
  onopen: (() => void) | null = null
  url: string
  readyState = 0
  closed = false

  constructor(url: string) {
    this.url = url
    MockEventSource.instances.push(this)
  }

  close() {
    this.closed = true
  }

  simulateMessage(data: string) {
    if (this.onmessage) {
      this.onmessage(new MessageEvent('message', { data }))
    }
  }
}

describe('useSSE', () => {
  beforeEach(() => {
    MockEventSource.instances = []
    vi.stubGlobal('EventSource', MockEventSource)
  })

  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('connects to SSE endpoint and returns empty sessions initially', () => {
    const { result } = renderHook(() => useSSE('/api/dashboard/sse'))
    expect(result.current.sessions).toEqual([])
    expect(MockEventSource.instances).toHaveLength(1)
    expect(MockEventSource.instances[0].url).toBe('/api/dashboard/sse')
  })

  it('updates sessions when message received', () => {
    const { result } = renderHook(() => useSSE('/api/dashboard/sse'))
    act(() => {
      MockEventSource.instances[0].simulateMessage(JSON.stringify([baseStream]))
    })
    expect(result.current.sessions).toEqual([baseStream])
  })

  it('ignores malformed JSON messages without crashing', () => {
    const { result } = renderHook(() => useSSE('/api/dashboard/sse'))
    act(() => {
      MockEventSource.instances[0].simulateMessage('not valid json{{{')
    })
    expect(result.current.sessions).toEqual([])
  })

  it('closes connection on unmount', () => {
    const { unmount } = renderHook(() => useSSE('/api/dashboard/sse'))
    const es = MockEventSource.instances[0]
    expect(es.closed).toBe(false)
    unmount()
    expect(es.closed).toBe(true)
  })

  it('interpolates progress every second for active sessions', () => {
    vi.useFakeTimers()
    const streamWithProgress = { ...baseStream, progress_ms: 5000, duration_ms: 100000 }
    const { result } = renderHook(() => useSSE('/api/dashboard/sse'))

    act(() => {
      MockEventSource.instances[0].simulateMessage(JSON.stringify([streamWithProgress]))
    })
    expect(result.current.sessions[0].progress_ms).toBe(5000)

    act(() => {
      vi.advanceTimersByTime(1000)
    })
    expect(result.current.sessions[0].progress_ms).toBe(6000)

    act(() => {
      vi.advanceTimersByTime(1000)
    })
    expect(result.current.sessions[0].progress_ms).toBe(7000)

    vi.useRealTimers()
  })

  it('does not interpolate past duration', () => {
    vi.useFakeTimers()
    const streamNearEnd = { ...baseStream, progress_ms: 99500, duration_ms: 100000 }
    const { result } = renderHook(() => useSSE('/api/dashboard/sse'))

    act(() => {
      MockEventSource.instances[0].simulateMessage(JSON.stringify([streamNearEnd]))
    })

    act(() => {
      vi.advanceTimersByTime(1000)
    })
    expect(result.current.sessions[0].progress_ms).toBe(100000)

    act(() => {
      vi.advanceTimersByTime(1000)
    })
    expect(result.current.sessions[0].progress_ms).toBe(100000)

    vi.useRealTimers()
  })

  it('interpolates live TV (duration=0) without limit', () => {
    vi.useFakeTimers()
    const liveStream = { ...baseStream, media_type: 'livetv' as const, progress_ms: 30000, duration_ms: 0 }
    const { result } = renderHook(() => useSSE('/api/dashboard/sse'))

    act(() => {
      MockEventSource.instances[0].simulateMessage(JSON.stringify([liveStream]))
    })
    expect(result.current.sessions[0].progress_ms).toBe(30000)

    act(() => {
      vi.advanceTimersByTime(1000)
    })
    expect(result.current.sessions[0].progress_ms).toBe(31000)

    act(() => {
      vi.advanceTimersByTime(1000)
    })
    expect(result.current.sessions[0].progress_ms).toBe(32000)

    vi.useRealTimers()
  })

  describe('interpolation array reference stability', () => {
    it('returns the same sessions array reference when every session is paused', () => {
      vi.useFakeTimers()
      const pausedStream = { ...baseStream, state: 'paused' as const, progress_ms: 5000, duration_ms: 100000 }
      const { result } = renderHook(() => useSSE('/api/dashboard/sse'))

      act(() => {
        MockEventSource.instances[0].simulateMessage(JSON.stringify([pausedStream]))
      })
      const before = result.current.sessions

      act(() => {
        vi.advanceTimersByTime(1000)
      })

      expect(result.current.sessions).toBe(before)
      expect(result.current.sessions[0].progress_ms).toBe(5000)

      vi.useRealTimers()
    })

    it('returns the same sessions array reference when a session is already at duration', () => {
      vi.useFakeTimers()
      const finishedStream = { ...baseStream, progress_ms: 100000, duration_ms: 100000 }
      const { result } = renderHook(() => useSSE('/api/dashboard/sse'))

      act(() => {
        MockEventSource.instances[0].simulateMessage(JSON.stringify([finishedStream]))
      })
      const before = result.current.sessions

      act(() => {
        vi.advanceTimersByTime(1000)
      })

      expect(result.current.sessions).toBe(before)

      vi.useRealTimers()
    })

    it('returns the same reference for an empty session list', () => {
      vi.useFakeTimers()
      const { result } = renderHook(() => useSSE('/api/dashboard/sse'))
      const before = result.current.sessions

      act(() => {
        vi.advanceTimersByTime(1000)
      })

      expect(result.current.sessions).toBe(before)

      vi.useRealTimers()
    })

    it('returns a new sessions array reference when a playing session actually progresses', () => {
      vi.useFakeTimers()
      const streamWithProgress = { ...baseStream, progress_ms: 5000, duration_ms: 100000 }
      const { result } = renderHook(() => useSSE('/api/dashboard/sse'))

      act(() => {
        MockEventSource.instances[0].simulateMessage(JSON.stringify([streamWithProgress]))
      })
      const before = result.current.sessions

      act(() => {
        vi.advanceTimersByTime(1000)
      })

      expect(result.current.sessions).not.toBe(before)

      vi.useRealTimers()
    })

    it('preserves individual object references for non-progressing sessions in a mixed set', () => {
      vi.useFakeTimers()
      const playing = { ...baseStream, session_id: 'p1', progress_ms: 1000, duration_ms: 100000 }
      const paused = { ...baseStream, session_id: 'p2', state: 'paused' as const, progress_ms: 2000, duration_ms: 100000 }
      const { result } = renderHook(() => useSSE('/api/dashboard/sse'))

      act(() => {
        MockEventSource.instances[0].simulateMessage(JSON.stringify([playing, paused]))
      })
      const pausedBefore = result.current.sessions.find(s => s.session_id === 'p2')

      act(() => {
        vi.advanceTimersByTime(1000)
      })

      const pausedAfter = result.current.sessions.find(s => s.session_id === 'p2')
      expect(pausedAfter).toBe(pausedBefore)

      vi.useRealTimers()
    })
  })
})
