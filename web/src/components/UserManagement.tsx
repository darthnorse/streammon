import { useState } from 'react'
import { api } from '../lib/api'
import { useFetch } from '../hooks/useFetch'
import { useAuth } from '../context/AuthContext'
import { EmptyState } from './EmptyState'

interface AdminUser {
  id: number
  name: string
  email: string
  role: 'admin' | 'viewer'
  thumb_url: string
  created_at: string
  updated_at: string
}

const btnOutline = 'px-3 py-1.5 text-xs font-medium rounded-md border border-border dark:border-border-dark hover:border-accent/30 transition-colors'
const btnDanger = 'px-3 py-1.5 text-xs font-medium rounded-md border border-red-300 dark:border-red-500/30 text-red-600 dark:text-red-400 hover:bg-red-500/10 transition-colors'

export function UserManagement() {
  const { user: currentUser } = useAuth()
  const { data: users, loading, error, refetch } = useFetch<AdminUser[]>('/api/admin/users')
  const [actionError, setActionError] = useState('')
  const [updatingId, setUpdatingId] = useState<number | null>(null)

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
      setActionError(err instanceof Error ? err.message : 'Failed to update role')
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
      setActionError(err instanceof Error ? err.message : 'Failed to delete user')
    } finally {
      setUpdatingId(null)
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
                    <span className="font-medium truncate">{user.name}</span>
                    {isCurrentUser && (
                      <span className="text-xs text-muted dark:text-muted-dark">(you)</span>
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
                      disabled={isUpdating || isCurrentUser}
                      className={btnOutline + ((isUpdating || isCurrentUser) ? ' opacity-50 cursor-not-allowed' : '')}
                      title={isCurrentUser ? "You can't change your own role" : user.role === 'admin' ? 'Demote to viewer' : 'Promote to admin'}
                    >
                      {isUpdating ? '...' : user.role === 'admin' ? 'Demote' : 'Promote'}
                    </button>

                    <button
                      onClick={() => handleDelete(user)}
                      disabled={isUpdating || isCurrentUser}
                      className={btnDanger + ((isUpdating || isCurrentUser) ? ' opacity-50 cursor-not-allowed' : '')}
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
