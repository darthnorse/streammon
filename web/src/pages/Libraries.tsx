import { useState } from 'react'
import { useFetch } from '../hooks/useFetch'
import type { LibrariesResponse, Library, LibraryType, ServerType } from '../types'

const serverAccent: Record<ServerType, string> = {
  plex: 'bg-warn/10 text-warn',
  emby: 'bg-emby/10 text-emby',
  jellyfin: 'bg-jellyfin/10 text-jellyfin',
}

const libraryTypeIcon: Record<LibraryType, string> = {
  movie: '◉',
  show: '▦',
  music: '♫',
  other: '▤',
}

const libraryTypeLabel: Record<LibraryType, string> = {
  movie: 'Movies',
  show: 'TV Shows',
  music: 'Music',
  other: 'Other',
}

function formatCount(count: number): string {
  return count.toLocaleString()
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

function LibraryRow({ library }: { library: Library }) {
  const accent = serverAccent[library.server_type] || 'bg-gray-100 text-gray-600'
  const icon = libraryTypeIcon[library.type] || '▤'

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
      <td className="hidden md:table-cell px-4 py-3 text-right text-muted dark:text-muted-dark">
        {library.grandchild_count > 0 ? formatCount(library.grandchild_count) : '-'}
      </td>
    </tr>
  )
}

export function Libraries() {
  const [selectedServer, setSelectedServer] = useState<number | 'all'>('all')
  const { data, loading, error } = useFetch<LibrariesResponse>('/api/libraries')

  const libraries = data?.libraries || []
  const servers = getUniqueServers(libraries)

  const displayedLibraries = selectedServer === 'all'
    ? libraries
    : libraries.filter(l => l.server_id === selectedServer)

  const totals = displayedLibraries.reduce(
    (acc, lib) => ({
      items: acc.items + lib.item_count,
      children: acc.children + lib.child_count,
      grandchildren: acc.grandchildren + lib.grandchild_count,
    }),
    { items: 0, children: 0, grandchildren: 0 }
  )

  return (
    <div>
      <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-4 mb-6">
        <div>
          <h1 className="text-2xl font-semibold">Libraries</h1>
          <p className="text-sm text-muted dark:text-muted-dark mt-1">
            Content from all media servers
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
        <div className="card overflow-hidden">
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
                  <th className="hidden md:table-cell px-4 py-3 text-right text-xs font-semibold text-muted dark:text-muted-dark uppercase tracking-wider">
                    Episodes / Tracks
                  </th>
                </tr>
              </thead>
              <tbody>
                {displayedLibraries.map(library => (
                  <LibraryRow key={`${library.server_id}-${library.id}`} library={library} />
                ))}
                {displayedLibraries.length === 0 && (
                  <tr>
                    <td colSpan={6} className="px-4 py-12 text-center">
                      <div className="text-4xl mb-3 opacity-30">▤</div>
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
                    <td className="hidden md:table-cell px-4 py-3 text-right text-muted dark:text-muted-dark">
                      {totals.grandchildren > 0 ? formatCount(totals.grandchildren) : '-'}
                    </td>
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
