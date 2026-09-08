import { useEffect, useRef } from 'react'

export function useModal(onClose: () => void) {
  const modalRef = useRef<HTMLDivElement>(null)
  // Callers pass an inline closure, so onClose changes identity on every
  // render; reading it through a ref keeps the effect below mount-only.
  const onCloseRef = useRef(onClose)
  onCloseRef.current = onClose

  useEffect(() => {
    const modal = modalRef.current
    const previouslyFocused = document.activeElement as HTMLElement | null
    modal?.querySelector<HTMLElement>('input, select, button, textarea')?.focus()

    function handleKeyDown(e: KeyboardEvent) {
      if (e.key === 'Escape') {
        onCloseRef.current()
        return
      }
      if (e.key !== 'Tab' || !modal) return
      const focusable = modal.querySelectorAll<HTMLElement>(
        'input, select, button, textarea, [tabindex]:not([tabindex="-1"])'
      )
      if (focusable.length === 0) return
      const first = focusable[0]
      const last = focusable[focusable.length - 1]
      if (e.shiftKey && document.activeElement === first) {
        e.preventDefault()
        last.focus()
      } else if (!e.shiftKey && document.activeElement === last) {
        e.preventDefault()
        first.focus()
      }
    }

    document.addEventListener('keydown', handleKeyDown)
    return () => {
      document.removeEventListener('keydown', handleKeyDown)
      previouslyFocused?.focus()
    }
  }, [])

  return modalRef
}
