import { useState, useEffect, useCallback } from 'react'
import type { Server } from '../types'
import { api } from '../lib/api'
import { ServerForm } from '../components/ServerForm'

const serverTypeColors: Record<string, string> = {
  plex: 'badge-warn',
  emby: 'badge-emby',
  jellyfin: 'badge-jellyfin',
}

export function Settings() {
  const [servers, setServers] = useState<Server[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [editingServer, setEditingServer] = useState<Server | undefined>()
  const [showForm, setShowForm] = useState(false)

  const fetchServers = useCallback(async () => {
    setLoading(true)
    setError('')
    try {
      const data = await api.get<Server[]>('/api/servers')
      setServers(data)
    } catch {
      setError('Failed to load servers')
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => { fetchServers() }, [fetchServers])

  function openAdd() {
    setEditingServer(undefined)
    setShowForm(true)
  }

  function openEdit(server: Server) {
    setEditingServer(server)
    setShowForm(true)
  }

  function closeForm() {
    setShowForm(false)
    setEditingServer(undefined)
  }

  function handleSaved() {
    closeForm()
    fetchServers()
  }

  async function handleDelete(server: Server) {
    if (!window.confirm(`Delete "${server.name}"?`)) return
    try {
      await api.del(`/api/servers/${server.id}`)
      setError('')
      fetchServers()
    } catch {
      setError('Failed to delete server')
    }
  }

  return (
    <div>
      <div className="flex items-center justify-between mb-6">
        <div>
          <h1 className="text-2xl font-semibold">Settings</h1>
          <p className="text-sm text-muted dark:text-muted-dark mt-1">
            Manage media servers
          </p>
        </div>
        <button onClick={openAdd} className="px-4 py-2.5 text-sm font-semibold rounded-lg
                   bg-accent text-gray-900 hover:bg-accent/90 transition-colors">
          Add Server
        </button>
      </div>

      {loading && (
        <div className="card p-12 text-center">
          <p className="text-muted dark:text-muted-dark">Loading...</p>
        </div>
      )}

      {error && !loading && (
        <div className="card p-12 text-center">
          <p className="text-red-500 dark:text-red-400">{error}</p>
          <button onClick={fetchServers} className="mt-3 text-sm text-accent hover:underline">
            Retry
          </button>
        </div>
      )}

      {!loading && !error && servers.length === 0 && (
        <div className="card p-12 text-center">
          <div className="text-4xl mb-3 opacity-30">&#9881;</div>
          <p className="text-muted dark:text-muted-dark">No servers configured</p>
          <p className="text-sm text-muted dark:text-muted-dark mt-1">
            Add a Plex, Emby, or Jellyfin server to get started
          </p>
        </div>
      )}

      {!loading && !error && servers.length > 0 && (
        <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
          {servers.map(srv => (
            <div key={srv.id} className="card card-hover p-5">
              <div className="flex items-start justify-between mb-3">
                <div className="min-w-0 flex-1">
                  <h3 className="font-semibold text-base truncate">{srv.name}</h3>
                  <p className="text-sm text-muted dark:text-muted-dark font-mono truncate mt-0.5">
                    {srv.url}
                  </p>
                </div>
                <span className={`badge ${srv.enabled ? 'badge-accent' : 'badge-muted'} ml-3 shrink-0`}>
                  {srv.enabled ? 'Enabled' : 'Disabled'}
                </span>
              </div>

              <div className="flex items-center gap-2 mb-4">
                <span className={`badge ${serverTypeColors[srv.type] ?? 'badge-muted'}`}>
                  {srv.type}
                </span>
                <span className="text-xs text-muted dark:text-muted-dark">
                  Added {new Date(srv.created_at).toLocaleDateString()}
                </span>
              </div>

              <div className="flex items-center gap-2 border-t border-border dark:border-border-dark pt-3">
                <button
                  onClick={() => openEdit(srv)}
                  aria-label="Edit"
                  className="px-3 py-1.5 text-xs font-medium rounded-md
                             border border-border dark:border-border-dark
                             hover:border-accent/30 transition-colors"
                >
                  Edit
                </button>
                <button
                  onClick={() => handleDelete(srv)}
                  aria-label="Delete"
                  className="px-3 py-1.5 text-xs font-medium rounded-md
                             border border-red-300 dark:border-red-500/30
                             text-red-600 dark:text-red-400
                             hover:bg-red-500/10 transition-colors"
                >
                  Delete
                </button>
              </div>
            </div>
          ))}
        </div>
      )}

      {showForm && (
        <ServerForm
          server={editingServer}
          onClose={closeForm}
          onSaved={handleSaved}
        />
      )}
    </div>
  )
}
