import { vi } from 'vitest'

export function setupIntersectionObserver() {
  let trigger: () => void = () => {}
  const disconnect = vi.fn()

  vi.stubGlobal(
    'IntersectionObserver',
    vi.fn((cb: IntersectionObserverCallback) => {
      trigger = () => {
        cb(
          [{ isIntersecting: true } as IntersectionObserverEntry],
          {} as IntersectionObserver,
        )
      }
      return { observe: vi.fn(), disconnect, unobserve: vi.fn() }
    }),
  )

  return {
    triggerIntersection: () => trigger(),
    disconnect,
  }
}
