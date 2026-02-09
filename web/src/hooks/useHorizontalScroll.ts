import { useRef, useCallback, useEffect, useState } from 'react'

export function useHorizontalScroll() {
  const ref = useRef<HTMLDivElement>(null)
  const state = useRef({ isDown: false, startX: 0, scrollLeft: 0, dragged: false })
  const [canScrollLeft, setCanScrollLeft] = useState(false)
  const [canScrollRight, setCanScrollRight] = useState(true)

  const updateArrows = useCallback(() => {
    const el = ref.current
    if (!el) return
    setCanScrollLeft(el.scrollLeft > 0)
    setCanScrollRight(el.scrollLeft + el.clientWidth < el.scrollWidth - 1)
  }, [])

  const scrollBy = useCallback((direction: 'left' | 'right') => {
    const el = ref.current
    if (!el) return
    const amount = el.clientWidth * 0.75
    el.scrollBy({ left: direction === 'left' ? -amount : amount, behavior: 'smooth' })
  }, [])

  const onMouseDown = useCallback((e: React.MouseEvent) => {
    const el = ref.current
    if (!el) return
    state.current = {
      isDown: true,
      startX: e.pageX - el.offsetLeft,
      scrollLeft: el.scrollLeft,
      dragged: false,
    }
    el.style.cursor = 'grabbing'
  }, [])

  const onMouseMove = useCallback((e: React.MouseEvent) => {
    const el = ref.current
    if (!el || !state.current.isDown) return
    e.preventDefault()
    const x = e.pageX - el.offsetLeft
    const walk = x - state.current.startX
    if (Math.abs(walk) > 3) state.current.dragged = true
    el.scrollLeft = state.current.scrollLeft - walk
  }, [])

  const onMouseUp = useCallback(() => {
    const el = ref.current
    if (!el) return
    state.current.isDown = false
    el.style.cursor = 'grab'
  }, [])

  const onClickCapture = useCallback((e: React.MouseEvent) => {
    if (state.current.dragged) {
      e.stopPropagation()
      e.preventDefault()
    }
  }, [])

  useEffect(() => {
    const el = ref.current
    if (!el) return
    el.style.cursor = 'grab'
    updateArrows()
    el.addEventListener('scroll', updateArrows, { passive: true })
    const ro = new ResizeObserver(updateArrows)
    ro.observe(el)
    const mo = new MutationObserver(updateArrows)
    mo.observe(el, { childList: true })
    return () => {
      el.removeEventListener('scroll', updateArrows)
      ro.disconnect()
      mo.disconnect()
    }
  }, [updateArrows])

  return {
    ref,
    onMouseDown,
    onMouseMove,
    onMouseUp,
    onMouseLeave: onMouseUp,
    onClickCapture,
    canScrollLeft,
    canScrollRight,
    scrollBy,
  }
}
