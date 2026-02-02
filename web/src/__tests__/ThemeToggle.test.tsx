import { describe, it, expect, beforeEach } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/react'
import { ThemeToggle } from '../components/ThemeToggle'

describe('ThemeToggle', () => {
  beforeEach(() => {
    localStorage.clear()
    document.documentElement.classList.remove('dark')
  })

  it('renders a button', () => {
    render(<ThemeToggle />)
    expect(screen.getByRole('button', { name: /theme/i })).toBeDefined()
  })

  it('cycles through system -> light -> dark -> system', () => {
    render(<ThemeToggle />)
    const btn = screen.getByRole('button', { name: /theme/i })

    // Initial: system (matchMedia returns false, so no dark class)
    expect(document.documentElement.classList.contains('dark')).toBe(false)

    // Click 1: system -> light
    fireEvent.click(btn)
    expect(localStorage.getItem('streammon-theme')).toBe('light')
    expect(document.documentElement.classList.contains('dark')).toBe(false)

    // Click 2: light -> dark
    fireEvent.click(btn)
    expect(localStorage.getItem('streammon-theme')).toBe('dark')
    expect(document.documentElement.classList.contains('dark')).toBe(true)

    // Click 3: dark -> system
    fireEvent.click(btn)
    expect(localStorage.getItem('streammon-theme')).toBe('system')
    expect(document.documentElement.classList.contains('dark')).toBe(false)
  })

  it('restores persisted theme from localStorage', () => {
    localStorage.setItem('streammon-theme', 'dark')
    render(<ThemeToggle />)
    expect(document.documentElement.classList.contains('dark')).toBe(true)
  })
})
