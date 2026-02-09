import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, screen, waitFor, act } from '@testing-library/react'
import { useInfiniteFetch } from '../hooks/useInfiniteFetch'
import { setupIntersectionObserver } from './helpers/mockIntersectionObserver'

vi.mock('../lib/api', () => ({
  api: { get: vi.fn() },
}))

import { api } from '../lib/api'

const mockGet = vi.mocked(api.get)

type Item = { id: number }

let triggerIntersection: () => void
let mockDisconnect: ReturnType<typeof vi.fn>

function makeResponse(ids: number[], totalPages: number, mode: 'offset' | 'page' = 'offset') {
  const results = ids.map(id => ({ id }))
  return mode === 'page'
    ? { results, totalPages }
    : { results, pageInfo: { pages: totalPages } }
}

function mockNextResponse(ids: number[], totalPages: number, mode: 'offset' | 'page' = 'offset') {
  mockGet.mockResolvedValueOnce(makeResponse(ids, totalPages, mode) as never)
}

function mockNextResponseDeferred(mode: 'offset' | 'page' = 'offset') {
  let resolve!: (value: unknown) => void
  mockGet.mockImplementationOnce(
    () => new Promise(r => { resolve = r }) as never,
  )
  return (ids: number[], totalPages: number) => resolve(makeResponse(ids, totalPages, mode))
}

function TestHarness({ url, pageSize, mode }: { url: string | null; pageSize: number; mode?: 'offset' | 'page' }) {
  const { items, loading, loadingMore, hasMore, error, sentinelRef, retry, refetch } =
    useInfiniteFetch<Item>(url, pageSize, mode)
  return (
    <div>
      {loading && <div data-testid="loading">Loading</div>}
      {items.map(item => (
        <div key={item.id} data-testid="item">
          {item.id}
        </div>
      ))}
      <div ref={sentinelRef} data-testid="sentinel" />
      {loadingMore && <div data-testid="loading-more">Loading more</div>}
      {error && <div data-testid="error">{error}</div>}
      {error && (
        <button data-testid="retry" onClick={retry}>
          Retry
        </button>
      )}
      <button data-testid="refetch" onClick={refetch}>
        Refetch
      </button>
      {!hasMore && !loading && <div data-testid="no-more">No more</div>}
    </div>
  )
}

describe('useInfiniteFetch', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    const observer = setupIntersectionObserver()
    triggerIntersection = observer.triggerIntersection
    mockDisconnect = observer.disconnect
  })

  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('fetches page 0 on mount and displays items', async () => {
    mockNextResponse([1, 2, 3], 2)

    render(<TestHarness url="/api/test" pageSize={3} />)

    expect(screen.getByTestId('loading')).toBeDefined()

    await waitFor(() => {
      expect(screen.queryByTestId('loading')).toBeNull()
    })

    expect(screen.getAllByTestId('item')).toHaveLength(3)
    expect(mockGet).toHaveBeenCalledWith(
      '/api/test?take=3&skip=0',
      expect.any(AbortSignal),
    )
  })

  it('does not fetch when url is null', async () => {
    render(<TestHarness url={null} pageSize={3} />)

    await waitFor(() => {
      expect(screen.queryByTestId('loading')).toBeNull()
    })

    expect(screen.queryAllByTestId('item')).toHaveLength(0)
    expect(mockGet).not.toHaveBeenCalled()
  })

  it('appends query param with & when baseUrl has ?', async () => {
    mockNextResponse([1], 1)

    render(<TestHarness url="/api/test?filter=all" pageSize={5} />)

    await waitFor(() => {
      expect(screen.queryByTestId('loading')).toBeNull()
    })

    expect(mockGet).toHaveBeenCalledWith(
      '/api/test?filter=all&take=5&skip=0',
      expect.any(AbortSignal),
    )
  })

  it('loads next page on intersection', async () => {
    mockNextResponse([1, 2], 3)

    render(<TestHarness url="/api/test" pageSize={2} />)

    await waitFor(() => {
      expect(screen.queryByTestId('loading')).toBeNull()
    })
    expect(screen.getAllByTestId('item')).toHaveLength(2)

    mockNextResponse([3, 4], 3)
    act(() => triggerIntersection())

    await waitFor(() => {
      expect(screen.getAllByTestId('item')).toHaveLength(4)
    })

    expect(mockGet).toHaveBeenCalledWith(
      '/api/test?take=2&skip=2',
      expect.any(AbortSignal),
    )
  })

  it('sets hasMore to false when last page is reached', async () => {
    mockNextResponse([1, 2], 1)

    render(<TestHarness url="/api/test" pageSize={2} />)

    await waitFor(() => {
      expect(screen.getByTestId('no-more')).toBeDefined()
    })
  })

  it('resets items on URL change', async () => {
    mockNextResponse([1, 2], 2)

    const { rerender } = render(
      <TestHarness url="/api/test?filter=all" pageSize={2} />,
    )

    await waitFor(() => {
      expect(screen.getAllByTestId('item')).toHaveLength(2)
    })

    mockNextResponse([5, 6], 1)
    rerender(<TestHarness url="/api/test?filter=pending" pageSize={2} />)

    await waitFor(() => {
      const items = screen.getAllByTestId('item')
      expect(items).toHaveLength(2)
      expect(items[0].textContent).toBe('5')
    })
  })

  it('shows error on fetch failure', async () => {
    mockGet.mockRejectedValueOnce(new Error('Network error') as never)

    render(<TestHarness url="/api/test" pageSize={2} />)

    await waitFor(() => {
      expect(screen.getByTestId('error')).toBeDefined()
    })

    expect(screen.getByTestId('error').textContent).toBe('Network error')
    expect(screen.getByTestId('no-more')).toBeDefined()
  })

  it('retries successfully after error', async () => {
    mockGet.mockRejectedValueOnce(new Error('Network error') as never)

    render(<TestHarness url="/api/test" pageSize={2} />)

    await waitFor(() => {
      expect(screen.getByTestId('error')).toBeDefined()
    })

    mockNextResponse([1, 2], 1)
    act(() => {
      screen.getByTestId('retry').click()
    })

    await waitFor(() => {
      expect(screen.queryByTestId('error')).toBeNull()
      expect(screen.getAllByTestId('item')).toHaveLength(2)
    })
  })

  it('aborts in-flight request on URL change', async () => {
    const resolveFirst = mockNextResponseDeferred()

    const { rerender } = render(
      <TestHarness url="/api/test?v=1" pageSize={2} />,
    )

    mockNextResponse([3, 4], 1)
    rerender(<TestHarness url="/api/test?v=2" pageSize={2} />)

    // Resolve the old request — should be ignored (aborted controller)
    act(() => resolveFirst([1, 2], 2))

    await waitFor(() => {
      expect(screen.queryByTestId('loading')).toBeNull()
    })

    const items = screen.getAllByTestId('item')
    expect(items).toHaveLength(2)
    expect(items[0].textContent).toBe('3')
  })

  it('prevents duplicate fetches on rapid intersection triggers', async () => {
    mockNextResponse([1, 2], 3)

    render(<TestHarness url="/api/test" pageSize={2} />)

    await waitFor(() => {
      expect(screen.queryByTestId('loading')).toBeNull()
    })

    const resolvePage2 = mockNextResponseDeferred()

    act(() => {
      triggerIntersection()
      triggerIntersection()
      triggerIntersection()
    })

    // Only one additional call (page 0 + page 1 = 2 total)
    expect(mockGet).toHaveBeenCalledTimes(2)

    act(() => resolvePage2([3, 4], 3))

    await waitFor(() => {
      expect(screen.getAllByTestId('item')).toHaveLength(4)
    })
  })

  it('shows error after later page failure with retry', async () => {
    mockNextResponse([1, 2], 3)

    render(<TestHarness url="/api/test" pageSize={2} />)

    await waitFor(() => {
      expect(screen.getAllByTestId('item')).toHaveLength(2)
    })

    mockGet.mockRejectedValueOnce(new Error('Timeout') as never)
    act(() => triggerIntersection())

    await waitFor(() => {
      expect(screen.getByTestId('error').textContent).toBe('Timeout')
    })

    expect(screen.getAllByTestId('item')).toHaveLength(2)

    mockNextResponse([3, 4], 3)
    act(() => {
      screen.getByTestId('retry').click()
    })

    await waitFor(() => {
      expect(screen.queryByTestId('error')).toBeNull()
      expect(screen.getAllByTestId('item')).toHaveLength(4)
    })
  })

  it('cleans up observer on unmount', async () => {
    mockNextResponse([1], 2)

    const { unmount } = render(<TestHarness url="/api/test" pageSize={2} />)

    await waitFor(() => {
      expect(screen.queryByTestId('loading')).toBeNull()
    })

    unmount()

    expect(mockDisconnect).toHaveBeenCalled()
  })

  it('refetch reloads from page 0 with same URL', async () => {
    mockNextResponse([1, 2], 2)

    render(<TestHarness url="/api/test" pageSize={2} />)

    await waitFor(() => {
      expect(screen.getAllByTestId('item')).toHaveLength(2)
    })

    mockNextResponse([3, 4], 2)
    act(() => triggerIntersection())

    await waitFor(() => {
      expect(screen.getAllByTestId('item')).toHaveLength(4)
    })

    mockNextResponse([10, 20], 1)
    act(() => {
      screen.getByTestId('refetch').click()
    })

    await waitFor(() => {
      const items = screen.getAllByTestId('item')
      expect(items).toHaveLength(2)
      expect(items[0].textContent).toBe('10')
    })
  })

  it('aborts in-flight request on unmount', async () => {
    const resolveFirst = mockNextResponseDeferred()

    const { unmount } = render(<TestHarness url="/api/test" pageSize={2} />)

    unmount()

    // Resolve after unmount — should not cause errors
    act(() => resolveFirst([1, 2], 1))
  })

  it('resets on pageSize change', async () => {
    mockNextResponse([1, 2], 2)

    const { rerender } = render(
      <TestHarness url="/api/test" pageSize={2} />,
    )

    await waitFor(() => {
      expect(screen.getAllByTestId('item')).toHaveLength(2)
    })

    mockNextResponse([1, 2, 3], 1)
    rerender(<TestHarness url="/api/test" pageSize={3} />)

    await waitFor(() => {
      expect(screen.getAllByTestId('item')).toHaveLength(3)
    })

    expect(mockGet).toHaveBeenLastCalledWith(
      '/api/test?take=3&skip=0',
      expect.any(AbortSignal),
    )
  })

  describe('page mode', () => {
    it('uses page=1 on mount (1-indexed)', async () => {
      mockNextResponse([1, 2, 3], 3, 'page')

      render(<TestHarness url="/api/test" pageSize={20} mode="page" />)

      await waitFor(() => {
        expect(screen.queryByTestId('loading')).toBeNull()
      })

      expect(screen.getAllByTestId('item')).toHaveLength(3)
      expect(mockGet).toHaveBeenCalledWith(
        '/api/test?page=1',
        expect.any(AbortSignal),
      )
    })

    it('accumulates items across pages', async () => {
      mockNextResponse([1, 2], 3, 'page')

      render(<TestHarness url="/api/test" pageSize={20} mode="page" />)

      await waitFor(() => {
        expect(screen.getAllByTestId('item')).toHaveLength(2)
      })

      mockNextResponse([3, 4], 3, 'page')
      act(() => triggerIntersection())

      await waitFor(() => {
        expect(screen.getAllByTestId('item')).toHaveLength(4)
      })

      expect(mockGet).toHaveBeenLastCalledWith(
        '/api/test?page=2',
        expect.any(AbortSignal),
      )
    })

    it('sets hasMore to false on last page', async () => {
      mockNextResponse([1, 2], 1, 'page')

      render(<TestHarness url="/api/test" pageSize={20} mode="page" />)

      await waitFor(() => {
        expect(screen.getByTestId('no-more')).toBeDefined()
      })

      expect(screen.getAllByTestId('item')).toHaveLength(2)
    })

    it('appends page param with & when baseUrl has ?', async () => {
      mockNextResponse([1], 1, 'page')

      render(<TestHarness url="/api/test?language=en" pageSize={20} mode="page" />)

      await waitFor(() => {
        expect(screen.queryByTestId('loading')).toBeNull()
      })

      expect(mockGet).toHaveBeenCalledWith(
        '/api/test?language=en&page=1',
        expect.any(AbortSignal),
      )
    })

    it('sets hasMore to false when totalPages is 0', async () => {
      mockNextResponse([], 0, 'page')

      render(<TestHarness url="/api/test" pageSize={20} mode="page" />)

      await waitFor(() => {
        expect(screen.queryByTestId('loading')).toBeNull()
      })

      expect(screen.queryAllByTestId('item')).toHaveLength(0)
      expect(screen.getByTestId('no-more')).toBeDefined()
    })
  })

  it('resets on mode change', async () => {
    mockNextResponse([1, 2], 2)

    const { rerender } = render(
      <TestHarness url="/api/test" pageSize={2} />,
    )

    await waitFor(() => {
      expect(screen.getAllByTestId('item')).toHaveLength(2)
    })

    mockNextResponse([5, 6], 1, 'page')
    rerender(<TestHarness url="/api/test" pageSize={2} mode="page" />)

    await waitFor(() => {
      const items = screen.getAllByTestId('item')
      expect(items).toHaveLength(2)
      expect(items[0].textContent).toBe('5')
    })

    expect(mockGet).toHaveBeenLastCalledWith(
      '/api/test?page=1',
      expect.any(AbortSignal),
    )
  })
})
