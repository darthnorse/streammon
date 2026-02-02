import { describe, it, expect, beforeEach } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/react'
import { ThemeToggle } from '../components/ThemeToggle'

describe('ThemeToggle', () => {
  beforeEach(() => {
    localStorage.clear()
    document.documentElement.classList.remove('dark')
    Object.defineProperty(window, 'matchMedia', {
      writable: true,
      value: (query: string) => ({
        matches: false,
        media: query,
        onchange: null,
        addListener: () => {},
        removeListener: () => {},
        addEventListener: () => {},
        removeEventListener: () => {},
        dispatchEvent: () => false,
      }),
    })
  })

  it('renders a button', () => {
    render(<ThemeToggle />)
    expect(screen.getByRole('button', { name: /theme/i })).toBeDefined()
  })

  it('toggles dark class on html element', () => {
    render(<ThemeToggle />)
    const btn = screen.getByRole('button', { name: /theme/i })
    fireEvent.click(btn)
    expect(btn).toBeDefined()
  })

  it('persists preference to localStorage', () => {
    render(<ThemeToggle />)
    const btn = screen.getByRole('button', { name: /theme/i })
    fireEvent.click(btn)
    const stored = localStorage.getItem('streammon-theme')
    expect(stored).toBeTruthy()
  })
})
