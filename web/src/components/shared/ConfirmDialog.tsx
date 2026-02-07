import { ReactNode } from 'react'

interface ConfirmDialogProps {
  title: string
  message: ReactNode
  confirmLabel: string
  cancelLabel?: string
  onConfirm: () => void
  onCancel: () => void
  isDestructive?: boolean
  disabled?: boolean
  children?: ReactNode
}

export function ConfirmDialog({
  title,
  message,
  confirmLabel,
  cancelLabel = 'Cancel',
  onConfirm,
  onCancel,
  isDestructive = false,
  disabled = false,
  children,
}: ConfirmDialogProps) {
  return (
    <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
      <div className="card p-6 max-w-md mx-4">
        <h3 className="text-lg font-semibold mb-2">{title}</h3>
        <div className="text-muted dark:text-muted-dark mb-4">{message}</div>
        {children}
        <div className="flex justify-end gap-3 mt-4">
          <button
            onClick={onCancel}
            disabled={disabled}
            className="px-4 py-2 text-sm font-medium rounded-lg border border-border dark:border-border-dark
                     hover:bg-surface dark:hover:bg-surface-dark transition-colors disabled:opacity-50"
          >
            {cancelLabel}
          </button>
          <button
            onClick={onConfirm}
            disabled={disabled}
            className={`px-4 py-2 text-sm font-medium rounded-lg transition-colors disabled:opacity-50
              ${isDestructive
                ? 'bg-red-500 text-white hover:bg-red-600'
                : 'bg-accent text-gray-900 hover:bg-accent/90'
              }`}
          >
            {confirmLabel}
          </button>
        </div>
      </div>
    </div>
  )
}
