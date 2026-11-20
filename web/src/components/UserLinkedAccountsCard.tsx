import { useMemo, useState } from 'react'
import { useFetch } from '../hooks/useFetch'
import { api } from '../lib/api'
import type { UserAliasesResponse, UserSummary } from '../types'

interface UserLinkedAccountsCardProps {
  userName: string
  onLinked?: () => void
}

export function UserLinkedAccountsCard({ userName, onLinked }: UserLinkedAccountsCardProps) {
  const base = `/api/users/${encodeURIComponent(userName)}/aliases`
  const { data, error, refetch } = useFetch<UserAliasesResponse>(base)
  const aliases = useMemo(() => data?.aliases ?? [], [data])

  const [adding, setAdding] = useState(false)
  const [draft, setDraft] = useState('')
  const [busy, setBusy] = useState(false)
  const [actionError, setActionError] = useState<string | null>(null)
  const [message, setMessage] = useState<string | null>(null)

  const { data: users } = useFetch<UserSummary[]>(adding ? '/api/users/summary' : null)
  const suggestions = useMemo(
    () => (users ?? []).map(u => u.name).filter(n => n !== userName && !aliases.includes(n)),
    [users, userName, aliases],
  )

  const run = async (action: () => Promise<string | null>) => {
    setBusy(true)
    setActionError(null)
    setMessage(null)
    try {
      setMessage(await action())
      refetch()
    } catch (err) {
      setActionError((err as Error).message)
    } finally {
      setBusy(false)
    }
  }

  const link = () => {
    const alias = draft.trim()
    if (!alias) return
    if (!window.confirm(
      `Merge "${alias}" into "${userName}"?\n\n` +
      `"${alias}"'s watch history moves to "${userName}" and the "${alias}" user is deleted. ` +
      `New plays from "${alias}" on any server will be recorded as "${userName}".`,
    )) return
    run(async () => {
      const res = await api.post<UserAliasesResponse>(base, { alias })
      setAdding(false)
      setDraft('')
      onLinked?.()
      const moved = res.watch_history_moved ?? 0
      return `Linked "${alias}" — moved ${moved} ${moved === 1 ? 'play' : 'plays'}.`
    })
  }

  const unlink = (alias: string) => {
    if (!window.confirm(
      `Unlink "${alias}" from "${userName}"?\n\n` +
      `Plays already merged stay with "${userName}". New plays from "${alias}" will be recorded as "${alias}" again.`,
    )) return
    run(async () => {
      await api.del(`${base}/${encodeURIComponent(alias)}`)
      return null
    })
  }

  return (
    <div className="card p-4">
      <div className="flex items-center justify-between mb-2">
        <h2 className="text-sm font-semibold text-muted dark:text-muted-dark">Linked accounts</h2>
        {!adding && !error && (
          <button onClick={() => setAdding(true)} className="text-sm hover:text-accent hover:underline">
            + Link account
          </button>
        )}
      </div>

      {error ? (
        <div className="flex items-center justify-between gap-2">
          <p className="text-sm text-muted dark:text-muted-dark">Failed to load linked accounts.</p>
          <button onClick={() => refetch()} className="text-sm hover:text-accent hover:underline">
            Retry
          </button>
        </div>
      ) : aliases.length > 0 ? (
        <ul className="space-y-1">
          {aliases.map(alias => (
            <li key={alias} className="flex items-center justify-between gap-2 text-sm">
              <span className="truncate">{alias}</span>
              <button
                onClick={() => unlink(alias)}
                disabled={busy}
                aria-label={`Unlink ${alias}`}
                className="shrink-0 text-muted dark:text-muted-dark hover:text-accent hover:underline disabled:opacity-50"
              >
                Unlink
              </button>
            </li>
          ))}
        </ul>
      ) : data ? (
        <p className="text-sm text-muted dark:text-muted-dark">
          No linked accounts. Link a username from another media server to merge its plays here.
        </p>
      ) : null}

      {adding && (
        <div className="mt-3 flex flex-col sm:flex-row gap-2">
          <input
            value={draft}
            onChange={e => setDraft(e.target.value)}
            onKeyDown={e => { if (e.key === 'Enter') link() }}
            list="linked-account-suggestions"
            aria-label="Username to link"
            placeholder="Username on another server"
            autoFocus
            className="flex-1 min-w-0 rounded-lg border border-border dark:border-border-dark bg-transparent px-3 py-2 text-sm"
          />
          <datalist id="linked-account-suggestions">
            {suggestions.map(n => <option key={n} value={n} />)}
          </datalist>
          <div className="flex gap-2">
            <button
              onClick={link}
              disabled={busy || !draft.trim()}
              className="px-4 py-2 text-sm font-medium rounded-lg bg-accent text-gray-900 hover:bg-accent/90 disabled:opacity-50"
            >
              Link
            </button>
            <button
              onClick={() => { setAdding(false); setDraft(''); setActionError(null) }}
              disabled={busy}
              className="px-4 py-2 text-sm hover:text-accent hover:underline disabled:opacity-50"
            >
              Cancel
            </button>
          </div>
        </div>
      )}

      {actionError && <p className="mt-2 text-sm text-red-500 dark:text-red-400">{actionError}</p>}
      {message && <p className="mt-2 text-sm text-muted dark:text-muted-dark">{message}</p>}
    </div>
  )
}
