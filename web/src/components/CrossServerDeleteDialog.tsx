import { useState, useMemo, useEffect } from 'react'
import { useFetch } from '../hooks/useFetch'
import { useLibraryLookup } from './MaintenanceRulesTab'
import { formatSize } from '../lib/format'
import type { LibraryItemCache } from '../types'

interface CrossServerDeleteDialogProps {
  candidateId: number
  item: LibraryItemCache
  onConfirm: (candidateId: number, sourceItemId: number, crossServerItemIds: number[]) => void
  onCancel: () => void
}

function DialogShell({ children, onCancel }: { children: React.ReactNode; onCancel: () => void }) {
  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape') onCancel()
    }
    document.addEventListener('keydown', handleKeyDown)
    return () => document.removeEventListener('keydown', handleKeyDown)
  }, [onCancel])

  return (
    <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
      <div className="card p-6 max-w-md mx-4">
        {children}
      </div>
    </div>
  )
}

function DialogActions({ onCancel, onConfirm, confirmLabel }: {
  onCancel: () => void
  onConfirm: () => void
  confirmLabel: string
}) {
  return (
    <div className="flex justify-end gap-3 mt-4">
      <button
        onClick={onCancel}
        className="px-4 py-2 text-sm font-medium rounded-lg border border-border dark:border-border-dark
                 hover:bg-surface dark:hover:bg-surface-dark transition-colors"
      >
        Cancel
      </button>
      <button
        onClick={onConfirm}
        className="px-4 py-2 text-sm font-medium rounded-lg bg-red-500 text-white hover:bg-red-600 transition-colors"
      >
        {confirmLabel}
      </button>
    </div>
  )
}

export function CrossServerDeleteDialog({ candidateId, item, onConfirm, onCancel }: CrossServerDeleteDialogProps) {
  const { data: crossServerItems, loading, error } = useFetch<LibraryItemCache[]>(
    `/api/maintenance/candidates/${candidateId}/cross-server`
  )
  const lookup = useLibraryLookup()

  const otherItems = useMemo(() => {
    if (!crossServerItems) return []
    return crossServerItems.filter(ci => ci.id !== item.id)
  }, [crossServerItems, item.id])

  const hasMatches = otherItems.length > 0
  const [selectedOtherIds, setSelectedOtherIds] = useState<Set<number>>(new Set())

  const toggleItem = (id: number) => {
    setSelectedOtherIds(prev => {
      const next = new Set(prev)
      if (next.has(id)) {
        next.delete(id)
      } else {
        next.add(id)
      }
      return next
    })
  }

  const confirmDeleteSource = () => onConfirm(candidateId, item.id, [])

  if (loading) {
    return (
      <DialogShell onCancel={onCancel}>
        <div className="text-muted dark:text-muted-dark animate-pulse">Checking for cross-server matches...</div>
      </DialogShell>
    )
  }

  if (error) {
    return (
      <DialogShell onCancel={onCancel}>
        <h3 className="text-lg font-semibold mb-2">Delete {item.title}?</h3>
        <div className="text-muted dark:text-muted-dark mb-4">
          Could not check for cross-server matches. Delete from current server only?
        </div>
        <DialogActions onCancel={onCancel} onConfirm={confirmDeleteSource} confirmLabel="Delete" />
      </DialogShell>
    )
  }

  if (!hasMatches) {
    return (
      <DialogShell onCancel={onCancel}>
        <h3 className="text-lg font-semibold mb-2">Delete {item.title}?</h3>
        <div className="text-muted dark:text-muted-dark mb-4">
          <p>This will permanently delete this file from your media server. This cannot be undone.</p>
          {item.file_size ? (
            <p className="text-sm mt-2 font-medium">{formatSize(item.file_size)} will be reclaimed.</p>
          ) : null}
        </div>
        <DialogActions onCancel={onCancel} onConfirm={confirmDeleteSource} confirmLabel="Delete" />
      </DialogShell>
    )
  }

  const selectedCount = 1 + selectedOtherIds.size
  return (
    <DialogShell onCancel={onCancel}>
      <h3 className="text-lg font-semibold mb-2">Delete {item.title}?</h3>
      <div className="text-muted dark:text-muted-dark mb-4">
        <p>This item exists on multiple servers. Select which copies to delete.</p>
        <p className="text-xs mt-1">Each server is contacted independently. If a server is unreachable, other deletions will still proceed and you can retry the failed ones.</p>
      </div>

      <div className="space-y-2 mb-4">
        {/* Original item - always checked, disabled */}
        <label className="flex items-center gap-3 py-1.5 px-2 rounded bg-surface dark:bg-surface-dark">
          <input
            type="checkbox"
            checked={true}
            disabled={true}
            className="rounded border-border dark:border-border-dark"
          />
          <div className="flex-1 text-sm">
            <span className="font-medium">{lookup.getServerName(item.server_id)}</span>
            <span className="text-muted dark:text-muted-dark"> &mdash; {lookup.getLibraryName(item.server_id, item.library_id)}</span>
            {item.file_size ? (
              <span className="text-muted dark:text-muted-dark"> ({formatSize(item.file_size)})</span>
            ) : null}
          </div>
        </label>

        {otherItems.map(ci => (
          <label
            key={ci.id}
            className="flex items-center gap-3 py-1.5 px-2 rounded hover:bg-surface dark:hover:bg-surface-dark cursor-pointer"
          >
            <input
              type="checkbox"
              checked={selectedOtherIds.has(ci.id)}
              onChange={() => toggleItem(ci.id)}
              className="rounded border-border dark:border-border-dark"
            />
            <div className="flex-1 text-sm">
              <span className="font-medium">{lookup.getServerName(ci.server_id)}</span>
              <span className="text-muted dark:text-muted-dark"> &mdash; {lookup.getLibraryName(ci.server_id, ci.library_id)}</span>
              {ci.file_size ? (
                <span className="text-muted dark:text-muted-dark"> ({formatSize(ci.file_size)})</span>
              ) : null}
            </div>
          </label>
        ))}
      </div>

      <DialogActions
        onCancel={onCancel}
        onConfirm={() => onConfirm(candidateId, item.id, Array.from(selectedOtherIds))}
        confirmLabel={`Delete Selected (${selectedCount})`}
      />
    </DialogShell>
  )
}
