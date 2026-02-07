import { useState } from 'react'

type ResultType = 'success' | 'partial' | 'error'

interface ErrorDetail {
  title: string
  error: string
}

interface OperationResultProps {
  type: ResultType
  message: string
  onDismiss: () => void
  errors?: ErrorDetail[]
}

const resultStyles: Record<ResultType, string> = {
  success: 'bg-green-500/10 text-green-500',
  partial: 'bg-amber-500/10 text-amber-500',
  error: 'bg-red-500/10 text-red-500',
}

export function OperationResult({ type, message, onDismiss, errors }: OperationResultProps) {
  const [showErrors, setShowErrors] = useState(false)
  // Defensive: handle null, undefined, or empty array
  const hasErrors = Array.isArray(errors) && errors.length > 0

  return (
    <div className={`p-3 rounded-lg text-sm ${resultStyles[type]}`}>
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-2">
          <span>{message}</span>
          {hasErrors && (
            <button
              onClick={() => setShowErrors(!showErrors)}
              className="underline hover:no-underline text-xs"
            >
              {showErrors ? 'Hide details' : 'Show details'}
            </button>
          )}
        </div>
        <button onClick={onDismiss} className="ml-4 hover:opacity-70">
          ✕
        </button>
      </div>
      {showErrors && hasErrors && (
        <div className="mt-2 pt-2 border-t border-current/20 max-h-32 overflow-y-auto">
          {errors.map((err, i) => (
            <div key={i} className="text-xs py-0.5">
              <span className="font-medium">{err.title}:</span> {err.error}
            </div>
          ))}
        </div>
      )}
    </div>
  )
}
