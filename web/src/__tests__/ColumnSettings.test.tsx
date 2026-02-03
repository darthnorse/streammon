import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { ColumnSettings } from '../components/ColumnSettings'
import type { ColumnDef } from '../lib/historyColumns'

const mockColumns: ColumnDef[] = [
  { id: 'a', label: 'Column A', defaultVisible: true, render: () => null },
  { id: 'b', label: 'Column B', defaultVisible: true, render: () => null },
  { id: 'c', label: 'Column C', defaultVisible: false, render: () => null },
]

describe('ColumnSettings', () => {
  const defaultProps = {
    columns: mockColumns,
    visibleColumns: ['a', 'b'],
    onToggle: vi.fn(),
    onMove: vi.fn(),
    onReset: vi.fn(),
  }

  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('renders settings button with correct aria attributes', () => {
    render(<ColumnSettings {...defaultProps} />)
    const button = screen.getByRole('button', { name: /column settings/i })
    expect(button).toBeDefined()
    expect(button.getAttribute('aria-expanded')).toBe('false')
  })

  it('opens dropdown on click and updates aria-expanded', async () => {
    const user = userEvent.setup()
    render(<ColumnSettings {...defaultProps} />)

    const button = screen.getByRole('button', { name: /column settings/i })
    await user.click(button)

    expect(button.getAttribute('aria-expanded')).toBe('true')
    expect(screen.getByText('Show columns')).toBeDefined()
  })

  it('closes dropdown on second click', async () => {
    const user = userEvent.setup()
    render(<ColumnSettings {...defaultProps} />)

    const button = screen.getByRole('button', { name: /column settings/i })
    await user.click(button)
    expect(screen.getByText('Show columns')).toBeDefined()

    await user.click(button)
    expect(screen.queryByText('Show columns')).toBeNull()
  })

  it('closes dropdown on Escape key', async () => {
    const user = userEvent.setup()
    render(<ColumnSettings {...defaultProps} />)

    await user.click(screen.getByRole('button', { name: /column settings/i }))
    expect(screen.getByText('Show columns')).toBeDefined()

    await user.keyboard('{Escape}')
    expect(screen.queryByText('Show columns')).toBeNull()
  })

  it('closes dropdown on click outside', async () => {
    const user = userEvent.setup()
    render(
      <div>
        <div data-testid="outside">Outside</div>
        <ColumnSettings {...defaultProps} />
      </div>
    )

    await user.click(screen.getByRole('button', { name: /column settings/i }))
    expect(screen.getByText('Show columns')).toBeDefined()

    fireEvent.mouseDown(screen.getByTestId('outside'))
    expect(screen.queryByText('Show columns')).toBeNull()
  })

  it('renders checkboxes for all columns', async () => {
    const user = userEvent.setup()
    render(<ColumnSettings {...defaultProps} />)

    await user.click(screen.getByRole('button', { name: /column settings/i }))

    expect(screen.getByLabelText('Column A')).toBeDefined()
    expect(screen.getByLabelText('Column B')).toBeDefined()
    expect(screen.getByLabelText('Column C')).toBeDefined()
  })

  it('shows checked state for visible columns', async () => {
    const user = userEvent.setup()
    render(<ColumnSettings {...defaultProps} />)

    await user.click(screen.getByRole('button', { name: /column settings/i }))

    expect((screen.getByLabelText('Column A') as HTMLInputElement).checked).toBe(true)
    expect((screen.getByLabelText('Column B') as HTMLInputElement).checked).toBe(true)
    expect((screen.getByLabelText('Column C') as HTMLInputElement).checked).toBe(false)
  })

  it('calls onToggle when checkbox is clicked', async () => {
    const user = userEvent.setup()
    render(<ColumnSettings {...defaultProps} />)

    await user.click(screen.getByRole('button', { name: /column settings/i }))
    await user.click(screen.getByLabelText('Column A'))

    expect(defaultProps.onToggle).toHaveBeenCalledWith('a')
  })

  it('excludes columns from excludeColumns list', async () => {
    const user = userEvent.setup()
    render(<ColumnSettings {...defaultProps} excludeColumns={['a']} />)

    await user.click(screen.getByRole('button', { name: /column settings/i }))

    expect(screen.queryByLabelText('Column A')).toBeNull()
    expect(screen.getByLabelText('Column B')).toBeDefined()
    expect(screen.getByLabelText('Column C')).toBeDefined()
  })

  it('shows move buttons only for visible columns', async () => {
    const user = userEvent.setup()
    render(<ColumnSettings {...defaultProps} />)

    await user.click(screen.getByRole('button', { name: /column settings/i }))

    expect(screen.getByRole('button', { name: /move column a up/i })).toBeDefined()
    expect(screen.getByRole('button', { name: /move column a down/i })).toBeDefined()
    expect(screen.getByRole('button', { name: /move column b up/i })).toBeDefined()
    expect(screen.getByRole('button', { name: /move column b down/i })).toBeDefined()
    expect(screen.queryByRole('button', { name: /move column c/i })).toBeNull()
  })

  it('disables move up for first visible column', async () => {
    const user = userEvent.setup()
    render(<ColumnSettings {...defaultProps} />)

    await user.click(screen.getByRole('button', { name: /column settings/i }))

    const moveUpA = screen.getByRole('button', { name: /move column a up/i })
    expect(moveUpA.hasAttribute('disabled')).toBe(true)
  })

  it('disables move down for last visible column', async () => {
    const user = userEvent.setup()
    render(<ColumnSettings {...defaultProps} />)

    await user.click(screen.getByRole('button', { name: /column settings/i }))

    const moveDownB = screen.getByRole('button', { name: /move column b down/i })
    expect(moveDownB.hasAttribute('disabled')).toBe(true)
  })

  it('calls onMove when move button is clicked', async () => {
    const user = userEvent.setup()
    render(<ColumnSettings {...defaultProps} />)

    await user.click(screen.getByRole('button', { name: /column settings/i }))
    await user.click(screen.getByRole('button', { name: /move column b up/i }))

    expect(defaultProps.onMove).toHaveBeenCalledWith('b', 'up')
  })

  it('calls onReset and closes dropdown when reset is clicked', async () => {
    const user = userEvent.setup()
    render(<ColumnSettings {...defaultProps} />)

    await user.click(screen.getByRole('button', { name: /column settings/i }))
    await user.click(screen.getByText('Reset to defaults'))

    expect(defaultProps.onReset).toHaveBeenCalled()
    expect(screen.queryByText('Show columns')).toBeNull()
  })
})
