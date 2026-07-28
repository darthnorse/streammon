import { vi } from 'vitest'

/**
 * observe() delivers the current state, then the callback fires only on
 * transitions — the real API's semantics, which stalls buggy infinite scroll.
 * triggerIntersection is an unconditional pulse and deliberately does not
 * touch the tracked state; tests that pulse it repeatedly depend on that.
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
