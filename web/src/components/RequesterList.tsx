import type { OverseerrRequestUser } from '../types'
import { requesterDisplayName } from '../lib/overseerr'

interface RequesterListProps {
  requesters: OverseerrRequestUser[]
}

// RequesterList renders a compact "Requested by <avatars>" line. The caller is
// responsible for the admin gate — this component renders unconditionally when
// `requesters` is non-empty. The backend at /api/overseerr/movie|tv/{id}
// already strips `mediaInfo.requests` for non-admin callers, so passing a
// non-empty list here already implies an admin caller; the explicit gate at
// the call site is defense-in-depth.
export function RequesterList({ requesters }: RequesterListProps) {
  if (requesters.length === 0) return null
  return (
    <div className="flex items-center flex-wrap gap-x-3 gap-y-1 text-xs text-muted dark:text-muted-dark">
      <span>Requested by</span>
      {requesters.map(u => {
        const name = requesterDisplayName(u)
        return (
          <span key={u.id} className="inline-flex items-center gap-1.5">
            {u.avatar ? (
              <img
                src={u.avatar}
                alt=""
                className="w-5 h-5 rounded-full object-cover"
                loading="lazy"
                referrerPolicy="no-referrer"
              />
            ) : (
              <span aria-hidden="true" className="w-5 h-5 rounded-full bg-border dark:bg-border-dark inline-flex items-center justify-center text-[10px] text-muted dark:text-muted-dark">
                {name.slice(0, 1).toUpperCase()}
              </span>
            )}
            <span className="text-gray-900 dark:text-gray-100 font-medium">{name}</span>
          </span>
        )
      })}
    </div>
  )
}

// dedupRequesters collapses an Overseerr requests[] array down to a list of
// distinct requesters (one entry per user id, in first-appearance order).
// Same media can have multiple requests (partial seasons, re-requests, etc.).
export function dedupRequesters(
  requests: { requestedBy?: OverseerrRequestUser }[],
): OverseerrRequestUser[] {
  const byId = new Map<number, OverseerrRequestUser>()
  for (const r of requests) {
    if (r.requestedBy && !byId.has(r.requestedBy.id)) {
      byId.set(r.requestedBy.id, r.requestedBy)
    }
  }
  return [...byId.values()]
}
