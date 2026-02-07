import { formatSize } from '../../lib/format'

export interface DeleteProgress {
  current: number
  total: number
  title: string
  status: 'deleting' | 'deleted' | 'failed' | 'skipped'
  deleted: number
  failed: number
  skipped: number
  total_size: number
}

interface DeleteProgressModalProps {
  progress: DeleteProgress
  onCancel?: () => void
}

export function DeleteProgressModal({ progress, onCancel }: DeleteProgressModalProps) {
  const percent = progress.total > 0 ? Math.round((progress.current / progress.total) * 100) : 0

  return (
    <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50 animate-slide-up">
      <div className="bg-panel dark:bg-panel-dark rounded-lg shadow-xl max-w-md w-full mx-4 p-6">
        <h2 className="text-lg font-semibold mb-4">Deleting Items</h2>

        <div className="mb-4">
          <div className="flex justify-between text-sm mb-1">
            <span className="text-muted dark:text-muted-dark">Progress</span>
            <span>{progress.current} of {progress.total}</span>
          </div>
          <div className="w-full h-2 bg-surface dark:bg-surface-dark rounded-full overflow-hidden">
            <div
              className="h-full bg-accent transition-all duration-300 ease-out"
              style={{ width: `${percent}%` }}
            />
          </div>
        </div>

        <div className="mb-4 p-3 rounded bg-surface dark:bg-surface-dark">
          <div className="text-sm text-muted dark:text-muted-dark mb-1">Currently processing:</div>
          <div className="font-medium truncate" title={progress.title}>
            {progress.title}
          </div>
        </div>

        <div className="grid grid-cols-3 gap-4 mb-4 text-center">
          <div>
            <div className="text-2xl font-semibold text-green-500">{progress.deleted}</div>
            <div className="text-xs text-muted dark:text-muted-dark">Deleted</div>
          </div>
          <div>
            <div className="text-2xl font-semibold text-red-500">{progress.failed}</div>
            <div className="text-xs text-muted dark:text-muted-dark">Failed</div>
          </div>
          <div>
            <div className="text-2xl font-semibold text-amber-500">{progress.skipped}</div>
            <div className="text-xs text-muted dark:text-muted-dark">Skipped</div>
          </div>
        </div>

        {progress.total_size > 0 && (
          <div className="text-sm text-muted dark:text-muted-dark text-center mb-4">
            Space reclaimed: {formatSize(progress.total_size)}
          </div>
        )}

        {onCancel && (
          <div className="flex justify-end">
            <button
              onClick={onCancel}
              className="px-4 py-2 text-sm font-medium rounded border border-border dark:border-border-dark
                       hover:bg-surface dark:hover:bg-surface-dark transition-colors"
            >
              Cancel
            </button>
          </div>
        )}
      </div>
    </div>
  )
}
