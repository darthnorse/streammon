import type { ViewMode } from '../../types'

interface ViewModeToggleProps {
  viewMode: ViewMode
  onChange: (mode: ViewMode) => void
}

const modes: { value: ViewMode; label: string }[] = [
  { value: 'heatmap', label: 'Heatmap' },
  { value: 'markers', label: 'Markers' },
]

export function ViewModeToggle({ viewMode, onChange }: ViewModeToggleProps) {
  return (
    <div className="flex gap-1 bg-gray-100 dark:bg-gray-800 rounded-md p-0.5" role="group" aria-label="Map view mode">
      {modes.map(({ value, label }) => (
        <button
          key={value}
          onClick={() => onChange(value)}
          aria-pressed={viewMode === value}
          className={`px-2 py-1 text-xs rounded transition-colors ${
            viewMode === value
              ? 'bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100 shadow-sm'
              : 'text-gray-600 dark:text-gray-400 hover:text-gray-900 dark:hover:text-gray-200'
          }`}
        >
          {label}
        </button>
      ))}
    </div>
  )
}
