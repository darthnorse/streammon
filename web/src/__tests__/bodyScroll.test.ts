import { lockBodyScroll, unlockBodyScroll, _resetLockCount } from '../lib/bodyScroll'

describe('bodyScroll', () => {
  beforeEach(() => {
    _resetLockCount()
  })

  it('sets overflow hidden on first lock', () => {
    lockBodyScroll()
    expect(document.body.style.overflow).toBe('hidden')
  })

  it('restores overflow on unlock back to zero', () => {
    lockBodyScroll()
    unlockBodyScroll()
    expect(document.body.style.overflow).toBe('')
  })

  it('requires matching unlocks for multiple locks', () => {
    lockBodyScroll()
    lockBodyScroll()
    lockBodyScroll()

    unlockBodyScroll()
    expect(document.body.style.overflow).toBe('hidden')

    unlockBodyScroll()
    expect(document.body.style.overflow).toBe('hidden')

    unlockBodyScroll()
    expect(document.body.style.overflow).toBe('')
  })

  it('does not go below zero on extra unlocks', () => {
    lockBodyScroll()
    unlockBodyScroll()
    unlockBodyScroll()
    unlockBodyScroll()
    expect(document.body.style.overflow).toBe('')

    // Can still lock again after extra unlocks
    lockBodyScroll()
    expect(document.body.style.overflow).toBe('hidden')
  })

  it('simulates modal stack lifecycle: parent + child modals', () => {
    // Parent modal mounts
    lockBodyScroll()
    expect(document.body.style.overflow).toBe('hidden')

    // Child modal mounts (active)
    lockBodyScroll()
    expect(document.body.style.overflow).toBe('hidden')

    // Child modal unmounts (popped)
    unlockBodyScroll()
    expect(document.body.style.overflow).toBe('hidden') // parent still open

    // Parent modal unmounts
    unlockBodyScroll()
    expect(document.body.style.overflow).toBe('')
  })
})
