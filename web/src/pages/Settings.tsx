import { useState, useEffect, useCallback, useRef } from 'react'
import type { Server, OIDCSettings, TautulliSettings, OverseerrSettings, SonarrSettings, EnrichmentStatus } from '../types'
import { api } from '../lib/api'
import { useFetch } from '../hooks/useFetch'
import { useUnits } from '../hooks/useUnits'
import { ServerForm } from '../components/ServerForm'
import { OIDCForm } from '../components/OIDCForm'
import { MaxMindForm, type MaxMindSettings } from '../components/MaxMindForm'
import { TautulliForm } from '../components/TautulliForm'
import { OverseerrForm } from '../components/OverseerrForm'
import { SonarrForm } from '../components/SonarrForm'
import { EmptyState } from '../components/EmptyState'
import { IntegrationCard } from '../components/IntegrationCard'
import { UserManagement } from '../components/UserManagement'
import { btnOutline, btnDanger } from '../lib/constants'

const serverTypeColors: Record<string, string> = {
  plex: 'badge-warn',
  emby: 'badge-emby',
  jellyfin: 'badge-jellyfin',
}

const tabs: { key: TabKey; label: string }[] = [
  { key: 'servers', label: 'Servers' },
  { key: 'users', label: 'Users' },
  { key: 'auth', label: 'Authentication' },
  { key: 'integrations', label: 'Integrations' },
  { key: 'geoip', label: 'GeoIP' },
  { key: 'import', label: 'Import' },
  { key: 'display', label: 'Display' },
]

type TabKey = 'servers' | 'users' | 'auth' | 'integrations' | 'geoip' | 'import' | 'display'

export function Settings() {
  const [tab, setTab] = useState<TabKey>('servers')
  const { data: servers, loading, error: fetchError, refetch: refetchServers } = useFetch<Server[]>('/api/servers')
  const { data: oidc, loading: oidcLoading, error: oidcFetchError, refetch: refetchOidc } = useFetch<OIDCSettings>(tab === 'auth' ? '/api/settings/oidc' : null)
  const { data: maxmind, loading: maxmindLoading, refetch: refetchMaxmind } = useFetch<MaxMindSettings>(tab === 'geoip' ? '/api/settings/maxmind' : null)
  const { data: tautulli, loading: tautulliLoading, error: tautulliFetchError, refetch: refetchTautulli } = useFetch<TautulliSettings>(tab === 'import' ? '/api/settings/tautulli' : null)
  const { data: overseerr, loading: overseerrLoading, error: overseerrFetchError, refetch: refetchOverseerr } = useFetch<OverseerrSettings>(tab === 'integrations' ? '/api/settings/overseerr' : null)
  const { data: sonarr, loading: sonarrLoading, error: sonarrFetchError, refetch: refetchSonarr } = useFetch<SonarrSettings>(tab === 'integrations' ? '/api/settings/sonarr' : null)

  const [editingServer, setEditingServer] = useState<Server | undefined>()
  const [showForm, setShowForm] = useState(false)
  const [showOidcForm, setShowOidcForm] = useState(false)
  const [showTautulliForm, setShowTautulliForm] = useState(false)
  const [showOverseerrForm, setShowOverseerrForm] = useState(false)
  const [showSonarrForm, setShowSonarrForm] = useState(false)
  const [actionError, setActionError] = useState('')
  const [calculatingHouseholds, setCalculatingHouseholds] = useState(false)
  const [householdResult, setHouseholdResult] = useState<{ created: number } | null>(null)
  const [syncingAvatars, setSyncingAvatars] = useState(false)
  const [avatarSyncResult, setAvatarSyncResult] = useState<{ synced: number; updated: number; errors?: string[] } | null>(null)

  const serverList = servers ?? []
  const oidcConfigured = !!oidc?.issuer
  const tautulliConfigured = !!tautulli?.url

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
    refetchServers()
  }

  async function handleDelete(server: Server) {
    if (!window.confirm(`Delete "${server.name}"?`)) return
    try {
      await api.del(`/api/servers/${server.id}`)
      setActionError('')
      refetchServers()
    } catch {
      setActionError('Failed to delete server')
    }
  }

  function handleOidcSaved() {
    setShowOidcForm(false)
    refetchOidc()
  }

  async function handleDeleteOidc() {
    if (!window.confirm('Remove OIDC configuration? Authentication will be disabled.')) return
    try {
      await api.del('/api/settings/oidc')
      setActionError('')
      refetchOidc()
    } catch {
      setActionError('Failed to delete OIDC configuration')
    }
  }

  function handleTautulliSaved() {
    setShowTautulliForm(false)
    refetchTautulli()
  }

  async function handleDeleteTautulli() {
    if (!window.confirm('Remove Tautulli configuration?')) return
    try {
      await api.del('/api/settings/tautulli')
      setActionError('')
      refetchTautulli()
    } catch {
      setActionError('Failed to delete Tautulli configuration')
    }
  }

  function makeIntegrationSaved(setShow: (v: boolean) => void, refetch: () => void) {
    return () => { setShow(false); refetch() }
  }

  function makeIntegrationDelete(path: string, name: string, refetch: () => void) {
    return async () => {
      if (!window.confirm(`Remove ${name} configuration?`)) return
      try {
        await api.del(path)
        setActionError('')
        refetch()
      } catch {
        setActionError(`Failed to delete ${name} configuration`)
      }
    }
  }

  const handleOverseerrSaved = makeIntegrationSaved(setShowOverseerrForm, refetchOverseerr)
  const handleDeleteOverseerr = makeIntegrationDelete('/api/settings/overseerr', 'Overseerr', refetchOverseerr)
  const handleSonarrSaved = makeIntegrationSaved(setShowSonarrForm, refetchSonarr)
  const handleDeleteSonarr = makeIntegrationDelete('/api/settings/sonarr', 'Sonarr', refetchSonarr)

  async function handleCalculateHouseholds() {
    setCalculatingHouseholds(true)
    setHouseholdResult(null)
    setActionError('')
    try {
      const result = await api.post<{ created: number }>('/api/household/calculate', { min_sessions: 10 })
      setHouseholdResult(result)
    } catch {
      setActionError('Failed to calculate household locations')
    } finally {
      setCalculatingHouseholds(false)
    }
  }

  async function handleSyncUserAvatars() {
    setSyncingAvatars(true)
    setAvatarSyncResult(null)
    setActionError('')
    try {
      const result = await api.post<{ synced: number; updated: number; errors?: string[] }>('/api/users/sync-avatars', {})
      setAvatarSyncResult(result)
    } catch {
      setActionError('Failed to sync user avatars')
    } finally {
      setSyncingAvatars(false)
    }
  }

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

      <div className="flex gap-1 mb-6 border-b border-border dark:border-border-dark overflow-x-auto scrollbar-hide">
        {tabs.map(t => (
          <button
            key={t.key}
            onClick={() => setTab(t.key)}
            className={`px-4 py-2 text-sm font-medium border-b-2 transition-colors whitespace-nowrap shrink-0 ${
              tab === t.key
                ? 'border-accent text-accent'
                : 'border-transparent text-muted dark:text-muted-dark hover:text-gray-800 dark:hover:text-gray-200'
            }`}
          >
            {t.label}
          </button>
        ))}
      </div>

      {actionError && (
        <div className="card p-4 mb-4 text-center text-red-500 dark:text-red-400">
          {actionError}
        </div>
      )}

      {tab === 'servers' && (
        <>
          {loading && <EmptyState icon="⟳" title="Loading..." />}

          {fetchError && !loading && (
            <EmptyState icon="!" title="Failed to load servers">
              <button onClick={refetchServers} className="text-sm text-accent hover:underline">Retry</button>
            </EmptyState>
          )}

          {!loading && !fetchError && serverList.length === 0 && (
            <EmptyState icon="&#9881;" title="No servers configured" description="Add a Plex, Emby, or Jellyfin server to get started" />
          )}

          {!loading && !fetchError && serverList.length > 0 && (
            <>
              <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                {serverList.map(srv => (
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
                      <button onClick={() => openEdit(srv)} aria-label="Edit" className={btnOutline}>
                        Edit
                      </button>
                      <button onClick={() => handleDelete(srv)} aria-label="Delete" className={btnDanger}>
                        Delete
                      </button>
                    </div>
                  </div>
                ))}
              </div>

              <div className="card p-5 mt-4">
                <h3 className="font-semibold text-base mb-2">User Avatars</h3>
                <p className="text-sm text-muted dark:text-muted-dark mb-4">
                  Sync user profile pictures from your media servers. For Plex, avatars are fetched from plex.tv.
                  For Jellyfin/Emby, avatars are loaded from the server.
                </p>
                <div className="flex items-center gap-3">
                  <button
                    onClick={handleSyncUserAvatars}
                    disabled={syncingAvatars}
                    className={btnOutline + (syncingAvatars ? ' opacity-50 cursor-not-allowed' : '')}
                  >
                    {syncingAvatars ? 'Syncing...' : 'Sync User Avatars'}
                  </button>
                  {avatarSyncResult && (
                    <span className="text-sm text-green-500">
                      {avatarSyncResult.synced + avatarSyncResult.updated === 0
                        ? 'No changes needed'
                        : `${avatarSyncResult.synced} new, ${avatarSyncResult.updated} updated`}
                      {avatarSyncResult.errors && avatarSyncResult.errors.length > 0 && (
                        <span className="text-amber-500 ml-2">
                          ({avatarSyncResult.errors.length} error{avatarSyncResult.errors.length > 1 ? 's' : ''})
                        </span>
                      )}
                    </span>
                  )}
                </div>
              </div>

              <div className="card p-5 mt-4">
                <h3 className="font-semibold text-base mb-2">Household Locations</h3>
                <p className="text-sm text-muted dark:text-muted-dark mb-4">
                  Scan watch history to auto-detect home locations based on frequently used IPs (10+ sessions).
                </p>
                <div className="flex items-center gap-3">
                  <button
                    onClick={handleCalculateHouseholds}
                    disabled={calculatingHouseholds}
                    className={btnOutline + (calculatingHouseholds ? ' opacity-50 cursor-not-allowed' : '')}
                  >
                    {calculatingHouseholds ? 'Calculating...' : 'Calculate Household Locations'}
                  </button>
                  {householdResult && (
                    <span className="text-sm text-green-500">
                      {householdResult.created === 0
                        ? 'No new locations found'
                        : `Created ${householdResult.created} new location${householdResult.created > 1 ? 's' : ''}`}
                    </span>
                  )}
                </div>
              </div>
            </>
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

      {tab === 'users' && (
        <UserManagement />
      )}

      {tab === 'auth' && (
        <>
          {oidcLoading && <EmptyState icon="⟳" title="Loading..." />}

          {oidcFetchError && !oidcLoading && (
            <EmptyState icon="!" title="Failed to load OIDC settings">
              <button onClick={refetchOidc} className="text-sm text-accent hover:underline">Retry</button>
            </EmptyState>
          )}

          {!oidcLoading && !oidcFetchError && !oidcConfigured && (
            <EmptyState icon="&#128274;" title="OIDC Not Configured" description="Configure OpenID Connect to enable single sign-on authentication.">
              <button
                onClick={() => setShowOidcForm(true)}
                className="px-4 py-2.5 text-sm font-semibold rounded-lg
                           bg-accent text-gray-900 hover:bg-accent/90 transition-colors"
              >
                Configure OIDC
              </button>
            </EmptyState>
          )}

          {!oidcLoading && !oidcFetchError && oidcConfigured && oidc && (
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
                <button onClick={() => setShowOidcForm(true)} className={btnOutline}>
                  Edit
                </button>
                <button onClick={handleDeleteOidc} className={btnDanger}>
                  Remove
                </button>
              </div>
            </div>
          )}

          {showOidcForm && (
            <OIDCForm
              settings={oidc ?? undefined}
              onClose={() => setShowOidcForm(false)}
              onSaved={handleOidcSaved}
            />
          )}
        </>
      )}

      {tab === 'integrations' && (
        <div className="space-y-4">
          <IntegrationCard
            name="Overseerr"
            icon="&#127916;"
            description="Connect to Overseerr to enable media requests directly from StreamMon."
            settings={overseerr}
            loading={overseerrLoading}
            error={overseerrFetchError}
            onConfigure={() => setShowOverseerrForm(true)}
            onEdit={() => setShowOverseerrForm(true)}
            onDelete={handleDeleteOverseerr}
            onRetry={refetchOverseerr}
          />

          {showOverseerrForm && (
            <OverseerrForm
              settings={overseerr ?? undefined}
              onClose={() => setShowOverseerrForm(false)}
              onSaved={handleOverseerrSaved}
            />
          )}

          <IntegrationCard
            name="Sonarr"
            icon="&#128250;"
            description="Connect to Sonarr to view your TV calendar with upcoming episodes."
            settings={sonarr}
            loading={sonarrLoading}
            error={sonarrFetchError}
            onConfigure={() => setShowSonarrForm(true)}
            onEdit={() => setShowSonarrForm(true)}
            onDelete={handleDeleteSonarr}
            onRetry={refetchSonarr}
          />

          {showSonarrForm && (
            <SonarrForm
              settings={sonarr ?? undefined}
              onClose={() => setShowSonarrForm(false)}
              onSaved={handleSonarrSaved}
            />
          )}
        </div>
      )}

      {tab === 'geoip' && (
        <>
          {maxmindLoading && <EmptyState icon="&#8635;" title="Loading..." />}
          {!maxmindLoading && (
            <MaxMindForm settings={maxmind} onSaved={refetchMaxmind} />
          )}
        </>
      )}

      {tab === 'import' && (
        <>
          {tautulliLoading && <EmptyState icon="&#8635;" title="Loading..." />}

          {tautulliFetchError && !tautulliLoading && (
            <EmptyState icon="!" title="Failed to load Tautulli settings">
              <button onClick={refetchTautulli} className="text-sm text-accent hover:underline">Retry</button>
            </EmptyState>
          )}

          {!tautulliLoading && !tautulliFetchError && !tautulliConfigured && (
            <EmptyState icon="&#128230;" title="Tautulli Not Configured" description="Connect to Tautulli to import your existing watch history.">
              <button
                onClick={() => setShowTautulliForm(true)}
                className="px-4 py-2.5 text-sm font-semibold rounded-lg
                           bg-accent text-gray-900 hover:bg-accent/90 transition-colors"
              >
                Configure Tautulli
              </button>
            </EmptyState>
          )}

          {!tautulliLoading && !tautulliFetchError && tautulliConfigured && tautulli && (
            <div className="card p-5">
              <div className="flex items-start justify-between mb-4">
                <h3 className="font-semibold text-base">Tautulli</h3>
                <span className="badge badge-accent">Connected</span>
              </div>
              <div className="space-y-2 text-sm mb-4">
                <div>
                  <span className="text-muted dark:text-muted-dark">URL: </span>
                  <span className="font-mono">{tautulli.url}</span>
                </div>
              </div>
              <div className="flex items-center gap-2 border-t border-border dark:border-border-dark pt-3">
                <button onClick={() => setShowTautulliForm(true)} className={btnOutline}>
                  Edit
                </button>
                <button onClick={handleDeleteTautulli} className={btnDanger}>
                  Remove
                </button>
              </div>
            </div>
          )}

          {!tautulliLoading && !tautulliFetchError && tautulliConfigured && (
            <TautulliEnrichment />
          )}

          {showTautulliForm && (
            <TautulliForm
              settings={tautulli ?? undefined}
              onClose={() => setShowTautulliForm(false)}
              onSaved={handleTautulliSaved}
            />
          )}
        </>
      )}

      {tab === 'display' && (
        <DisplaySettings />
      )}
    </div>
  )
}

interface EnrichStartResponse {
  total: number
  status: 'none' | 'started'
}

function TautulliEnrichment() {
  const { data: servers } = useFetch<Server[]>('/api/servers')
  const plexServers = servers?.filter(s => s.type === 'plex')

  const [selectedServer, setSelectedServer] = useState<number>(0)
  const [enrichStatus, setEnrichStatus] = useState<EnrichmentStatus | null>(null)
  const [enriching, setEnriching] = useState(false)
  const [enrichDone, setEnrichDone] = useState(false)
  const [error, setError] = useState('')
  const enrichPollRef = useRef<ReturnType<typeof setInterval> | null>(null)
  const mountedRef = useRef(true)

  useEffect(() => {
    if (plexServers && plexServers.length > 0) {
      setSelectedServer(prev => prev === 0 ? plexServers[0].id : prev)
    }
  }, [plexServers])

  useEffect(() => {
    return () => {
      mountedRef.current = false
      if (enrichPollRef.current) clearInterval(enrichPollRef.current)
    }
  }, [])

  const startEnrichPoll = useCallback(() => {
    if (enrichPollRef.current) clearInterval(enrichPollRef.current)
    enrichPollRef.current = setInterval(async () => {
      try {
        const status = await api.get<EnrichmentStatus>('/api/settings/tautulli/enrich/status')
        if (!mountedRef.current) return
        setEnrichStatus(status)
        if (!status.running) {
          setEnriching(false)
          setEnrichDone(true)
          if (enrichPollRef.current) {
            clearInterval(enrichPollRef.current)
            enrichPollRef.current = null
          }
        }
      } catch {
        // ignore poll errors
      }
    }, 2000)
  }, [])

  useEffect(() => {
    if (!selectedServer) return
    api.get<EnrichmentStatus>(`/api/settings/tautulli/enrich/status?server_id=${selectedServer}`)
      .then(status => {
        if (!mountedRef.current) return
        setEnrichStatus(status)
        if (status.running) {
          setEnriching(true)
          startEnrichPoll()
        }
      })
      .catch(() => {})
  }, [selectedServer, startEnrichPoll])

  async function handleStopEnrich() {
    try {
      await api.post('/api/settings/tautulli/enrich/stop', {})
      if (!mountedRef.current) return
      setEnriching(false)
      setEnrichDone(true)
      if (enrichPollRef.current) {
        clearInterval(enrichPollRef.current)
        enrichPollRef.current = null
      }
      const url = selectedServer
        ? `/api/settings/tautulli/enrich/status?server_id=${selectedServer}`
        : '/api/settings/tautulli/enrich/status'
      api.get<EnrichmentStatus>(url)
        .then(status => { if (mountedRef.current) setEnrichStatus(status) })
        .catch(() => {})
    } catch {
      // poll will pick up the stopped state
    }
  }

  async function handleEnrich() {
    if (!selectedServer) {
      setError('Please select a server')
      return
    }

    setEnrichDone(false)
    setError('')

    try {
      const resp = await api.post<EnrichStartResponse>('/api/settings/tautulli/enrich', {
        server_id: selectedServer,
      })
      if (!mountedRef.current) return

      if (resp.status === 'none') return

      setEnriching(true)
      setEnrichStatus({ running: true, processed: 0, total: resp.total, server_id: selectedServer })
      startEnrichPoll()
    } catch (err) {
      if (mountedRef.current) setError((err as Error).message)
    }
  }

  const enrichPending = enrichStatus && !enrichStatus.running && enrichStatus.total > 0 && !enrichDone
  const enrichRunning = enriching && enrichStatus?.running
  const enrichPct = enrichStatus && enrichStatus.total > 0
    ? Math.round((enrichStatus.processed / enrichStatus.total) * 100)
    : 0

  if (!plexServers?.length) return null

  const selectClass = `w-full px-3 py-2.5 rounded-lg text-sm
    bg-surface dark:bg-surface-dark
    border border-border dark:border-border-dark
    focus:outline-none focus:border-accent/50 focus:ring-1 focus:ring-accent/20
    transition-colors`

  return (
    <div className="card p-5 mt-4">
      <h3 className="font-semibold text-base mb-2">Stream Detail Enrichment</h3>
      <p className="text-sm text-muted dark:text-muted-dark mb-4">
        Imported Tautulli records only contain basic metadata. Enrichment fetches full stream
        details (resolution, bitrate, codec, player, transcoding info) from Tautulli for each
        record, filling in data needed for detailed statistics and charts.
      </p>

      <div className="flex flex-col sm:flex-row gap-3 sm:items-end">
        <div className="flex-1">
          <label htmlFor="enrich-server" className="block text-sm font-medium mb-1.5">
            Server
          </label>
          <select
            id="enrich-server"
            value={selectedServer}
            onChange={e => setSelectedServer(Number(e.target.value))}
            className={selectClass}
          >
            {plexServers.map(srv => (
              <option key={srv.id} value={srv.id}>
                {srv.name}
              </option>
            ))}
          </select>
        </div>
        {!enrichRunning && (
          <button
            type="button"
            onClick={handleEnrich}
            disabled={enriching || !selectedServer}
            className="px-4 py-2.5 text-sm font-medium rounded-lg
                       bg-accent text-gray-900 hover:bg-accent/90
                       disabled:opacity-50 transition-colors"
          >
            Enrich Now
          </button>
        )}
      </div>

      {enrichPending && !enrichRunning && (
        <p className="text-sm text-muted dark:text-muted-dark mt-3">
          {enrichStatus.total.toLocaleString()} records awaiting enrichment
        </p>
      )}

      {enrichRunning && enrichStatus && (
        <div className="mt-3 space-y-2">
          <div className="flex justify-between text-xs text-muted dark:text-muted-dark">
            <span>Enriching stream details...</span>
            <span>{enrichStatus.processed.toLocaleString()} / {enrichStatus.total.toLocaleString()} ({enrichPct}%)</span>
          </div>
          <div className="flex gap-3 items-center">
            <div className="flex-1 bg-surface dark:bg-surface-dark rounded-full h-2 overflow-hidden">
              <div
                className="bg-accent h-2 rounded-full transition-all duration-300"
                style={{ width: `${enrichPct}%` }}
              />
            </div>
            <button
              type="button"
              onClick={handleStopEnrich}
              className="px-3 py-1 text-xs font-medium rounded-lg
                         border border-red-400/50 text-red-500 dark:text-red-400
                         hover:bg-red-500/10 transition-colors"
            >
              Stop
            </button>
          </div>
        </div>
      )}

      {enrichDone && enrichStatus && !enrichStatus.running && (
        <div className="mt-3 text-sm font-mono px-3 py-2 rounded-lg bg-green-500/10 text-green-600 dark:text-green-400">
          Enriched {enrichStatus.processed.toLocaleString()} records
        </div>
      )}

      {error && (
        <div className="mt-3 text-sm text-red-500 dark:text-red-400 font-mono">
          {error}
        </div>
      )}
    </div>
  )
}

function DisplaySettings() {
  const { system, setSystem } = useUnits()

  return (
    <div className="card p-5">
      <h3 className="font-semibold text-base mb-4">Units</h3>
      <p className="text-sm text-muted dark:text-muted-dark mb-4">
        Choose how distances and speeds are displayed throughout the app.
      </p>
      <div className="flex gap-2">
        <button
          onClick={() => setSystem('metric')}
          className={`px-4 py-2 text-sm font-medium rounded-lg transition-colors ${
            system === 'metric'
              ? 'bg-accent text-gray-900'
              : 'bg-gray-100 dark:bg-white/10 text-gray-700 dark:text-gray-300 hover:bg-gray-200 dark:hover:bg-white/20'
          }`}
        >
          Metric (km, km/h)
        </button>
        <button
          onClick={() => setSystem('imperial')}
          className={`px-4 py-2 text-sm font-medium rounded-lg transition-colors ${
            system === 'imperial'
              ? 'bg-accent text-gray-900'
              : 'bg-gray-100 dark:bg-white/10 text-gray-700 dark:text-gray-300 hover:bg-gray-200 dark:hover:bg-white/20'
          }`}
        >
          Imperial (mi, mph)
        </button>
      </div>
    </div>
  )
}
