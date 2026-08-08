import { useState, useRef, useCallback } from 'react'
import { useClickOutside } from '../hooks/useClickOutside'

export interface DropdownOption<T extends string = string> {
  value: T
  label: string
}

interface SingleDropdownProps<T extends string = string> {
  options: DropdownOption<T>[]
  value: T
  onChange: (value: T) => void
  label?: string
  className?: string
  disabled?: boolean
  'aria-label'?: string
  multi?: false
}

interface MultiDropdownProps<T extends string = string> {
  options: DropdownOption<T>[]
  selected: T[]
  onChange: (selected: T[]) => void
  allLabel?: string
  noneLabel?: string
  className?: string
  disabled?: boolean
  disabledOptions?: T[]
  'aria-label'?: string
  multi: true
}

type DropdownProps<T extends string = string> = SingleDropdownProps<T> | MultiDropdownProps<T>

function isMulti<T extends string>(props: DropdownProps<T>): props is MultiDropdownProps<T> {
  return props.multi === true
}

function getButtonLabel<T extends string>(props: DropdownProps<T>): string {
  if (isMulti(props)) {
    const { options, selected, allLabel = 'All', noneLabel = 'None' } = props
    if (selected.length === 0) return noneLabel
    if (selected.length === options.length) return allLabel
    return `${selected.length} selected`
  }
  if (props.label) return props.label
  if (props.options.length === 0) return ''
  const match = props.options.find(o => o.value === props.value)
  return match ? match.label : ''
}

function MultiOption<T extends string>({ opt, checked, disabled, onToggle }: { opt: DropdownOption<T>; checked: boolean; disabled?: boolean; onToggle: (v: T) => void }) {
  return (
    <label className={`flex items-center gap-2 px-3 py-1.5 text-sm hover:bg-surface dark:hover:bg-surface-dark ${disabled ? 'opacity-50 cursor-not-allowed' : 'cursor-pointer'}`}>
      <input
        type="checkbox"
        checked={checked}
        disabled={disabled}
        onChange={() => onToggle(opt.value)}
        className="rounded border-gray-300 dark:border-gray-600 text-accent focus:ring-accent"
      />
      {opt.label}
    </label>
  )
}

function SingleOption<T extends string>({ opt, selected, onSelect }: { opt: DropdownOption<T>; selected: boolean; onSelect: (v: T) => void }) {
  return (
    <button
      type="button"
      onClick={() => onSelect(opt.value)}
      className={`flex items-center gap-2 w-full text-left px-3 py-1.5 text-sm hover:bg-surface dark:hover:bg-surface-dark ${
        selected ? 'bg-surface/50 dark:bg-surface-dark/50' : ''
      }`}
    >
      <span className="w-3 text-center">{selected ? '✓' : ''}</span>
      {opt.label}
    </button>
  )
}

export function Dropdown<T extends string = string>(props: DropdownProps<T>) {
  const { options, className, disabled } = props
  const [open, setOpen] = useState(false)
  const ref = useRef<HTMLDivElement>(null)
  const close = useCallback(() => setOpen(false), [])
  useClickOutside(ref, open, close)

  function handleOptionClick(value: T) {
    if (isMulti(props)) {
      const { selected, onChange } = props
      const next = selected.includes(value)
        ? selected.filter(v => v !== value)
        : [...selected, value]
      onChange(next)
    } else {
      props.onChange(value)
      setOpen(false)
    }
  }

  return (
    <div ref={ref} className={`relative inline-block ${className ?? ''}`}>
      <button
        type="button"
        disabled={disabled}
        aria-label={props['aria-label']}
        onClick={() => !disabled && options.length > 0 && setOpen(!open)}
        aria-expanded={open}
        aria-haspopup="listbox"
        className="bg-panel dark:bg-panel-dark border border-border dark:border-border-dark rounded px-3 py-1.5 text-sm font-medium flex items-center gap-1 disabled:opacity-50 disabled:cursor-not-allowed"
      >
        <span>{getButtonLabel(props)}</span>
        <span className="text-lg">▾</span>
      </button>
      {open && !disabled && (
        <div className="absolute z-50 top-full mt-1 left-0 min-w-full bg-panel dark:bg-panel-dark border border-border dark:border-border-dark rounded shadow-lg py-1">
          {options.map(opt => (
            isMulti(props)
              ? <MultiOption
                  key={opt.value}
                  opt={opt}
                  checked={props.selected.includes(opt.value)}
                  disabled={props.disabledOptions?.includes(opt.value)}
                  onToggle={handleOptionClick}
                />
              : <SingleOption key={opt.value} opt={opt} selected={opt.value === props.value} onSelect={handleOptionClick} />
          ))}
        </div>
      )}
    </div>
  )
}
