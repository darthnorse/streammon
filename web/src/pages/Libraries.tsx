import { useState, useEffect, useCallback, useMemo, useRef } from 'react'
import { useNavigate } from 'react-router-dom'
import { useFetch } from '../hooks/useFetch'
import { api, ApiError } from '../lib/api'
import { formatCount, formatSize } from '../lib/format'
import type {
  LibrariesResponse,
  Library,
  LibraryType,
  ServerType,
  MaintenanceDashboard,
  LibraryMaintenance,
  SyncProgress,
} from '../types'

const serverAccent: Record<ServerType, string> = {
  plex: 'bg-warn/10 text-warn',
  emby: 'bg-emby/10 text-emby',
  jellyfin: 'bg-jellyfin/10 text-jellyfin',
}

const libraryTypeIcon: Record<LibraryType, string> = {
  movie: '\u25C9',
  show: '\u25A6',
  music: '\u266B',
  other: '\u25A4',
}

const libraryTypeLabel: Record<LibraryType, string> = {
  movie: 'Movies',
  show: 'TV Shows',
  music: 'Music',
  other: 'Other',
}

function syncKey(lib: Library): string {
  return `${lib.server_id}-${lib.id}`
}

function getUniqueServers(libraries: Library[]): { id: number; name: string }[] {
  const seen = new Map<number, string>()
  for (const lib of libraries) {
    if (!seen.has(lib.server_id)) {
      seen.set(lib.server_id, lib.server_name)
    }
  }
  return Array.from(seen, ([id, name]) => ({ id, name }))
}

function formatSyncStatus(state: SyncProgress): string {
  if (state.phase === 'items') {
    if (state.total) {
      return `Scanning ${state.current ?? 0}/${state.total}`
    }
    return 'Scanning...'
  }
  if (state.phase === 'history') {
    if (state.total) {
      return `History ${state.current ?? 0}/${state.total}`
    }
    return 'Fetching history...'
  }
  return 'Syncing...'
}

interface LibraryRowProps {
  library: Library
  maintenance: LibraryMaintenance | null
  syncState: SyncProgress | null
  onSync: () => void
  onRules: () => void
  onViolations: () => void
}

function LibraryRow({ library, maintenance, syncState, onSync, onRules, onViolations }: LibraryRowProps) {
  const accent = serverAccent[library.server_type] || 'bg-gray-100 text-gray-600'
  const icon = libraryTypeIcon[library.type]
  const rules = maintenance?.rules || []
  const ruleCount = rules.length
  const violationCount = rules.reduce((sum, r) => sum + r.candidate_count, 0)
  const isMaintenanceSupported = library.type === 'movie' || library.type === 'show'

  return (
    <tr className="border-b border-border dark:border-border-dark hover:bg-gray-50 dark:hover:bg-white/5 transition-colors">
      <td className="px-4 py-3">
        <div className="flex items-center gap-3">
          <span className="text-xl">{icon}</span>
          <span className="font-medium text-gray-900 dark:text-gray-100">{library.name}</span>
        </div>
      </td>
      <td className="px-4 py-3">
        <span className={`inline-flex px-2 py-0.5 rounded text-xs font-medium ${accent}`}>
          {library.server_name}
        </span>
      </td>
      <td className="px-4 py-3 text-muted dark:text-muted-dark">
        {libraryTypeLabel[library.type]}
      </td>
      <td className="px-4 py-3 text-right font-medium text-gray-900 dark:text-gray-100">
        {formatCount(library.item_count)}
      </td>
      <td className="hidden md:table-cell px-4 py-3 text-right text-muted dark:text-muted-dark">
        {library.child_count > 0 ? formatCount(library.child_count) : '-'}
      </td>
      <td className="hidden lg:table-cell px-4 py-3 text-right text-muted dark:text-muted-dark">
        {library.grandchild_count > 0 ? formatCount(library.grandchild_count) : '-'}
      </td>
      <td className="hidden xl:table-cell px-4 py-3 text-right text-muted dark:text-muted-dark">
        {formatSize(library.total_size)}
      </td>
      <td className="px-4 py-3 text-center">
        {isMaintenanceSupported ? (
          <button
            onClick={onRules}
            className="text-sm hover:text-accent hover:underline"
          >
            {ruleCount}
          </button>
        ) : (
          <span className="text-muted dark:text-muted-dark">-</span>
        )}
      </td>
      <td className="px-4 py-3 text-center">
        {isMaintenanceSupported && violationCount > 0 ? (
          <button
            onClick={onViolations}
            className="text-sm text-amber-500 hover:underline font-medium"
          >
            {formatCount(violationCount)}
          </button>
        ) : (
          <span className="text-muted dark:text-muted-dark">-</span>
        )}
      </td>
      <td className="px-4 py-3">
        {isMaintenanceSupported && (
          <div className="flex items-center justify-end gap-2">
            <span className="relative group/sync">
              <button
                onClick={onSync}
                disabled={!!syncState}
                className="px-2 py-1 text-xs font-medium rounded border border-border dark:border-border-dark
                         hover:bg-surface dark:hover:bg-surface-dark transition-colors disabled:opacity-50"
              >
                {syncState ? formatSyncStatus(syncState) : 'Sync'}
                {syncState?.phase === 'history' && syncState.total && (
                  <span className="ml-0.5 opacity-60">&#9432;</span>
                )}
              </button>
              {syncState?.phase === 'history' && syncState.total && (
                <div className="absolute bottom-full right-0 mb-1 px-2 py-1 text-xs rounded w-48
                              bg-gray-900 text-white dark:bg-gray-700
                              opacity-0 group-hover/sync:opacity-100 pointer-events-none transition-opacity z-50">
                  This is total watch history, not episode count — includes rewatches
                </div>
              )}
            </span>
            <button
              onClick={onRules}
              className="px-2 py-1 text-xs font-medium rounded border border-border dark:border-border-dark
                       hover:bg-surface dark:hover:bg-surface-dark transition-colors"
            >
              Rules
            </button>
          </div>
        )}
      </td>
    </tr>
  )
}

export function Libraries() {
  const navigate = useNavigate()
  const [selectedServer, setSelectedServer] = useState<number | 'all'>('all')
  const [syncStates, setSyncStates] = useState<Record<string, SyncProgress>>({})
  const handledKeysRef = useRef(new Set<string>())
  const [syncError, setSyncError] = useState<string | null>(null)

  const { data, loading, error, refetch } = useFetch<LibrariesResponse>('/api/libraries')
  const { data: maintenanceData, refetch: refetchMaintenance } = useFetch<MaintenanceDashboard>('/api/maintenance/dashboard')

  const libraries = data?.libraries || []
  const servers = useMemo(() => getUniqueServers(libraries), [libraries])

  const displayedLibraries = useMemo(
    () => selectedServer === 'all'
      ? libraries
      : libraries.filter(l => l.server_id === selectedServer),
    [libraries, selectedServer]
  )

  const getMaintenanceForLibrary = useCallback((lib: Library): LibraryMaintenance | null => {
    return maintenanceData?.libraries.find(
      m => m.server_id === lib.server_id && m.library_id === lib.id
    ) || null
  }, [maintenanceData])

  // Poll for sync progress
  const hasSyncsRunning = Object.keys(syncStates).length > 0
  useEffect(() => {
    if (!hasSyncsRunning) return

    let active = true

    const poll = async () => {
      if (!active) return
      try {
        const status = await api.get<Record<string, SyncProgress>>('/api/maintenance/sync/status')
        if (!active) return

        let needRefresh = false
        const activeStates: Record<string, SyncProgress> = {}

        for (const [key, progress] of Object.entries(status)) {
          if (progress.phase === 'done') {
            if (!handledKeysRef.current.has(key)) {
              handledKeysRef.current.add(key)
              needRefresh = true
            }
          } else if (progress.phase === 'error') {
            if (!handledKeysRef.current.has(key)) {
              handledKeysRef.current.add(key)
              setSyncError(`Sync failed: ${progress.error}`)
              needRefresh = true
            }
          } else {
            activeStates[key] = progress
            handledKeysRef.current.delete(key)
          }
        }

        setSyncStates(prev => {
          const next: Record<string, SyncProgress> = { ...activeStates }
          for (const key of Object.keys(prev)) {
            if (!(key in next) && !(key in status)) {
              next[key] = prev[key]
            }
          }
          return next
        })
        if (needRefresh) {
          refetch()
          refetchMaintenance()
        }
      } catch { /* ignore polling errors */ }
    }

    poll()
    const interval = setInterval(poll, 1500)
    return () => { active = false; clearInterval(interval) }
  }, [hasSyncsRunning, refetch, refetchMaintenance])

  const handleSync = async (library: Library) => {
    setSyncError(null)
    try {
      await api.post('/api/maintenance/sync', {
        server_id: library.server_id,
        library_id: library.id,
      })
      setSyncStates(prev => ({
        ...prev,
        [syncKey(library)]: { phase: 'items', library: library.id },
      }))
    } catch (err) {
      if ((err as ApiError).status !== 409) {
        setSyncError(`Failed to start sync for "${library.name}"`)
      }
    }
  }

  const navigateToRules = useCallback((library: Library) => {
    navigate(`/rules?tab=maintenance&server_id=${library.server_id}&library_id=${encodeURIComponent(library.id)}`)
  }, [navigate])

  const totals = useMemo(
    () => displayedLibraries.reduce(
      (acc, lib) => ({
        items: acc.items + lib.item_count,
        children: acc.children + lib.child_count,
        grandchildren: acc.grandchildren + lib.grandchild_count,
        totalSize: acc.totalSize + lib.total_size,
      }),
      { items: 0, children: 0, grandchildren: 0, totalSize: 0 }
    ),
    [displayedLibraries]
  )

  return (
    <div>
      <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-4 mb-6">
        <div>
          <h1 className="text-2xl font-semibold">Libraries</h1>
          <p className="text-sm text-muted dark:text-muted-dark mt-1">
            Content and maintenance across all media servers
          </p>
        </div>

        {servers.length > 1 && (
          <div className="flex items-center gap-2">
            <label htmlFor="server-filter" className="text-sm text-muted dark:text-muted-dark">
              Server
            </label>
            <select
              id="server-filter"
              value={selectedServer}
              onChange={(e) => setSelectedServer(e.target.value === 'all' ? 'all' : Number(e.target.value))}
              className="px-3 py-1.5 text-sm rounded border border-border dark:border-border-dark bg-panel dark:bg-panel-dark text-gray-900 dark:text-gray-100"
            >
              <option value="all">All Servers</option>
              {servers.map(server => (
                <option key={server.id} value={server.id}>
                  {server.name}
                </option>
              ))}
            </select>
          </div>
        )}
      </div>

      {syncError && (
        <div className="mb-4 p-3 rounded-lg bg-red-500/10 text-red-500 text-sm flex items-center justify-between">
          <span>{syncError}</span>
          <button
            onClick={() => setSyncError(null)}
            className="ml-4 text-red-400 hover:text-red-300"
            aria-label="Dismiss error"
          >
            {'\u2715'}
          </button>
        </div>
      )}

      {error && (
        <div className="card p-6 text-center text-red-500 dark:text-red-400">
          Error loading libraries
        </div>
      )}

      {loading && !data && (
        <div className="card p-12 text-center">
          <div className="text-muted dark:text-muted-dark animate-pulse">Loading...</div>
        </div>
      )}

      {data && (
        <div className="card">
          <div className="overflow-x-auto">
            <table className="w-full">
              <thead>
                <tr className="bg-gray-50 dark:bg-white/5 border-b border-border dark:border-border-dark">
                  <th className="px-4 py-3 text-left text-xs font-semibold text-muted dark:text-muted-dark uppercase tracking-wider">
                    Library
                  </th>
                  <th className="px-4 py-3 text-left text-xs font-semibold text-muted dark:text-muted-dark uppercase tracking-wider">
                    Server
                  </th>
                  <th className="px-4 py-3 text-left text-xs font-semibold text-muted dark:text-muted-dark uppercase tracking-wider">
                    Type
                  </th>
                  <th className="px-4 py-3 text-right text-xs font-semibold text-muted dark:text-muted-dark uppercase tracking-wider">
                    Items
                  </th>
                  <th className="hidden md:table-cell px-4 py-3 text-right text-xs font-semibold text-muted dark:text-muted-dark uppercase tracking-wider">
                    Seasons / Albums
                  </th>
                  <th className="hidden lg:table-cell px-4 py-3 text-right text-xs font-semibold text-muted dark:text-muted-dark uppercase tracking-wider">
                    Episodes / Tracks
                  </th>
                  <th className="hidden xl:table-cell px-4 py-3 text-right text-xs font-semibold text-muted dark:text-muted-dark uppercase tracking-wider">
                    Total Size
                  </th>
                  <th className="px-4 py-3 text-center text-xs font-semibold text-muted dark:text-muted-dark uppercase tracking-wider">
                    Rules
                  </th>
                  <th className="px-4 py-3 text-center text-xs font-semibold text-muted dark:text-muted-dark uppercase tracking-wider">
                    Violations
                  </th>
                  <th className="px-4 py-3 text-right text-xs font-semibold text-muted dark:text-muted-dark uppercase tracking-wider">
                    Actions
                  </th>
                </tr>
              </thead>
              <tbody>
                {displayedLibraries.map(library => {
                  const maintenance = getMaintenanceForLibrary(library)
                  const key = syncKey(library)
                  return (
                    <LibraryRow
                      key={key}
                      library={library}
                      maintenance={maintenance}
                      syncState={syncStates[key] || null}
                      onSync={() => handleSync(library)}
                      onRules={() => navigateToRules(library)}
                      onViolations={() => navigateToRules(library)}
                    />
                  )
                })}
                {displayedLibraries.length === 0 && (
                  <tr>
                    <td colSpan={10} className="px-4 py-12 text-center">
                      <div className="text-4xl mb-3 opacity-30">{'\u25A4'}</div>
                      <p className="text-muted dark:text-muted-dark">No libraries found</p>
                    </td>
                  </tr>
                )}
              </tbody>
              {displayedLibraries.length > 0 && (
                <tfoot>
                  <tr className="bg-gray-50 dark:bg-white/5 border-t border-border dark:border-border-dark font-semibold">
                    <td className="px-4 py-3 text-gray-900 dark:text-gray-100" colSpan={3}>
                      Total ({displayedLibraries.length} {displayedLibraries.length === 1 ? 'library' : 'libraries'})
                    </td>
                    <td className="px-4 py-3 text-right text-gray-900 dark:text-gray-100">
                      {formatCount(totals.items)}
                    </td>
                    <td className="hidden md:table-cell px-4 py-3 text-right text-muted dark:text-muted-dark">
                      {totals.children > 0 ? formatCount(totals.children) : '-'}
                    </td>
                    <td className="hidden lg:table-cell px-4 py-3 text-right text-muted dark:text-muted-dark">
                      {totals.grandchildren > 0 ? formatCount(totals.grandchildren) : '-'}
                    </td>
                    <td className="hidden xl:table-cell px-4 py-3 text-right text-muted dark:text-muted-dark">
                      {formatSize(totals.totalSize)}
                    </td>
                    <td className="px-4 py-3" colSpan={3}></td>
                  </tr>
                </tfoot>
              )}
            </table>
          </div>
        </div>
      )}
    </div>
  )
}
