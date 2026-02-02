import { describe, it, expect } from 'vitest'
import { screen } from '@testing-library/react'
import { renderWithRouter } from '../test-utils'
import { Layout } from '../components/Layout'

describe('Layout', () => {
  it('renders sidebar nav links', () => {
    renderWithRouter(<Layout />)
    expect(screen.getAllByText('Dashboard').length).toBeGreaterThanOrEqual(1)
    expect(screen.getAllByText('History').length).toBeGreaterThanOrEqual(1)
    expect(screen.getAllByText('Settings').length).toBeGreaterThanOrEqual(1)
  })

  it('renders theme toggle', () => {
    renderWithRouter(<Layout />)
    expect(screen.getAllByRole('button', { name: /theme/i }).length).toBeGreaterThanOrEqual(1)
  })

  it('renders mobile nav', () => {
    renderWithRouter(<Layout />)
    const dashLinks = screen.getAllByText('Dashboard')
    expect(dashLinks.length).toBeGreaterThanOrEqual(2)
  })
})
