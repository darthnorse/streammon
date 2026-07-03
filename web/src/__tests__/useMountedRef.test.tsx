import { describe, it, expect } from 'vitest'
import { StrictMode } from 'react'
import { renderHook } from '@testing-library/react'
import { useMountedRef } from '../hooks/useMountedRef'

describe('useMountedRef', () => {
  it('is true immediately after mount', () => {
    const { result } = renderHook(() => useMountedRef())
    expect(result.current.current).toBe(true)
  })

  it('flips to false after unmount', () => {
    const { result, unmount } = renderHook(() => useMountedRef())
    unmount()
    expect(result.current.current).toBe(false)
  })

  it('stays true across a StrictMode dev double-invoke remount', () => {
    const { result } = renderHook(() => useMountedRef(), {
      wrapper: ({ children }) => <StrictMode>{children}</StrictMode>,
    })
    expect(result.current.current).toBe(true)
  })
})
