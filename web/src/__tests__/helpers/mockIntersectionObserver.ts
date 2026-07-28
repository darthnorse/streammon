import { vi } from 'vitest'

/**
 * Models IntersectionObserver's transition semantics: observe() delivers the
 * current state (synchronously here, asynchronously in a real browser) and the
 * callback otherwise fires only when the state changes. Use setIntersecting to
 * drive scrolling; triggerIntersection fires an unconditional hit for the
 * simple cases and deliberately leaves the tracked state alone.
 */
export function setupIntersectionObserver(initiallyIntersecting = false) {
  const observers: Array<{ cb: IntersectionObserverCallback; active: boolean }> = []
  const disconnect = vi.fn()
  let state = initiallyIntersecting

  const emit = (cb: IntersectionObserverCallback, isIntersecting: boolean) =>
    cb([{ isIntersecting } as IntersectionObserverEntry], {} as IntersectionObserver)

  const emitAll = (isIntersecting: boolean) =>
    observers.filter(o => o.active).forEach(o => emit(o.cb, isIntersecting))

  vi.stubGlobal(
    'IntersectionObserver',
    vi.fn((cb: IntersectionObserverCallback) => {
      const entry = { cb, active: false }
      observers.push(entry)
      return {
        observe: vi.fn(() => {
          entry.active = true
          emit(cb, state)
        }),
        disconnect: vi.fn(() => {
          entry.active = false
          disconnect()
        }),
        unobserve: vi.fn(() => { entry.active = false }),
      }
    }),
  )

  return {
    triggerIntersection: () => emitAll(true),
    setIntersecting: (value: boolean) => {
      if (value === state) return
      state = value
      emitAll(value)
    },
    disconnect,
  }
}
