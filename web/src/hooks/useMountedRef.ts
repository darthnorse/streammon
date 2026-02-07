import { useRef, useEffect } from 'react'

/**
 * Returns a ref that tracks whether the component is still mounted.
 * Use this to guard async state updates after component unmount.
 */
export function useMountedRef() {
  const mountedRef = useRef(true)

  useEffect(() => {
    return () => {
      mountedRef.current = false
    }
  }, [])

  return mountedRef
}
