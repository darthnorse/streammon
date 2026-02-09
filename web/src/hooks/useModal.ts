import { useEffect, useRef } from 'react'

export function useModal(onClose: () => void) {
  const modalRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    function handleEscape(e: KeyboardEvent) {
      if (e.key === 'Escape') onClose()
    }
    document.addEventListener('keydown', handleEscape)
    const previouslyFocused = document.activeElement as HTMLElement | null
    modalRef.current?.querySelector<HTMLElement>('input, select, button, textarea')?.focus()
    return () => {
      document.removeEventListener('keydown', handleEscape)
      previouslyFocused?.focus()
    }
  }, [onClose])

  useEffect(() => {
    const modal = modalRef.current
    if (!modal) return
    function trapFocus(e: KeyboardEvent) {
      if (e.key !== 'Tab') return
      const focusable = modal!.querySelectorAll<HTMLElement>(
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
    document.addEventListener('keydown', trapFocus)
    return () => document.removeEventListener('keydown', trapFocus)
  }, [])

  return modalRef
}
