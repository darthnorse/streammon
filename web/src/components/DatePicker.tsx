import { useState, useRef, useEffect, useMemo, useCallback } from 'react'
import { localToday, parseYMD, padDate } from '../lib/format'
import { useClickOutside } from '../hooks/useClickOutside'

interface DatePickerProps {
  value: string
  onChange: (value: string) => void
  label: string
  max?: string
  min?: string
}

const DAYS_OF_WEEK = ['Su', 'Mo', 'Tu', 'We', 'Th', 'Fr', 'Sa']

function formatDisplay(value: string): string {
  if (!value) return ''
  const { year, month, day } = parseYMD(value)
  const dt = new Date(year, month, day)
  return dt.toLocaleDateString(undefined, { month: 'short', day: 'numeric', year: 'numeric' })
}

export function DatePicker({ value, onChange, label, max, min }: DatePickerProps) {
  const [open, setOpen] = useState(false)
  const ref = useRef<HTMLDivElement>(null)
  const close = useCallback(() => setOpen(false), [])
  useClickOutside(ref, open, close)

  const initial = value ? parseYMD(value) : { year: new Date().getFullYear(), month: new Date().getMonth(), day: 1 }
  const [viewYear, setViewYear] = useState(initial.year)
  const [viewMonth, setViewMonth] = useState(initial.month)

  // Sync view to selected value only when opening
  const prevOpen = useRef(false)
  useEffect(() => {
    if (open && !prevOpen.current && value) {
      const { year, month } = parseYMD(value)
      setViewYear(year)
      setViewMonth(month)
    }
    prevOpen.current = open
  }, [open, value])

  function prevMonth() {
    if (viewMonth === 0) {
      setViewMonth(11)
      setViewYear(viewYear - 1)
    } else {
      setViewMonth(viewMonth - 1)
    }
  }

  function nextMonth() {
    if (viewMonth === 11) {
      setViewMonth(0)
      setViewYear(viewYear + 1)
    } else {
      setViewMonth(viewMonth + 1)
    }
  }

  function dayStr(day: number): string {
    return padDate(new Date(viewYear, viewMonth, day))
  }

  function selectDay(day: number) {
    onChange(dayStr(day))
    setOpen(false)
  }

  const cells = useMemo(() => {
    const totalDays = new Date(viewYear, viewMonth + 1, 0).getDate()
    const firstDow = new Date(viewYear, viewMonth, 1).getDay()
    const result: (number | null)[] = []
    for (let i = 0; i < firstDow; i++) result.push(null)
    for (let d = 1; d <= totalDays; d++) result.push(d)
    return result
  }, [viewYear, viewMonth])

  const today = localToday()

  function isDisabled(day: number): boolean {
    const dateStr = dayStr(day)
    if (max && dateStr > max) return true
    if (min && dateStr < min) return true
    return false
  }

  function isSelected(day: number): boolean {
    if (!value) return false
    const { year, month, day: vd } = parseYMD(value)
    return year === viewYear && month === viewMonth && vd === day
  }

  const monthLabel = new Date(viewYear, viewMonth).toLocaleDateString(undefined, { month: 'long', year: 'numeric' })

  return (
    <div ref={ref} className="relative inline-block">
      <button
        type="button"
        onClick={() => setOpen(!open)}
        className="bg-panel dark:bg-panel-dark border border-border dark:border-border-dark rounded px-3 py-1.5 text-xs font-medium flex items-center gap-1.5 min-w-[130px]"
        aria-label={label}
        aria-expanded={open}
      >
        <svg className="w-3.5 h-3.5 text-muted dark:text-muted-dark shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
          <path strokeLinecap="round" strokeLinejoin="round" d="M8 7V3m8 4V3m-9 8h10M5 21h14a2 2 0 002-2V7a2 2 0 00-2-2H5a2 2 0 00-2 2v12a2 2 0 002 2z" />
        </svg>
        <span>{value ? formatDisplay(value) : label}</span>
        <span className="text-lg text-muted dark:text-muted-dark">▾</span>
      </button>

      {open && (
        <div className="absolute z-50 top-full mt-1 right-0 bg-panel dark:bg-panel-dark border border-border dark:border-border-dark rounded-lg shadow-lg p-3 w-[260px]">
          <div className="flex items-center justify-between mb-2">
            <button type="button" onClick={prevMonth} className="p-1 rounded hover:bg-surface dark:hover:bg-surface-dark text-muted dark:text-muted-dark">
              <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                <path strokeLinecap="round" strokeLinejoin="round" d="M15 19l-7-7 7-7" />
              </svg>
            </button>
            <span className="text-xs font-semibold">{monthLabel}</span>
            <button type="button" onClick={nextMonth} className="p-1 rounded hover:bg-surface dark:hover:bg-surface-dark text-muted dark:text-muted-dark">
              <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                <path strokeLinecap="round" strokeLinejoin="round" d="M9 5l7 7-7 7" />
              </svg>
            </button>
          </div>

          <div className="grid grid-cols-7 gap-0.5 mb-1">
            {DAYS_OF_WEEK.map(d => (
              <div key={d} className="text-center text-[10px] font-medium text-muted dark:text-muted-dark py-1">
                {d}
              </div>
            ))}
          </div>

          <div className="grid grid-cols-7 gap-0.5">
            {cells.map((day, i) => {
              if (day === null) return <div key={`e-${i}`} />
              const disabled = isDisabled(day)
              const selected = isSelected(day)
              const isTodayCell = dayStr(day) === today
              return (
                <button
                  key={day}
                  type="button"
                  disabled={disabled}
                  onClick={() => selectDay(day)}
                  className={`text-xs py-1.5 rounded transition-colors
                    ${disabled ? 'text-muted/30 dark:text-muted-dark/30 cursor-not-allowed' : 'hover:bg-surface dark:hover:bg-surface-dark'}
                    ${selected ? 'bg-accent/20 text-accent font-semibold' : ''}
                    ${isTodayCell && !selected ? 'font-semibold underline' : ''}
                  `}
                >
                  {day}
                </button>
              )
            })}
          </div>
        </div>
      )}
    </div>
  )
}
