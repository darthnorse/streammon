import { useState, useEffect, useRef } from 'react'
import type { Server } from '../types'

interface ServerDeleteDialogProps {
  server: Server
  error?: string
  onConfirm: (keepHistory: boolean) => void
  onCancel: () => void
}

export function ServerDeleteDialog({ server, error, onConfirm, onCancel }: ServerDeleteDialogProps) {
  const [keepHistory, setKeepHistory] = useState(true)
  const onCancelRef = useRef(onCancel)
  onCancelRef.current = onCancel

  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape') onCancelRef.current()
    }
    document.addEventListener('keydown', handleKeyDown)
    return () => document.removeEventListener('keydown', handleKeyDown)
  }, [])

  return (
    <div
      className="fixed inset-0 bg-black/50 flex items-center justify-center z-50"
      onClick={onCancel}
    >
      <div className="card p-6 max-w-md mx-4" onClick={e => e.stopPropagation()}>
        <h3 className="text-lg font-semibold mb-4">Delete &ldquo;{server.name}&rdquo;?</h3>

        <div className="space-y-3 mb-4">
          <label className="flex items-start gap-3 p-3 rounded-lg cursor-pointer hover:bg-surface dark:hover:bg-surface-dark transition-colors">
            <input
              type="radio"
              name="delete-mode"
              checked={keepHistory}
              onChange={() => setKeepHistory(true)}
              className="mt-0.5"
            />
            <div>
              <div className="text-sm font-medium">Keep watch history</div>
              <div className="text-xs text-muted dark:text-muted-dark mt-0.5">
                Remove the server but keep all watch history and statistics.
                The server will appear as deleted in filters.
              </div>
            </div>
          </label>

          <label className="flex items-start gap-3 p-3 rounded-lg cursor-pointer hover:bg-surface dark:hover:bg-surface-dark transition-colors">
            <input
              type="radio"
              name="delete-mode"
              checked={!keepHistory}
              onChange={() => setKeepHistory(false)}
              className="mt-0.5"
            />
            <div>
              <div className="text-sm font-medium text-red-500 dark:text-red-400">Delete everything</div>
              <div className="text-xs text-red-500/80 dark:text-red-400/80 mt-0.5">
                Permanently delete the server and ALL associated watch history.
                This cannot be undone.
              </div>
            </div>
          </label>
        </div>

        {error && (
          <div className="text-sm text-red-500 dark:text-red-400 mb-3">
            {error}
          </div>
        )}

        <div className="flex justify-end gap-3">
          <button
            onClick={onCancel}
            className="px-4 py-2 text-sm font-medium rounded-lg border border-border dark:border-border-dark
                     hover:bg-surface dark:hover:bg-surface-dark transition-colors"
          >
            Cancel
          </button>
          <button
            onClick={() => onConfirm(keepHistory)}
            className={`px-4 py-2 text-sm font-medium rounded-lg transition-colors ${
              keepHistory
                ? 'bg-accent text-gray-900 hover:bg-accent/90'
                : 'bg-red-500 text-white hover:bg-red-600'
            }`}
          >
            {keepHistory ? 'Delete Server' : 'Delete Everything'}
          </button>
        </div>
      </div>
    </div>
  )
}
