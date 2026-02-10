import { useState, useCallback } from 'react'
import { Link } from 'react-router-dom'
import { api } from '../lib/api'
import { useFetch } from '../hooks/useFetch'
import { useAuth } from '../context/AuthContext'
import { errorMessage } from '../lib/utils'
import { EmptyState } from './EmptyState'
import type { AdminUser } from '../types'

interface SettingsToggleProps {
  endpoint: string
  title: string
  description: string
}

function SettingsToggle({ endpoint, title, description }: SettingsToggleProps) {
  const { data, loading, refetch } = useFetch<{ enabled: boolean }>(endpoint)
  const [optimistic, setOptimistic] = useState<boolean | null>(null)
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState('')

  const enabled = optimistic ?? data?.enabled ?? false

  const toggle = useCallback(async () => {
    if (!data) return
    const newValue = !enabled
    setOptimistic(newValue)
    setSaving(true)
    setError('')
    try {
      await api.put(endpoint, { enabled: newValue })
      setOptimistic(null)
      refetch()
    } catch (err) {
      setOptimistic(null)
      setError(errorMessage(err))
    } finally {
      setSaving(false)
    }
  }, [data, enabled, endpoint, refetch])

  if (loading) return null

  return (
    <div className="card p-4 mb-4">
      <div className="flex items-center justify-between">
        <div>
          <h3 className="font-semibold">{title}</h3>
          <p className="text-sm text-muted dark:text-muted-dark mt-0.5">
            {description}
          </p>
          {error && <p className="text-sm text-red-500 mt-1">{error}</p>}
        </div>
        <button
          onClick={toggle}
          disabled={saving}
          className={`relative inline-flex h-6 w-11 shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors duration-200 ${
            enabled ? 'bg-accent' : 'bg-gray-300 dark:bg-white/20'
          } ${saving ? 'opacity-50 cursor-not-allowed' : ''}`}
          role="switch"
          aria-checked={enabled}
        >
          <span
            className={`pointer-events-none inline-block h-5 w-5 rounded-full bg-white shadow transform transition-transform duration-200 ${
              enabled ? 'translate-x-5' : 'translate-x-0'
            }`}
          />
        </button>
      </div>
    </div>
  )
}

const btnOutline = 'px-3 py-1.5 text-xs font-medium rounded-md border border-border dark:border-border-dark hover:border-accent/30 transition-colors'
const btnDanger = 'px-3 py-1.5 text-xs font-medium rounded-md border border-red-300 dark:border-red-500/30 text-red-600 dark:text-red-400 hover:bg-red-500/10 transition-colors'
const disabledStyle = ' opacity-50 cursor-not-allowed'

function btnClass(base: string, disabled: boolean): string {
  return base + (disabled ? disabledStyle : '')
}

export function UserManagement() {
  const { user: currentUser } = useAuth()
  const { data: users, loading, error, refetch } = useFetch<AdminUser[]>('/api/admin/users')
  const [actionError, setActionError] = useState('')
  const [updatingId, setUpdatingId] = useState<number | null>(null)
  const [mergeKeepId, setMergeKeepId] = useState<number | null>(null)
  const [mergeDeleteId, setMergeDeleteId] = useState<number | null>(null)
  const [merging, setMerging] = useState(false)

  async function handleToggleRole(user: AdminUser) {
    const newRole = user.role === 'admin' ? 'viewer' : 'admin'
    const action = newRole === 'admin' ? 'promote to admin' : 'demote to viewer'

    if (!window.confirm(`Are you sure you want to ${action} "${user.name}"?`)) {
      return
    }

    setUpdatingId(user.id)
    setActionError('')

    try {
      await api.put(`/api/admin/users/${user.id}/role`, { role: newRole })
      refetch()
    } catch (err) {
      setActionError(errorMessage(err))
    } finally {
      setUpdatingId(null)
    }
  }

  async function handleDelete(user: AdminUser) {
    if (!window.confirm(`Are you sure you want to delete "${user.name}"? This action cannot be undone.`)) {
      return
    }

    setUpdatingId(user.id)
    setActionError('')

    try {
      await api.del(`/api/admin/users/${user.id}`)
      refetch()
    } catch (err) {
      setActionError(errorMessage(err))
    } finally {
      setUpdatingId(null)
    }
  }

  async function handleUnlink(user: AdminUser) {
    if (!window.confirm(`Unlink "${user.name}" from ${user.provider}? They can re-link their account on next login.`)) {
      return
    }

    setUpdatingId(user.id)
    setActionError('')

    try {
      await api.post(`/api/admin/users/${user.id}/unlink`, {})
      refetch()
    } catch (err) {
      setActionError(errorMessage(err))
    } finally {
      setUpdatingId(null)
    }
  }

  async function handleMerge() {
    if (!mergeKeepId || !mergeDeleteId) return

    const keepUser = users?.find(u => u.id === mergeKeepId)
    const deleteUser = users?.find(u => u.id === mergeDeleteId)
    if (!keepUser || !deleteUser) return

    if (!window.confirm(
      `Merge "${deleteUser.name}" into "${keepUser.name}"?\n\n` +
      `• All watch history from "${deleteUser.name}" will be transferred to "${keepUser.name}"\n` +
      `• "${deleteUser.name}" will be deleted\n\n` +
      `This action cannot be undone.`
    )) {
      return
    }

    setMerging(true)
    setActionError('')

    try {
      await api.post('/api/admin/users/merge', { keep_id: mergeKeepId, delete_id: mergeDeleteId })
      setMergeKeepId(null)
      setMergeDeleteId(null)
      refetch()
    } catch (err) {
      setActionError(errorMessage(err))
    } finally {
      setMerging(false)
    }
  }

  if (loading) {
    return <EmptyState icon="⟳" title="Loading..." />
  }

  if (error) {
    return (
      <EmptyState icon="!" title="Failed to load users">
        <button onClick={refetch} className="text-sm text-accent hover:underline">Retry</button>
      </EmptyState>
    )
  }

  const userList = users ?? []

  if (userList.length === 0) {
    return <EmptyState icon="👤" title="No users" description="Users will appear here after they sign in." />
  }

  return (
    <div>
      <SettingsToggle
        endpoint="/api/settings/guest-access"
        title="Guest Access"
        description="Allow non-admin users to sign in. When disabled, only admins can log in."
      />
      <SettingsToggle
        endpoint="/api/settings/trust-visibility"
        title="Trust Score Visibility"
        description="Allow non-admin users to see their own trust score and rule violations."
      />

      {actionError && (
        <div className="card p-4 mb-4 text-center text-red-500 dark:text-red-400">
          {actionError}
        </div>
      )}

      <div className="card">
        <div className="p-4 border-b border-border dark:border-border-dark">
          <h3 className="font-semibold">Registered Users</h3>
          <p className="text-sm text-muted dark:text-muted-dark mt-1">
            Manage user roles and access. Admins have full access, viewers can only see their own data.
          </p>
        </div>

        <div className="divide-y divide-border dark:divide-border-dark">
          {userList.map(user => {
            const isCurrentUser = currentUser?.id === user.id
            const isUpdating = updatingId === user.id
            const cantModify = isUpdating || isCurrentUser

            return (
              <div key={user.id} className="p-4 flex items-center gap-4">
                <div className="w-10 h-10 rounded-full bg-gray-200 dark:bg-white/10 overflow-hidden shrink-0">
                  {user.thumb_url ? (
                    <img
                      src={user.thumb_url}
                      alt=""
                      className="w-full h-full object-cover"
                    />
                  ) : (
                    <div className="w-full h-full flex items-center justify-center text-lg text-muted dark:text-muted-dark">
                      {user.name.charAt(0).toUpperCase()}
                    </div>
                  )}
                </div>

                <div className="flex-1 min-w-0">
                  <div className="flex items-center gap-2">
                    <Link
                      to={`/users/${encodeURIComponent(user.name)}`}
                      className="font-medium truncate hover:text-accent transition-colors"
                    >
                      {user.name}
                    </Link>
                    {isCurrentUser && (
                      <span className="text-xs text-muted dark:text-muted-dark">(you)</span>
                    )}
                    {user.provider && (
                      <span className="text-xs px-1.5 py-0.5 rounded bg-blue-100 dark:bg-blue-900/30 text-blue-700 dark:text-blue-300">
                        {user.provider}
                      </span>
                    )}
                  </div>
                  {user.email && (
                    <div className="text-sm text-muted dark:text-muted-dark truncate">
                      {user.email}
                    </div>
                  )}
                </div>

                <div className="flex items-center gap-3 shrink-0">
                  <span className={`badge ${user.role === 'admin' ? 'badge-accent' : 'badge-muted'}`}>
                    {user.role}
                  </span>

                  <div className="flex items-center gap-2">
                    <button
                      onClick={() => handleToggleRole(user)}
                      disabled={cantModify}
                      className={btnClass(btnOutline, cantModify)}
                      title={isCurrentUser ? "You can't change your own role" : user.role === 'admin' ? 'Demote to viewer' : 'Promote to admin'}
                    >
                      {isUpdating ? '...' : user.role === 'admin' ? 'Demote' : 'Promote'}
                    </button>

                    {user.provider && user.provider_id && (
                      <button
                        onClick={() => handleUnlink(user)}
                        disabled={isUpdating}
                        className={btnClass(btnOutline, isUpdating)}
                        title={`Unlink from ${user.provider}`}
                      >
                        Unlink
                      </button>
                    )}

                    <button
                      onClick={() => handleDelete(user)}
                      disabled={cantModify}
                      className={btnClass(btnDanger, cantModify)}
                      title={isCurrentUser ? "You can't delete yourself" : 'Delete user'}
                    >
                      Delete
                    </button>
                  </div>
                </div>
              </div>
            )
          })}
        </div>
      </div>

      {userList.length > 1 && (
        <div className="card p-4 mt-4">
          <h4 className="font-medium mb-2">Merge Users</h4>
          <p className="text-sm text-muted dark:text-muted-dark mb-4">
            Combine two user accounts by transferring watch history from one to another.
          </p>
          <div className="flex flex-wrap items-end gap-3">
            <div className="flex-1 min-w-[180px]">
              <label className="block text-xs text-muted dark:text-muted-dark mb-1">Keep this user</label>
              <select
                value={mergeKeepId ?? ''}
                onChange={(e) => setMergeKeepId(e.target.value ? Number(e.target.value) : null)}
                className="w-full px-3 py-2 text-sm rounded-md border border-border dark:border-border-dark bg-background dark:bg-background-dark"
              >
                <option value="">Select user to keep...</option>
                {userList.filter(u => u.id !== mergeDeleteId).map(u => (
                  <option key={u.id} value={u.id}>
                    {u.name}{u.provider ? ` (${u.provider})` : ''}
                  </option>
                ))}
              </select>
            </div>
            <div className="flex-1 min-w-[180px]">
              <label className="block text-xs text-muted dark:text-muted-dark mb-1">Merge and delete this user</label>
              <select
                value={mergeDeleteId ?? ''}
                onChange={(e) => setMergeDeleteId(e.target.value ? Number(e.target.value) : null)}
                className="w-full px-3 py-2 text-sm rounded-md border border-border dark:border-border-dark bg-background dark:bg-background-dark"
              >
                <option value="">Select user to merge...</option>
                {userList.filter(u => u.id !== mergeKeepId && u.id !== currentUser?.id).map(u => (
                  <option key={u.id} value={u.id}>
                    {u.name}{u.provider ? ` (${u.provider})` : ''}
                  </option>
                ))}
              </select>
            </div>
            <button
              onClick={handleMerge}
              disabled={!mergeKeepId || !mergeDeleteId || mergeKeepId === mergeDeleteId || merging}
              className={btnClass(btnOutline, !mergeKeepId || !mergeDeleteId || mergeKeepId === mergeDeleteId || merging)}
            >
              {merging ? 'Merging...' : 'Merge Users'}
            </button>
          </div>
        </div>
      )}

      <div className="card p-4 mt-4">
        <h4 className="font-medium mb-2">Role Permissions</h4>
        <div className="text-sm text-muted dark:text-muted-dark space-y-1">
          <p><strong className="text-foreground dark:text-foreground-dark">Admin:</strong> Full access to all features, settings, and all users' data</p>
          <p><strong className="text-foreground dark:text-foreground-dark">Viewer:</strong> Can only view their own watch history and stats</p>
        </div>
      </div>
    </div>
  )
}
