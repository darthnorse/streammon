import { useState, useCallback, useMemo } from 'react'
import { useNavigate } from 'react-router-dom'
import { useFetch } from '../hooks/useFetch'
import { Dropdown } from '../components/Dropdown'
import { formatCount, formatSize } from '../lib/format'
import { SERVER_ACCENT } from '../lib/constants'
import type {
  LibrariesResponse,
  Library,
  LibraryType,
  MaintenanceDashboard,
  LibraryMaintenance,
} from '../types'

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

function getUniqueServers(libraries: Library[]): { id: number; name: string }[] {
  const seen = new Map<number, string>()
  for (const lib of libraries) {
    if (!seen.has(lib.server_id)) {
      seen.set(lib.server_id, lib.server_name)
    }
  }
  return Array.from(seen, ([id, name]) => ({ id, name }))
}

interface LibraryRowProps {
  library: Library
  maintenance: LibraryMaintenance | null
  onRules: () => void
  onViolations: () => void
}

function LibraryRow({ library, maintenance, onRules, onViolations }: LibraryRowProps) {
  const accent = SERVER_ACCENT[library.server_type] || 'bg-gray-100 text-gray-600'
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
            className="text-sm font-medium hover:text-accent hover:underline"
          >
            {formatCount(violationCount)}
          </button>
        ) : (
          <span className="text-muted dark:text-muted-dark">-</span>
        )}
      </td>
      <td className="px-4 py-3">
        {isMaintenanceSupported && (
          <div className="flex items-center justify-end">
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
  const [selectedServers, setSelectedServers] = useState<string[]>([])

  const { data, loading, error } = useFetch<LibrariesResponse>('/api/libraries')
  const { data: maintenanceData } = useFetch<MaintenanceDashboard>('/api/maintenance/dashboard')

  const libraries = data?.libraries || []
  const servers = useMemo(() => getUniqueServers(libraries), [libraries])

  const selectedIds = useMemo(() => new Set(selectedServers.map(Number)), [selectedServers])

  const displayedLibraries = useMemo(
    () => selectedIds.size === 0
      ? libraries
      : libraries.filter(l => selectedIds.has(l.server_id)),
    [libraries, selectedIds]
  )

  const getMaintenanceForLibrary = (lib: Library): LibraryMaintenance | null => {
    return maintenanceData?.libraries.find(
      m => m.server_id === lib.server_id && m.library_id === lib.id
    ) || null
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
          <Dropdown
            multi
            options={servers.map(s => ({ value: String(s.id), label: s.name }))}
            selected={selectedServers}
            onChange={setSelectedServers}
            allLabel="All Servers"
            noneLabel="All Servers"
          />
        )}
      </div>

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
                {displayedLibraries.map(library => (
                  <LibraryRow
                    key={`${library.server_id}-${library.id}`}
                    library={library}
                    maintenance={getMaintenanceForLibrary(library)}
                    onRules={() => navigateToRules(library)}
                    onViolations={() => navigateToRules(library)}
                  />
                ))}
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
