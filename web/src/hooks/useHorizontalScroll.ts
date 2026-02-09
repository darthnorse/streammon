import { useRef, useCallback, useEffect, useState } from 'react'

export function useHorizontalScroll() {
  const [el, setEl] = useState<HTMLDivElement | null>(null)
  const ref = useCallback((node: HTMLDivElement | null) => setEl(node), [])
  const drag = useRef({ isDown: false, startX: 0, scrollLeft: 0, dragged: false })
  const [canScrollLeft, setCanScrollLeft] = useState(false)
  const [canScrollRight, setCanScrollRight] = useState(true)

  const updateArrows = useCallback(() => {
    if (!el) return
    setCanScrollLeft(el.scrollLeft > 1)
    setCanScrollRight(el.scrollLeft + el.clientWidth < el.scrollWidth - 1)
  }, [el])

  const scrollBy = useCallback((direction: 'left' | 'right') => {
    if (!el) return
    const amount = el.clientWidth * 0.75
    el.scrollBy({ left: direction === 'left' ? -amount : amount, behavior: 'smooth' })
  }, [el])

  const onMouseDown = useCallback((e: React.MouseEvent) => {
    if (!el) return
    drag.current.isDown = true
    drag.current.startX = e.pageX - el.offsetLeft
    drag.current.scrollLeft = el.scrollLeft
    drag.current.dragged = false
    el.style.cursor = 'grabbing'
  }, [el])

  const onMouseMove = useCallback((e: React.MouseEvent) => {
    if (!el || !drag.current.isDown) return
    e.preventDefault()
    const x = e.pageX - el.offsetLeft
    const walk = x - drag.current.startX
    if (Math.abs(walk) > 3) drag.current.dragged = true
    el.scrollLeft = drag.current.scrollLeft - walk
  }, [el])

  const onMouseUp = useCallback(() => {
    if (!el) return
    drag.current.isDown = false
    el.style.cursor = 'grab'
  }, [el])

  const onClickCapture = useCallback((e: React.MouseEvent) => {
    if (drag.current.dragged) {
      e.stopPropagation()
      e.preventDefault()
    }
  }, [])

  useEffect(() => {
    if (!el) return
    el.style.cursor = 'grab'
    updateArrows()
    el.addEventListener('scroll', updateArrows, { passive: true })
    let ro: ResizeObserver | undefined
    if (typeof ResizeObserver !== 'undefined') {
      ro = new ResizeObserver(updateArrows)
      ro.observe(el)
    }
    return () => {
      el.removeEventListener('scroll', updateArrows)
      ro?.disconnect()
    }
  }, [el, updateArrows])

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
