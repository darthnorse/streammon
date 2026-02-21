import { renderHook, act } from '@testing-library/react'
import { useModalStack } from '../hooks/useModalStack'
import type { ModalEntry } from '../types'

describe('useModalStack', () => {
  it('starts with empty stack and null current', () => {
    const { result } = renderHook(() => useModalStack())
    expect(result.current.stack).toEqual([])
    expect(result.current.current).toBeNull()
  })

  it('pushes entries and returns the top as current', () => {
    const { result } = renderHook(() => useModalStack())
    const person: ModalEntry = { type: 'person', personId: 42 }
    const tmdb: ModalEntry = { type: 'tmdb', mediaType: 'movie', mediaId: 100 }

    act(() => result.current.push(person))
    expect(result.current.stack).toEqual([person])
    expect(result.current.current).toEqual(person)

    act(() => result.current.push(tmdb))
    expect(result.current.stack).toEqual([person, tmdb])
    expect(result.current.current).toEqual(tmdb)
  })

  it('pops entries in LIFO order', () => {
    const { result } = renderHook(() => useModalStack())
    const person: ModalEntry = { type: 'person', personId: 1 }
    const tmdb: ModalEntry = { type: 'tmdb', mediaType: 'tv', mediaId: 2 }

    act(() => {
      result.current.push(person)
      result.current.push(tmdb)
    })

    act(() => result.current.pop())
    expect(result.current.current).toEqual(person)
    expect(result.current.stack).toHaveLength(1)

    act(() => result.current.pop())
    expect(result.current.current).toBeNull()
    expect(result.current.stack).toHaveLength(0)
  })

  it('pop on empty stack is a no-op', () => {
    const { result } = renderHook(() => useModalStack())
    act(() => result.current.pop())
    expect(result.current.stack).toEqual([])
    expect(result.current.current).toBeNull()
  })

  it('handles arbitrary depth Person→TMDB→Person→TMDB chain', () => {
    const { result } = renderHook(() => useModalStack())
    const entries: ModalEntry[] = [
      { type: 'person', personId: 10 },
      { type: 'tmdb', mediaType: 'movie', mediaId: 20 },
      { type: 'person', personId: 30 },
      { type: 'tmdb', mediaType: 'tv', mediaId: 40 },
    ]

    act(() => entries.forEach(e => result.current.push(e)))
    expect(result.current.stack).toEqual(entries)
    expect(result.current.current).toEqual(entries[3])

    // Pop back through the chain
    for (let i = entries.length - 1; i >= 0; i--) {
      expect(result.current.current).toEqual(entries[i])
      act(() => result.current.pop())
    }
    expect(result.current.current).toBeNull()
  })

  describe('max depth', () => {
    it('ignores push beyond 20 entries', () => {
      const { result } = renderHook(() => useModalStack())

      act(() => {
        for (let i = 0; i < 25; i++) {
          result.current.push({ type: 'person', personId: i })
        }
      })

      expect(result.current.stack).toHaveLength(20)
      expect(result.current.current).toEqual({ type: 'person', personId: 19 })
    })

    it('allows push after popping below max depth', () => {
      const { result } = renderHook(() => useModalStack())

      act(() => {
        for (let i = 0; i < 20; i++) {
          result.current.push({ type: 'person', personId: i })
        }
      })
      expect(result.current.stack).toHaveLength(20)

      act(() => result.current.pop())
      expect(result.current.stack).toHaveLength(19)

      act(() => result.current.push({ type: 'tmdb', mediaType: 'movie', mediaId: 999 }))
      expect(result.current.stack).toHaveLength(20)
      expect(result.current.current).toEqual({ type: 'tmdb', mediaType: 'movie', mediaId: 999 })
    })
  })
})
