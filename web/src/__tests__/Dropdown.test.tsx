import { describe, it, expect, vi } from 'vitest'
import { screen, fireEvent } from '@testing-library/react'
import { renderWithRouter } from '../test-utils'
import { Dropdown } from '../components/Dropdown'

const options = [
  { value: 'a', label: 'Alpha' },
  { value: 'b', label: 'Beta' },
  { value: 'c', label: 'Gamma' },
]

describe('Dropdown', () => {
  describe('single-select', () => {
    it('renders with label showing selected value', () => {
      renderWithRouter(
        <Dropdown options={options} value="b" onChange={() => {}} />
      )
      expect(screen.getByText('Beta')).toBeDefined()
      expect(screen.getByText(/▾/)).toBeDefined()
    })

    it('renders with custom label when provided', () => {
      renderWithRouter(
        <Dropdown options={options} value="a" onChange={() => {}} label="Pick one" />
      )
      expect(screen.getByText('Pick one')).toBeDefined()
    })

    it('opens on click and shows options', () => {
      renderWithRouter(
        <Dropdown options={options} value="a" onChange={() => {}} />
      )
      fireEvent.click(screen.getByRole('button'))
      // Alpha appears in both the button label and the option list
      expect(screen.getAllByText('Alpha').length).toBe(2)
      expect(screen.getByText('Beta')).toBeDefined()
      expect(screen.getByText('Gamma')).toBeDefined()
    })

    it('closes on click outside', () => {
      renderWithRouter(
        <div>
          <div data-testid="outside">Outside</div>
          <Dropdown options={options} value="a" onChange={() => {}} />
        </div>
      )
      fireEvent.click(screen.getByRole('button'))
      expect(screen.getAllByText('Alpha').length).toBeGreaterThanOrEqual(2)

      fireEvent.mouseDown(screen.getByTestId('outside'))
      // After closing, only the button label remains
      expect(screen.getAllByText('Alpha').length).toBe(1)
    })

    it('selects option, calls onChange, and closes', () => {
      const onChange = vi.fn()
      renderWithRouter(
        <Dropdown options={options} value="a" onChange={onChange} />
      )
      fireEvent.click(screen.getByRole('button'))
      // Click 'Beta' in the dropdown panel
      const betaOptions = screen.getAllByText('Beta')
      fireEvent.click(betaOptions[betaOptions.length - 1])

      expect(onChange).toHaveBeenCalledWith('b')
      // Dropdown should be closed — only the button text remains
      expect(screen.queryAllByText('Gamma').length).toBe(0)
    })

    it('closes on Escape', () => {
      renderWithRouter(
        <Dropdown options={options} value="a" onChange={() => {}} />
      )
      fireEvent.click(screen.getByRole('button'))
      expect(screen.getAllByText('Beta').length).toBeGreaterThanOrEqual(1)

      fireEvent.keyDown(document, { key: 'Escape' })
      // Panel is closed — only button label 'Alpha' remains
      expect(screen.getAllByText('Alpha').length).toBe(1)
      expect(screen.queryAllByText('Beta').length).toBe(0)
    })

    it('handles empty options gracefully', () => {
      renderWithRouter(
        <Dropdown options={[]} value="" onChange={() => {}} />
      )
      const button = screen.getByRole('button')
      expect(button).toBeDefined()
      // Should not open when clicked with no options
      fireEvent.click(button)
      expect(screen.queryByRole('button', { name: /Alpha/ })).toBeNull()
    })
  })

  describe('multi-select', () => {
    it('toggles checkboxes and stays open', () => {
      const onChange = vi.fn()
      renderWithRouter(
        <Dropdown
          multi
          options={options}
          selected={['a']}
          onChange={onChange}
        />
      )
      fireEvent.click(screen.getByRole('button'))

      const betaCheckbox = screen.getByLabelText('Beta') as HTMLInputElement
      expect(betaCheckbox.checked).toBe(false)
      fireEvent.click(betaCheckbox)

      expect(onChange).toHaveBeenCalledWith(['a', 'b'])
      // Panel should still be open
      expect(screen.getByLabelText('Gamma')).toBeDefined()
    })

    it('unchecks an already selected option', () => {
      const onChange = vi.fn()
      renderWithRouter(
        <Dropdown
          multi
          options={options}
          selected={['a', 'b']}
          onChange={onChange}
        />
      )
      fireEvent.click(screen.getByRole('button'))

      const alphaCheckbox = screen.getByLabelText('Alpha') as HTMLInputElement
      expect(alphaCheckbox.checked).toBe(true)
      fireEvent.click(alphaCheckbox)

      expect(onChange).toHaveBeenCalledWith(['b'])
    })

    it('shows allLabel when all selected', () => {
      renderWithRouter(
        <Dropdown
          multi
          options={options}
          selected={['a', 'b', 'c']}
          onChange={() => {}}
          allLabel="All Servers"
        />
      )
      expect(screen.getByText('All Servers')).toBeDefined()
    })

    it('shows default "All" when all selected and no allLabel', () => {
      renderWithRouter(
        <Dropdown
          multi
          options={options}
          selected={['a', 'b', 'c']}
          onChange={() => {}}
        />
      )
      expect(screen.getByText('All')).toBeDefined()
    })

    it('shows "{n} selected" when partial', () => {
      renderWithRouter(
        <Dropdown
          multi
          options={options}
          selected={['a', 'c']}
          onChange={() => {}}
        />
      )
      expect(screen.getByText('2 selected')).toBeDefined()
    })

    it('shows noneLabel when none selected', () => {
      renderWithRouter(
        <Dropdown
          multi
          options={options}
          selected={[]}
          onChange={() => {}}
          noneLabel="No servers"
        />
      )
      expect(screen.getByText('No servers')).toBeDefined()
    })

    it('shows default "None" when none selected and no noneLabel', () => {
      renderWithRouter(
        <Dropdown
          multi
          options={options}
          selected={[]}
          onChange={() => {}}
        />
      )
      expect(screen.getByText('None')).toBeDefined()
    })

    it('closes on Escape', () => {
      renderWithRouter(
        <Dropdown
          multi
          options={options}
          selected={['a']}
          onChange={() => {}}
        />
      )
      fireEvent.click(screen.getByRole('button'))
      expect(screen.getByLabelText('Alpha')).toBeDefined()

      fireEvent.keyDown(document, { key: 'Escape' })
      expect(screen.queryByLabelText('Alpha')).toBeNull()
    })
  })

  describe('accessible name', () => {
    it('names the trigger with the field and its current value', () => {
      renderWithRouter(
        <Dropdown options={options} value="b" onChange={() => {}} aria-label="Sort by" />
      )
      expect(screen.getByRole('button', { name: 'Sort by: Beta' })).toBeDefined()
    })

    it('names a multi trigger with its selection summary', () => {
      renderWithRouter(
        <Dropdown multi options={options} selected={['a', 'b']} onChange={() => {}} aria-label="Genres" />
      )
      expect(screen.getByRole('button', { name: 'Genres: 2 selected' })).toBeDefined()
    })

    it('falls back to the field alone when there is no value to announce', () => {
      renderWithRouter(
        <Dropdown options={[]} value="" onChange={() => {}} aria-label="Genres" />
      )
      expect(screen.getByRole('button', { name: 'Genres' })).toBeDefined()
    })
  })

  describe('disabled', () => {
    const view = (disabled: boolean) => (
      <Dropdown options={options} value="a" onChange={() => {}} aria-label="Kind" disabled={disabled} />
    )
    const trigger = () => screen.getAllByRole('button')[0]

    it('collapses when it becomes disabled and stays closed when re-enabled', () => {
      const { rerender } = renderWithRouter(view(false))

      fireEvent.click(trigger())
      expect(trigger().getAttribute('aria-expanded')).toBe('true')
      expect(screen.getByText('Gamma')).toBeDefined()

      rerender(view(true))
      expect(trigger().getAttribute('aria-expanded')).toBe('false')
      expect(screen.queryByText('Gamma')).toBeNull()

      rerender(view(false))
      expect(trigger().getAttribute('aria-expanded')).toBe('false')
      expect(screen.queryByText('Gamma')).toBeNull()
    })

    it('reopens on a click after being re-enabled', () => {
      const { rerender } = renderWithRouter(view(false))
      fireEvent.click(trigger())
      rerender(view(true))
      rerender(view(false))

      fireEvent.click(trigger())
      expect(screen.getByText('Gamma')).toBeDefined()
    })
  })
})
