import { useState, useCallback } from 'react'
import type { ModalEntry } from '../types'

const MAX_DEPTH = 20

export function useModalStack() {
  const [stack, setStack] = useState<ModalEntry[]>([])

  const current = stack.length > 0 ? stack[stack.length - 1] : null

  const push = useCallback((entry: ModalEntry) => {
    setStack(prev => (prev.length >= MAX_DEPTH ? prev : [...prev, entry]))
  }, [])

  const pop = useCallback(() => {
    setStack(prev => (prev.length > 0 ? prev.slice(0, -1) : prev))
  }, [])

  return { stack, current, push, pop }
}
