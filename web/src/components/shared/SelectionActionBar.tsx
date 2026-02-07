import { ReactNode } from 'react'

interface SelectionActionBarProps {
  selectedCount: number
  children: ReactNode
}

export function SelectionActionBar({ selectedCount, children }: SelectionActionBarProps) {
  if (selectedCount === 0) return null

  return (
    <div className="flex items-center gap-4 p-3 rounded-lg bg-surface dark:bg-surface-dark">
      <span className="text-sm font-medium">{selectedCount} selected</span>
      <div className="flex-1" />
      {children}
    </div>
  )
}
