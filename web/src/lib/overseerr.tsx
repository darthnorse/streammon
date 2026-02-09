export const TMDB_IMG = 'https://image.tmdb.org/t/p'

export const OVERSEERR_MEDIA_STATUS: Record<number, { label: string; color: string }> = {
  1: { label: 'Unknown', color: 'bg-gray-500/80 text-white' },
  2: { label: 'Pending', color: 'bg-yellow-500/90 text-gray-900' },
  3: { label: 'Processing', color: 'bg-blue-500/80 text-white' },
  4: { label: 'Partial', color: 'bg-orange-500/80 text-white' },
  5: { label: 'Available', color: 'bg-green-600/80 text-white' },
  6: { label: 'Deleted', color: 'bg-red-500/80 text-white' },
}

export const OVERSEERR_REQUEST_STATUS: Record<number, { label: string; color: string }> = {
  1: { label: 'Pending', color: 'bg-yellow-500/90 text-gray-900' },
  2: { label: 'Approved', color: 'bg-green-600/80 text-white' },
  3: { label: 'Declined', color: 'bg-red-500/80 text-white' },
}

function statusBadge(statusMap: Record<number, { label: string; color: string }>, status: number, small?: boolean) {
  const info = statusMap[status]
  const label = info?.label ?? `Status ${status}`
  const color = info?.color ?? 'bg-gray-500/20 text-gray-400'
  const size = small ? 'text-[10px] px-1.5 py-0.5' : 'text-xs px-2 py-0.5'
  return (
    <span className={`font-medium rounded-full ${size} ${color}`}>
      {label}
    </span>
  )
}

export function mediaStatusBadge(status: number | undefined) {
  if (!status || status === 1) return null
  return statusBadge(OVERSEERR_MEDIA_STATUS, status, true)
}

export function requestStatusBadge(status: number) {
  return statusBadge(OVERSEERR_REQUEST_STATUS, status)
}
