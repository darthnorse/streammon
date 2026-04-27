interface ToggleSwitchProps {
  enabled: boolean
  onToggle: () => void
  disabled?: boolean
  ariaLabel?: string
  className?: string
}

export function ToggleSwitch({ enabled, onToggle, disabled, ariaLabel, className }: ToggleSwitchProps) {
  return (
    <button
      type="button"
      onClick={onToggle}
      disabled={disabled}
      className={`relative inline-flex h-6 w-11 shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors duration-200 ${
        enabled ? 'bg-accent' : 'bg-gray-300 dark:bg-white/20'
      } ${disabled ? 'opacity-50 cursor-not-allowed' : ''} ${className ?? ''}`}
      role="switch"
      aria-checked={enabled}
      aria-label={ariaLabel}
    >
      <span
        className={`pointer-events-none inline-block h-5 w-5 rounded-full bg-white shadow transform transition-transform duration-200 ${
          enabled ? 'translate-x-5' : 'translate-x-0'
        }`}
      />
    </button>
  )
}
