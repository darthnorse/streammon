import { useMemo } from 'react'
import { useParams, Link } from 'react-router-dom'
import { useFetch } from '../hooks/useFetch'
import { useMediaDetailModal } from '../hooks/useMediaDetailModal'
import { LibraryItemsView } from '../components/LibraryItemsView'
import { formatSize } from '../lib/format'
import type { LibrarySummary, Library } from '../types'

export function LibraryDetail() {
  const { serverId = '', libraryId = '' } = useParams()
  const { handleTitleClick, modal } = useMediaDetailModal()

  // Library name/type/server aren't stored per-item; resolve them from the
  // (cached, admin-only) libraries list to drive the header and the
  // per-type column layout.
  const { data: libsData, loading: libsLoading } = useFetch<{ libraries: Library[] }>('/api/libraries')
  const lib = useMemo(
    () => libsData?.libraries.find(l => String(l.server_id) === serverId && l.id === libraryId),
    [libsData, serverId, libraryId],
  )

  const base = `/api/libraries/${serverId}/${encodeURIComponent(libraryId)}`
  const { data: summary } = useFetch<LibrarySummary>(`${base}/summary`)
  const watchedPct = summary && summary.total_titles > 0
    ? Math.round((summary.watched_titles / summary.total_titles) * 100) : 0

  return (
    <div>
      <div className="mb-4">
        <Link to="/library" className="text-sm hover:text-accent hover:underline">← Libraries</Link>
        <h1 className="text-2xl font-semibold mt-1">{lib?.name ?? 'Library'}</h1>
        {lib && <p className="text-sm text-muted dark:text-muted-dark">{lib.server_name}</p>}
      </div>

      {summary && (
        <div className="grid grid-cols-2 md:grid-cols-5 gap-3 mb-6">
          <Stat label="Total titles" value={String(summary.total_titles)} />
          <Stat label="Total size" value={formatSize(summary.total_size)} />
          <Stat label="Ever watched" value={`${summary.watched_titles} (${watchedPct}%)`} />
          <Stat label="Never played" value={String(summary.never_played)} />
          <Stat label="Reclaimable" value={formatSize(summary.reclaimable_size)} />
        </div>
      )}

      {/* Wait for the libraries list so the column layout mounts with the right
          per-type storage key; re-key on type so it remounts if the type changes. */}
      {libsLoading && !libsData ? (
        <div className="card p-12 text-center text-muted dark:text-muted-dark animate-pulse">Loading...</div>
      ) : (
        <LibraryItemsView
          key={lib?.type ?? 'all'}
          serverId={serverId}
          libraryId={libraryId}
          libraryType={lib?.type}
          onTitleClick={handleTitleClick}
        />
      )}
      {modal}
    </div>
  )
}

function Stat({ label, value }: { label: string; value: string }) {
  return (
    <div className="card p-3">
      <div className="text-xs text-muted dark:text-muted-dark">{label}</div>
      <div className="text-lg font-semibold mt-0.5">{value}</div>
    </div>
  )
}
