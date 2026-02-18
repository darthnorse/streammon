import { describe, it, expect, vi } from 'vitest'
import { render, screen, within, fireEvent } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { DatePicker } from '../components/DatePicker'

describe('DatePicker', () => {
  it('renders button with label when no value', () => {
    render(<DatePicker value="" onChange={vi.fn()} label="Start date" />)
    expect(screen.getByLabelText('Start date')).toBeInTheDocument()
    expect(screen.getByText('Start date')).toBeInTheDocument()
  })

  it('renders button with formatted date when value set', () => {
    render(<DatePicker value="2024-06-15" onChange={vi.fn()} label="Start date" />)
    expect(screen.getByText(/Jun 15, 2024/)).toBeInTheDocument()
  })

  it('opens calendar on click', async () => {
    render(<DatePicker value="2024-06-15" onChange={vi.fn()} label="Start date" />)
    await userEvent.click(screen.getByLabelText('Start date'))
    expect(screen.getByText('June 2024')).toBeInTheDocument()
    expect(screen.getByText('Su')).toBeInTheDocument()
    expect(screen.getByText('15')).toBeInTheDocument()
  })

  it('sets aria-expanded when open', async () => {
    render(<DatePicker value="2024-06-15" onChange={vi.fn()} label="Start date" />)
    const btn = screen.getByLabelText('Start date')
    expect(btn).toHaveAttribute('aria-expanded', 'false')
    await userEvent.click(btn)
    expect(btn).toHaveAttribute('aria-expanded', 'true')
  })

  it('closes on Escape', async () => {
    render(<DatePicker value="2024-06-15" onChange={vi.fn()} label="Start date" />)
    await userEvent.click(screen.getByLabelText('Start date'))
    expect(screen.getByText('June 2024')).toBeInTheDocument()
    await userEvent.keyboard('{Escape}')
    expect(screen.queryByText('June 2024')).not.toBeInTheDocument()
  })

  it('closes on click outside', async () => {
    render(<DatePicker value="2024-06-15" onChange={vi.fn()} label="Start date" />)
    await userEvent.click(screen.getByLabelText('Start date'))
    expect(screen.getByText('June 2024')).toBeInTheDocument()
    fireEvent.mouseDown(document.body)
    expect(screen.queryByText('June 2024')).not.toBeInTheDocument()
  })

  it('navigates to previous month', async () => {
    render(<DatePicker value="2024-06-15" onChange={vi.fn()} label="Start date" />)
    await userEvent.click(screen.getByLabelText('Start date'))
    expect(screen.getByText('June 2024')).toBeInTheDocument()

    const header = screen.getByText('June 2024').parentElement!
    const navButtons = within(header).getAllByRole('button')
    await userEvent.click(navButtons[0])
    expect(screen.getByText('May 2024')).toBeInTheDocument()
  })

  it('navigates to next month', async () => {
    render(<DatePicker value="2024-06-15" onChange={vi.fn()} label="Start date" />)
    await userEvent.click(screen.getByLabelText('Start date'))

    const header = screen.getByText('June 2024').parentElement!
    const navButtons = within(header).getAllByRole('button')
    await userEvent.click(navButtons[1])
    expect(screen.getByText('July 2024')).toBeInTheDocument()
  })

  it('calls onChange and closes when a day is clicked', async () => {
    const onChange = vi.fn()
    render(<DatePicker value="2024-06-15" onChange={onChange} label="Start date" />)
    await userEvent.click(screen.getByLabelText('Start date'))
    await userEvent.click(screen.getByText('20'))
    expect(onChange).toHaveBeenCalledWith('2024-06-20')
    expect(screen.queryByText('June 2024')).not.toBeInTheDocument()
  })

  it('highlights selected day', async () => {
    render(<DatePicker value="2024-06-15" onChange={vi.fn()} label="Start date" />)
    await userEvent.click(screen.getByLabelText('Start date'))
    const day15 = screen.getByText('15')
    expect(day15.className).toContain('text-accent')
  })

  it('disables days after max date', async () => {
    render(<DatePicker value="2024-06-15" onChange={vi.fn()} label="Start date" max="2024-06-20" />)
    await userEvent.click(screen.getByLabelText('Start date'))
    const day25 = screen.getByText('25')
    expect(day25).toBeDisabled()
  })

  it('disables days before min date', async () => {
    render(<DatePicker value="2024-06-15" onChange={vi.fn()} label="Start date" min="2024-06-10" />)
    await userEvent.click(screen.getByLabelText('Start date'))
    const day5 = screen.getByText('5')
    expect(day5).toBeDisabled()
  })
})
