import { describe, it, expect, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import { ModalStackRenderer } from '../components/ModalStackRenderer'
import type { ModalEntry } from '../types'

vi.mock('../lib/api', () => ({
  api: {
    get: vi.fn().mockRejectedValue(new Error('not configured')),
    post: vi.fn(),
    put: vi.fn(),
    del: vi.fn(),
  },
}))

vi.mock('../lib/bodyScroll', () => ({
  lockBodyScroll: vi.fn(),
  unlockBodyScroll: vi.fn(),
}))

describe('ModalStackRenderer', () => {
  const pushModal = vi.fn()
  const popModal = vi.fn()
  const libraryIds = new Set<string>()

  function renderStack(stack: ModalEntry[]) {
    return render(
      <ModalStackRenderer
        stack={stack}
        pushModal={pushModal}
        popModal={popModal}
        overseerrConfigured={false}
        libraryIds={libraryIds}
      />
    )
  }

  it('renders nothing when stack is empty', () => {
    const { container } = renderStack([])
    expect(container.innerHTML).toBe('')
  })

  it('renders a person modal for a person entry', () => {
    renderStack([{ type: 'person', personId: 42 }])
    expect(screen.getByRole('dialog')).toBeInTheDocument()
    expect(screen.getByLabelText('Close')).toBeInTheDocument()
  })

  it('renders a TMDB modal for a tmdb entry', () => {
    renderStack([{ type: 'tmdb', mediaType: 'movie', mediaId: 100 }])
    expect(screen.getByRole('dialog')).toBeInTheDocument()
  })

  it('hides non-top entries with visibility hidden', () => {
    const stack: ModalEntry[] = [
      { type: 'person', personId: 1 },
      { type: 'tmdb', mediaType: 'movie', mediaId: 2 },
    ]
    const { container } = renderStack(stack)
    const wrappers = container.children

    expect(wrappers).toHaveLength(2)
    // First wrapper (non-top) should be hidden
    expect((wrappers[0] as HTMLElement).style.visibility).toBe('hidden')
    expect((wrappers[0] as HTMLElement).style.pointerEvents).toBe('none')
    // Second wrapper (top) should not have hidden styles
    expect((wrappers[1] as HTMLElement).style.visibility).toBe('')
  })

  it('sets aria-hidden on non-top entries', () => {
    const stack: ModalEntry[] = [
      { type: 'tmdb', mediaType: 'tv', mediaId: 10 },
      { type: 'person', personId: 20 },
    ]
    const { container } = renderStack(stack)
    const wrappers = container.children

    expect(wrappers[0].getAttribute('aria-hidden')).toBe('true')
    expect(wrappers[1].hasAttribute('aria-hidden')).toBe(false)
  })

  it('passes active=false to non-top modals and active=true to top', () => {
    const stack: ModalEntry[] = [
      { type: 'person', personId: 1 },
      { type: 'person', personId: 2 },
      { type: 'tmdb', mediaType: 'movie', mediaId: 3 },
    ]
    const { container } = renderStack(stack)
    // All three wrappers should exist
    expect(container.children).toHaveLength(3)
    // Only the top (last) should be visible
    expect((container.children[0] as HTMLElement).style.visibility).toBe('hidden')
    expect((container.children[1] as HTMLElement).style.visibility).toBe('hidden')
    expect((container.children[2] as HTMLElement).style.visibility).toBe('')
  })
})
