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
})
