import { useEffect } from 'react'
import { Link } from 'react-router-dom'
import { useFetch } from '../hooks/useFetch'
import { formatDate } from '../lib/format'
import type { ServerHealthResponse } from '../types'

const REFRESH_MS = 30_000

export function ServerHealthBanner() {
  const { data, refetch } = useFetch<ServerHealthResponse>('/api/dashboard/server-health')

  useEffect(() => {
    const id = setInterval(refetch, REFRESH_MS)
    return () => clearInterval(id)
  }, [refetch])

  const failing = data?.failing ?? []
  if (failing.length === 0) return null

  return (
    <div role="alert" className="mb-6 rounded-lg border border-warn/40 bg-warn/10 px-4 py-3 text-sm">
      <p className="font-semibold text-amber-700 dark:text-warn">
        {failing.length === 1 ? 'A media server is' : `${failing.length} media servers are`} not responding — playback on {failing.length === 1 ? 'it' : 'them'} is not being recorded
      </p>
      <ul className="mt-2 space-y-1.5">
        {failing.map(f => (
          <li key={f.server_id} className="min-w-0">
            <span className="font-medium">{f.server_name}</span>
            <span className="text-muted dark:text-muted-dark"> · failing since {formatDate(f.failing_since)}</span>
            <code className="block mt-0.5 text-xs font-mono text-muted dark:text-muted-dark break-all">{f.error}</code>
          </li>
        ))}
      </ul>
      <Link to="/settings" className="inline-block mt-2 text-xs hover:text-accent hover:underline">
        Check server settings →
      </Link>
    </div>
  )
}
