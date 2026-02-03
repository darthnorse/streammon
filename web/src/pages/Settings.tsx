import { useState, useEffect, useCallback } from 'react'
import type { Server, OIDCSettings } from '../types'
import { api } from '../lib/api'
import { ServerForm } from '../components/ServerForm'
import { OIDCForm } from '../components/OIDCForm'
import { EmptyState } from '../components/EmptyState'

const serverTypeColors: Record<string, string> = {
  plex: 'badge-warn',
  emby: 'badge-emby',
  jellyfin: 'badge-jellyfin',
}

function LoadingCard() {
  return (
    <div className="card p-12 text-center">
      <p className="text-muted dark:text-muted-dark">Loading...</p>
    </div>
  )
}

function ErrorCard({ message, onRetry }: { message: string; onRetry: () => void }) {
  return (
    <div className="card p-12 text-center">
      <p className="text-red-500 dark:text-red-400">{message}</p>
      <button onClick={onRetry} className="mt-3 text-sm text-accent hover:underline">
        Retry
      </button>
    </div>
  )
}

export function Settings() {
  const [tab, setTab] = useState<'servers' | 'auth'>('servers')
  const [servers, setServers] = useState<Server[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [editingServer, setEditingServer] = useState<Server | undefined>()
  const [showForm, setShowForm] = useState(false)

  const [oidc, setOidc] = useState<OIDCSettings | undefined>()
  const [oidcLoading, setOidcLoading] = useState(true)
  const [oidcError, setOidcError] = useState('')
  const [showOidcForm, setShowOidcForm] = useState(false)

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

  const fetchOidc = useCallback(async () => {
    setOidcLoading(true)
    setOidcError('')
    try {
      const data = await api.get<OIDCSettings>('/api/settings/oidc')
      setOidc(data)
    } catch {
      setOidcError('Failed to load OIDC settings')
    } finally {
      setOidcLoading(false)
    }
  }, [])

  useEffect(() => { fetchServers() }, [fetchServers])
  useEffect(() => { if (tab === 'auth') fetchOidc() }, [tab, fetchOidc])

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

  function handleOidcSaved() {
    setShowOidcForm(false)
    fetchOidc()
  }

  async function handleDeleteOidc() {
    if (!window.confirm('Remove OIDC configuration? Authentication will be disabled.')) return
    try {
      await api.del('/api/settings/oidc')
      setOidcError('')
      fetchOidc()
    } catch {
      setOidcError('Failed to delete OIDC configuration')
    }
  }

  const oidcConfigured = !!oidc?.issuer

  return (
    <div>
      <div className="flex items-center justify-between mb-6">
        <div>
          <h1 className="text-2xl font-semibold">Settings</h1>
          <p className="text-sm text-muted dark:text-muted-dark mt-1">
            Manage media servers and authentication
          </p>
        </div>
        {tab === 'servers' && (
          <button onClick={openAdd} className="px-4 py-2.5 text-sm font-semibold rounded-lg
                     bg-accent text-gray-900 hover:bg-accent/90 transition-colors">
            Add Server
          </button>
        )}
      </div>

      <div className="flex gap-1 mb-6 border-b border-border dark:border-border-dark">
        <button
          onClick={() => setTab('servers')}
          className={`px-4 py-2 text-sm font-medium border-b-2 transition-colors ${
            tab === 'servers'
              ? 'border-accent text-accent'
              : 'border-transparent text-muted dark:text-muted-dark hover:text-gray-800 dark:hover:text-gray-200'
          }`}
        >
          Servers
        </button>
        <button
          onClick={() => setTab('auth')}
          className={`px-4 py-2 text-sm font-medium border-b-2 transition-colors ${
            tab === 'auth'
              ? 'border-accent text-accent'
              : 'border-transparent text-muted dark:text-muted-dark hover:text-gray-800 dark:hover:text-gray-200'
          }`}
        >
          Authentication
        </button>
      </div>

      {tab === 'servers' && (
        <>
          {loading && <LoadingCard />}

          {error && !loading && <ErrorCard message={error} onRetry={fetchServers} />}

          {!loading && !error && servers.length === 0 && (
            <EmptyState icon="&#9881;" title="No servers configured" description="Add a Plex, Emby, or Jellyfin server to get started" />
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
        </>
      )}

      {tab === 'auth' && (
        <>
          {oidcLoading && <LoadingCard />}

          {oidcError && !oidcLoading && <ErrorCard message={oidcError} onRetry={fetchOidc} />}

          {!oidcLoading && !oidcError && !oidcConfigured && (
            <div className="card p-8 text-center">
              <div className="text-4xl mb-3 opacity-30">&#128274;</div>
              <p className="font-medium mb-1">OIDC Not Configured</p>
              <p className="text-sm text-muted dark:text-muted-dark mb-4">
                Configure OpenID Connect to enable single sign-on authentication.
              </p>
              <button
                onClick={() => setShowOidcForm(true)}
                className="px-4 py-2.5 text-sm font-semibold rounded-lg
                           bg-accent text-gray-900 hover:bg-accent/90 transition-colors"
              >
                Configure OIDC
              </button>
            </div>
          )}

          {!oidcLoading && !oidcError && oidcConfigured && oidc && (
            <div className="card p-5">
              <div className="flex items-start justify-between mb-4">
                <h3 className="font-semibold text-base">OpenID Connect</h3>
                <span className={`badge ${oidc.enabled ? 'badge-accent' : 'badge-muted'}`}>
                  {oidc.enabled ? 'Enabled' : 'Disabled'}
                </span>
              </div>
              <div className="space-y-2 text-sm mb-4">
                <div>
                  <span className="text-muted dark:text-muted-dark">Issuer: </span>
                  <span className="font-mono">{oidc.issuer}</span>
                </div>
                <div>
                  <span className="text-muted dark:text-muted-dark">Client ID: </span>
                  <span className="font-mono">{oidc.client_id}</span>
                </div>
                <div>
                  <span className="text-muted dark:text-muted-dark">Redirect URL: </span>
                  <span className="font-mono">{oidc.redirect_url}</span>
                </div>
              </div>
              <div className="flex items-center gap-2 border-t border-border dark:border-border-dark pt-3">
                <button
                  onClick={() => setShowOidcForm(true)}
                  className="px-3 py-1.5 text-xs font-medium rounded-md
                             border border-border dark:border-border-dark
                             hover:border-accent/30 transition-colors"
                >
                  Edit
                </button>
                <button
                  onClick={handleDeleteOidc}
                  className="px-3 py-1.5 text-xs font-medium rounded-md
                             border border-red-300 dark:border-red-500/30
                             text-red-600 dark:text-red-400
                             hover:bg-red-500/10 transition-colors"
                >
                  Remove
                </button>
              </div>
            </div>
          )}

          {showOidcForm && (
            <OIDCForm
              settings={oidc}
              onClose={() => setShowOidcForm(false)}
              onSaved={handleOidcSaved}
            />
          )}
        </>
      )}
    </div>
  )
}
