import { describe, it, expect } from 'vitest'
import { render, screen } from '@testing-library/react'
import { EmptyState } from '../components/EmptyState'

describe('EmptyState', () => {
  it('renders icon, title, and description', () => {
    render(<EmptyState icon="▣" title="No items" description="Nothing here yet" />)
    expect(screen.getByText('▣')).toBeInTheDocument()
    expect(screen.getByText('No items')).toBeInTheDocument()
    expect(screen.getByText('Nothing here yet')).toBeInTheDocument()
  })

  it('renders without description', () => {
    render(<EmptyState icon="?" title="Not found" />)
    expect(screen.getByText('Not found')).toBeInTheDocument()
  })

  it('renders children (action slot)', () => {
    render(
      <EmptyState icon="+" title="Empty">
        <button>Add item</button>
      </EmptyState>
    )
    expect(screen.getByRole('button', { name: 'Add item' })).toBeInTheDocument()
  })
})
