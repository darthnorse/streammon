import { describe, it, expect, vi } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/react'
import { Pagination } from '../components/Pagination'

describe('Pagination', () => {
  it('renders nothing when totalPages is 1', () => {
    const { container } = render(
      <Pagination page={1} totalPages={1} onPageChange={() => {}} />
    )
    expect(container.innerHTML).toBe('')
  })

  it('renders nothing when totalPages is 0', () => {
    const { container } = render(
      <Pagination page={1} totalPages={0} onPageChange={() => {}} />
    )
    expect(container.innerHTML).toBe('')
  })

  it('shows page indicator', () => {
    render(<Pagination page={2} totalPages={5} onPageChange={() => {}} />)
    expect(screen.getByText('2 / 5')).toBeDefined()
  })

  it('disables Previous on first page', () => {
    render(<Pagination page={1} totalPages={3} onPageChange={() => {}} />)
    expect(screen.getByText('Previous').closest('button')!.disabled).toBe(true)
  })

  it('disables Next on last page', () => {
    render(<Pagination page={3} totalPages={3} onPageChange={() => {}} />)
    expect(screen.getByText('Next').closest('button')!.disabled).toBe(true)
  })

  it('calls onPageChange with correct values', () => {
    const onChange = vi.fn()
    render(<Pagination page={2} totalPages={5} onPageChange={onChange} />)
    fireEvent.click(screen.getByText('Previous'))
    expect(onChange).toHaveBeenCalledWith(1)
    fireEvent.click(screen.getByText('Next'))
    expect(onChange).toHaveBeenCalledWith(3)
  })
})
