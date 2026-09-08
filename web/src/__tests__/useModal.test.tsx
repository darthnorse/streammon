import { describe, it, expect, vi } from 'vitest'
import { useState } from 'react'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { useModal } from '../hooks/useModal'

// Mirrors TerminateSessionDialog: an inline closure whose behaviour depends on
// state that flips after mount, so a handler captured at mount is observable.
function Dialog({ onClose }: { onClose: () => void }) {
  const [text, setText] = useState('')
  const [busy, setBusy] = useState(false)
  const modalRef = useModal(() => { if (!busy) onClose() })
  return (
    <div ref={modalRef}>
      <button aria-label="Close" onClick={onClose}>&times;</button>
      <textarea
        aria-label="Message"
        value={text}
        onChange={e => setText(e.target.value)}
      />
      <button aria-label="Start" onClick={() => setBusy(true)}>go</button>
    </div>
  )
}

describe('useModal', () => {
  it('keeps focus in the field while typing when onClose identity changes each render', async () => {
    const user = userEvent.setup()
    render(<Dialog onClose={vi.fn()} />)

    const field = screen.getByLabelText('Message')
    await user.click(field)
    await user.keyboard('hello')

    expect((field as HTMLTextAreaElement).value).toBe('hello')
    expect(document.activeElement).toBe(field)
  })

  it('moves focus into the modal on open', () => {
    render(<Dialog onClose={vi.fn()} />)
    expect(screen.getByLabelText('Close')).toBe(document.activeElement)
  })

  it('closes on Escape', async () => {
    const user = userEvent.setup()
    const onClose = vi.fn()
    render(<Dialog onClose={onClose} />)

    await user.keyboard('{Escape}')

    expect(onClose).toHaveBeenCalledTimes(1)
  })

  it('Escape sees state that changed after mount, not the handler captured then', async () => {
    const user = userEvent.setup()
    const onClose = vi.fn()
    render(<Dialog onClose={onClose} />)

    await user.click(screen.getByLabelText('Start'))
    await user.keyboard('{Escape}')

    expect(onClose).not.toHaveBeenCalled()
  })

  it('traps Tab inside the modal', async () => {
    const user = userEvent.setup()
    render(<Dialog onClose={vi.fn()} />)

    await user.click(screen.getByLabelText('Start'))
    await user.tab()

    expect(document.activeElement).toBe(screen.getByLabelText('Close'))
  })
})
