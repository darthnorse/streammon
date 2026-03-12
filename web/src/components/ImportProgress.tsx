import type { ImportProgress, ImportResult } from '../types'

interface ImportProgressBarProps {
  progress: ImportProgress
}

export function ImportProgressBar({ progress }: ImportProgressBarProps) {
  if (progress.total <= 0) return null
  return (
    <div className="space-y-2">
      <div className="flex justify-between text-xs text-muted dark:text-muted-dark">
        <span>Importing history...</span>
        <span>{progress.processed} / {progress.total}</span>
      </div>
      <div className="w-full bg-surface dark:bg-surface-dark rounded-full h-2 overflow-hidden">
        <div
          className="bg-accent h-2 rounded-full transition-all duration-300"
          style={{ width: `${Math.round((progress.processed / progress.total) * 100)}%` }}
        />
      </div>
    </div>
  )
}

interface ImportResultBannerProps {
  result: ImportResult
}

export function ImportResultBanner({ result }: ImportResultBannerProps) {
  if (result.error) return null
  return (
    <div className="text-sm font-mono px-3 py-2 rounded-lg bg-green-500/10 text-green-600 dark:text-green-400">
      Imported {result.imported.toLocaleString()} records
      {result.consolidated > 0 && `, merged ${result.consolidated.toLocaleString()}`}
      {result.skipped > 0 && `, skipped ${result.skipped.toLocaleString()} duplicates`}
    </div>
  )
}
