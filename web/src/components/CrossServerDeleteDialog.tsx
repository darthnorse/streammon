import { useState, useMemo } from 'react'
import { useFetch } from '../hooks/useFetch'
import { formatSize } from '../lib/format'
import type { LibraryItemCache, LibrariesResponse } from '../types'

interface CrossServerDeleteDialogProps {
  candidateId: number
  item: LibraryItemCache
  onConfirm: (candidateId: number, sourceItemId: number, crossServerItemIds: number[]) => void
  onCancel: () => void
}

export function CrossServerDeleteDialog({ candidateId, item, onConfirm, onCancel }: CrossServerDeleteDialogProps) {
  const { data: crossServerItems, loading, error } = useFetch<LibraryItemCache[]>(
    `/api/maintenance/candidates/${candidateId}/cross-server`
  )
  const { data: librariesData } = useFetch<LibrariesResponse>('/api/libraries')

  // Build lookup maps for server and library names
  const serverNames = useMemo(() => {
    const map = new Map<number, string>()
    if (librariesData?.libraries) {
      for (const lib of librariesData.libraries) {
        if (!map.has(lib.server_id)) {
          map.set(lib.server_id, lib.server_name)
        }
      }
    }
    return map
  }, [librariesData?.libraries])

  const libraryNames = useMemo(() => {
    const map = new Map<string, string>()
    if (librariesData?.libraries) {
      for (const lib of librariesData.libraries) {
        map.set(`${lib.server_id}-${lib.id}`, lib.name)
      }
    }
    return map
  }, [librariesData?.libraries])

  // Other-server matches (exclude the original item)
  const otherItems = useMemo(() => {
    if (!crossServerItems) return []
    return crossServerItems.filter(ci => ci.id !== item.id)
  }, [crossServerItems, item.id])

  const hasMatches = otherItems.length > 0

  // Track selected cross-server item IDs (start unchecked — user must opt in)
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

  const handleConfirm = () => {
    onConfirm(candidateId, item.id, Array.from(selectedOtherIds))
  }

  const getServerName = (serverId: number): string => {
    return serverNames.get(serverId) ?? `Server ${serverId}`
  }

  const getLibraryName = (serverId: number, libraryId: string): string => {
    return libraryNames.get(`${serverId}-${libraryId}`) ?? libraryId
  }

  if (loading) {
    return (
      <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
        <div className="card p-6 max-w-md mx-4">
          <div className="text-muted dark:text-muted-dark animate-pulse">Checking for cross-server matches...</div>
        </div>
      </div>
    )
  }

  if (error) {
    return (
      <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
        <div className="card p-6 max-w-md mx-4">
          <h3 className="text-lg font-semibold mb-2">Delete {item.title}?</h3>
          <div className="text-muted dark:text-muted-dark mb-4">
            Could not check for cross-server matches. Delete from current server only?
          </div>
          <div className="flex justify-end gap-3 mt-4">
            <button
              onClick={onCancel}
              className="px-4 py-2 text-sm font-medium rounded-lg border border-border dark:border-border-dark
                       hover:bg-surface dark:hover:bg-surface-dark transition-colors"
            >
              Cancel
            </button>
            <button
              onClick={() => onConfirm(candidateId, item.id, [])}
              className="px-4 py-2 text-sm font-medium rounded-lg bg-red-500 text-white hover:bg-red-600 transition-colors"
            >
              Delete
            </button>
          </div>
        </div>
      </div>
    )
  }

  // Simple confirmation if no cross-server matches
  if (!hasMatches) {
    return (
      <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
        <div className="card p-6 max-w-md mx-4">
          <h3 className="text-lg font-semibold mb-2">Delete {item.title}?</h3>
          <div className="text-muted dark:text-muted-dark mb-4">
            <p>This will permanently delete this file from your media server. This cannot be undone.</p>
            {item.file_size ? (
              <p className="text-sm mt-2 font-medium">{formatSize(item.file_size)} will be reclaimed.</p>
            ) : null}
          </div>
          <div className="flex justify-end gap-3 mt-4">
            <button
              onClick={onCancel}
              className="px-4 py-2 text-sm font-medium rounded-lg border border-border dark:border-border-dark
                       hover:bg-surface dark:hover:bg-surface-dark transition-colors"
            >
              Cancel
            </button>
            <button
              onClick={() => onConfirm(candidateId, item.id, [])}
              className="px-4 py-2 text-sm font-medium rounded-lg bg-red-500 text-white hover:bg-red-600 transition-colors"
            >
              Delete
            </button>
          </div>
        </div>
      </div>
    )
  }

  const selectedCount = 1 + selectedOtherIds.size
  return (
    <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
      <div className="card p-6 max-w-md mx-4">
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
              <span className="font-medium">{getServerName(item.server_id)}</span>
              <span className="text-muted dark:text-muted-dark"> &mdash; {getLibraryName(item.server_id, item.library_id)}</span>
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
                <span className="font-medium">{getServerName(ci.server_id)}</span>
                <span className="text-muted dark:text-muted-dark"> &mdash; {getLibraryName(ci.server_id, ci.library_id)}</span>
                {ci.file_size ? (
                  <span className="text-muted dark:text-muted-dark"> ({formatSize(ci.file_size)})</span>
                ) : null}
              </div>
            </label>
          ))}
        </div>

        <div className="flex justify-end gap-3">
          <button
            onClick={onCancel}
            className="px-4 py-2 text-sm font-medium rounded-lg border border-border dark:border-border-dark
                     hover:bg-surface dark:hover:bg-surface-dark transition-colors"
          >
            Cancel
          </button>
          <button
            onClick={handleConfirm}
            className="px-4 py-2 text-sm font-medium rounded-lg bg-red-500 text-white hover:bg-red-600 transition-colors"
          >
            Delete Selected ({selectedCount})
          </button>
        </div>
      </div>
    </div>
  )
}
