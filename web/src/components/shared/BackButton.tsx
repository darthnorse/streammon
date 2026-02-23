interface BackButtonProps {
  onClick: () => void
  variant?: 'icon' | 'text'
}

export function BackButton({ onClick, variant = 'icon' }: BackButtonProps) {
  if (variant === 'text') {
    return (
      <button
        type="button"
        onClick={onClick}
        className="text-sm hover:text-accent hover:underline"
      >
        &larr; Back
      </button>
    )
  }

  return (
    <button
      onClick={onClick}
      className="p-2 rounded-lg hover:bg-surface dark:hover:bg-surface-dark transition-colors"
      aria-label="Go back"
    >
      <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M15 19l-7-7 7-7" />
      </svg>
    </button>
  )
}
