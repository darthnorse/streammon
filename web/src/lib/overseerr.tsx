import { OVERSEERR_MEDIA_STATUS, OVERSEERR_REQUEST_STATUS } from '../types'

export const TMDB_IMG = 'https://image.tmdb.org/t/p'

function statusBadge(statusMap: Record<number, { label: string; color: string }>, status: number) {
  const info = statusMap[status]
  const label = info?.label ?? `Status ${status}`
  const color = info?.color ?? 'bg-gray-500/20 text-gray-400'
  return (
    <span className={`text-xs font-medium px-2.5 py-1 rounded-full ${color}`}>
      {label}
    </span>
  )
}

export function mediaStatusBadge(status: number | undefined) {
  if (!status || status === 1) return null
  return statusBadge(OVERSEERR_MEDIA_STATUS, status)
}

export function requestStatusBadge(status: number) {
  return statusBadge(OVERSEERR_REQUEST_STATUS, status)
}
