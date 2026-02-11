import { describe, it, expect, beforeEach } from 'vitest'
import { renderHook, act } from '@testing-library/react'
import { useColumnConfig } from '../hooks/useColumnConfig'
import type { ColumnDef } from '../lib/historyColumns'

const mockColumns: ColumnDef[] = [
  { id: 'a', label: 'A', defaultVisible: true, render: () => null },
  { id: 'b', label: 'B', defaultVisible: true, render: () => null },
  { id: 'c', label: 'C', defaultVisible: false, render: () => null },
  { id: 'd', label: 'D', defaultVisible: true, render: () => null },
]

describe('useColumnConfig', () => {
  beforeEach(() => {
    localStorage.clear()
  })

  it('initializes with default visible columns when no stored config', () => {
    const { result } = renderHook(() =>
      useColumnConfig(mockColumns)
    )
    expect(result.current.visibleColumns).toEqual(['a', 'b', 'd'])
  })

  it('initializes from localStorage when present', () => {
    localStorage.setItem('history-columns', JSON.stringify(['b', 'a']))
    const { result } = renderHook(() =>
      useColumnConfig(mockColumns)
    )
    expect(result.current.visibleColumns).toEqual(['b', 'a'])
  })

  it('falls back to defaults when localStorage has invalid data', () => {
    localStorage.setItem('history-columns', 'invalid json')
    const { result } = renderHook(() =>
      useColumnConfig(mockColumns)
    )
    expect(result.current.visibleColumns).toEqual(['a', 'b', 'd'])
  })

  it('filters out excluded columns from defaults', () => {
    const { result } = renderHook(() =>
      useColumnConfig(mockColumns, ['a'])
    )
    expect(result.current.visibleColumns).toEqual(['b', 'd'])
  })

  it('filters out excluded columns from stored config', () => {
    localStorage.setItem('history-columns', JSON.stringify(['a', 'b', 'c']))
    const { result } = renderHook(() =>
      useColumnConfig(mockColumns, ['a'])
    )
    expect(result.current.visibleColumns).toEqual(['b', 'c'])
  })

  it('persists changes to localStorage', () => {
    const { result } = renderHook(() =>
      useColumnConfig(mockColumns)
    )
    act(() => {
      result.current.toggleColumn('a')
    })
    const stored = JSON.parse(localStorage.getItem('history-columns')!)
    expect(stored).not.toContain('a')
  })

  it('toggleColumn removes visible column', () => {
    const { result } = renderHook(() =>
      useColumnConfig(mockColumns)
    )
    act(() => {
      result.current.toggleColumn('b')
    })
    expect(result.current.visibleColumns).toEqual(['a', 'd'])
  })

  it('toggleColumn adds column in correct position', () => {
    localStorage.setItem('history-columns', JSON.stringify(['a', 'd']))
    const { result } = renderHook(() =>
      useColumnConfig(mockColumns)
    )
    act(() => {
      result.current.toggleColumn('b')
    })
    expect(result.current.visibleColumns).toEqual(['a', 'b', 'd'])
  })

  it('toggleColumn ignores excluded columns', () => {
    const { result } = renderHook(() =>
      useColumnConfig(mockColumns, ['a'])
    )
    const before = [...result.current.visibleColumns]
    act(() => {
      result.current.toggleColumn('a')
    })
    expect(result.current.visibleColumns).toEqual(before)
  })

  it('moveColumn moves column up', () => {
    const { result } = renderHook(() =>
      useColumnConfig(mockColumns)
    )
    act(() => {
      result.current.moveColumn('b', 'up')
    })
    expect(result.current.visibleColumns).toEqual(['b', 'a', 'd'])
  })

  it('moveColumn moves column down', () => {
    const { result } = renderHook(() =>
      useColumnConfig(mockColumns)
    )
    act(() => {
      result.current.moveColumn('b', 'down')
    })
    expect(result.current.visibleColumns).toEqual(['a', 'd', 'b'])
  })

  it('moveColumn does nothing when at boundary', () => {
    const { result } = renderHook(() =>
      useColumnConfig(mockColumns)
    )
    act(() => {
      result.current.moveColumn('a', 'up')
    })
    expect(result.current.visibleColumns).toEqual(['a', 'b', 'd'])

    act(() => {
      result.current.moveColumn('d', 'down')
    })
    expect(result.current.visibleColumns).toEqual(['a', 'b', 'd'])
  })

  it('moveColumn does nothing when column is not visible', () => {
    const { result } = renderHook(() =>
      useColumnConfig(mockColumns)
    )
    const before = [...result.current.visibleColumns]
    act(() => {
      result.current.moveColumn('c', 'up')
    })
    expect(result.current.visibleColumns).toEqual(before)
  })

  it('resetToDefaults restores default visible columns', () => {
    const { result } = renderHook(() =>
      useColumnConfig(mockColumns)
    )
    act(() => {
      result.current.toggleColumn('a')
      result.current.toggleColumn('b')
    })
    expect(result.current.visibleColumns).toEqual(['d'])

    act(() => {
      result.current.resetToDefaults()
    })
    expect(result.current.visibleColumns).toEqual(['a', 'b', 'd'])
  })

  it('resetToDefaults respects excludeColumns', () => {
    const { result } = renderHook(() =>
      useColumnConfig(mockColumns, ['a'])
    )
    act(() => {
      result.current.toggleColumn('b')
    })
    act(() => {
      result.current.resetToDefaults()
    })
    expect(result.current.visibleColumns).toEqual(['b', 'd'])
  })

  it('re-filters visibleColumns when excludeColumns changes', () => {
    let excludeColumns: string[] = []
    const { result, rerender } = renderHook(
      ({ exclude }) => useColumnConfig(mockColumns, exclude),
      { initialProps: { exclude: excludeColumns } }
    )

    expect(result.current.visibleColumns).toEqual(['a', 'b', 'd'])

    excludeColumns = ['a']
    rerender({ exclude: excludeColumns })

    expect(result.current.visibleColumns).toEqual(['b', 'd'])
  })

  it('preserves excluded default columns in localStorage on first save', () => {
    // Simulate visiting UserDetail first (excludes 'a'), then History (no exclusions)
    const { unmount } = renderHook(() =>
      useColumnConfig(mockColumns, ['a'])
    )
    unmount()

    // Now load without exclusions — 'a' should be present from stored config
    const { result } = renderHook(() =>
      useColumnConfig(mockColumns)
    )
    expect(result.current.visibleColumns).toContain('a')
    expect(result.current.visibleColumns[0]).toBe('a')
  })

  it('resets to defaults when all visible columns become excluded', () => {
    localStorage.setItem('history-columns', JSON.stringify(['a']))
    let excludeColumns: string[] = []
    const { result, rerender } = renderHook(
      ({ exclude }) => useColumnConfig(mockColumns, exclude),
      { initialProps: { exclude: excludeColumns } }
    )

    expect(result.current.visibleColumns).toEqual(['a'])

    excludeColumns = ['a']
    rerender({ exclude: excludeColumns })

    expect(result.current.visibleColumns).toEqual(['b', 'd'])
  })
})
